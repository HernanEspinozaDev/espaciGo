package verificationhttp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
	"github.com/HernanEspinozaDev/espaciGo/internal/verification"
)

type mockAuthWithErr struct {
	err error
}

func (m mockAuthWithErr) Authorize(_ context.Context, _ identity.Secret, _ identity.Role, _ identity.ActivityKind) (identity.Principal, error) {
	if m.err != nil {
		return identity.Principal{}, m.err
	}
	return identity.Principal{AccountID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", Roles: []identity.Role{"arrendatario"}}, nil
}

type failingRepoForHandler struct {
	repo
	listOwnErr     error
	eligibilityErr error
	pendingErr     error
	rejectedErr    error
}

func (f *failingRepoForHandler) ListOwn(ctx context.Context, owner string) ([]verification.Case, error) {
	if f.listOwnErr != nil {
		return nil, f.listOwnErr
	}
	return f.repo.ListOwn(ctx, owner)
}

func (f *failingRepoForHandler) ListPending(ctx context.Context) ([]verification.Case, error) {
	if f.pendingErr != nil {
		return nil, f.pendingErr
	}
	return f.repo.ListPending(ctx)
}

func (f *failingRepoForHandler) ListRejected(ctx context.Context) ([]verification.Case, error) {
	if f.rejectedErr != nil {
		return nil, f.rejectedErr
	}
	return f.repo.ListRejected(ctx)
}

func TestHandlerCORSAndQueryBoundaries(t *testing.T) {
	handler := NewHandler(testAuth{}, nil, []string{"http://localhost:4200", "https://app.espacigo.com"})

	// 1. Allowed origin sets CORS headers
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/verifications", nil)
	req.Header.Set("Origin", "http://localhost:4200")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 for OPTIONS with allowed origin, got: %d", rec.Code)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "http://localhost:4200" {
		t.Fatalf("unexpected allow origin: %q", rec.Header().Get("Access-Control-Allow-Origin"))
	}
	if rec.Header().Get("Access-Control-Allow-Methods") != "GET, POST, DELETE, OPTIONS" {
		t.Fatalf("unexpected allow methods: %q", rec.Header().Get("Access-Control-Allow-Methods"))
	}

	// 2. Denied origin returns 403
	reqDenied := httptest.NewRequest(http.MethodGet, "/api/v1/verifications", nil)
	reqDenied.Header.Set("Origin", "http://malicious-origin.com")
	recDenied := httptest.NewRecorder()
	handler.ServeHTTP(recDenied, reqDenied)
	if recDenied.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for denied origin, got: %d", recDenied.Code)
	}
	if !strings.Contains(recDenied.Body.String(), "origin_denied") {
		t.Fatalf("expected origin_denied in body, got: %s", recDenied.Body.String())
	}

	// 3. RawQuery rejection
	reqQuery := httptest.NewRequest(http.MethodGet, "/api/v1/verifications?page=1", nil)
	recQuery := httptest.NewRecorder()
	handler.ServeHTTP(recQuery, reqQuery)
	if recQuery.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for request with query params, got: %d", recQuery.Code)
	}
	if !strings.Contains(recQuery.Body.String(), "invalid_request") {
		t.Fatalf("expected invalid_request code, got: %s", recQuery.Body.String())
	}

	// 4. Retired external provider route rejection
	for _, path := range []string{
		"/api/v1/webhooks/didit",
		"/api/v1/verifications/didit/webhook",
		"/api/v1/verifications/didit/intent",
		"/api/v1/verifications/didit/intent/123",
	} {
		reqDidit := httptest.NewRequest(http.MethodPost, path, nil)
		recDidit := httptest.NewRecorder()
		handler.ServeHTTP(recDidit, reqDidit)
		if recDidit.Code != http.StatusNotFound {
			t.Fatalf("expected 404 for retired route %s, got: %d", path, recDidit.Code)
		}
	}
}

