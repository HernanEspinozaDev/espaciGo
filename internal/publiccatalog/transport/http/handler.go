package publiccataloghttp

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/credentials"
	"github.com/HernanEspinozaDev/espaciGo/internal/publiccatalog"
)

type Handler struct {
	service *publiccatalog.Service
	origins map[string]bool
}

func New(service *publiccatalog.Service, allowedOrigins ...string) http.Handler {
	origins := map[string]bool{}
	for _, origin := range allowedOrigins {
		origins[origin] = true
	}
	return &Handler{service: service, origins: origins}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rid, err := (credentials.Generator{}).ID()
	if err != nil {
		publicError(w, http.StatusInternalServerError)
		return
	}
	w.Header().Set("X-Request-ID", rid)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Add("Vary", "Origin")
	if origin := r.Header.Get("Origin"); origin != "" {
		if !h.origins[origin] {
			publicError(w, http.StatusForbidden)
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	}
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	switch {
	case r.URL.Path == "/api/v1/public/space-categories":
		if len(r.URL.Query()) != 0 {
			publicError(w, 422)
			return
		}
		v, err := h.service.Categories(r.Context())
		if err != nil {
			publicError(w, 500)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"items": v})
	case r.URL.Path == "/api/v1/public/spaces":
		f, ok := publicFilters(r)
		if !ok {
			publicError(w, 422)
			return
		}
		v, err := h.service.List(r.Context(), f)
		if errors.Is(err, publiccatalog.ErrInvalid) {
			publicError(w, 422)
			return
		}
		if err != nil {
			publicError(w, 500)
			return
		}
		_ = json.NewEncoder(w).Encode(v)
	case strings.HasPrefix(r.URL.Path, "/api/v1/public/spaces/"):
		id := strings.TrimPrefix(r.URL.Path, "/api/v1/public/spaces/")
		if id == "" || strings.Contains(id, "/") || r.URL.RawQuery != "" {
			publicError(w, 404)
			return
		}
		v, err := h.service.Detail(r.Context(), id)
		if errors.Is(err, publiccatalog.ErrNotFound) {
			publicError(w, 404)
			return
		}
		if err != nil {
			publicError(w, 500)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": v})
	default:
		publicError(w, 404)
	}
}

func publicFilters(r *http.Request) (publiccatalog.Filters, bool) {
	q := r.URL.Query()
	allowed := map[string]bool{"category_code": true, "rate_unit": true, "min_unit_price_clp": true, "max_unit_price_clp": true, "sort": true, "page_size": true, "cursor": true}
	for key, values := range q {
		if !allowed[key] || len(values) != 1 {
			return publiccatalog.Filters{}, false
		}
	}
	f := publiccatalog.Filters{CategoryCode: q.Get("category_code"), RateUnit: q.Get("rate_unit"), Sort: q.Get("sort"), Cursor: q.Get("cursor")}
	if raw, ok := q["page_size"]; ok {
		n, e := strconv.Atoi(raw[0])
		if e != nil {
			return f, false
		}
		f.PageSize = n
		f.PageSizeSet = true
	}
	for key, target := range map[string]**int64{"min_unit_price_clp": &f.MinUnitPriceCLP, "max_unit_price_clp": &f.MaxUnitPriceCLP} {
		if raw, ok := q[key]; ok {
			n, e := strconv.ParseInt(raw[0], 10, 64)
			if e != nil || n < 0 {
				return f, false
			}
			*target = &n
		}
	}
	return f, true
}

func publicError(w http.ResponseWriter, status int) {
	w.WriteHeader(status)
	code := map[int]string{403: "origin_denied", 404: "not_found", 422: "invalid_request", 405: "method_not_allowed", 500: "internal_error"}[status]
	_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": code, "message": "No fue posible completar la consulta pública.", "request_id": w.Header().Get("X-Request-ID")}})
}
