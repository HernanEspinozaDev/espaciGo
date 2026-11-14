package rentalflowhttp

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/rentalflow"
)

type duplicateCalendarResponse struct {
	intervals []rentalflow.Interval
	err       error
	reads     int
}

func (c *duplicateCalendarResponse) PublicIntervals(context.Context, rentalflow.Listing, string, int) ([]rentalflow.Interval, error) {
	c.reads++
	return c.intervals, c.err
}

func (*duplicateCalendarResponse) IntervalAvailable(context.Context, rentalflow.Listing, rentalflow.Interval) (bool, error) {
	return false, nil
}

func TestSyntheticHTTPAvailabilityRejectsDuplicateCalendarIntervals(t *testing.T) {
	const ownerID = "44444444-4444-4444-8444-444444444444"
	now := time.Date(2030, time.January, 1, 12, 0, 0, 0, time.UTC)
	start := time.Date(2030, time.January, 2, 13, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	alternateOffset := time.FixedZone("synthetic UTC-5", -5*60*60)
	for _, tc := range []struct {
		name      string
		intervals []rentalflow.Interval
	}{
		{name: "identical interval", intervals: []rentalflow.Interval{{StartAt: start, EndAt: end}, {StartAt: start, EndAt: end}}},
		{name: "same instants with different offsets", intervals: []rentalflow.Interval{
			{StartAt: start, EndAt: end},
			{StartAt: start.In(alternateOffset), EndAt: end.In(alternateOffset)},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			backend := &syntheticFlowBackend{listing: rentalflow.Listing{
				ID: testSpaceID, OwnerID: ownerID, State: "activa", Title: "Sala sintética",
				RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", RateVersion: 1, TimeZone: "America/Santiago",
			}}
			calendar := &duplicateCalendarResponse{intervals: tc.intervals}
			availability, err := rentalflow.NewAvailabilityService(backend, calendar, func() time.Time { return now })
			if err != nil {
				t.Fatal(err)
			}
			handler := New(nil, availability, nil, nil, nil)
			request := httptest.NewRequest(http.MethodGet,
				"/api/v1/spaces/"+testSpaceID+"/rental-availability?date=2030-01-02&duration=1", nil)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)

			if response.Code != http.StatusServiceUnavailable || response.Header().Get("Cache-Control") != "no-store" ||
				strings.Contains(response.Body.String(), `"data"`) || strings.Contains(response.Body.String(), start.Format(time.RFC3339)) ||
				backend.listingReads != 1 || calendar.reads != 1 {
				t.Fatalf("duplicate calendar response status=%d cache=%q listing reads=%d calendar reads=%d body=%s", response.Code, response.Header().Get("Cache-Control"), backend.listingReads, calendar.reads, response.Body.String())
			}
		})
	}
}

func TestSyntheticHTTPAvailabilityDiscardsPartialCalendarResultsOnError(t *testing.T) {
	const ownerID = "44444444-4444-4444-8444-444444444444"
	now := time.Date(2030, time.January, 1, 12, 0, 0, 0, time.UTC)
	start := time.Date(2030, time.January, 2, 13, 0, 0, 0, time.UTC)
	backend := &syntheticFlowBackend{listing: rentalflow.Listing{
		ID: testSpaceID, OwnerID: ownerID, State: "activa", Title: "Sala sintética",
		RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", RateVersion: 1, TimeZone: "America/Santiago",
	}}
	calendar := &duplicateCalendarResponse{
		intervals: []rentalflow.Interval{{StartAt: start, EndAt: start.Add(time.Hour)}},
		err:       errors.New("synthetic private calendar backend failure"),
	}
	availability, err := rentalflow.NewAvailabilityService(backend, calendar, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	handler := New(nil, availability, nil, nil, nil)
	request := httptest.NewRequest(http.MethodGet,
		"/api/v1/spaces/"+testSpaceID+"/rental-availability?date=2030-01-02&duration=1", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable || response.Header().Get("Cache-Control") != "no-store" ||
		strings.Contains(response.Body.String(), `"data"`) || strings.Contains(response.Body.String(), start.Format(time.RFC3339)) ||
		strings.Contains(response.Body.String(), "synthetic private calendar backend failure") || backend.listingReads != 1 || calendar.reads != 1 {
		t.Fatalf("partial calendar error response status=%d cache=%q listing reads=%d calendar reads=%d body=%s", response.Code, response.Header().Get("Cache-Control"), backend.listingReads, calendar.reads, response.Body.String())
	}
}
