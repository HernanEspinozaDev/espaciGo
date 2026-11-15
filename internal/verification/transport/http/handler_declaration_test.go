package verificationhttp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
	"github.com/HernanEspinozaDev/espaciGo/internal/verification"
	"github.com/HernanEspinozaDev/espaciGo/internal/verification/declarationcrypto"
)

type declarationHTTPRepository struct {
	rows        map[string]verification.DeclarationRevision
	createCalls int
}

func (r *declarationHTTPRepository) CreateRevision(_ context.Context, in verification.DeclarationRevisionInput, seal verification.DeclarationSealer) (verification.DeclarationRevision, error) {
	r.createCalls++
	var revision int64 = 1
	for _, row := range r.rows {
		if row.AccountID == in.AccountID && row.Revision >= revision {
			revision = row.Revision + 1
		}
	}
	keyVersion, ciphertext, err := seal(revision)
	if err != nil {
		return verification.DeclarationRevision{}, err
	}
	row := verification.DeclarationRevision{ID: in.ID, AccountID: in.AccountID, Revision: revision, KeyVersion: keyVersion, Ciphertext: ciphertext, SchemaVersion: in.SchemaVersion, ConsentVersion: in.ConsentVersion, ConsentGivenAt: in.ConsentGivenAt, RetentionAt: in.RetentionAt, CreatedAt: in.ConsentGivenAt}
	r.rows[row.ID] = row
	return row, nil
}

func (r *declarationHTTPRepository) GetRevision(_ context.Context, accountID, id string) (verification.DeclarationRevision, error) {
	row, ok := r.rows[id]
	if !ok || row.AccountID != accountID {
		return verification.DeclarationRevision{}, verification.ErrNotFound
	}
	return row, nil
}

func (r *declarationHTTPRepository) GetCurrent(_ context.Context, accountID string) (verification.DeclarationRevision, error) {
	var found verification.DeclarationRevision
	for _, row := range r.rows {
		if row.AccountID == accountID && row.Revision > found.Revision {
			found = row
		}
	}
	if found.ID == "" {
		return verification.DeclarationRevision{}, verification.ErrNotFound
	}
	return found, nil
}

func declarationHandlerFixture(t *testing.T, repo *declarationHTTPRepository) http.Handler {
	t.Helper()
	keyring, err := declarationcrypto.New("synthetic-key", map[string][]byte{"synthetic-key": make([]byte, 32)})
	if err != nil {
		t.Fatal(err)
	}
	service, err := verification.NewDeclarationService(repo, keyring, &testIDs{}, func() time.Time { return time.Date(2026, time.November, 8, 12, 0, 0, 0, time.UTC) }, func(at time.Time) time.Time { return at.Add(24 * time.Hour) }, "consent-v1")
	if err != nil {
		t.Fatal(err)
	}
	return NewHandlerWithDeclaration(testAuth{}, nil, []string{"http://localhost:4200"}, nil, service)
}

func TestIdentityDeclarationHandlerSavesThenReadsOwnDeclaration(t *testing.T) {
	repo := &declarationHTTPRepository{rows: make(map[string]verification.DeclarationRevision)}
	handler := declarationHandlerFixture(t, repo)
	body := `{"first_name":"Synthetic","last_name":"Pérez","date_of_birth":"1990-01-02","rut":"11.222.333-9","id_country":"CHL","document_type":"ID","consent_given":true,"consent_version":"consent-v1"}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/verifications/identity-declaration", strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer owner")
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("POST status=%d cache=%q body=%s", response.Code, response.Header().Get("Cache-Control"), response.Body.String())
	}
	var saved map[string]json.RawMessage
	if err := json.Unmarshal(response.Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}
	var revision int64
	if err := json.Unmarshal(saved["revision"], &revision); err != nil || revision != 1 {
		t.Fatalf("saved revision=%d err=%v", revision, err)
	}
	if repo.createCalls != 1 {
		t.Fatalf("declaration writes=%d, want 1", repo.createCalls)
	}

	get := httptest.NewRequest(http.MethodGet, "/api/v1/verifications/identity-declaration", nil)
	get.Header.Set("Authorization", "Bearer owner")
	getResponse := httptest.NewRecorder()
	handler.ServeHTTP(getResponse, get)
	if getResponse.Code != http.StatusOK || getResponse.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("GET status=%d cache=%q body=%s", getResponse.Code, getResponse.Header().Get("Cache-Control"), getResponse.Body.String())
	}
	var current map[string]json.RawMessage
	if err := json.Unmarshal(getResponse.Body.Bytes(), &current); err != nil {
		t.Fatal(err)
	}
	var rut, firstName string
	if err := json.Unmarshal(current["rut"], &rut); err != nil || rut != "11222333-9" {
		t.Fatalf("GET rut=%q err=%v", rut, err)
	}
	if err := json.Unmarshal(current["first_name"], &firstName); err != nil || firstName != "Synthetic" {
		t.Fatalf("GET first_name=%q err=%v", firstName, err)
	}
}

func TestIdentityDeclarationHandlerRequiresCurrentConsentBeforeWrite(t *testing.T) {
	repo := &declarationHTTPRepository{rows: make(map[string]verification.DeclarationRevision)}
	handler := declarationHandlerFixture(t, repo)
	body := `{"first_name":"Synthetic","last_name":"Person","date_of_birth":"1990-01-02","rut":"11222333-9","id_country":"CHL","document_type":"ID","consent_given":true,"consent_version":"stale-v0"}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/verifications/identity-declaration", strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer owner")
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnprocessableEntity || repo.createCalls != 0 {
		t.Fatalf("POST status=%d writes=%d body=%s", response.Code, repo.createCalls, response.Body.String())
	}
}

