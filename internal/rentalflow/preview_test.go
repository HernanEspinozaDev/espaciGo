package rentalflow

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/identityverification"
)

const testQuoteID = "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee"

type previewQuoteRepository struct {
	quote  Quote
	err    error
	calls  int
	onRead func()
}

func (s *previewQuoteRepository) CreateValidatedQuote(context.Context, string, Listing, Interval, int64, int64, time.Time) (Quote, error) {
	return Quote{}, errors.New("unexpected quote write")
}

func (s *previewQuoteRepository) GetForParticipant(context.Context, string, string) (Quote, error) {
	s.calls++
	if s.onRead != nil {
		s.onRead()
	}
	return s.quote, s.err
}

type identityGateStub struct {
	err       error
	calls     int
	host      string
	renter    string
	asOf      []time.Time
	onRequire func()
}

func (s *identityGateStub) RequireForBooking(_ context.Context, hostID, renterID string, asOf time.Time) error {
	s.calls++
	s.host, s.renter = hostID, renterID
	s.asOf = append(s.asOf, asOf)
	if s.onRequire != nil {
		s.onRequire()
	}
	return s.err
}

type contractIdentityReaderStub struct {
	identities   map[string]ContractIdentity
	calls        []string
	asOf         []time.Time
	err          error
	errBySubject map[string]error
	onRead       func()
}

func TestContractPreviewDoesNotReadQuoteOrIdentityForCanceledContext(t *testing.T) {
	now := time.Date(2030, time.May, 10, 12, 0, 0, 0, time.UTC)
	catalog := &catalogStub{}
	calendar := &calendarStub{}
	quotes := &previewQuoteRepository{}
	gate := &identityGateStub{}
	identities := &contractIdentityReaderStub{}
	service, err := NewContractPreviewService(catalog, calendar, quotes, gate, identities, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = service.Preview(ctx, testRenterID, testQuoteID)
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("canceled request error=%v, want ErrUnavailable", err)
	}
	if quotes.calls != 0 || catalog.calls != 0 || calendar.checkCalls != 0 || gate.calls != 0 || len(identities.calls) != 0 {
		t.Fatalf("canceled request started work: quote=%d listing=%d calendar=%d gate=%d legal_identity=%d", quotes.calls, catalog.calls, calendar.checkCalls, gate.calls, len(identities.calls))
	}
}

func TestContractPreviewRejectsMalformedActorOrQuoteBeforeDependencyReads(t *testing.T) {
	cases := []struct {
		name    string
		actorID string
		quoteID string
	}{
		{name: "empty actor", actorID: "", quoteID: testQuoteID},
		{name: "malformed actor", actorID: "not-a-uuid", quoteID: testQuoteID},
		{name: "malformed quote", actorID: testRenterID, quoteID: "not-a-uuid"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Date(2030, time.May, 10, 12, 0, 0, 0, time.UTC)
			quotes := &previewQuoteRepository{}
			catalog := &catalogStub{}
			calendar := &calendarStub{}
			gate := &identityGateStub{}
			identities := &contractIdentityReaderStub{}
			service, err := NewContractPreviewService(catalog, calendar, quotes, gate, identities, func() time.Time { return now })
			if err != nil {
				t.Fatal(err)
			}

			_, err = service.Preview(context.Background(), tc.actorID, tc.quoteID)
			if !errors.Is(err, ErrInvalid) {
				t.Fatalf("malformed preview identifiers error=%v, want ErrInvalid", err)
			}
			if quotes.calls != 0 || catalog.calls != 0 || calendar.checkCalls != 0 || gate.calls != 0 || len(identities.calls) != 0 {
				t.Fatalf("malformed identifiers reached dependencies: quote=%d listing=%d calendar=%d gate=%d identity=%d", quotes.calls, catalog.calls, calendar.checkCalls, gate.calls, len(identities.calls))
			}
		})
	}
}

func TestContractPreviewHidesMissingQuoteBeforePublicationOrIdentityReads(t *testing.T) {
	now := time.Date(2030, time.May, 10, 12, 0, 0, 0, time.UTC)
	catalog := &catalogStub{}
	calendar := &calendarStub{}
	quotes := &previewQuoteRepository{
		quote: Quote{ID: testQuoteID, SpaceID: testSpaceID, OwnerID: testOwnerID, RenterID: testRenterID, SpaceTitle: "partial-missing-quote-secret", SubtotalCLP: 987654},
		err:   ErrNotFound,
	}
	gate := &identityGateStub{}
	identities := &contractIdentityReaderStub{}
	service, err := NewContractPreviewService(catalog, calendar, quotes, gate, identities, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}

	got, err := service.Preview(context.Background(), testRenterID, testQuoteID)
	missingQuoteMapped := errors.Is(err, ErrNotFound)
	if !missingQuoteMapped {
		t.Errorf("missing quote error=%v, want ErrNotFound", err)
	}
	previewDiscarded := contractPreviewIsEmpty(got)
	if !previewDiscarded {
		t.Errorf("missing quote returned partial contract preview: %+v", got)
	}
	followUpReadsStopped := quotes.calls == 1 && catalog.calls == 0 && calendar.checkCalls == 0 && gate.calls == 0 && len(identities.calls) == 0
	if !followUpReadsStopped {
		t.Errorf("missing quote caused follow-up reads: quote=%d listing=%d calendar=%d gate=%d legal_identity=%d", quotes.calls, catalog.calls, calendar.checkCalls, gate.calls, len(identities.calls))
	}
	if !missingQuoteMapped || !previewDiscarded || !followUpReadsStopped {
		return
	}
}

