package galleryhttp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sort"
	"strings"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/credentials"
	"github.com/HernanEspinozaDev/espaciGo/internal/gallery"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
	"github.com/HernanEspinozaDev/espaciGo/internal/media"
)

type Authenticator interface {
	Authorize(context.Context, identity.Secret, identity.Role, identity.ActivityKind) (identity.Principal, error)
}

type Handler struct {
	auth    Authenticator
	service *gallery.Service
	media   *media.Service
	origins map[string]bool
}

func NewHandlerWithMedia(auth Authenticator, service *gallery.Service, mediaService *media.Service, origins []string) http.Handler {
	m := map[string]bool{}
	for _, origin := range origins {
		m[origin] = true
	}
	return &Handler{auth: auth, service: service, media: mediaService, origins: m}
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
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
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
	case photoID == "" && r.Method == http.MethodPost && strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "multipart/form-data"):
		if h.media == nil {
			failure(w, 404, "not_found", "Recurso no encontrado.")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, media.MaxInputBytes+(1<<20))
		parseErr := r.ParseMultipartForm(1 << 20)
		// Register cleanup before handling parse errors or looking up the
		// expected field: malformed and missing-field uploads can spill files.
		defer removeMultipartTempFiles(r)
		if parseErr != nil {
			failure(w, 413, "upload_too_large", "La imagen supera el tamaño permitido.")
			return
		}
		file, _, err := r.FormFile("image")
		if err != nil {
			failure(w, 422, "validation_error", "Adjunta una imagen en el campo image.")
			return
		}
		defer file.Close()
		body, err := io.ReadAll(io.LimitReader(file, media.MaxInputBytes+1))
		if err != nil || len(body) > media.MaxInputBytes {
			failure(w, 413, "upload_too_large", "La imagen supera el tamaño permitido.")
			return
		}
		result, err := h.media.UploadSpace(r.Context(), principal.AccountID, spaceID, r.Header.Get("Idempotency-Key"), body)
		if err != nil {
			mediaError(w, err)
			return
		}
		status := http.StatusCreated
		if result.Reused {
			status = http.StatusOK
		}
		write(w, status, map[string]any{"photo": result.Photo, "reused": result.Reused})
	case photoID != "" && strings.HasPrefix(r.URL.Path, "/api/v1/spaces/") && strings.Contains(r.URL.Path, "/variants/") && r.Method == http.MethodGet:
		if h.media == nil {
			failure(w, 404, "not_found", "Recurso no encontrado.")
			return
		}
		variant := media.VariantName(strings.TrimPrefix(r.URL.Path[strings.LastIndex(r.URL.Path, "/variants/")+len("/variants/"):], "/"))
		_, v, body, err := h.media.ContentSpace(r.Context(), principal.AccountID, spaceID, photoID, variant)
		if err != nil {
			mediaError(w, err)
			return
		}
		w.Header().Set("Content-Type", "image/webp")
		w.Header().Set("Cache-Control", "private, no-store")
		w.Header().Set("Content-Disposition", "inline; filename=space-image.webp")
		w.Header().Set("X-Content-SHA256", v.SHA256)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	case photoID == "cover" && r.Method == http.MethodPut:
		var input struct {
			PhotoID string `json:"photo_id"`
		}
		decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil || strings.TrimSpace(input.PhotoID) == "" {
			failure(w, 422, "validation_error", "Indica una imagen sintética activa de la galería propia.")
			return
		}
		var extra any
		if err := decoder.Decode(&extra); err != io.EOF {
			failure(w, 422, "validation_error", "El cuerpo JSON debe contener un único objeto.")
			return
		}
		result, err := h.service.SelectCover(r.Context(), principal.AccountID, spaceID, input.PhotoID)
		if err != nil {
			if h.media == nil || !errors.Is(err, gallery.ErrNotFound) {
				galleryError(w, err)
				return
			}
			if mediaErr := h.media.SelectCover(r.Context(), principal.AccountID, spaceID, input.PhotoID); mediaErr != nil {
				mediaError(w, mediaErr)
				return
			}
			write(w, 200, map[string]any{"space_id": spaceID, "photo_id": input.PhotoID, "selected": true})
			return
		}
		write(w, 200, result)
	case photoID == "" && r.Method == http.MethodGet:
		items, err := h.service.List(r.Context(), principal.AccountID, spaceID)
		if err != nil {
			galleryError(w, err)
			return
		}
		if h.media != nil {
			real, e := h.media.ListSpace(r.Context(), principal.AccountID, spaceID)
			if e != nil {
				mediaError(w, e)
				return
			}
			type ordered struct {
				position int
				value    any
			}
			combined := make([]ordered, 0, len(items)+len(real))
			for _, item := range items {
				combined = append(combined, ordered{item.Position, item})
			}
			for _, item := range real {
				combined = append(combined, ordered{item.Position, item})
			}
			sort.Slice(combined, func(i, j int) bool { return combined[i].position < combined[j].position })
			values := make([]any, 0, len(combined))
			for _, item := range combined {
				values = append(values, item.value)
			}
			write(w, 200, map[string]any{"items": values, "max_photos": gallery.MaxPhotosPerSpace})
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
	case photoID != "" && strings.HasSuffix(r.URL.Path, "/publication") && r.Method == http.MethodPut:
		var input struct {
			PublicInCatalog *bool `json:"public_in_catalog"`
		}
		decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil || input.PublicInCatalog == nil {
			failure(w, 422, "validation_error", "Indica public_in_catalog como booleano.")
			return
		}
		var extra any
		if err := decoder.Decode(&extra); err != io.EOF {
			failure(w, 422, "validation_error", "El cuerpo JSON debe contener un único objeto.")
			return
		}
		result, err := h.service.SetPublicInCatalog(r.Context(), principal.AccountID, spaceID, photoID, *input.PublicInCatalog)
		if err != nil {
			if h.media == nil || !errors.Is(err, gallery.ErrNotFound) {
				galleryError(w, err)
				return
			}
			visible := *input.PublicInCatalog
			real, mediaErr := h.media.UpdateSpace(r.Context(), principal.AccountID, spaceID, photoID, nil, &visible, nil)
			if mediaErr != nil {
				mediaError(w, mediaErr)
				return
			}
			write(w, 200, real)
			return
		}
		write(w, 200, result)
	case photoID != "" && !content && r.Method == http.MethodPut && h.media != nil:
		var input struct {
			AltText  *string `json:"alt_text"`
			Visible  *bool   `json:"visible"`
			Position *int    `json:"position"`
		}
		decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&input) != nil || (input.AltText == nil && input.Visible == nil && input.Position == nil) {
			failure(w, 422, "validation_error", "Indica alt_text, visible o position.")
			return
		}
		var extra any
		if decoder.Decode(&extra) != io.EOF {
			failure(w, 422, "validation_error", "El cuerpo JSON debe contener un único objeto.")
			return
		}
		item, err := h.media.UpdateSpace(r.Context(), principal.AccountID, spaceID, photoID, input.AltText, input.Visible, input.Position)
		if err != nil {
			mediaError(w, err)
			return
		}
		write(w, 200, item)
	case photoID != "" && !content && r.Method == http.MethodGet:
		item, err := h.service.Metadata(r.Context(), principal.AccountID, spaceID, photoID)
		if err != nil {
			if h.media == nil || !errors.Is(err, gallery.ErrNotFound) {
				galleryError(w, err)
				return
			}
			real, mediaErr := h.media.GetSpace(r.Context(), principal.AccountID, spaceID, photoID)
			if mediaErr != nil {
				mediaError(w, mediaErr)
				return
			}
			write(w, 200, real)
			return
		}
		write(w, 200, item)
	case photoID != "" && !content && r.Method == http.MethodDelete:
		result, err := h.service.Remove(r.Context(), principal.AccountID, spaceID, photoID)
		if err != nil {
			if h.media == nil || !errors.Is(err, gallery.ErrNotFound) {
				galleryError(w, err)
				return
			}
			if mediaErr := h.media.RemoveSpace(r.Context(), principal.AccountID, spaceID, photoID); mediaErr != nil {
				mediaError(w, mediaErr)
				return
			}
			write(w, 200, map[string]any{"photo_id": photoID, "removed": true})
			return
		}
		write(w, 200, result)
	default:
		failure(w, 405, "method_not_allowed", "Método no permitido.")
	}
}

