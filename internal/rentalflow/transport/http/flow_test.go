package rentalflowhttp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
	"github.com/HernanEspinozaDev/espaciGo/internal/rentalflow"
)

// This in-memory fixture is intentionally synthetic: it exercises the real
// services and HTTP handlers without starting the API or opening a database.
type syntheticFlowBackend struct {
	listing                 rentalflow.Listing
	listingErr              error
	onListingRead           func()
	listingReads            int
	interval                rentalflow.Interval
	available               bool
	quote                   rentalflow.Quote
	quoteID                 string
	quoteErr                error
	onQuoteCreate           func()
	publicDuration          int
	quoteWrites             int
	availabilityChecks      int
	availabilityReads       int
	checkedIntervals        []rentalflow.Interval
	returnQuoteToAnyActor   bool
	returnListingToAnyState bool
	emptyIntervalsNil       bool
}

type partialErrorAvailabilityCalendar struct {
	*syntheticFlowBackend
	intervals []rentalflow.Interval
	err       error
}

func (c *partialErrorAvailabilityCalendar) PublicIntervals(_ context.Context, _ rentalflow.Listing, _ string, _ int) ([]rentalflow.Interval, error) {
	c.availabilityReads++
	return c.intervals, c.err
}

func (b *syntheticFlowBackend) PublicListing(_ context.Context, spaceID string) (rentalflow.Listing, error) {
	b.listingReads++
	if b.onListingRead != nil {
		b.onListingRead()
	}
	if b.listingErr != nil {
		return rentalflow.Listing{}, b.listingErr
	}
	if spaceID != b.listing.ID || (!b.returnListingToAnyState && b.listing.State != "activa") {
		return rentalflow.Listing{}, rentalflow.ErrNotFound
	}
	return b.listing, nil
}

func (b *syntheticFlowBackend) PublicIntervals(_ context.Context, listing rentalflow.Listing, date string, duration int) ([]rentalflow.Interval, error) {
	b.availabilityReads++
	local := b.interval.StartAt.In(mustLoadLocation(listing.TimeZone))
	durationMatches := b.publicDuration == duration
	if b.publicDuration == 0 {
		durationMatches = b.interval.EndAt.Sub(b.interval.StartAt) == time.Duration(duration)*time.Hour
	}
	if local.Format("2006-01-02") != date || !durationMatches || !b.available {
		if b.emptyIntervalsNil {
			return nil, nil
		}
		return []rentalflow.Interval{}, nil
	}
	return []rentalflow.Interval{b.interval}, nil
}

func (b *syntheticFlowBackend) IntervalAvailable(_ context.Context, listing rentalflow.Listing, interval rentalflow.Interval) (bool, error) {
	b.availabilityChecks++
	b.checkedIntervals = append(b.checkedIntervals, interval)
	return b.available && listing.ID == b.listing.ID && interval.StartAt.Equal(b.interval.StartAt) && interval.EndAt.Equal(b.interval.EndAt), nil
}

func (b *syntheticFlowBackend) CreateValidatedQuote(_ context.Context, renterID string, listing rentalflow.Listing, interval rentalflow.Interval, units, subtotal int64, createdAt time.Time) (rentalflow.Quote, error) {
	b.quoteWrites++
	if b.onQuoteCreate != nil {
		b.onQuoteCreate()
	}
	if b.quoteErr != nil {
		return rentalflow.Quote{}, b.quoteErr
	}
	if !b.available || b.listing.State != "activa" || listing.ID != b.listing.ID || listing.OwnerID != b.listing.OwnerID || listing.State != b.listing.State ||
		listing.RateVersion != b.listing.RateVersion || listing.TimeZone != b.listing.TimeZone ||
		!interval.StartAt.Equal(b.interval.StartAt) || !interval.EndAt.Equal(b.interval.EndAt) {
		return rentalflow.Quote{}, rentalflow.ErrConflict
	}
	quoteID := b.quoteID
	if quoteID == "" {
		quoteID = testQuoteID
	}
	b.quote = rentalflow.Quote{
		ID: quoteID, SpaceID: listing.ID, OwnerID: listing.OwnerID, RenterID: renterID,
		SpaceTitle: listing.Title, RateVersion: listing.RateVersion, RateUnit: listing.RateUnit,
		UnitPriceCLP: listing.UnitPriceCLP, Currency: listing.Currency, Units: units, SubtotalCLP: subtotal,
		StartAt: interval.StartAt, EndAt: interval.EndAt, TimeZone: listing.TimeZone,
		CreatedAt: createdAt, ExpiresAt: createdAt.Add(15 * time.Minute),
	}
	return b.quote, nil
}

func (b *syntheticFlowBackend) GetForParticipant(_ context.Context, actorID, quoteID string) (rentalflow.Quote, error) {
	if quoteID != b.quote.ID || (!b.returnQuoteToAnyActor && actorID != b.quote.OwnerID && actorID != b.quote.RenterID) {
		return rentalflow.Quote{}, rentalflow.ErrNotFound
	}
	return b.quote, nil
}

type syntheticIdentityGate struct {
	ownerID  string
	renterID string
	calls    int
}

func (g *syntheticIdentityGate) RequireForBooking(_ context.Context, ownerID, renterID string, _ time.Time) error {
	g.calls++
	if ownerID != g.ownerID || renterID != g.renterID {
		return rentalflow.ErrIdentityRequired
	}
	return nil
}

type syntheticIdentityReader struct {
	quoteID      string
	parties      map[string]rentalflow.ContractIdentity
	calls        int
	err          error
	errBySubject map[string]error
}

func (r *syntheticIdentityReader) CurrentForRentalContract(_ context.Context, requesterID, quoteID, subjectID string, _ time.Time) (rentalflow.ContractIdentity, error) {
	r.calls++
	if quoteID != r.quoteID || (requesterID != "33333333-3333-4333-8333-333333333333" && requesterID != "44444444-4444-4444-8444-444444444444") {
		return rentalflow.ContractIdentity{}, errors.New("not an authorized participant")
	}
	if r.err != nil {
		return rentalflow.ContractIdentity{}, r.err
	}
	if err := r.errBySubject[subjectID]; err != nil {
		return rentalflow.ContractIdentity{}, err
	}
	identity, ok := r.parties[subjectID]
	if !ok {
		return rentalflow.ContractIdentity{}, rentalflow.ErrIdentityUnavailable
	}
	return identity, nil
}

func mustLoadLocation(name string) *time.Location {
	location, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return location
}

