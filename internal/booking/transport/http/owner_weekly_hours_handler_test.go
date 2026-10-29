package bookinghttp

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/booking"
	"github.com/HernanEspinozaDev/espaciGo/internal/spaces"
	spaceshttp "github.com/HernanEspinozaDev/espaciGo/internal/spaces/transport/http"
)

const ownerScheduleSpaceID = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"

var ownerScheduleRevision = time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)

type ownerScheduleServiceStub struct {
	value      booking.WeeklyHours
	getErr     error
	saveErr    error
	getCalls   int
	saveCalls  int
	ownerID    string
	spaceID    string
	expected   time.Time
	savedValue booking.WeeklyHours
}

func (s *ownerScheduleServiceStub) OwnerWeeklyHours(_ context.Context, ownerID, spaceID string) (booking.WeeklyHours, error) {
	s.getCalls++
	s.ownerID, s.spaceID = ownerID, spaceID
	return s.value, s.getErr
}

func (s *ownerScheduleServiceStub) SaveOwnerWeeklyHoursCAS(_ context.Context, ownerID, spaceID string, value booking.WeeklyHours, expected time.Time) (booking.WeeklyHours, error) {
	s.saveCalls++
	s.ownerID, s.spaceID, s.expected, s.savedValue = ownerID, spaceID, expected, value
	if s.saveErr != nil {
		return booking.WeeklyHours{}, s.saveErr
	}
	value.SpaceID = spaceID
	value.TimeZone = "America/Santiago"
	value.UpdatedAt = expected.Add(time.Minute)
	return value, nil
}

func ownerScheduleDays() []booking.WeeklyDay {
	days := make([]booking.WeeklyDay, 7)
	for i := range days {
		days[i] = booking.WeeklyDay{Weekday: i + 1, Periods: []booking.WeeklyPeriod{}}
	}
	days[0].Periods = []booking.WeeklyPeriod{{Open: "09:00", Close: "17:00"}}
	return days
}

