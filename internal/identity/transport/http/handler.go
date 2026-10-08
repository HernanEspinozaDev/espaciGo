package identityhttp

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/credentials"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
	"github.com/HernanEspinozaDev/espaciGo/internal/privacy"
)

type Handler struct {
	service                 *identity.AuthenticationService
	terms                   identity.AuthenticationRepository
	origins                 map[string]bool
	privacy                 *privacy.Service
	evidenceCleaner         privacy.SyntheticEvidenceCleaner
	suppressionRegistryPath string
	now                     func() time.Time
}

func NewHandler(service *identity.AuthenticationService, terms identity.AuthenticationRepository, origins []string, privacyServices ...*privacy.Service) http.Handler {
	return newHandler(service, terms, origins, nil, privacyServices...)
}

func NewHandlerWithSuppressionCleaner(service *identity.AuthenticationService, terms identity.AuthenticationRepository, origins []string, privacyService *privacy.Service, cleaner privacy.SyntheticEvidenceCleaner) http.Handler {
	return newHandler(service, terms, origins, cleaner, privacyService)
}

func NewHandlerWithSuppressionClock(service *identity.AuthenticationService, terms identity.AuthenticationRepository, origins []string, privacyService *privacy.Service, cleaner privacy.SyntheticEvidenceCleaner, now func() time.Time) http.Handler {
	h := newHandler(service, terms, origins, cleaner, privacyService).(*Handler)
	if now != nil {
		h.now = now
	}
	return h
}

// NewHandlerWithSuppressionRegistry keeps the local restore sidecar current
// whenever a suppression execution has committed successfully.
func NewHandlerWithSuppressionRegistry(service *identity.AuthenticationService, terms identity.AuthenticationRepository, origins []string, privacyService *privacy.Service, cleaner privacy.SyntheticEvidenceCleaner, registryPath string) http.Handler {
	h := newHandler(service, terms, origins, cleaner, privacyService).(*Handler)
	h.suppressionRegistryPath = registryPath
	return h
}

func NewHandlerWithSuppressionClockAndRegistry(service *identity.AuthenticationService, terms identity.AuthenticationRepository, origins []string, privacyService *privacy.Service, cleaner privacy.SyntheticEvidenceCleaner, now func() time.Time, registryPath string) http.Handler {
	h := newHandler(service, terms, origins, cleaner, privacyService).(*Handler)
	if now != nil {
		h.now = now
	}
	h.suppressionRegistryPath = registryPath
	return h
}

