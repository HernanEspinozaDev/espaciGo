package http

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/credentials"
	"github.com/HernanEspinozaDev/espaciGo/internal/adminlocal"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
)

type Authenticator interface {
	Authorize(rctx context.Context, token identity.Secret, role identity.Role, activity identity.ActivityKind) (identity.Principal, error)
}
type Handler struct {
	auth    Authenticator
	service *adminlocal.Service
	origins map[string]bool
}

func NewHandler(auth Authenticator, service *adminlocal.Service, origins []string) http.Handler {
	allowed := map[string]bool{}
	for _, origin := range origins {
		allowed[origin] = true
	}
	return &Handler{auth: auth, service: service, origins: allowed}
}
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	id, err := (credentials.Generator{}).ID()
	if err != nil {
		fail(w, 500, "internal_error", "")
		return
	}
	w.Header().Set("X-Request-ID", id)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Add("Vary", "Origin")
	if origin := r.Header.Get("Origin"); origin != "" {
		if !h.origins[origin] {
			fail(w, 403, "origin_denied", id)
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Idempotency-Key")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	}
	if r.Method == http.MethodOptions {
		w.WriteHeader(204)
		return
	}
	if h.auth == nil {
		fail(w, 404, "not_found", id)
		return
	}
	principal, authErr := h.auth.Authorize(r.Context(), identity.Secret(bearer(r.Header.Get("Authorization"))), identity.RoleAdministrator, identity.UserOperation)
	if authErr != nil {
		if errors.Is(authErr, identity.ErrForbidden) {
			fail(w, 403, "forbidden", id)
		} else {
			w.Header().Set("WWW-Authenticate", "Bearer")
			fail(w, 401, "unauthenticated", id)
		}
		return
	}
	if h.service == nil {
		fail(w, 404, "not_found", id)
		return
	}
	path := strings.TrimSuffix(r.URL.Path, "/")
	if r.Method == http.MethodPost && strings.HasPrefix(path, "/api/v1/admin/local/accounts/") {
		parts := strings.Split(strings.TrimPrefix(path, "/api/v1/admin/local/accounts/"), "/")
		if len(parts) != 2 || (parts[1] != "block" && parts[1] != "unblock") {
			fail(w, 404, "not_found", id)
			return
		}
		var input struct {
			Reason string `json:"reason_code"`
		}
		if decode(w, r, &input) != nil {
			fail(w, 400, "invalid_request", id)
			return
		}
		var result adminlocal.BlockResult
		if parts[1] == "block" {
			result, err = h.service.BlockAccount(r.Context(), principal.AccountID, parts[0], input.Reason, id)
		} else {
			result, err = h.service.UnblockAccount(r.Context(), principal.AccountID, parts[0], input.Reason, id)
		}
		if err != nil {
			replyServiceError(w, err, id)
			return
		}
		write(w, 200, map[string]any{"data": result, "notice": adminlocal.SafetyNotice})
		return
	}
	if r.Method != http.MethodGet {
		fail(w, 405, "method_not_allowed", id)
		return
	}
	if path != "/api/v1/admin/local/reports/reservations" && path != "/api/v1/admin/local/reports/finance" {
		fail(w, 404, "not_found", id)
		return
	}
	period, ok := parsePeriod(r.URL.Query())
	if !ok {
		fail(w, 422, "invalid_period", id)
		return
	}
	if path == "/api/v1/admin/local/reports/reservations" {
		value, e := h.service.ReservationReport(r.Context(), principal.AccountID, id, period)
		if e != nil {
			replyServiceError(w, e, id)
			return
		}
		write(w, 200, map[string]any{"data": value, "notice": adminlocal.SafetyNotice})
		return
	}
	value, e := h.service.FinanceReport(r.Context(), principal.AccountID, id, period)
	if e != nil {
		replyServiceError(w, e, id)
		return
	}
	write(w, 200, map[string]any{"data": value, "notice": adminlocal.SafetyNotice})
}

func bearer(value string) string {
	parts := strings.Fields(value)
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		return parts[1]
	}
	return ""
}
func decode(w http.ResponseWriter, r *http.Request, v any) error {
	defer r.Body.Close()
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return err
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return errors.New("trailing json")
	}
	return nil
}
func parsePeriod(q url.Values) (adminlocal.Period, bool) {
	if len(q) != 3 || len(q["from"]) != 1 || len(q["until"]) != 1 || len(q["timezone"]) != 1 {
		return adminlocal.Period{}, false
	}
	loc, err := time.LoadLocation(q.Get("timezone"))
	if err != nil {
		return adminlocal.Period{}, false
	}
	from, err := time.Parse(time.RFC3339, q.Get("from"))
	if err != nil {
		return adminlocal.Period{}, false
	}
	until, err := time.Parse(time.RFC3339, q.Get("until"))
	if err != nil {
		return adminlocal.Period{}, false
	}
	_, fromOffset := from.In(loc).Zone()
	_, untilOffset := until.In(loc).Zone()
	_, suppliedFromOffset := from.Zone()
	_, suppliedUntilOffset := until.Zone()
	if fromOffset != suppliedFromOffset || untilOffset != suppliedUntilOffset {
		return adminlocal.Period{}, false
	}
	p := adminlocal.Period{From: from.UTC(), Until: until.UTC(), TimeZone: loc.String()}
	return p, p.Validate() == nil
}
func replyServiceError(w http.ResponseWriter, err error, id string) {
	switch {
	case errors.Is(err, adminlocal.ErrInvalid):
		fail(w, 422, "invalid_request", id)
	case errors.Is(err, adminlocal.ErrForbidden):
		fail(w, 403, "forbidden", id)
	case errors.Is(err, adminlocal.ErrNotFound):
		fail(w, 404, "not_found", id)
	case errors.Is(err, adminlocal.ErrConflict):
		fail(w, 409, "conflict", id)
	default:
		fail(w, 500, "internal_error", id)
	}
}
func fail(w http.ResponseWriter, status int, code, id string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": code, "message": "No fue posible completar la operación administrativa local.", "request_id": id}})
}
func write(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
