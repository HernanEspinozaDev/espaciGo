package http

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/credentials"
	"github.com/HernanEspinozaDev/espaciGo/internal/adminglobal"
	"github.com/HernanEspinozaDev/espaciGo/internal/booking"
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
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Idempotency-Key")
		w.Header().Set("Access-Control-Allow-Methods", "GET, PATCH, PUT, POST, OPTIONS")
	}
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if h.auth == nil || h.service == nil {
		writeError(w, 404, "not_found", requestID)
		return
	}
	path := strings.TrimSuffix(r.URL.Path, "/")
	if h.serveAccountMutation(w, r, path, requestID) {
		return
	}
	if h.servePublicationMutation(w, r, path, requestID) {
		return
	}
	if h.servePublicationSchedule(w, r, path, requestID) {
		return
	}
	if h.servePublicationGallery(w, r, path, requestID) {
		return
	}
	if h.serveAccountResources(w, r, path, requestID) {
		return
	}
	if r.Method != http.MethodGet {
		writeError(w, 405, "method_not_allowed", requestID)
		return
	}
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
	case "/api/v1/admin/global/kpis":
		principal, ok := h.authorize(w, r, identity.CapabilityKPIRead, requestID)
		if !ok {
			return
		}
		period, valid := parseReportPeriod(r.URL.Query())
		if !valid {
			writeError(w, 422, "invalid_request", requestID)
			return
		}
		value, err := h.service.KPIs(r.Context(), principal.AccountID, requestID, period)
		if err != nil {
			replyError(w, err, requestID)
			return
		}
		writeJSON(w, 200, map[string]any{"data": value})
	case "/api/v1/admin/global/audit-events", "/api/v1/admin/global/audit-events/export":
		export := strings.HasSuffix(path, "/export")
		cap := identity.CapabilityAuditRead
		if export {
			cap = identity.CapabilityAuditExport
		}
		principal, ok := h.authorize(w, r, cap, requestID)
		if !ok {
			return
		}
		filter, valid := parseAuditFilter(r.URL.Query())
		if !valid {
			writeError(w, 422, "invalid_request", requestID)
			return
		}
		value, err := h.service.Audit(r.Context(), principal.AccountID, requestID, filter, export)
		if err != nil {
			replyError(w, err, requestID)
			return
		}
		if export {
			w.Header().Set("Content-Disposition", `attachment; filename="gadm-audit-`+requestID+`.json"`)
			if strings.EqualFold(r.URL.Query().Get("format"), "csv") {
				w.Header().Set("Content-Type", "text/csv; charset=utf-8")
				w.Header().Set("Content-Disposition", `attachment; filename="gadm-audit-`+requestID+`.csv"`)
				if err = adminglobal.WriteAuditCSV(w, value); err != nil {
					return
				}
				return
			}
			writeJSON(w, 200, map[string]any{"data": value})
		} else {
			writeJSON(w, 200, map[string]any{"data": value})
		}
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
	case "/api/v1/admin/global/publications":
		principal, ok := h.authorize(w, r, identity.CapabilityPublicationsList, requestID)
		if !ok {
			return
		}
		filter, valid := parsePublicationFilter(r.URL.Query())
		if !valid {
			writeError(w, 422, "invalid_request", requestID)
			return
		}
		value, err := h.service.ListPublications(r.Context(), principal.AccountID, requestID, filter)
		if err != nil {
			replyError(w, err, requestID)
			return
		}
		writeJSON(w, 200, map[string]any{"data": value})
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