func newHandler(service *identity.AuthenticationService, terms identity.AuthenticationRepository, origins []string, cleaner privacy.SyntheticEvidenceCleaner, privacyServices ...*privacy.Service) http.Handler {
	h := &Handler{service: service, terms: terms, origins: map[string]bool{}, now: time.Now}
	h.evidenceCleaner = cleaner
	if len(privacyServices) > 0 {
		h.privacy = privacyServices[0]
	}
	for _, origin := range origins {
		h.origins[origin] = true
	}
	return h
}
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	requestID, err := (credentials.Generator{}).ID()
	if err != nil {
		w.WriteHeader(500)
		return
	}
	w.Header().Set("X-Request-ID", requestID)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Add("Vary", "Origin")
	origin := r.Header.Get("Origin")
	if origin != "" {
		if !h.origins[origin] {
			h.fail(w, http.StatusForbidden, "origin_denied", "Origen no permitido.")
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Expose-Headers", "X-Request-ID, Retry-After")
	}
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Idempotency-Key")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.URL.RawQuery != "" {
		h.fail(w, 400, "invalid_request", "No se admiten parámetros de URL en autenticación.")
		return
	}
	expected := http.MethodPost
	if r.URL.Path == "/api/v1/auth/session" || r.URL.Path == "/api/v1/auth/terms" || r.URL.Path == "/api/v1/profile" || r.URL.Path == "/api/v1/rights-requests" || r.URL.Path == "/api/v1/privacy/export" || r.URL.Path == "/api/v1/privacy/export/archive" {
		expected = http.MethodGet
	}
	if r.URL.Path == "/api/v1/profile" && r.Method == http.MethodPut || r.URL.Path == "/api/v1/rights-requests" && r.Method == http.MethodPost {
		expected = r.Method
	}
	const suppressionQueuePath = "/api/v1/privacy/suppression-requests"
	if r.URL.Path == suppressionQueuePath {
		expected = http.MethodGet
	}
	const reviewPrefix = "/api/v1/privacy/suppression-requests/"
	reviewSuffix := "/review"
	reviewPath := strings.HasPrefix(r.URL.Path, reviewPrefix) && strings.HasSuffix(r.URL.Path, reviewSuffix)
	executeSuffix := "/execute"
	executePath := strings.HasPrefix(r.URL.Path, reviewPrefix) && strings.HasSuffix(r.URL.Path, executeSuffix)
	if reviewPath || executePath {
		expected = http.MethodPost
	}
	paths := map[string]bool{"/api/v1/auth/register": true, "/api/v1/auth/verification/reissue": true, "/api/v1/auth/verification": true, "/api/v1/auth/login": true, "/api/v1/auth/session": true, "/api/v1/auth/logout": true, "/api/v1/auth/terms": true, "/api/v1/auth/password/recovery": true, "/api/v1/auth/password/recovery/consume": true, "/api/v1/auth/password/change": true, "/api/v1/profile": true, "/api/v1/rights-requests": true, "/api/v1/privacy/export": true, "/api/v1/privacy/export/archive": true, "/api/v1/privacy/retention/purge": true, suppressionQueuePath: true}
	if !paths[r.URL.Path] && !reviewPath && !executePath {
		h.fail(w, 404, "not_found", "Recurso no encontrado.")
		return
	}
	if r.Method != expected {
		allow := expected
		if r.URL.Path == "/api/v1/profile" {
			allow = "GET, PUT"
		}
		if r.URL.Path == "/api/v1/rights-requests" {
			allow = "GET, POST"
		}
		w.Header().Set("Allow", allow)
		h.fail(w, 405, "method_not_allowed", "Método no permitido.")
		return
	}
	clientIP, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		h.fail(w, 400, "invalid_request", "Dirección de cliente inválida.")
		return
	}
	// RemoteAddr is trusted for the local direct topology. Never trust forwarded headers.
	if reviewPath || executePath {
		suffix := reviewSuffix
		if executePath {
			suffix = executeSuffix
		}
		relativePath := strings.TrimPrefix(r.URL.Path, reviewPrefix)
		if !strings.HasSuffix(relativePath, suffix) {
			h.fail(w, http.StatusNotFound, "not_found", "Recurso no encontrado.")
			return
		}
		requestID := strings.TrimSuffix(relativePath, suffix)
		if strings.Contains(requestID, "/") || !canonicalUUID(requestID) {
			h.fail(w, http.StatusNotFound, "not_found", "Recurso no encontrado.")
			return
		}
		if h.privacy == nil {
			h.fail(w, http.StatusServiceUnavailable, "privacy_unavailable", "Servicio de privacidad no disponible.")
			return
		}
		parts := strings.Fields(r.Header.Get("Authorization"))
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			h.serviceError(w, identity.ErrUnauthorized)
			return
		}
		principal, err := h.service.Authorize(r.Context(), identity.Secret(parts[1]), identity.RoleAdministrator, identity.UserOperation)
		if err != nil {
			h.serviceError(w, err)
			return
		}
		if executePath {
			result, err := h.privacy.ExecuteSuppression(r.Context(), principal.AccountID, requestID, r.Header.Get("Idempotency-Key"), w.Header().Get("X-Request-ID"), h.now, h.evidenceCleaner)
			if err != nil {
				h.privacyError(w, err)
				return
			}
			if result.Status == "completada" && h.suppressionRegistryPath != "" {
				if err := h.privacy.ExportSuppressionReplayManifestToFile(r.Context(), h.suppressionRegistryPath); err != nil {
					h.fail(w, http.StatusServiceUnavailable, "privacy_replay_registry_unavailable", "La baja se completó, pero el registro local de recuperación requiere reintento.")
					return
				}
			}
			h.write(w, http.StatusOK, result)
		} else {
			review, err := h.privacy.ReviewSuppression(r.Context(), principal.AccountID, requestID, r.Header.Get("Idempotency-Key"), w.Header().Get("X-Request-ID"), h.now)
			if err != nil {
				h.privacyError(w, err)
				return
			}
			h.write(w, http.StatusOK, review)
		}
		return
	}
	if r.URL.Path == "/api/v1/privacy/retention/purge" {
		if r.Method != http.MethodPost {
			h.fail(w, http.StatusMethodNotAllowed, "method_not_allowed", "Método no permitido.")
			return
		}
		if h.privacy == nil {
			h.fail(w, http.StatusServiceUnavailable, "privacy_unavailable", "Servicio de privacidad no disponible.")
			return
		}
		parts := strings.Fields(r.Header.Get("Authorization"))
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			h.serviceError(w, identity.ErrUnauthorized)
			return
		}
		if _, err := h.service.Authorize(r.Context(), identity.Secret(parts[1]), identity.RoleAdministrator, identity.UserOperation); err != nil {
			h.serviceError(w, err)
			return
		}
		result, err := h.privacy.PurgeExpiredReservationLinks(r.Context(), h.now().UTC(), 100)
		if err != nil {
			h.privacyError(w, err)
			return
		}
		h.write(w, http.StatusOK, result)
		return
	}
	if r.URL.Path == suppressionQueuePath {
		if h.privacy == nil {
			h.fail(w, http.StatusServiceUnavailable, "privacy_unavailable", "Servicio de privacidad no disponible.")
			return
		}
		parts := strings.Fields(r.Header.Get("Authorization"))
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			h.serviceError(w, identity.ErrUnauthorized)
			return
		}
		if _, err := h.service.Authorize(r.Context(), identity.Secret(parts[1]), identity.RoleAdministrator, identity.UserOperation); err != nil {
			h.serviceError(w, err)
			return
		}
		items, err := h.privacy.PendingSuppressions(r.Context())
		if err != nil {
			h.privacyError(w, err)
			return
		}
		h.write(w, http.StatusOK, map[string]any{"items": items})
		return
	}
	switch r.URL.Path {
	case "/api/v1/profile", "/api/v1/rights-requests", "/api/v1/privacy/export", "/api/v1/privacy/export/archive":
		if h.privacy == nil {
			h.fail(w, http.StatusServiceUnavailable, "privacy_unavailable", "Servicio de perfil no disponible.")
			return
		}
		parts := strings.Fields(r.Header.Get("Authorization"))
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			h.serviceError(w, identity.ErrUnauthorized)
			return
		}
		principal, err := h.service.Authorize(r.Context(), identity.Secret(parts[1]), "", identity.UserOperation)
		if err != nil {
			h.serviceError(w, err)
			return
		}
		if r.URL.Path == "/api/v1/privacy/export/archive" {
			archive, err := h.privacy.ExportOwnArchive(r.Context(), principal.AccountID)
			if err != nil {
				h.privacyError(w, err)
				return
			}
			w.Header().Set("Content-Type", "application/zip")
			w.Header().Set("Content-Disposition", `attachment; filename="espacigo-datos-propios.zip"`)
			w.Header().Set("Cache-Control", "no-store")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(archive)
			return
		}
		if r.URL.Path == "/api/v1/privacy/export" {
			data, err := h.privacy.ExportOwnData(r.Context(), principal.AccountID)
			if err != nil {
				h.privacyError(w, err)
				return
			}
			h.write(w, http.StatusOK, data)
			return
		}
		if r.URL.Path == "/api/v1/profile" {
			if r.Method == http.MethodGet {
				profile, err := h.privacy.Profile(r.Context(), principal.AccountID)
				if err != nil {
					h.privacyError(w, err)
					return
				}
				h.write(w, http.StatusOK, map[string]any{"display_name": profile.Name, "phone": profile.Phone, "updated_at": profile.UpdatedAt.UTC()})
				return
			}
			var input struct {
				Name  string `json:"display_name"`
				Phone string `json:"phone"`
			}
			if !h.decode(w, r, &input) {
				return
			}
			profile, err := h.privacy.UpdateProfile(r.Context(), principal.AccountID, input.Name, input.Phone)
			if err != nil {
				h.privacyError(w, err)
				return
			}
			h.write(w, http.StatusOK, map[string]any{"display_name": profile.Name, "phone": profile.Phone, "updated_at": profile.UpdatedAt.UTC()})
			return
		}
		if r.Method == http.MethodGet {
			items, err := h.privacy.OwnRequests(r.Context(), principal.AccountID)
			if err != nil {
				h.privacyError(w, err)
				return
			}
			h.write(w, http.StatusOK, map[string]any{"items": items})
			return
		}
		var input struct {
			Kind string `json:"type"`
		}
		if !h.decode(w, r, &input) {
			return
		}
		item, err := h.privacy.RequestRight(r.Context(), principal.AccountID, input.Kind, "web")
		if err != nil {
			h.privacyError(w, err)
			return
		}
		h.write(w, http.StatusAccepted, item)
	case "/api/v1/auth/terms":
		items := []map[string]any{}
		for _, id := range []string{"00000000-0000-4000-8000-000000000001", "00000000-0000-4000-8000-000000000002"} {
			version, err := h.terms.TermsVersion(r.Context(), id)
			if err != nil {
				h.serviceError(w, err)
				return
			}
			items = append(items, map[string]any{"id": version.ID, "code": version.Code, "type": version.Type, "sha256": version.SHA256, "published_at": version.PublishedAt.UTC(), "synthetic": true})
		}
		h.write(w, 200, map[string]any{"items": items})
	case "/api/v1/auth/register":
		var input struct {
			Email           string          `json:"email"`
			Password        identity.Secret `json:"password"`
			UsePreference   string          `json:"use_preference"`
			TermsVersionIDs []string        `json:"terms_version_ids"`
		}
		if !h.decode(w, r, &input) {
			return
		}
		id, err := h.service.Register(r.Context(), identity.RegisterInput{Email: input.Email, Password: input.Password, UsePreference: input.UsePreference, TermsVersionIDs: input.TermsVersionIDs, Channel: "web", ClientIP: clientIP})
		if err != nil {
			if id != "" {
				h.write(w, 202, map[string]any{"account_id": id, "status": "verification_pending", "use_preference": input.UsePreference, "message": "Cuenta creada; solicita el reenvío de verificación cuando el servicio esté disponible."})
				return
			}
			h.serviceError(w, err)
			return
		}
		h.write(w, 201, map[string]any{"account_id": id, "status": "email_pending", "use_preference": input.UsePreference})
	case "/api/v1/auth/verification/reissue":
		var input struct {
			Email string `json:"email"`
		}
		if !h.decode(w, r, &input) {
			return
		}
		if err := h.service.ReissueVerification(r.Context(), input.Email, clientIP); err != nil {
			h.serviceError(w, err)
			return
		}
		w.WriteHeader(204)
	case "/api/v1/auth/verification":
		var input struct {
			TokenID string          `json:"token_id"`
			Token   identity.Secret `json:"token"`
		}
		if !h.decode(w, r, &input) {
			return
		}
		if err := h.service.VerifyEmail(r.Context(), input.TokenID, input.Token, clientIP); err != nil {
			h.serviceError(w, err)
			return
		}
		w.WriteHeader(204)
	case "/api/v1/auth/password/recovery":
		var input struct {
			Email string `json:"email"`
		}
		if !h.decode(w, r, &input) {
			return
		}
		if err := h.service.RequestPasswordRecovery(r.Context(), input.Email, clientIP); err != nil {
			h.serviceError(w, err)
			return
		}
		h.write(w, 202, map[string]any{"status": "accepted", "message": "Si la cuenta es elegible, recibirás instrucciones en el correo registrado."})
	case "/api/v1/auth/password/recovery/consume":
		var input struct {
			TokenID      string          `json:"token_id"`
			Token        identity.Secret `json:"token"`
			Password     identity.Secret `json:"new_password"`
			Confirmation identity.Secret `json:"confirm_password"`
		}
		if !h.decode(w, r, &input) {
			return
		}
		err := h.service.ResetPassword(r.Context(), identity.ResetPasswordInput{TokenID: input.TokenID, Token: input.Token, Password: input.Password, Confirmation: input.Confirmation, ClientIP: clientIP})
		if err != nil {
			h.serviceError(w, err, r.URL.Path)
			return
		}
		w.WriteHeader(204)
	case "/api/v1/auth/password/change":
		var input struct {
			Current      identity.Secret `json:"current_password"`
			Password     identity.Secret `json:"new_password"`
			Confirmation identity.Secret `json:"confirm_password"`
		}
		if !h.decode(w, r, &input) {
			return
		}
		parts := strings.Fields(r.Header.Get("Authorization"))
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			h.serviceError(w, identity.ErrUnauthorized)
			return
		}
		err := h.service.ChangePassword(r.Context(), identity.ChangePasswordInput{SessionToken: parts[1], CurrentPassword: input.Current, Password: input.Password, Confirmation: input.Confirmation})
		if err != nil {
			h.serviceError(w, err, r.URL.Path)
			return
		}
		w.WriteHeader(204)
	case "/api/v1/auth/login":
		var input struct {
			Email    string          `json:"email"`
			Password identity.Secret `json:"password"`
		}
		if !h.decode(w, r, &input) {
			return
		}
		result, err := h.service.Login(r.Context(), identity.LoginInput{Email: input.Email, Password: input.Password})
		if err != nil {
			h.serviceError(w, err)
			return
		}
		// Explicit transport serialization; Secret remains redacted in all other formatting.
		h.write(w, 200, map[string]any{"account_id": result.AccountID, "roles": result.Roles, "access_token": string(result.Token), "token_type": "Bearer", "expires_at": result.ExpiresAt.UTC()})
	case "/api/v1/auth/session", "/api/v1/auth/logout":
		parts := strings.Fields(r.Header.Get("Authorization"))
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			h.serviceError(w, identity.ErrUnauthorized)
			return
		}
		token := identity.Secret(parts[1])
		principal, err := h.service.Authorize(r.Context(), token, "", identity.AutomaticPolling)
		if err != nil {
			h.serviceError(w, err)
			return
		}
		if r.URL.Path == "/api/v1/auth/session" {
			h.write(w, 200, map[string]any{"account_id": principal.AccountID, "roles": principal.Roles})
			return
		}
		if err := h.service.Logout(r.Context(), token); err != nil {
			h.serviceError(w, err)
			return
		}
		w.WriteHeader(204)
	}
}
func (h *Handler) decode(w http.ResponseWriter, r *http.Request, target any) bool {
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		h.fail(w, 415, "unsupported_media_type", "Usa application/json.")
		return false
	}
	reader := http.MaxBytesReader(w, r.Body, 16<<10)
	defer reader.Close()
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		h.fail(w, 400, "invalid_request", "JSON inválido o campos no admitidos.")
		return false
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		h.fail(w, 400, "invalid_request", "Solo se admite un objeto JSON.")
		return false
	}
	return true
}
func (h *Handler) serviceError(w http.ResponseWriter, err error, paths ...string) {
	requestPath := ""
	if len(paths) > 0 {
		requestPath = paths[0]
	}
	status, code, message := 500, "internal_error", "Ocurrió un error inesperado."
	var blocked *identity.LoginBlockedError
	switch {
	case errors.As(err, &blocked):
		status = 429
		code = "login_blocked"
		message = "Login bloqueado temporalmente."
		seconds := int(time.Until(blocked.Until).Seconds()) + 1
		if seconds < 1 {
			seconds = 1
		}
		w.Header().Set("Retry-After", strconv.Itoa(seconds))
	case errors.Is(err, identity.ErrEmailRegistered):
		status = 409
		code = "email_registered"
		message = identity.ErrEmailRegistered.Error()
	case errors.Is(err, identity.ErrCredentials), errors.Is(err, identity.ErrUnauthorized):
		status = 401
		code = "unauthenticated"
		message = "Credencial ausente, incorrecta o expirada."
		w.Header().Set("WWW-Authenticate", "Bearer")
	case errors.Is(err, identity.ErrEmailUnverified):
		status = 403
		code = "email_unverified"
		message = "Confirma tu correo antes de iniciar sesión."
	case errors.Is(err, identity.ErrAccountDisabled), errors.Is(err, identity.ErrForbidden):
		status = 403
		code = "forbidden"
		message = "La cuenta no permite esta operación."
	case errors.Is(err, identity.ErrTokenInvalid):
		status = 422
		code = "invalid_token"
		message = "Token inválido, expirado o ya utilizado."
	case errors.Is(err, identity.ErrCurrentPassword):
		status = 422
		code = "current_password_invalid"
		message = identity.ErrCurrentPassword.Error()
	case errors.Is(err, identity.ErrPasswordSame):
		status = 422
		code = "password_unchanged"
		message = identity.ErrPasswordSame.Error()
	case errors.Is(err, identity.ErrPasswordRecentlyUsed):
		status = 422
		code = "password_recently_used"
		message = identity.ErrPasswordRecentlyUsed.Error()
	case errors.Is(err, identity.ErrPasswordConfirm):
		status = 422
		code = "password_confirmation_mismatch"
		message = identity.ErrPasswordConfirm.Error()
	case errors.Is(err, identity.ErrRateLimited):
		status = 429
		code = "rate_limited"
		message = "Límite de verificación alcanzado; intenta más tarde."
		w.Header().Set("Retry-After", "3600")
	case errors.Is(err, identity.ErrInvalid):
		status = 422
		code = "validation_error"
		message = "Revisa correo, contraseña y aceptación de términos."
	case errors.Is(err, identity.ErrNotFound):
		status = 404
		code = "not_found"
		message = "Recurso no encontrado."
	case errors.Is(err, identity.ErrDelivery):
		status = 503
		code = "mail_unavailable"
		message = "Correo de desarrollo no disponible; solicita reenvío."
		if requestPath == "/api/v1/auth/password/change" || requestPath == "/api/v1/auth/password/recovery/consume" {
			message = "La contraseña puede haberse actualizado y las sesiones revocado; revisa Mailpit e inicia sesión con la nueva contraseña."
		}
	}
	h.fail(w, status, code, message)
}

func (h *Handler) privacyError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, identity.ErrConflict):
		h.fail(w, http.StatusConflict, "conflict", "La solicitud de supresión cambió o la clave idempotente pertenece a otra revisión.")
	case errors.Is(err, privacy.ErrInvalid):
		h.fail(w, http.StatusUnprocessableEntity, "validation_error", "Revisa los datos enviados para esta operación de privacidad.")
	case errors.Is(err, privacy.ErrNotFound):
		h.fail(w, http.StatusNotFound, "not_found", "Recurso no encontrado.")
	default:
		h.fail(w, http.StatusInternalServerError, "internal_error", "No se pudo completar la operación.")
	}
}

func canonicalUUID(value string) bool {
	if len(value) != 36 {
		return false
	}
	for i, r := range value {
		switch i {
		case 8, 13, 18, 23:
			if r != '-' {
				return false
			}
		default:
			if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f' || r >= 'A' && r <= 'F') {
				return false
			}
		}
	}
	return true
}
func (h *Handler) fail(w http.ResponseWriter, status int, code, message string) {
	h.write(w, status, map[string]any{"error": map[string]any{"code": code, "message": message, "request_id": w.Header().Get("X-Request-ID")}})
}
func (h *Handler) write(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
