package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/HernanEspinozaDev/espaciGo/internal/verification"
)

func TestFakeProviderSessionLifecycleIsLocalAndFailClosed(t *testing.T) {
	server := httptest.NewServer(NewHandler())
	defer server.Close()

	body := `{"workflow_id":"local-kyc-test-workflow","vendor_data":"11111111-1111-4111-8111-111111111111","expected_details":{"first_name":"Persona","last_name":"Sintetica","date_of_birth":"1990-01-01","id_country":"CHL","document_type":"ID","identification_number":"12345670-K","expected_document_types":["ID"]}}`
	create := httptest.NewRequest(http.MethodPost, "/v3/session/", strings.NewReader(body))
	create.Header.Set("Content-Type", "application/json")
	create.Header.Set("x-api-key", fakeAPIKey)
	created := httptest.NewRecorder()
	server.Config.Handler.ServeHTTP(created, create)
	if created.Code != http.StatusCreated {
		t.Fatalf("create status=%d, want %d", created.Code, http.StatusCreated)
	}
	var response struct {
		SessionID string `json:"session_id"`
		URL       string `json:"url"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.SessionID == "" || response.URL != "https://verify.didit.me/session/"+response.SessionID {
		t.Fatalf("unexpected synthetic session response: session id present=%t, URL host/path valid=%t", response.SessionID != "", strings.HasPrefix(response.URL, "https://verify.didit.me/session/"))
	}

	decisionRequest := httptest.NewRequest(http.MethodGet, "/v3/session/"+response.SessionID+"/decision/", nil)
	decisionRequest.Header.Set("x-api-key", fakeAPIKey)
	decision := httptest.NewRecorder()
	server.Config.Handler.ServeHTTP(decision, decisionRequest)
	if decision.Code != http.StatusOK || !strings.Contains(decision.Body.String(), `"status":"In Review"`) || strings.Contains(decision.Body.String(), "Persona") || strings.Contains(decision.Body.String(), "12345670-K") {
		t.Fatalf("fake decision was not safe fail-closed output: status=%d", decision.Code)
	}

	deleteRequest := httptest.NewRequest(http.MethodDelete, "/v3/session/"+response.SessionID+"/delete/", nil)
	deleteRequest.Header.Set("x-api-key", fakeAPIKey)
	deleted := httptest.NewRecorder()
	server.Config.Handler.ServeHTTP(deleted, deleteRequest)
	if deleted.Code != http.StatusNoContent {
		t.Fatalf("delete status=%d, want %d", deleted.Code, http.StatusNoContent)
	}
	missingRequest := httptest.NewRequest(http.MethodGet, "/v3/session/"+response.SessionID+"/decision/", nil)
	missingRequest.Header.Set("x-api-key", fakeAPIKey)
	missing := httptest.NewRecorder()
	server.Config.Handler.ServeHTTP(missing, missingRequest)
	if missing.Code != http.StatusNotFound {
		t.Fatalf("deleted session status=%d, want %d", missing.Code, http.StatusNotFound)
	}
}

func TestFakeProviderRejectsWrongKeyAndUnknownRoute(t *testing.T) {
	server := httptest.NewServer(NewHandler())
	defer server.Close()

	request := httptest.NewRequest(http.MethodGet, "/v3/session/11111111-1111-4111-8111-111111111111/decision/", nil)
	request.Header.Set("x-api-key", "not-the-local-fake-key")
	response := httptest.NewRecorder()
	server.Config.Handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("wrong-key status=%d, want %d", response.Code, http.StatusUnauthorized)
	}

	unknown := httptest.NewRequest(http.MethodGet, "/v3/other/", nil)
	unknown.Header.Set("x-api-key", fakeAPIKey)
	unknownResponse := httptest.NewRecorder()
	server.Config.Handler.ServeHTTP(unknownResponse, unknown)
	if unknownResponse.Code != http.StatusNotFound {
		t.Fatalf("unknown-route status=%d, want %d", unknownResponse.Code, http.StatusNotFound)
	}
	if strings.Contains(unknownResponse.Body.String(), "http://") || strings.Contains(unknownResponse.Body.String(), "https://") {
		t.Fatal("fake provider returned external-routing data")
	}
}

func TestFakeProviderStatsExposeOnlyAuthenticatedAggregateCounters(t *testing.T) {
	handler := NewHandler()
	request := func(method, path, key, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		if key != "" {
			r.Header.Set("x-api-key", key)
		}
		if method == http.MethodPost {
			r.Header.Set("Content-Type", "application/json")
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	assertStats := func(w *httptest.ResponseRecorder, requests, active int) {
		t.Helper()
		if w.Code != http.StatusOK {
			t.Fatalf("stats status=%d, want 200; body=%s", w.Code, w.Body.String())
		}
		var got map[string]json.RawMessage
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		var counts struct {
			CreationRequests int `json:"creation_requests"`
			ActiveSessions   int `json:"active_sessions"`
		}
		if len(got) != 2 || json.Unmarshal(w.Body.Bytes(), &counts) != nil || counts.CreationRequests != requests || counts.ActiveSessions != active {
			t.Fatalf("stats=%s, want exactly integer creation_requests=%d and active_sessions=%d", w.Body.String(), requests, active)
		}
		if strings.Contains(w.Body.String(), "session_id") || strings.Contains(w.Body.String(), "vendor_data") || strings.Contains(w.Body.String(), "Persona") {
			t.Fatalf("stats leaked session data: %s", w.Body.String())
		}
		if w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Fatalf("missing security headers: %v", w.Header())
		}
	}
	assertStats(request(http.MethodGet, "/__local_test/stats", fakeAPIKey, ""), 0, 0)
	if got := request(http.MethodPost, "/v3/session/", "incorrect", "{").Code; got != http.StatusUnauthorized {
		t.Fatalf("wrong-key create status=%d, want 401", got)
	}
	if got := request(http.MethodPost, "/unknown", fakeAPIKey, "{}").Code; got != http.StatusNotFound {
		t.Fatalf("unknown create route status=%d, want 404", got)
	}
	assertStats(request(http.MethodGet, "/__local_test/stats", fakeAPIKey, ""), 0, 0)
	for _, key := range []string{"", "incorrect"} {
		if got := request(http.MethodGet, "/__local_test/stats", key, "").Code; got != http.StatusUnauthorized {
			t.Fatalf("stats key status=%d, want 401", got)
		}
	}
	assertStats(request(http.MethodGet, "/__local_test/stats", fakeAPIKey, ""), 0, 0)
	if got := request(http.MethodPost, "/v3/session/", fakeAPIKey, "{").Code; got != http.StatusBadRequest {
		t.Fatalf("malformed create status=%d, want 400", got)
	}
	assertStats(request(http.MethodGet, "/__local_test/stats", fakeAPIKey, ""), 1, 0)
	body := `{"workflow_id":"local-kyc-test-workflow","vendor_data":"11111111-1111-4111-8111-111111111111","expected_details":{"first_name":"Persona","last_name":"Sintetica","date_of_birth":"1990-01-01","id_country":"CHL","document_type":"ID","identification_number":"12345670-K","expected_document_types":["ID"]}}`
	created := request(http.MethodPost, "/v3/session/", fakeAPIKey, body)
	if created.Code != http.StatusCreated {
		t.Fatalf("valid create status=%d, want 201", created.Code)
	}
	var session struct {
		SessionID string `json:"session_id"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &session); err != nil || session.SessionID == "" {
		t.Fatalf("invalid create response: err=%v", err)
	}
	assertStats(request(http.MethodGet, "/__local_test/stats", fakeAPIKey, ""), 2, 1)
	if got := request(http.MethodDelete, "/v3/session/"+session.SessionID+"/delete/", fakeAPIKey, "").Code; got != http.StatusNoContent {
		t.Fatalf("delete status=%d, want 204", got)
	}
	assertStats(request(http.MethodGet, "/__local_test/stats", fakeAPIKey, ""), 2, 0)
	if got := request(http.MethodGet, "/__local_test/stats?x=1", fakeAPIKey, "").Code; got != http.StatusBadRequest {
		t.Fatalf("query stats status=%d, want 400", got)
	}
	if got := request(http.MethodPost, "/__local_test/stats", fakeAPIKey, "").Code; got != http.StatusNotFound {
		t.Fatalf("unsupported stats method status=%d, want 404", got)
	}
	if got := request(http.MethodGet, "/unknown", fakeAPIKey, "").Code; got != http.StatusNotFound {
		t.Fatalf("unknown route status=%d, want 404", got)
	}
}