func (h *Handler) servePublicationGallery(w http.ResponseWriter, r *http.Request, path, requestID string) bool {
	const prefix = "/api/v1/admin/global/publications/"
	if !strings.HasPrefix(path, prefix) {
		return false
	}
	parts := strings.Split(strings.TrimPrefix(path, prefix), "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] != "gallery" {
		return false
	}
	if len(parts) == 2 && r.Method == http.MethodGet {
		principal, ok := h.authorize(w, r, identity.CapabilityPublicationsRead, requestID)
		if !ok {
			return true
		}
		owner := r.URL.Query().Get("owner_id")
		value, err := h.service.PublicationGallery(r.Context(), principal.AccountID, parts[0], owner, requestID)
		if err != nil {
			replyError(w, err, requestID)
			return true
		}
		writeJSON(w, 200, map[string]any{"data": value})
		return true
	}
	if len(parts) != 4 || parts[2] == "" {
		return false
	}
	action := parts[3]
	methodOK := (action == "visibility" && r.Method == http.MethodPatch) || ((action == "cover" || action == "remove") && r.Method == http.MethodPost)
	if !methodOK {
		return false
	}
	capability := identity.CapabilityPublicationsUpdate
	if action == "remove" {
		capability = identity.CapabilityPublicationsRetire
	}
	principal, ok := h.authorize(w, r, capability, requestID)
	if !ok {
		return true
	}
	if !recentlyAuthenticated(principal.AuthenticatedAt, time.Now().UTC()) {
		writeError(w, 428, "recent_authentication_required", requestID)
		return true
	}
	var input struct {
		OwnerID           string    `json:"owner_id"`
		Enabled           bool      `json:"enabled"`
		ExpectedUpdatedAt time.Time `json:"expected_updated_at"`
		Reason            string    `json:"reason"`
	}
	if !decodeManagementBody(w, r, &input) {
		writeError(w, 422, "invalid_request", requestID)
		return true
	}
	var value any
	var err error
	switch action {
	case "visibility":
		value, err = h.service.SetPublicationPhotoVisibility(r.Context(), principal.AccountID, parts[0], input.OwnerID, parts[2], requestID, input.Enabled, input.ExpectedUpdatedAt, input.Reason)
	case "cover":
		value, err = h.service.SelectPublicationCover(r.Context(), principal.AccountID, parts[0], input.OwnerID, parts[2], requestID, input.ExpectedUpdatedAt, input.Reason)
	case "remove":
		value, err = h.service.RemovePublicationPhoto(r.Context(), principal.AccountID, parts[0], input.OwnerID, parts[2], requestID, input.ExpectedUpdatedAt, input.Reason)
	}
	if err != nil {
		replyError(w, err, requestID)
		return true
	}
	writeJSON(w, 200, map[string]any{"data": value})
	return true
}

func (h *Handler) serveAccountResources(w http.ResponseWriter, r *http.Request, path, requestID string) bool {
	const prefix = "/api/v1/admin/global/accounts/"
	if !strings.HasPrefix(path, prefix) || r.Method != http.MethodGet {
		return false
	}
	parts := strings.Split(strings.TrimPrefix(path, prefix), "/")
	if len(parts) != 3 && len(parts) != 4 {
		return false
	}
	if parts[0] == "" || parts[1] != "resources" || (parts[2] != "reservations" && parts[2] != "incidents") {
		return false
	}
	principal, ok := h.authorize(w, r, identity.CapabilityAccountsRead, requestID)
	if !ok {
		return true
	}
	if len(parts) == 4 {
		if parts[3] == "" || r.URL.RawQuery != "" {
			writeError(w, 422, "invalid_request", requestID)
			return true
		}
		value, err := h.service.GetAccountResource(r.Context(), principal.AccountID, parts[0], parts[3], requestID, parts[2])
		if err != nil {
			replyError(w, err, requestID)
			return true
		}
		writeJSON(w, 200, map[string]any{"data": value})
		return true
	}
	for key, values := range r.URL.Query() {
		if (key != "page_size" && key != "cursor") || len(values) != 1 {
			writeError(w, 422, "invalid_request", requestID)
			return true
		}
	}
	pageSize := 25
	if raw := r.URL.Query().Get("page_size"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			writeError(w, 422, "invalid_request", requestID)
			return true
		}
		pageSize = value
	}
	value, err := h.service.ListAccountResources(r.Context(), principal.AccountID, parts[0], requestID, parts[2], r.URL.Query().Get("cursor"), pageSize)
	if err != nil {
		replyError(w, err, requestID)
		return true
	}
	writeJSON(w, 200, map[string]any{"data": value})
	return true
}

