package disputehttp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/credentials"
	"github.com/HernanEspinozaDev/espaciGo/internal/dispute"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
)

type Authenticator interface {
	Authorize(context.Context, identity.Secret, identity.Role, identity.ActivityKind) (identity.Principal, error)
}

type Handler struct {
	auth    Authenticator
	service *dispute.Service
	origins map[string]bool
}

func NewHandler(auth Authenticator, service *dispute.Service, origins []string) http.Handler {
	allowed := make(map[string]bool, len(origins))
	for _, origin := range origins {
		allowed[origin] = true
	}
	return &Handler{auth: auth, service: service, origins: allowed}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	id, err := (credentials.Generator{}).ID()
	if err != nil {
		fail(w, http.StatusInternalServerError, "internal_error")
		return
	}
	w.Header().Set("X-Request-ID", id)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Add("Vary", "Origin")
	if origin := r.Header.Get("Origin"); origin != "" {
		if !h.origins[origin] {
			fail(w, http.StatusForbidden, "origin_denied")
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Expose-Headers", "X-Request-ID")
	}
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Idempotency-Key")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.URL.RawQuery != "" {
		fail(w, http.StatusBadRequest, "invalid_request")
		return
	}
	const reservationPrefix = "/api/v1/local/booking-trial/reservations/"
	const participantHistoryPrefix = "/api/v1/local/booking-trial/disputes/"
	const adminPrefix = "/api/v1/admin/disputes"
	path := strings.TrimSuffix(r.URL.Path, "/")
	if strings.HasPrefix(path, participantHistoryPrefix) {
		tail := strings.TrimPrefix(path, participantHistoryPrefix)
		parts := strings.Split(tail, "/")
		if len(parts) != 2 || !canonicalUUID(parts[0]) || parts[1] != "history" || r.Method != http.MethodGet {
			fail(w, http.StatusNotFound, "not_found")
			return
		}
		principal, ok := h.authenticate(w, r, "")
		if !ok {
			return
		}
		items, err := h.service.History(r.Context(), principal.AccountID, parts[0])
		if err != nil {
			h.replyError(w, err)
			return
		}
		write(w, http.StatusOK, map[string]any{"items": items})
		return
	}
	if strings.HasPrefix(path, reservationPrefix) {
		tail := strings.TrimPrefix(path, reservationPrefix)
		parts := strings.Split(tail, "/")
		if len(parts) != 2 || !canonicalUUID(parts[0]) || parts[1] != "disputes" || r.Method != http.MethodGet && r.Method != http.MethodPost {
			fail(w, http.StatusNotFound, "not_found")
			return
		}
		principal, ok := h.authenticate(w, r, "")
		if !ok {
			return
		}
		if r.Method == http.MethodGet {
			items, err := h.service.ListForParticipant(r.Context(), principal.AccountID, parts[0])
			if err != nil {
				h.replyError(w, err)
				return
			}
			write(w, http.StatusOK, map[string]any{"items": items, "scope": "local_synthetic_disputes"})
			return
		}
		var input struct {
			ReasonCode string `json:"reason_code"`
		}
		if !decode(w, r, &input) {
			return
		}
		item, err := h.service.Open(r.Context(), principal.AccountID, parts[0], input.ReasonCode, r.Header.Get("Idempotency-Key"))
		if err != nil {
			h.replyError(w, err)
			return
		}
		status := http.StatusCreated
		if item.Reused {
			status = http.StatusOK
		}
		write(w, status, item)
		return
	}
	if path == adminPrefix {
		if r.Method != http.MethodGet {
			fail(w, http.StatusMethodNotAllowed, "method_not_allowed")
			return
		}
		principal, ok := h.authenticate(w, r, identity.RoleAdministrator)
		if !ok {
			return
		}
		_ = principal
		items, err := h.service.ListOpen(r.Context())
		if err != nil {
			h.replyError(w, err)
			return
		}
		write(w, http.StatusOK, map[string]any{"items": items})
		return
	}
	if strings.HasPrefix(path, adminPrefix+"/") {
		tail := strings.TrimPrefix(path, adminPrefix+"/")
		parts := strings.Split(tail, "/")
		if len(parts) != 2 || !canonicalUUID(parts[0]) || parts[1] != "close" || r.Method != http.MethodPost {
			fail(w, http.StatusNotFound, "not_found")
			return
		}
		principal, ok := h.authenticate(w, r, identity.RoleAdministrator)
		if !ok {
			return
		}
		var input struct {
			ReasonCode string `json:"reason_code"`
		}
		if !decode(w, r, &input) {
			return
		}
		item, err := h.service.Close(r.Context(), principal.AccountID, parts[0], input.ReasonCode)
		if err != nil {
			h.replyError(w, err)
			return
		}
		write(w, http.StatusOK, item)
		return
	}
	fail(w, http.StatusNotFound, "not_found")
}

