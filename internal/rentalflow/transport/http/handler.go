package rentalflowhttp

import (
	"bytes"
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
	"github.com/HernanEspinozaDev/espaciGo/internal/rentalflow"
)

type Authenticator interface {
	Authorize(context.Context, identity.Secret, identity.Role, identity.ActivityKind) (identity.Principal, error)
}

type AvailabilityService interface {
	ListPublic(context.Context, string, string, int) (rentalflow.Availability, error)
}

type QuoteService interface {
	Create(context.Context, string, rentalflow.QuoteRequest) (rentalflow.Quote, error)
}

type PreviewService interface {
	Preview(context.Context, string, string) (rentalflow.ContractPreview, error)
}

type Handler struct {
	auth         Authenticator
	availability AvailabilityService
	quotes       QuoteService
	previews     PreviewService
	origins      map[string]struct{}
	mux          *http.ServeMux
}

// New mounts the rental-flow HTTP contract without binding it to a storage
// adapter. Composition is deliberately left to the application entry point.
func New(auth Authenticator, availability AvailabilityService, quotes QuoteService, previews PreviewService, allowedOrigins []string) http.Handler {
	h := &Handler{auth: auth, availability: availability, quotes: quotes, previews: previews, origins: make(map[string]struct{}, len(allowedOrigins)), mux: http.NewServeMux()}
	for _, origin := range allowedOrigins {
		if origin != "" {
			h.origins[origin] = struct{}{}
		}
	}
	h.mux.HandleFunc("GET /api/v1/spaces/{spaceID}/rental-availability", h.listAvailability)
	h.mux.HandleFunc("POST /api/v1/rental-quotes", h.createQuote)
	h.mux.HandleFunc("POST /api/v1/rental-quotes/{quoteID}/contract-preview", h.previewContract)
	h.mux.HandleFunc("OPTIONS /api/v1/spaces/{spaceID}/rental-availability", func(w http.ResponseWriter, r *http.Request) {
		h.options(w, r, http.MethodGet)
	})
	h.mux.HandleFunc("OPTIONS /api/v1/rental-quotes", func(w http.ResponseWriter, r *http.Request) {
		h.options(w, r, http.MethodPost)
	})
	h.mux.HandleFunc("OPTIONS /api/v1/rental-quotes/{quoteID}/contract-preview", func(w http.ResponseWriter, r *http.Request) {
		h.options(w, r, http.MethodPost)
	})
	h.mux.HandleFunc("HEAD /api/v1/spaces/{spaceID}/rental-availability", func(w http.ResponseWriter, _ *http.Request) {
		methodNotAllowed(w, "GET, OPTIONS")
	})
	h.mux.HandleFunc("/api/v1/spaces/{spaceID}/rental-availability", func(w http.ResponseWriter, _ *http.Request) {
		methodNotAllowed(w, "GET, OPTIONS")
	})
	h.mux.HandleFunc("/api/v1/rental-quotes", func(w http.ResponseWriter, _ *http.Request) {
		methodNotAllowed(w, "POST, OPTIONS")
	})
	h.mux.HandleFunc("/api/v1/rental-quotes/{quoteID}/contract-preview", func(w http.ResponseWriter, _ *http.Request) {
		methodNotAllowed(w, "POST, OPTIONS")
	})
	h.mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusNotFound, "not_found")
	})
	return h
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Add("Vary", "Origin")
	requestID, err := (credentials.Generator{}).ID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	w.Header().Set("X-Request-ID", requestID)
	origins := r.Header.Values("Origin")
	if len(origins) > 1 {
		writeError(w, http.StatusForbidden, "origin_denied")
		return
	}
	if len(origins) == 1 && origins[0] != "" {
		origin := origins[0]
		if _, ok := h.origins[origin]; !ok {
			writeError(w, http.StatusForbidden, "origin_denied")
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
	}
	h.mux.ServeHTTP(w, r)
}

func (h *Handler) options(w http.ResponseWriter, r *http.Request, allowedMethod string) {
	methods := r.Header.Values("Access-Control-Request-Method")
	requestedHeaders := strings.Join(r.Header.Values("Access-Control-Request-Headers"), ",")
	if r.Header.Get("Origin") == "" || len(methods) != 1 || methods[0] != allowedMethod || !allowedPreflightHeaders(requestedHeaders) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	w.Header().Set("Access-Control-Allow-Methods", allowedMethod)
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	w.WriteHeader(http.StatusNoContent)
}

func allowedPreflightHeaders(value string) bool {
	if strings.TrimSpace(value) == "" {
		return true
	}
	for _, requested := range strings.Split(value, ",") {
		switch {
		case strings.EqualFold(strings.TrimSpace(requested), "Content-Type"),
			strings.EqualFold(strings.TrimSpace(requested), "Authorization"):
		default:
			return false
		}
	}
	return true
}

func methodNotAllowed(w http.ResponseWriter, allowedMethods string) {
	w.Header().Set("Allow", allowedMethods)
	writeError(w, http.StatusMethodNotAllowed, "method_not_allowed")
}

