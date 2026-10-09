package galleryhttp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/credentials"
	"github.com/HernanEspinozaDev/espaciGo/internal/gallery"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
)

type Authenticator interface {
	Authorize(context.Context, identity.Secret, identity.Role, identity.ActivityKind) (identity.Principal, error)
}

type Handler struct {
	auth    Authenticator
	service *gallery.Service
	origins map[string]bool
}

func NewHandler(auth Authenticator, service *gallery.Service, origins []string) http.Handler {
	m := map[string]bool{}
	for _, origin := range origins {
		m[origin] = true
	}
	return &Handler{auth: auth, service: service, origins: m}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	id, err := (credentials.Generator{}).ID()
	if err != nil {
		failure(w, 500, "internal_error", "Ocurrió un error inesperado.")
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
		w.Header().Set("Access-Control-Expose-Headers", "X-Request-ID")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Idempotency-Key")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
	}
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.URL.RawQuery != "" {
		failure(w, 422, "validation_error", "No se admiten parámetros de consulta.")
		return
	}
	spaceID, photoID, content, ok := route(r.URL.Path)
	if !ok {
		failure(w, 404, "not_found", "Recurso no encontrado.")
		return
	}
	if h.auth == nil || h.service == nil {
		failure(w, 404, "not_found", "Recurso no encontrado.")
		return
	}
	parts := strings.Fields(r.Header.Get("Authorization"))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		w.Header().Set("WWW-Authenticate", "Bearer")
		failure(w, 401, "unauthenticated", "Credencial ausente o expirada.")
		return
	}
	principal, err := h.auth.Authorize(r.Context(), identity.Secret(parts[1]), identity.RoleLandlord, identity.UserOperation)
	if err != nil {
		if errors.Is(err, identity.ErrForbidden) {
			failure(w, 403, "forbidden", "Se requiere el rol anfitrión.")
		} else {
			w.Header().Set("WWW-Authenticate", "Bearer")
			failure(w, 401, "unauthenticated", "Credencial ausente o expirada.")
		}
		return
	}
	switch {
	case photoID == "" && r.Method == http.MethodGet:
		items, err := h.service.List(r.Context(), principal.AccountID, spaceID)
		if err != nil {
			galleryError(w, err)
			return
		}
		write(w, 200, map[string]any{"items": items, "max_photos": gallery.MaxPhotosPerSpace, "synthetic": true})
	case photoID == "" && r.Method == http.MethodPost:
		if !emptyBody(w, r) {
			return
		}
		item, reused, err := h.service.AddSynthetic(r.Context(), principal.AccountID, spaceID, r.Header.Get("Idempotency-Key"))
		if err != nil {
			galleryError(w, err)
			return
		}
		status := http.StatusCreated
		if reused {
			status = http.StatusOK
		}
		write(w, status, map[string]any{"photo": item, "reused": reused, "synthetic": true})
	case photoID != "" && content && r.Method == http.MethodGet:
		item, blob, err := h.service.Content(r.Context(), principal.AccountID, spaceID, photoID)
		if err != nil {
			galleryError(w, err)
			return
		}
		w.Header().Set("Content-Type", item.MIME)
		w.Header().Set("Content-Disposition", "inline; filename=space-gallery-synthetic.png")
		w.Header().Set("X-Content-SHA256", item.SHA256)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(blob)
	case photoID != "" && !content && r.Method == http.MethodGet:
		item, err := h.service.Metadata(r.Context(), principal.AccountID, spaceID, photoID)
		if err != nil {
			galleryError(w, err)
			return
		}
		write(w, 200, item)
	case photoID != "" && !content && r.Method == http.MethodDelete:
		result, err := h.service.Remove(r.Context(), principal.AccountID, spaceID, photoID)
		if err != nil {
			galleryError(w, err)
			return
		}
		write(w, 200, result)
	default:
		failure(w, 405, "method_not_allowed", "Método no permitido.")
	}
}

func route(path string) (spaceID, photoID string, content, ok bool) {
	const prefix = "/api/v1/spaces/"
	if !strings.HasPrefix(path, prefix) {
		return "", "", false, false
	}
	parts := strings.Split(strings.Trim(strings.TrimPrefix(path, prefix), "/"), "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] != "gallery" {
		return "", "", false, false
	}
	if len(parts) == 2 {
		return parts[0], "", false, true
	}
	if len(parts) == 3 && parts[2] != "" {
		return parts[0], parts[2], false, true
	}
	if len(parts) == 4 && parts[2] != "" && parts[3] == "content" {
		return parts[0], parts[2], true, true
	}
	return "", "", false, false
}

func emptyBody(w http.ResponseWriter, r *http.Request) bool {
	if r.Body == nil || r.Body == http.NoBody {
		return true
	}
	defer r.Body.Close()
	b, err := io.ReadAll(io.LimitReader(r.Body, 2))
	if err != nil || len(b) > 0 {
		failure(w, 400, "invalid_request", "No se admite contenido del cliente; el Backend genera la imagen sintética.")
		return false
	}
	return true
}

func galleryError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, gallery.ErrInvalid):
		failure(w, 422, "validation_error", "Revisa la clave de idempotencia.")
	case errors.Is(err, gallery.ErrLimit):
		failure(w, 409, "gallery_limit", "El espacio ya tiene el máximo de 10 imágenes activas.")
	case errors.Is(err, gallery.ErrCandidateUnavailable):
		failure(w, 409, "gallery_operation_pending", "La operación de imagen requiere limpieza o recuperación; vuelve a intentarlo.")
	case errors.Is(err, gallery.ErrNotFound):
		failure(w, 404, "not_found", "Espacio o imagen no encontrados.")
	case errors.Is(err, gallery.ErrFileStore):
		failure(w, 503, "file_unavailable", "El archivo sintético privado no está disponible.")
	default:
		failure(w, 500, "internal_error", "Ocurrió un error inesperado.")
	}
}

func failure(w http.ResponseWriter, status int, code, message string) {
	write(w, status, map[string]any{"error": map[string]string{"code": code, "message": message, "request_id": w.Header().Get("X-Request-ID")}})
}
func write(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
