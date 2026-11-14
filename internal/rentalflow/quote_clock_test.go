package rentalflow

import (
	"context"
	"testing"
	"time"
)

func TestCreateQuoteUsesOneClockSnapshotForValidationAndRepositoryWrite(t *testing.T) {
	now := time.Date(2030, time.May, 10, 12, 0, 0, 0, time.UTC)
	start := time.Date(2030, time.May, 11, 15, 0, 0, 0, time.UTC)
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
		RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", RateVersion: 3, TimeZone: "America/Santiago",
	}
	calendar := &calendarStub{available: true}
	quote := Quote{
		ID: "dddddddd-dddd-4ddd-8ddd-dddddddddddd", SpaceID: listing.ID, OwnerID: listing.OwnerID,
		RenterID: testRenterID, SpaceTitle: listing.Title, RateVersion: listing.RateVersion,
		RateUnit: listing.RateUnit, UnitPriceCLP: listing.UnitPriceCLP, Units: 1, SubtotalCLP: listing.UnitPriceCLP,
		Currency: listing.Currency, StartAt: start, EndAt: start.Add(time.Hour), TimeZone: listing.TimeZone,
		CreatedAt: now, ExpiresAt: now.Add(15 * time.Minute),
	}
	quotes := &quoteRepositoryStub{result: quote}
	service, err := NewQuoteService(&catalogStub{listing: listing}, calendar, quotes, clock)
	if err != nil {
		t.Fatal(err)
	}

	got, err := service.Create(context.Background(), testRenterID, QuoteRequest{
		SpaceID: testSpaceID, StartAt: start.Format(time.RFC3339), EndAt: start.Add(time.Hour).Format(time.RFC3339),
	})
	quoteCreated := err == nil
	if !quoteCreated {
		t.Errorf("quote with a valid first clock snapshot failed: %v", err)
	}
	clockSnapshotMatches := clockCalls == 1 && quotes.input.CreatedAt.Equal(now) && got.CreatedAt.Equal(now) && got.ExpiresAt.Equal(now.Add(15*time.Minute))
	if !clockSnapshotMatches {
		t.Errorf("clock calls=%d repository created_at=%s quote created_at=%s expires_at=%s; want one call and consistent %s snapshot", clockCalls, quotes.input.CreatedAt, got.CreatedAt, got.ExpiresAt, now)
	}
	intervalMatches := got.StartAt.Equal(start) && got.EndAt.Equal(start.Add(time.Hour))
	if !intervalMatches {
		t.Errorf("quote interval=%s..%s, want %s..%s", got.StartAt, got.EndAt, start, start.Add(time.Hour))
	}
	flowCallsMatch := calendar.checkCalls == 1 && quotes.calls == 1
	if !flowCallsMatch {
		t.Errorf("availability checks=%d quote writes=%d, want one each", calendar.checkCalls, quotes.calls)
	}
	if !quoteCreated || !clockSnapshotMatches || !intervalMatches || !flowCallsMatch {
		return
	}
}