func TestSyntheticHTTPAvailabilityRepresentsNoSlotsAsEmptyArray(t *testing.T) {
	const ownerID = "44444444-4444-4444-8444-444444444444"
	now := time.Date(2030, time.January, 1, 12, 0, 0, 0, time.UTC)
	start := time.Date(2030, time.January, 2, 13, 0, 0, 0, time.UTC)
	backend := &syntheticFlowBackend{
		listing:  rentalflow.Listing{ID: testSpaceID, OwnerID: ownerID, State: "activa", Title: "Sala sintética", RateUnit: "hora", UnitPriceCLP: 12000, Currency: "CLP", RateVersion: 2, TimeZone: "America/Santiago"},
		interval: rentalflow.Interval{StartAt: start, EndAt: start.Add(time.Hour)}, available: false, emptyIntervalsNil: true,
	}
	availability, err := rentalflow.NewAvailabilityService(backend, backend, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	handler := New(nil, availability, nil, nil, nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet,
		"/api/v1/spaces/"+testSpaceID+"/rental-availability?date=2030-01-02&duration=1", nil))
	if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("empty availability status=%d cache=%q body=%s", response.Code, response.Header().Get("Cache-Control"), response.Body.String())
	}
	var envelope struct {
		Data struct {
			Items json.RawMessage `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode empty availability: %v", err)
	}
	if string(envelope.Data.Items) != "[]" {
		t.Fatalf("empty availability items=%s, want []", envelope.Data.Items)
	}
	if backend.availabilityReads != 1 || backend.availabilityChecks != 0 || backend.quoteWrites != 0 {
		t.Fatalf("empty lookup side effects=%+v, want one public read and no revalidation/quote writes", backend)
	}
}

func TestSyntheticHTTPAvailabilityDiscardsPartialCalendarResultOnError(t *testing.T) {
	const ownerID = "44444444-4444-4444-8444-444444444444"
	now := time.Date(2030, time.January, 1, 12, 0, 0, 0, time.UTC)
	start := time.Date(2030, time.January, 2, 13, 0, 0, 0, time.UTC)
	calendarFailure := errors.New("private calendar backend detail")
	backend := &syntheticFlowBackend{
		listing: rentalflow.Listing{ID: testSpaceID, OwnerID: ownerID, State: "activa", Title: "Sala sintética", RateUnit: "hora", UnitPriceCLP: 12000, Currency: "CLP", RateVersion: 2, TimeZone: "America/Santiago"},
	}
	calendar := &partialErrorAvailabilityCalendar{
		syntheticFlowBackend: backend,
		intervals:            []rentalflow.Interval{{StartAt: start, EndAt: start.Add(time.Hour)}},
		err:                  calendarFailure,
	}
	availability, err := rentalflow.NewAvailabilityService(backend, calendar, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	handler := New(nil, availability, nil, nil, nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet,
		"/api/v1/spaces/"+testSpaceID+"/rental-availability?date=2030-01-02&duration=1", nil))
	if response.Code != http.StatusServiceUnavailable || response.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("partial calendar failure status=%d cache=%q body=%s, want 503/no-store", response.Code, response.Header().Get("Cache-Control"), response.Body.String())
	}
	if strings.Contains(response.Body.String(), "data") || strings.Contains(response.Body.String(), calendarFailure.Error()) || strings.Contains(response.Body.String(), start.Format(time.RFC3339)) {
		t.Errorf("partial calendar result or private error escaped HTTP response: %s", response.Body.String())
	}
	var envelope map[string]json.RawMessage
	if decodeErr := json.Unmarshal(response.Body.Bytes(), &envelope); decodeErr != nil {
		t.Errorf("invalid error response JSON: %v", decodeErr)
	} else if len(envelope) != 1 || envelope["error"] == nil {
		t.Errorf("error envelope keys=%v, want only error", envelope)
	}
	if backend.listingReads != 1 || backend.availabilityReads != 1 || backend.availabilityChecks != 0 || backend.quoteWrites != 0 {
		t.Errorf("partial calendar error flow effects: listing=%d calendar=%d rechecks=%d quotes=%d", backend.listingReads, backend.availabilityReads, backend.availabilityChecks, backend.quoteWrites)
	}
}

func TestSyntheticHTTPFlowAvailabilityQuoteAndContractPreview(t *testing.T) {
	const ownerID = "44444444-4444-4444-8444-444444444444"
	const renterID = "33333333-3333-4333-8333-333333333333"
	const spaceTitle = "قاعة عمل"
	now := time.Date(2030, time.January, 1, 12, 0, 0, 0, time.UTC)
	start := time.Date(2030, time.January, 2, 13, 0, 0, 0, time.UTC)
	backend := &syntheticFlowBackend{
		listing:  rentalflow.Listing{ID: testSpaceID, OwnerID: ownerID, State: "activa", Title: spaceTitle, RateUnit: "hora", UnitPriceCLP: 12000, Currency: "CLP", RateVersion: 2, TimeZone: "America/Santiago"},
		interval: rentalflow.Interval{StartAt: start, EndAt: start.Add(time.Hour)}, available: true,
	}
	gate := &syntheticIdentityGate{ownerID: ownerID, renterID: renterID}
	identities := &syntheticIdentityReader{quoteID: testQuoteID, parties: map[string]rentalflow.ContractIdentity{
		ownerID:  {SubjectID: ownerID, LegalName: "Arrendador de prueba", RUT: "11.111.111-1"},
		renterID: {SubjectID: renterID, LegalName: "Arrendataria de prueba", RUT: "22.222.222-2"},
	}}
	clock := func() time.Time { return now }
	availability, err := rentalflow.NewAvailabilityService(backend, backend, clock)
	if err != nil {
		t.Fatal(err)
	}
	quotes, err := rentalflow.NewQuoteService(backend, backend, backend, clock)
	if err != nil {
		t.Fatal(err)
	}
	previews, err := rentalflow.NewContractPreviewService(backend, backend, backend, gate, identities, clock)
	if err != nil {
		t.Fatal(err)
	}
	auth := &authStub{principal: identity.Principal{AccountID: renterID}}
	handler := New(auth, availability, quotes, previews, nil)

	availabilityResponse := httptest.NewRecorder()
	handler.ServeHTTP(availabilityResponse, httptest.NewRequest(http.MethodGet,
		"/api/v1/spaces/"+testSpaceID+"/rental-availability?date=2030-01-02&duration=1", nil))
	if availabilityResponse.Code != http.StatusOK {
		t.Fatalf("availability status=%d body=%s", availabilityResponse.Code, availabilityResponse.Body.String())
	}
	var availabilityEnvelope struct {
		Data rentalflow.Availability `json:"data"`
	}
	if err := json.Unmarshal(availabilityResponse.Body.Bytes(), &availabilityEnvelope); err != nil {
		t.Fatal(err)
	}
	if len(availabilityEnvelope.Data.Items) != 1 || !availabilityEnvelope.Data.Items[0].StartAt.Equal(start) {
		t.Fatalf("unexpected public availability: %+v", availabilityEnvelope.Data)
	}
	for _, forbidden := range []string{ownerID, renterID, "owner_id", "renter_id", "reservation_id", "occupant", "block_reason", "legal_name", "rut"} {
		if strings.Contains(availabilityResponse.Body.String(), forbidden) {
			t.Fatalf("public availability response leaked %q: %s", forbidden, availabilityResponse.Body.String())
		}
	}

	quoteBody := `{"space_id":"` + testSpaceID + `","start_at":"` + start.Format(time.RFC3339) + `","end_at":"` + start.Add(time.Hour).Format(time.RFC3339) + `"}`
	quoteRequest := httptest.NewRequest(http.MethodPost, "/api/v1/rental-quotes", strings.NewReader(quoteBody))
	quoteRequest.Header.Set("Content-Type", "application/json")
	quoteRequest.Header.Set("Authorization", "Bearer synthetic-session")
	quoteResponse := httptest.NewRecorder()
	handler.ServeHTTP(quoteResponse, quoteRequest)
	if quoteResponse.Code != http.StatusCreated {
		t.Fatalf("quote status=%d body=%s", quoteResponse.Code, quoteResponse.Body.String())
	}
	var quoteEnvelope struct {
		Data rentalflow.Quote `json:"data"`
	}
	if err := json.Unmarshal(quoteResponse.Body.Bytes(), &quoteEnvelope); err != nil {
		t.Fatal(err)
	}
	if quoteEnvelope.Data.ID != testQuoteID || quoteEnvelope.Data.SubtotalCLP != 12000 || quoteEnvelope.Data.Currency != "CLP" || quoteEnvelope.Data.SpaceTitle != spaceTitle {
		t.Fatalf("unexpected quote snapshot: %+v", quoteEnvelope.Data)
	}
	if strings.Contains(quoteResponse.Body.String(), "rate_version") || strings.Contains(quoteResponse.Body.String(), ownerID) || strings.Contains(quoteResponse.Body.String(), renterID) {
		t.Fatalf("quote response leaked internal fields: %s", quoteResponse.Body.String())
	}
	if gate.calls != 0 || identities.calls != 0 {
		t.Fatalf("quote request reached contract identity gate/reader early: gate=%d identityReads=%d", gate.calls, identities.calls)
	}
	availabilityAfterQuote := httptest.NewRecorder()
	handler.ServeHTTP(availabilityAfterQuote, httptest.NewRequest(http.MethodGet,
		"/api/v1/spaces/"+testSpaceID+"/rental-availability?date=2030-01-02&duration=1", nil))
	var availableAgain struct {
		Data rentalflow.Availability `json:"data"`
	}
	if err := json.Unmarshal(availabilityAfterQuote.Body.Bytes(), &availableAgain); err != nil {
		t.Fatal(err)
	}
	if availabilityAfterQuote.Code != http.StatusOK || len(availableAgain.Data.Items) != 1 ||
		!availableAgain.Data.Items[0].StartAt.Equal(start) || backend.quoteWrites != 1 || backend.availabilityReads != 2 || !backend.available {
		t.Fatalf("quote retained public interval: status=%d items=%+v publicReads=%d quoteWrites=%d available=%v body=%s", availabilityAfterQuote.Code, availableAgain.Data.Items, backend.availabilityReads, backend.quoteWrites, backend.available, availabilityAfterQuote.Body.String())
	}

	previewRequest := httptest.NewRequest(http.MethodPost, "/api/v1/rental-quotes/"+testQuoteID+"/contract-preview", nil)
	previewRequest.Header.Set("Authorization", "Bearer synthetic-session")
	previewResponse := httptest.NewRecorder()
	handler.ServeHTTP(previewResponse, previewRequest)
	if previewResponse.Code != http.StatusOK || previewResponse.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("preview status=%d cache=%q body=%s", previewResponse.Code, previewResponse.Header().Get("Cache-Control"), previewResponse.Body.String())
	}
	var previewEnvelope struct {
		Data rentalflow.ContractPreview `json:"data"`
	}
	if err := json.Unmarshal(previewResponse.Body.Bytes(), &previewEnvelope); err != nil {
		t.Fatal(err)
	}
	if previewEnvelope.Data.State != "preview" || previewEnvelope.Data.SpaceTitle != spaceTitle || len(previewEnvelope.Data.Parties) != 2 || previewEnvelope.Data.Parties[0].RUT != "11111111-1" || previewEnvelope.Data.Parties[1].RUT != "22222222-2" {
		t.Fatalf("unexpected contract preview: %+v", previewEnvelope.Data)
	}
	auth.principal.AccountID = ownerID
	ownerPreviewRequest := httptest.NewRequest(http.MethodPost, "/api/v1/rental-quotes/"+testQuoteID+"/contract-preview", nil)
	ownerPreviewRequest.Header.Set("Authorization", "Bearer synthetic-session")
	ownerPreviewResponse := httptest.NewRecorder()
	handler.ServeHTTP(ownerPreviewResponse, ownerPreviewRequest)
	if ownerPreviewResponse.Code != http.StatusOK || ownerPreviewResponse.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("owner participant preview status=%d cache=%q body=%s", ownerPreviewResponse.Code, ownerPreviewResponse.Header().Get("Cache-Control"), ownerPreviewResponse.Body.String())
	}
	if !strings.Contains(ownerPreviewResponse.Body.String(), "Arrendador de prueba") || !strings.Contains(ownerPreviewResponse.Body.String(), "Arrendataria de prueba") {
		t.Fatalf("owner participant did not receive both verified parties: %s", ownerPreviewResponse.Body.String())
	}

	backend.returnListingToAnyState = true
	backend.listing.State = "retirada"
	withdrawnListingRequest := httptest.NewRequest(http.MethodPost, "/api/v1/rental-quotes/"+testQuoteID+"/contract-preview", nil)
	withdrawnListingRequest.Header.Set("Authorization", "Bearer synthetic-session")
	withdrawnListingResponse := httptest.NewRecorder()
	handler.ServeHTTP(withdrawnListingResponse, withdrawnListingRequest)
	if withdrawnListingResponse.Code != http.StatusConflict || withdrawnListingResponse.Header().Get("Cache-Control") != "no-store" ||
		backend.availabilityChecks != 3 || gate.calls != 2 || identities.calls != 4 ||
		strings.Contains(withdrawnListingResponse.Body.String(), spaceTitle) ||
		strings.Contains(withdrawnListingResponse.Body.String(), "Arrendador de prueba") ||
		strings.Contains(withdrawnListingResponse.Body.String(), "Arrendataria de prueba") {
		t.Fatalf("withdrawn listing reached calendar/identity reads or disclosure: status=%d calendar=%d gate=%d reads=%d body=%s", withdrawnListingResponse.Code, backend.availabilityChecks, gate.calls, identities.calls, withdrawnListingResponse.Body.String())
	}
	backend.listing.State = "activa"
	backend.returnListingToAnyState = false

	backend.listing.UnitPriceCLP = 13000
	backend.listing.RateVersion++
	changedTariffRequest := httptest.NewRequest(http.MethodPost, "/api/v1/rental-quotes/"+testQuoteID+"/contract-preview", nil)
	changedTariffRequest.Header.Set("Authorization", "Bearer synthetic-session")
	changedTariffResponse := httptest.NewRecorder()
	handler.ServeHTTP(changedTariffResponse, changedTariffRequest)
	if changedTariffResponse.Code != http.StatusConflict || changedTariffResponse.Header().Get("Cache-Control") != "no-store" ||
		backend.availabilityChecks != 3 || gate.calls != 2 || identities.calls != 4 ||
		strings.Contains(changedTariffResponse.Body.String(), "13000") ||
		strings.Contains(changedTariffResponse.Body.String(), "Arrendador de prueba") ||
		strings.Contains(changedTariffResponse.Body.String(), "Arrendataria de prueba") {
		t.Fatalf("changed tariff reached calendar/identity reads or disclosure: status=%d calendar=%d gate=%d reads=%d body=%s", changedTariffResponse.Code, backend.availabilityChecks, gate.calls, identities.calls, changedTariffResponse.Body.String())
	}
	backend.listing.UnitPriceCLP = 12000
	backend.listing.RateVersion = 2

	backend.listing.TimeZone = "America/New_York"
	changedZoneRequest := httptest.NewRequest(http.MethodPost, "/api/v1/rental-quotes/"+testQuoteID+"/contract-preview", nil)
	changedZoneRequest.Header.Set("Authorization", "Bearer synthetic-session")
	changedZoneResponse := httptest.NewRecorder()
	handler.ServeHTTP(changedZoneResponse, changedZoneRequest)
	if changedZoneResponse.Code != http.StatusConflict || changedZoneResponse.Header().Get("Cache-Control") != "no-store" ||
		backend.availabilityChecks != 3 || gate.calls != 2 || identities.calls != 4 ||
		strings.Contains(changedZoneResponse.Body.String(), "America/New_York") ||
		strings.Contains(changedZoneResponse.Body.String(), "Arrendador de prueba") ||
		strings.Contains(changedZoneResponse.Body.String(), "Arrendataria de prueba") {
		t.Fatalf("changed time zone reached calendar/identity reads or disclosure: status=%d calendar=%d gate=%d reads=%d body=%s", changedZoneResponse.Code, backend.availabilityChecks, gate.calls, identities.calls, changedZoneResponse.Body.String())
	}
	backend.listing.TimeZone = "America/Santiago"

	backend.listing.OwnerID = "66666666-6666-4666-8666-666666666666"
	transferredListingRequest := httptest.NewRequest(http.MethodPost, "/api/v1/rental-quotes/"+testQuoteID+"/contract-preview", nil)
	transferredListingRequest.Header.Set("Authorization", "Bearer synthetic-session")
	transferredListingResponse := httptest.NewRecorder()
	handler.ServeHTTP(transferredListingResponse, transferredListingRequest)
	if transferredListingResponse.Code != http.StatusConflict || transferredListingResponse.Header().Get("Cache-Control") != "no-store" ||
		backend.availabilityChecks != 3 || gate.calls != 2 || identities.calls != 4 ||
		strings.Contains(transferredListingResponse.Body.String(), "66666666-6666-4666-8666-666666666666") ||
		strings.Contains(transferredListingResponse.Body.String(), "Arrendador de prueba") ||
		strings.Contains(transferredListingResponse.Body.String(), "Arrendataria de prueba") {
		t.Fatalf("transferred listing reached calendar/identity reads or disclosure: status=%d calendar=%d gate=%d reads=%d body=%s", transferredListingResponse.Code, backend.availabilityChecks, gate.calls, identities.calls, transferredListingResponse.Body.String())
	}
	backend.listing.OwnerID = ownerID

	backend.available = false
	staleAvailabilityRequest := httptest.NewRequest(http.MethodPost, "/api/v1/rental-quotes/"+testQuoteID+"/contract-preview", nil)
	staleAvailabilityRequest.Header.Set("Authorization", "Bearer synthetic-session")
	staleAvailabilityResponse := httptest.NewRecorder()
	handler.ServeHTTP(staleAvailabilityResponse, staleAvailabilityRequest)
	if staleAvailabilityResponse.Code != http.StatusConflict || staleAvailabilityResponse.Header().Get("Cache-Control") != "no-store" ||
		gate.calls != 2 || identities.calls != 4 ||
		strings.Contains(staleAvailabilityResponse.Body.String(), "Arrendador de prueba") ||
		strings.Contains(staleAvailabilityResponse.Body.String(), "Arrendataria de prueba") {
		t.Fatalf("stale interval reached identity gate/disclosure: status=%d gate=%d reads=%d body=%s", staleAvailabilityResponse.Code, gate.calls, identities.calls, staleAvailabilityResponse.Body.String())
	}
	backend.available = true
	selectedInterval := backend.interval
	backend.interval = rentalflow.Interval{StartAt: start.Add(time.Hour), EndAt: start.Add(2 * time.Hour)}
	changedSlotRequest := httptest.NewRequest(http.MethodPost, "/api/v1/rental-quotes/"+testQuoteID+"/contract-preview", nil)
	changedSlotRequest.Header.Set("Authorization", "Bearer synthetic-session")
	changedSlotResponse := httptest.NewRecorder()
	handler.ServeHTTP(changedSlotResponse, changedSlotRequest)
	var lastCheckedInterval rentalflow.Interval
	if len(backend.checkedIntervals) > 0 {
		lastCheckedInterval = backend.checkedIntervals[len(backend.checkedIntervals)-1]
	}
	if changedSlotResponse.Code != http.StatusConflict || changedSlotResponse.Header().Get("Cache-Control") != "no-store" ||
		backend.availabilityChecks != 5 || len(backend.checkedIntervals) == 0 ||
		!lastCheckedInterval.StartAt.Equal(selectedInterval.StartAt) || !lastCheckedInterval.EndAt.Equal(selectedInterval.EndAt) ||
		gate.calls != 2 || identities.calls != 4 ||
		strings.Contains(changedSlotResponse.Body.String(), "Arrendador de prueba") ||
		strings.Contains(changedSlotResponse.Body.String(), "Arrendataria de prueba") {
		t.Fatalf("changed slot did not invalidate the original quote interval: status=%d checked=%+v gate=%d reads=%d body=%s", changedSlotResponse.Code, lastCheckedInterval, gate.calls, identities.calls, changedSlotResponse.Body.String())
	}
	backend.interval = selectedInterval

	backend.quote.ExpiresAt = now
	expiredQuoteRequest := httptest.NewRequest(http.MethodPost, "/api/v1/rental-quotes/"+testQuoteID+"/contract-preview", nil)
	expiredQuoteRequest.Header.Set("Authorization", "Bearer synthetic-session")
	expiredQuoteResponse := httptest.NewRecorder()
	handler.ServeHTTP(expiredQuoteResponse, expiredQuoteRequest)
	if expiredQuoteResponse.Code != http.StatusConflict || gate.calls != 2 || identities.calls != 4 ||
		strings.Contains(expiredQuoteResponse.Body.String(), "Arrendador de prueba") ||
		strings.Contains(expiredQuoteResponse.Body.String(), "Arrendataria de prueba") {
		t.Fatalf("expired quote reached identity gate/disclosure: status=%d gate=%d reads=%d body=%s", expiredQuoteResponse.Code, gate.calls, identities.calls, expiredQuoteResponse.Body.String())
	}
	backend.quote.ExpiresAt = now.Add(15 * time.Minute)

	const unrelatedActorID = "ffffffff-ffff-4fff-8fff-ffffffffffff"
	// Deliberately weaken the fake repository so this request reaches the
	// service's own participant check instead of passing only the adapter guard.
	backend.returnQuoteToAnyActor = true
	auth.principal.AccountID = unrelatedActorID
	nonparticipantRequest := httptest.NewRequest(http.MethodPost, "/api/v1/rental-quotes/"+testQuoteID+"/contract-preview", nil)
	nonparticipantRequest.Header.Set("Authorization", "Bearer synthetic-session")
	nonparticipantResponse := httptest.NewRecorder()
	handler.ServeHTTP(nonparticipantResponse, nonparticipantRequest)
	if nonparticipantResponse.Code != http.StatusNotFound || gate.calls != 2 || identities.calls != 4 ||
		strings.Contains(nonparticipantResponse.Body.String(), "Arrendador de prueba") ||
		strings.Contains(nonparticipantResponse.Body.String(), "Arrendataria de prueba") ||
		strings.Contains(nonparticipantResponse.Body.String(), "11.111.111-1") ||
		strings.Contains(nonparticipantResponse.Body.String(), "22.222.222-2") {
		t.Fatalf("nonparticipant could reach identity gate or disclosure: status=%d gate=%d reads=%d body=%s", nonparticipantResponse.Code, gate.calls, identities.calls, nonparticipantResponse.Body.String())
	}
	if backend.quoteWrites != 1 || backend.availabilityChecks != 5 || !backend.available || gate.calls != 2 || identities.calls != 4 {
		t.Fatalf("synthetic flow invariants failed: backend=%+v gate=%+v identityReads=%d", backend, gate, identities.calls)
	}
	if auth.calls != 11 {
		t.Fatalf("authenticated operation count=%d, want quote and ten preview attempts", auth.calls)
	}
}

func TestSyntheticHTTPContractPreviewFailsClosedOnMalformedOrUnavailableIdentity(t *testing.T) {
	cases := []struct {
		name          string
		party         string
		legalName     string
		rut           string
		readerFailure string
		wantReads     int
	}{
		{name: "bidi control", legalName: "Arrendador\u202e sintético", rut: "11.111.111-1", wantReads: 1},
		{name: "invalid UTF-8", legalName: string([]byte{0xff}), rut: "11.111.111-1", wantReads: 1},
		{name: "Unicode line separator", legalName: "Arrendador\u2028sintético", rut: "11.111.111-1", wantReads: 1},
		{name: "invalid RUT checksum", legalName: "Arrendador sintético", rut: "11.111.111-2", wantReads: 1},
		{name: "renter RUT invalid after owner read", party: "renter", legalName: "Arrendataria sintética", rut: "22.222.222-3", wantReads: 2},
		{name: "renter reader fails after owner read", party: "renter", legalName: "Arrendataria sintética", rut: "22.222.222-2", readerFailure: "renter", wantReads: 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			const ownerID = "44444444-4444-4444-8444-444444444444"
			const renterID = "33333333-3333-4333-8333-333333333333"
			now := time.Date(2030, time.January, 1, 12, 0, 0, 0, time.UTC)
			start := time.Date(2030, time.January, 2, 13, 0, 0, 0, time.UTC)
			ownerName, ownerRUT := "Arrendador sintético", "11.111.111-1"
			renterName, renterRUT := "Arrendataria sintética", "22.222.222-2"
			if tc.party == "renter" {
				renterName, renterRUT = tc.legalName, tc.rut
			} else {
				ownerName, ownerRUT = tc.legalName, tc.rut
			}
			backend := &syntheticFlowBackend{
				listing:  rentalflow.Listing{ID: testSpaceID, OwnerID: ownerID, State: "activa", Title: "Sala sintética", RateUnit: "hora", UnitPriceCLP: 12000, Currency: "CLP", RateVersion: 1, TimeZone: "America/Santiago"},
				interval: rentalflow.Interval{StartAt: start, EndAt: start.Add(time.Hour)}, available: true,
			}
			identities := &syntheticIdentityReader{quoteID: testQuoteID, parties: map[string]rentalflow.ContractIdentity{
				ownerID:  {SubjectID: ownerID, LegalName: ownerName, RUT: ownerRUT},
				renterID: {SubjectID: renterID, LegalName: renterName, RUT: renterRUT},
			}}
			if tc.readerFailure == "renter" {
				identities.errBySubject = map[string]error{renterID: errors.New("private synthetic renter identity reader failure")}
			}
			clock := func() time.Time { return now }
			availability, err := rentalflow.NewAvailabilityService(backend, backend, clock)
			if err != nil {
				t.Fatal(err)
			}
			quotes, err := rentalflow.NewQuoteService(backend, backend, backend, clock)
			if err != nil {
				t.Fatal(err)
			}
			previews, err := rentalflow.NewContractPreviewService(backend, backend, backend, &syntheticIdentityGate{ownerID: ownerID, renterID: renterID}, identities, clock)
			if err != nil {
				t.Fatal(err)
			}
			handler := New(&authStub{principal: identity.Principal{AccountID: renterID}}, availability, quotes, previews, nil)

			quoteBody := `{"space_id":"` + testSpaceID + `","start_at":"` + start.Format(time.RFC3339) + `","end_at":"` + start.Add(time.Hour).Format(time.RFC3339) + `"}`
			quoteRequest := httptest.NewRequest(http.MethodPost, "/api/v1/rental-quotes", strings.NewReader(quoteBody))
			quoteRequest.Header.Set("Content-Type", "application/json")
			quoteRequest.Header.Set("Authorization", "Bearer synthetic-session")
			quoteResponse := httptest.NewRecorder()
			handler.ServeHTTP(quoteResponse, quoteRequest)
			if quoteResponse.Code != http.StatusCreated {
				t.Fatalf("synthetic quote status=%d body=%s", quoteResponse.Code, quoteResponse.Body.String())
			}

			previewRequest := httptest.NewRequest(http.MethodPost, "/api/v1/rental-quotes/"+testQuoteID+"/contract-preview", nil)
			previewRequest.Header.Set("Authorization", "Bearer synthetic-session")
			previewResponse := httptest.NewRecorder()
			handler.ServeHTTP(previewResponse, previewRequest)
			var errorEnvelope struct {
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			if err := json.Unmarshal(previewResponse.Body.Bytes(), &errorEnvelope); err != nil {
				t.Fatalf("invalid preview error envelope: %v body=%s", err, previewResponse.Body.String())
			}
			if previewResponse.Code != http.StatusServiceUnavailable || errorEnvelope.Error.Code != "service_unavailable" ||
				previewResponse.Header().Get("Cache-Control") != "no-store" || identities.calls != tc.wantReads ||
				strings.Contains(previewResponse.Body.String(), ownerName) || strings.Contains(previewResponse.Body.String(), ownerRUT) ||
				strings.Contains(previewResponse.Body.String(), renterName) || strings.Contains(previewResponse.Body.String(), renterRUT) ||
				strings.Contains(previewResponse.Body.String(), "\uFFFD") || strings.Contains(previewResponse.Body.String(), "private synthetic renter identity reader failure") {
				t.Fatalf("%s was not hidden at HTTP boundary: status=%d code=%q cache=%q reads=%d body=%s", tc.name, previewResponse.Code, errorEnvelope.Error.Code, previewResponse.Header().Get("Cache-Control"), identities.calls, previewResponse.Body.String())
			}
		})
	}
}

func TestSyntheticHTTPPreviewRejectsUnsafeListingTitleBeforeIdentityDisclosure(t *testing.T) {
	const ownerID = "44444444-4444-4444-8444-444444444444"
	const renterID = "33333333-3333-4333-8333-333333333333"
	now := time.Date(2030, time.January, 1, 12, 0, 0, 0, time.UTC)
	start := time.Date(2030, time.January, 2, 13, 0, 0, 0, time.UTC)
	backend := &syntheticFlowBackend{
		listing:  rentalflow.Listing{ID: testSpaceID, OwnerID: ownerID, State: "activa", Title: "Sala sintética", RateUnit: "hora", UnitPriceCLP: 12000, Currency: "CLP", RateVersion: 1, TimeZone: "America/Santiago"},
		interval: rentalflow.Interval{StartAt: start, EndAt: start.Add(time.Hour)}, available: true,
	}
	gate := &syntheticIdentityGate{ownerID: ownerID, renterID: renterID}
	identities := &syntheticIdentityReader{quoteID: testQuoteID, parties: map[string]rentalflow.ContractIdentity{
		ownerID:  {SubjectID: ownerID, LegalName: "Arrendador sintético", RUT: "11.111.111-1"},
		renterID: {SubjectID: renterID, LegalName: "Arrendataria sintética", RUT: "22.222.222-2"},
	}}
	clock := func() time.Time { return now }
	availability, err := rentalflow.NewAvailabilityService(backend, backend, clock)
	if err != nil {
		t.Fatal(err)
	}
	quotes, err := rentalflow.NewQuoteService(backend, backend, backend, clock)
	if err != nil {
		t.Fatal(err)
	}
	previews, err := rentalflow.NewContractPreviewService(backend, backend, backend, gate, identities, clock)
	if err != nil {
		t.Fatal(err)
	}
	handler := New(&authStub{principal: identity.Principal{AccountID: renterID}}, availability, quotes, previews, nil)
	quoteBody := `{"space_id":"` + testSpaceID + `","start_at":"` + start.Format(time.RFC3339) + `","end_at":"` + start.Add(time.Hour).Format(time.RFC3339) + `"}`
	quoteRequest := httptest.NewRequest(http.MethodPost, "/api/v1/rental-quotes", strings.NewReader(quoteBody))
	quoteRequest.Header.Set("Content-Type", "application/json")
	quoteRequest.Header.Set("Authorization", "Bearer synthetic-session")
	quoteResponse := httptest.NewRecorder()
	handler.ServeHTTP(quoteResponse, quoteRequest)
	if quoteResponse.Code != http.StatusCreated || backend.quoteWrites != 1 {
		t.Fatalf("synthetic quote setup failed: status=%d writes=%d body=%s", quoteResponse.Code, backend.quoteWrites, quoteResponse.Body.String())
	}

	backend.listing.Title = "Sala\u202eprivada"
	previewRequest := httptest.NewRequest(http.MethodPost, "/api/v1/rental-quotes/"+testQuoteID+"/contract-preview", nil)
	previewRequest.Header.Set("Authorization", "Bearer synthetic-session")
	previewResponse := httptest.NewRecorder()
	handler.ServeHTTP(previewResponse, previewRequest)
	var envelope struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(previewResponse.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("invalid preview error envelope: %v body=%s", err, previewResponse.Body.String())
	}
	if previewResponse.Code != http.StatusConflict || envelope.Error.Code != "conflict" ||
		previewResponse.Header().Get("Cache-Control") != "no-store" || gate.calls != 0 || identities.calls != 0 ||
		backend.availabilityChecks != 1 || strings.Contains(previewResponse.Body.String(), "Sala") ||
		strings.Contains(previewResponse.Body.String(), "11.111.111-1") {
		t.Fatalf("unsafe listing title reached private gates or disclosure: status=%d code=%q cache=%q gate=%d identityReads=%d availabilityChecks=%d body=%s", previewResponse.Code, envelope.Error.Code, previewResponse.Header().Get("Cache-Control"), gate.calls, identities.calls, backend.availabilityChecks, previewResponse.Body.String())
	}
}

func TestSyntheticHTTPQuoteHidesUnsafeListingTitleBeforeAvailabilityOrWrite(t *testing.T) {
	const ownerID = "44444444-4444-4444-8444-444444444444"
	const renterID = "33333333-3333-4333-8333-333333333333"
	now := time.Date(2030, time.January, 1, 12, 0, 0, 0, time.UTC)
	start := time.Date(2030, time.January, 2, 13, 0, 0, 0, time.UTC)
	backend := &syntheticFlowBackend{
		listing:  rentalflow.Listing{ID: testSpaceID, OwnerID: ownerID, State: "activa", Title: "Sala\nprivada", RateUnit: "hora", UnitPriceCLP: 12000, Currency: "CLP", RateVersion: 1, TimeZone: "America/Santiago"},
		interval: rentalflow.Interval{StartAt: start, EndAt: start.Add(time.Hour)}, available: true,
	}
	clock := func() time.Time { return now }
	availability, err := rentalflow.NewAvailabilityService(backend, backend, clock)
	if err != nil {
		t.Fatal(err)
	}
	quotes, err := rentalflow.NewQuoteService(backend, backend, backend, clock)
	if err != nil {
		t.Fatal(err)
	}
	handler := New(&authStub{principal: identity.Principal{AccountID: renterID}}, availability, quotes, nil, nil)
	quoteBody := `{"space_id":"` + testSpaceID + `","start_at":"` + start.Format(time.RFC3339) + `","end_at":"` + start.Add(time.Hour).Format(time.RFC3339) + `"}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/rental-quotes", strings.NewReader(quoteBody))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer synthetic-session")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	var envelope struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("invalid quote error envelope: %v body=%s", err, response.Body.String())
	}
	if response.Code != http.StatusServiceUnavailable || envelope.Error.Code != "service_unavailable" ||
		response.Header().Get("Cache-Control") != "no-store" || backend.availabilityChecks != 0 || backend.quoteWrites != 0 ||
		strings.Contains(response.Body.String(), "Sala") || strings.Contains(response.Body.String(), "privada") {
		t.Fatalf("unsafe listing title was exposed or reached downstream: status=%d code=%q cache=%q availability=%d writes=%d body=%s", response.Code, envelope.Error.Code, response.Header().Get("Cache-Control"), backend.availabilityChecks, backend.quoteWrites, response.Body.String())
	}
}

