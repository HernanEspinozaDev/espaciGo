package rentalflow

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type uuidCaseListingReader struct {
	listing   Listing
	requested []string
}

func (r *uuidCaseListingReader) PublicListing(_ context.Context, spaceID string) (Listing, error) {
	r.requested = append(r.requested, spaceID)
	return r.listing, nil
}

type uuidCasePreviewQuoteRepository struct {
	quote    Quote
	actorID  string
	quoteID  string
	readCall int
}

func (*uuidCasePreviewQuoteRepository) CreateValidatedQuote(context.Context, string, Listing, Interval, int64, int64, time.Time) (Quote, error) {
	return Quote{}, errors.New("unexpected quote write")
}

func (r *uuidCasePreviewQuoteRepository) GetForParticipant(_ context.Context, actorID, quoteID string) (Quote, error) {
	r.actorID, r.quoteID = actorID, quoteID
	r.readCall++
	return r.quote, nil
}

func TestPublicAvailabilityCanonicalizesAcceptedUppercaseUUIDs(t *testing.T) {
	now := time.Date(2030, time.May, 10, 12, 0, 0, 0, time.UTC)
	start := time.Date(2030, time.May, 11, 15, 0, 0, 0, time.UTC)
	listing := Listing{
		ID: strings.ToUpper(testSpaceID), OwnerID: strings.ToUpper(testOwnerID), State: "activa",
		Title: "Sala sintética", RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP",
		RateVersion: 1, TimeZone: "America/Santiago",
	}
	catalog := &uuidCaseListingReader{listing: listing}
	calendar := &calendarStub{intervals: []Interval{{StartAt: start, EndAt: start.Add(time.Hour)}}}
	service, err := NewAvailabilityService(catalog, calendar, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}

	got, err := service.ListPublic(context.Background(), strings.ToUpper(testSpaceID), "2030-05-11", 1)
	if err != nil {
		t.Fatalf("uppercase UUID availability request failed: %v", err)
	}
	if len(catalog.requested) != 1 || catalog.requested[0] != testSpaceID || got.SpaceID != testSpaceID || len(got.Items) != 1 {
		t.Fatalf("UUID casing was not canonicalized across availability: requested=%v response=%+v", catalog.requested, got)
	}
}

func TestCreateQuoteRejectsCaseVariantSelfRentalBeforeAvailability(t *testing.T) {
	now := time.Date(2030, time.May, 10, 12, 0, 0, 0, time.UTC)
	start := time.Date(2030, time.May, 11, 15, 0, 0, 0, time.UTC)
	catalog := &uuidCaseListingReader{listing: Listing{
		ID: testSpaceID, OwnerID: testOwnerID, State: "activa", Title: "Sala sintética",
		RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", RateVersion: 1, TimeZone: "America/Santiago",
	}}
	calendar := &calendarStub{available: true}
	quotes := &quoteRepositoryStub{}
	service, err := NewQuoteService(catalog, calendar, quotes, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.Create(context.Background(), strings.ToUpper(testOwnerID), QuoteRequest{
		SpaceID: testSpaceID, StartAt: start.Format(time.RFC3339), EndAt: start.Add(time.Hour).Format(time.RFC3339),
	})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("case-variant owner self-rental error=%v, want ErrNotFound", err)
	}
	if calendar.checkCalls != 0 || quotes.calls != 0 {
		t.Fatalf("case-variant self-rental reached downstream dependencies: availability=%d quote writes=%d", calendar.checkCalls, quotes.calls)
	}
}

