package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOperationRoutesDoNotInterceptBookingRoutes(t *testing.T) {
	mux := http.NewServeMux()
	booking := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusAccepted) })
	operations := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusCreated) })
	mux.Handle("/api/v1/local/booking-trial/", booking)
	registerOperationRoutes(mux, operations)
	registerDamageClaimRoutes(mux, operations)
	tests := []struct {
		method, path string
		want         int
	}{
		{http.MethodPost, "/api/v1/local/booking-trial/reservations/11111111-1111-4111-8111-111111111111/check-in", http.StatusCreated},
		{http.MethodPost, "/api/v1/local/booking-trial/reservations/11111111-1111-4111-8111-111111111111/check-out", http.StatusCreated},
		{http.MethodPost, "/api/v1/local/booking-trial/reservations/11111111-1111-4111-8111-111111111111/reception", http.StatusCreated},
		{http.MethodGet, "/api/v1/local/booking-trial/reservations/11111111-1111-4111-8111-111111111111/operations", http.StatusCreated},
		{http.MethodPost, "/api/v1/local/booking-trial/reservations/11111111-1111-4111-8111-111111111111/damage-claim", http.StatusCreated},
		{http.MethodPost, "/api/v1/local/booking-trial/reservations/11111111-1111-4111-8111-111111111111/damage-claim/defense", http.StatusCreated},
		{http.MethodGet, "/api/v1/local/booking-trial/reservations/11111111-1111-4111-8111-111111111111", http.StatusAccepted},
	}
	for _, test := range tests {
		t.Run(test.method+" "+test.path, func(t *testing.T) {
			response := httptest.NewRecorder()
			mux.ServeHTTP(response, httptest.NewRequest(test.method, test.path, nil))
			if response.Code != test.want {
				t.Fatalf("status=%d, want %d", response.Code, test.want)
			}
		})
	}
}
