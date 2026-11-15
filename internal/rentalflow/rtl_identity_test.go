package rentalflow

import (
	"context"
	"testing"
	"time"
)

func TestContractPreviewAllowsRTLLegalNameWithoutBidiOrSeparators(t *testing.T) {
	now := time.Date(2030, time.May, 10, 12, 0, 0, 0, time.UTC)
	start := time.Date(2030, time.May, 11, 15, 0, 0, 0, time.UTC)
	ownerLegalName := "المالك"
	listing := Listing{ID: testSpaceID, OwnerID: testOwnerID, State: "activa", Title: "Sala sintética", RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", RateVersion: 7, TimeZone: "America/Santiago"}
	quote := Quote{ID: testQuoteID, SpaceID: testSpaceID, OwnerID: testOwnerID, RenterID: testRenterID, SpaceTitle: listing.Title, RateVersion: listing.RateVersion, RateUnit: listing.RateUnit, UnitPriceCLP: listing.UnitPriceCLP, Currency: "CLP", Units: 1, SubtotalCLP: 8000, StartAt: start, EndAt: start.Add(time.Hour), TimeZone: listing.TimeZone, CreatedAt: now.Add(-time.Minute), ExpiresAt: now.Add(5 * time.Minute)}
	identities := &contractIdentityReaderStub{identities: map[string]ContractIdentity{
		testOwnerID:  {SubjectID: testOwnerID, LegalName: ownerLegalName, RUT: "11.111.111-1"},
		testRenterID: {SubjectID: testRenterID, LegalName: "Arrendataria Sintética", RUT: "22.222.222-2"},
	}}
	service, err := NewContractPreviewService(
		&catalogStub{listing: listing},
		&calendarStub{available: true},
		&previewQuoteRepository{quote: quote},
		&identityGateStub{}, identities,
		func() time.Time { return now },
	)
	if err != nil {
		t.Fatal(err)
	}

	preview, err := service.Preview(context.Background(), testRenterID, testQuoteID)
	if err != nil {
		t.Fatalf("ordinary RTL legal name was rejected: %v", err)
	}
	if len(preview.Parties) != 2 || preview.Parties[0].LegalName != ownerLegalName {
		t.Fatalf("RTL legal name was not preserved in the synthetic preview: %+v", preview.Parties)
	}
}
