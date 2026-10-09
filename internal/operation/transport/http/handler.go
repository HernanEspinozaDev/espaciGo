package operationhttp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/credentials"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
	"github.com/HernanEspinozaDev/espaciGo/internal/operation"
)

type Authenticator interface {
	Authorize(context.Context, identity.Secret, identity.Role, identity.ActivityKind) (identity.Principal, error)
}

type Handler struct {
	auth    Authenticator
	service *operation.Service
	origins map[string]struct{}
}

func NewHandler(auth Authenticator, service *operation.Service, allowedOrigins []string) http.Handler {
	origins := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		if origin != "" {
			origins[origin] = struct{}{}
		}
	}
	return &Handler{auth: auth, service: service, origins: origins}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Prototype-Safety", operation.SafetyNotice)
	w.Header().Add("Vary", "Origin")
	requestID, _ := (credentials.Generator{}).ID()
	w.Header().Set("X-Request-ID", requestID)
	if origin := r.Header.Get("Origin"); origin != "" {
		if _, ok := h.origins[origin]; !ok {
			writeError(w, http.StatusForbidden, "origin_denied", requestID)
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Expose-Headers", "X-Request-ID, X-Prototype-Safety")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Idempotency-Key")
	}
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	principal, err := h.auth.Authorize(r.Context(), identity.Secret(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")), "", identity.UserOperation)
	if err != nil {
		w.Header().Set("WWW-Authenticate", "Bearer")
		writeError(w, http.StatusUnauthorized, "unauthenticated", requestID)
		return
	}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 6 || parts[0] != "api" || parts[1] != "v1" || parts[2] != "local" || parts[3] != "booking-trial" || parts[4] != "reservations" {
		writeError(w, http.StatusNotFound, "not_found", requestID)
		return
	}
	reservationID := parts[5]
	if len(parts) == 7 && parts[6] == "operations" && r.Method == http.MethodGet {
		items, e := h.service.List(r.Context(), principal.AccountID, reservationID)
		h.reply(w, http.StatusOK, map[string]any{"items": items}, e, requestID)
		return
	}
	if len(parts) == 7 && parts[6] == "check-in" && r.Method == http.MethodPost {
		h.record(w, r, principal.AccountID, reservationID, operation.CheckIn, requestID)
		return
	}
	if len(parts) == 7 && parts[6] == "check-out" && r.Method == http.MethodPost {
		h.record(w, r, principal.AccountID, reservationID, operation.CheckOut, requestID)
		return
	}
	if len(parts) == 7 && parts[6] == "reception" && r.Method == http.MethodPost {
		h.record(w, r, principal.AccountID, reservationID, operation.Receipt, requestID)
		return
	}
	if len(parts) == 8 && parts[6] == "evidence" && r.Method == http.MethodGet {
		item, content, e := h.service.EvidenceContent(r.Context(), principal.AccountID, reservationID, parts[7])
		if e != nil {
			h.reply(w, 0, nil, e, requestID)
			return
		}
		w.Header().Set("Content-Type", item.MIME)
		w.Header().Set("Content-Disposition", "inline; filename=operacion-sintetica.png")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(content)
		return
	}
	writeError(w, http.StatusNotFound, "not_found", requestID)
}

func (h *Handler) record(w http.ResponseWriter, r *http.Request, actor, reservation, kind, requestID string) {
	if r.Header.Get("Idempotency-Key") == "" {
		writeError(w, http.StatusUnprocessableEntity, "idempotency_key_required", requestID)
		return
	}
	var input operation.Input
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "invalid_request", requestID)
		return
	}
	input.Kind = kind
	item, err := h.service.Record(r.Context(), actor, reservation, r.Header.Get("Idempotency-Key"), input)
	if err != nil {
		h.reply(w, 0, nil, err, requestID)
		return
	}
	status := http.StatusCreated
	if item.Reused {
		status = http.StatusOK
	}
	h.reply(w, status, item, nil, requestID)
}

func (h *Handler) reply(w http.ResponseWriter, status int, data any, err error, requestID string) {
	if err != nil {
		switch {
		case errors.Is(err, operation.ErrInvalid):
			writeError(w, http.StatusUnprocessableEntity, "invalid_request", requestID)
		case errors.Is(err, operation.ErrNotFound):
			writeError(w, http.StatusNotFound, "not_found", requestID)
		case errors.Is(err, operation.ErrConflict):
			writeError(w, http.StatusConflict, "state_or_date_conflict", requestID)
		case errors.Is(err, operation.ErrFileStore):
			writeError(w, http.StatusServiceUnavailable, "synthetic_evidence_unavailable", requestID)
		default:
			writeError(w, http.StatusInternalServerError, "internal_error", requestID)
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"data": data, "notice": operation.SafetyNotice})
}

func writeError(w http.ResponseWriter, status int, code, requestID string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": code, "message": "No fue posible registrar la operación local.", "request_id": requestID}})
}
