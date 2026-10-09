package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

type routeName string

func (n routeName) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
	_, _ = w.Write([]byte(n))
}

func TestReputationRoutesDoNotInterceptBookingOperations(t *testing.T) {
	mux := http.NewServeMux()
	booking := routeName("booking")
	reviews := routeName("reputation")
	mux.Handle("/api/v1/local/booking-trial/", booking)
	mux.Handle("/api/v1/local/booking-trial/reservations/", booking)
	registerReputationReservationRoutes(mux, reviews)

	cases := []struct{ method, path, want string }{
		{http.MethodGet, "/api/v1/local/booking-trial/reservations/11111111-1111-4111-8111-111111111111", "booking"},
		{http.MethodPost, "/api/v1/local/booking-trial/reservations/11111111-1111-4111-8111-111111111111/payment", "booking"},
		{http.MethodGet, "/api/v1/local/booking-trial/reservations/11111111-1111-4111-8111-111111111111/reviews", "reputation"},
		{http.MethodPost, "/api/v1/local/booking-trial/reservations/11111111-1111-4111-8111-111111111111/reviews", "reputation"},
		{http.MethodPost, "/api/v1/local/booking-trial/reservations/11111111-1111-4111-8111-111111111111/reviews/22222222-2222-4222-8222-222222222222/report", "reputation"},
	}
	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, nil))
			if got := rec.Body.String(); got != tc.want {
				t.Fatalf("handler=%q, want %q", got, tc.want)
			}
		})
	}
}
