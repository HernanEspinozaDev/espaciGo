package verificationhttp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/credentials"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
	"github.com/HernanEspinozaDev/espaciGo/internal/verification"
)

type Authenticator interface {
	Authorize(context.Context, identity.Secret, identity.Role, identity.ActivityKind) (identity.Principal, error)
}
type Handler struct {
	auth     Authenticator
	service  *verification.Service
	evidence *verification.EvidenceService
	origins  map[string]bool
}

func NewHandler(auth Authenticator, service *verification.Service, origins []string, evidence ...*verification.EvidenceService) http.Handler {
	h := &Handler{auth: auth, service: service, origins: map[string]bool{}}
	if len(evidence) > 0 {
		h.evidence = evidence[0]
	}
	for _, o := range origins {
		h.origins[o] = true
	}
	return h
}

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-8][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	requestID, idErr := (credentials.Generator{}).ID()
	if idErr != nil {
		writeError(w, 500, "internal_error", "Ocurrió un error inesperado.")
		return
	}
	w.Header().Set("X-Request-ID", requestID)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Add("Vary", "Origin")
	if origin := r.Header.Get("Origin"); origin != "" {
		if !h.origins[origin] {
			writeError(w, 403, "origin_denied", "Origen no permitido.")
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Idempotency-Key")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
	}
	if r.Method == http.MethodOptions {
		w.WriteHeader(204)
		return
	}
	if r.URL.RawQuery != "" {
		writeError(w, 400, "invalid_request", "No se admiten parámetros de URL.")
		return
	}
	path := strings.TrimSuffix(r.URL.Path, "/")
	if path == "" {
		path = "/"
	}
	admin := strings.HasPrefix(path, "/api/v1/admin/verifications")
	principal, err := h.authorize(r, admin)
	if err != nil {
		if errors.Is(err, identity.ErrForbidden) {
			writeError(w, 403, "forbidden", "Permiso insuficiente.")
		} else {
			w.Header().Set("WWW-Authenticate", "Bearer")
			writeError(w, 401, "unauthenticated", "Credencial ausente, incorrecta o expirada.")
		}
		return
	}
	if h.evidence != nil {
		if handled := h.serveEvidence(w, r, path, principal, admin); handled {
			return
		}
	}
	if path == "/api/v1/verifications" && r.Method == http.MethodGet {
		items, e := h.service.ListOwn(r.Context(), principal.AccountID)
		if e != nil {
			writeError(w, 500, "internal_error", "Ocurrió un error inesperado.")
			return
		}
		writeJSON(w, 200, map[string]any{"items": items})
		return
	}
	if path == "/api/v1/verifications/eligibility" && r.Method == http.MethodGet {
		items, e := h.service.Eligibility(r.Context(), principal.AccountID)
		if e != nil {
			serviceError(w, e)
			return
		}
		writeJSON(w, 200, map[string]any{"items": items, "scope": "synthetic_local"})
		return
	}
	if path == "/api/v1/verifications" && r.Method == http.MethodPost {
		var in struct {
			Type string `json:"type"`
		}
		if !decode(w, r, &in) {
			return
		}
		item, e := h.service.Start(r.Context(), principal.AccountID, in.Type, r.Header.Get("Idempotency-Key"))
		if e != nil {
			serviceError(w, e)
			return
		}
		writeJSON(w, 201, item)
		return
	}
	if path == "/api/v1/admin/verifications" && admin && r.Method == http.MethodGet {
		items, e := h.service.Pending(r.Context())
		if e != nil {
			writeError(w, 500, "internal_error", "Ocurrió un error inesperado.")
			return
		}
		writeJSON(w, 200, map[string]any{"items": items})
		return
	}
	if path == "/api/v1/admin/verifications/failed" && admin && r.Method == http.MethodGet {
		items, e := h.service.Rejected(r.Context())
		if e != nil {
			writeError(w, 500, "internal_error", "Ocurrió un error inesperado.")
			return
		}
		writeJSON(w, 200, map[string]any{"items": items})
		return
	}
	if strings.HasPrefix(path, "/api/v1/verifications/") {
		tail := strings.TrimPrefix(path, "/api/v1/verifications/")
		parts := strings.Split(tail, "/")
		if len(parts) == 1 && uuidPattern.MatchString(parts[0]) && r.Method == http.MethodGet {
			item, e := h.service.Own(r.Context(), principal.AccountID, parts[0])
			if e != nil {
				serviceError(w, e)
				return
			}
			writeJSON(w, 200, item)
			return
		}
		if len(parts) == 2 && parts[1] == "history" && uuidPattern.MatchString(parts[0]) && r.Method == http.MethodGet {
			items, e := h.service.History(r.Context(), principal.AccountID, parts[0])
			if e != nil {
				serviceError(w, e)
				return
			}
			writeJSON(w, 200, map[string]any{"items": items})
			return
		}
		if len(parts) == 2 && parts[1] == "retry" && uuidPattern.MatchString(parts[0]) && r.Method == http.MethodPost {
			var in struct {
				CorrectionCode string `json:"correction_code"`
			}
			if !decode(w, r, &in) {
				return
			}
			item, e := h.service.Retry(r.Context(), principal.AccountID, parts[0], r.Header.Get("Idempotency-Key"), in.CorrectionCode)
			if e != nil {
				serviceError(w, e)
				return
			}
			writeJSON(w, 201, item)
			return
		}
	}
	if strings.HasPrefix(path, "/api/v1/admin/verifications/") {
		tail := strings.TrimPrefix(path, "/api/v1/admin/verifications/")
		parts := strings.Split(tail, "/")
		if admin && len(parts) == 2 && parts[1] == "review" && uuidPattern.MatchString(parts[0]) && r.Method == http.MethodPost {
			var in struct {
				Decision string `json:"decision"`
				Reason   string `json:"reason_code"`
			}
			if !decode(w, r, &in) {
				return
			}
			item, e := h.service.Review(r.Context(), parts[0], principal.AccountID, in.Decision, in.Reason)
			if e != nil {
				serviceError(w, e)
				return
			}
			writeJSON(w, 200, item)
			return
		}
		if admin && len(parts) == 2 && parts[1] == "revoke" && uuidPattern.MatchString(parts[0]) && r.Method == http.MethodPost {
			var in struct {
				Reason string `json:"reason_code"`
			}
			if !decode(w, r, &in) {
				return
			}
			item, e := h.service.Revoke(r.Context(), parts[0], principal.AccountID, in.Reason, r.Header.Get("Idempotency-Key"), requestID)
			if e != nil {
				serviceError(w, e)
				return
			}
			writeJSON(w, 200, item)
			return
		}
	}
	writeError(w, 404, "not_found", "Recurso no encontrado.")
}

func (h *Handler) serveEvidence(w http.ResponseWriter, r *http.Request, path string, principal identity.Principal, admin bool) bool {
	const ownPrefix = "/api/v1/verifications/"
	const adminPrefix = "/api/v1/admin/verifications/"
	if strings.HasPrefix(path, ownPrefix) {
		tail := strings.TrimPrefix(path, ownPrefix)
		parts := strings.Split(tail, "/")
		if len(parts) < 2 || !uuidPattern.MatchString(parts[0]) || parts[1] != "evidence" {
			return false
		}
		caseID := parts[0]
		if len(parts) == 2 {
			switch r.Method {
			case http.MethodGet:
				items, err := h.evidence.ListOwn(r.Context(), principal.AccountID, caseID)
				if err != nil {
					serviceError(w, err)
					return true
				}
				writeJSON(w, http.StatusOK, map[string]any{"items": items})
				return true
			case http.MethodPost:
				var in struct {
					FixtureCode string `json:"fixture_code"`
				}
				if !decode(w, r, &in) {
					return true
				}
				if in.FixtureCode != verification.SyntheticEvidenceFixture {
					serviceError(w, verification.ErrInvalid)
					return true
				}
				item, err := h.evidence.Upload(r.Context(), principal.AccountID, caseID)
				if err != nil {
					serviceError(w, err)
					return true
				}
				writeJSON(w, http.StatusCreated, item)
				return true
			}
		}
		if len(parts) == 3 && uuidPattern.MatchString(parts[2]) {
			switch r.Method {
			case http.MethodGet:
				item, blob, err := h.evidence.OwnContent(r.Context(), principal.AccountID, caseID, parts[2])
				if err != nil {
					serviceError(w, err)
					return true
				}
				writeEvidenceContent(w, item, blob)
				return true
			case http.MethodDelete:
				if err := h.evidence.DeleteOwn(r.Context(), principal.AccountID, caseID, parts[2]); err != nil {
					serviceError(w, err)
					return true
				}
				w.WriteHeader(http.StatusNoContent)
				return true
			}
		}
	}
	if admin && strings.HasPrefix(path, adminPrefix) {
		tail := strings.TrimPrefix(path, adminPrefix)
		parts := strings.Split(tail, "/")
		if len(parts) == 2 && uuidPattern.MatchString(parts[0]) && parts[1] == "evidence" && r.Method == http.MethodGet {
			items, err := h.evidence.ListReview(r.Context(), parts[0])
			if err != nil {
				serviceError(w, err)
				return true
			}
			writeJSON(w, http.StatusOK, map[string]any{"items": items})
			return true
		}
		if len(parts) == 3 && uuidPattern.MatchString(parts[0]) && parts[1] == "evidence" && uuidPattern.MatchString(parts[2]) {
			switch r.Method {
			case http.MethodGet:
				item, blob, err := h.evidence.ReviewContent(r.Context(), parts[0], parts[2])
				if err != nil {
					serviceError(w, err)
					return true
				}
				writeEvidenceContent(w, item, blob)
				return true
			case http.MethodDelete:
				if err := h.evidence.DeleteReview(r.Context(), parts[0], parts[2]); err != nil {
					serviceError(w, err)
					return true
				}
				w.WriteHeader(http.StatusNoContent)
				return true
			}
		}
	}
	return false
}

func writeEvidenceContent(w http.ResponseWriter, item verification.Evidence, blob []byte) {
	w.Header().Set("Content-Type", item.MIMEType)
	w.Header().Set("Content-Length", strconv.FormatInt(int64(len(blob)), 10))
	w.Header().Set("Content-Disposition", `inline; filename="synthetic-evidence.png"`)
	w.Header().Set("X-Evidence-ID", item.ID)
	w.Header().Set("ETag", `"`+item.SHA256+`"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(blob)
}
func (h *Handler) authorize(r *http.Request, admin bool) (identity.Principal, error) {
	parts := strings.Fields(r.Header.Get("Authorization"))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return identity.Principal{}, identity.ErrUnauthorized
	}
	role := identity.Role("")
	if admin {
		role = identity.Role("administrador")
	}
	return h.auth.Authorize(r.Context(), identity.Secret(parts[1]), role, identity.UserOperation)
}
func decode(w http.ResponseWriter, r *http.Request, out any) bool {
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		writeError(w, 415, "unsupported_media_type", "Usa application/json.")
		return false
	}
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	d.DisallowUnknownFields()
	if d.Decode(out) != nil {
		writeError(w, 400, "invalid_request", "JSON inválido.")
		return false
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		writeError(w, 400, "invalid_request", "Solo se admite un objeto JSON.")
		return false
	}
	return true
}
func serviceError(w http.ResponseWriter, e error) {
	switch {
	case errors.Is(e, verification.ErrInvalid):
		writeError(w, 422, "invalid_input", "Los datos de la solicitud no son válidos.")
	case errors.Is(e, verification.ErrNotFound):
		writeError(w, 404, "not_found", "Verificación no encontrada.")
	case errors.Is(e, verification.ErrConflict):
		writeError(w, 409, "state_conflict", "La verificación ya cambió de estado o la solicitud ya fue procesada.")
	default:
		writeError(w, 500, "internal_error", "Ocurrió un error inesperado.")
	}
}
func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message, "request_id": w.Header().Get("X-Request-ID")}})
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
