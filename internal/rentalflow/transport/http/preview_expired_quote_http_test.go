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

type countedPreviewQuoteRepository struct {
	*syntheticFlowBackend
	reads int
}

func (r *countedPreviewQuoteRepository) GetForParticipant(ctx context.Context, actorID, quoteID string) (rentalflow.Quote, error) {
	r.reads++
	return r.syntheticFlowBackend.GetForParticipant(ctx, actorID, quoteID)
}

func TestSyntheticHTTPPreviewRejectsQuoteExpiredAtCurrentInstant(t *testing.T) {
	const ownerID = "44444444-4444-4444-8444-444444444444"
	const renterID = "33333333-3333-4333-8333-333333333333"
	const title = "Título sintético reservado"
	const ownerName = "Propietario sintético privado"
	const renterName = "Arrendatario sintético privado"
	now := time.Date(2030, time.May, 10, 12, 0, 0, 0, time.UTC)
	start := time.Date(2030, time.May, 11, 15, 0, 0, 0, time.UTC)
	listing := rentalflow.Listing{
		ID: testSpaceID, OwnerID: ownerID, State: "activa", Title: title,
		RateUnit: "hora", UnitPriceCLP: 8000, Currency: "CLP", RateVersion: 7, TimeZone: "America/Santiago",
	}
	backend := &syntheticFlowBackend{
		listing: listing, interval: rentalflow.Interval{StartAt: start, EndAt: start.Add(time.Hour)}, available: true,
		quote: rentalflow.Quote{
			ID: testQuoteID, SpaceID: testSpaceID, OwnerID: ownerID, RenterID: renterID, SpaceTitle: title,
			RateVersion: listing.RateVersion, RateUnit: listing.RateUnit, UnitPriceCLP: listing.UnitPriceCLP,
			Currency: listing.Currency, Units: 1, SubtotalCLP: listing.UnitPriceCLP,
			StartAt: start, EndAt: start.Add(time.Hour), TimeZone: listing.TimeZone,
			CreatedAt: now.Add(-time.Minute), ExpiresAt: now,
		},
	}
	quotes := &countedPreviewQuoteRepository{syntheticFlowBackend: backend}
	gate := &syntheticIdentityGate{ownerID: ownerID, renterID: renterID}
	identities := &syntheticIdentityReader{
		quoteID: testQuoteID,
		parties: map[string]rentalflow.ContractIdentity{
			ownerID:  {SubjectID: ownerID, LegalName: ownerName, RUT: "11.111.111-1"},
			renterID: {SubjectID: renterID, LegalName: renterName, RUT: "22.222.222-2"},
		},
	}
	previews, err := rentalflow.NewContractPreviewService(backend, backend, quotes, gate, identities, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	handler := New(&authStub{principal: identity.Principal{AccountID: renterID}}, nil, nil, previews, nil)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/rental-quotes/"+testQuoteID+"/contract-preview", nil)
	request.Header.Set("Authorization", "Bearer synthetic-session")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode expired preview response: %v; body=%s", err, response.Body.String())
	}
	var errorDetails struct {
		Code      string `json:"code"`
		Message   string `json:"message"`
		RequestID string `json:"request_id"`
	}
	if err := json.Unmarshal(envelope["error"], &errorDetails); err != nil {
		t.Fatalf("decode expired preview error: %v; envelope=%v", err, envelope)
	}
	if response.Code != http.StatusConflict || response.Header().Get("Cache-Control") != "no-store" ||
		len(envelope) != 1 || envelope["error"] == nil || errorDetails.Code != "conflict" || errorDetails.Message == "" || errorDetails.RequestID == "" {
		t.Fatalf("expired quote response status=%d cache=%q error=%+v envelope=%v body=%s; want 409/no-store generic conflict error only", response.Code, response.Header().Get("Cache-Control"), errorDetails, envelope, response.Body.String())
	}
	for _, forbidden := range []string{title, ownerID, renterID, testQuoteID, ownerName, renterName, "11.111.111-1", "22.222.222-2", "8000"} {
		if strings.Contains(errorDisclosureBody(t, response.Body.Bytes()), forbidden) {
			t.Fatalf("expired quote response disclosed %q: %s", forbidden, response.Body.String())
		}
	}
	if quotes.reads != 1 || backend.listingReads != 0 || backend.availabilityChecks != 0 || gate.calls != 0 || identities.calls != 0 {
		t.Fatalf("expired quote continued into private dependencies: quote reads=%d listing=%d calendar=%d gate=%d identity=%d", quotes.reads, backend.listingReads, backend.availabilityChecks, gate.calls, identities.calls)
	}
}
