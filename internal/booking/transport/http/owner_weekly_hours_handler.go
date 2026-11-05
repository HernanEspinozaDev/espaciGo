package bookinghttp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/credentials"
	"github.com/HernanEspinozaDev/espaciGo/internal/booking"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
	"github.com/HernanEspinozaDev/espaciGo/internal/spaces"
	spaceshttp "github.com/HernanEspinozaDev/espaciGo/internal/spaces/transport/http"
)

// OwnerWeeklyHoursService exposes schedules for ordinary owner-managed spaces
// independently of the local booking-trial service and payment adapter.
type OwnerWeeklyHoursService interface {
	OwnerWeeklyHours(context.Context, string, string) (booking.WeeklyHours, error)
	SaveOwnerWeeklyHoursCAS(context.Context, string, string, booking.WeeklyHours, time.Time) (booking.WeeklyHours, error)
}

type ownerWeeklyHoursHandler struct {
	auth    Authenticator
	service OwnerWeeklyHoursService
	origins map[string]bool
}

func NewOwnerWeeklyHoursHandler(auth Authenticator, service OwnerWeeklyHoursService, origins []string) http.Handler {
	allowed := make(map[string]bool, len(origins))
	for _, origin := range origins {
		allowed[origin] = true
	}
	return &ownerWeeklyHoursHandler{auth: auth, service: service, origins: allowed}
}

func (h *ownerWeeklyHoursHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rid, err := (credentials.Generator{}).ID()
	if err != nil {
		ownerWeeklyHoursFail(w, http.StatusInternalServerError, "internal_error")
		return
	}
	w.Header().Set("X-Request-ID", rid)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Add("Vary", "Origin")
	if origin := r.Header.Get("Origin"); origin != "" {
		if !h.origins[origin] {
			ownerWeeklyHoursFail(w, http.StatusForbidden, "origin_denied")
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, If-Match")
		w.Header().Set("Access-Control-Allow-Methods", "GET, PUT, OPTIONS")
		w.Header().Set("Access-Control-Expose-Headers", "ETag, X-Request-ID")
	}
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	spaceID, ok := ownerWeeklyHoursSpaceID(r.URL.Path)
	if !ok || r.URL.RawQuery != "" {
		ownerWeeklyHoursFail(w, http.StatusUnprocessableEntity, "invalid_request")
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodPut {
		w.Header().Set("Allow", "GET, PUT, OPTIONS")
		ownerWeeklyHoursFail(w, http.StatusMethodNotAllowed, "method_not_allowed")
		return
	}
	if h.auth == nil || h.service == nil {
		ownerWeeklyHoursFail(w, http.StatusInternalServerError, "internal_error")
		return
	}
	principal, authErr := h.auth.Authorize(r.Context(), identity.Secret(bearer(r.Header.Get("Authorization"))), "", identity.UserOperation)
	if authErr != nil {
		if errors.Is(authErr, identity.ErrForbidden) {
			ownerWeeklyHoursFail(w, http.StatusForbidden, "forbidden")
		} else {
			w.Header().Set("WWW-Authenticate", "Bearer")
			ownerWeeklyHoursFail(w, http.StatusUnauthorized, "unauthenticated")
		}
		return
	}

	switch r.Method {
	case http.MethodGet:
		value, getErr := h.service.OwnerWeeklyHours(r.Context(), principal.AccountID, spaceID)
		if getErr != nil {
			h.replyError(w, getErr, false)
			return
		}
		w.Header().Set("ETag", spaceshttp.DraftETag(spaces.Draft{ID: spaceID, UpdatedAt: value.UpdatedAt}))
		write(w, http.StatusOK, map[string]any{"data": value})
	case http.MethodPut:
		stamp, preconditionErr := spaceshttp.ParseIfMatch(r.Header.Values("If-Match"), spaceID)
		if preconditionErr != nil {
			switch {
			case errors.Is(preconditionErr, spaceshttp.ErrMissingPrecondition):
				ownerWeeklyHoursFail(w, http.StatusPreconditionRequired, "precondition_required")
			case errors.Is(preconditionErr, spaceshttp.ErrStalePrecondition):
				ownerWeeklyHoursFail(w, http.StatusPreconditionFailed, "precondition_failed")
			default:
				ownerWeeklyHoursFail(w, http.StatusBadRequest, "invalid_request")
			}
			return
		}
		expected, parseErr := time.Parse(time.RFC3339Nano, stamp)
		if parseErr != nil {
			ownerWeeklyHoursFail(w, http.StatusBadRequest, "invalid_request")
			return
		}
		var input struct {
			Enabled bool                `json:"enabled"`
			Days    []booking.WeeklyDay `json:"days"`
		}
		if !decodeOwnerWeeklyHours(w, r, &input) {
			return
		}
		value, saveErr := h.service.SaveOwnerWeeklyHoursCAS(r.Context(), principal.AccountID, spaceID, booking.WeeklyHours{Enabled: input.Enabled, Days: input.Days}, expected.UTC())
		if errors.Is(saveErr, booking.ErrConflict) {
			ownerWeeklyHoursFail(w, http.StatusPreconditionFailed, "precondition_failed")
			return
		}
		if saveErr != nil {
			h.replyError(w, saveErr, true)
			return
		}
		w.Header().Set("ETag", spaceshttp.DraftETag(spaces.Draft{ID: spaceID, UpdatedAt: value.UpdatedAt}))
		write(w, http.StatusOK, map[string]any{"data": value})
	}
}

func decodeOwnerWeeklyHours(w http.ResponseWriter, r *http.Request, value any) bool {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || !strings.EqualFold(mediaType, "application/json") {
		ownerWeeklyHoursFail(w, http.StatusUnsupportedMediaType, "unsupported_media_type")
		return false
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16*1024))
	decoder.DisallowUnknownFields()
	if decoder.Decode(value) != nil {
		ownerWeeklyHoursFail(w, http.StatusBadRequest, "invalid_request")
		return false
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		ownerWeeklyHoursFail(w, http.StatusBadRequest, "invalid_request")
		return false
	}
	return true
}

func ownerWeeklyHoursSpaceID(path string) (string, bool) {
	const prefix, suffix = "/api/v1/spaces/", "/weekly-hours"
	if !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, suffix) {
		return "", false
	}
	spaceID := strings.TrimSuffix(strings.TrimPrefix(path, prefix), suffix)
	if spaceID == "" || strings.Contains(spaceID, "/") {
		return "", false
	}
	return spaceID, true
}

func (h *ownerWeeklyHoursHandler) replyError(w http.ResponseWriter, err error, put bool) {
	switch {
	case errors.Is(err, booking.ErrInvalid):
		ownerWeeklyHoursFail(w, http.StatusUnprocessableEntity, "invalid_request")
	case errors.Is(err, booking.ErrNotFound):
		ownerWeeklyHoursFail(w, http.StatusNotFound, "not_found")
	case errors.Is(err, booking.ErrConflict) && put:
		ownerWeeklyHoursFail(w, http.StatusPreconditionFailed, "precondition_failed")
	default:
		ownerWeeklyHoursFail(w, http.StatusInternalServerError, "internal_error")
	}
}

func ownerWeeklyHoursFail(w http.ResponseWriter, status int, code string) {
	write(w, status, map[string]any{"error": map[string]string{
		"code": code, "message": "No fue posible completar la solicitud.", "request_id": w.Header().Get("X-Request-ID"),
	}})
}
