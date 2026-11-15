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
	"github.com/HernanEspinozaDev/espaciGo/internal/identityverification"
	"github.com/HernanEspinozaDev/espaciGo/internal/rentalflow"
)

type uuidCaseIdentityReader struct {
	requesterID string
	quoteID     string
	parties     map[string]rentalflow.ContractIdentity
	calls       int
}

func (r *uuidCaseIdentityReader) CurrentForRentalContract(_ context.Context, requesterID, quoteID, subjectID string, _ time.Time) (rentalflow.ContractIdentity, error) {
	r.calls++
	if requesterID != r.requesterID || quoteID != r.quoteID {
		return rentalflow.ContractIdentity{}, rentalflow.ErrIdentityUnavailable
	}
	identity, ok := r.parties[subjectID]
	if !ok {
		return rentalflow.ContractIdentity{}, rentalflow.ErrIdentityUnavailable
	}
	return identity, nil
}

func TestSyntheticHTTPQuoteAndPreviewCanonicalizeUppercaseSpaceAndActorUUIDs(t *testing.T) {
	const ownerID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	const renterID = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	const spaceID = "cccccccc-cccc-4ccc-8ccc-cccccccccccc"
	const quoteID = "dddddddd-dddd-4ddd-8ddd-dddddddddddd"
	now := time.Date(2030, time.January, 1, 12, 0, 0, 0, time.UTC)
	start := time.Date(2030, time.January, 2, 13, 0, 0, 0, time.UTC)
	backend := &syntheticFlowBackend{
		listing: rentalflow.Listing{
			ID: spaceID, OwnerID: ownerID, State: "activa", Title: "Sala sintética",
			RateUnit: "hora", UnitPriceCLP: 12000, Currency: "CLP", RateVersion: 2, TimeZone: "America/Santiago",
		},
		interval: rentalflow.Interval{StartAt: start, EndAt: start.Add(time.Hour)}, available: true, quoteID: quoteID,
	}
	gate := &syntheticIdentityGate{ownerID: ownerID, renterID: renterID}
	identities := &uuidCaseIdentityReader{requesterID: renterID, quoteID: quoteID, parties: map[string]rentalflow.ContractIdentity{
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
	handler := New(&authStub{principal: identity.Principal{AccountID: strings.ToUpper(renterID)}}, availability, quotes, previews, nil)

	quoteBody := `{"space_id":"` + strings.ToUpper(spaceID) + `","start_at":"` + start.Format(time.RFC3339) + `","end_at":"` + start.Add(time.Hour).Format(time.RFC3339) + `"}`
	quoteRequest := httptest.NewRequest(http.MethodPost, "/api/v1/rental-quotes", strings.NewReader(quoteBody))
	quoteRequest.Header.Set("Content-Type", "application/json")
	quoteRequest.Header.Set("Authorization", "Bearer synthetic-session")
	quoteResponse := httptest.NewRecorder()
	handler.ServeHTTP(quoteResponse, quoteRequest)
	if quoteResponse.Code != http.StatusCreated {
		t.Fatalf("uppercase UUID quote status=%d body=%s", quoteResponse.Code, quoteResponse.Body.String())
	}
	var quoteEnvelope struct {
		Data rentalflow.Quote `json:"data"`
	}
	if err := json.Unmarshal(quoteResponse.Body.Bytes(), &quoteEnvelope); err != nil {
		t.Fatal(err)
	}
	if quoteEnvelope.Data.ID != quoteID || quoteEnvelope.Data.SpaceID != spaceID || backend.quote.OwnerID != ownerID || backend.quote.RenterID != renterID || backend.quoteWrites != 1 {
		t.Fatalf("quote IDs were not canonicalized: response=%+v stored snapshot=%+v writes=%d", quoteEnvelope.Data, backend.quote, backend.quoteWrites)
	}
	if strings.Contains(quoteResponse.Body.String(), ownerID) || strings.Contains(quoteResponse.Body.String(), renterID) {
		t.Fatalf("quote response exposed participant IDs: %s", quoteResponse.Body.String())
	}

	previewRequest := httptest.NewRequest(http.MethodPost, "/api/v1/rental-quotes/"+strings.ToUpper(quoteID)+"/contract-preview", nil)
	previewRequest.Header.Set("Authorization", "Bearer synthetic-session")
	previewResponse := httptest.NewRecorder()
	handler.ServeHTTP(previewResponse, previewRequest)
	if previewResponse.Code != http.StatusOK || previewResponse.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("uppercase UUID preview status=%d cache=%q body=%s", previewResponse.Code, previewResponse.Header().Get("Cache-Control"), previewResponse.Body.String())
	}
	var previewEnvelope struct {
		Data rentalflow.ContractPreview `json:"data"`
	}
	if err := json.Unmarshal(previewResponse.Body.Bytes(), &previewEnvelope); err != nil {
		t.Fatal(err)
	}
	if previewEnvelope.Data.QuoteID != quoteID || previewEnvelope.Data.SpaceID != spaceID || gate.calls != 1 || identities.calls != 2 {
		t.Fatalf("preview IDs/gate were not canonicalized: preview=%+v gate=%d identity reads=%d", previewEnvelope.Data, gate.calls, identities.calls)
	}
}

func TestSyntheticHTTPPreviewCanonicalizesUppercaseQuoteAndActorUUIDsBeforeDocumentaryGate(t *testing.T) {
	const ownerID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	const renterID = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	const spaceID = "cccccccc-cccc-4ccc-8ccc-cccccccccccc"
	const quoteID = "dddddddd-dddd-4ddd-8ddd-dddddddddddd"
	now := time.Date(2030, time.January, 1, 12, 0, 0, 0, time.UTC)
	start := time.Date(2030, time.January, 2, 13, 0, 0, 0, time.UTC)
	listing := rentalflow.Listing{
		ID: spaceID, OwnerID: ownerID, State: "activa", Title: "Sala sintética",
		RateUnit: "hora", UnitPriceCLP: 12000, Currency: "CLP", RateVersion: 2, TimeZone: "America/Santiago",
	}
	backend := &syntheticFlowBackend{
		listing: listing, interval: rentalflow.Interval{StartAt: start, EndAt: start.Add(time.Hour)}, available: true,
		quote: rentalflow.Quote{
			ID: quoteID, SpaceID: spaceID, OwnerID: ownerID, RenterID: renterID, SpaceTitle: listing.Title,
			RateVersion: listing.RateVersion, RateUnit: listing.RateUnit, UnitPriceCLP: listing.UnitPriceCLP,
			Currency: listing.Currency, Units: 1, SubtotalCLP: 12000, StartAt: start, EndAt: start.Add(time.Hour),
			TimeZone: listing.TimeZone, CreatedAt: now, ExpiresAt: now.Add(15 * time.Minute),
		},
	}
	verifiedAt := now.Add(-time.Hour)
	verificationFor := func(subjectID string) identityverification.Verification {
		return identityverification.Verification{
			SubjectID: subjectID, Kind: identityverification.KindIdentityDocument,
			State: identityverification.StateApproved, ReviewMethod: identityverification.ReviewMethodManualDocuments,
			VerifiedAt: verifiedAt, ExpiresAt: verifiedAt.AddDate(0, 12, 0),
		}
	}
	verificationRepo := &syntheticVerificationRepository{current: map[string]identityverification.Verification{
		ownerID: verificationFor(ownerID), renterID: verificationFor(renterID),
	}}
	gate, err := identityverification.NewService(verificationRepo)
	if err != nil {
		t.Fatal(err)
	}
	identities := &uuidCaseIdentityReader{requesterID: renterID, quoteID: quoteID, parties: map[string]rentalflow.ContractIdentity{
		ownerID:  {SubjectID: ownerID, LegalName: "Arrendador sintético", RUT: "11.111.111-1"},
		renterID: {SubjectID: renterID, LegalName: "Arrendataria sintética", RUT: "22.222.222-2"},
	}}
	previews, err := rentalflow.NewContractPreviewService(backend, backend, backend, gate, identities, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	handler := New(&authStub{principal: identity.Principal{AccountID: strings.ToUpper(renterID)}}, nil, nil, previews, nil)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/rental-quotes/"+strings.ToUpper(quoteID)+"/contract-preview", nil)
	request.Header.Set("Authorization", "Bearer synthetic-session")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("uppercase quote/actor UUID preview status=%d cache=%q body=%s", response.Code, response.Header().Get("Cache-Control"), response.Body.String())
	}
	var envelope struct {
		Data rentalflow.ContractPreview `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data.QuoteID != quoteID || envelope.Data.SpaceID != spaceID || len(verificationRepo.calls) != 2 ||
		verificationRepo.calls[0] != ownerID || verificationRepo.calls[1] != renterID || identities.calls != 2 {
		t.Fatalf("uppercase quote/actor UUIDs did not reach the documentary gate canonically: data=%+v verification reads=%v legal reads=%d", envelope.Data, verificationRepo.calls, identities.calls)
	}
}