func (h *Handler) servePublicationSchedule(w http.ResponseWriter, r *http.Request, path, requestID string) bool {
	const prefix = "/api/v1/admin/global/publications/"
	if !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, "/schedule") || (r.Method != http.MethodGet && r.Method != http.MethodPut) {
		return false
	}
	id := strings.TrimSuffix(strings.TrimPrefix(path, prefix), "/schedule")
	if id == "" || strings.Contains(id, "/") {
		writeError(w, 422, "invalid_request", requestID)
		return true
	}
	capability := identity.CapabilityPublicationsRead
	if r.Method == http.MethodPut {
		capability = identity.CapabilityPublicationsUpdate
	}
	principal, ok := h.authorize(w, r, capability, requestID)
	if !ok {
		return true
	}
	if r.Method == http.MethodGet {
		if len(r.URL.Query()) != 1 || r.URL.Query().Get("owner_id") == "" {
			writeError(w, 422, "invalid_request", requestID)
			return true
		}
		value, err := h.service.PublicationSchedule(r.Context(), principal.AccountID, id, r.URL.Query().Get("owner_id"), requestID)
		if err != nil {
			replyError(w, err, requestID)
			return true
		}
		writeJSON(w, 200, map[string]any{"data": value})
		return true
	}
	if !recentlyAuthenticated(principal.AuthenticatedAt, time.Now().UTC()) {
		writeError(w, http.StatusPreconditionRequired, "recent_authentication_required", requestID)
		return true
	}
	var input struct {
		OwnerID           string              `json:"owner_id"`
		ExpectedUpdatedAt time.Time           `json:"expected_updated_at"`
		Reason            string              `json:"reason"`
		Enabled           bool                `json:"enabled"`
		Days              []booking.WeeklyDay `json:"days"`
	}
	if !decodeManagementBody(w, r, &input) {
		writeError(w, 422, "invalid_request", requestID)
		return true
	}
	value, err := h.service.SavePublicationSchedule(r.Context(), principal.AccountID, id, input.OwnerID, requestID, input.ExpectedUpdatedAt, input.Reason, booking.WeeklyHours{SpaceID: id, Enabled: input.Enabled, Days: input.Days})
	if err != nil {
		replyError(w, err, requestID)
		return true
	}
	writeJSON(w, 200, map[string]any{"data": value})
	return true
}

func (h *Handler) servePublicationMutation(w http.ResponseWriter, r *http.Request, path, requestID string) bool {
	const base = "/api/v1/admin/global/publications/"
	if !strings.HasPrefix(path, base) {
		return false
	}
	parts := strings.Split(strings.TrimPrefix(path, base), "/")
	if len(parts) != 2 || parts[0] == "" {
		return false
	}
	var capability identity.Capability
	switch {
	case len(parts) == 2 && parts[1] == "state" && r.Method == http.MethodPost:
		capability = identity.CapabilityPublicationsModerate
	case len(parts) == 2 && parts[1] == "content" && r.Method == http.MethodPatch:
		capability = identity.CapabilityPublicationsUpdate
	case len(parts) == 2 && parts[1] == "retire" && r.Method == http.MethodPost:
		capability = identity.CapabilityPublicationsRetire
	default:
		return false
	}
	principal, ok := h.authorize(w, r, capability, requestID)
	if !ok {
		return true
	}
	if !recentlyAuthenticated(principal.AuthenticatedAt, time.Now().UTC()) {
		writeError(w, http.StatusPreconditionRequired, "recent_authentication_required", requestID)
		return true
	}
	if capability == identity.CapabilityPublicationsRetire {
		var input struct {
			OwnerID           string    `json:"owner_id"`
			ExpectedUpdatedAt time.Time `json:"expected_updated_at"`
			Reason            string    `json:"reason"`
		}
		if !decodeManagementBody(w, r, &input) {
			writeError(w, 422, "invalid_request", requestID)
			return true
		}
		value, err := h.service.RetirePublication(r.Context(), principal.AccountID, parts[0], input.OwnerID, requestID, input.ExpectedUpdatedAt, input.Reason)
		if err != nil {
			replyError(w, err, requestID)
			return true
		}
		writeJSON(w, 200, map[string]any{"data": value})
		return true
	}
	if parts[1] == "content" {
		var input adminglobal.PublicationContentUpdate
		if !decodeManagementBody(w, r, &input) {
			writeError(w, 422, "invalid_request", requestID)
			return true
		}
		value, err := h.service.UpdatePublication(r.Context(), principal.AccountID, parts[0], requestID, input)
		if err != nil {
			replyError(w, err, requestID)
			return true
		}
		writeJSON(w, 200, map[string]any{"data": value})
		return true
	}
	var input adminglobal.PublicationStateInput
	if !decodeManagementBody(w, r, &input) {
		writeError(w, 422, "invalid_request", requestID)
		return true
	}
	value, err := h.service.SetPublicationState(r.Context(), principal.AccountID, parts[0], requestID, input)
	if err != nil {
		replyError(w, err, requestID)
		return true
	}
	writeJSON(w, 200, map[string]any{"data": value})
	return true
}

