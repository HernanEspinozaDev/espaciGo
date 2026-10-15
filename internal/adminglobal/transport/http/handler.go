package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/credentials"
	"github.com/HernanEspinozaDev/espaciGo/internal/adminglobal"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
)

type CapabilityAuthenticator interface {
	AuthorizeCapability(context.Context, identity.Secret, identity.Capability, identity.ActivityKind) (identity.Principal, error)
}

type Handler struct {
	auth    CapabilityAuthenticator
	service *adminglobal.Service
	origins map[string]bool
}

func NewHandler(auth CapabilityAuthenticator, service *adminglobal.Service, origins []string) http.Handler {
	allowed := make(map[string]bool, len(origins))
	for _, origin := range origins {
		allowed[origin] = true
	}
	return &Handler{auth: auth, service: service, origins: allowed}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	requestID, err := (credentials.Generator{}).ID()
	if err != nil {
		writeError(w, 500, "internal_error", "")
		return
	}
	w.Header().Set("X-Request-ID", requestID)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
	w.Header().Add("Vary", "Origin")
	if origin := r.Header.Get("Origin"); origin != "" {
		if !h.origins[origin] {
			writeError(w, 403, "origin_denied", requestID)
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	}
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet {
		writeError(w, 405, "method_not_allowed", requestID)
		return
	}
	if h.auth == nil || h.service == nil {
		writeError(w, 404, "not_found", requestID)
		return
	}
	path := strings.TrimSuffix(r.URL.Path, "/")
	switch path {
	case "/api/v1/admin/global/me":
		principal, ok := h.authorize(w, r, identity.CapabilityAccountsList, requestID)
		if !ok {
			return
		}
		value, err := h.service.Me(r.Context(), principal)
		if err != nil {
			replyError(w, err, requestID)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": value})
	case "/api/v1/admin/global/overview":
		principal, ok := h.authorize(w, r, identity.CapabilityAccountsList, requestID)
		if !ok {
			return
		}
		value, err := h.service.Overview(r.Context(), principal.AccountID, requestID)
		if err != nil {
			replyError(w, err, requestID)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": value})
	case "/api/v1/admin/global/accounts":
		principal, ok := h.authorize(w, r, identity.CapabilityAccountsList, requestID)
		if !ok {
			return
		}
		filter, valid := parseAccountFilter(r.URL.Query())
		if !valid {
			writeError(w, 422, "invalid_request", requestID)
			return
		}
		value, err := h.service.ListAccounts(r.Context(), principal.AccountID, requestID, filter)
		if err != nil {
			replyError(w, err, requestID)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": value})
	default:
		const base = "/api/v1/admin/global/accounts/"
		if !strings.HasPrefix(path, base) || strings.Contains(strings.TrimPrefix(path, base), "/") || strings.TrimPrefix(path, base) == "" {
			writeError(w, 404, "not_found", requestID)
			return
		}
		principal, ok := h.authorize(w, r, identity.CapabilityAccountsRead, requestID)
		if !ok {
			return
		}
		value, err := h.service.LookupAccount(r.Context(), principal.AccountID, strings.TrimPrefix(path, base), requestID)
		if err != nil {
			replyError(w, err, requestID)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": value})
	}
}

func (h *Handler) authorize(w http.ResponseWriter, r *http.Request, capability identity.Capability, requestID string) (identity.Principal, bool) {
	principal, err := h.auth.AuthorizeCapability(r.Context(), identity.Secret(bearer(r.Header.Get("Authorization"))), capability, identity.UserOperation)
	if err == nil {
		return principal, true
	}
	if errors.Is(err, identity.ErrForbidden) || errors.Is(err, identity.ErrRestrictedMode) {
		writeError(w, 403, "forbidden", requestID)
		return identity.Principal{}, false
	}
	w.Header().Set("WWW-Authenticate", "Bearer")
	writeError(w, 401, "unauthenticated", requestID)
	return identity.Principal{}, false
}

func parseAccountFilter(query url.Values) (adminglobal.AccountFilter, bool) {
	allowed := map[string]bool{"account_id": true, "email_prefix": true, "name_prefix": true, "role": true, "state": true, "administrative_blocked": true, "created_from": true, "created_until": true, "page_size": true, "cursor": true}
	for key, values := range query {
		if !allowed[key] || len(values) != 1 {
			return adminglobal.AccountFilter{}, false
		}
	}
	get := func(key string) string {
		values := query[key]
		if len(values) == 0 {
			return ""
		}
		return values[0]
	}
	filter := adminglobal.AccountFilter{AccountID: get("account_id"), EmailPrefix: get("email_prefix"), NamePrefix: get("name_prefix"), Cursor: get("cursor"), PageSize: 25}
	if value := get("role"); value != "" {
		filter.Role = identity.Role(value)
	}
	if value := get("state"); value != "" {
		filter.State = identity.AccountState(value)
	}
	if value := get("administrative_blocked"); value != "" {
		blocked, err := strconv.ParseBool(value)
		if err != nil {
			return adminglobal.AccountFilter{}, false
		}
		filter.AdministrativeBlocked = &blocked
	}
	if value := get("created_from"); value != "" {
		parsed, err := time.Parse(time.RFC3339, value)
		if err != nil {
			return adminglobal.AccountFilter{}, false
		}
		filter.CreatedFrom = &parsed
	}
	if value := get("created_until"); value != "" {
		parsed, err := time.Parse(time.RFC3339, value)
		if err != nil {
			return adminglobal.AccountFilter{}, false
		}
		filter.CreatedUntil = &parsed
	}
	if value := get("page_size"); value != "" {
		pageSize, err := strconv.Atoi(value)
		if err != nil {
			return adminglobal.AccountFilter{}, false
		}
		filter.PageSize = pageSize
	}
	return filter, true
}

func bearer(value string) string {
	parts := strings.Fields(value)
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		return parts[1]
	}
	return ""
}

func replyError(w http.ResponseWriter, err error, requestID string) {
	switch {
	case errors.Is(err, adminglobal.ErrInvalid):
		writeError(w, 422, "invalid_request", requestID)
	case errors.Is(err, adminglobal.ErrForbidden):
		writeError(w, 403, "forbidden", requestID)
	case errors.Is(err, adminglobal.ErrNotFound):
		writeError(w, 404, "not_found", requestID)
	default:
		writeError(w, 500, "internal_error", requestID)
	}
}

func writeError(w http.ResponseWriter, status int, code, requestID string) {
	writeJSON(w, status, map[string]any{"error": map[string]any{"code": code, "request_id": requestID}})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
