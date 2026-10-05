package spaceshttp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/credentials"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
	"github.com/HernanEspinozaDev/espaciGo/internal/occupancy"
	"github.com/HernanEspinozaDev/espaciGo/internal/spaces"
)

type Authenticator interface {
	Authorize(context.Context, identity.Secret, identity.Role, identity.ActivityKind) (identity.Principal, error)
}
type Handler struct {
	auth     Authenticator
	service  *spaces.Service
	calendar CalendarService
	origins  map[string]bool
}

type CalendarService interface {
	SetTimeZone(context.Context, string, string, string) error
	TimeZone(context.Context, string, string) (string, error)
	Availability(context.Context, string, string, string, string) (occupancy.Availability, error)
	ListBlocks(context.Context, string, string, string, string) (occupancy.Calendar, error)
	CreateBlock(context.Context, string, string, occupancy.BlockInput) (occupancy.Block, error)
	DeleteBlock(context.Context, string, string, string) error
}

func NewHandler(auth Authenticator, service *spaces.Service, origins []string, calendar ...CalendarService) http.Handler {
	m := map[string]bool{}
	for _, v := range origins {
		m[v] = true
	}
	var calendarService CalendarService
	if len(calendar) > 0 {
		calendarService = calendar[0]
	}
	return &Handler{auth: auth, service: service, calendar: calendarService, origins: m}
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
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	}
	if r.Method == http.MethodOptions {
		w.WriteHeader(204)
		return
	}
	path := strings.TrimSuffix(r.URL.Path, "/")
	calendarPath := strings.Contains(path, "/availability")
	if r.URL.RawQuery != "" && !calendarPath {
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
	const profilesPrefix = "/api/v1/spaces/categories/"
	if strings.HasPrefix(path, profilesPrefix) && r.Method == http.MethodGet {
		parts := strings.Split(strings.TrimPrefix(path, profilesPrefix), "/")
		if len(parts) != 2 && len(parts) != 3 || parts[0] == "" || parts[1] != "attributes" {
			failure(w, 404, "not_found", "Perfil no encontrado.")
			return
		}
		version := 0
		if len(parts) == 3 {
			version, e = strconv.Atoi(parts[2])
			if e != nil || version < 1 {
				failure(w, 404, "not_found", "Perfil no encontrado.")
				return
			}
		}
		profile, e := h.service.Profile(r.Context(), parts[0], version)
		if e != nil {
			serviceError(w, e)
			return
		}
		write(w, 200, profile)
		return
	}
	if calendarPath {
		h.serveCalendar(w, r, path, principal.AccountID)
		return
	}
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

func (h *Handler) serveCalendar(w http.ResponseWriter, r *http.Request, path, owner string) {
	if h.calendar == nil {
		failure(w, 404, "not_found", "Recurso no encontrado.")
		return
	}
	const prefix = "/api/v1/spaces/"
	if !strings.HasPrefix(path, prefix) {
		failure(w, 404, "not_found", "Recurso no encontrado.")
		return
	}
	parts := strings.Split(strings.TrimPrefix(path, prefix), "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] != "availability" {
		failure(w, 404, "not_found", "Recurso no encontrado.")
		return
	}
	spaceID := parts[0]
	if len(parts) == 2 {
		switch r.Method {
		case http.MethodPut:
			if r.URL.RawQuery != "" {
				failure(w, 400, "invalid_request", "No se admiten parámetros de URL.")
				return
			}
			var in struct {
				TimeZone string `json:"time_zone"`
			}
			if !decode(w, r, &in) {
				return
			}
			if err := h.calendar.SetTimeZone(r.Context(), owner, spaceID, in.TimeZone); err != nil {
				calendarError(w, err)
				return
			}
			write(w, 200, map[string]string{"space_id": spaceID, "time_zone": in.TimeZone})
		case http.MethodGet:
			from, to, ok := queryWindow(w, r.URL.Query())
			if !ok {
				return
			}
			result, err := h.calendar.Availability(r.Context(), owner, spaceID, from, to)
			if err != nil {
				calendarError(w, err)
				return
			}
			write(w, 200, result)
		default:
			failure(w, 404, "not_found", "Recurso no encontrado.")
		}
		return
	}
	if len(parts) == 3 && parts[2] == "blocks" {
		switch r.Method {
		case http.MethodGet:
			from, to, ok := queryWindow(w, r.URL.Query())
			if !ok {
				return
			}
			result, err := h.calendar.ListBlocks(r.Context(), owner, spaceID, from, to)
			if err != nil {
				calendarError(w, err)
				return
			}
			write(w, 200, result)
		case http.MethodPost:
			if r.URL.RawQuery != "" {
				failure(w, 400, "invalid_request", "No se admiten parámetros de URL.")
				return
			}
			var in occupancy.BlockInput
			if !decode(w, r, &in) {
				return
			}
			result, err := h.calendar.CreateBlock(r.Context(), owner, spaceID, in)
			if err != nil {
				calendarError(w, err)
				return
			}
			write(w, 201, result)
		default:
			failure(w, 404, "not_found", "Recurso no encontrado.")
		}
		return
	}
	if len(parts) == 3 && parts[2] == "timezone" && r.Method == http.MethodGet && r.URL.RawQuery == "" {
		zone, err := h.calendar.TimeZone(r.Context(), owner, spaceID)
		if err != nil {
			calendarError(w, err)
			return
		}
		write(w, 200, map[string]string{"space_id": spaceID, "time_zone": zone})
		return
	}
	if len(parts) == 4 && parts[2] == "blocks" && r.Method == http.MethodDelete && r.URL.RawQuery == "" {
		if err := h.calendar.DeleteBlock(r.Context(), owner, spaceID, parts[3]); err != nil {
			calendarError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	failure(w, 404, "not_found", "Recurso no encontrado.")
}

func queryWindow(w http.ResponseWriter, values url.Values) (string, string, bool) {
	if len(values) != 2 || len(values["from"]) != 1 || len(values["to"]) != 1 || values.Get("from") == "" || values.Get("to") == "" {
		failure(w, 400, "invalid_request", "Se requieren from y to como timestamps RFC 3339.")
		return "", "", false
	}
	return values.Get("from"), values.Get("to"), true
}

func calendarError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, occupancy.ErrInvalid):
		failure(w, 422, "validation_error", "Revisa la zona horaria y el intervalo indicado.")
	case errors.Is(err, occupancy.ErrNotFound):
		failure(w, 404, "not_found", "Espacio o bloqueo no encontrado.")
	case errors.Is(err, occupancy.ErrConflict):
		failure(w, 409, "occupancy_conflict", "El intervalo se superpone con una ocupación activa.")
	case errors.Is(err, occupancy.ErrTimezoneRequired):
		failure(w, 409, "time_zone_required", "Configura la zona horaria del espacio antes de usar el calendario.")
	default:
		failure(w, 500, "internal_error", "Ocurrió un error inesperado.")
	}
}
func decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	mt, _, e := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if e != nil || mt != "application/json" {
		failure(w, 415, "unsupported_media_type", "Se requiere application/json.")
		return false
	}
	// Leave room for the common draft fields; the versioned attribute object
	// has its own stricter 16 KiB limit in the domain validator.
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	defer r.Body.Close()
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(dst); err != nil {
		var typeErr *json.UnmarshalTypeError
		if errors.As(err, &typeErr) && (typeErr.Field == "area_m2" || typeErr.Field == "capacity" || typeErr.Field == "base_price_clp") {
			failure(w, 422, "validation_error", "Los límites numéricos del borrador no son válidos.")
			return false
		}
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
	write(w, status, map[string]any{"error": map[string]string{"code": code, "message": msg, "request_id": w.Header().Get("X-Request-ID")}})
}
func write(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