func TestSyntheticHTTPAvailabilityCharacterizesCurrentDurationMatrix(t *testing.T) {
	now := time.Date(2030, time.January, 1, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		unit     string
		duration int
	}{
		{unit: "hora", duration: 3},
		{unit: "dia", duration: 4},
		{unit: "mes", duration: 2},
	} {
		t.Run(tc.unit, func(t *testing.T) {
			backend := &syntheticFlowBackend{
				listing: rentalflow.Listing{
					ID: testSpaceID, OwnerID: "44444444-4444-4444-8444-444444444444",
					State: "activa", Title: "Espacio sintético", RateUnit: tc.unit,
					UnitPriceCLP: 12000, Currency: "CLP", RateVersion: 1, TimeZone: "America/Santiago",
				},
				available: true,
			}
			availability, err := rentalflow.NewAvailabilityService(backend, backend, func() time.Time { return now })
			if err != nil {
				t.Fatal(err)
			}
			handler := New(nil, availability, nil, nil, nil)
			url := "/api/v1/spaces/" + testSpaceID + "/rental-availability?date=2030-01-02&duration=" + strconv.Itoa(tc.duration)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, url, nil))
			var envelope struct {
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
				t.Fatalf("invalid error envelope: %v body=%s", err, response.Body.String())
			}
			if response.Code != http.StatusUnprocessableEntity || envelope.Error.Code != "invalid_request" ||
				response.Header().Get("Cache-Control") != "no-store" || backend.availabilityReads != 0 {
				t.Fatalf("unit=%s duration=%d status=%d code=%q cache=%q availability_reads=%d body=%s", tc.unit, tc.duration, response.Code, envelope.Error.Code, response.Header().Get("Cache-Control"), backend.availabilityReads, response.Body.String())
			}
		})
	}
}

