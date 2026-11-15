package rentalflowhttp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
	"github.com/HernanEspinozaDev/espaciGo/internal/rentalflow"
)

func TestSyntheticHTTPAvailabilityMapsCanceledMissingListingAsUnavailable(t *testing.T) {
	const ownerID = "44444444-4444-4444-8444-444444444444"
	now := time.Date(2030, time.January, 1, 12, 0, 0, 0, time.UTC)
	ctx, cancel := context.WithCancel(context.Background())
	backend := &syntheticFlowBackend{
		listing:    rentalflow.Listing{ID: testSpaceID, OwnerID: ownerID, State: "activa", Title: "Sala sintética", RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", RateVersion: 1, TimeZone: "America/Santiago"},
		listingErr: rentalflow.ErrNotFound, onListingRead: cancel,
	}
	availability, err := rentalflow.NewAvailabilityService(backend, backend, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	handler := New(nil, availability, nil, nil, nil)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/spaces/"+testSpaceID+"/rental-availability?date=2030-01-02&duration=1", nil).WithContext(ctx)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable || !strings.Contains(response.Body.String(), "service_unavailable") ||
		response.Header().Get("Cache-Control") != "no-store" || backend.listingReads != 1 || backend.availabilityReads != 0 {
		t.Fatalf("canceled missing-listing response status=%d listingReads=%d calendarReads=%d body=%s", response.Code, backend.listingReads, backend.availabilityReads, response.Body.String())
	}
}

func TestSyntheticHTTPAvailabilityRejectsUnsafeListingBeforeCalendarRead(t *testing.T) {
	const ownerID = "44444444-4444-4444-8444-444444444444"
	now := time.Date(2030, time.January, 1, 12, 0, 0, 0, time.UTC)
	start := time.Date(2030, time.January, 2, 13, 0, 0, 0, time.UTC)
	unsafeTitle := "Sala\u202e privada"
	backend := &syntheticFlowBackend{
		listing:  rentalflow.Listing{ID: testSpaceID, OwnerID: ownerID, State: "activa", Title: unsafeTitle, RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", RateVersion: 1, TimeZone: "America/Santiago"},
		interval: rentalflow.Interval{StartAt: start, EndAt: start.Add(time.Hour)}, available: true,
	}
	availability, err := rentalflow.NewAvailabilityService(backend, backend, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	handler := New(nil, availability, nil, nil, nil)
	request := httptest.NewRequest(http.MethodGet,
		"/api/v1/spaces/"+testSpaceID+"/rental-availability?date=2030-01-02&duration=1", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	responseBody := response.Body.String()
	if response.Code != http.StatusServiceUnavailable || response.Header().Get("Cache-Control") != "no-store" ||
		backend.listingReads != 1 || backend.availabilityReads != 0 || strings.Contains(responseBody, unsafeTitle) || strings.Contains(responseBody, `"data"`) {
		t.Fatalf("unsafe listing response status=%d listingReads=%d calendarReads=%d body=%s", response.Code, backend.listingReads, backend.availabilityReads, responseBody)
	}
}

func TestSyntheticHTTPAvailabilityFailsClosedOnCurrentOrPastIntervals(t *testing.T) {
	const ownerID = "44444444-4444-4444-8444-444444444444"
	now := time.Date(2030, time.January, 2, 13, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name  string
		start time.Time
	}{
		{name: "starts exactly at as-of", start: now},
		{name: "started one hour before as-of", start: now.Add(-time.Hour)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			backend := &syntheticFlowBackend{
				listing:  rentalflow.Listing{ID: testSpaceID, OwnerID: ownerID, State: "activa", Title: "Sala sintética", RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", RateVersion: 1, TimeZone: "America/Santiago"},
				interval: rentalflow.Interval{StartAt: tc.start, EndAt: tc.start.Add(time.Hour)}, available: true,
			}
			availability, err := rentalflow.NewAvailabilityService(backend, backend, func() time.Time { return now })
			if err != nil {
				t.Fatal(err)
			}
			handler := New(nil, availability, nil, nil, nil)
			request := httptest.NewRequest(http.MethodGet,
				"/api/v1/spaces/"+testSpaceID+"/rental-availability?date=2030-01-02&duration=1", nil)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusServiceUnavailable || response.Header().Get("Cache-Control") != "no-store" ||
				backend.availabilityReads != 1 || strings.Contains(response.Body.String(), `"data"`) ||
				strings.Contains(response.Body.String(), tc.start.Format(time.RFC3339)) {
				t.Fatalf("stale interval response status=%d calendarReads=%d body=%s", response.Code, backend.availabilityReads, response.Body.String())
			}
		})
	}
}

func TestSyntheticHTTPAvailabilityAcceptsIntervalOneNanosecondAfterAsOf(t *testing.T) {
	const ownerID = "44444444-4444-4444-8444-444444444444"
	start := time.Date(2030, time.January, 2, 13, 0, 0, 0, time.UTC)
	asOf := start.Add(-time.Nanosecond)
	backend := &syntheticFlowBackend{
		listing:  rentalflow.Listing{ID: testSpaceID, OwnerID: ownerID, State: "activa", Title: "Sala sintética", RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", RateVersion: 1, TimeZone: "America/Santiago"},
		interval: rentalflow.Interval{StartAt: start, EndAt: start.Add(time.Hour)}, available: true,
	}
	availability, err := rentalflow.NewAvailabilityService(backend, backend, func() time.Time { return asOf })
	if err != nil {
		t.Fatal(err)
	}
	handler := New(nil, availability, nil, nil, nil)
	request := httptest.NewRequest(http.MethodGet,
		"/api/v1/spaces/"+testSpaceID+"/rental-availability?date=2030-01-02&duration=1", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	var envelope struct {
		Data rentalflow.Availability `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "no-store" ||
		!envelope.Data.CheckedAt.Equal(asOf) || len(envelope.Data.Items) != 1 || !envelope.Data.Items[0].StartAt.Equal(start) {
		t.Fatalf("future interval at one-nanosecond boundary status=%d availability=%+v body=%s", response.Code, envelope.Data, response.Body.String())
	}
}

func TestSyntheticHTTPQuoteRevalidatesAvailabilityAfterPublicRead(t *testing.T) {
	const ownerID = "44444444-4444-4444-8444-444444444444"
	const renterID = "33333333-3333-4333-8333-333333333333"
	now := time.Date(2030, time.January, 1, 12, 0, 0, 0, time.UTC)
	start := time.Date(2030, time.January, 2, 13, 0, 0, 0, time.UTC)
	backend := &syntheticFlowBackend{
		listing:  rentalflow.Listing{ID: testSpaceID, OwnerID: ownerID, State: "activa", Title: "Sala sintética", RateUnit: "hora", UnitPriceCLP: 12000, Currency: "CLP", RateVersion: 2, TimeZone: "America/Santiago"},
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
	previews, err := rentalflow.NewContractPreviewService(backend, backend, backend, &syntheticIdentityGate{ownerID: ownerID, renterID: renterID},
		&syntheticIdentityReader{quoteID: testQuoteID, parties: map[string]rentalflow.ContractIdentity{}}, clock)
	if err != nil {
		t.Fatal(err)
	}
	handler := New(&authStub{principal: identity.Principal{AccountID: renterID}}, availability, quotes, previews, nil)

	publicResponse := httptest.NewRecorder()
	handler.ServeHTTP(publicResponse, httptest.NewRequest(http.MethodGet,
		"/api/v1/spaces/"+testSpaceID+"/rental-availability?date=2030-01-02&duration=1", nil))
	if publicResponse.Code != http.StatusOK || backend.availabilityReads != 1 {
		t.Fatalf("public availability status=%d reads=%d body=%s", publicResponse.Code, backend.availabilityReads, publicResponse.Body.String())
	}

	// A different actor consumes the interval between the informational read and
	// the authenticated quote; the request must fail without persisting a quote.
	backend.available = false
	body := `{"space_id":"` + testSpaceID + `","start_at":"` + start.Format(time.RFC3339) + `","end_at":"` + start.Add(time.Hour).Format(time.RFC3339) + `"}`
	quoteRequest := httptest.NewRequest(http.MethodPost, "/api/v1/rental-quotes", strings.NewReader(body))
	quoteRequest.Header.Set("Content-Type", "application/json")
	quoteRequest.Header.Set("Authorization", "Bearer synthetic-session")
	quoteResponse := httptest.NewRecorder()
	handler.ServeHTTP(quoteResponse, quoteRequest)
	if quoteResponse.Code != http.StatusConflict || backend.quoteWrites != 0 || backend.availabilityChecks != 1 {
		t.Fatalf("stale selection status=%d writes=%d checks=%d body=%s", quoteResponse.Code, backend.quoteWrites, backend.availabilityChecks, quoteResponse.Body.String())
	}
}

func TestSyntheticHTTPQuoteRejectsTariffChangeBeforeAtomicCreate(t *testing.T) {
	const ownerID = "44444444-4444-4444-8444-444444444444"
	const renterID = "33333333-3333-4333-8333-333333333333"
	now := time.Date(2030, time.January, 1, 12, 0, 0, 0, time.UTC)
	start := time.Date(2030, time.January, 2, 13, 0, 0, 0, time.UTC)
	backend := &syntheticFlowBackend{
		listing:  rentalflow.Listing{ID: testSpaceID, OwnerID: ownerID, State: "activa", Title: "Sala sintética", RateUnit: "hora", UnitPriceCLP: 12000, Currency: "CLP", RateVersion: 2, TimeZone: "America/Santiago"},
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

	publicResponse := httptest.NewRecorder()
	handler.ServeHTTP(publicResponse, httptest.NewRequest(http.MethodGet,
		"/api/v1/spaces/"+testSpaceID+"/rental-availability?date=2030-01-02&duration=1", nil))
	if publicResponse.Code != http.StatusOK || backend.availabilityReads != 1 {
		t.Fatalf("public availability status=%d reads=%d body=%s", publicResponse.Code, backend.availabilityReads, publicResponse.Body.String())
	}

	// A newer tariff wins before the repository's atomic create; the stale quote
	// must not be persisted or returned.
	backend.onQuoteCreate = func() {
		backend.listing.UnitPriceCLP = 15000
		backend.listing.RateVersion++
	}
	body := `{"space_id":"` + testSpaceID + `","start_at":"` + start.Format(time.RFC3339) + `","end_at":"` + start.Add(time.Hour).Format(time.RFC3339) + `"}`
	quoteRequest := httptest.NewRequest(http.MethodPost, "/api/v1/rental-quotes", strings.NewReader(body))
	quoteRequest.Header.Set("Content-Type", "application/json")
	quoteRequest.Header.Set("Authorization", "Bearer synthetic-session")
	quoteResponse := httptest.NewRecorder()
	handler.ServeHTTP(quoteResponse, quoteRequest)
	responseBody := quoteResponse.Body.String()
	if quoteResponse.Code != http.StatusConflict || quoteResponse.Header().Get("Cache-Control") != "no-store" ||
		backend.quoteWrites != 1 || backend.availabilityChecks != 1 || backend.quote.ID != "" ||
		strings.Contains(errorDisclosureBody(t, []byte(responseBody)), testQuoteID) || strings.Contains(errorDisclosureBody(t, []byte(responseBody)), "12000") {
		t.Fatalf("stale-tariff response status=%d writes=%d checks=%d persisted=%+v body=%s", quoteResponse.Code, backend.quoteWrites, backend.availabilityChecks, backend.quote, responseBody)
	}
}

func TestSyntheticHTTPQuoteRejectsPublicationChangeBeforeAtomicCreate(t *testing.T) {
	const ownerID = "44444444-4444-4444-8444-444444444444"
	const renterID = "33333333-3333-4333-8333-333333333333"
	now := time.Date(2030, time.January, 1, 12, 0, 0, 0, time.UTC)
	start := time.Date(2030, time.January, 2, 13, 0, 0, 0, time.UTC)
	backend := &syntheticFlowBackend{
		listing:  rentalflow.Listing{ID: testSpaceID, OwnerID: ownerID, State: "activa", Title: "Sala sintética", RateUnit: "hora", UnitPriceCLP: 12000, Currency: "CLP", RateVersion: 2, TimeZone: "America/Santiago"},
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

	publicResponse := httptest.NewRecorder()
	handler.ServeHTTP(publicResponse, httptest.NewRequest(http.MethodGet,
		"/api/v1/spaces/"+testSpaceID+"/rental-availability?date=2030-01-02&duration=1", nil))
	if publicResponse.Code != http.StatusOK || backend.availabilityReads != 1 {
		t.Fatalf("public availability status=%d reads=%d body=%s", publicResponse.Code, backend.availabilityReads, publicResponse.Body.String())
	}

	// Publication is withdrawn after the reads but before the atomic quote write.
	backend.onQuoteCreate = func() { backend.listing.State = "pausada" }
	body := `{"space_id":"` + testSpaceID + `","start_at":"` + start.Format(time.RFC3339) + `","end_at":"` + start.Add(time.Hour).Format(time.RFC3339) + `"}`
	quoteRequest := httptest.NewRequest(http.MethodPost, "/api/v1/rental-quotes", strings.NewReader(body))
	quoteRequest.Header.Set("Content-Type", "application/json")
	quoteRequest.Header.Set("Authorization", "Bearer synthetic-session")
	quoteResponse := httptest.NewRecorder()
	handler.ServeHTTP(quoteResponse, quoteRequest)
	responseBody := quoteResponse.Body.String()
	if quoteResponse.Code != http.StatusConflict || quoteResponse.Header().Get("Cache-Control") != "no-store" ||
		backend.quoteWrites != 1 || backend.availabilityChecks != 1 || backend.quote.ID != "" ||
		strings.Contains(errorDisclosureBody(t, []byte(responseBody)), testQuoteID) || strings.Contains(errorDisclosureBody(t, []byte(responseBody)), "12000") {
		t.Fatalf("unpublished-space response status=%d writes=%d checks=%d persisted=%+v body=%s", quoteResponse.Code, backend.quoteWrites, backend.availabilityChecks, backend.quote, responseBody)
	}
}

func TestSyntheticHTTPQuoteRejectsTimeZoneChangeBeforeAtomicCreate(t *testing.T) {
	const ownerID = "44444444-4444-4444-8444-444444444444"
	const renterID = "33333333-3333-4333-8333-333333333333"
	now := time.Date(2030, time.January, 1, 12, 0, 0, 0, time.UTC)
	start := time.Date(2030, time.January, 2, 13, 0, 0, 0, time.UTC)
	backend := &syntheticFlowBackend{
		listing:  rentalflow.Listing{ID: testSpaceID, OwnerID: ownerID, State: "activa", Title: "Sala sintética", RateUnit: "hora", UnitPriceCLP: 12000, Currency: "CLP", RateVersion: 2, TimeZone: "America/Santiago"},
		interval: rentalflow.Interval{StartAt: start, EndAt: start.Add(time.Hour)}, available: true,
	}
	quotes, err := rentalflow.NewQuoteService(backend, backend, backend, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	handler := New(&authStub{principal: identity.Principal{AccountID: renterID}}, nil, quotes, nil, nil)
	backend.onQuoteCreate = func() { backend.listing.TimeZone = "Pacific/Kiritimati" }
	body := `{"space_id":"` + testSpaceID + `","start_at":"` + start.Format(time.RFC3339) + `","end_at":"` + start.Add(time.Hour).Format(time.RFC3339) + `"}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/rental-quotes", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer synthetic-session")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	responseBody := response.Body.String()
	if response.Code != http.StatusConflict || response.Header().Get("Cache-Control") != "no-store" ||
		backend.quoteWrites != 1 || backend.availabilityChecks != 1 || backend.quote.ID != "" ||
		strings.Contains(errorDisclosureBody(t, []byte(responseBody)), testQuoteID) || strings.Contains(errorDisclosureBody(t, []byte(responseBody)), "12000") {
		t.Fatalf("stale-time-zone response status=%d writes=%d checks=%d persisted=%+v body=%s", response.Code, backend.quoteWrites, backend.availabilityChecks, backend.quote, responseBody)
	}
}

func TestSyntheticHTTPQuoteRejectsOwnerChangeBeforeAtomicCreate(t *testing.T) {
	const ownerID = "44444444-4444-4444-8444-444444444444"
	const replacementOwnerID = "55555555-5555-4555-8555-555555555555"
	const renterID = "33333333-3333-4333-8333-333333333333"
	now := time.Date(2030, time.January, 1, 12, 0, 0, 0, time.UTC)
	start := time.Date(2030, time.January, 2, 13, 0, 0, 0, time.UTC)
	backend := &syntheticFlowBackend{
		listing:  rentalflow.Listing{ID: testSpaceID, OwnerID: ownerID, State: "activa", Title: "Sala sintética", RateUnit: "hora", UnitPriceCLP: 12000, Currency: "CLP", RateVersion: 2, TimeZone: "America/Santiago"},
		interval: rentalflow.Interval{StartAt: start, EndAt: start.Add(time.Hour)}, available: true,
	}
	quotes, err := rentalflow.NewQuoteService(backend, backend, backend, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	handler := New(&authStub{principal: identity.Principal{AccountID: renterID}}, nil, quotes, nil, nil)
	backend.onQuoteCreate = func() { backend.listing.OwnerID = replacementOwnerID }
	body := `{"space_id":"` + testSpaceID + `","start_at":"` + start.Format(time.RFC3339) + `","end_at":"` + start.Add(time.Hour).Format(time.RFC3339) + `"}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/rental-quotes", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer synthetic-session")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	responseBody := response.Body.String()
	if response.Code != http.StatusConflict || response.Header().Get("Cache-Control") != "no-store" ||
		backend.quoteWrites != 1 || backend.availabilityChecks != 1 || backend.quote.ID != "" ||
		strings.Contains(errorDisclosureBody(t, []byte(responseBody)), testQuoteID) || strings.Contains(errorDisclosureBody(t, []byte(responseBody)), ownerID) {
		t.Fatalf("stale-owner response status=%d writes=%d checks=%d persisted=%+v body=%s", response.Code, backend.quoteWrites, backend.availabilityChecks, backend.quote, responseBody)
	}
}

func TestSyntheticHTTPQuoteCancellationDuringConflictIsUnavailable(t *testing.T) {
	const ownerID = "44444444-4444-4444-8444-444444444444"
	const renterID = "33333333-3333-4333-8333-333333333333"
	now := time.Date(2030, time.January, 1, 12, 0, 0, 0, time.UTC)
	start := time.Date(2030, time.January, 2, 13, 0, 0, 0, time.UTC)
	ctx, cancel := context.WithCancel(context.Background())
	backend := &syntheticFlowBackend{
		listing:  rentalflow.Listing{ID: testSpaceID, OwnerID: ownerID, State: "activa", Title: "Sala sintética", RateUnit: "hora", UnitPriceCLP: 12000, Currency: "CLP", RateVersion: 2, TimeZone: "America/Santiago"},
		interval: rentalflow.Interval{StartAt: start, EndAt: start.Add(time.Hour)}, available: true,
		quoteErr: rentalflow.ErrConflict, onQuoteCreate: cancel,
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
	body := `{"space_id":"` + testSpaceID + `","start_at":"` + start.Format(time.RFC3339) + `","end_at":"` + start.Add(time.Hour).Format(time.RFC3339) + `"}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/rental-quotes", strings.NewReader(body)).WithContext(ctx)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer synthetic-session")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	responseBody := response.Body.String()
	if response.Code != http.StatusServiceUnavailable || !strings.Contains(responseBody, "service_unavailable") ||
		response.Header().Get("Cache-Control") != "no-store" || backend.quoteWrites != 1 || backend.availabilityChecks != 1 ||
		strings.Contains(errorDisclosureBody(t, []byte(responseBody)), "quote_id") || strings.Contains(errorDisclosureBody(t, []byte(responseBody)), "space_title") || strings.Contains(errorDisclosureBody(t, []byte(responseBody)), "12000") {
		t.Fatalf("canceled quote response status=%d writes=%d checks=%d body=%s", response.Code, backend.quoteWrites, backend.availabilityChecks, responseBody)
	}
}
