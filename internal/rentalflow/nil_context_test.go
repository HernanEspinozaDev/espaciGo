package rentalflow

import (
	"errors"
	"testing"
	"time"
)

func TestRentalFlowServicesRejectNilContextBeforeDependencies(t *testing.T) {
	now := time.Date(2030, time.May, 10, 12, 0, 0, 0, time.UTC)
	start := time.Date(2030, time.May, 11, 15, 0, 0, 0, time.UTC)
	listing := Listing{ID: testSpaceID, OwnerID: testOwnerID, State: "activa", Title: "Sala sintética", RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", RateVersion: 3, TimeZone: "America/Santiago"}
	catalog := &catalogStub{listing: listing}
	calendar := &calendarStub{available: true}
	quotes := &quoteRepositoryStub{}
	previewQuotes := &previewQuoteRepository{}
	gate := &identityGateStub{}
	identities := &contractIdentityReaderStub{}
	clock := func() time.Time { return now }

	availability, err := NewAvailabilityService(catalog, calendar, clock)
	if err != nil {
		t.Fatal(err)
	}
	quote, err := NewQuoteService(catalog, calendar, quotes, clock)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := NewContractPreviewService(catalog, calendar, previewQuotes, gate, identities, clock)
	if err != nil {
		t.Fatal(err)
	}

	calls := []struct {
		name string
		run  func() error
	}{
		{name: "availability", run: func() error {
			_, err := availability.ListPublic(nil, testSpaceID, "2030-05-11", 1)
			return err
		}},
		{name: "quote", run: func() error {
			_, err := quote.Create(nil, testRenterID, QuoteRequest{SpaceID: testSpaceID, StartAt: start.Format(time.RFC3339), EndAt: start.Add(time.Hour).Format(time.RFC3339)})
			return err
		}},
		{name: "contract preview", run: func() error {
			_, err := preview.Preview(nil, testRenterID, testQuoteID)
			return err
		}},
	}
	for _, tc := range calls {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.run(); !errors.Is(err, ErrInvalid) {
				t.Fatalf("nil context error=%v, want ErrInvalid", err)
			}
		})
	}

	if catalog.calls != 0 || calendar.listCalls != 0 || calendar.checkCalls != 0 || quotes.calls != 0 || previewQuotes.calls != 0 || gate.calls != 0 || len(identities.calls) != 0 {
		t.Fatalf("nil context reached dependencies: listing=%d public_intervals=%d availability=%d quote_write=%d quote_read=%d identity_gate=%d legal_identity=%d", catalog.calls, calendar.listCalls, calendar.checkCalls, quotes.calls, previewQuotes.calls, gate.calls, len(identities.calls))
	}
}