func TestSyntheticHTTPAvailabilityAndQuoteAgreeForDailyDSTAndMonthlyClampedIntervals(t *testing.T) {
	loc := mustLoadLocation("America/Santiago")
	for _, tc := range []struct {
		name        string
		unit        string
		now         time.Time
		start       time.Time
		end         time.Time
		wantElapsed time.Duration
	}{
		{
			name:  "daily interval across skipped midnight",
			unit:  "dia",
			now:   time.Date(2026, time.September, 5, 12, 0, 0, 0, loc),
			start: time.Date(2026, time.September, 6, 1, 0, 0, 0, loc),
			end:   time.Date(2026, time.September, 7, 0, 0, 0, 0, loc),
		},
		{
			name:        "daily interval across repeated hour",
			unit:        "dia",
			now:         time.Date(2026, time.April, 3, 12, 0, 0, 0, loc),
			start:       time.Date(2026, time.April, 4, 0, 0, 0, 0, loc),
			end:         time.Date(2026, time.April, 5, 0, 0, 0, 0, loc),
			wantElapsed: 25 * time.Hour,
		},
		{
			name:  "monthly interval clamps January 31",
			unit:  "mes",
			now:   time.Date(2030, time.January, 1, 12, 0, 0, 0, loc),
			start: time.Date(2030, time.January, 31, 0, 0, 0, 0, loc),
			end:   time.Date(2030, time.February, 28, 0, 0, 0, 0, loc),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.wantElapsed != 0 && tc.end.Sub(tc.start) != tc.wantElapsed {
				t.Fatalf("synthetic interval elapsed=%s, want %s", tc.end.Sub(tc.start), tc.wantElapsed)
			}
			const ownerID = "44444444-4444-4444-8444-444444444444"
			const renterID = testActorID
			backend := &syntheticFlowBackend{
				listing: rentalflow.Listing{
					ID: testSpaceID, OwnerID: ownerID, State: "activa", Title: "Espacio calendario sintético",
					RateUnit: tc.unit, UnitPriceCLP: 12000, Currency: "CLP", RateVersion: 2, TimeZone: loc.String(),
				},
				interval:       rentalflow.Interval{StartAt: tc.start, EndAt: tc.end},
				publicDuration: 1,
				available:      true,
			}
			clock := func() time.Time { return tc.now }
			availability, err := rentalflow.NewAvailabilityService(backend, backend, clock)
			if err != nil {
				t.Fatal(err)
			}
			quotes, err := rentalflow.NewQuoteService(backend, backend, backend, clock)
			if err != nil {
				t.Fatal(err)
			}
			gate := &syntheticIdentityGate{ownerID: ownerID, renterID: renterID}
			identities := &syntheticIdentityReader{quoteID: testQuoteID, parties: map[string]rentalflow.ContractIdentity{
				ownerID:  {SubjectID: ownerID, LegalName: "Arrendador sintético", RUT: "11.111.111-1"},
				renterID: {SubjectID: renterID, LegalName: "Arrendataria sintética", RUT: "22.222.222-2"},
			}}
			previews, err := rentalflow.NewContractPreviewService(backend, backend, backend, gate, identities, clock)
			if err != nil {
				t.Fatal(err)
			}
			auth := &authStub{principal: identity.Principal{AccountID: renterID}}
			handler := New(auth, availability, quotes, previews, nil)
			date := tc.start.In(loc).Format("2006-01-02")
			availabilityResponse := httptest.NewRecorder()
			handler.ServeHTTP(availabilityResponse, httptest.NewRequest(http.MethodGet,
				"/api/v1/spaces/"+testSpaceID+"/rental-availability?date="+date+"&duration=1", nil))
			var availabilityEnvelope struct {
				Data rentalflow.Availability `json:"data"`
			}
			if err := json.Unmarshal(availabilityResponse.Body.Bytes(), &availabilityEnvelope); err != nil {
				t.Fatalf("invalid availability envelope: %v body=%s", err, availabilityResponse.Body.String())
			}
			if availabilityResponse.Code != http.StatusOK || len(availabilityEnvelope.Data.Items) != 1 ||
				!availabilityEnvelope.Data.Items[0].StartAt.Equal(tc.start) || !availabilityEnvelope.Data.Items[0].EndAt.Equal(tc.end) {
				t.Fatalf("availability status=%d data=%+v body=%s", availabilityResponse.Code, availabilityEnvelope.Data, availabilityResponse.Body.String())
			}

			body, err := json.Marshal(rentalflow.QuoteRequest{
				SpaceID: testSpaceID, StartAt: tc.start.Format(time.RFC3339Nano), EndAt: tc.end.Format(time.RFC3339Nano),
			})
			if err != nil {
				t.Fatal(err)
			}
			request := httptest.NewRequest(http.MethodPost, "/api/v1/rental-quotes", strings.NewReader(string(body)))
			request.Header.Set("Authorization", "Bearer synthetic-session")
			request.Header.Set("Content-Type", "application/json")
			quoteResponse := httptest.NewRecorder()
			handler.ServeHTTP(quoteResponse, request)
			var quoteEnvelope struct {
				Data rentalflow.Quote `json:"data"`
			}
			if err := json.Unmarshal(quoteResponse.Body.Bytes(), &quoteEnvelope); err != nil {
				t.Fatalf("invalid quote envelope: %v body=%s", err, quoteResponse.Body.String())
			}
			if quoteResponse.Code != http.StatusCreated || quoteEnvelope.Data.Units != 1 || quoteEnvelope.Data.SubtotalCLP != 12000 ||
				backend.availabilityReads != 1 || backend.availabilityChecks != 1 || backend.quoteWrites != 1 {
				t.Fatalf("quote status=%d data=%+v reads=%d checks=%d writes=%d body=%s", quoteResponse.Code, quoteEnvelope.Data, backend.availabilityReads, backend.availabilityChecks, backend.quoteWrites, quoteResponse.Body.String())
			}

			previewRequest := httptest.NewRequest(http.MethodPost, "/api/v1/rental-quotes/"+testQuoteID+"/contract-preview", nil)
			previewRequest.Header.Set("Authorization", "Bearer synthetic-session")
			previewResponse := httptest.NewRecorder()
			handler.ServeHTTP(previewResponse, previewRequest)
			var previewEnvelope struct {
				Data rentalflow.ContractPreview `json:"data"`
			}
			if err := json.Unmarshal(previewResponse.Body.Bytes(), &previewEnvelope); err != nil {
				t.Fatalf("invalid preview envelope: %v body=%s", err, previewResponse.Body.String())
			}
			if previewResponse.Code != http.StatusOK || previewResponse.Header().Get("Cache-Control") != "no-store" ||
				previewEnvelope.Data.State != "preview" || previewEnvelope.Data.Units != 1 || previewEnvelope.Data.SubtotalCLP != 12000 ||
				len(previewEnvelope.Data.Parties) != 2 || backend.availabilityChecks != 2 || backend.quoteWrites != 1 ||
				gate.calls != 1 || identities.calls != 2 {
				t.Fatalf("preview status=%d cache=%q data=%+v checks=%d writes=%d gate=%d identity_reads=%d body=%s", previewResponse.Code, previewResponse.Header().Get("Cache-Control"), previewEnvelope.Data, backend.availabilityChecks, backend.quoteWrites, gate.calls, identities.calls, previewResponse.Body.String())
			}
		})
	}
}