func parsePublicationFilter(query url.Values) (adminglobal.PublicationFilter, bool) {
	allowed := map[string]bool{"owner_id": true, "category": true, "state": true, "search": true, "publicly_visible": true, "page_size": true, "cursor": true}
	for key, values := range query {
		if !allowed[key] || len(values) != 1 {
			return adminglobal.PublicationFilter{}, false
		}
	}
	get := func(k string) string {
		if len(query[k]) == 0 {
			return ""
		}
		return query.Get(k)
	}
	f := adminglobal.PublicationFilter{OwnerID: get("owner_id"), Category: get("category"), State: get("state"), Search: get("search"), Cursor: get("cursor"), PageSize: 25}
	if value := get("publicly_visible"); value != "" {
		v, err := strconv.ParseBool(value)
		if err != nil {
			return f, false
		}
		f.Visible = &v
	}
	if value := get("page_size"); value != "" {
		n, err := strconv.Atoi(value)
		if err != nil {
			return f, false
		}
		f.PageSize = n
	}
	return f, true
}

func (h *Handler) serveAccountMutation(w http.ResponseWriter, r *http.Request, path, requestID string) bool {
	const base = "/api/v1/admin/global/accounts/"
	if !strings.HasPrefix(path, base) {
		return false
	}
	parts := strings.Split(strings.TrimPrefix(path, base), "/")
	if len(parts) < 2 || parts[0] == "" {
		return false
	}
	var capability identity.Capability
	switch {
	case len(parts) == 2 && parts[1] == "profile" && r.Method == http.MethodPatch:
		capability = identity.CapabilityAccountsUpdate
	case len(parts) == 3 && parts[1] == "roles" && r.Method == http.MethodPut:
		capability = identity.CapabilityAccountsRoles
	case len(parts) == 2 && parts[1] == "legacy-administrator" && r.Method == http.MethodPut:
		capability = identity.CapabilityAdminAccessManage
	case len(parts) == 3 && parts[1] == "sessions" && parts[2] == "revoke" && r.Method == http.MethodPost:
		capability = identity.CapabilityAccountsBlock
	case len(parts) == 2 && parts[1] == "block" && r.Method == http.MethodPost:
		capability = identity.CapabilityAccountsBlock
	default:
		return false
	}
	principal, ok := h.authorize(w, r, capability, requestID)
	if !ok {
		return true
	}
	if !recentlyAuthenticated(principal.AuthenticatedAt, time.Now().UTC()) {
		writeError(w, http.StatusPreconditionRequired, "recent_authentication_required", requestID)
		return true
	}
	if len(parts) == 2 {
		if parts[1] == "legacy-administrator" {
			var input struct {
				Granted bool   `json:"granted"`
				Reason  string `json:"reason"`
			}
			if !decodeManagementBody(w, r, &input) {
				writeError(w, 422, "invalid_request", requestID)
				return true
			}
			value, err := h.service.SetLegacyAdministrator(r.Context(), principal.AccountID, parts[0], input.Reason, requestID, input.Granted)
			if err != nil {
				replyError(w, err, requestID)
				return true
			}
			writeJSON(w, 200, map[string]any{"data": value})
			return true
		}
		if parts[1] == "block" {
			var input struct {
				Blocked bool   `json:"blocked"`
				Reason  string `json:"reason"`
			}
			if !decodeManagementBody(w, r, &input) {
				writeError(w, 422, "invalid_request", requestID)
				return true
			}
			value, err := h.service.SetAccountBlocked(r.Context(), principal.AccountID, parts[0], input.Reason, requestID, input.Blocked)
			if err != nil {
				replyError(w, err, requestID)
				return true
			}
			writeJSON(w, 200, map[string]any{"data": value})
			return true
		}
		var input adminglobal.AccountProfileInput
		if !decodeManagementBody(w, r, &input) {
			writeError(w, 422, "invalid_request", requestID)
			return true
		}
		value, err := h.service.UpdateAccountProfile(r.Context(), principal.AccountID, parts[0], requestID, input)
		if err != nil {
			replyError(w, err, requestID)
			return true
		}
		writeJSON(w, 200, map[string]any{"data": value})
		return true
	}
	if parts[1] == "sessions" {
		var input struct {
			Reason string `json:"reason"`
		}
		if !decodeManagementBody(w, r, &input) {
			writeError(w, 422, "invalid_request", requestID)
			return true
		}
		value, err := h.service.RevokeAccountSessions(r.Context(), principal.AccountID, parts[0], input.Reason, requestID)
		if err != nil {
			replyError(w, err, requestID)
			return true
		}
		writeJSON(w, 200, map[string]any{"data": value})
		return true
	}
	var input struct {
		Granted bool   `json:"granted"`
		Reason  string `json:"reason"`
	}
	if !decodeManagementBody(w, r, &input) {
		writeError(w, 422, "invalid_request", requestID)
		return true
	}
	value, err := h.service.SetAccountRole(r.Context(), principal.AccountID, parts[0], parts[2], input.Reason, requestID, input.Granted)
	if err != nil {
		replyError(w, err, requestID)
		return true
	}
	writeJSON(w, 200, map[string]any{"data": value})
	return true
}