func TestCreateQuoteCanonicalizesUUIDsAcrossRepositoryBoundaries(t *testing.T) {
	now := time.Date(2030, time.May, 10, 12, 0, 0, 0, time.UTC)
	start := time.Date(2030, time.May, 11, 15, 0, 0, 0, time.UTC)
	listing := Listing{
		ID: strings.ToUpper(testSpaceID), OwnerID: strings.ToUpper(testOwnerID), State: "activa", Title: "Sala sintética",
		RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", RateVersion: 1, TimeZone: "America/Santiago",
	}
	catalog := &uuidCaseListingReader{listing: listing}
	calendar := &calendarStub{available: true}
	quotes := &quoteRepositoryStub{result: Quote{
		ID: strings.ToUpper(testQuoteID), SpaceID: strings.ToUpper(testSpaceID), OwnerID: strings.ToUpper(testOwnerID),
		RenterID: strings.ToUpper(testRenterID), SpaceTitle: listing.Title, RateVersion: 1, RateUnit: "hora",
		UnitPriceCLP: 8000, Currency: "CLP", Units: 1, SubtotalCLP: 8000, StartAt: start, EndAt: start.Add(time.Hour),
		TimeZone: listing.TimeZone, CreatedAt: now, ExpiresAt: now.Add(15 * time.Minute),
	}}
	service, err := NewQuoteService(catalog, calendar, quotes, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}

	got, err := service.Create(context.Background(), strings.ToUpper(testRenterID), QuoteRequest{
		SpaceID: strings.ToUpper(testSpaceID), StartAt: start.Format(time.RFC3339), EndAt: start.Add(time.Hour).Format(time.RFC3339),
	})
	if err != nil {
		t.Fatalf("uppercase UUID quote request failed: %v", err)
	}
	if catalog.requested[0] != testSpaceID || quotes.input.RenterID != testRenterID || quotes.input.SpaceID != testSpaceID ||
		got.ID != testQuoteID || got.SpaceID != testSpaceID || got.OwnerID != testOwnerID || got.RenterID != testRenterID {
		t.Fatalf("UUID casing crossed quote boundaries: catalog=%v input=%+v quote=%+v", catalog.requested, quotes.input, got)
	}
}

func TestContractPreviewCanonicalizesUUIDsBeforeParticipantAndIdentityChecks(t *testing.T) {
	now := time.Date(2030, time.May, 10, 12, 0, 0, 0, time.UTC)
	start := time.Date(2030, time.May, 11, 15, 0, 0, 0, time.UTC)
	listing := Listing{
		ID: strings.ToUpper(testSpaceID), OwnerID: strings.ToUpper(testOwnerID), State: "activa", Title: "Sala sintética",
		RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", RateVersion: 1, TimeZone: "America/Santiago",
	}
	quote := Quote{
		ID: strings.ToUpper(testQuoteID), SpaceID: strings.ToUpper(testSpaceID), OwnerID: strings.ToUpper(testOwnerID),
		RenterID: strings.ToUpper(testRenterID), SpaceTitle: listing.Title, RateVersion: 1, RateUnit: "hora",
		UnitPriceCLP: 8000, Currency: "CLP", Units: 1, SubtotalCLP: 8000, StartAt: start,
		EndAt: start.Add(time.Hour), TimeZone: listing.TimeZone, CreatedAt: now, ExpiresAt: now.Add(15 * time.Minute),
	}
	quotes := &uuidCasePreviewQuoteRepository{quote: quote}
	catalog := &uuidCaseListingReader{listing: listing}
	calendar := &calendarStub{available: true}
	gate := &identityGateStub{}
	identities := &contractIdentityReaderStub{identities: map[string]ContractIdentity{
		testOwnerID:  {SubjectID: strings.ToUpper(testOwnerID), LegalName: "Arrendador Sintético", RUT: "11.111.111-1"},
		testRenterID: {SubjectID: strings.ToUpper(testRenterID), LegalName: "Arrendataria Sintética", RUT: "22.222.222-2"},
	}}
	service, err := NewContractPreviewService(catalog, calendar, quotes, gate, identities, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}

	got, err := service.Preview(context.Background(), strings.ToUpper(testRenterID), strings.ToUpper(testQuoteID))
	if err != nil {
		t.Fatalf("uppercase UUID preview request failed: %v", err)
	}
	if quotes.actorID != testRenterID || quotes.quoteID != testQuoteID || catalog.requested[0] != testSpaceID ||
		gate.host != testOwnerID || gate.renter != testRenterID || len(identities.calls) != 2 ||
		identities.calls[0] != testRenterID+"/"+testQuoteID+"/"+testOwnerID || identities.calls[1] != testRenterID+"/"+testQuoteID+"/"+testRenterID ||
		got.QuoteID != testQuoteID || got.SpaceID != testSpaceID || got.Parties[0].LegalName != "Arrendador Sintético" {
		t.Fatalf("UUID casing crossed preview boundaries: quote=%+v catalog=%v gate=%+v identities=%v response=%+v", quotes, catalog.requested, gate, identities.calls, got)
	}
}