func (h *Handler) listAvailability(w http.ResponseWriter, r *http.Request) {
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil || !onlyKeys(query, "date", "duration") || len(query["date"]) != 1 || len(query["duration"]) != 1 {
		writeError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	duration, err := strconv.Atoi(query.Get("duration"))
	if err != nil || duration < 1 || duration > 4 {
		writeError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	spaceID, date := r.PathValue("spaceID"), query.Get("date")
	if !rentalflow.ValidUUID(spaceID) || !rentalflow.ValidDate(date) {
		writeError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	if h.availability == nil {
		writeError(w, http.StatusServiceUnavailable, "service_unavailable")
		return
	}
	result, err := h.availability.ListPublic(r.Context(), spaceID, date, duration)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": result})
}

func (h *Handler) createQuote(w http.ResponseWriter, r *http.Request) {
	if r.URL.RawQuery != "" {
		writeError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	principal, ok := h.authorize(w, r)
	if !ok {
		return
	}
	var input rentalflow.QuoteRequest
	if !decodeJSON(w, r, &input) {
		return
	}
	if !rentalflow.ValidUUID(input.SpaceID) || !rentalflow.ValidTimestamp(input.StartAt) || !rentalflow.ValidTimestamp(input.EndAt) {
		writeError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	if h.quotes == nil {
		writeError(w, http.StatusServiceUnavailable, "service_unavailable")
		return
	}
	result, err := h.quotes.Create(r.Context(), principal.AccountID, input)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"data": result})
}

func (h *Handler) previewContract(w http.ResponseWriter, r *http.Request) {
	if r.URL.RawQuery != "" || requestBodyPresent(r) {
		writeError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	principal, ok := h.authorize(w, r)
	if !ok {
		return
	}
	quoteID := r.PathValue("quoteID")
	if !rentalflow.ValidUUID(quoteID) {
		writeError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	if h.previews == nil {
		writeError(w, http.StatusServiceUnavailable, "service_unavailable")
		return
	}
	result, err := h.previews.Preview(r.Context(), principal.AccountID, quoteID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": result})
}

func requestBodyPresent(r *http.Request) bool {
	if r.Body == nil {
		return false
	}
	var probe [1]byte
	n, err := io.ReadFull(r.Body, probe[:])
	return n > 0 || (err != nil && err != io.EOF && err != io.ErrUnexpectedEOF)
}

func (h *Handler) authorize(w http.ResponseWriter, r *http.Request) (identity.Principal, bool) {
	if h.auth == nil {
		writeError(w, http.StatusServiceUnavailable, "service_unavailable")
		return identity.Principal{}, false
	}
	if len(r.Header.Values("Authorization")) > 1 {
		w.Header().Set("WWW-Authenticate", "Bearer")
		writeError(w, http.StatusUnauthorized, "unauthenticated")
		return identity.Principal{}, false
	}
	principal, err := h.auth.Authorize(r.Context(), bearer(r.Header.Get("Authorization")), "", identity.UserOperation)
	if err == nil && rentalflow.ValidUUID(principal.AccountID) {
		return principal, true
	}
	if errors.Is(err, identity.ErrForbidden) || errors.Is(err, identity.ErrRestrictedMode) {
		writeError(w, http.StatusForbidden, "forbidden")
		return identity.Principal{}, false
	}
	if err != nil && !errors.Is(err, identity.ErrUnauthorized) {
		writeError(w, http.StatusServiceUnavailable, "service_unavailable")
		return identity.Principal{}, false
	}
	w.Header().Set("WWW-Authenticate", "Bearer")
	writeError(w, http.StatusUnauthorized, "unauthenticated")
	return identity.Principal{}, false
}

func decodeJSON(w http.ResponseWriter, r *http.Request, destination any) bool {
	contentTypes := r.Header.Values("Content-Type")
	if len(contentTypes) != 1 {
		writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type")
		return false
	}
	mediaType, _, err := mime.ParseMediaType(contentTypes[0])
	if err != nil || mediaType != "application/json" {
		writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type")
		return false
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 16*1024))
	if err != nil || rejectDuplicateJSONKeys(body) != nil {
		writeError(w, http.StatusBadRequest, "invalid_request")
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request")
		return false
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		writeError(w, http.StatusBadRequest, "invalid_request")
		return false
	}
	return true
}

func rejectDuplicateJSONKeys(body []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(body))
	root, err := decoder.Token()
	if err != nil {
		return err
	}
	if delimiter, ok := root.(json.Delim); !ok || delimiter != '{' {
		return nil
	}
	seen := make(map[string]struct{})
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return err
		}
		key, ok := keyToken.(string)
		if !ok {
			return errors.New("invalid JSON object key")
		}
		for _, character := range key {
			if character > 0x7f {
				return errors.New("non-ASCII JSON object key")
			}
		}
		normalizedKey := strings.ToLower(key)
		if _, duplicate := seen[normalizedKey]; duplicate {
			return errors.New("duplicate JSON object key")
		}
		seen[normalizedKey] = struct{}{}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return err
		}
	}
	end, err := decoder.Token()
	if err != nil {
		return err
	}
	if end != json.Delim('}') {
		return errors.New("invalid JSON object")
	}
	if _, err := decoder.Token(); err != io.EOF {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}

func onlyKeys(query map[string][]string, allowed ...string) bool {
	for key := range query {
		found := false
		for _, candidate := range allowed {
			if key == candidate {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func bearer(value string) identity.Secret {
	parts := strings.SplitN(value, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return identity.Secret(strings.TrimSpace(parts[1]))
}

func writeDomainError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, rentalflow.ErrInvalid):
		writeError(w, http.StatusUnprocessableEntity, "invalid_request")
	case errors.Is(err, rentalflow.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found")
	case errors.Is(err, rentalflow.ErrConflict), errors.Is(err, rentalflow.ErrIdentityRequired):
		writeError(w, http.StatusConflict, "conflict")
	case errors.Is(err, rentalflow.ErrUnavailable), errors.Is(err, rentalflow.ErrIdentityUnavailable):
		writeError(w, http.StatusServiceUnavailable, "service_unavailable")
	default:
		writeError(w, http.StatusInternalServerError, "internal_error")
	}
}

func writeError(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{
		"code": code, "message": "No fue posible completar la solicitud.", "request_id": w.Header().Get("X-Request-ID"),
	}})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
