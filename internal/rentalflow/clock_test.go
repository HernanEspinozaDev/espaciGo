package rentalflow

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRentalFlowServicesRejectZeroClockBeforeDependencyReads(t *testing.T) {
	listing := Listing{ID: testSpaceID, OwnerID: testOwnerID, State: "activa", Title: "Sala sintética", RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", RateVersion: 2, TimeZone: "America/Santiago"}
	validClockInput := func() time.Time { return time.Date(2030, time.May, 10, 12, 0, 0, 0, time.UTC) }
	t.Run("availability", func(t *testing.T) {
		catalog := &catalogStub{listing: listing}
		calendar := &calendarStub{}
		service, err := NewAvailabilityService(catalog, calendar, func() time.Time { return time.Time{} })
		if err != nil {
			t.Fatal(err)
		}
		_, err = service.ListPublic(context.Background(), testSpaceID, "2030-05-11", 1)
		if !errors.Is(err, ErrInvalid) || catalog.calls != 0 || calendar.listCalls != 0 {
			t.Fatalf("availability error=%v catalog=%d calendar=%d", err, catalog.calls, calendar.listCalls)
		}
	})

	t.Run("quote", func(t *testing.T) {
		catalog := &catalogStub{listing: listing}
		calendar := &calendarStub{available: true}
		quotes := &quoteRepositoryStub{}
		service, err := NewQuoteService(catalog, calendar, quotes, func() time.Time { return time.Time{} })
		if err != nil {
			t.Fatal(err)
		}
		_, err = service.Create(context.Background(), testRenterID, QuoteRequest{
			SpaceID: testSpaceID, StartAt: validClockInput().Add(time.Hour).Format(time.RFC3339),
			EndAt: validClockInput().Add(2 * time.Hour).Format(time.RFC3339),
		})
		if !errors.Is(err, ErrInvalid) || catalog.calls != 0 || calendar.checkCalls != 0 || quotes.calls != 0 {
			t.Fatalf("quote error=%v catalog=%d calendar=%d quote=%d", err, catalog.calls, calendar.checkCalls, quotes.calls)
		}
	})

	t.Run("contract preview", func(t *testing.T) {
		catalog := &catalogStub{listing: listing}
		calendar := &calendarStub{available: true}
		quotes := &previewQuoteRepository{}
		gate := &identityGateStub{}
		identities := &contractIdentityReaderStub{}
		service, err := NewContractPreviewService(catalog, calendar, quotes, gate, identities, func() time.Time { return time.Time{} })
		if err != nil {
			t.Fatal(err)
		}
		_, err = service.Preview(context.Background(), testRenterID, testQuoteID)
		if !errors.Is(err, ErrInvalid) || quotes.calls != 0 || catalog.calls != 0 || calendar.checkCalls != 0 || gate.calls != 0 || len(identities.calls) != 0 {
			t.Fatalf("preview error=%v quote=%d catalog=%d calendar=%d gate=%d identity_reads=%d", err, quotes.calls, catalog.calls, calendar.checkCalls, gate.calls, len(identities.calls))
		}
	})
}
