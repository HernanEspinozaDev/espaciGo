package mockserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRootServesStaticMockPage(t *testing.T) {
	handler := NewHandler("http://localhost:8080/health/ready")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if got := response.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
		t.Fatalf("Content-Type = %q, want text/html", got)
	}
}

func TestConfigEndpointReturnsConfiguredAPIReadyURL(t *testing.T) {
	wantURL := "http://localhost:8080/health/ready"
	handler := NewHandler(wantURL)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/config.json", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	var config struct {
		APIReadyURL string `json:"apiReadyURL"`
	}
	if err := json.NewDecoder(response.Body).Decode(&config); err != nil {
		t.Fatal(err)
	}
	if config.APIReadyURL != wantURL {
		t.Fatalf("apiReadyURL = %q, want %q", config.APIReadyURL, wantURL)
	}
}

func TestMockHealthEndpointIsAvailable(t *testing.T) {
	handler := NewHandler("http://localhost:8080/health/ready")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
}
