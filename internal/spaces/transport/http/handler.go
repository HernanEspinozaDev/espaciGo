package spaceshttp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/credentials"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
	"github.com/HernanEspinozaDev/espaciGo/internal/spaces"
)

type Authenticator interface {
	Authorize(context.Context, identity.Secret, identity.Role, identity.ActivityKind) (identity.Principal, error)
}
type Handler struct {
	auth    Authenticator
	service *spaces.Service
	origins map[string]bool
}

func NewHandler(auth Authenticator, service *spaces.Service, origins []string) http.Handler {
	m := map[string]bool{}
	for _, v := range origins {
		m[v] = true
	}
	return &Handler{auth, service, m}
}
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	id, err := (credentials.Generator{}).ID()
	if err != nil {
		write(w, 500, map[string]any{"error": map[string]string{"code": "internal_error", "message": "Ocurrió un error inesperado."}})
		return
	}
	w.Header().Set("X-Request-ID", id)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Add("Vary", "Origin")
	if origin := r.Header.Get("Origin"); origin != "" {
		if !h.origins[origin] {
			failure(w, 403, "origin_denied", "Origen no permitido.")
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, OPTIONS")
	}
	if r.Method == http.MethodOptions {
		w.WriteHeader(204)
		return
	}
	if r.URL.RawQuery != "" {
		failure(w, 400, "invalid_request", "No se admiten parámetros de URL.")
		return
	}
	principal, e := h.auth.Authorize(r.Context(), identity.Secret(bearer(r.Header.Get("Authorization"))), "", identity.UserOperation)
	if e != nil {
		if errors.Is(e, identity.ErrForbidden) {
			failure(w, 403, "forbidden", "Permiso insuficiente.")
		} else {
			w.Header().Set("WWW-Authenticate", "Bearer")
			failure(w, 401, "unauthenticated", "Credencial ausente, incorrecta o expirada.")
		}
		return
	}
	path := strings.TrimSuffix(r.URL.Path, "/")
	if path == "/api/v1/spaces/categories" && r.Method == http.MethodGet {
		items, e := h.service.Categories(r.Context())
		if e != nil {
			failure(w, 500, "internal_error", "Ocurrió un error inesperado.")
			return
		}
		write(w, 200, map[string]any{"items": items})
		return
	}
	if path == "/api/v1/spaces" && r.Method == http.MethodGet {
		items, e := h.service.ListOwn(r.Context(), principal.AccountID)
		if e != nil {
			failure(w, 500, "internal_error", "Ocurrió un error inesperado.")
			return
		}
		write(w, 200, map[string]any{"items": items})
		return
	}
	if path == "/api/v1/spaces" && r.Method == http.MethodPost {
		var in spaces.Input
		if !decode(w, r, &in) {
			return
		}
		item, e := h.service.Create(r.Context(), principal.AccountID, in)
		if e != nil {
			serviceError(w, e)
			return
		}
		write(w, 201, item)
		return
	}
	if strings.HasPrefix(path, "/api/v1/spaces/") {
		id := strings.TrimPrefix(path, "/api/v1/spaces/")
		if strings.Contains(id, "/") {
			failure(w, 404, "not_found", "Recurso no encontrado.")
			return
		}
		switch r.Method {
		case http.MethodGet:
			item, e := h.service.GetOwn(r.Context(), principal.AccountID, id)
			if e != nil {
				serviceError(w, e)
				return
			}
			write(w, 200, item)
			return
		case http.MethodPut:
			var in spaces.Input
			if !decode(w, r, &in) {
				return
			}
			item, e := h.service.UpdateOwn(r.Context(), principal.AccountID, id, in)
			if e != nil {
				serviceError(w, e)
				return
			}
			write(w, 200, item)
			return
		}
	}
	failure(w, 404, "not_found", "Recurso no encontrado.")
}
func bearer(v string) string {
	p := strings.SplitN(v, " ", 2)
	if len(p) != 2 || !strings.EqualFold(p[0], "Bearer") {
		return ""
	}
	return p[1]
}
func decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	mt, _, e := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if e != nil || mt != "application/json" {
		failure(w, 415, "unsupported_media_type", "Se requiere application/json.")
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16*1024)
	defer r.Body.Close()
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if d.Decode(dst) != nil {
		failure(w, 400, "invalid_request", "El cuerpo JSON no es válido.")
		return false
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		failure(w, 400, "invalid_request", "El cuerpo JSON debe contener un único objeto.")
		return false
	}
	return true
}
func serviceError(w http.ResponseWriter, e error) {
	switch {
	case errors.Is(e, spaces.ErrInvalid):
		failure(w, 422, "validation_error", "Revisa los campos obligatorios y sus límites.")
	case errors.Is(e, spaces.ErrNotFound):
		failure(w, 404, "not_found", "Borrador no encontrado.")
	default:
		failure(w, 500, "internal_error", "Ocurrió un error inesperado.")
	}
}
func failure(w http.ResponseWriter, status int, code, msg string) {
	write(w, status, map[string]any{"error": map[string]string{"code": code, "message": msg}})
}
func write(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