func TestContractPreviewHidesMismatchedRepositoryQuoteIDBeforePrivateReads(t *testing.T) {
	now := time.Date(2030, time.May, 10, 12, 0, 0, 0, time.UTC)
	start := time.Date(2030, time.May, 11, 15, 0, 0, 0, time.UTC)
	listing := Listing{ID: testSpaceID, OwnerID: testOwnerID, State: "activa", Title: "Sala sintética", RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", RateVersion: 7, TimeZone: "America/Santiago"}
	quote := Quote{ID: "ffffffff-ffff-4fff-8fff-ffffffffffff", SpaceID: testSpaceID, OwnerID: testOwnerID, RenterID: testRenterID, SpaceTitle: listing.Title, RateVersion: listing.RateVersion, RateUnit: listing.RateUnit, UnitPriceCLP: listing.UnitPriceCLP, Currency: listing.Currency, Units: 1, SubtotalCLP: 8000, StartAt: start, EndAt: start.Add(time.Hour), TimeZone: listing.TimeZone, CreatedAt: now.Add(-time.Minute), ExpiresAt: now.Add(5 * time.Minute)}
	quotes := &previewQuoteRepository{quote: quote}
	catalog := &catalogStub{listing: listing}
	calendar := &calendarStub{available: true}
	gate := &identityGateStub{}
	identities := &contractIdentityReaderStub{}
	service, err := NewContractPreviewService(catalog, calendar, quotes, gate, identities, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}

	got, err := service.Preview(context.Background(), testRenterID, testQuoteID)
	quoteMismatchMapped := errors.Is(err, ErrNotFound)
	if !quoteMismatchMapped {
		t.Errorf("mismatched repository quote ID error=%v, want ErrNotFound", err)
	}
	previewDiscarded := contractPreviewIsEmpty(got)
	if !previewDiscarded {
		t.Errorf("mismatched quote returned partial contract preview: %+v", got)
	}
	privateReadsStopped := quotes.calls == 1 && catalog.calls == 0 && calendar.checkCalls == 0 && gate.calls == 0 && len(identities.calls) == 0
	if !privateReadsStopped {
		t.Errorf("mismatched repository ID caused private reads: quote=%d listing=%d calendar=%d gate=%d identity=%d", quotes.calls, catalog.calls, calendar.checkCalls, gate.calls, len(identities.calls))
	}
	if !quoteMismatchMapped || !previewDiscarded || !privateReadsStopped {
		return
	}
}

func contractPreviewIsEmpty(preview ContractPreview) bool {
	return preview.State == "" && preview.QuoteID == "" && preview.SpaceID == "" && preview.SpaceTitle == "" &&
		preview.RateUnit == "" && preview.UnitPriceCLP == 0 && preview.Units == 0 && preview.SubtotalCLP == 0 &&
		preview.Currency == "" && preview.StartAt.IsZero() && preview.EndAt.IsZero() && preview.TimeZone == "" &&
		len(preview.Parties) == 0 && preview.GeneratedAt.IsZero()
}

func TestContractPreviewStopsAtExpiredOrUnavailableDependencies(t *testing.T) {
	now := time.Date(2030, time.May, 10, 12, 0, 0, 0, time.UTC)
	start := time.Date(2030, time.May, 11, 15, 0, 0, 0, time.UTC)
	listing := Listing{ID: testSpaceID, OwnerID: testOwnerID, State: "activa", Title: "Sala sintética", RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", RateVersion: 7, TimeZone: "America/Santiago"}
	cases := []struct {
		name                 string
		quoteErr, listingErr error
		gateErr              error
		mutateListing        func(*Listing)
		expired              bool
		wantErr              error
		wantListings         int
		wantCalendar         int
		wantGate             int
	}{
		{name: "expired quote", expired: true, wantErr: ErrConflict},
		{name: "quote reader unavailable", quoteErr: errors.New("private quote store failure"), wantErr: ErrUnavailable},
		{name: "listing disappeared", listingErr: ErrNotFound, wantErr: ErrConflict, wantListings: 1},
		{name: "malformed listing time zone", mutateListing: func(v *Listing) { v.TimeZone = "Local" }, wantErr: ErrConflict, wantListings: 1},
		{name: "listing title with bidi control", mutateListing: func(v *Listing) { v.Title = "Sala\u202eprivada" }, wantErr: ErrConflict, wantListings: 1},
		{name: "listing title with invalid UTF-8", mutateListing: func(v *Listing) { v.Title = string([]byte{'S', 'a', 'l', 'a', 0xff}) }, wantErr: ErrConflict, wantListings: 1},
		{name: "identity gate unavailable", gateErr: errors.New("private verification failure"), wantErr: ErrIdentityUnavailable, wantListings: 1, wantCalendar: 1, wantGate: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			quote := Quote{ID: testQuoteID, SpaceID: testSpaceID, OwnerID: testOwnerID, RenterID: testRenterID, SpaceTitle: listing.Title, RateVersion: listing.RateVersion, RateUnit: listing.RateUnit, UnitPriceCLP: listing.UnitPriceCLP, Currency: "CLP", Units: 1, SubtotalCLP: listing.UnitPriceCLP, StartAt: start, EndAt: start.Add(time.Hour), TimeZone: listing.TimeZone, CreatedAt: now.Add(-time.Minute), ExpiresAt: now.Add(5 * time.Minute)}
			if tc.expired {
				quote.ExpiresAt = now
			}
			quotes := &previewQuoteRepository{quote: quote, err: tc.quoteErr}
			listingSnapshot := listing
			if tc.mutateListing != nil {
				tc.mutateListing(&listingSnapshot)
			}
			catalog := &catalogStub{listing: listingSnapshot, err: tc.listingErr}
			calendar := &calendarStub{available: true}
			gate := &identityGateStub{err: tc.gateErr}
			identities := &contractIdentityReaderStub{}
			service, err := NewContractPreviewService(catalog, calendar, quotes, gate, identities, func() time.Time { return now })
			if err != nil {
				t.Fatal(err)
			}

			got, err := service.Preview(context.Background(), testRenterID, testQuoteID)
			errorMatches := errors.Is(err, tc.wantErr)
			if !errorMatches {
				t.Errorf("preview error=%v, want %v", err, tc.wantErr)
			}
			previewDiscarded := contractPreviewIsEmpty(got)
			if !previewDiscarded {
				t.Errorf("failed preview returned partial data: %+v", got)
			}
			dependenciesStopped := catalog.calls == tc.wantListings && calendar.checkCalls == tc.wantCalendar && gate.calls == tc.wantGate && len(identities.calls) == 0
			if !dependenciesStopped {
				t.Errorf("unexpected downstream calls: listing=%d calendar=%d gate=%d identity=%d", catalog.calls, calendar.checkCalls, gate.calls, len(identities.calls))
			}
			if !errorMatches || !previewDiscarded || !dependenciesStopped {
				return
			}
		})
	}
}

func TestContractPreviewAcceptsQuoteOneNanosecondBeforeExpiry(t *testing.T) {
	now := time.Date(2030, time.May, 10, 12, 0, 0, 0, time.UTC)
	start := time.Date(2030, time.May, 11, 15, 0, 0, 0, time.UTC)
	listing := Listing{ID: testSpaceID, OwnerID: testOwnerID, State: "activa", Title: "Sala sintética", RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", RateVersion: 7, TimeZone: "America/Santiago"}
	quote := Quote{ID: testQuoteID, SpaceID: testSpaceID, OwnerID: testOwnerID, RenterID: testRenterID, SpaceTitle: listing.Title, RateVersion: listing.RateVersion, RateUnit: listing.RateUnit, UnitPriceCLP: listing.UnitPriceCLP, Currency: "CLP", Units: 1, SubtotalCLP: 8000, StartAt: start, EndAt: start.Add(time.Hour), TimeZone: listing.TimeZone, CreatedAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Nanosecond)}
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
	if err != nil || preview.State != "preview" {
		t.Fatalf("preview at one nanosecond before expiry=%+v err=%v", preview, err)
	}
	if quotes.calls != 1 || catalog.calls != 1 || calendar.checkCalls != 1 || gate.calls != 1 || len(identities.calls) != 2 {
		t.Fatalf("valid boundary preview calls: quote=%d listing=%d calendar=%d gate=%d identities=%d", quotes.calls, catalog.calls, calendar.checkCalls, gate.calls, len(identities.calls))
	}
}

func TestContractPreviewStopsBeforeListingWhenContextIsCanceledByQuoteRead(t *testing.T) {
	now := time.Date(2030, time.May, 10, 12, 0, 0, 0, time.UTC)
	start := time.Date(2030, time.May, 11, 15, 0, 0, 0, time.UTC)
	listing := Listing{ID: testSpaceID, OwnerID: testOwnerID, State: "activa", Title: "Sala sintética", RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", RateVersion: 7, TimeZone: "America/Santiago"}
	quote := Quote{ID: testQuoteID, SpaceID: testSpaceID, OwnerID: testOwnerID, RenterID: testRenterID, SpaceTitle: listing.Title, RateVersion: listing.RateVersion, RateUnit: listing.RateUnit, UnitPriceCLP: listing.UnitPriceCLP, Currency: "CLP", Units: 1, SubtotalCLP: listing.UnitPriceCLP, StartAt: start, EndAt: start.Add(time.Hour), TimeZone: listing.TimeZone, CreatedAt: now.Add(-time.Minute), ExpiresAt: now.Add(5 * time.Minute)}
	ctx, cancel := context.WithCancel(context.Background())
	quotes := &previewQuoteRepository{quote: quote, onRead: cancel}
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

	_, err = service.Preview(ctx, testRenterID, testQuoteID)
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("canceled preview error=%v, want ErrUnavailable", err)
	}
	if quotes.calls != 1 || catalog.calls != 0 || calendar.checkCalls != 0 || gate.calls != 0 || len(identities.calls) != 0 {
		t.Fatalf("canceled preview continued after quote read: quote=%d listing=%d calendar=%d gate=%d legal_identity=%d", quotes.calls, catalog.calls, calendar.checkCalls, gate.calls, len(identities.calls))
	}
}

func TestContractPreviewMapsCanceledNotFoundReadsAsUnavailable(t *testing.T) {
	now := time.Date(2030, time.May, 10, 12, 0, 0, 0, time.UTC)
	start := time.Date(2030, time.May, 11, 15, 0, 0, 0, time.UTC)
	listing := Listing{ID: testSpaceID, OwnerID: testOwnerID, State: "activa", Title: "Sala sintética", RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", RateVersion: 7, TimeZone: "America/Santiago"}
	cases := []struct {
		name            string
		cancelAt        string
		wantListingRead int
	}{
		{name: "quote lookup", cancelAt: "quote"},
		{name: "listing lookup", cancelAt: "listing", wantListingRead: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			quotes := &previewQuoteRepository{}
			catalog := &catalogStub{listing: listing}
			if tc.cancelAt == "quote" {
				quotes.err = ErrNotFound
				quotes.onRead = cancel
			} else {
				quotes.quote = Quote{ID: testQuoteID, SpaceID: testSpaceID, OwnerID: testOwnerID, RenterID: testRenterID, SpaceTitle: listing.Title, RateVersion: listing.RateVersion, RateUnit: listing.RateUnit, UnitPriceCLP: 8000, Currency: "CLP", Units: 1, SubtotalCLP: 8000, StartAt: start, EndAt: start.Add(time.Hour), TimeZone: listing.TimeZone, CreatedAt: now.Add(-time.Minute), ExpiresAt: now.Add(5 * time.Minute)}
				catalog.err = ErrNotFound
				catalog.onRead = cancel
			}
			calendar := &calendarStub{available: true}
			gate := &identityGateStub{}
			identities := &contractIdentityReaderStub{}
			service, err := NewContractPreviewService(catalog, calendar, quotes, gate, identities, func() time.Time { return now })
			if err != nil {
				t.Fatal(err)
			}

			_, err = service.Preview(ctx, testRenterID, testQuoteID)
			if !errors.Is(err, ErrUnavailable) {
				t.Fatalf("canceled %s read error=%v, want ErrUnavailable", tc.cancelAt, err)
			}
			if quotes.calls != 1 || catalog.calls != tc.wantListingRead || calendar.checkCalls != 0 || gate.calls != 0 || len(identities.calls) != 0 {
				t.Fatalf("canceled read continued: quote=%d listing=%d calendar=%d gate=%d legal_identity=%d", quotes.calls, catalog.calls, calendar.checkCalls, gate.calls, len(identities.calls))
			}
		})
	}
}

func TestContractPreviewStopsBeforeCalendarWhenContextIsCanceledByListingRead(t *testing.T) {
	now := time.Date(2030, time.May, 10, 12, 0, 0, 0, time.UTC)
	start := time.Date(2030, time.May, 11, 15, 0, 0, 0, time.UTC)
	listing := Listing{ID: testSpaceID, OwnerID: testOwnerID, State: "activa", Title: "Sala sintética", RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", RateVersion: 7, TimeZone: "America/Santiago"}
	quote := Quote{ID: testQuoteID, SpaceID: testSpaceID, OwnerID: testOwnerID, RenterID: testRenterID, SpaceTitle: listing.Title, RateVersion: listing.RateVersion, RateUnit: listing.RateUnit, UnitPriceCLP: listing.UnitPriceCLP, Currency: "CLP", Units: 1, SubtotalCLP: listing.UnitPriceCLP, StartAt: start, EndAt: start.Add(time.Hour), TimeZone: listing.TimeZone, CreatedAt: now.Add(-time.Minute), ExpiresAt: now.Add(5 * time.Minute)}
	ctx, cancel := context.WithCancel(context.Background())
	quotes := &previewQuoteRepository{quote: quote}
	catalog := &catalogStub{listing: listing, onRead: cancel}
	calendar := &calendarStub{available: true}
	gate := &identityGateStub{}
	identities := &contractIdentityReaderStub{}
	service, err := NewContractPreviewService(catalog, calendar, quotes, gate, identities, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.Preview(ctx, testRenterID, testQuoteID)
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("canceled preview error=%v, want ErrUnavailable", err)
	}
	if catalog.calls != 1 || calendar.checkCalls != 0 || gate.calls != 0 || len(identities.calls) != 0 {
		t.Fatalf("canceled preview continued after listing read: listing=%d calendar=%d gate=%d legal_identity=%d", catalog.calls, calendar.checkCalls, gate.calls, len(identities.calls))
	}
}

func TestContractPreviewStopsBeforeIdentityGateWhenContextIsCanceledByCalendarRead(t *testing.T) {
	now := time.Date(2030, time.May, 10, 12, 0, 0, 0, time.UTC)
	start := time.Date(2030, time.May, 11, 15, 0, 0, 0, time.UTC)
	listing := Listing{ID: testSpaceID, OwnerID: testOwnerID, State: "activa", Title: "Sala sintética", RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", RateVersion: 7, TimeZone: "America/Santiago"}
	quote := Quote{ID: testQuoteID, SpaceID: testSpaceID, OwnerID: testOwnerID, RenterID: testRenterID, SpaceTitle: listing.Title, RateVersion: listing.RateVersion, RateUnit: listing.RateUnit, UnitPriceCLP: listing.UnitPriceCLP, Currency: "CLP", Units: 1, SubtotalCLP: listing.UnitPriceCLP, StartAt: start, EndAt: start.Add(time.Hour), TimeZone: listing.TimeZone, CreatedAt: now.Add(-time.Minute), ExpiresAt: now.Add(5 * time.Minute)}
	ctx, cancel := context.WithCancel(context.Background())
	quotes := &previewQuoteRepository{quote: quote}
	catalog := &catalogStub{listing: listing}
	calendar := &calendarStub{available: true, onCheck: cancel}
	gate := &identityGateStub{}
	identities := &contractIdentityReaderStub{}
	service, err := NewContractPreviewService(catalog, calendar, quotes, gate, identities, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.Preview(ctx, testRenterID, testQuoteID)
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("canceled preview error=%v, want ErrUnavailable", err)
	}
	if calendar.checkCalls != 1 || gate.calls != 0 || len(identities.calls) != 0 {
		t.Fatalf("canceled preview reached identity gate: calendar=%d gate=%d legal_identity=%d", calendar.checkCalls, gate.calls, len(identities.calls))
	}
}

func TestContractPreviewDoesNotReadLegalIdentityWhenContextIsCanceledByVerificationGate(t *testing.T) {
	now := time.Date(2030, time.May, 10, 12, 0, 0, 0, time.UTC)
	start := time.Date(2030, time.May, 11, 15, 0, 0, 0, time.UTC)
	listing := Listing{ID: testSpaceID, OwnerID: testOwnerID, State: "activa", Title: "Sala sintética", RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", RateVersion: 7, TimeZone: "America/Santiago"}
	quote := Quote{ID: testQuoteID, SpaceID: testSpaceID, OwnerID: testOwnerID, RenterID: testRenterID, SpaceTitle: listing.Title, RateVersion: listing.RateVersion, RateUnit: listing.RateUnit, UnitPriceCLP: listing.UnitPriceCLP, Currency: "CLP", Units: 1, SubtotalCLP: listing.UnitPriceCLP, StartAt: start, EndAt: start.Add(time.Hour), TimeZone: listing.TimeZone, CreatedAt: now.Add(-time.Minute), ExpiresAt: now.Add(5 * time.Minute)}
	ctx, cancel := context.WithCancel(context.Background())
	quotes := &previewQuoteRepository{quote: quote}
	calendar := &calendarStub{available: true}
	gate := &identityGateStub{onRequire: cancel}
	identities := &contractIdentityReaderStub{identities: map[string]ContractIdentity{
		testOwnerID:  {SubjectID: testOwnerID, LegalName: "Arrendador Sintético", RUT: "11.111.111-1"},
		testRenterID: {SubjectID: testRenterID, LegalName: "Arrendataria Sintética", RUT: "22.222.222-2"},
	}}
	service, err := NewContractPreviewService(&catalogStub{listing: listing}, calendar, quotes, gate, identities, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.Preview(ctx, testRenterID, testQuoteID)
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("canceled preview error=%v, want ErrUnavailable", err)
	}
	if gate.calls != 1 || len(identities.calls) != 0 {
		t.Fatalf("canceled preview read legal identity after gate: gate=%d legal_identity=%d", gate.calls, len(identities.calls))
	}
}

func TestContractPreviewClassifiesVerificationGateCancellationAsUnavailable(t *testing.T) {
	now := time.Date(2030, time.May, 10, 12, 0, 0, 0, time.UTC)
	start := time.Date(2030, time.May, 11, 15, 0, 0, 0, time.UTC)
	listing := Listing{ID: testSpaceID, OwnerID: testOwnerID, State: "activa", Title: "Sala sintética", RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", RateVersion: 7, TimeZone: "America/Santiago"}
	quote := Quote{ID: testQuoteID, SpaceID: testSpaceID, OwnerID: testOwnerID, RenterID: testRenterID, SpaceTitle: listing.Title, RateVersion: listing.RateVersion, RateUnit: listing.RateUnit, UnitPriceCLP: listing.UnitPriceCLP, Currency: "CLP", Units: 1, SubtotalCLP: 8000, StartAt: start, EndAt: start.Add(time.Hour), TimeZone: listing.TimeZone, CreatedAt: now.Add(-time.Minute), ExpiresAt: now.Add(5 * time.Minute)}
	ctx, cancel := context.WithCancel(context.Background())
	gate := &identityGateStub{err: context.Canceled, onRequire: cancel}
	identities := &contractIdentityReaderStub{}
	service, err := NewContractPreviewService(&catalogStub{listing: listing}, &calendarStub{available: true}, &previewQuoteRepository{quote: quote}, gate, identities, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.Preview(ctx, testRenterID, testQuoteID)
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("canceled identity gate error=%v, want ErrUnavailable", err)
	}
	if gate.calls != 1 || len(identities.calls) != 0 {
		t.Fatalf("canceled identity gate continued legal reads: gate=%d legal_identity=%d", gate.calls, len(identities.calls))
	}
}

func TestContractPreviewPrefersCanceledContextToNotVerifiedGateResult(t *testing.T) {
	now := time.Date(2030, time.May, 10, 12, 0, 0, 0, time.UTC)
	start := time.Date(2030, time.May, 11, 15, 0, 0, 0, time.UTC)
	listing := Listing{ID: testSpaceID, OwnerID: testOwnerID, State: "activa", Title: "Sala sintética", RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", RateVersion: 7, TimeZone: "America/Santiago"}
	quote := Quote{ID: testQuoteID, SpaceID: testSpaceID, OwnerID: testOwnerID, RenterID: testRenterID, SpaceTitle: listing.Title, RateVersion: listing.RateVersion, RateUnit: listing.RateUnit, UnitPriceCLP: 8000, Currency: "CLP", Units: 1, SubtotalCLP: 8000, StartAt: start, EndAt: start.Add(time.Hour), TimeZone: listing.TimeZone, CreatedAt: now.Add(-time.Minute), ExpiresAt: now.Add(5 * time.Minute)}
	ctx, cancel := context.WithCancel(context.Background())
	gate := &identityGateStub{err: identityverification.ErrNotVerified, onRequire: cancel}
	identities := &contractIdentityReaderStub{}
	service, err := NewContractPreviewService(&catalogStub{listing: listing}, &calendarStub{available: true}, &previewQuoteRepository{quote: quote}, gate, identities, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.Preview(ctx, testRenterID, testQuoteID)
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("canceled gate with not-verified result error=%v, want ErrUnavailable", err)
	}
	if gate.calls != 1 || len(identities.calls) != 0 {
		t.Fatalf("canceled identity gate continued legal reads: gate=%d legal_identity=%d", gate.calls, len(identities.calls))
	}
}

func (s *contractIdentityReaderStub) CurrentForRentalContract(_ context.Context, requesterID, quoteID, subjectID string, asOf time.Time) (ContractIdentity, error) {
	s.calls = append(s.calls, requesterID+"/"+quoteID+"/"+subjectID)
	s.asOf = append(s.asOf, asOf)
	if s.onRead != nil {
		s.onRead()
	}
	if err, ok := s.errBySubject[subjectID]; ok {
		return s.identities[subjectID], err
	}
	return s.identities[subjectID], s.err
}

func TestContractPreviewMapsLegalIdentityReadFailuresWithoutPartialData(t *testing.T) {
	now := time.Date(2030, time.May, 10, 12, 0, 0, 0, time.UTC)
	start := time.Date(2030, time.May, 11, 15, 0, 0, 0, time.UTC)
	listing := Listing{ID: testSpaceID, OwnerID: testOwnerID, State: "activa", Title: "Sala sintética", RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", RateVersion: 7, TimeZone: "America/Santiago"}
	privateFailure := errors.New("private legal identity store detail")
	cases := []struct {
		name      string
		subjectID string
		wantReads int
	}{
		{name: "owner identity read", subjectID: testOwnerID, wantReads: 1},
		{name: "renter identity read", subjectID: testRenterID, wantReads: 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			quote := Quote{ID: testQuoteID, SpaceID: testSpaceID, OwnerID: testOwnerID, RenterID: testRenterID, SpaceTitle: listing.Title, RateVersion: listing.RateVersion, RateUnit: listing.RateUnit, UnitPriceCLP: listing.UnitPriceCLP, Currency: "CLP", Units: 1, SubtotalCLP: listing.UnitPriceCLP, StartAt: start, EndAt: start.Add(time.Hour), TimeZone: listing.TimeZone, CreatedAt: now.Add(-time.Minute), ExpiresAt: now.Add(5 * time.Minute)}
			identities := &contractIdentityReaderStub{
				identities: map[string]ContractIdentity{
					testOwnerID:  {SubjectID: testOwnerID, LegalName: "Arrendador Sintético", RUT: "11.111.111-1"},
					testRenterID: {SubjectID: testRenterID, LegalName: "Arrendataria Sintética", RUT: "22.222.222-2"},
				},
				errBySubject: map[string]error{tc.subjectID: privateFailure},
			}
			service, err := NewContractPreviewService(&catalogStub{listing: listing}, &calendarStub{available: true}, &previewQuoteRepository{quote: quote}, &identityGateStub{}, identities, func() time.Time { return now })
			if err != nil {
				t.Fatal(err)
			}

			got, err := service.Preview(context.Background(), testRenterID, testQuoteID)
			identityFailureMapped := errors.Is(err, ErrIdentityUnavailable) && !errors.Is(err, privateFailure)
			if !identityFailureMapped {
				t.Errorf("identity read error=%v, want generic ErrIdentityUnavailable", err)
			}
			privateDetailsHidden := err == nil || !strings.Contains(err.Error(), privateFailure.Error())
			if !privateDetailsHidden {
				t.Errorf("private identity-store detail was exposed in error %q", err.Error())
			}
			previewDiscarded := contractPreviewIsEmpty(got)
			if !previewDiscarded {
				t.Errorf("failed identity read returned partial preview: %+v", got)
			}
			readCountMatches := len(identities.calls) == tc.wantReads
			if !readCountMatches {
				t.Errorf("identity reads=%d, want=%d", len(identities.calls), tc.wantReads)
			}
			if !identityFailureMapped || !privateDetailsHidden || !previewDiscarded || !readCountMatches {
				return
			}
		})
	}
}

func TestContractPreviewClassifiesLegalIdentityReadCancellationAsUnavailable(t *testing.T) {
	now := time.Date(2030, time.May, 10, 12, 0, 0, 0, time.UTC)
	start := time.Date(2030, time.May, 11, 15, 0, 0, 0, time.UTC)
	listing := Listing{ID: testSpaceID, OwnerID: testOwnerID, State: "activa", Title: "Sala sintética", RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", RateVersion: 7, TimeZone: "America/Santiago"}
	cases := []struct {
		name      string
		subjectID string
		wantReads int
	}{
		{name: "owner identity reader", subjectID: testOwnerID, wantReads: 1},
		{name: "renter identity reader after owner", subjectID: testRenterID, wantReads: 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			quote := Quote{ID: testQuoteID, SpaceID: testSpaceID, OwnerID: testOwnerID, RenterID: testRenterID, SpaceTitle: listing.Title, RateVersion: listing.RateVersion, RateUnit: listing.RateUnit, UnitPriceCLP: 8000, Currency: "CLP", Units: 1, SubtotalCLP: 8000, StartAt: start, EndAt: start.Add(time.Hour), TimeZone: listing.TimeZone, CreatedAt: now.Add(-time.Minute), ExpiresAt: now.Add(5 * time.Minute)}
			ctx, cancel := context.WithCancel(context.Background())
			readCount := 0
			identities := &contractIdentityReaderStub{
				identities: map[string]ContractIdentity{
					testOwnerID:  {SubjectID: testOwnerID, LegalName: "Arrendador Sintético", RUT: "11.111.111-1"},
					testRenterID: {SubjectID: testRenterID, LegalName: "Arrendataria Sintética", RUT: "22.222.222-2"},
				},
				errBySubject: map[string]error{tc.subjectID: context.Canceled},
				onRead: func() {
					readCount++
					if readCount == tc.wantReads {
						cancel()
					}
				},
			}
			service, err := NewContractPreviewService(&catalogStub{listing: listing}, &calendarStub{available: true}, &previewQuoteRepository{quote: quote}, &identityGateStub{}, identities, func() time.Time { return now })
			if err != nil {
				t.Fatal(err)
			}

			got, err := service.Preview(ctx, testRenterID, testQuoteID)
			if !errors.Is(err, ErrUnavailable) {
				t.Fatalf("canceled legal identity read error=%v, want ErrUnavailable", err)
			}
			if got.State != "" || len(got.Parties) != 0 || len(identities.calls) != tc.wantReads {
				t.Fatalf("canceled legal identity read returned partial data: preview=%+v reads=%d want=%d", got, len(identities.calls), tc.wantReads)
			}
		})
	}
}

func TestContractPreviewStopsLegalIdentityReadsWhenContextIsCanceledByFirstRead(t *testing.T) {
	now := time.Date(2030, time.May, 10, 12, 0, 0, 0, time.UTC)
	start := time.Date(2030, time.May, 11, 15, 0, 0, 0, time.UTC)
	listing := Listing{ID: testSpaceID, OwnerID: testOwnerID, State: "activa", Title: "Sala sintética", RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", RateVersion: 7, TimeZone: "America/Santiago"}
	quote := Quote{ID: testQuoteID, SpaceID: testSpaceID, OwnerID: testOwnerID, RenterID: testRenterID, SpaceTitle: listing.Title, RateVersion: listing.RateVersion, RateUnit: listing.RateUnit, UnitPriceCLP: listing.UnitPriceCLP, Currency: "CLP", Units: 1, SubtotalCLP: listing.UnitPriceCLP, StartAt: start, EndAt: start.Add(time.Hour), TimeZone: listing.TimeZone, CreatedAt: now.Add(-time.Minute), ExpiresAt: now.Add(5 * time.Minute)}
	ctx, cancel := context.WithCancel(context.Background())
	quotes := &previewQuoteRepository{quote: quote}
	calendar := &calendarStub{available: true}
	gate := &identityGateStub{}
	identities := &contractIdentityReaderStub{
		identities: map[string]ContractIdentity{
			testOwnerID:  {SubjectID: testOwnerID, LegalName: "Arrendador Sintético", RUT: "11.111.111-1"},
			testRenterID: {SubjectID: testRenterID, LegalName: "Arrendataria Sintética", RUT: "22.222.222-2"},
		},
		onRead: cancel,
	}
	service, err := NewContractPreviewService(&catalogStub{listing: listing}, calendar, quotes, gate, identities, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.Preview(ctx, testRenterID, testQuoteID)
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("canceled preview error=%v, want ErrUnavailable", err)
	}
	if len(identities.calls) != 1 || identities.calls[0] != testRenterID+"/"+testQuoteID+"/"+testOwnerID {
		t.Fatalf("canceled preview continued legal identity reads: calls=%v", identities.calls)
	}
}

func TestContractPreviewDoesNotReturnIdentityDataWhenContextIsCanceledBySecondRead(t *testing.T) {
	now := time.Date(2030, time.May, 10, 12, 0, 0, 0, time.UTC)
	start := time.Date(2030, time.May, 11, 15, 0, 0, 0, time.UTC)
	listing := Listing{ID: testSpaceID, OwnerID: testOwnerID, State: "activa", Title: "Sala sintética", RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", RateVersion: 7, TimeZone: "America/Santiago"}
	quote := Quote{ID: testQuoteID, SpaceID: testSpaceID, OwnerID: testOwnerID, RenterID: testRenterID, SpaceTitle: listing.Title, RateVersion: listing.RateVersion, RateUnit: listing.RateUnit, UnitPriceCLP: listing.UnitPriceCLP, Currency: "CLP", Units: 1, SubtotalCLP: listing.UnitPriceCLP, StartAt: start, EndAt: start.Add(time.Hour), TimeZone: listing.TimeZone, CreatedAt: now.Add(-time.Minute), ExpiresAt: now.Add(5 * time.Minute)}
	ctx, cancel := context.WithCancel(context.Background())
	readCount := 0
	identities := &contractIdentityReaderStub{
		identities: map[string]ContractIdentity{
			testOwnerID:  {SubjectID: testOwnerID, LegalName: "Arrendador Sintético", RUT: "11.111.111-1"},
			testRenterID: {SubjectID: testRenterID, LegalName: "Arrendataria Sintética", RUT: "22.222.222-2"},
		},
		onRead: func() {
			readCount++
			if readCount == 2 {
				cancel()
			}
		},
	}
	service, err := NewContractPreviewService(&catalogStub{listing: listing}, &calendarStub{available: true}, &previewQuoteRepository{quote: quote}, &identityGateStub{}, identities, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}

	preview, err := service.Preview(ctx, testRenterID, testQuoteID)
	if !errors.Is(err, ErrUnavailable) || len(preview.Parties) != 0 {
		t.Fatalf("canceled preview=%+v error=%v, want no preview and ErrUnavailable", preview, err)
	}
	if readCount != 2 || len(identities.calls) != 2 {
		t.Fatalf("legal identity reads=%d, want exactly two reads completed before cancellation", len(identities.calls))
	}
}

func TestContractPreviewStopsBeforeIdentitySnapshotWhenEitherPartyIsUnverified(t *testing.T) {
	now := time.Date(2030, time.May, 10, 12, 0, 0, 0, time.UTC)
	start := time.Date(2030, time.May, 11, 15, 0, 0, 0, time.UTC)
	listing := Listing{ID: testSpaceID, OwnerID: testOwnerID, State: "activa", Title: "Sala sintética", RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", RateVersion: 7, TimeZone: "America/Santiago"}
	quote := Quote{ID: testQuoteID, SpaceID: testSpaceID, OwnerID: testOwnerID, RenterID: testRenterID, SpaceTitle: listing.Title, RateVersion: 7, RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", Units: 1, SubtotalCLP: 8000, StartAt: start, EndAt: start.Add(time.Hour), TimeZone: listing.TimeZone, CreatedAt: now.Add(-time.Minute), ExpiresAt: now.Add(5 * time.Minute)}
	quotes := &previewQuoteRepository{quote: quote}
	calendar := &calendarStub{available: true}
	gate := &identityGateStub{err: identityverification.ErrNotVerified}
	identities := &contractIdentityReaderStub{identities: map[string]ContractIdentity{}}
	service, err := NewContractPreviewService(&catalogStub{listing: listing}, calendar, quotes, gate, identities, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.Preview(context.Background(), testRenterID, testQuoteID)
	identityBlocked := errors.Is(err, ErrIdentityRequired)
	if !identityBlocked {
		t.Errorf("unverified preview error=%v, want ErrIdentityRequired", err)
	}
	identityReadsBlocked := gate.calls == 1 && len(identities.calls) == 0
	if !identityReadsBlocked {
		t.Errorf("gate calls=%d identity snapshot reads=%d; snapshot must not be read before the gate", gate.calls, len(identities.calls))
	}
	if !identityBlocked || !identityReadsBlocked {
		return
	}
}

func TestContractPreviewRejectsNonCanonicalIntervalBeforeIdentityDisclosure(t *testing.T) {
	now := time.Date(2030, time.May, 10, 12, 0, 0, 0, time.UTC)
	start := time.Date(2030, time.May, 11, 15, 0, 0, 0, time.UTC)
	listing := Listing{ID: testSpaceID, OwnerID: testOwnerID, State: "activa", Title: "Sala sintética", RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", RateVersion: 7, TimeZone: "America/Santiago"}
	quote := Quote{ID: testQuoteID, SpaceID: testSpaceID, OwnerID: testOwnerID, RenterID: testRenterID, SpaceTitle: listing.Title, RateVersion: 7, RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", Units: 2, SubtotalCLP: 16000, StartAt: start, EndAt: start.Add(90 * time.Minute), TimeZone: listing.TimeZone, CreatedAt: now.Add(-time.Minute), ExpiresAt: now.Add(5 * time.Minute)}
	quotes := &previewQuoteRepository{quote: quote}
	calendar := &calendarStub{available: true}
	gate := &identityGateStub{}
	identities := &contractIdentityReaderStub{}
	service, err := NewContractPreviewService(&catalogStub{listing: listing}, calendar, quotes, gate, identities, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.Preview(context.Background(), testRenterID, testQuoteID)
	intervalRejected := errors.Is(err, ErrConflict)
	if !intervalRejected {
		t.Errorf("noncanonical interval error=%v, want ErrConflict", err)
	}
	privateReadsBlocked := calendar.checkCalls == 0 && gate.calls == 0 && len(identities.calls) == 0
	if !privateReadsBlocked {
		t.Errorf("invalid quote reached availability/identity reads: calendar=%d gate=%d snapshots=%d", calendar.checkCalls, gate.calls, len(identities.calls))
	}
	if !intervalRejected || !privateReadsBlocked {
		return
	}
}

func TestContractPreviewRequiresParticipantAndReturnsOnlyAfterCurrentIdentityGate(t *testing.T) {
	now := time.Date(2030, time.May, 10, 12, 0, 0, 0, time.UTC)
	start := time.Date(2030, time.May, 11, 15, 0, 0, 0, time.UTC)
	listing := Listing{ID: testSpaceID, OwnerID: testOwnerID, State: "activa", Title: "Sala sintética", RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", RateVersion: 7, TimeZone: "America/Santiago"}
	quote := Quote{ID: testQuoteID, SpaceID: testSpaceID, OwnerID: testOwnerID, RenterID: testRenterID, SpaceTitle: listing.Title, RateVersion: 7, RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", Units: 1, SubtotalCLP: 8000, StartAt: start, EndAt: start.Add(time.Hour), TimeZone: listing.TimeZone, CreatedAt: now.Add(-time.Minute), ExpiresAt: now.Add(5 * time.Minute)}
	quotes := &previewQuoteRepository{quote: quote}
	calendar := &calendarStub{available: true}
	gate := &identityGateStub{}
	catalog := &catalogStub{listing: listing}
	identities := &contractIdentityReaderStub{identities: map[string]ContractIdentity{
		testOwnerID:  {SubjectID: testOwnerID, LegalName: "Arrendador Sintético", RUT: "11.111.111-1"},
		testRenterID: {SubjectID: testRenterID, LegalName: "Arrendataria Sintética", RUT: "22.222.222-2"},
	}}
	service, err := NewContractPreviewService(catalog, calendar, quotes, gate, identities, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}

	outsiderPreview, err := service.Preview(context.Background(), "ffffffff-ffff-4fff-8fff-ffffffffffff", testQuoteID)
	nonparticipantRejected := errors.Is(err, ErrNotFound)
	if !nonparticipantRejected {
		t.Errorf("nonparticipant preview error=%v, want ErrNotFound", err)
	}
	outsiderPreviewDiscarded := contractPreviewIsEmpty(outsiderPreview)
	if !outsiderPreviewDiscarded {
		t.Errorf("nonparticipant received partial contract preview: %+v", outsiderPreview)
	}
	outsiderStoppedBeforePrivateReads := catalog.calls == 0 && calendar.checkCalls == 0 && gate.calls == 0 && len(identities.calls) == 0
	if !outsiderStoppedBeforePrivateReads {
		t.Errorf("nonparticipant caused follow-up reads: listing=%d calendar=%d gate=%d snapshots=%d", catalog.calls, calendar.checkCalls, gate.calls, len(identities.calls))
	}
	if !nonparticipantRejected || !outsiderPreviewDiscarded || !outsiderStoppedBeforePrivateReads {
		return
	}

	preview, err := service.Preview(context.Background(), testRenterID, testQuoteID)
	if err != nil {
		t.Fatal(err)
	}
	if preview.State != "preview" || preview.QuoteID != testQuoteID || preview.SubtotalCLP != 8000 || len(preview.Parties) != 2 {
		t.Fatalf("contract preview=%+v", preview)
	}
	if gate.calls != 1 || gate.host != testOwnerID || gate.renter != testRenterID || len(identities.calls) != 2 || len(identities.asOf) != 2 || !identities.asOf[0].Equal(now) || !identities.asOf[1].Equal(now) {
		t.Fatalf("identity gate/snapshot calls do not match participant preview: gate=%+v snapshots=%v", gate, identities.calls)
	}
	if preview.Parties[0].Role != "arrendador" || preview.Parties[0].LegalName != "Arrendador Sintético" || preview.Parties[0].RUT != "11111111-1" ||
		preview.Parties[1].Role != "arrendatario" || preview.Parties[1].LegalName != "Arrendataria Sintética" || preview.Parties[1].RUT != "22222222-2" {
		t.Fatalf("legal identity snapshots must be complete and RUTs canonicalized: %+v", preview.Parties)
	}
	ownerPreview, err := service.Preview(context.Background(), testOwnerID, testQuoteID)
	if err != nil {
		t.Fatalf("owner participant preview: %v", err)
	}
	if ownerPreview.State != "preview" || len(ownerPreview.Parties) != 2 || ownerPreview.Parties[0].Role != "arrendador" || ownerPreview.Parties[1].Role != "arrendatario" || gate.calls != 2 || len(identities.calls) != 4 ||
		identities.calls[2] != testOwnerID+"/"+testQuoteID+"/"+testOwnerID || identities.calls[3] != testOwnerID+"/"+testQuoteID+"/"+testRenterID {
		t.Fatalf("owner participant was not authorized with quote-scoped identity reads: preview=%+v gate=%+v calls=%v", ownerPreview, gate, identities.calls)
	}
	calendar.available = false
	if _, err = service.Preview(context.Background(), testRenterID, testQuoteID); !errors.Is(err, ErrConflict) {
		t.Fatalf("newly unavailable interval error=%v, want ErrConflict", err)
	}
	if gate.calls != 2 || len(identities.calls) != 4 {
		t.Fatalf("unavailable interval caused identity reads: gate=%d snapshots=%d", gate.calls, len(identities.calls))
	}
	calendar.available = true
	identities.identities[testOwnerID] = ContractIdentity{SubjectID: testOwnerID, LegalName: "Arrendador Sintético", RUT: "11.111.111-2"}
	if _, err = service.Preview(context.Background(), testRenterID, testQuoteID); !errors.Is(err, ErrIdentityUnavailable) {
		t.Fatalf("invalid legal RUT error=%v, want ErrIdentityUnavailable", err)
	}
}

func TestContractPreviewRevalidatesHourlyIntervalsAcrossDSTTransitions(t *testing.T) {
	for _, tc := range []struct {
		name  string
		now   time.Time
		start string
		end   string
	}{
		{
			name:  "fall repeated local hour",
			now:   time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC),
			start: "2026-11-01T01:30:00-04:00",
			end:   "2026-11-01T01:30:00-05:00",
		},
		{
			name:  "spring skipped local hour",
			now:   time.Date(2026, time.February, 1, 0, 0, 0, 0, time.UTC),
			start: "2026-03-08T01:30:00-05:00",
			end:   "2026-03-08T03:30:00-04:00",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			start, err := time.Parse(time.RFC3339, tc.start)
			if err != nil {
				t.Fatal(err)
			}
			end, err := time.Parse(time.RFC3339, tc.end)
			if err != nil {
				t.Fatal(err)
			}
			listing := Listing{ID: testSpaceID, OwnerID: testOwnerID, State: "activa", Title: "Sala sintética", RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", RateVersion: 7, TimeZone: "America/New_York"}
			quote := Quote{ID: testQuoteID, SpaceID: listing.ID, OwnerID: listing.OwnerID, RenterID: testRenterID, SpaceTitle: listing.Title, RateVersion: listing.RateVersion, RateUnit: listing.RateUnit, UnitPriceCLP: listing.UnitPriceCLP, Currency: "CLP", Units: 1, SubtotalCLP: 8000, StartAt: start.UTC(), EndAt: end.UTC(), TimeZone: listing.TimeZone, CreatedAt: tc.now.Add(-time.Minute), ExpiresAt: tc.now.Add(5 * time.Minute)}
			calendar := &calendarStub{available: true}
			gate := &identityGateStub{}
			identities := &contractIdentityReaderStub{identities: map[string]ContractIdentity{
				testOwnerID:  {SubjectID: testOwnerID, LegalName: "Arrendador Sintético", RUT: "11.111.111-1"},
				testRenterID: {SubjectID: testRenterID, LegalName: "Arrendataria Sintética", RUT: "22.222.222-2"},
			}}
			service, err := NewContractPreviewService(&catalogStub{listing: listing}, calendar, &previewQuoteRepository{quote: quote}, gate, identities, func() time.Time { return tc.now })
			if err != nil {
				t.Fatal(err)
			}

			got, err := service.Preview(context.Background(), testRenterID, testQuoteID)
			if err != nil {
				t.Fatalf("Preview() error=%v", err)
			}
			if got.Units != 1 || got.SubtotalCLP != 8000 || !got.StartAt.Equal(start.UTC()) || !got.EndAt.Equal(end.UTC()) ||
				calendar.checkCalls != 1 || gate.calls != 1 || len(identities.calls) != 2 {
				t.Fatalf("preview=%+v availability=%d gate=%d identity reads=%d; want one elapsed hour revalidated before identity", got, calendar.checkCalls, gate.calls, len(identities.calls))
			}
		})
	}
}

func TestContractPreviewRejectsMismatchedQuoteSnapshotsBeforeIdentityReads(t *testing.T) {
	now := time.Date(2030, time.May, 10, 12, 0, 0, 0, time.UTC)
	start := time.Date(2030, time.May, 11, 15, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name   string
		mutate func(*Quote)
	}{
		{name: "wrong space", mutate: func(quote *Quote) { quote.SpaceID = "ffffffff-ffff-4fff-8fff-ffffffffffff" }},
		{name: "wrong owner", mutate: func(quote *Quote) { quote.OwnerID = "ffffffff-ffff-4fff-8fff-ffffffffffff" }},
		{name: "wrong title", mutate: func(quote *Quote) { quote.SpaceTitle += " alterado" }},
		{name: "stale rate revision", mutate: func(quote *Quote) { quote.RateVersion++ }},
		{name: "wrong rate unit", mutate: func(quote *Quote) { quote.RateUnit = "dia" }},
		{name: "wrong unit price", mutate: func(quote *Quote) { quote.UnitPriceCLP++ }},
		{name: "wrong currency", mutate: func(quote *Quote) { quote.Currency = "USD" }},
		{name: "wrong units", mutate: func(quote *Quote) { quote.Units++ }},
		{name: "wrong subtotal", mutate: func(quote *Quote) { quote.SubtotalCLP++ }},
		{name: "wrong start", mutate: func(quote *Quote) { quote.StartAt = quote.StartAt.Add(30 * time.Minute) }},
		{name: "wrong end", mutate: func(quote *Quote) { quote.EndAt = quote.EndAt.Add(30 * time.Minute) }},
		{name: "wrong time zone", mutate: func(quote *Quote) { quote.TimeZone = "UTC" }},
		{name: "missing creation timestamp", mutate: func(quote *Quote) { quote.CreatedAt = time.Time{} }},
		{name: "creation timestamp in future", mutate: func(quote *Quote) { quote.CreatedAt = now.Add(time.Nanosecond) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			listing := Listing{ID: testSpaceID, OwnerID: testOwnerID, State: "activa", Title: "Sala sintética", RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", RateVersion: 7, TimeZone: "America/Santiago"}
			quote := Quote{ID: testQuoteID, SpaceID: listing.ID, OwnerID: listing.OwnerID, RenterID: testRenterID, SpaceTitle: listing.Title, RateVersion: listing.RateVersion, RateUnit: listing.RateUnit, UnitPriceCLP: listing.UnitPriceCLP, Currency: listing.Currency, Units: 1, SubtotalCLP: 8000, StartAt: start, EndAt: start.Add(time.Hour), TimeZone: listing.TimeZone, CreatedAt: now.Add(-time.Minute), ExpiresAt: now.Add(5 * time.Minute)}
			tc.mutate(&quote)
			quotes := &previewQuoteRepository{quote: quote}
			catalog := &catalogStub{listing: listing}
			calendar := &calendarStub{available: true}
			gate := &identityGateStub{}
			identities := &contractIdentityReaderStub{}
			service, err := NewContractPreviewService(catalog, calendar, quotes, gate, identities, func() time.Time { return now })
			if err != nil {
				t.Fatal(err)
			}

			got, err := service.Preview(context.Background(), testRenterID, testQuoteID)
			mismatchMapped := errors.Is(err, ErrConflict)
			if !mismatchMapped {
				t.Errorf("mismatched quote preview error=%v, want ErrConflict", err)
			}
			previewDiscarded := contractPreviewIsEmpty(got)
			if !previewDiscarded {
				t.Errorf("mismatched quote returned partial contract preview: %+v", got)
			}
			laterReadsStopped := quotes.calls == 1 && catalog.calls == 1 && calendar.checkCalls == 0 && gate.calls == 0 && len(identities.calls) == 0
			if !laterReadsStopped {
				t.Errorf("mismatched quote reached later reads: quote=%d listing=%d calendar=%d gate=%d identity=%d", quotes.calls, catalog.calls, calendar.checkCalls, gate.calls, len(identities.calls))
			}
			if !mismatchMapped || !previewDiscarded || !laterReadsStopped {
				return
			}
		})
	}
}

func TestContractPreviewRejectsPublicationOrRateChangesBeforeIdentityDisclosure(t *testing.T) {
	now := time.Date(2030, time.May, 10, 12, 0, 0, 0, time.UTC)
	start := time.Date(2030, time.May, 11, 15, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name   string
		mutate func(*Listing)
	}{
		{name: "publication withdrawn", mutate: func(listing *Listing) { listing.State = "retirada" }},
		{name: "rate changed", mutate: func(listing *Listing) { listing.UnitPriceCLP = 9000; listing.RateVersion++ }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			listing := Listing{ID: testSpaceID, OwnerID: testOwnerID, State: "activa", Title: "Sala sintética", RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", RateVersion: 7, TimeZone: "America/Santiago"}
			quote := Quote{ID: testQuoteID, SpaceID: testSpaceID, OwnerID: testOwnerID, RenterID: testRenterID, SpaceTitle: listing.Title, RateVersion: 7, RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", Units: 1, SubtotalCLP: 8000, StartAt: start, EndAt: start.Add(time.Hour), TimeZone: listing.TimeZone, CreatedAt: now.Add(-time.Minute), ExpiresAt: now.Add(5 * time.Minute)}
			tc.mutate(&listing)
			catalog := &catalogStub{listing: listing}
			calendar := &calendarStub{available: true}
			gate := &identityGateStub{}
			identities := &contractIdentityReaderStub{identities: map[string]ContractIdentity{
				testOwnerID:  {SubjectID: testOwnerID, LegalName: "Arrendador Sintético", RUT: "11.111.111-1"},
				testRenterID: {SubjectID: testRenterID, LegalName: "Arrendataria Sintética", RUT: "22.222.222-2"},
			}}
			quotes := &previewQuoteRepository{quote: quote}
			service, err := NewContractPreviewService(catalog, calendar, quotes, gate, identities, func() time.Time { return now })
			if err != nil {
				t.Fatal(err)
			}

			got, err := service.Preview(context.Background(), testRenterID, testQuoteID)
			staleStateRejected := errors.Is(err, ErrConflict)
			if !staleStateRejected {
				t.Errorf("stale publication/rate preview error=%v, want ErrConflict", err)
			}
			previewDiscarded := contractPreviewIsEmpty(got)
			if !previewDiscarded {
				t.Errorf("stale publication/rate returned partial contract preview: %+v", got)
			}
			laterReadsStopped := quotes.calls == 1 && catalog.calls == 1 && calendar.checkCalls == 0 && gate.calls == 0 && len(identities.calls) == 0
			if !laterReadsStopped {
				t.Errorf("stale quote reached later reads: quote=%d listing=%d calendar=%d gate=%d snapshots=%d", quotes.calls, catalog.calls, calendar.checkCalls, gate.calls, len(identities.calls))
			}
			if !staleStateRejected || !previewDiscarded || !laterReadsStopped {
				return
			}
		})
	}
}

func TestContractPreviewFailsClosedOnCalendarReaderErrorBeforeIdentityDisclosure(t *testing.T) {
	now := time.Date(2030, time.May, 10, 12, 0, 0, 0, time.UTC)
	start := time.Date(2030, time.May, 11, 15, 0, 0, 0, time.UTC)
	listing := Listing{ID: testSpaceID, OwnerID: testOwnerID, State: "activa", Title: "Sala sintética", RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", RateVersion: 7, TimeZone: "America/Santiago"}
	quote := Quote{ID: testQuoteID, SpaceID: testSpaceID, OwnerID: testOwnerID, RenterID: testRenterID, SpaceTitle: listing.Title, RateVersion: 7, RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", Units: 1, SubtotalCLP: 8000, StartAt: start, EndAt: start.Add(time.Hour), TimeZone: listing.TimeZone, CreatedAt: now.Add(-time.Minute), ExpiresAt: now.Add(5 * time.Minute)}
	calendarFailure := errors.New("private calendar backend detail")
	calendar := &calendarStub{available: true, checkErr: calendarFailure}
	gate := &identityGateStub{}
	identities := &contractIdentityReaderStub{identities: map[string]ContractIdentity{
		testOwnerID:  {SubjectID: testOwnerID, LegalName: "Arrendador Sintético", RUT: "11.111.111-1"},
		testRenterID: {SubjectID: testRenterID, LegalName: "Arrendataria Sintética", RUT: "22.222.222-2"},
	}}
	service, err := NewContractPreviewService(&catalogStub{listing: listing}, calendar, &previewQuoteRepository{quote: quote}, gate, identities, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}

	got, err := service.Preview(context.Background(), testRenterID, testQuoteID)
	calendarFailureMapped := errors.Is(err, ErrUnavailable) && !errors.Is(err, calendarFailure)
	if !calendarFailureMapped {
		t.Errorf("calendar failure preview error=%v, want generic ErrUnavailable", err)
	}
	privateDetailHidden := err == nil || !strings.Contains(err.Error(), calendarFailure.Error())
	if !privateDetailHidden {
		t.Errorf("private calendar detail was exposed in error %q", err.Error())
	}
	previewEmpty := contractPreviewIsEmpty(got)
	if !previewEmpty {
		t.Errorf("calendar failure returned partial contract preview: %+v", got)
	}
	identityDisclosureStopped := calendar.checkCalls == 1 && gate.calls == 0 && len(identities.calls) == 0
	if !identityDisclosureStopped {
		t.Errorf("calendar failure reached identity gate/read: calendar=%d gate=%d identityReads=%d", calendar.checkCalls, gate.calls, len(identities.calls))
	}
	if !calendarFailureMapped || !privateDetailHidden || !previewEmpty || !identityDisclosureStopped {
		return
	}
}

func TestContractPreviewFailsClosedWhenListingReaderErrorsBeforeIdentityDisclosure(t *testing.T) {
	now := time.Date(2030, time.May, 10, 12, 0, 0, 0, time.UTC)
	start := time.Date(2030, time.May, 11, 15, 0, 0, 0, time.UTC)
	quote := Quote{ID: testQuoteID, SpaceID: testSpaceID, OwnerID: testOwnerID, RenterID: testRenterID, SpaceTitle: "Sala sintética", RateVersion: 7, RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", Units: 1, SubtotalCLP: 8000, StartAt: start, EndAt: start.Add(time.Hour), TimeZone: "America/Santiago", CreatedAt: now.Add(-time.Minute), ExpiresAt: now.Add(5 * time.Minute)}
	listingFailure := errors.New("private listing backend detail")
	listing := Listing{ID: testSpaceID, OwnerID: testOwnerID, State: "activa", Title: "Sala sintética", RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", RateVersion: 7, TimeZone: "America/Santiago"}
	catalog := &catalogStub{listing: listing, err: listingFailure}
	calendar := &calendarStub{available: true}
	gate := &identityGateStub{}
	identities := &contractIdentityReaderStub{identities: map[string]ContractIdentity{
		testOwnerID:  {SubjectID: testOwnerID, LegalName: "Arrendador Sintético", RUT: "11.111.111-1"},
		testRenterID: {SubjectID: testRenterID, LegalName: "Arrendataria Sintética", RUT: "22.222.222-2"},
	}}
	quotes := &previewQuoteRepository{quote: quote}
	service, err := NewContractPreviewService(catalog, calendar, quotes, gate, identities, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}

	got, err := service.Preview(context.Background(), testRenterID, testQuoteID)
	listingFailureMapped := errors.Is(err, ErrUnavailable) && !errors.Is(err, listingFailure)
	if !listingFailureMapped {
		t.Errorf("listing failure preview error=%v, want generic ErrUnavailable", err)
	}
	privateDetailHidden := err == nil || !strings.Contains(err.Error(), listingFailure.Error())
	if !privateDetailHidden {
		t.Errorf("private listing detail was exposed in error %q", err.Error())
	}
	previewDiscarded := contractPreviewIsEmpty(got)
	if !previewDiscarded {
		t.Errorf("listing failure returned partial contract preview: %+v", got)
	}
	listingFailureStoppedFlow := quotes.calls == 1 && catalog.calls == 1 && calendar.checkCalls == 0 && gate.calls == 0 && len(identities.calls) == 0
	if !listingFailureStoppedFlow {
		t.Errorf("listing failure reached subsequent dependencies: quotes=%d catalog=%d calendar=%d gate=%d identities=%d", quotes.calls, catalog.calls, calendar.checkCalls, gate.calls, len(identities.calls))
	}
	if !listingFailureMapped || !privateDetailHidden || !previewDiscarded || !listingFailureStoppedFlow {
		return
	}
}

func TestContractPreviewRejectsMalformedLegalNamesWithoutReturningThem(t *testing.T) {
	cases := []struct {
		name      string
		legalName string
	}{
		{name: "empty"},
		{name: "leading whitespace", legalName: " Arrendador Sintético"},
		{name: "trailing whitespace", legalName: "Arrendador Sintético "},
		{name: "control character", legalName: "Arrendador\nSintético"},
		{name: "Unicode line separator", legalName: "Arrendador\u2028Sintético"},
		{name: "Unicode paragraph separator", legalName: "Arrendador\u2029Sintético"},
		{name: "right-to-left override", legalName: "Arrendador\u202eSintético"},
		{name: "invalid UTF-8", legalName: "Arrendador \xff Sintético"},
		{name: "over 200 runes", legalName: strings.Repeat("N", 201)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Date(2030, time.May, 10, 12, 0, 0, 0, time.UTC)
			start := time.Date(2030, time.May, 11, 15, 0, 0, 0, time.UTC)
			listing := Listing{ID: testSpaceID, OwnerID: testOwnerID, State: "activa", Title: "Sala sintética", RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", RateVersion: 7, TimeZone: "America/Santiago"}
			quote := Quote{ID: testQuoteID, SpaceID: testSpaceID, OwnerID: testOwnerID, RenterID: testRenterID, SpaceTitle: listing.Title, RateVersion: 7, RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", Units: 1, SubtotalCLP: 8000, StartAt: start, EndAt: start.Add(time.Hour), TimeZone: listing.TimeZone, CreatedAt: now.Add(-time.Minute), ExpiresAt: now.Add(5 * time.Minute)}
			calendar := &calendarStub{available: true}
			gate := &identityGateStub{}
			identities := &contractIdentityReaderStub{identities: map[string]ContractIdentity{
				testOwnerID:  {SubjectID: testOwnerID, LegalName: tc.legalName, RUT: "11.111.111-1"},
				testRenterID: {SubjectID: testRenterID, LegalName: "Arrendataria Sintética", RUT: "22.222.222-2"},
			}}
			service, err := NewContractPreviewService(&catalogStub{listing: listing}, calendar, &previewQuoteRepository{quote: quote}, gate, identities, func() time.Time { return now })
			if err != nil {
				t.Fatal(err)
			}

			_, err = service.Preview(context.Background(), testRenterID, testQuoteID)
			if !errors.Is(err, ErrIdentityUnavailable) {
				t.Fatalf("malformed legal name error=%v, want ErrIdentityUnavailable", err)
			}
			if tc.legalName != "" && strings.Contains(err.Error(), tc.legalName) {
				t.Fatalf("error exposed malformed legal name: %v", err)
			}
			if gate.calls != 1 || len(identities.calls) != 1 {
				t.Fatalf("malformed owner identity did not stop after first snapshot: gate=%d identityReads=%d", gate.calls, len(identities.calls))
			}
		})
	}
}

func TestContractPreviewDoesNotReturnOwnerSnapshotWhenRenterIdentityIsMalformed(t *testing.T) {
	now := time.Date(2030, time.May, 10, 12, 0, 0, 0, time.UTC)
	start := time.Date(2030, time.May, 11, 15, 0, 0, 0, time.UTC)
	listing := Listing{ID: testSpaceID, OwnerID: testOwnerID, State: "activa", Title: "Sala sintética", RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", RateVersion: 7, TimeZone: "America/Santiago"}
	quote := Quote{ID: testQuoteID, SpaceID: testSpaceID, OwnerID: testOwnerID, RenterID: testRenterID, SpaceTitle: listing.Title, RateVersion: 7, RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", Units: 1, SubtotalCLP: 8000, StartAt: start, EndAt: start.Add(time.Hour), TimeZone: listing.TimeZone, CreatedAt: now.Add(-time.Minute), ExpiresAt: now.Add(5 * time.Minute)}
	malformedRenterName := "Arrendataria\nSintética"
	identities := &contractIdentityReaderStub{identities: map[string]ContractIdentity{
		testOwnerID:  {SubjectID: testOwnerID, LegalName: "Arrendador Sintético", RUT: "11.111.111-1"},
		testRenterID: {SubjectID: testRenterID, LegalName: malformedRenterName, RUT: "22.222.222-2"},
	}}
	gate := &identityGateStub{}
	service, err := NewContractPreviewService(&catalogStub{listing: listing}, &calendarStub{available: true}, &previewQuoteRepository{quote: quote}, gate, identities, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}

	got, err := service.Preview(context.Background(), testRenterID, testQuoteID)
	if !errors.Is(err, ErrIdentityUnavailable) || strings.Contains(err.Error(), malformedRenterName) {
		t.Fatalf("malformed renter identity error=%v, want generic ErrIdentityUnavailable", err)
	}
	if got.State != "" || len(got.Parties) != 0 || len(identities.calls) != 2 || gate.calls != 1 {
		t.Fatalf("partial owner identity escaped failed preview: preview=%+v identityReads=%d gate=%d", got, len(identities.calls), gate.calls)
	}
}
