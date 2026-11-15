package verification

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Synthetic identity only: exercise the real HTTP JSON boundary, not OCR.
func TestDiditExpectedDetailsPreservesUTF8AndNationalIdentifier(t *testing.T) {
	expected := ExpectedDetails{FirstName: "Julián Andrés", LastName: "Muñoz Pérez", DateOfBirth: "1990-01-02", IDCountry: "CHL", DocumentType: "ID", IdentificationNumber: "12345670-K", ExpectedDocumentTypes: []string{"ID"}}
	var received CreateSessionRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v3/session/" {
			t.Error("unexpected request")
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Error("invalid JSON request")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"session_id":"synthetic-session","url":"https://verify.didit.me/session/synthetic"}`))
	}))
	defer server.Close()
	client := NewHTTPDiditClient("synthetic-key", "", true, time.Second)
	client.BaseURL = server.URL
	_, err := client.CreateSession(context.Background(), CreateSessionRequest{WorkflowID: "synthetic-workflow", ExpectedDetails: expected})
	if err != nil {
		t.Fatal(err)
	}
	if received.ExpectedDetails.FirstName != expected.FirstName || received.ExpectedDetails.LastName != expected.LastName || received.ExpectedDetails.DateOfBirth != expected.DateOfBirth || received.ExpectedDetails.IdentificationNumber != expected.IdentificationNumber {
		t.Fatal("HTTP serialization altered expected identity")
	}
	// Deliberately do not add document_number, MRZ, image or document serial fields.
}