func TestSyntheticHTTPQuoteCharacterizesCurrentDurationMatrix(t *testing.T) {
	loc := mustLoadLocation("America/Santiago")
	now := time.Date(2030, time.January, 1, 12, 0, 0, 0, time.UTC)
	start := time.Date(2030, time.January, 2, 0, 0, 0, 0, loc)
	for _, tc := range []struct {
		name  string
		unit  string
		start time.Time
		end   time.Time
	}{
		{name: "hourly three hours", unit: "hora", start: start.Add(10 * time.Hour), end: start.Add(13 * time.Hour)},
		{name: "daily four days", unit: "dia", start: start, end: start.AddDate(0, 0, 4)},
		{name: "monthly two months", unit: "mes", start: start, end: start.AddDate(0, 2, 0)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			backend := &syntheticFlowBackend{
				listing: rentalflow.Listing{
					ID: testSpaceID, OwnerID: "44444444-4444-4444-8444-444444444444",
					State: "activa", Title: "Espacio sintético", RateUnit: tc.unit,
					UnitPriceCLP: 12000, Currency: "CLP", RateVersion: 1, TimeZone: loc.String(),
				},
				available: true,
			}
			quotes, err := rentalflow.NewQuoteService(backend, backend, backend, func() time.Time { return now })
			if err != nil {
				t.Fatal(err)
			}
			auth := &authStub{principal: identity.Principal{AccountID: testActorID}}
			handler := New(auth, nil, quotes, nil, nil)
			body, err := json.Marshal(rentalflow.QuoteRequest{
				SpaceID: testSpaceID, StartAt: tc.start.Format(time.RFC3339Nano), EndAt: tc.end.Format(time.RFC3339Nano),
			})
			if err != nil {
				t.Fatal(err)
			}
			request := httptest.NewRequest(http.MethodPost, "/api/v1/rental-quotes", strings.NewReader(string(body)))
			request.Header.Set("Authorization", "Bearer synthetic-session")
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			var envelope struct {
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
				t.Fatalf("invalid error envelope: %v body=%s", err, response.Body.String())
			}
			if response.Code != http.StatusUnprocessableEntity || envelope.Error.Code != "invalid_request" ||
				response.Header().Get("Cache-Control") != "no-store" || auth.calls != 1 ||
				backend.availabilityChecks != 0 || backend.quoteWrites != 0 {
				t.Fatalf("unit=%s status=%d code=%q cache=%q auth=%d availability_checks=%d quote_writes=%d body=%s", tc.unit, response.Code, envelope.Error.Code, response.Header().Get("Cache-Control"), auth.calls, backend.availabilityChecks, backend.quoteWrites, response.Body.String())
			}
		})
	}
}