func removeMultipartTempFiles(r *http.Request) {
	if r.MultipartForm != nil {
		_ = r.MultipartForm.RemoveAll()
	}
}

func route(path string) (spaceID, photoID string, content, ok bool) {
	const prefix = "/api/v1/spaces/"
	if !strings.HasPrefix(path, prefix) {
		return "", "", false, false
	}
	parts := strings.Split(strings.Trim(strings.TrimPrefix(path, prefix), "/"), "/")
	if len(parts) == 2 && parts[0] != "" && parts[1] == "cover" {
		return parts[0], "cover", false, true
	}
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
	if len(parts) == 4 && parts[2] != "" && parts[3] == "publication" {
		return parts[0], parts[2], false, true
	}
	if len(parts) == 5 && parts[2] != "" && parts[3] == "variants" && (parts[4] == "display" || parts[4] == "thumbnail") {
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

func mediaError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, media.ErrInvalid), errors.Is(err, media.ErrNotSupported):
		failure(w, 422, "validation_error", "La imagen o solicitud no cumple los requisitos.")
	case errors.Is(err, media.ErrLimit):
		failure(w, 409, "gallery_limit", "El espacio ya tiene el máximo de 10 imágenes activas.")
	case errors.Is(err, media.ErrConflict):
		failure(w, 409, "media_conflict", "La operación de imagen está pendiente o la clave de idempotencia ya se usó.")
	case errors.Is(err, media.ErrNotFound):
		failure(w, 404, "not_found", "Espacio o imagen no encontrados.")
	default:
		failure(w, 503, "media_unavailable", "El almacenamiento privado está temporalmente no disponible.")
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