func TestHandlerAuthBoundaries(t *testing.T) {
	r := newRepo()
	svc, _ := verification.NewService(r, &testIDs{}, verification.LocalFixtureProvider{}, func() time.Time { return time.Unix(7, 0) })
	handler := NewHandler(testAuth{}, svc, nil)

	// 1. Missing or invalid Authorization header format
	for _, badAuth := range []string{"", "Token 123", "Bearer", "Bearer t1 t2"} {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/verifications", nil)
		if badAuth != "" {
			req.Header.Set("Authorization", badAuth)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 for bad auth %q, got: %d", badAuth, rec.Code)
		}
	}

	// 2. Non-admin forbidden on admin endpoint
	reqAdmin := httptest.NewRequest(http.MethodGet, "/api/v1/admin/verifications", nil)
	reqAdmin.Header.Set("Authorization", "Bearer non-admin-token")
	recAdmin := httptest.NewRecorder()
	handler.ServeHTTP(recAdmin, reqAdmin)
	if recAdmin.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-admin on admin endpoint, got: %d", recAdmin.Code)
	}

	// 3. Generic auth failure propagates as 401
	errHandler := NewHandler(mockAuthWithErr{err: errors.New("auth server down")}, svc, nil)
	reqErr := httptest.NewRequest(http.MethodGet, "/api/v1/verifications", nil)
	reqErr.Header.Set("Authorization", "Bearer some-token")
	recErr := httptest.NewRecorder()
	errHandler.ServeHTTP(recErr, reqErr)
	if recErr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for generic auth failure, got: %d", recErr.Code)
	}
}

func TestHandlerDecodeAndJSONBoundaries(t *testing.T) {
	r := newRepo()
	svc, _ := verification.NewService(r, &testIDs{}, verification.LocalFixtureProvider{}, func() time.Time { return time.Unix(7, 0) })
	handler := NewHandler(testAuth{}, svc, nil)

	// 1. Unsupported Media Type (not application/json)
	reqBadMedia := httptest.NewRequest(http.MethodPost, "/api/v1/verifications", strings.NewReader(`{"type":"rut"}`))
	reqBadMedia.Header.Set("Authorization", "Bearer user")
	reqBadMedia.Header.Set("Content-Type", "text/plain")
	recBadMedia := httptest.NewRecorder()
	handler.ServeHTTP(recBadMedia, reqBadMedia)
	if recBadMedia.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("expected 415 for text/plain, got: %d", recBadMedia.Code)
	}

	// 2. Corrupted JSON body
	reqBadJSON := httptest.NewRequest(http.MethodPost, "/api/v1/verifications", strings.NewReader(`{invalid-json`))
	reqBadJSON.Header.Set("Authorization", "Bearer user")
	reqBadJSON.Header.Set("Content-Type", "application/json")
	recBadJSON := httptest.NewRecorder()
	handler.ServeHTTP(recBadJSON, reqBadJSON)
	if recBadJSON.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for corrupted JSON, got: %d", recBadJSON.Code)
	}

	// 3. Extra JSON object in body
	reqExtra := httptest.NewRequest(http.MethodPost, "/api/v1/verifications", strings.NewReader(`{"type":"rut"}{"extra":true}`))
	reqExtra.Header.Set("Authorization", "Bearer user")
	reqExtra.Header.Set("Content-Type", "application/json")
	recExtra := httptest.NewRecorder()
	handler.ServeHTTP(recExtra, reqExtra)
	if recExtra.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for multiple JSON objects, got: %d", recExtra.Code)
	}
	if !strings.Contains(recExtra.Body.String(), "Solo se admite un objeto JSON") {
		t.Fatalf("expected multi-object error message, got: %s", recExtra.Body.String())
	}
}