func ownerScheduleRequest(method, body string) *http.Request {
	req := httptest.NewRequest(method, "/api/v1/spaces/"+ownerScheduleSpaceID+"/weekly-hours", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-session")
	if method == http.MethodPut {
		req.Header.Set("Content-Type", "application/json")
	}
	return req
}

func ownerScheduleETag(at time.Time) string {
	return spaceshttp.DraftETag(spaces.Draft{ID: ownerScheduleSpaceID, UpdatedAt: at})
}

func TestOwnerWeeklyHoursGETScopesOwnerAndReturnsSharedRevision(t *testing.T) {
	repo := &ownerScheduleServiceStub{value: booking.WeeklyHours{
		SpaceID: ownerScheduleSpaceID, Enabled: true, TimeZone: "America/Santiago",
		UpdatedAt: ownerScheduleRevision, Days: ownerScheduleDays(),
	}}
	h := NewOwnerWeeklyHoursHandler(authStub{}, repo, nil)
	res := httptest.NewRecorder()
	h.ServeHTTP(res, ownerScheduleRequest(http.MethodGet, ""))

	if res.Code != http.StatusOK || repo.getCalls != 1 || repo.ownerID != renterID || repo.spaceID != ownerScheduleSpaceID {
		t.Fatalf("GET status=%d calls=%d owner=%q space=%q body=%s", res.Code, repo.getCalls, repo.ownerID, repo.spaceID, res.Body.String())
	}
	if got, want := res.Header().Get("ETag"), ownerScheduleETag(ownerScheduleRevision); got != want {
		t.Fatalf("GET ETag=%q want=%q", got, want)
	}
	if strings.Contains(res.Body.String(), "safety_notice") {
		t.Fatalf("owner space response leaked local booking-trial envelope: %s", res.Body.String())
	}
}

func TestOwnerWeeklyHoursPUTChecksIfMatchBeforeBodyAndMapsCAS(t *testing.T) {
	validBody := `{"enabled":true,"days":[{"weekday":1,"periods":[{"open":"09:00","close":"17:00"}]},{"weekday":2,"periods":[]},{"weekday":3,"periods":[]},{"weekday":4,"periods":[]},{"weekday":5,"periods":[]},{"weekday":6,"periods":[]},{"weekday":7,"periods":[]}]}`

	t.Run("missing precondition precedes malformed body", func(t *testing.T) {
		repo := &ownerScheduleServiceStub{}
		h := NewOwnerWeeklyHoursHandler(authStub{}, repo, nil)
		res := httptest.NewRecorder()
		h.ServeHTTP(res, ownerScheduleRequest(http.MethodPut, "{"))
		if strings.Contains(res.Body.String(), "ensayo local") {
			t.Fatalf("owner endpoint returned local booking-trial copy: %s", res.Body.String())
		}
		if res.Code != http.StatusPreconditionRequired || repo.saveCalls != 0 {
			t.Fatalf("status=%d saveCalls=%d body=%s", res.Code, repo.saveCalls, res.Body.String())
		}
	})

	for _, tag := range []string{`W/"weak"`, "*", `"a", "b"`, "malformed"} {
		t.Run("invalid If-Match "+tag, func(t *testing.T) {
			repo := &ownerScheduleServiceStub{}
			h := NewOwnerWeeklyHoursHandler(authStub{}, repo, nil)
			req := ownerScheduleRequest(http.MethodPut, "{")
			req.Header.Set("If-Match", tag)
			res := httptest.NewRecorder()
			h.ServeHTTP(res, req)
			if res.Code != http.StatusBadRequest || repo.saveCalls != 0 {
				t.Fatalf("status=%d saveCalls=%d body=%s", res.Code, repo.saveCalls, res.Body.String())
			}
		})
	}

	t.Run("invalid JSON does not call service", func(t *testing.T) {
		repo := &ownerScheduleServiceStub{}
		h := NewOwnerWeeklyHoursHandler(authStub{}, repo, nil)
		req := ownerScheduleRequest(http.MethodPut, "{")
		req.Header.Set("If-Match", ownerScheduleETag(ownerScheduleRevision))
		res := httptest.NewRecorder()
		h.ServeHTTP(res, req)
		if res.Code != http.StatusBadRequest || repo.saveCalls != 0 {
			t.Fatalf("status=%d saveCalls=%d body=%s", res.Code, repo.saveCalls, res.Body.String())
		}
		if strings.Contains(res.Body.String(), "ensayo local") {
			t.Fatalf("owner endpoint returned local booking-trial copy: %s", res.Body.String())
		}
	})

	t.Run("unsupported media type is generic and does not call service", func(t *testing.T) {
		repo := &ownerScheduleServiceStub{}
		h := NewOwnerWeeklyHoursHandler(authStub{}, repo, nil)
		req := ownerScheduleRequest(http.MethodPut, validBody)
		req.Header.Set("If-Match", ownerScheduleETag(ownerScheduleRevision))
		req.Header.Set("Content-Type", "text/plain")
		res := httptest.NewRecorder()
		h.ServeHTTP(res, req)
		if res.Code != http.StatusUnsupportedMediaType || repo.saveCalls != 0 || strings.Contains(res.Body.String(), "ensayo local") {
			t.Fatalf("status=%d saveCalls=%d body=%s", res.Code, repo.saveCalls, res.Body.String())
		}
	})

	t.Run("JSON media type accepts charset parameters", func(t *testing.T) {
		repo := &ownerScheduleServiceStub{}
		h := NewOwnerWeeklyHoursHandler(authStub{}, repo, nil)
		req := ownerScheduleRequest(http.MethodPut, validBody)
		req.Header.Set("If-Match", ownerScheduleETag(ownerScheduleRevision))
		req.Header.Set("Content-Type", "application/json; charset=utf-8")
		res := httptest.NewRecorder()
		h.ServeHTTP(res, req)
		if res.Code != http.StatusOK || repo.saveCalls != 1 {
			t.Fatalf("status=%d saveCalls=%d body=%s", res.Code, repo.saveCalls, res.Body.String())
		}
	})

	t.Run("stale revision maps to 412 without renewed ETag", func(t *testing.T) {
		repo := &ownerScheduleServiceStub{saveErr: booking.ErrConflict}
		h := NewOwnerWeeklyHoursHandler(authStub{}, repo, nil)
		req := ownerScheduleRequest(http.MethodPut, validBody)
		req.Header.Set("If-Match", ownerScheduleETag(ownerScheduleRevision))
		res := httptest.NewRecorder()
		h.ServeHTTP(res, req)
		if res.Code != http.StatusPreconditionFailed || repo.saveCalls != 1 || res.Header().Get("ETag") != "" {
			t.Fatalf("status=%d saveCalls=%d ETag=%q body=%s", res.Code, repo.saveCalls, res.Header().Get("ETag"), res.Body.String())
		}
		if repo.ownerID != renterID || repo.spaceID != ownerScheduleSpaceID || !repo.expected.Equal(ownerScheduleRevision) {
			t.Fatalf("CAS scope owner=%q space=%q expected=%s", repo.ownerID, repo.spaceID, repo.expected)
		}
	})

	t.Run("success returns renewed ETag", func(t *testing.T) {
		repo := &ownerScheduleServiceStub{}
		h := NewOwnerWeeklyHoursHandler(authStub{}, repo, nil)
		req := ownerScheduleRequest(http.MethodPut, validBody)
		req.Header.Set("If-Match", ownerScheduleETag(ownerScheduleRevision))
		res := httptest.NewRecorder()
		h.ServeHTTP(res, req)
		if res.Code != http.StatusOK || repo.saveCalls != 1 {
			t.Fatalf("status=%d saveCalls=%d body=%s", res.Code, repo.saveCalls, res.Body.String())
		}
		if got, want := res.Header().Get("ETag"), ownerScheduleETag(ownerScheduleRevision.Add(time.Minute)); got != want {
			t.Fatalf("success ETag=%q want=%q", got, want)
		}
		if repo.ownerID != renterID || repo.spaceID != ownerScheduleSpaceID || !repo.expected.Equal(ownerScheduleRevision) || !repo.savedValue.Enabled || len(repo.savedValue.Days) != 7 {
			t.Fatalf("CAS arguments owner=%q space=%q expected=%s value=%+v", repo.ownerID, repo.spaceID, repo.expected, repo.savedValue)
		}
	})

	t.Run("owner cannot read another account's space", func(t *testing.T) {
		repo := &ownerScheduleServiceStub{getErr: booking.ErrNotFound}
		h := NewOwnerWeeklyHoursHandler(authStub{}, repo, nil)
		res := httptest.NewRecorder()
		h.ServeHTTP(res, ownerScheduleRequest(http.MethodGet, ""))
		if res.Code != http.StatusNotFound || repo.getCalls != 1 || res.Header().Get("ETag") != "" {
			t.Fatalf("status=%d getCalls=%d ETag=%q body=%s", res.Code, repo.getCalls, res.Header().Get("ETag"), res.Body.String())
		}
	})
}

func TestOwnerWeeklyHoursCORSAllowsCASAndExposesETag(t *testing.T) {
	h := NewOwnerWeeklyHoursHandler(authStub{}, &ownerScheduleServiceStub{}, []string{"http://ui.local"})
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/spaces/"+ownerScheduleSpaceID+"/weekly-hours", nil)
	req.Header.Set("Origin", "http://ui.local")
	res := httptest.NewRecorder()
	h.ServeHTTP(res, req)
	if res.Code != http.StatusNoContent || !strings.Contains(res.Header().Get("Access-Control-Allow-Headers"), "If-Match") || !strings.Contains(res.Header().Get("Access-Control-Expose-Headers"), "ETag") {
		t.Fatalf("CORS status=%d allow=%q expose=%q", res.Code, res.Header().Get("Access-Control-Allow-Headers"), res.Header().Get("Access-Control-Expose-Headers"))
	}
}

func TestOwnerWeeklyHoursPropagatesRepositoryErrorsAsInternal(t *testing.T) {
	unexpected := errors.New("synthetic repository failure")
	repo := &ownerScheduleServiceStub{getErr: unexpected}
	h := NewOwnerWeeklyHoursHandler(authStub{}, repo, nil)
	res := httptest.NewRecorder()
	h.ServeHTTP(res, ownerScheduleRequest(http.MethodGet, ""))
	if res.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d body=%s", res.Code, res.Body.String())
	}
}
