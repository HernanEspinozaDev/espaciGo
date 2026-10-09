package noticehttp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/credentials"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
	"github.com/HernanEspinozaDev/espaciGo/internal/localnotice"
)

type Authenticator interface {
	Authorize(context.Context, identity.Secret, identity.Role, identity.ActivityKind) (identity.Principal, error)
}
type Handler struct {
	auth    Authenticator
	service *localnotice.Service
	origins map[string]bool
}

func NewHandler(a Authenticator, s *localnotice.Service, origins []string) *Handler {
	m := map[string]bool{}
	for _, o := range origins {
		m[o] = true
	}
	return &Handler{auth: a, service: s, origins: m}
}
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	rid, _ := (credentials.Generator{}).ID()
	w.Header().Set("X-Request-ID", rid)
	w.Header().Add("Vary", "Origin")
	if o := r.Header.Get("Origin"); o != "" {
		if !h.origins[o] {
			h.err(w, 403, "origin_denied", rid)
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", o)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Idempotency-Key")
		w.Header().Set("Access-Control-Expose-Headers", "X-Request-ID")
	}
	if r.Method == http.MethodOptions {
		w.WriteHeader(204)
		return
	}
	p, e := h.auth.Authorize(r.Context(), identity.Secret(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")), identity.RoleAdministrator, identity.UserOperation)
	if e != nil {
		if errors.Is(e, identity.ErrForbidden) {
			h.err(w, 403, "forbidden", rid)
		} else {
			w.Header().Set("WWW-Authenticate", "Bearer")
			h.err(w, 401, "unauthenticated", rid)
		}
		return
	}
	const base = "/api/v1/admin/local/notices"
	path := strings.TrimSuffix(r.URL.Path, "/")
	if path == base && r.Method == http.MethodGet {
		items, err := h.service.Terminal(r.Context(), 50)
		h.reply(w, 200, map[string]any{"items": items}, err, rid)
		return
	}
	if strings.HasPrefix(path, base+"/") && r.Method == http.MethodPost {
		parts := strings.Split(strings.TrimPrefix(path, base+"/"), "/")
		if len(parts) != 2 || parts[1] != "reopen" {
			h.err(w, 404, "not_found", rid)
			return
		}
		var in struct {
			Reason      string `json:"reason_code"`
			Correlation string `json:"correlation_id"`
		}
		d := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
		d.DisallowUnknownFields()
		if d.Decode(&in) != nil {
			h.err(w, 422, "invalid_request", rid)
			return
		}
		key := r.Header.Get("Idempotency-Key")
		out, err := h.service.Reopen(r.Context(), parts[0], p.AccountID, in.Reason, key, in.Correlation)
		status := 201
		if out.Reused {
			status = 200
		}
		h.reply(w, status, out, err, rid)
		return
	}
	h.err(w, 404, "not_found", rid)
}
func (h *Handler) reply(w http.ResponseWriter, status int, v any, e error, rid string) {
	if e != nil {
		code := 500
		name := "internal_error"
		if errors.Is(e, localnotice.ErrInvalid) {
			code = 422
			name = "invalid_request"
		}
		h.err(w, code, name, rid)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"data": v, "notice": "ENSAYO LOCAL — AVISO SINTÉTICO"})
}
func (h *Handler) err(w http.ResponseWriter, status int, code, rid string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": code, "message": "No fue posible completar la operación administrativa del aviso local.", "request_id": rid}})
}
