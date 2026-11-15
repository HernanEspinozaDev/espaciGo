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

type dstAvailabilityFixture struct {
	listing   rentalflow.Listing
	intervals []rentalflow.Interval
}

const dstOwnerID = "44444444-4444-4444-8444-444444444444"

func (f *dstAvailabilityFixture) PublicListing(_ context.Context, spaceID string) (rentalflow.Listing, error) {
	if spaceID != f.listing.ID {
		return rentalflow.Listing{}, rentalflow.ErrNotFound
	}
	return f.listing, nil
}

func (f *dstAvailabilityFixture) PublicIntervals(context.Context, rentalflow.Listing, string, int) ([]rentalflow.Interval, error) {
	return f.intervals, nil
}

func (*dstAvailabilityFixture) IntervalAvailable(context.Context, rentalflow.Listing, rentalflow.Interval) (bool, error) {
	return false, nil
}

func TestSyntheticHTTPAvailabilityPreservesBothDSTFoldInstants(t *testing.T) {
	firstStart, err := time.Parse(time.RFC3339, "2026-11-01T01:30:00-04:00")
	if err != nil {
		t.Fatal(err)
	}
	secondStart, err := time.Parse(time.RFC3339, "2026-11-01T01:30:00-05:00")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC)
	fixture := &dstAvailabilityFixture{
		listing: rentalflow.Listing{
			ID: testSpaceID, OwnerID: dstOwnerID, State: "activa", Title: "Sala sintética",
			RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", RateVersion: 2,
			TimeZone: "America/New_York",
		},
		intervals: []rentalflow.Interval{
			{StartAt: secondStart, EndAt: secondStart.Add(time.Hour)},
			{StartAt: firstStart, EndAt: firstStart.Add(time.Hour)},
		},
	}
	availability, err := rentalflow.NewAvailabilityService(fixture, fixture, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	handler := New(&authStub{}, availability, &quoteStub{}, &previewStub{}, nil)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/spaces/"+testSpaceID+"/rental-availability?date=2026-11-01&duration=1", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != http.StatusOK || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("status=%d cache=%q body=%s", w.Code, w.Header().Get("Cache-Control"), w.Body.String())
	}
	var body struct {
		Data struct {
			TimeZone string `json:"time_zone"`
			Items    []struct {
				StartAt time.Time `json:"start_at"`
				EndAt   time.Time `json:"end_at"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Data.TimeZone != "America/New_York" || len(body.Data.Items) != 2 ||
		body.Data.Items[0].StartAt.Format(time.RFC3339) != "2026-11-01T05:30:00Z" ||
		body.Data.Items[1].StartAt.Format(time.RFC3339) != "2026-11-01T06:30:00Z" ||
		!body.Data.Items[0].EndAt.Equal(firstStart.Add(time.Hour)) || !body.Data.Items[1].EndAt.Equal(secondStart.Add(time.Hour)) {
		t.Fatalf("wire availability lost or merged repeated local slots: %+v", body.Data)
	}
}

func TestSyntheticHTTPAvailabilityEncodesNoIntervalsAsEmptyArray(t *testing.T) {
	now := time.Date(2030, time.May, 1, 12, 0, 0, 0, time.UTC)
	fixture := &dstAvailabilityFixture{
		listing: rentalflow.Listing{
			ID: testSpaceID, OwnerID: dstOwnerID, State: "activa", Title: "Sala sintética",
			RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", RateVersion: 2,
			TimeZone: "America/New_York",
		},
	}
	availability, err := rentalflow.NewAvailabilityService(fixture, fixture, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	auth := &authStub{}
	handler := New(auth, availability, &quoteStub{}, &previewStub{}, nil)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/spaces/"+testSpaceID+"/rental-availability?date=2030-05-02&duration=1", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != http.StatusOK || auth.calls != 0 || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("status=%d auth calls=%d cache=%q body=%s", w.Code, auth.calls, w.Header().Get("Cache-Control"), w.Body.String())
	}
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	var data struct {
		Items json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(envelope.Data, &data); err != nil {
		t.Fatal(err)
	}
	if string(data.Items) != "[]" {
		t.Fatalf("empty availability wire value=%s, want []", data.Items)
	}
}
