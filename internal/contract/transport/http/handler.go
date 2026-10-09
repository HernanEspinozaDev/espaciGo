package contracthttp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/credentials"
	"github.com/HernanEspinozaDev/espaciGo/internal/contract"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
)

type Authenticator interface {
	Authorize(context.Context, identity.Secret, identity.Role, identity.ActivityKind) (identity.Principal, error)
}
type Handler struct {
	auth    Authenticator
	service *contract.Service
}

func NewHandler(auth Authenticator, service *contract.Service) http.Handler {
	return &Handler{auth: auth, service: service}
}
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Prototype-Safety", contract.SafetyNotice)
	rid, _ := (credentials.Generator{}).ID()
	w.Header().Set("X-Request-ID", rid)
	p, err := h.auth.Authorize(r.Context(), identity.Secret(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")), "", identity.UserOperation)
	if err != nil {
		w.Header().Set("WWW-Authenticate", "Bearer")
		writeError(w, 401, "unauthenticated")
		return
	}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 6 || parts[0] != "api" || parts[1] != "v1" || parts[2] != "local" || parts[3] != "booking-trial" {
		writeError(w, 404, "not_found")
		return
	}
	if len(parts) == 7 && parts[4] == "reservations" && parts[6] == "contract" && r.Method == http.MethodPost {
		v, e := h.service.Create(r.Context(), p.AccountID, parts[5])
		h.reply(w, v, e)
		return
	}
	if len(parts) >= 6 && parts[4] == "contracts" {
		id := parts[5]
		if len(parts) == 6 && r.Method == http.MethodGet {
			v, e := h.service.Get(r.Context(), p.AccountID, id)
			h.reply(w, v, e)
			return
		}
		if len(parts) == 7 && parts[6] == "sign" && r.Method == http.MethodPost {
			v, e := h.service.Sign(r.Context(), p.AccountID, id)
			h.reply(w, v, e)
			return
		}
		if len(parts) == 7 && parts[6] == "reject" && r.Method == http.MethodPost {
			v, e := h.service.Reject(r.Context(), p.AccountID, id, "")
			h.reply(w, v, e)
			return
		}
		if len(parts) == 7 && parts[6] == "document" && r.Method == http.MethodGet {
			v, e := h.service.Get(r.Context(), p.AccountID, id)
			if e != nil {
				h.reply(w, nil, e)
				return
			}
			if v.State != "firmado" {
				writeError(w, 409, "contract_not_signed")
				return
			}
			w.Header().Set("Content-Type", "application/pdf")
			w.Header().Set("Content-Disposition", "attachment; filename=contrato-ensayo-sintetico.pdf")
			w.WriteHeader(200)
			_, _ = w.Write(v.Artifact)
			return
		}
	}
	writeError(w, 404, "not_found")
}
func (h *Handler) reply(w http.ResponseWriter, v any, err error) {
	if err != nil {
		switch {
		case errors.Is(err, contract.ErrInvalid):
			writeError(w, 422, "invalid_request")
		case errors.Is(err, contract.ErrNotFound):
			writeError(w, 404, "not_found")
		case errors.Is(err, contract.ErrConflict):
			writeError(w, 409, "conflict")
		default:
			writeError(w, 500, "internal_error")
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"data": v, "notice": contract.SafetyNotice})
}
func decode(r *http.Request, v any) bool {
	d := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	d.DisallowUnknownFields()
	return d.Decode(v) == nil
}
func writeError(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": code, "message": "No fue posible completar la operación de ensayo.", "request_id": w.Header().Get("X-Request-ID")}, "safety_notice": contract.SafetyNotice})
}