func TestSyntheticApprovalOnlyApprovesExactFixture(t *testing.T) {
	handler := NewSyntheticApprovalHandler()
	request := func(method, path, key, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("x-api-key", key)
		if method == http.MethodPost {
			r.Header.Set("Content-Type", "application/json")
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	body := `{"workflow_id":"local-kyc-test-workflow","vendor_data":"corr","expected_details":{"first_name":"Synthetic","last_name":"Person","date_of_birth":"1990-01-02","id_country":"CHL","document_type":"ID","identification_number":"11.222.333-9"}}`
	created := request(http.MethodPost, "/v3/session/", fakeAPIKey, body)
	if created.Code != http.StatusCreated {
		t.Fatalf("fixture create status=%d body=%s", created.Code, created.Body.String())
	}
	var session struct {
		SessionID string `json:"session_id"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &session); err != nil {
		t.Fatal(err)
	}
	decision := request(http.MethodGet, "/v3/session/"+session.SessionID+"/decision/", fakeAPIKey, "")
	var got map[string]any
	if decision.Code != http.StatusOK || json.Unmarshal(decision.Body.Bytes(), &got) != nil {
		t.Fatalf("decision status=%d body=%s", decision.Code, decision.Body.String())
	}
	if got["status"] != "Approved" || got["session_id"] != session.SessionID || got["workflow_id"] != fakeWorkflowID || got["vendor_data"] != "corr" {
		t.Fatalf("approval bindings/status mismatch: %s", decision.Body.String())
	}
	for _, key := range []string{"id_verifications", "liveness_checks", "face_matches"} {
		if len(got[key].([]any)) != 1 {
			t.Fatalf("missing %s: %s", key, decision.Body.String())
		}
	}
	var authoritative verification.DiditDecisionResponse
	if err := json.Unmarshal(decision.Body.Bytes(), &authoritative); err != nil {
		t.Fatal(err)
	}
	status, flags, warnings := verification.EvaluateDecision(&verification.IdentityVerificationAttempt{
		IDCountry: "CHL", DocumentType: "ID", DeclaredFirstName: "Synthetic",
		DeclaredLastName: "Person", DeclaredDateOfBirth: "1990-01-02",
		DeclaredRUTNormalized: "11222333-9", DiditSessionID: session.SessionID,
		WorkflowID: fakeWorkflowID,
	}, &authoritative)
	if status != "APPROVED" || len(flags) != 0 || len(warnings) != 0 {
		t.Fatalf("synthetic authoritative response did not evaluate approved: status=%s", status)
	}
	for _, bad := range []string{strings.Replace(body, "Synthetic", "Other", 1), strings.Replace(body, "11.222.333-9", "11222334-7", 1)} {
		if got := request(http.MethodPost, "/v3/session/", fakeAPIKey, bad).Code; got != http.StatusBadRequest {
			t.Fatalf("nonfixture create status=%d, want 400", got)
		}
	}
	if got := request(http.MethodPost, "/v3/session/", "wrong", body).Code; got != http.StatusUnauthorized {
		t.Fatalf("unauthorized status=%d", got)
	}
	stats := request(http.MethodGet, "/__local_test/stats", fakeAPIKey, "")
	if stats.Body.String() != `{"creation_requests":3,"active_sessions":1}`+"\n" {
		t.Fatalf("stats changed shape/count: %s", stats.Body.String())
	}
}
