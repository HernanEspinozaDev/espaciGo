package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestManualKYCOnboardingRoutesRemainUnavailable(t *testing.T) {
	var reached []string
	auth := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = append(reached, r.Method+" "+r.URL.Path)
		w.WriteHeader(http.StatusNoContent)
	})
	verification := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	health := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	routes := []string{"/api/v1/auth/register", "/api/v1/auth/verification", "/api/v1/auth/verification/status", "/api/v1/auth/verification/reissue"}
	mux := newLocalKYCMux(auth, verification, health)
	for _, path := range routes {
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, path, nil))
		if rr.Code != http.StatusNotFound {
			t.Errorf("POST %s = %d, want 404", path, rr.Code)
		}
	}
	if len(reached) != 0 {
		t.Fatalf("auth handler reached for retired onboarding routes: %v", reached)
	}
}
