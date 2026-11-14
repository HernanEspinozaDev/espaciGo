package rentalflow

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCreateQuoteRejectsListingAtPublicPriceFloorBeforeAvailability(t *testing.T) {
	now := time.Date(2030, time.May, 10, 12, 0, 0, 0, time.UTC)
	start := time.Date(2030, time.May, 11, 15, 0, 0, 0, time.UTC)
	listing := Listing{
		ID: testSpaceID, OwnerID: testOwnerID, State: "activa", Title: "Sala sintética",
		RateUnit: "hora", UnitPriceCLP: 5000, Currency: "CLP", RateVersion: 3, TimeZone: "America/Santiago",
	}
	catalog := &catalogStub{listing: listing}
	calendar := &calendarStub{available: true}
	quotes := &quoteRepositoryStub{}
	service, err := NewQuoteService(catalog, calendar, quotes, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.Create(context.Background(), testRenterID, QuoteRequest{
		SpaceID: testSpaceID, StartAt: start.Format(time.RFC3339), EndAt: start.Add(time.Hour).Format(time.RFC3339),
	})
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("quote error=%v, want ErrUnavailable for invalid current listing", err)
	}
	if catalog.calls != 1 || calendar.checkCalls != 0 || quotes.calls != 0 {
		t.Fatalf("invalid listing reached downstream dependency: listing=%d availability=%d quote_write=%d", catalog.calls, calendar.checkCalls, quotes.calls)
	}
}