func (h *Handler) authenticate(w http.ResponseWriter, r *http.Request, role identity.Role) (identity.Principal, bool) {
	parts := strings.Fields(r.Header.Get("Authorization"))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		w.Header().Set("WWW-Authenticate", "Bearer")
		fail(w, http.StatusUnauthorized, "unauthenticated")
		return identity.Principal{}, false
	}
	principal, err := h.auth.Authorize(r.Context(), identity.Secret(parts[1]), role, identity.UserOperation)
	if err != nil {
		if errors.Is(err, identity.ErrForbidden) {
			fail(w, http.StatusForbidden, "forbidden")
		} else {
			w.Header().Set("WWW-Authenticate", "Bearer")
			fail(w, http.StatusUnauthorized, "unauthenticated")
		}
		return identity.Principal{}, false
	}
	return principal, true
}

func (h *Handler) replyError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, dispute.ErrInvalid):
		fail(w, http.StatusUnprocessableEntity, "invalid_request")
	case errors.Is(err, dispute.ErrNotFound):
		fail(w, http.StatusNotFound, "not_found")
	case errors.Is(err, dispute.ErrConflict):
		fail(w, http.StatusConflict, "conflict")
	default:
		fail(w, http.StatusInternalServerError, "internal_error")
	}
}

func decode(w http.ResponseWriter, r *http.Request, target any) bool {
	if r.Header.Get("Content-Type") != "application/json" {
		fail(w, http.StatusUnsupportedMediaType, "unsupported_media_type")
		return false
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		fail(w, http.StatusBadRequest, "invalid_json")
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		fail(w, http.StatusBadRequest, "invalid_json")
		return false
	}
	return true
}

func canonicalUUID(value string) bool {
	if len(value) != 36 || strings.ToLower(value) != value {
		return false
	}
	for index, character := range value {
		if index == 8 || index == 13 || index == 18 || index == 23 {
			if character != '-' {
				return false
			}
		} else if !(character >= '0' && character <= '9' || character >= 'a' && character <= 'f') {
			return false
		}
	}
	return true
}

func fail(w http.ResponseWriter, status int, code string) {
	requestID := w.Header().Get("X-Request-ID")
	if requestID == "" {
		generated, err := (credentials.Generator{}).ID()
		if err != nil {
			generated = "00000000-0000-4000-8000-000000000000"
		}
		requestID = generated
		w.Header().Set("X-Request-ID", requestID)
	}
	messages := map[string]string{
		"origin_denied":          "El origen de la solicitud no está permitido.",
		"invalid_request":        "La solicitud contiene campos inválidos.",
		"not_found":              "No se encontró la incidencia o reserva solicitada.",
		"method_not_allowed":     "El método HTTP no está permitido para esta operación.",
		"unauthenticated":        "La sesión no es válida o expiró.",
		"forbidden":              "La cuenta no tiene permiso para esta operación.",
		"unsupported_media_type": "El cuerpo debe usar application/json.",
		"invalid_json":           "El cuerpo JSON no es válido.",
		"conflict":               "La incidencia cambió o la solicitud entra en conflicto con su estado actual.",
		"internal_error":         "Ocurrió un error inesperado.",
	}
	message, ok := messages[code]
	if !ok {
		message = "No fue posible completar la solicitud."
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": code, "message": message, "request_id": requestID}})
}

func write(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
