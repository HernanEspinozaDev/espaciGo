package rentalflow

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestContractPreviewRejectsIdentityOutsideRequestedSubjectScope(t *testing.T) {
	now := time.Date(2030, time.May, 10, 12, 0, 0, 0, time.UTC)
	start := time.Date(2030, time.May, 11, 15, 0, 0, 0, time.UTC)
	listing := Listing{ID: testSpaceID, OwnerID: testOwnerID, State: "activa", Title: "Sala sintética", RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", RateVersion: 7, TimeZone: "America/Santiago"}
	quote := Quote{ID: testQuoteID, SpaceID: testSpaceID, OwnerID: testOwnerID, RenterID: testRenterID, SpaceTitle: listing.Title, RateVersion: 7, RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", Units: 1, SubtotalCLP: 8000, StartAt: start, EndAt: start.Add(time.Hour), TimeZone: listing.TimeZone, CreatedAt: now.Add(-time.Minute), ExpiresAt: now.Add(5 * time.Minute)}
	identities := &contractIdentityReaderStub{identities: map[string]ContractIdentity{
		testOwnerID:  {SubjectID: testRenterID, LegalName: "Identidad de otra persona", RUT: "22.222.222-2"},
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

	_, err = service.Preview(context.Background(), testRenterID, testQuoteID)
	if !errors.Is(err, ErrIdentityUnavailable) {
		t.Fatalf("mismatched identity subject error=%v, want ErrIdentityUnavailable", err)
	}
	if len(identities.calls) != 1 || identities.calls[0] != testRenterID+"/"+testQuoteID+"/"+testOwnerID || len(identities.asOf) != 1 || !identities.asOf[0].Equal(now) {
		t.Fatalf("identity read was not scoped to requester/quote/owner at one as-of instant: calls=%v asOf=%v", identities.calls, identities.asOf)
	}
}