func TestIdentityDeclarationHandlerRejectsMinorBeforeWrite(t *testing.T) {
	repo := &declarationHTTPRepository{rows: make(map[string]verification.DeclarationRevision)}
	handler := declarationHandlerFixture(t, repo)
	body := `{"first_name":"MinorSynthetic","last_name":"Person","date_of_birth":"2008-11-09","rut":"11222333-9","id_country":"CHL","document_type":"ID","consent_given":true,"consent_version":"consent-v1"}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/verifications/identity-declaration", strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer owner")
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnprocessableEntity || repo.createCalls != 0 {
		t.Fatalf("minor POST status=%d writes=%d body=%s", response.Code, repo.createCalls, response.Body.String())
	}
	if strings.Contains(response.Body.String(), "MinorSynthetic") || strings.Contains(response.Body.String(), "11222333") {
		t.Fatalf("minor error response reflected PII: %s", response.Body.String())
	}
}

func TestIdentityDeclarationHandlerDoesNotRevealAnotherAccount(t *testing.T) {
	repo := &declarationHTTPRepository{rows: make(map[string]verification.DeclarationRevision)}
	handler := declarationHandlerFixture(t, repo)
	body := `{"first_name":"Synthetic","last_name":"Person","date_of_birth":"1990-01-02","rut":"11222333-9","id_country":"CHL","document_type":"ID","consent_given":true,"consent_version":"consent-v1"}`
	post := httptest.NewRequest(http.MethodPost, "/api/v1/verifications/identity-declaration", strings.NewReader(body))
	post.Header.Set("Authorization", "Bearer owner")
	post.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(httptest.NewRecorder(), post)

	get := httptest.NewRequest(http.MethodGet, "/api/v1/verifications/identity-declaration", nil)
	get.Header.Set("Authorization", "Bearer other")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, get)
	if response.Code != http.StatusNotFound || strings.Contains(response.Body.String(), "Synthetic") {
		t.Fatalf("cross-account GET status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestIdentityDeclarationRoutesRequireAuthenticatedHandlerConfiguration(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/v1/verifications/identity-declaration", nil)
	response := httptest.NewRecorder()
	NewHandler(testAuth{}, nil, nil).ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated GET status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestIdentityDeclarationRouteFailsClosedWhenServiceIsNotConfigured(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		t.Run(method, func(t *testing.T) {
			body := ""
			if method == http.MethodPost {
				body = `{"first_name":"synthetic"}`
			}
			request := httptest.NewRequest(method, "/api/v1/verifications/identity-declaration", strings.NewReader(body))
			request.Header.Set("Authorization", "Bearer owner")
			response := httptest.NewRecorder()
			NewHandler(testAuth{}, nil, nil).ServeHTTP(response, request)

			if response.Code != http.StatusServiceUnavailable {
				t.Fatalf("authenticated %s status=%d body=%s, want service unavailable", method, response.Code, response.Body.String())
			}
			if response.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("Cache-Control=%q, want no-store", response.Header().Get("Cache-Control"))
			}
			if strings.Contains(response.Body.String(), "synthetic") {
				t.Fatalf("unconfigured handler reflected request data: %s", response.Body.String())
			}
		})
	}
}

func TestExternalProviderRoutesAreUnavailableInOwnIdentityRuntime(t *testing.T) {
	handlers := []struct {
		name    string
		handler http.Handler
	}{
		{"base handler", NewHandler(testAuth{}, nil, nil)},
		{"default declaration handler with legacy provider argument", NewHandlerWithDeclaration(testAuth{}, nil, nil, nil, nil, &verification.DiditService{})},
		{"provider-disabled declaration handler", NewHandlerWithDeclarationWithoutExternalProvider(testAuth{}, nil, nil, nil, nil)},
	}
	cases := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/v1/verifications/didit/intent"},
		{http.MethodGet, "/api/v1/verifications/didit/intent"},
		{http.MethodOptions, "/api/v1/verifications/didit/intent"},
		{http.MethodGet, "/api/v1/verifications/didit/intent/123e4567-e89b-42d3-a456-426614174000"},
		{http.MethodGet, "/api/v1/verifications/didit/intent/123e4567-e89b-42d3-a456-426614174000/refresh"},
		{http.MethodOptions, "/api/v1/verifications/didit/intent/123e4567-e89b-42d3-a456-426614174000/refresh"},
		{http.MethodPost, "/api/v1/verifications/didit/webhook"},
		{http.MethodOptions, "/api/v1/verifications/didit/webhook"},
		{http.MethodPost, "/api/v1/webhooks/didit"},
		{http.MethodOptions, "/api/v1/webhooks/didit"},
	}
	for _, configuredHandler := range handlers {
		for _, tc := range cases {
			t.Run(configuredHandler.name+"/"+tc.method+" "+tc.path, func(t *testing.T) {
				request := httptest.NewRequest(tc.method, tc.path, strings.NewReader(`{"synthetic":true}`))
				request.Header.Set("Authorization", "Bearer synthetic")
				response := httptest.NewRecorder()
				configuredHandler.handler.ServeHTTP(response, request)
				if response.Code != http.StatusNotFound {
					t.Fatalf("status=%d body=%s, want not found", response.Code, response.Body.String())
				}
			})
		}
	}
}

var _ identity.Principal
