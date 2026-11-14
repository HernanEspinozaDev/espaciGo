package rentalflowhttp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
	"github.com/HernanEspinozaDev/espaciGo/internal/rentalflow"
)

type malformedQuoteSnapshotRepository struct {
	*syntheticFlowBackend
}

type partialErrorQuoteCreateRepository struct {
	*syntheticFlowBackend
	writeErr error
	cancel   context.CancelFunc
}

func (r *malformedQuoteSnapshotRepository) CreateValidatedQuote(ctx context.Context, renterID string, listing rentalflow.Listing, interval rentalflow.Interval, units, subtotal int64, createdAt time.Time) (rentalflow.Quote, error) {
	quote, err := r.syntheticFlowBackend.CreateValidatedQuote(ctx, renterID, listing, interval, units, subtotal, createdAt)
	if err != nil {
		return rentalflow.Quote{}, err
	}
	quote.SubtotalCLP++
	return quote, nil
}

func (r *partialErrorQuoteCreateRepository) CreateValidatedQuote(ctx context.Context, renterID string, listing rentalflow.Listing, interval rentalflow.Interval, units, subtotal int64, createdAt time.Time) (rentalflow.Quote, error) {
	quote, err := r.syntheticFlowBackend.CreateValidatedQuote(ctx, renterID, listing, interval, units, subtotal, createdAt)
	if err != nil {
		return quote, err
	}
	if r.cancel != nil {
		r.cancel()
	}
	return quote, r.writeErr
}

type malformedPreviewSnapshotRepository struct {
	*syntheticFlowBackend
	calls int
}

func (r *malformedPreviewSnapshotRepository) GetForParticipant(ctx context.Context, actorID, quoteID string) (rentalflow.Quote, error) {
	r.calls++
	quote, err := r.syntheticFlowBackend.GetForParticipant(ctx, actorID, quoteID)
	if err != nil {
		return rentalflow.Quote{}, err
	}
	quote.SubtotalCLP++
	return quote, nil
}

func TestSyntheticHTTPQuoteFailsClosedOnMalformedRepositorySnapshot(t *testing.T) {
	const ownerID = "44444444-4444-4444-8444-444444444444"
	const renterID = "33333333-3333-4333-8333-333333333333"
	const title = "Título sintético reservado"
	now := time.Date(2030, time.January, 1, 12, 0, 0, 0, time.UTC)
	start := time.Date(2030, time.January, 2, 13, 0, 0, 0, time.UTC)
	backend := &syntheticFlowBackend{
		listing:  rentalflow.Listing{ID: testSpaceID, OwnerID: ownerID, State: "activa", Title: title, RateUnit: "hora", UnitPriceCLP: 12000, Currency: "CLP", RateVersion: 2, TimeZone: "America/Santiago"},
		interval: rentalflow.Interval{StartAt: start, EndAt: start.Add(time.Hour)}, available: true,
	}
	quotes, err := rentalflow.NewQuoteService(backend, backend, &malformedQuoteSnapshotRepository{syntheticFlowBackend: backend}, func() time.Time { return now })
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

	if response.Code != http.StatusServiceUnavailable || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("malformed repository snapshot status=%d cache=%q body=%s, want generic 503/no-store", response.Code, response.Header().Get("Cache-Control"), response.Body.String())
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response envelope: %v", err)
	}
	if _, ok := envelope["data"]; ok {
		t.Fatalf("malformed quote snapshot was returned as data: %s", response.Body.String())
	}
	for _, forbidden := range []string{title, ownerID, renterID, testQuoteID, "12000", "rate_version"} {
		if strings.Contains(errorDisclosureBody(t, response.Body.Bytes()), forbidden) {
			t.Fatalf("malformed quote response disclosed %q: %s", forbidden, response.Body.String())
		}
	}
	if backend.quoteWrites != 1 || backend.availabilityChecks != 1 {
		t.Fatalf("unexpected synthetic side effects: quote writes=%d availability checks=%d", backend.quoteWrites, backend.availabilityChecks)
	}
}

