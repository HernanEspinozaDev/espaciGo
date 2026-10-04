package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCheckEndpointAcceptsHealthyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	if err := checkEndpoint(server.URL); err != nil {
		t.Fatalf("checkEndpoint returned error for HTTP 200: %v", err)
	}
}

func TestCheckEndpointRejectsUnavailableResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	if err := checkEndpoint(server.URL); err == nil {
		t.Fatal("checkEndpoint succeeded for HTTP 503")
	}
}
