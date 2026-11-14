package rentalflowhttp

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
	"github.com/HernanEspinozaDev/espaciGo/internal/rentalflow"
)

type previewCalendarReturningAvailableWithError struct {
	*syntheticFlowBackend
	err    error
	cancel context.CancelFunc
	calls  int
}

func (c *previewCalendarReturningAvailableWithError) IntervalAvailable(context.Context, rentalflow.Listing, rentalflow.Interval) (bool, error) {
	c.calls++
	if c.cancel != nil {
		c.cancel()
	}
	return true, c.err
}

func TestSyntheticHTTPPreviewStopsBeforeIdentityReadsWhenCalendarErrors(t *testing.T) {
	const ownerID = "44444444-4444-4444-8444-444444444444"
	const renterID = "33333333-3333-4333-8333-333333333333"
	const title = "Título sintético reservado"
	const ownerName = "Propietario sintético privado"
	const renterName = "Arrendatario sintético privado"
	now := time.Date(2030, time.January, 1, 12, 0, 0, 0, time.UTC)
	start := time.Date(2030, time.January, 2, 13, 0, 0, 0, time.UTC)
	backend := &syntheticFlowBackend{
		listing: rentalflow.Listing{
			ID: testSpaceID, OwnerID: ownerID, State: "activa", Title: title,
			RateUnit: "hora", UnitPriceCLP: 12000, Currency: "CLP", RateVersion: 2, TimeZone: "America/Santiago",
		},
		quote: rentalflow.Quote{
			ID: testQuoteID, SpaceID: testSpaceID, OwnerID: ownerID, RenterID: renterID, SpaceTitle: title,
			RateVersion: 2, RateUnit: "hora", UnitPriceCLP: 12000, Currency: "CLP", Units: 1, SubtotalCLP: 12000,
			StartAt: start, EndAt: start.Add(time.Hour), TimeZone: "America/Santiago",
			CreatedAt: now, ExpiresAt: now.Add(15 * time.Minute),
		},
	}
	calendar := &previewCalendarReturningAvailableWithError{
		syntheticFlowBackend: backend, err: errors.New("synthetic private calendar backend failure"),
	}
	gate := &syntheticIdentityGate{ownerID: ownerID, renterID: renterID}
	identities := &syntheticIdentityReader{quoteID: testQuoteID, parties: map[string]rentalflow.ContractIdentity{
		ownerID:  {SubjectID: ownerID, LegalName: ownerName, RUT: "11.111.111-1"},
		renterID: {SubjectID: renterID, LegalName: renterName, RUT: "22.222.222-2"},
	}}
	previews, err := rentalflow.NewContractPreviewService(backend, calendar, backend, gate, identities, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	handler := New(&authStub{principal: identity.Principal{AccountID: renterID}}, nil, nil, previews, nil)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/rental-quotes/"+testQuoteID+"/contract-preview", nil)
	request.Header.Set("Authorization", "Bearer synthetic-session")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable || response.Header().Get("Cache-Control") != "no-store" ||
		strings.Contains(response.Body.String(), `"data"`) || strings.Contains(response.Body.String(), "synthetic private calendar backend failure") {
		t.Fatalf("calendar error preview status=%d cache=%q body=%s", response.Code, response.Header().Get("Cache-Control"), response.Body.String())
	}
	for _, forbidden := range []string{title, ownerID, renterID, ownerName, renterName, "11111111-1", "22222222-2", "12000"} {
		if strings.Contains(errorDisclosureBody(t, response.Body.Bytes()), forbidden) {
			t.Fatalf("calendar error preview disclosed %q: %s", forbidden, response.Body.String())
		}
	}
	if backend.listingReads != 1 || calendar.calls != 1 || gate.calls != 0 || identities.calls != 0 {
		t.Fatalf("calendar failure reached identity dependencies: listing=%d calendar=%d gate=%d legal reads=%d", backend.listingReads, calendar.calls, gate.calls, identities.calls)
	}
}

func TestSyntheticHTTPQuoteStopsBeforeWriteWhenCalendarErrors(t *testing.T) {
	const ownerID = "44444444-4444-4444-8444-444444444444"
	const renterID = "33333333-3333-4333-8333-333333333333"
	const title = "Título sintético reservado"
	now := time.Date(2030, time.January, 1, 12, 0, 0, 0, time.UTC)
	start := time.Date(2030, time.January, 2, 13, 0, 0, 0, time.UTC)
	backend := &syntheticFlowBackend{
		listing: rentalflow.Listing{
			ID: testSpaceID, OwnerID: ownerID, State: "activa", Title: title,
			RateUnit: "hora", UnitPriceCLP: 12000, Currency: "CLP", RateVersion: 2, TimeZone: "America/Santiago",
		},
		interval: rentalflow.Interval{StartAt: start, EndAt: start.Add(time.Hour)}, available: true,
	}
	calendar := &previewCalendarReturningAvailableWithError{
		syntheticFlowBackend: backend, err: errors.New("synthetic private calendar backend failure"),
	}
	quotes, err := rentalflow.NewQuoteService(backend, calendar, backend, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	handler := New(&authStub{principal: identity.Principal{AccountID: renterID}}, nil, quotes, nil, nil)
	body := `{"space_id":"` + testSpaceID + `","start_at":"` + start.Format(time.RFC3339) + `","end_at":"` + start.Add(time.Hour).Format(time.RFC3339) + `"}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/rental-quotes", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer synthetic-session")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable || response.Header().Get("Cache-Control") != "no-store" ||
		strings.Contains(response.Body.String(), `"data"`) || strings.Contains(response.Body.String(), "synthetic private calendar backend failure") {
		t.Fatalf("calendar error quote status=%d cache=%q body=%s", response.Code, response.Header().Get("Cache-Control"), response.Body.String())
	}
	for _, forbidden := range []string{title, ownerID, renterID, testQuoteID, "12000", "rate_version"} {
		if strings.Contains(errorDisclosureBody(t, response.Body.Bytes()), forbidden) {
			t.Fatalf("calendar error quote disclosed %q: %s", forbidden, response.Body.String())
		}
	}
	if backend.listingReads != 1 || calendar.calls != 1 || backend.quoteWrites != 0 {
		t.Fatalf("calendar error proceeded to quote write: listing=%d calendar=%d quote writes=%d", backend.listingReads, calendar.calls, backend.quoteWrites)
	}
}

func TestSyntheticHTTPQuoteStopsBeforeWriteWhenCalendarCancelsContext(t *testing.T) {
	const ownerID = "44444444-4444-4444-8444-444444444444"
	const renterID = "33333333-3333-4333-8333-333333333333"
	const title = "Título sintético reservado"
	now := time.Date(2030, time.January, 1, 12, 0, 0, 0, time.UTC)
	start := time.Date(2030, time.January, 2, 13, 0, 0, 0, time.UTC)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	backend := &syntheticFlowBackend{
		listing: rentalflow.Listing{
			ID: testSpaceID, OwnerID: ownerID, State: "activa", Title: title,
			RateUnit: "hora", UnitPriceCLP: 12000, Currency: "CLP", RateVersion: 2, TimeZone: "America/Santiago",
		},
		interval: rentalflow.Interval{StartAt: start, EndAt: start.Add(time.Hour)}, available: true,
	}
	calendar := &previewCalendarReturningAvailableWithError{syntheticFlowBackend: backend, cancel: cancel}
	quotes, err := rentalflow.NewQuoteService(backend, calendar, backend, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	handler := New(&authStub{principal: identity.Principal{AccountID: renterID}}, nil, quotes, nil, nil)
	body := `{"space_id":"` + testSpaceID + `","start_at":"` + start.Format(time.RFC3339) + `","end_at":"` + start.Add(time.Hour).Format(time.RFC3339) + `"}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/rental-quotes", strings.NewReader(body)).WithContext(ctx)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer synthetic-session")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable || response.Header().Get("Cache-Control") != "no-store" ||
		strings.Contains(response.Body.String(), `"data"`) {
		t.Fatalf("canceled calendar quote status=%d cache=%q body=%s, want 503/no-store without data", response.Code, response.Header().Get("Cache-Control"), response.Body.String())
	}
	for _, forbidden := range []string{title, ownerID, renterID, testQuoteID, "12000", "rate_version"} {
		if strings.Contains(errorDisclosureBody(t, response.Body.Bytes()), forbidden) {
			t.Fatalf("canceled calendar quote disclosed %q: %s", forbidden, response.Body.String())
		}
	}
	if backend.listingReads != 1 || calendar.calls != 1 || backend.quoteWrites != 0 {
		t.Fatalf("calendar cancellation proceeded to quote write: listing=%d calendar=%d quote writes=%d", backend.listingReads, calendar.calls, backend.quoteWrites)
	}
}

func TestSyntheticHTTPPreviewStopsBeforeIdentityGateWhenCalendarCancelsContext(t *testing.T) {
	const ownerID = "44444444-4444-4444-8444-444444444444"
	const renterID = "33333333-3333-4333-8333-333333333333"
	const title = "Título sintético reservado"
	const ownerName = "Propietario sintético privado"
	const renterName = "Arrendatario sintético privado"
	now := time.Date(2030, time.January, 1, 12, 0, 0, 0, time.UTC)
	start := time.Date(2030, time.January, 2, 13, 0, 0, 0, time.UTC)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	backend := &syntheticFlowBackend{
		listing: rentalflow.Listing{
			ID: testSpaceID, OwnerID: ownerID, State: "activa", Title: title,
			RateUnit: "hora", UnitPriceCLP: 12000, Currency: "CLP", RateVersion: 2, TimeZone: "America/Santiago",
		},
		quote: rentalflow.Quote{
			ID: testQuoteID, SpaceID: testSpaceID, OwnerID: ownerID, RenterID: renterID, SpaceTitle: title,
			RateVersion: 2, RateUnit: "hora", UnitPriceCLP: 12000, Currency: "CLP", Units: 1, SubtotalCLP: 12000,
			StartAt: start, EndAt: start.Add(time.Hour), TimeZone: "America/Santiago",
			CreatedAt: now, ExpiresAt: now.Add(15 * time.Minute),
		},
	}
	calendar := &previewCalendarReturningAvailableWithError{syntheticFlowBackend: backend, cancel: cancel}
	gate := &syntheticIdentityGate{ownerID: ownerID, renterID: renterID}
	identities := &syntheticIdentityReader{quoteID: testQuoteID, parties: map[string]rentalflow.ContractIdentity{
		ownerID:  {SubjectID: ownerID, LegalName: ownerName, RUT: "11.111.111-1"},
		renterID: {SubjectID: renterID, LegalName: renterName, RUT: "22.222.222-2"},
	}}
	previews, err := rentalflow.NewContractPreviewService(backend, calendar, backend, gate, identities, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	handler := New(&authStub{principal: identity.Principal{AccountID: renterID}}, nil, nil, previews, nil)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/rental-quotes/"+testQuoteID+"/contract-preview", nil).WithContext(ctx)
	request.Header.Set("Authorization", "Bearer synthetic-session")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable || response.Header().Get("Cache-Control") != "no-store" ||
		strings.Contains(response.Body.String(), `"data"`) {
		t.Fatalf("canceled calendar preview status=%d cache=%q body=%s, want 503/no-store without data", response.Code, response.Header().Get("Cache-Control"), response.Body.String())
	}
	for _, forbidden := range []string{title, ownerID, renterID, ownerName, renterName, "11111111-1", "22222222-2", "12000"} {
		if strings.Contains(errorDisclosureBody(t, response.Body.Bytes()), forbidden) {
			t.Fatalf("canceled calendar preview disclosed %q: %s", forbidden, response.Body.String())
		}
	}
	if backend.listingReads != 1 || calendar.calls != 1 || gate.calls != 0 || identities.calls != 0 {
		t.Fatalf("calendar cancellation reached identity dependencies: listing=%d calendar=%d gate=%d legal reads=%d", backend.listingReads, calendar.calls, gate.calls, identities.calls)
	}
}
