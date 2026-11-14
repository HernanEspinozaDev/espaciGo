package rentalflow

import (
	"context"
	"testing"
	"time"
)

func TestContractPreviewUsesOneAsOfForGateIdentitiesAndGeneratedAt(t *testing.T) {
	now := time.Date(2030, time.May, 10, 12, 0, 0, 0, time.UTC)
	start := time.Date(2030, time.May, 11, 15, 0, 0, 0, time.UTC)
	listing := Listing{ID: testSpaceID, OwnerID: testOwnerID, State: "activa", Title: "Sala sintética", RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", RateVersion: 7, TimeZone: "America/Santiago"}
	quote := Quote{ID: testQuoteID, SpaceID: testSpaceID, OwnerID: testOwnerID, RenterID: testRenterID, SpaceTitle: listing.Title, RateVersion: listing.RateVersion, RateUnit: listing.RateUnit, UnitPriceCLP: listing.UnitPriceCLP, Currency: "CLP", Units: 1, SubtotalCLP: 8000, StartAt: start, EndAt: start.Add(time.Hour), TimeZone: listing.TimeZone, CreatedAt: now.Add(-time.Minute), ExpiresAt: now.Add(5 * time.Minute)}
	quotes := &previewQuoteRepository{quote: quote}
	gate := &identityGateStub{}
	identities := &contractIdentityReaderStub{identities: map[string]ContractIdentity{
		testOwnerID:  {SubjectID: testOwnerID, LegalName: "Arrendador Sintético", RUT: "11.111.111-1"},
		testRenterID: {SubjectID: testRenterID, LegalName: "Arrendataria Sintética", RUT: "22.222.222-2"},
	}}
	clockCalls := 0
	clock := func() time.Time {
		clockCalls++
		if clockCalls == 1 {
			return now
		}
		return now.Add(time.Minute)
	}
	service, err := NewContractPreviewService(&catalogStub{listing: listing}, &calendarStub{available: true}, quotes, gate, identities, clock)
	if err != nil {
		t.Fatal(err)
	}

	preview, err := service.Preview(context.Background(), testRenterID, testQuoteID)
	if err != nil {
		t.Fatalf("synthetic preview failed: %v", err)
	}
	if clockCalls != 1 || !preview.GeneratedAt.Equal(now) {
		t.Fatalf("preview clock/generated_at mismatch: calls=%d generated_at=%s want=%s", clockCalls, preview.GeneratedAt, now)
	}
	if len(gate.asOf) != 1 || !gate.asOf[0].Equal(now) {
		t.Fatalf("identity gate as_of=%v, want exactly %s", gate.asOf, now)
	}
	if len(identities.asOf) != 2 || !identities.asOf[0].Equal(now) || !identities.asOf[1].Equal(now) {
		t.Fatalf("legal identity read as_of values=%v, want both %s", identities.asOf, now)
	}
}
