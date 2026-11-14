package rentalflow

import (
	"context"
	"testing"
	"time"
)

func TestPublicAvailabilityUsesOneClockSnapshotForValidationAndResponse(t *testing.T) {
	now := time.Date(2030, time.May, 10, 12, 0, 0, 0, time.UTC)
	start := now.Add(24 * time.Hour)
	clockCalls := 0
	clock := func() time.Time {
		clockCalls++
		if clockCalls == 1 {
			return now
		}
		return start
	}
	listing := Listing{
		ID: testSpaceID, OwnerID: testOwnerID, State: "activa", Title: "Sala sintética",
		RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", RateVersion: 1, TimeZone: "America/Santiago",
	}
	calendar := &calendarStub{intervals: []Interval{{StartAt: start, EndAt: start.Add(time.Hour)}}}
	service, err := NewAvailabilityService(&catalogStub{listing: listing}, calendar, clock)
	if err != nil {
		t.Fatal(err)
	}

	got, err := service.ListPublic(context.Background(), testSpaceID, "2030-05-11", 1)
	availabilityReturned := err == nil
	if !availabilityReturned {
		t.Errorf("availability with a valid first clock snapshot failed: %v", err)
	}
	if clockCalls != 1 || !got.CheckedAt.Equal(now) {
		t.Errorf("clock calls=%d checked_at=%s; want one call and %s", clockCalls, got.CheckedAt, now)
	}
	intervalMatches := len(got.Items) == 1 && got.Items[0].StartAt.Equal(start) && got.Items[0].EndAt.Equal(start.Add(time.Hour))
	if !intervalMatches {
		t.Errorf("availability=%+v; want interval evaluated at the original clock snapshot", got)
	}
	calendarReadMatches := calendar.listCalls == 1
	if !calendarReadMatches {
		t.Errorf("calendar reads=%d, want one", calendar.listCalls)
	}
	if !availabilityReturned || clockCalls != 1 || !intervalMatches || !calendarReadMatches {
		return
	}
}