func TestSyntheticHTTPPreviewRejectsMalformedRepositorySnapshotBeforeSensitiveReads(t *testing.T) {
	const ownerID = "44444444-4444-4444-8444-444444444444"
	const renterID = "33333333-3333-4333-8333-333333333333"
	const title = "Título sintético reservado"
	const ownerName = "Propietario sintético privado"
	const renterName = "Arrendatario sintético privado"
	now := time.Date(2030, time.January, 1, 12, 0, 0, 0, time.UTC)
	start := time.Date(2030, time.January, 2, 13, 0, 0, 0, time.UTC)
	backend := &syntheticFlowBackend{
		listing:  rentalflow.Listing{ID: testSpaceID, OwnerID: ownerID, State: "activa", Title: title, RateUnit: "hora", UnitPriceCLP: 12000, Currency: "CLP", RateVersion: 2, TimeZone: "America/Santiago"},
		interval: rentalflow.Interval{StartAt: start, EndAt: start.Add(time.Hour)}, available: true,
		quote: rentalflow.Quote{
			ID: testQuoteID, SpaceID: testSpaceID, OwnerID: ownerID, RenterID: renterID, SpaceTitle: title,
			RateVersion: 2, RateUnit: "hora", UnitPriceCLP: 12000, Currency: "CLP", Units: 1, SubtotalCLP: 12000,
			StartAt: start, EndAt: start.Add(time.Hour), TimeZone: "America/Santiago",
			CreatedAt: now, ExpiresAt: now.Add(15 * time.Minute),
		},
	}
	quotes := &malformedPreviewSnapshotRepository{syntheticFlowBackend: backend}
	gate := &syntheticIdentityGate{ownerID: ownerID, renterID: renterID}
	identities := &syntheticIdentityReader{quoteID: testQuoteID, parties: map[string]rentalflow.ContractIdentity{
		ownerID:  {SubjectID: ownerID, LegalName: ownerName, RUT: "11.111.111-1"},
		renterID: {SubjectID: renterID, LegalName: renterName, RUT: "22.222.222-2"},
	}}
	previews, err := rentalflow.NewContractPreviewService(backend, backend, quotes, gate, identities, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	handler := New(&authStub{principal: identity.Principal{AccountID: renterID}}, nil, nil, previews, nil)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/rental-quotes/"+testQuoteID+"/contract-preview", nil)
	request.Header.Set("Authorization", "Bearer synthetic-session")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusConflict || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("malformed quote preview status=%d cache=%q body=%s, want 409/no-store", response.Code, response.Header().Get("Cache-Control"), response.Body.String())
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response envelope: %v", err)
	}
	if _, ok := envelope["data"]; ok {
		t.Fatalf("malformed quote preview was returned as data: %s", response.Body.String())
	}
	for _, forbidden := range []string{title, ownerID, renterID, ownerName, renterName, "11111111-1", "22222222-2", "12000", "12001"} {
		if strings.Contains(errorDisclosureBody(t, response.Body.Bytes()), forbidden) {
			t.Fatalf("malformed quote preview disclosed %q: %s", forbidden, response.Body.String())
		}
	}
	if quotes.calls != 1 || backend.listingReads != 1 || backend.availabilityChecks != 0 || gate.calls != 0 || identities.calls != 0 {
		t.Fatalf("malformed quote reached sensitive dependencies: quote reads=%d listing reads=%d availability=%d gate=%d identity reads=%d", quotes.calls, backend.listingReads, backend.availabilityChecks, gate.calls, identities.calls)
	}
}

func TestSyntheticHTTPQuoteMapsRepositoryOutcomesWithoutReturningPartialSnapshot(t *testing.T) {
	const ownerID = "44444444-4444-4444-8444-444444444444"
	const renterID = "33333333-3333-4333-8333-333333333333"
	const title = "Título sintético reservado"
	cases := []struct {
		name       string
		writeErr   error
		cancel     bool
		wantStatus int
	}{
		{name: "operational failure", writeErr: errors.New("synthetic private quote repository failure"), wantStatus: http.StatusServiceUnavailable},
		{name: "atomic conflict", writeErr: rentalflow.ErrConflict, wantStatus: http.StatusConflict},
		{name: "cancellation concurrent with conflict", writeErr: rentalflow.ErrConflict, cancel: true, wantStatus: http.StatusServiceUnavailable},
		{name: "listing disappeared", writeErr: rentalflow.ErrNotFound, wantStatus: http.StatusNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			now := time.Date(2030, time.January, 1, 12, 0, 0, 0, time.UTC)
			start := time.Date(2030, time.January, 2, 13, 0, 0, 0, time.UTC)
			backend := &syntheticFlowBackend{
				listing: rentalflow.Listing{
					ID: testSpaceID, OwnerID: ownerID, State: "activa", Title: title,
					RateUnit: "hora", UnitPriceCLP: 12000, Currency: "CLP", RateVersion: 2, TimeZone: "America/Santiago",
				},
				interval: rentalflow.Interval{StartAt: start, EndAt: start.Add(time.Hour)}, available: true,
			}
			repository := &partialErrorQuoteCreateRepository{syntheticFlowBackend: backend, writeErr: tc.writeErr}
			if tc.cancel {
				repository.cancel = cancel
			}
			quotes, err := rentalflow.NewQuoteService(backend, backend, repository, func() time.Time { return now })
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

			if response.Code != tc.wantStatus || response.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("partial quote repository error status=%d cache=%q body=%s, want %d/no-store", response.Code, response.Header().Get("Cache-Control"), response.Body.String(), tc.wantStatus)
			}
			var envelope map[string]json.RawMessage
			if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
				t.Fatalf("decode response envelope: %v", err)
			}
			if _, ok := envelope["data"]; ok {
				t.Fatalf("quote snapshot returned with error was exposed: %s", response.Body.String())
			}
			for _, forbidden := range []string{title, ownerID, renterID, testQuoteID, "12000", tc.writeErr.Error(), "rate_version"} {
				if strings.Contains(errorDisclosureBody(t, response.Body.Bytes()), forbidden) {
					t.Fatalf("quote error response disclosed %q: %s", forbidden, response.Body.String())
				}
			}
			if backend.listingReads != 1 || backend.availabilityChecks != 1 || backend.quoteWrites != 1 {
				t.Fatalf("unexpected synthetic side effects: listing=%d availability=%d quote writes=%d", backend.listingReads, backend.availabilityChecks, backend.quoteWrites)
			}
		})
	}
}
