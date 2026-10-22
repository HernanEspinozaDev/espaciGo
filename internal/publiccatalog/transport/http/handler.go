package publiccataloghttp

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/credentials"
	"github.com/HernanEspinozaDev/espaciGo/internal/publiccatalog"
)

type Handler struct {
	service *publiccatalog.Service
	origins map[string]bool
}

var (
	publicAttributeCodePattern   = regexp.MustCompile(`^[a-z][a-z0-9_]{0,59}$`)
	publicAttributeValuePattern  = regexp.MustCompile(`^(0|[1-9][0-9]*)(\.[0-9]+)?$`)
	publicAttributeOptionPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,79}$`)
)

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
	const imagesPrefix = "/api/v1/public/spaces/"
	if strings.HasPrefix(r.URL.Path, imagesPrefix) && strings.Contains(strings.TrimPrefix(r.URL.Path, imagesPrefix), "/images/") {
		if r.URL.RawQuery != "" {
			publicError(w, 404)
			return
		}
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, imagesPrefix), "/images/")
		if len(parts) != 2 || parts[0] == "" || strings.Contains(parts[0], "/") {
			publicError(w, 404)
			return
		}
		if strings.Contains(parts[1], "/variants/") {
			variantParts := strings.Split(parts[1], "/variants/")
			if len(variantParts) != 2 || variantParts[0] == "" || variantParts[1] == "" || strings.Contains(variantParts[0], "/") || strings.Contains(variantParts[1], "/") {
				publicError(w, 404)
				return
			}
			item, blob, err := h.service.MediaImageContent(r.Context(), parts[0], variantParts[0], variantParts[1])
			if errors.Is(err, publiccatalog.ErrNotFound) {
				publicError(w, 404)
				return
			}
			if err != nil {
				publicError(w, 500)
				return
			}
			w.Header().Set("Content-Type", item.MIME)
			w.Header().Set("Content-Length", strconv.Itoa(len(blob)))
			w.Header().Set("Content-Disposition", "inline; filename=space-image.webp")
			w.Header().Set("ETag", `"`+item.SHA256+`"`)
			w.Header().Set("Cache-Control", "public, max-age=60, must-revalidate")
			w.WriteHeader(200)
			_, _ = w.Write(blob)
			return
		}
		if parts[1] == "" || strings.Contains(parts[1], "/") {
			publicError(w, 404)
			return
		}
		item, blob, err := h.service.ImageContent(r.Context(), parts[0], parts[1])
		if errors.Is(err, publiccatalog.ErrNotFound) {
			publicError(w, 404)
			return
		}
		if err != nil {
			publicError(w, 500)
			return
		}
		w.Header().Set("Content-Type", item.MIME)
		w.Header().Set("Content-Length", strconv.Itoa(len(blob)))
		w.Header().Set("Content-Disposition", "inline; filename=espacigo-public-image.png")
		w.WriteHeader(200)
		_, _ = w.Write(blob)
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
	case strings.HasPrefix(r.URL.Path, "/api/v1/public/space-categories/") && strings.HasSuffix(r.URL.Path, "/filters"):
		if len(r.URL.Query()) != 0 {
			publicError(w, 422)
			return
		}
		category := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/v1/public/space-categories/"), "/filters")
		if category == "" || strings.Contains(category, "/") {
			publicError(w, 404)
			return
		}
		filters, err := h.service.AttributeFilters(r.Context(), category)
		if errors.Is(err, publiccatalog.ErrNotFound) {
			publicError(w, 404)
			return
		}
		if err != nil {
			publicError(w, 500)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"category_code": category, "items": filters})
	case r.URL.Path == "/api/v1/public/locations":
		query, region, limit, ok := publicLocationFilters(r)
		if !ok {
			publicError(w, 422)
			return
		}
		locations, err := h.service.Locations(r.Context(), query, region, limit)
		if errors.Is(err, publiccatalog.ErrInvalid) {
			publicError(w, 422)
			return
		}
		if err != nil {
			publicError(w, 500)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"items": locations})
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
	allowed := map[string]bool{
		"category_code": true, "rate_unit": true, "min_unit_price_clp": true, "max_unit_price_clp": true,
		"sort": true, "page_size": true, "cursor": true, "q": true, "region_code": true,
		"comuna_code": true, "min_area_m2": true, "attribute": true, "start_at": true,
		"end_at": true, "min_total_clp": true, "max_total_clp": true,
	}
	for key, values := range q {
		if !allowed[key] || (key != "attribute" && len(values) != 1) || (key == "attribute" && len(values) > 32) {
			return publiccatalog.Filters{}, false
		}
	}
	f := publiccatalog.Filters{
		CategoryCode: q.Get("category_code"), RateUnit: q.Get("rate_unit"), Sort: q.Get("sort"), Cursor: q.Get("cursor"),
		Query: q.Get("q"), RegionCode: q.Get("region_code"), CommuneCode: q.Get("comuna_code"), MinAreaM2: q.Get("min_area_m2"),
	}
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
	if len(q["attribute"]) > 0 {
		var ok bool
		f.Attributes, f.TypedAttributes, ok = parsePublicAttributeFilters(q["attribute"])
		if !ok {
			return f, false
		}
	}
	for key, target := range map[string]**int64{"min_total_clp": &f.MinTotalCLP, "max_total_clp": &f.MaxTotalCLP} {
		if raw, ok := q[key]; ok {
			n, e := strconv.ParseInt(raw[0], 10, 64)
			if e != nil || n < 0 {
				return f, false
			}
			*target = &n
		}
	}
	for key, target := range map[string]**time.Time{"start_at": &f.StartAt, "end_at": &f.EndAt} {
		if raw, ok := q[key]; ok {
			parsed, e := time.Parse(time.RFC3339Nano, raw[0])
			if e != nil {
				return f, false
			}
			instant := parsed.UTC()
			*target = &instant
		}
	}
	return f, true
}

func parsePublicAttributeFilters(entries []string) (map[string]bool, map[string]publiccatalog.AttributePredicate, bool) {
	booleans := make(map[string]bool)
	typed := make(map[string]publiccatalog.AttributePredicate)
	for _, raw := range entries {
		if len(raw) > 180 {
			return nil, nil, false
		}
		parts := strings.SplitN(raw, ":", 3)
		if len(parts) < 2 || !publicAttributeCodePattern.MatchString(parts[0]) {
			return nil, nil, false
		}
		code := parts[0]
		if len(parts) == 2 {
			if parts[1] != "true" && parts[1] != "false" {
				return nil, nil, false
			}
			if _, duplicate := booleans[code]; duplicate {
				return nil, nil, false
			}
			if _, duplicate := typed[code]; duplicate {
				return nil, nil, false
			}
			booleans[code] = parts[1] == "true"
			continue
		}
		if _, duplicate := booleans[code]; duplicate {
			return nil, nil, false
		}
		predicate := typed[code]
		value := parts[2]
		switch parts[1] {
		case "gte", "lte":
			if !publicAttributeValuePattern.MatchString(value) || predicate.Value != nil || len(predicate.Values) > 0 {
				return nil, nil, false
			}
			bound := value
			if parts[1] == "gte" {
				if predicate.Minimum != nil {
					return nil, nil, false
				}
				predicate.Minimum = &bound
			} else {
				if predicate.Maximum != nil {
					return nil, nil, false
				}
				predicate.Maximum = &bound
			}
		case "eq":
			if !publicAttributeOptionPattern.MatchString(value) || predicate.Minimum != nil || predicate.Maximum != nil || predicate.Value != nil || len(predicate.Values) > 0 {
				return nil, nil, false
			}
			predicate.Value = &value
		case "in":
			if predicate.Minimum != nil || predicate.Maximum != nil || predicate.Value != nil {
				return nil, nil, false
			}
			values := strings.Split(value, ",")
			if len(values) < 1 || len(values) > 32 {
				return nil, nil, false
			}
			seen := make(map[string]bool, len(predicate.Values)+len(values))
			for _, previous := range predicate.Values {
				seen[previous] = true
			}
			for _, option := range values {
				if !publicAttributeOptionPattern.MatchString(option) || seen[option] {
					return nil, nil, false
				}
				seen[option] = true
			}
			predicate.Values = append(predicate.Values, values...)
			sort.Strings(predicate.Values)
		default:
			return nil, nil, false
		}
		typed[code] = predicate
	}
	if len(booleans)+len(typed) > 16 {
		return nil, nil, false
	}
	return booleans, typed, true
}

func publicLocationFilters(r *http.Request) (query, region string, limit int, ok bool) {
	q := r.URL.Query()
	allowed := map[string]bool{"q": true, "region_code": true, "limit": true}
	for key, values := range q {
		if !allowed[key] || len(values) != 1 {
			return "", "", 0, false
		}
	}
	limit = 400
	if raw, exists := q["limit"]; exists {
		value, err := strconv.Atoi(raw[0])
		if err != nil {
			return "", "", 0, false
		}
		limit = value
	}
	return q.Get("q"), q.Get("region_code"), limit, true
}

func publicError(w http.ResponseWriter, status int) {
	w.WriteHeader(status)
	code := map[int]string{403: "origin_denied", 404: "not_found", 422: "invalid_request", 405: "method_not_allowed", 500: "internal_error"}[status]
	_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": code, "message": "No fue posible completar la consulta pública.", "request_id": w.Header().Get("X-Request-ID")}})
}
