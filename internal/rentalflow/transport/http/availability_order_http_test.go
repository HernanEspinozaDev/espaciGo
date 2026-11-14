package rentalflowhttp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/rentalflow"
)

type orderedIntervalsCalendar struct {
	intervals []rentalflow.Interval
	calls     int
}

func (c *orderedIntervalsCalendar) PublicIntervals(context.Context, rentalflow.Listing, string, int) ([]rentalflow.Interval, error) {
	c.calls++
	return append([]rentalflow.Interval(nil), c.intervals...), nil
}

func (*orderedIntervalsCalendar) IntervalAvailable(context.Context, rentalflow.Listing, rentalflow.Interval) (bool, error) {
	return true, nil
}

func TestSyntheticHTTPAvailabilitySortsOutOfOrderCalendarIntervals(t *testing.T) {
	const ownerID = "44444444-4444-4444-8444-444444444444"
	now := time.Date(2030, time.January, 1, 12, 0, 0, 0, time.UTC)
	earlier := time.Date(2030, time.January, 2, 13, 0, 0, 0, time.UTC)
	later := earlier.Add(time.Hour)
	backend := &syntheticFlowBackend{listing: rentalflow.Listing{
		ID: testSpaceID, OwnerID: ownerID, State: "activa", Title: "Sala sintética",
		RateUnit: "hora", UnitPriceCLP: 12000, Currency: "CLP", RateVersion: 1, TimeZone: "America/Santiago",
	}}
	calendar := &orderedIntervalsCalendar{intervals: []rentalflow.Interval{
		{StartAt: later, EndAt: later.Add(time.Hour)},
		{StartAt: earlier, EndAt: earlier.Add(time.Hour)},
	}}
	availability, err := rentalflow.NewAvailabilityService(backend, calendar, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	handler := New(nil, availability, nil, nil, nil)
	request := httptest.NewRequest(http.MethodGet,
		"/api/v1/spaces/"+testSpaceID+"/rental-availability?date=2030-01-02&duration=1", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	var envelope struct {
		Data struct {
			Items []rentalflow.Interval `json:"items"`
		} `json:"data"`
	}
	if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("availability response status=%d cache=%q body=%s, want 200/no-store", response.Code, response.Header().Get("Cache-Control"), response.Body.String())
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode public availability response: %v", err)
	}
	if len(envelope.Data.Items) != 2 || !envelope.Data.Items[0].StartAt.Equal(earlier) || !envelope.Data.Items[1].StartAt.Equal(later) {
		t.Fatalf("public intervals order=%v, want chronological [%s, %s]", envelope.Data.Items, earlier, later)
	}
	if backend.listingReads != 1 || calendar.calls != 1 {
		t.Fatalf("unexpected availability reads: listing=%d calendar=%d", backend.listingReads, calendar.calls)
	}
}
