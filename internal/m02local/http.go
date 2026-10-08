package m02local

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/credentials"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
)

type Authenticator interface {
	Authorize(context.Context, identity.Secret, identity.Role, identity.ActivityKind) (identity.Principal, error)
}
type Handler struct {
	auth    Authenticator
	service *Service
	origins map[string]bool
}

func NewHandler(auth Authenticator, s *Service, origins []string) http.Handler {
	m := map[string]bool{}
	for _, o := range origins {
		m[o] = true
	}
	return &Handler{auth: auth, service: s, origins: m}
}
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	requestID, err := (credentials.Generator{}).ID()
	if err != nil {
		write(w, http.StatusInternalServerError, "internal_error", "Ocurrió un error inesperado.")
		return
	}
	w.Header().Set("X-Request-ID", requestID)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Add("Vary", "Origin")
	if o := r.Header.Get("Origin"); o != "" {
		if !h.origins[o] {
			write(w, 403, "origin_denied", "Origen no permitido.")
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", o)
		w.Header().Set("Access-Control-Expose-Headers", "X-Request-ID")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Idempotency-Key")
		w.Header().Set("Access-Control-Allow-Methods", "GET, PUT, DELETE, OPTIONS")
	}
	if r.Method == http.MethodOptions {
		w.WriteHeader(204)
		return
	}
	if r.URL.RawQuery != "" {
		write(w, 422, "validation_error", "No se admiten parámetros de consulta.")
		return
	}
	parts := strings.Fields(r.Header.Get("Authorization"))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		w.Header().Set("WWW-Authenticate", "Bearer")
		write(w, 401, "unauthenticated", "Credencial ausente o expirada.")
		return
	}
	p, err := h.auth.Authorize(r.Context(), identity.Secret(parts[1]), "", identity.UserOperation)
	if err != nil {
		w.Header().Set("WWW-Authenticate", "Bearer")
		write(w, 401, "unauthenticated", "Credencial ausente o expirada.")
		return
	}
	key := r.Header.Get("Idempotency-Key")
	path := strings.TrimSuffix(r.URL.Path, "/")
	switch path {
	case "/api/v1/profile/photo":
		switch r.Method {
		case http.MethodGet:
			item, e := h.service.CurrentPhoto(r.Context(), p.AccountID)
			if e != nil {
				h.failure(w, e)
				return
			}
			writeJSON(w, 200, item)
		case http.MethodPut:
			if !emptyBody(w, r) {
				return
			}
			item, reused, e := h.service.SetPhoto(r.Context(), p.AccountID, key)
			if e != nil {
				h.failure(w, e)
				return
			}
			writeJSON(w, 200, map[string]any{"photo": item, "reused": reused, "synthetic": true})
		case http.MethodDelete:
			reused, e := h.service.RemovePhoto(r.Context(), p.AccountID, key)
			if e != nil {
				h.failure(w, e)
				return
			}
			writeJSON(w, 200, map[string]any{"removed": true, "reused": reused})
		default:
			write(w, 405, "method_not_allowed", "Método no permitido.")
		}
	case "/api/v1/profile/photo/content":
		if r.Method != http.MethodGet {
			write(w, 405, "method_not_allowed", "Método no permitido.")
			return
		}
		item, b, e := h.service.PhotoContent(r.Context(), p.AccountID)
		if e != nil {
			h.failure(w, e)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Content-Disposition", "inline; filename=profile-synthetic.png")
		w.Header().Set("X-Content-SHA256", item.SHA256)
		w.WriteHeader(200)
		_, _ = w.Write(b)
	case "/api/v1/payout-account":
		switch r.Method {
		case http.MethodGet:
			item, e := h.service.CurrentPayout(r.Context(), p.AccountID)
			if e != nil {
				h.failure(w, e)
				return
			}
			writeJSON(w, 200, item)
		case http.MethodPut:
			if !emptyBody(w, r) {
				return
			}
			item, reused, e := h.service.SetPayout(r.Context(), p.AccountID, key)
			if e != nil {
				h.failure(w, e)
				return
			}
			writeJSON(w, 200, map[string]any{"account": item, "reused": reused, "simulation_only": true})
		case http.MethodDelete:
			reused, e := h.service.RevokePayout(r.Context(), p.AccountID, key)
			if e != nil {
				h.failure(w, e)
				return
			}
			writeJSON(w, 200, map[string]any{"revoked": true, "reused": reused})
		default:
			write(w, 405, "method_not_allowed", "Método no permitido.")
		}
	default:
		write(w, 404, "not_found", "Recurso no encontrado.")
	}
}
func emptyBody(w http.ResponseWriter, r *http.Request) bool {
	b, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 1))
	if e != nil || len(b) != 0 {
		write(w, 422, "validation_error", "Esta operación no acepta archivos ni campos proporcionados por el cliente.")
		return false
	}
	return true
}
func (h *Handler) failure(w http.ResponseWriter, e error) {
	switch {
	case errors.Is(e, identity.ErrInvalid):
		write(w, 422, "validation_error", "Solicitud inválida.")
	case errors.Is(e, identity.ErrForbidden), errors.Is(e, ErrIneligible):
		write(w, 409, "eligibility_required", "Requiere cuenta activa y KYC sintético vigente.")
	case errors.Is(e, ErrNotFound):
		write(w, 404, "not_found", "Recurso propio no encontrado.")
	case errors.Is(e, ErrConflict):
		write(w, 409, "conflict", "La operación no puede completarse en el estado actual.")
	default:
		write(w, 500, "internal_error", "Ocurrió un error inesperado.")
	}
}
func write(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": msg, "request_id": w.Header().Get("X-Request-ID")}})
}
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
