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
	"github.com/HernanEspinozaDev/espaciGo/internal/identityverification"
	"github.com/HernanEspinozaDev/espaciGo/internal/rentalflow"
)

type syntheticVerificationRepository struct {
	current      map[string]identityverification.Verification
	err          error
	errByAccount map[string]error
	calls        []string
}

func (r *syntheticVerificationRepository) CurrentForAccount(_ context.Context, accountID string) (identityverification.Verification, error) {
	r.calls = append(r.calls, accountID)
	if err := r.errByAccount[accountID]; err != nil {
		return identityverification.Verification{}, err
	}
	if r.err != nil {
		return identityverification.Verification{}, r.err
	}
	verification, ok := r.current[accountID]
	if !ok {
		return identityverification.Verification{}, identityverification.ErrNotFound
	}
	return verification, nil
}

func TestSyntheticHTTPContractPreviewUsesCurrentDocumentaryIdentityGate(t *testing.T) {
	const ownerID = "44444444-4444-4444-8444-444444444444"
	const renterID = "33333333-3333-4333-8333-333333333333"
	now := time.Date(2030, time.January, 1, 12, 0, 0, 0, time.UTC)
	start := time.Date(2030, time.January, 2, 13, 0, 0, 0, time.UTC)
	backend := &syntheticFlowBackend{
		listing:  rentalflow.Listing{ID: testSpaceID, OwnerID: ownerID, State: "activa", Title: "Sala de ensayo sintética", RateUnit: "hora", UnitPriceCLP: 12000, Currency: "CLP", RateVersion: 2, TimeZone: "America/Santiago"},
		interval: rentalflow.Interval{StartAt: start, EndAt: start.Add(time.Hour)}, available: true,
	}
	verificationFor := func(accountID string) identityverification.Verification {
		verifiedAt := now.Add(-time.Hour)
		return identityverification.Verification{
			SubjectID: accountID, Kind: identityverification.KindIdentityDocument,
			State: identityverification.StateApproved, ReviewMethod: identityverification.ReviewMethodManualDocuments,
			VerifiedAt: verifiedAt, ExpiresAt: verifiedAt.AddDate(0, 12, 0),
		}
	}
	verificationRepo := &syntheticVerificationRepository{current: map[string]identityverification.Verification{
		ownerID: verificationFor(ownerID),
	}}
	identityGate, err := identityverification.NewService(verificationRepo)
	if err != nil {
		t.Fatal(err)
	}
	legalReader := &syntheticIdentityReader{quoteID: testQuoteID, parties: map[string]rentalflow.ContractIdentity{
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
	previews, err := rentalflow.NewContractPreviewService(backend, backend, backend, identityGate, legalReader, clock)
	if err != nil {
		t.Fatal(err)
	}
	auth := &authStub{principal: identity.Principal{AccountID: renterID}}
	handler := New(auth, availability, quotes, previews, nil)

	quoteBody := `{"space_id":"` + testSpaceID + `","start_at":"` + start.Format(time.RFC3339) + `","end_at":"` + start.Add(time.Hour).Format(time.RFC3339) + `"}`
	quoteRequest := httptest.NewRequest(http.MethodPost, "/api/v1/rental-quotes", strings.NewReader(quoteBody))
	quoteRequest.Header.Set("Content-Type", "application/json")
	quoteRequest.Header.Set("Authorization", "Bearer synthetic-session")
	quoteResponse := httptest.NewRecorder()
	handler.ServeHTTP(quoteResponse, quoteRequest)
	if quoteResponse.Code != http.StatusCreated {
		t.Fatalf("quote status=%d body=%s", quoteResponse.Code, quoteResponse.Body.String())
	}

	previewURL := "/api/v1/rental-quotes/" + testQuoteID + "/contract-preview"
	verificationRepo.current[renterID] = verificationFor(renterID)
	verificationReadsBeforeOutsider := len(verificationRepo.calls)
	auth.principal.AccountID = "55555555-5555-4555-8555-555555555555"
	outsiderRequest := httptest.NewRequest(http.MethodPost, previewURL, nil)
	outsiderRequest.Header.Set("Authorization", "Bearer synthetic-session")
	outsiderResponse := httptest.NewRecorder()
	handler.ServeHTTP(outsiderResponse, outsiderRequest)
	if outsiderResponse.Code != http.StatusNotFound || outsiderResponse.Header().Get("Cache-Control") != "no-store" ||
		len(verificationRepo.calls) != verificationReadsBeforeOutsider || legalReader.calls != 0 ||
		strings.Contains(outsiderResponse.Body.String(), "Arrendador de prueba") ||
		strings.Contains(outsiderResponse.Body.String(), "Arrendataria de prueba") {
		t.Fatalf("nonparticipant preview was not hidden before identity reads: status=%d verificationReads=%d legalReads=%d body=%s", outsiderResponse.Code, len(verificationRepo.calls)-verificationReadsBeforeOutsider, legalReader.calls, outsiderResponse.Body.String())
	}
	auth.principal.AccountID = renterID

	delete(verificationRepo.current, renterID)
	ownerVerification := verificationRepo.current[ownerID]
	delete(verificationRepo.current, ownerID)
	ownerMissingRequest := httptest.NewRequest(http.MethodPost, previewURL, nil)
	ownerMissingRequest.Header.Set("Authorization", "Bearer synthetic-session")
	ownerMissingResponse := httptest.NewRecorder()
	handler.ServeHTTP(ownerMissingResponse, ownerMissingRequest)
	if ownerMissingResponse.Code != http.StatusConflict || ownerMissingResponse.Header().Get("Cache-Control") != "no-store" || legalReader.calls != 0 ||
		strings.Contains(ownerMissingResponse.Body.String(), "Arrendador de prueba") || strings.Contains(ownerMissingResponse.Body.String(), "Arrendataria de prueba") ||
		strings.Contains(ownerMissingResponse.Body.String(), "11.111.111-1") || strings.Contains(ownerMissingResponse.Body.String(), "22.222.222-2") ||
		strings.Contains(ownerMissingResponse.Body.String(), "11111111-1") || strings.Contains(ownerMissingResponse.Body.String(), "22222222-2") {
		t.Fatalf("missing owner identity was not blocked before legal disclosure: status=%d calls=%d body=%s", ownerMissingResponse.Code, legalReader.calls, ownerMissingResponse.Body.String())
	}
	verificationRepo.current[ownerID] = ownerVerification

	previewRequest := httptest.NewRequest(http.MethodPost, previewURL, nil)
	previewRequest.Header.Set("Authorization", "Bearer synthetic-session")
	blockedResponse := httptest.NewRecorder()
	handler.ServeHTTP(blockedResponse, previewRequest)
	if blockedResponse.Code != http.StatusConflict || blockedResponse.Header().Get("Cache-Control") != "no-store" || legalReader.calls != 0 ||
		strings.Contains(blockedResponse.Body.String(), "Arrendador de prueba") || strings.Contains(blockedResponse.Body.String(), "Arrendataria de prueba") ||
		strings.Contains(blockedResponse.Body.String(), "11.111.111-1") || strings.Contains(blockedResponse.Body.String(), "22.222.222-2") ||
		strings.Contains(blockedResponse.Body.String(), "11111111-1") || strings.Contains(blockedResponse.Body.String(), "22222222-2") {
		t.Fatalf("missing renter identity was not blocked before legal disclosure: status=%d calls=%d body=%s", blockedResponse.Code, legalReader.calls, blockedResponse.Body.String())
	}

	verificationRepo.current[renterID] = verificationFor(renterID)
	verifiedRequest := httptest.NewRequest(http.MethodPost, previewURL, nil)
	verifiedRequest.Header.Set("Authorization", "Bearer synthetic-session")
	verifiedResponse := httptest.NewRecorder()
	handler.ServeHTTP(verifiedResponse, verifiedRequest)
	if verifiedResponse.Code != http.StatusOK || legalReader.calls != 2 {
		t.Fatalf("current identities should permit participant preview: status=%d calls=%d body=%s", verifiedResponse.Code, legalReader.calls, verifiedResponse.Body.String())
	}

	verificationRepo.err = errors.New("synthetic verification repository unavailable")
	unavailableRequest := httptest.NewRequest(http.MethodPost, previewURL, nil)
	unavailableRequest.Header.Set("Authorization", "Bearer synthetic-session")
	unavailableResponse := httptest.NewRecorder()
	handler.ServeHTTP(unavailableResponse, unavailableRequest)
	if unavailableResponse.Code != http.StatusServiceUnavailable || legalReader.calls != 2 ||
		strings.Contains(unavailableResponse.Body.String(), "Arrendador de prueba") ||
		strings.Contains(unavailableResponse.Body.String(), "Arrendataria de prueba") ||
		strings.Contains(unavailableResponse.Body.String(), "synthetic verification repository unavailable") {
		t.Fatalf("verification repository failure was not hidden and failed closed: status=%d reads=%d body=%s", unavailableResponse.Code, legalReader.calls, unavailableResponse.Body.String())
	}
	verificationRepo.err = nil

	expiredVerification := verificationRepo.current[renterID]
	expiredVerification.ExpiresAt = now
	verificationRepo.current[renterID] = expiredVerification
	expiredRequest := httptest.NewRequest(http.MethodPost, previewURL, nil)
	expiredRequest.Header.Set("Authorization", "Bearer synthetic-session")
	expiredResponse := httptest.NewRecorder()
	handler.ServeHTTP(expiredResponse, expiredRequest)
	if expiredResponse.Code != http.StatusConflict || expiredResponse.Header().Get("Cache-Control") != "no-store" || legalReader.calls != 2 ||
		strings.Contains(expiredResponse.Body.String(), "Arrendador de prueba") || strings.Contains(expiredResponse.Body.String(), "Arrendataria de prueba") ||
		strings.Contains(expiredResponse.Body.String(), "11.111.111-1") || strings.Contains(expiredResponse.Body.String(), "22.222.222-2") ||
		strings.Contains(expiredResponse.Body.String(), "11111111-1") || strings.Contains(expiredResponse.Body.String(), "22222222-2") {
		t.Fatalf("expired identity was not blocked before further legal reads: status=%d calls=%d body=%s", expiredResponse.Code, legalReader.calls, expiredResponse.Body.String())
	}

	revokedAt := now.Add(-time.Minute)
	renterVerification := verificationFor(renterID)
	renterVerification.RevokedAt = &revokedAt
	verificationRepo.current[renterID] = renterVerification
	revokedRequest := httptest.NewRequest(http.MethodPost, previewURL, nil)
	revokedRequest.Header.Set("Authorization", "Bearer synthetic-session")
	revokedResponse := httptest.NewRecorder()
	handler.ServeHTTP(revokedResponse, revokedRequest)
	if revokedResponse.Code != http.StatusConflict || revokedResponse.Header().Get("Cache-Control") != "no-store" || legalReader.calls != 2 ||
		strings.Contains(revokedResponse.Body.String(), "Arrendador de prueba") || strings.Contains(revokedResponse.Body.String(), "Arrendataria de prueba") ||
		strings.Contains(revokedResponse.Body.String(), "11.111.111-1") || strings.Contains(revokedResponse.Body.String(), "22.222.222-2") ||
		strings.Contains(revokedResponse.Body.String(), "11111111-1") || strings.Contains(revokedResponse.Body.String(), "22222222-2") {
		t.Fatalf("revoked identity was not blocked before further legal reads: status=%d calls=%d body=%s", revokedResponse.Code, legalReader.calls, revokedResponse.Body.String())
	}
	verificationRepo.current[renterID] = verificationFor(renterID)

	ownerFixtureVerification := verificationFor(ownerID)
	ownerFixtureVerification.Kind = "kyb"
	ownerFixtureVerification.ReviewMethod = "local-fixture-v1"
	verificationRepo.current[ownerID] = ownerFixtureVerification
	ownerFixtureRequest := httptest.NewRequest(http.MethodPost, previewURL, nil)
	ownerFixtureRequest.Header.Set("Authorization", "Bearer synthetic-session")
	ownerFixtureResponse := httptest.NewRecorder()
	handler.ServeHTTP(ownerFixtureResponse, ownerFixtureRequest)
	if ownerFixtureResponse.Code != http.StatusConflict || legalReader.calls != 2 ||
		strings.Contains(ownerFixtureResponse.Body.String(), "Arrendador de prueba") ||
		strings.Contains(ownerFixtureResponse.Body.String(), "Arrendataria de prueba") {
		t.Fatalf("owner fixture-only or KYB assertion was accepted for contract preview: status=%d calls=%d body=%s", ownerFixtureResponse.Code, legalReader.calls, ownerFixtureResponse.Body.String())
	}
	verificationRepo.current[ownerID] = verificationFor(ownerID)

	fixtureOnlyVerification := verificationFor(renterID)
	fixtureOnlyVerification.Kind = "kyc"
	fixtureOnlyVerification.ReviewMethod = "local-fixture-v1"
	verificationRepo.current[renterID] = fixtureOnlyVerification
	fixtureOnlyRequest := httptest.NewRequest(http.MethodPost, previewURL, nil)
	fixtureOnlyRequest.Header.Set("Authorization", "Bearer synthetic-session")
	fixtureOnlyResponse := httptest.NewRecorder()
	handler.ServeHTTP(fixtureOnlyResponse, fixtureOnlyRequest)
	if fixtureOnlyResponse.Code != http.StatusConflict || legalReader.calls != 2 ||
		strings.Contains(fixtureOnlyResponse.Body.String(), "Arrendador de prueba") ||
		strings.Contains(fixtureOnlyResponse.Body.String(), "Arrendataria de prueba") {
		t.Fatalf("fixture-only or generic KYC assertion was accepted for contract preview: status=%d calls=%d body=%s", fixtureOnlyResponse.Code, legalReader.calls, fixtureOnlyResponse.Body.String())
	}

	verificationRepo.current[renterID] = verificationFor(renterID)
	mismatchedIdentity := legalReader.parties[renterID]
	mismatchedIdentity.SubjectID = ownerID
	mismatchedIdentity.LegalName = "Identidad ajena de prueba"
	legalReader.parties[renterID] = mismatchedIdentity

	scopeRequest := httptest.NewRequest(http.MethodPost, previewURL, nil)
	scopeRequest.Header.Set("Authorization", "Bearer synthetic-session")
	scopeResponse := httptest.NewRecorder()
	handler.ServeHTTP(scopeResponse, scopeRequest)
	if scopeResponse.Code != http.StatusServiceUnavailable || legalReader.calls != 4 ||
		strings.Contains(scopeResponse.Body.String(), "Arrendador de prueba") ||
		strings.Contains(scopeResponse.Body.String(), "Identidad ajena de prueba") ||
		strings.Contains(scopeResponse.Body.String(), "22.222.222-2") {
		t.Fatalf("mismatched legal identity scope was not failed closed at HTTP boundary: status=%d calls=%d body=%s", scopeResponse.Code, legalReader.calls, scopeResponse.Body.String())
	}

	legalReader.err = errors.New("synthetic legal identity reader unavailable")
	readerFailureRequest := httptest.NewRequest(http.MethodPost, previewURL, nil)
	readerFailureRequest.Header.Set("Authorization", "Bearer synthetic-session")
	readerFailureResponse := httptest.NewRecorder()
	handler.ServeHTTP(readerFailureResponse, readerFailureRequest)
	if readerFailureResponse.Code != http.StatusServiceUnavailable || legalReader.calls != 5 ||
		strings.Contains(readerFailureResponse.Body.String(), "Arrendador de prueba") ||
		strings.Contains(readerFailureResponse.Body.String(), "synthetic legal identity reader unavailable") {
		t.Fatalf("legal identity reader failure was not hidden and failed closed: status=%d calls=%d body=%s", readerFailureResponse.Code, legalReader.calls, readerFailureResponse.Body.String())
	}

	legalReader.err = nil
	legalReader.parties[renterID] = rentalflow.ContractIdentity{SubjectID: renterID, LegalName: "Arrendataria de prueba", RUT: "22.222.222-2"}
	auth.principal.AccountID = ownerID
	ownerPreviewRequest := httptest.NewRequest(http.MethodPost, previewURL, nil)
	ownerPreviewRequest.Header.Set("Authorization", "Bearer synthetic-session")
	ownerPreviewResponse := httptest.NewRecorder()
	handler.ServeHTTP(ownerPreviewResponse, ownerPreviewRequest)
	if ownerPreviewResponse.Code != http.StatusOK || ownerPreviewResponse.Header().Get("Cache-Control") != "no-store" || legalReader.calls != 7 {
		t.Fatalf("quote owner participant could not preview the contract: status=%d legalReads=%d body=%s", ownerPreviewResponse.Code, legalReader.calls, ownerPreviewResponse.Body.String())
	}

	identityExpiry := now.Add(time.Nanosecond)
	ownerNearExpiry := verificationFor(ownerID)
	ownerNearExpiry.ExpiresAt = identityExpiry
	renterNearExpiry := verificationFor(renterID)
	renterNearExpiry.ExpiresAt = identityExpiry
	verificationRepo.current[ownerID] = ownerNearExpiry
	verificationRepo.current[renterID] = renterNearExpiry
	nearExpiryRequest := httptest.NewRequest(http.MethodPost, previewURL, nil)
	nearExpiryRequest.Header.Set("Authorization", "Bearer synthetic-session")
	nearExpiryResponse := httptest.NewRecorder()
	handler.ServeHTTP(nearExpiryResponse, nearExpiryRequest)
	if nearExpiryResponse.Code != http.StatusOK || legalReader.calls != 9 {
		t.Fatalf("identities expiring one nanosecond in the future were rejected: status=%d legalReads=%d body=%s", nearExpiryResponse.Code, legalReader.calls, nearExpiryResponse.Body.String())
	}

	now = identityExpiry
	backend.quote.ExpiresAt = now.Add(15 * time.Minute)
	ownerNearExpiry.ExpiresAt = now
	renterNearExpiry.ExpiresAt = now.Add(time.Nanosecond)
	verificationRepo.current[ownerID] = ownerNearExpiry
	verificationRepo.current[renterID] = renterNearExpiry
	ownerExpiredRequest := httptest.NewRequest(http.MethodPost, previewURL, nil)
	ownerExpiredRequest.Header.Set("Authorization", "Bearer synthetic-session")
	ownerExpiredResponse := httptest.NewRecorder()
	handler.ServeHTTP(ownerExpiredResponse, ownerExpiredRequest)
	if ownerExpiredResponse.Code != http.StatusConflict || legalReader.calls != 9 ||
		strings.Contains(ownerExpiredResponse.Body.String(), "Arrendador de prueba") {
		t.Fatalf("owner identity at exact expiry was not blocked before legal reads: status=%d legalReads=%d body=%s", ownerExpiredResponse.Code, legalReader.calls, ownerExpiredResponse.Body.String())
	}

	ownerNearExpiry.ExpiresAt = now.Add(time.Nanosecond)
	renterNearExpiry.ExpiresAt = now
	verificationRepo.current[ownerID] = ownerNearExpiry
	verificationRepo.current[renterID] = renterNearExpiry
	renterExpiredRequest := httptest.NewRequest(http.MethodPost, previewURL, nil)
	renterExpiredRequest.Header.Set("Authorization", "Bearer synthetic-session")
	renterExpiredResponse := httptest.NewRecorder()
	handler.ServeHTTP(renterExpiredResponse, renterExpiredRequest)
	if renterExpiredResponse.Code != http.StatusConflict || legalReader.calls != 9 ||
		strings.Contains(renterExpiredResponse.Body.String(), "Arrendataria de prueba") {
		t.Fatalf("renter identity at exact expiry was not blocked before legal reads: status=%d legalReads=%d body=%s", renterExpiredResponse.Code, legalReader.calls, renterExpiredResponse.Body.String())
	}

	futureVerifiedOwner := verificationFor(ownerID)
	futureVerifiedOwner.VerifiedAt = now.Add(time.Nanosecond)
	futureVerifiedOwner.ExpiresAt = now.Add(time.Hour)
	verificationRepo.current[ownerID] = futureVerifiedOwner
	verificationRepo.current[renterID] = verificationFor(renterID)
	futureVerifiedRequest := httptest.NewRequest(http.MethodPost, previewURL, nil)
	futureVerifiedRequest.Header.Set("Authorization", "Bearer synthetic-session")
	futureVerifiedResponse := httptest.NewRecorder()
	handler.ServeHTTP(futureVerifiedResponse, futureVerifiedRequest)
	if futureVerifiedResponse.Code != http.StatusConflict || legalReader.calls != 9 ||
		strings.Contains(futureVerifiedResponse.Body.String(), "Arrendador de prueba") ||
		strings.Contains(futureVerifiedResponse.Body.String(), "Arrendataria de prueba") {
		t.Fatalf("future-dated owner verification reached legal reads: status=%d legalReads=%d body=%s", futureVerifiedResponse.Code, legalReader.calls, futureVerifiedResponse.Body.String())
	}

	overlongOwnerVerification := verificationFor(ownerID)
	overlongOwnerVerification.VerifiedAt = now.AddDate(0, -24, 0)
	overlongOwnerVerification.ExpiresAt = overlongOwnerVerification.VerifiedAt.AddDate(0, 24, 0).Add(time.Nanosecond)
	verificationRepo.current[ownerID] = overlongOwnerVerification
	verificationRepo.current[renterID] = verificationFor(renterID)
	overlongVerificationRequest := httptest.NewRequest(http.MethodPost, previewURL, nil)
	overlongVerificationRequest.Header.Set("Authorization", "Bearer synthetic-session")
	overlongVerificationResponse := httptest.NewRecorder()
	handler.ServeHTTP(overlongVerificationResponse, overlongVerificationRequest)
	if overlongVerificationResponse.Code != http.StatusConflict || legalReader.calls != 9 ||
		strings.Contains(overlongVerificationResponse.Body.String(), "Arrendador de prueba") ||
		strings.Contains(overlongVerificationResponse.Body.String(), "Arrendataria de prueba") {
		t.Fatalf("owner verification exceeding the 24-month validity limit reached legal reads: status=%d legalReads=%d body=%s", overlongVerificationResponse.Code, legalReader.calls, overlongVerificationResponse.Body.String())
	}

	verificationRepo.current[ownerID] = verificationFor(ownerID)
	overlongRenterVerification := verificationFor(renterID)
	overlongRenterVerification.VerifiedAt = now.AddDate(0, -24, 0)
	overlongRenterVerification.ExpiresAt = overlongRenterVerification.VerifiedAt.AddDate(0, 24, 0).Add(time.Nanosecond)
	verificationRepo.current[renterID] = overlongRenterVerification
	overlongRenterRequest := httptest.NewRequest(http.MethodPost, previewURL, nil)
	overlongRenterRequest.Header.Set("Authorization", "Bearer synthetic-session")
	overlongRenterResponse := httptest.NewRecorder()
	handler.ServeHTTP(overlongRenterResponse, overlongRenterRequest)
	if overlongRenterResponse.Code != http.StatusConflict || legalReader.calls != 9 ||
		strings.Contains(overlongRenterResponse.Body.String(), "Arrendador de prueba") ||
		strings.Contains(overlongRenterResponse.Body.String(), "Arrendataria de prueba") {
		t.Fatalf("renter verification exceeding the 24-month validity limit reached legal reads: status=%d legalReads=%d body=%s", overlongRenterResponse.Code, legalReader.calls, overlongRenterResponse.Body.String())
	}

	futureRevokedOwner := verificationFor(ownerID)
	futureRevokedAt := now.Add(time.Hour)
	futureRevokedOwner.RevokedAt = &futureRevokedAt
	verificationRepo.current[ownerID] = futureRevokedOwner
	verificationRepo.current[renterID] = verificationFor(renterID)
	futureRevocationRequest := httptest.NewRequest(http.MethodPost, previewURL, nil)
	futureRevocationRequest.Header.Set("Authorization", "Bearer synthetic-session")
	futureRevocationResponse := httptest.NewRecorder()
	handler.ServeHTTP(futureRevocationResponse, futureRevocationRequest)
	if futureRevocationResponse.Code != http.StatusConflict || legalReader.calls != 9 ||
		strings.Contains(futureRevocationResponse.Body.String(), "Arrendador de prueba") ||
		strings.Contains(futureRevocationResponse.Body.String(), "Arrendataria de prueba") {
		t.Fatalf("future-revoked owner verification reached legal reads: status=%d legalReads=%d body=%s", futureRevocationResponse.Code, legalReader.calls, futureRevocationResponse.Body.String())
	}

	ownerVerifiedAtAsOf := verificationFor(ownerID)
	ownerVerifiedAtAsOf.VerifiedAt = now
	ownerVerifiedAtAsOf.ExpiresAt = now.AddDate(0, 24, 0)
	renterVerifiedAtAsOf := verificationFor(renterID)
	renterVerifiedAtAsOf.VerifiedAt = now
	renterVerifiedAtAsOf.ExpiresAt = now.AddDate(0, 24, 0)
	verificationRepo.current[ownerID] = ownerVerifiedAtAsOf
	verificationRepo.current[renterID] = renterVerifiedAtAsOf
	exactAsOfRequest := httptest.NewRequest(http.MethodPost, previewURL, nil)
	exactAsOfRequest.Header.Set("Authorization", "Bearer synthetic-session")
	exactAsOfResponse := httptest.NewRecorder()
	handler.ServeHTTP(exactAsOfResponse, exactAsOfRequest)
	if exactAsOfResponse.Code != http.StatusOK || legalReader.calls != 11 || exactAsOfResponse.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("verification at exact as-of timestamp was not accepted: status=%d legalReads=%d body=%s", exactAsOfResponse.Code, legalReader.calls, exactAsOfResponse.Body.String())
	}

	verificationRepo.current[ownerID] = verificationFor(ownerID)
	futureRevokedRenter := verificationFor(renterID)
	futureRenterRevokedAt := now.Add(time.Hour)
	futureRevokedRenter.RevokedAt = &futureRenterRevokedAt
	verificationRepo.current[renterID] = futureRevokedRenter
	verificationReadsBeforeRenterRevocation := len(verificationRepo.calls)
	futureRenterRevocationRequest := httptest.NewRequest(http.MethodPost, previewURL, nil)
	futureRenterRevocationRequest.Header.Set("Authorization", "Bearer synthetic-session")
	futureRenterRevocationResponse := httptest.NewRecorder()
	handler.ServeHTTP(futureRenterRevocationResponse, futureRenterRevocationRequest)
	if futureRenterRevocationResponse.Code != http.StatusConflict || legalReader.calls != 11 ||
		len(verificationRepo.calls)-verificationReadsBeforeRenterRevocation != 2 ||
		strings.Contains(futureRenterRevocationResponse.Body.String(), "Arrendador de prueba") ||
		strings.Contains(futureRenterRevocationResponse.Body.String(), "Arrendataria de prueba") {
		t.Fatalf("future-revoked renter reached legal reads after valid owner gate: status=%d gateReads=%d legalReads=%d body=%s", futureRenterRevocationResponse.Code, len(verificationRepo.calls)-verificationReadsBeforeRenterRevocation, legalReader.calls, futureRenterRevocationResponse.Body.String())
	}
}

func TestSyntheticHTTPPreviewStopsBeforeLegalReadsWhenRenterVerificationStoreFails(t *testing.T) {
	const ownerID = "44444444-4444-4444-8444-444444444444"
	const renterID = "33333333-3333-4333-8333-333333333333"
	now := time.Date(2030, time.January, 1, 12, 0, 0, 0, time.UTC)
	start := time.Date(2030, time.January, 2, 13, 0, 0, 0, time.UTC)
	backend := &syntheticFlowBackend{
		listing:  rentalflow.Listing{ID: testSpaceID, OwnerID: ownerID, State: "activa", Title: "Sala de ensayo sintética", RateUnit: "hora", UnitPriceCLP: 12000, Currency: "CLP", RateVersion: 2, TimeZone: "America/Santiago"},
		interval: rentalflow.Interval{StartAt: start, EndAt: start.Add(time.Hour)}, available: true,
	}
	verificationFor := func(accountID string) identityverification.Verification {
		verifiedAt := now.Add(-time.Hour)
		return identityverification.Verification{
			SubjectID: accountID, Kind: identityverification.KindIdentityDocument,
			State: identityverification.StateApproved, ReviewMethod: identityverification.ReviewMethodManualDocuments,
			VerifiedAt: verifiedAt, ExpiresAt: verifiedAt.AddDate(0, 12, 0),
		}
	}
	privateFailure := errors.New("private synthetic renter verification repository failure")
	verificationRepo := &syntheticVerificationRepository{
		current: map[string]identityverification.Verification{
			ownerID: verificationFor(ownerID), renterID: verificationFor(renterID),
		},
		errByAccount: map[string]error{renterID: privateFailure},
	}
	identityGate, err := identityverification.NewService(verificationRepo)
	if err != nil {
		t.Fatal(err)
	}
	legalReader := &syntheticIdentityReader{quoteID: testQuoteID, parties: map[string]rentalflow.ContractIdentity{
		ownerID:  {SubjectID: ownerID, LegalName: "Arrendador sintético privado", RUT: "11.111.111-1"},
		renterID: {SubjectID: renterID, LegalName: "Arrendataria sintética privada", RUT: "22.222.222-2"},
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
	previews, err := rentalflow.NewContractPreviewService(backend, backend, backend, identityGate, legalReader, clock)
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

	previewRequest := httptest.NewRequest(http.MethodPost, "/api/v1/rental-quotes/"+testQuoteID+"/contract-preview", nil)
	previewRequest.Header.Set("Authorization", "Bearer synthetic-session")
	previewResponse := httptest.NewRecorder()
	handler.ServeHTTP(previewResponse, previewRequest)
	body := previewResponse.Body.String()
	if previewResponse.Code != http.StatusServiceUnavailable || !strings.Contains(body, "service_unavailable") ||
		previewResponse.Header().Get("Cache-Control") != "no-store" || legalReader.calls != 0 ||
		len(verificationRepo.calls) != 2 || verificationRepo.calls[0] != ownerID || verificationRepo.calls[1] != renterID ||
		strings.Contains(body, "Arrendador sintético privado") || strings.Contains(body, "Arrendataria sintética privada") ||
		strings.Contains(body, "11.111.111-1") || strings.Contains(body, "22.222.222-2") || strings.Contains(body, privateFailure.Error()) {
		t.Fatalf("renter verification read failure reached legal identity or leaked details: status=%d reads=%v legalReads=%d body=%s", previewResponse.Code, verificationRepo.calls, legalReader.calls, body)
	}
}