func TestSyntheticHTTPFlowAcrossRepeatedDSTHour(t *testing.T) {
	const ownerID = "44444444-4444-4444-8444-444444444444"
	const renterID = "33333333-3333-4333-8333-333333333333"
	now := time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC)
	start, err := time.Parse(time.RFC3339, "2026-11-01T01:30:00-04:00")
	if err != nil {
		t.Fatal(err)
	}
	end, err := time.Parse(time.RFC3339, "2026-11-01T01:30:00-05:00")
	if err != nil {
		t.Fatal(err)
	}
	backend := &syntheticFlowBackend{
		listing:  rentalflow.Listing{ID: testSpaceID, OwnerID: ownerID, State: "activa", Title: "Sala DST sintética", RateUnit: "hora", UnitPriceCLP: 12000, Currency: "CLP", RateVersion: 3, TimeZone: "America/New_York"},
		interval: rentalflow.Interval{StartAt: start, EndAt: end}, available: true,
	}
	gate := &syntheticIdentityGate{ownerID: ownerID, renterID: renterID}
	identities := &syntheticIdentityReader{quoteID: testQuoteID, parties: map[string]rentalflow.ContractIdentity{
		ownerID:  {SubjectID: ownerID, LegalName: "Arrendador de prueba", RUT: "11.111.111-1"},
		renterID: {SubjectID: renterID, LegalName: "Arrendataria de prueba", RUT: "22.222.222-2"},
	}}
	clock := func() time.Time { return now }
	availability, err := rentalflow.NewAvailabilityService(backend, backend, clock)
	if err != nil {
		t.Fatal(err)
	}
	quotes, err := rentalflow.NewQuoteService(backend, backend, backend, clock)
	if err != nil {
		t.Fatal(err)
	}
	previews, err := rentalflow.NewContractPreviewService(backend, backend, backend, gate, identities, clock)
	if err != nil {
		t.Fatal(err)
	}
	auth := &authStub{principal: identity.Principal{AccountID: renterID}}
	handler := New(auth, availability, quotes, previews, nil)

	availabilityResponse := httptest.NewRecorder()
	handler.ServeHTTP(availabilityResponse, httptest.NewRequest(http.MethodGet,
		"/api/v1/spaces/"+testSpaceID+"/rental-availability?date=2026-11-01&duration=1", nil))
	if availabilityResponse.Code != http.StatusOK {
		t.Fatalf("availability status=%d body=%s", availabilityResponse.Code, availabilityResponse.Body.String())
	}
	var availabilityEnvelope struct {
		Data rentalflow.Availability `json:"data"`
	}
	if err := json.Unmarshal(availabilityResponse.Body.Bytes(), &availabilityEnvelope); err != nil {
		t.Fatal(err)
	}
	if len(availabilityEnvelope.Data.Items) != 1 || !availabilityEnvelope.Data.Items[0].StartAt.Equal(start.UTC()) ||
		!availabilityEnvelope.Data.Items[0].EndAt.Equal(end.UTC()) {
		t.Fatalf("public availability lost DST-fold instants: %+v", availabilityEnvelope.Data)
	}

	quoteBody := `{"space_id":"` + testSpaceID + `","start_at":"` + start.Format(time.RFC3339) + `","end_at":"` + end.Format(time.RFC3339) + `"}`
	quoteRequest := httptest.NewRequest(http.MethodPost, "/api/v1/rental-quotes", strings.NewReader(quoteBody))
	quoteRequest.Header.Set("Content-Type", "application/json")
	quoteRequest.Header.Set("Authorization", "Bearer synthetic-session")
	quoteResponse := httptest.NewRecorder()
	handler.ServeHTTP(quoteResponse, quoteRequest)
	if quoteResponse.Code != http.StatusCreated {
		t.Fatalf("quote status=%d body=%s", quoteResponse.Code, quoteResponse.Body.String())
	}
	var quoteEnvelope struct {
		Data rentalflow.Quote `json:"data"`
	}
	if err := json.Unmarshal(quoteResponse.Body.Bytes(), &quoteEnvelope); err != nil {
		t.Fatal(err)
	}
	if quoteEnvelope.Data.Units != 1 || quoteEnvelope.Data.SubtotalCLP != 12000 ||
		!quoteEnvelope.Data.StartAt.Equal(start.UTC()) || !quoteEnvelope.Data.EndAt.Equal(end.UTC()) {
		t.Fatalf("quote did not preserve the one elapsed DST-fold hour: %+v", quoteEnvelope.Data)
	}

	previewRequest := httptest.NewRequest(http.MethodPost, "/api/v1/rental-quotes/"+testQuoteID+"/contract-preview", nil)
	previewRequest.Header.Set("Authorization", "Bearer synthetic-session")
	previewResponse := httptest.NewRecorder()
	handler.ServeHTTP(previewResponse, previewRequest)
	if previewResponse.Code != http.StatusOK {
		t.Fatalf("preview status=%d body=%s", previewResponse.Code, previewResponse.Body.String())
	}
	var previewEnvelope struct {
		Data rentalflow.ContractPreview `json:"data"`
	}
	if err := json.Unmarshal(previewResponse.Body.Bytes(), &previewEnvelope); err != nil {
		t.Fatal(err)
	}
	if previewEnvelope.Data.Units != 1 || previewEnvelope.Data.SubtotalCLP != 12000 ||
		!previewEnvelope.Data.StartAt.Equal(start.UTC()) || !previewEnvelope.Data.EndAt.Equal(end.UTC()) ||
		len(previewEnvelope.Data.Parties) != 2 || gate.calls != 1 || identities.calls != 2 {
		t.Fatalf("preview failed to revalidate DST-fold interval before both identity reads: preview=%+v gate=%d reads=%d", previewEnvelope.Data, gate.calls, identities.calls)
	}
	if backend.quoteWrites != 1 || backend.availabilityReads != 1 || backend.availabilityChecks != 2 || auth.calls != 2 {
		t.Fatalf("synthetic DST flow invariants failed: backend=%+v auth calls=%d", backend, auth.calls)
	}
}