func TestHandlerServiceErrorMappingAndUnconfiguredDeclarationBoundaries(t *testing.T) {
	// 1. Unconfigured declaration service returns 503
	handlerNoDec := NewHandler(testAuth{}, nil, nil)
	reqDec := httptest.NewRequest(http.MethodGet, "/api/v1/verifications/identity-declaration", nil)
	reqDec.Header.Set("Authorization", "Bearer user")
	recDec := httptest.NewRecorder()
	handlerNoDec.ServeHTTP(recDec, reqDec)
	if recDec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 for unconfigured declaration, got: %d", recDec.Code)
	}

	// 2. ServiceError mappings
	recs := make([]*httptest.ResponseRecorder, 4)
	for i := range recs {
		recs[i] = httptest.NewRecorder()
	}
	serviceError(recs[0], verification.ErrInvalid)
	if recs[0].Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for ErrInvalid, got: %d", recs[0].Code)
	}
	serviceError(recs[1], verification.ErrNotFound)
	if recs[1].Code != http.StatusNotFound {
		t.Fatalf("expected 404 for ErrNotFound, got: %d", recs[1].Code)
	}
	serviceError(recs[2], verification.ErrConflict)
	if recs[2].Code != http.StatusConflict {
		t.Fatalf("expected 409 for ErrConflict, got: %d", recs[2].Code)
	}
	serviceError(recs[3], errors.New("unknown DB fault"))
	if recs[3].Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 for unknown error, got: %d", recs[3].Code)
	}

	// 3. Repository errors in ListOwn, Pending and Rejected endpoints
	failingR := &failingRepoForHandler{
		repo:        *newRepo(),
		listOwnErr:  errors.New("list own failed"),
		pendingErr:  errors.New("pending failed"),
		rejectedErr: errors.New("rejected failed"),
	}
	failingSvc, _ := verification.NewService(failingR, &testIDs{}, verification.LocalFixtureProvider{}, func() time.Time { return time.Unix(7, 0) })
	hWithFailingSvc := NewHandler(testAuth{}, failingSvc, nil)

	reqListOwn := httptest.NewRequest(http.MethodGet, "/api/v1/verifications", nil)
	reqListOwn.Header.Set("Authorization", "Bearer user")
	recListOwn := httptest.NewRecorder()
	hWithFailingSvc.ServeHTTP(recListOwn, reqListOwn)
	if recListOwn.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 for failing ListOwn, got: %d", recListOwn.Code)
	}

	reqPending := httptest.NewRequest(http.MethodGet, "/api/v1/admin/verifications", nil)
	reqPending.Header.Set("Authorization", "Bearer admin")
	recPending := httptest.NewRecorder()
	hWithFailingSvc.ServeHTTP(recPending, reqPending)
	if recPending.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 for failing Pending, got: %d", recPending.Code)
	}

	reqRejected := httptest.NewRequest(http.MethodGet, "/api/v1/admin/verifications/failed", nil)
	reqRejected.Header.Set("Authorization", "Bearer admin")
	recRejected := httptest.NewRecorder()
	hWithFailingSvc.ServeHTTP(recRejected, reqRejected)
	if recRejected.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 for failing Rejected, got: %d", recRejected.Code)
	}
}

func TestHandlerEvidenceUploadValidationBoundaries(t *testing.T) {
	caseRepo := newRepo()
	caseID := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	caseRepo.cases[caseID] = verification.Case{
		ID:      caseID,
		OwnerID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
		Type:    "kyc",
		State:   "en_revision",
	}
	ids := &testIDs{}
	metadata := &evidenceRepoFake{items: map[string]verification.Evidence{}}
	storage := &evidenceStoreFake{blobs: map[string][]byte{}}
	evidenceSvc, err := verification.NewEvidenceService(caseRepo, metadata, storage, ids, func() time.Time { return time.Unix(8, 0) })
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(testAuth{}, nil, nil, evidenceSvc)

	// 1. Evidence upload with invalid JSON body
	reqBadJSON := httptest.NewRequest(http.MethodPost, "/api/v1/verifications/"+caseID+"/evidence", bytes.NewReader([]byte(`not-json`)))
	reqBadJSON.Header.Set("Authorization", "Bearer user")
	reqBadJSON.Header.Set("Content-Type", "application/json")
	recBadJSON := httptest.NewRecorder()
	handler.ServeHTTP(recBadJSON, reqBadJSON)
	if recBadJSON.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for bad JSON upload, got: %d", recBadJSON.Code)
	}

	// 2. Evidence upload with unexpected fixture code
	badPayload, _ := json.Marshal(map[string]string{"fixture_code": "unknown-fixture-code"})
	reqBadCode := httptest.NewRequest(http.MethodPost, "/api/v1/verifications/"+caseID+"/evidence", bytes.NewReader(badPayload))
	reqBadCode.Header.Set("Authorization", "Bearer user")
	reqBadCode.Header.Set("Content-Type", "application/json")
	recBadCode := httptest.NewRecorder()
	handler.ServeHTTP(recBadCode, reqBadCode)
	if recBadCode.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for invalid fixture code, got: %d", recBadCode.Code)
	}
	if !strings.Contains(recBadCode.Body.String(), "invalid_input") {
		t.Fatalf("expected invalid_input error code, got: %s", recBadCode.Body.String())
	}
}
