package rentalflowhttp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/rentalflow"
)

type cancelingPublicIntervals struct {
	intervals []rentalflow.Interval
	cancel    context.CancelFunc
	calls     int
}

func (c *cancelingPublicIntervals) PublicIntervals(context.Context, rentalflow.Listing, string, int) ([]rentalflow.Interval, error) {
	c.calls++
	c.cancel()
	return c.intervals, nil
}

func (*cancelingPublicIntervals) IntervalAvailable(context.Context, rentalflow.Listing, rentalflow.Interval) (bool, error) {
	return false, nil
}

func TestSyntheticHTTPAvailabilityDiscardsIntervalsWhenCalendarCancelsContext(t *testing.T) {
	const ownerID = "44444444-4444-4444-8444-444444444444"
	const title = "Título sintético reservado"
	now := time.Date(2030, time.January, 1, 12, 0, 0, 0, time.UTC)
	start := time.Date(2030, time.January, 2, 13, 0, 0, 0, time.UTC)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	backend := &syntheticFlowBackend{listing: rentalflow.Listing{
		ID: testSpaceID, OwnerID: ownerID, State: "activa", Title: title,
		RateUnit: "hora", UnitPriceCLP: 12000, Currency: "CLP", RateVersion: 2, TimeZone: "America/Santiago",
	}}
	calendar := &cancelingPublicIntervals{
		intervals: []rentalflow.Interval{{StartAt: start, EndAt: start.Add(time.Hour)}}, cancel: cancel,
	}
	availability, err := rentalflow.NewAvailabilityService(backend, calendar, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	handler := New(nil, availability, nil, nil, nil)
	request := httptest.NewRequest(http.MethodGet,
		"/api/v1/spaces/"+testSpaceID+"/rental-availability?date=2030-01-02&duration=1", nil).WithContext(ctx)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable || response.Header().Get("Cache-Control") != "no-store" ||
		strings.Contains(response.Body.String(), `"data"`) || strings.Contains(response.Body.String(), start.Format(time.RFC3339)) {
		t.Fatalf("canceled calendar availability status=%d cache=%q body=%s, want 503/no-store without intervals", response.Code, response.Header().Get("Cache-Control"), response.Body.String())
	}
	for _, forbidden := range []string{title, ownerID, "12000", "America/Santiago", "rate_version"} {
		if strings.Contains(errorDisclosureBody(t, response.Body.Bytes()), forbidden) {
			t.Fatalf("canceled calendar availability disclosed %q: %s", forbidden, response.Body.String())
		}
	}
	if backend.listingReads != 1 || calendar.calls != 1 {
		t.Fatalf("unexpected reads before cancellation: listing=%d calendar=%d", backend.listingReads, calendar.calls)
	}
}