type cancelAfterSecondSyntheticIdentityRead struct {
	quoteID string
	parties map[string]rentalflow.ContractIdentity
	cancel  context.CancelFunc
	calls   int
}

func (r *cancelAfterSecondSyntheticIdentityRead) CurrentForRentalContract(_ context.Context, _, quoteID, subjectID string, _ time.Time) (rentalflow.ContractIdentity, error) {
	r.calls++
	if quoteID != r.quoteID {
		return rentalflow.ContractIdentity{}, rentalflow.ErrNotFound
	}
	identity, ok := r.parties[subjectID]
	if !ok {
		return rentalflow.ContractIdentity{}, rentalflow.ErrIdentityUnavailable
	}
	if r.calls == 2 {
		r.cancel()
	}
	return identity, nil
}

func TestSyntheticHTTPPreviewDoesNotExposeIdentitiesWhenContextCancelsAfterSuccessfulReads(t *testing.T) {
	const ownerID = "44444444-4444-4444-8444-444444444444"
	const renterID = "33333333-3333-4333-8333-333333333333"
	const ownerName = "Identidad Dueño Sintética"
	const renterName = "Identidad Arrendataria Sintética"
	now := time.Date(2030, time.January, 1, 12, 0, 0, 0, time.UTC)
	start := time.Date(2030, time.January, 2, 13, 0, 0, 0, time.UTC)
	listing := rentalflow.Listing{ID: testSpaceID, OwnerID: ownerID, State: "activa", Title: "Sala sintética", RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", RateVersion: 1, TimeZone: "America/Santiago"}
	quote := rentalflow.Quote{ID: testQuoteID, SpaceID: testSpaceID, OwnerID: ownerID, RenterID: renterID, SpaceTitle: listing.Title, RateVersion: listing.RateVersion, RateUnit: listing.RateUnit, UnitPriceCLP: listing.UnitPriceCLP, Currency: "CLP", Units: 1, SubtotalCLP: 8000, StartAt: start, EndAt: start.Add(time.Hour), TimeZone: listing.TimeZone, CreatedAt: now.Add(-time.Minute), ExpiresAt: now.Add(15 * time.Minute)}
	interval := rentalflow.Interval{StartAt: start, EndAt: start.Add(time.Hour)}
	backend := &syntheticFlowBackend{listing: listing, interval: interval, available: true, quote: quote}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	reader := &cancelAfterSecondSyntheticIdentityRead{
		quoteID: testQuoteID,
		cancel:  cancel,
		parties: map[string]rentalflow.ContractIdentity{
			ownerID:  {SubjectID: ownerID, LegalName: ownerName, RUT: "11.111.111-1"},
			renterID: {SubjectID: renterID, LegalName: renterName, RUT: "22.222.222-2"},
		},
	}
	previews, err := rentalflow.NewContractPreviewService(backend, backend, backend,
		&syntheticIdentityGate{ownerID: ownerID, renterID: renterID}, reader, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	handler := New(&authStub{principal: identity.Principal{AccountID: renterID}}, nil, nil, previews, nil)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/rental-quotes/"+testQuoteID+"/contract-preview", nil).WithContext(ctx)
	request.Header.Set("Authorization", "Bearer synthetic-session")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	responseBody := response.Body.String()
	if response.Code != http.StatusServiceUnavailable || !strings.Contains(responseBody, "service_unavailable") ||
		response.Header().Get("Cache-Control") != "no-store" || reader.calls != 2 || strings.Contains(responseBody, `"data"`) ||
		strings.Contains(responseBody, ownerName) || strings.Contains(responseBody, renterName) ||
		strings.Contains(responseBody, "11.111.111-1") || strings.Contains(responseBody, "22.222.222-2") {
		t.Fatalf("canceled preview exposed identity: status=%d reads=%d body=%s", response.Code, reader.calls, responseBody)
	}
}
