package rentalflow

import (
	"context"
	"errors"
	"testing"
	"time"
)

func FuzzContractPreviewExpiryUsesNanosecondBoundary(f *testing.F) {
	for _, offset := range []uint16{0, 32767, 32768, 32769, 65535} {
		f.Add(offset)
	}
	f.Fuzz(func(t *testing.T, offset uint16) {
		delta := time.Duration(int64(offset) - 32768)
		now := time.Date(2030, time.May, 10, 12, 0, 0, 0, time.UTC)
		start := time.Date(2030, time.May, 11, 15, 0, 0, 0, time.UTC)
		listing := Listing{ID: testSpaceID, OwnerID: testOwnerID, State: "activa", Title: "Sala sintética", RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", RateVersion: 7, TimeZone: "America/Santiago"}
		quote := Quote{ID: testQuoteID, SpaceID: testSpaceID, OwnerID: testOwnerID, RenterID: testRenterID, SpaceTitle: listing.Title, RateVersion: listing.RateVersion, RateUnit: listing.RateUnit, UnitPriceCLP: listing.UnitPriceCLP, Currency: "CLP", Units: 1, SubtotalCLP: 8000, StartAt: start, EndAt: start.Add(time.Hour), TimeZone: listing.TimeZone, CreatedAt: now.Add(-time.Minute), ExpiresAt: now.Add(delta)}
		quotes := &previewQuoteRepository{quote: quote}
		catalog := &catalogStub{listing: listing}
		calendar := &calendarStub{available: true}
		gate := &identityGateStub{}
		identities := &contractIdentityReaderStub{identities: map[string]ContractIdentity{
			testOwnerID:  {SubjectID: testOwnerID, LegalName: "Arrendador Sintético", RUT: "11.111.111-1"},
			testRenterID: {SubjectID: testRenterID, LegalName: "Arrendataria Sintética", RUT: "22.222.222-2"},
		}}
		service, err := NewContractPreviewService(catalog, calendar, quotes, gate, identities, func() time.Time { return now })
		if err != nil {
			t.Fatal(err)
		}

		preview, err := service.Preview(context.Background(), testRenterID, testQuoteID)
		if delta <= 0 {
			if !errors.Is(err, ErrConflict) || preview.State != "" {
				t.Fatalf("expired quote delta=%s preview=%+v err=%v; want conflict without preview", delta, preview, err)
			}
			if quotes.calls != 1 || catalog.calls != 0 || calendar.checkCalls != 0 || gate.calls != 0 || len(identities.calls) != 0 {
				t.Fatalf("expired quote reached later stages: quote=%d listing=%d calendar=%d gate=%d identities=%d", quotes.calls, catalog.calls, calendar.checkCalls, gate.calls, len(identities.calls))
			}
			return
		}
		if err != nil || preview.State != "preview" {
			t.Fatalf("future quote delta=%s preview=%+v err=%v", delta, preview, err)
		}
		if quotes.calls != 1 || catalog.calls != 1 || calendar.checkCalls != 1 || gate.calls != 1 || len(identities.calls) != 2 {
			t.Fatalf("live quote flow calls: quote=%d listing=%d calendar=%d gate=%d identities=%d", quotes.calls, catalog.calls, calendar.checkCalls, gate.calls, len(identities.calls))
		}
	})
}
