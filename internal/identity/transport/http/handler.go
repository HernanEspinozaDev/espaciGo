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
)

type Handler struct {
	service *identity.AuthenticationService
	terms   identity.AuthenticationRepository
	origins map[string]bool
}

func NewHandler(service *identity.AuthenticationService, terms identity.AuthenticationRepository, origins []string) http.Handler {
	h := &Handler{service: service, terms: terms, origins: map[string]bool{}}
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
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.URL.RawQuery != "" {
		h.fail(w, 400, "invalid_request", "No se admiten parámetros de URL en autenticación.")
		return
	}
	expected := http.MethodPost
	if r.URL.Path == "/api/v1/auth/session" || r.URL.Path == "/api/v1/auth/terms" {
		expected = http.MethodGet
	}
	paths := map[string]bool{"/api/v1/auth/register": true, "/api/v1/auth/verification/reissue": true, "/api/v1/auth/verification": true, "/api/v1/auth/login": true, "/api/v1/auth/session": true, "/api/v1/auth/logout": true, "/api/v1/auth/terms": true, "/api/v1/auth/password/recovery": true, "/api/v1/auth/password/recovery/consume": true, "/api/v1/auth/password/change": true}
	if !paths[r.URL.Path] {
		h.fail(w, 404, "not_found", "Recurso no encontrado.")
		return
	}
	if r.Method != expected {
		w.Header().Set("Allow", expected)
		h.fail(w, 405, "method_not_allowed", "Método no permitido.")
		return
	}
	clientIP, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		h.fail(w, 400, "invalid_request", "Dirección de cliente inválida.")
		return
	}
	// RemoteAddr is trusted for the local direct topology. Never trust forwarded headers.
	switch r.URL.Path {
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
			TermsVersionIDs []string        `json:"terms_version_ids"`
		}
		if !h.decode(w, r, &input) {
			return
		}
		id, err := h.service.Register(r.Context(), identity.RegisterInput{Email: input.Email, Password: input.Password, TermsVersionIDs: input.TermsVersionIDs, Channel: "web", ClientIP: clientIP})
		if err != nil {
			if id != "" {
				h.write(w, 202, map[string]any{"account_id": id, "status": "verification_pending", "message": "Cuenta creada; solicita el reenvío de verificación cuando el servicio esté disponible."})
				return
			}
			h.serviceError(w, err)
			return
		}
		h.write(w, 201, map[string]any{"account_id": id, "status": "email_pending"})
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
func (h *Handler) fail(w http.ResponseWriter, status int, code, message string) {
	h.write(w, status, map[string]any{"error": map[string]any{"code": code, "message": message, "request_id": w.Header().Get("X-Request-ID")}})
}
func (h *Handler) write(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