const recentAuthenticationTTL = 10 * time.Minute

func recentlyAuthenticated(at, now time.Time) bool {
	age := now.Sub(at)
	return !at.IsZero() && age >= 0 && age <= recentAuthenticationTTL
}

func decodeManagementBody(w http.ResponseWriter, r *http.Request, target any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 16*1024)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if decoder.Decode(target) != nil {
		return false
	}
	var extra any
	return decoder.Decode(&extra) == io.EOF
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

func parseReportPeriod(q url.Values) (adminglobal.ReportPeriod, bool) {
	if len(q) != 3 || len(q["from"]) != 1 || len(q["until"]) != 1 || len(q["time_zone"]) != 1 {
		return adminglobal.ReportPeriod{}, false
	}
	from, e1 := time.Parse(time.RFC3339, q.Get("from"))
	until, e2 := time.Parse(time.RFC3339, q.Get("until"))
	p := adminglobal.ReportPeriod{From: from, Until: until, TimeZone: q.Get("time_zone")}
	return p, e1 == nil && e2 == nil && p.Validate() == nil
}

func parseAuditFilter(q url.Values) (adminglobal.AuditFilter, bool) {
	allowed := map[string]bool{"from": true, "until": true, "actor_id": true, "target_account_id": true, "resource_type": true, "resource_id": true, "action": true, "result": true, "reason_code": true, "correlation_id": true, "page_size": true, "cursor": true, "snapshot_cutoff": true, "format": true}
	for k, v := range q {
		if !allowed[k] || len(v) != 1 {
			return adminglobal.AuditFilter{}, false
		}
	}
	from, e1 := time.Parse(time.RFC3339, q.Get("from"))
	until, e2 := time.Parse(time.RFC3339, q.Get("until"))
	size := 25
	if q.Get("page_size") != "" {
		var err error
		size, err = strconv.Atoi(q.Get("page_size"))
		if err != nil {
			return adminglobal.AuditFilter{}, false
		}
	}
	f := adminglobal.AuditFilter{From: from, Until: until, ActorID: q.Get("actor_id"), TargetID: q.Get("target_account_id"), ResourceType: q.Get("resource_type"), ResourceID: q.Get("resource_id"), Action: q.Get("action"), Result: q.Get("result"), ReasonCode: q.Get("reason_code"), CorrelationID: q.Get("correlation_id"), PageSize: size, Cursor: q.Get("cursor"), Format: q.Get("format")}
	if q.Get("snapshot_cutoff") != "" {
		cutoff, err := time.Parse(time.RFC3339Nano, q.Get("snapshot_cutoff"))
		if err != nil {
			return adminglobal.AuditFilter{}, false
		}
		f.SnapshotCutoff = &cutoff
	}
	format := q.Get("format")
	if format != "" && format != "json" && format != "csv" {
		return adminglobal.AuditFilter{}, false
	}
	return f, e1 == nil && e2 == nil && f.From.Before(f.Until) && f.Until.Sub(f.From) <= 31*24*time.Hour && size >= 1 && size <= 100
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
	case errors.Is(err, adminglobal.ErrConflict):
		writeError(w, 409, "conflict", requestID)
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
