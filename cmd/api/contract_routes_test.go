package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestContractRoutesRemainNarrowAndPreserveBookingRoutes(t *testing.T) {
	mux := http.NewServeMux()
	mux.Handle("/api/v1/local/booking-trial/", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	registerContractRoutes(mux, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusAccepted) }))
	for _, tc := range []struct {
		method, path string
		want         int
	}{
		{http.MethodPost, "/api/v1/local/booking-trial/reservations/00000000-0000-4000-8000-000000000001/contract", http.StatusAccepted},
		{http.MethodGet, "/api/v1/local/booking-trial/contracts/00000000-0000-4000-8000-000000000002", http.StatusAccepted},
		{http.MethodPost, "/api/v1/local/booking-trial/contracts/00000000-0000-4000-8000-000000000002/sign", http.StatusAccepted},
		{http.MethodPost, "/api/v1/local/booking-trial/contracts/00000000-0000-4000-8000-000000000002/reject", http.StatusAccepted},
		{http.MethodGet, "/api/v1/local/booking-trial/contracts/00000000-0000-4000-8000-000000000002/document", http.StatusAccepted},
		{http.MethodPost, "/api/v1/local/booking-trial/reservations/00000000-0000-4000-8000-000000000001/decision", http.StatusNoContent},
	} {
		request := httptest.NewRequest(tc.method, tc.path, nil)
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, request)
		if response.Code != tc.want {
			t.Fatalf("%s %s status=%d want=%d", tc.method, tc.path, response.Code, tc.want)
		}
	}
}
