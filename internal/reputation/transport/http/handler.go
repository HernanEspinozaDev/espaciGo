package reputationhttp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/credentials"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
	"github.com/HernanEspinozaDev/espaciGo/internal/reputation"
)

type Authenticator interface {
	Authorize(context.Context, identity.Secret, identity.Role, identity.ActivityKind) (identity.Principal, error)
}
type Handler struct {
	auth    Authenticator
	service *reputation.Service
	origins map[string]bool
}

func NewHandler(auth Authenticator, s *reputation.Service, origins []string) *Handler {
	m := map[string]bool{}
	for _, v := range origins {
		m[v] = true
	}
	return &Handler{auth: auth, service: s, origins: m}
}
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Prototype-Safety", reputation.SafetyNotice)
	rid, _ := (credentials.Generator{}).ID()
	w.Header().Set("X-Request-ID", rid)
	w.Header().Add("Vary", "Origin")
	if origin := r.Header.Get("Origin"); origin != "" {
		if !h.origins[origin] {
			h.writeError(w, 403, "origin_denied", rid)
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Idempotency-Key")
		w.Header().Set("Access-Control-Expose-Headers", "X-Request-ID, X-Prototype-Safety")
	}
	if r.Method == http.MethodOptions {
		w.WriteHeader(204)
		return
	}
	path := strings.TrimSuffix(r.URL.Path, "/")
	if strings.HasPrefix(path, "/api/v1/spaces/") && strings.HasSuffix(path, "/reviews") && r.Method == http.MethodGet {
		space := strings.TrimSuffix(strings.TrimPrefix(path, "/api/v1/spaces/"), "/reviews")
		if strings.Contains(space, "/") {
			h.writeError(w, 404, "not_found", rid)
			return
		}
		v, e := h.service.ListSpace(r.Context(), space)
		h.reply(w, 200, v, e, rid)
		return
	}
	if path == "/api/v1/local/reputation/me" && r.Method == http.MethodGet {
		p, e := h.authorize(w, r, "", rid)
		if e != nil {
			return
		}
		v, err := h.service.MyReputation(r.Context(), p.AccountID)
		h.reply(w, 200, v, err, rid)
		return
	}
	const base = "/api/v1/local/booking-trial/reservations/"
	if strings.HasPrefix(path, base) {
		rest := strings.TrimPrefix(path, base)
		parts := strings.Split(rest, "/")
		if len(parts) >= 2 && parts[0] != "" && parts[1] == "reviews" {
			p, e := h.authorize(w, r, "", rid)
			if e != nil {
				return
			}
			if len(parts) == 2 {
				switch r.Method {
				case http.MethodGet:
					v, err := h.service.ListReservation(r.Context(), p.AccountID, parts[0])
					h.reply(w, 200, map[string]any{"items": v}, err, rid)
				case http.MethodPost:
					var in reputation.ReviewInput
					if !decode(w, r, &in) {
						h.writeError(w, 422, "invalid_request", rid)
						return
					}
					key := r.Header.Get("Idempotency-Key")
					if key == "" {
						h.writeError(w, 422, "idempotency_key_required", rid)
						return
					}
					v, err := h.service.Create(r.Context(), p.AccountID, parts[0], key, in)
					status := 201
					if v.Reused {
						status = 200
					}
					h.reply(w, status, v, err, rid)
				default:
					h.writeError(w, 404, "not_found", rid)
				}
				return
			}
			if len(parts) == 4 && parts[2] != "" && parts[3] == "report" && r.Method == http.MethodPost {
				var in reputation.ReportInput
				if !decode(w, r, &in) {
					h.writeError(w, 422, "invalid_request", rid)
					return
				}
				key := r.Header.Get("Idempotency-Key")
				if key == "" {
					h.writeError(w, 422, "idempotency_key_required", rid)
					return
				}
				v, err := h.service.ReportReview(r.Context(), p.AccountID, parts[0], parts[2], key, in.Reason)
				status := 201
				if v.Reused {
					status = 200
				}
				h.reply(w, status, v, err, rid)
				return
			}
		}
	}
	const adminBase = "/api/v1/admin/local/review-reports"
	if path == adminBase && r.Method == http.MethodGet {
		if _, e := h.authorize(w, r, identity.RoleAdministrator, rid); e != nil {
			return
		}
		v, e := h.service.Reports(r.Context())
		h.reply(w, 200, map[string]any{"items": v}, e, rid)
		return
	}
	if strings.HasPrefix(path, adminBase+"/") && r.Method == http.MethodPost {
		p, e := h.authorize(w, r, identity.RoleAdministrator, rid)
		if e != nil {
			return
		}
		parts := strings.Split(strings.TrimPrefix(path, adminBase+"/"), "/")
		if len(parts) != 2 || (parts[1] != "hide" && parts[1] != "dismiss") {
			h.writeError(w, 404, "not_found", rid)
			return
		}
		var in struct {
			Reason      string `json:"reason_code"`
			Correlation string `json:"correlation_id"`
		}
		if !decode(w, r, &in) {
			h.writeError(w, 422, "invalid_request", rid)
			return
		}
		key := r.Header.Get("Idempotency-Key")
		if key == "" {
			h.writeError(w, 422, "idempotency_key_required", rid)
			return
		}
		decision := "desestimar"
		if parts[1] == "hide" {
			decision = "ocultar"
		}
		v, err := h.service.Moderate(r.Context(), p.AccountID, parts[0], decision, in.Reason, key, in.Correlation)
		h.reply(w, 200, v, err, rid)
		return
	}
	h.writeError(w, 404, "not_found", rid)
}
func (h *Handler) authorize(w http.ResponseWriter, r *http.Request, role identity.Role, rid string) (identity.Principal, error) {
	p, e := h.auth.Authorize(r.Context(), identity.Secret(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")), role, identity.UserOperation)
	if e != nil {
		if errors.Is(e, identity.ErrForbidden) {
			h.writeError(w, 403, "forbidden", rid)
		} else {
			w.Header().Set("WWW-Authenticate", "Bearer")
			h.writeError(w, 401, "unauthenticated", rid)
		}
		return identity.Principal{}, e
	}
	return p, nil
}
func decode(w http.ResponseWriter, r *http.Request, target any) bool {
	d := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	d.DisallowUnknownFields()
	if d.Decode(target) != nil {
		return false
	}
	var extra any
	return errors.Is(d.Decode(&extra), io.EOF)
}
func (h *Handler) reply(w http.ResponseWriter, status int, value any, err error, rid string) {
	if err != nil {
		switch {
		case errors.Is(err, reputation.ErrInvalid):
			h.writeError(w, 422, "invalid_request", rid)
		case errors.Is(err, reputation.ErrNotFound):
			h.writeError(w, 404, "not_found", rid)
		case errors.Is(err, reputation.ErrConflict):
			h.writeError(w, 409, "state_or_idempotency_conflict", rid)
		default:
			h.writeError(w, 500, "internal_error", rid)
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"data": value, "notice": reputation.SafetyNotice})
}
func (h *Handler) writeError(w http.ResponseWriter, status int, code, rid string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": code, "message": "No fue posible completar la operación de reputación sintética.", "request_id": rid}})
}
