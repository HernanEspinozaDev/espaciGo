package damageclaimhttp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/credentials"
	"github.com/HernanEspinozaDev/espaciGo/internal/damageclaim"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
)

type Authenticator interface {
	Authorize(context.Context, identity.Secret, identity.Role, identity.ActivityKind) (identity.Principal, error)
}
type Handler struct {
	auth    Authenticator
	service *damageclaim.Service
	origins map[string]struct{}
}

func NewHandler(auth Authenticator, service *damageclaim.Service, allowedOrigins []string) http.Handler {
	origins := map[string]struct{}{}
	for _, o := range allowedOrigins {
		if o != "" {
			origins[o] = struct{}{}
		}
	}
	return &Handler{auth: auth, service: service, origins: origins}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Prototype-Safety", damageclaim.SafetyNotice)
	w.Header().Add("Vary", "Origin")
	reqid, _ := (credentials.Generator{}).ID()
	w.Header().Set("X-Request-ID", reqid)
	if origin := r.Header.Get("Origin"); origin != "" {
		if _, ok := h.origins[origin]; !ok {
			writeError(w, 403, "origin_denied", reqid)
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Idempotency-Key")
		w.Header().Set("Access-Control-Expose-Headers", "X-Request-ID, X-Prototype-Safety")
	}
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	principal, err := h.auth.Authorize(r.Context(), identity.Secret(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")), "", identity.UserOperation)
	if err != nil {
		w.Header().Set("WWW-Authenticate", "Bearer")
		writeError(w, http.StatusUnauthorized, "unauthenticated", reqid)
		return
	}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 7 || parts[0] != "api" || parts[1] != "v1" || parts[2] != "local" || parts[3] != "booking-trial" || parts[4] != "reservations" {
		writeError(w, 404, "not_found", reqid)
		return
	}
	reservation := parts[5]
	if len(parts) == 7 && parts[6] == "damage-claim" && r.Method == http.MethodGet {
		item, e := h.service.Get(r.Context(), principal.AccountID, reservation)
		h.reply(w, 200, item, e, reqid)
		return
	}
	if len(parts) == 7 && parts[6] == "damage-claim" && r.Method == http.MethodPost {
		var input damageclaim.Input
		if !decode(r, &input) {
			writeError(w, 422, "invalid_request", reqid)
			return
		}
		if r.Header.Get("Idempotency-Key") == "" {
			writeError(w, 422, "idempotency_key_required", reqid)
			return
		}
		item, e := h.service.Open(r.Context(), principal.AccountID, reservation, r.Header.Get("Idempotency-Key"), input)
		status := 201
		if item.Reused {
			status = 200
		}
		h.reply(w, status, item, e, reqid)
		return
	}
	if len(parts) == 8 && parts[6] == "damage-claim" && parts[7] == "defense" && r.Method == http.MethodPost {
		var input damageclaim.Input
		if !decode(r, &input) {
			writeError(w, 422, "invalid_request", reqid)
			return
		}
		if r.Header.Get("Idempotency-Key") == "" {
			writeError(w, 422, "idempotency_key_required", reqid)
			return
		}
		item, e := h.service.Defend(r.Context(), principal.AccountID, reservation, r.Header.Get("Idempotency-Key"), input)
		status := 201
		if item.Reused {
			status = 200
		}
		h.reply(w, status, item, e, reqid)
		return
	}
	writeError(w, 404, "not_found", reqid)
}
func decode(r *http.Request, target any) bool {
	d := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	d.DisallowUnknownFields()
	if d.Decode(target) != nil {
		return false
	}
	var extra any
	return errors.Is(d.Decode(&extra), io.EOF)
}
func (h *Handler) reply(w http.ResponseWriter, status int, item any, err error, requestID string) {
	if err != nil {
		switch {
		case errors.Is(err, damageclaim.ErrInvalid):
			writeError(w, 422, "invalid_request", requestID)
		case errors.Is(err, damageclaim.ErrNotFound):
			writeError(w, 404, "not_found", requestID)
		case errors.Is(err, damageclaim.ErrConflict):
			writeError(w, 409, "state_or_deadline_conflict", requestID)
		default:
			writeError(w, 500, "internal_error", requestID)
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"data": item, "notice": damageclaim.SafetyNotice})
}
func writeError(w http.ResponseWriter, status int, code, requestID string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": code, "message": "No fue posible registrar el reclamo sintético.", "request_id": requestID}})
}
