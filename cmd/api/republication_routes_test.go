package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLocalRepublicationRejectsNonWorkerFreeAndBookingMix(t *testing.T) {
	for _, tc := range []struct {
		profile       runtimeProfileName
		flag, booking string
		want          bool
	}{
		{runtimeProfileWorkerFree, "1", "", true},
		{runtimeProfileStandard, "1", "", false},
		{runtimeProfileWorkerFree, "1", "1", false},
		{runtimeProfileWorkerFree, "yes", "", false},
	} {
		env := map[string]string{"LOCAL_REPUBLICATION_TRIAL": tc.flag, "LOCAL_BOOKING_TRIAL": tc.booking}
		enabled, err := localRepublicationEnabled(func(k string) string { return env[k] }, tc.profile)
		if tc.want != (err == nil && enabled) {
			t.Fatalf("unexpected scope gate result for %s", tc.profile)
		}
	}
}
func TestLocalRepublicationRoutesDoNotEnableContractOrFinancialWrites(t *testing.T) {
	mux := http.NewServeMux()
	registerLocalRepublicationRoutes(mux, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	for _, tc := range []struct {
		method, path string
		status       int
	}{
		{"GET", "/api/v1/local/booking-trial/spaces/space/returns", 204},
		{"POST", "/api/v1/local/booking-trial/reservations/reservation/check-out", 204},
		{"POST", "/api/v1/local/booking-trial/reservations/reservation/reception", 204},
		{"POST", "/api/v1/local/booking-trial/reservations/reservation/check-in", 404},
		{"POST", "/api/v1/local/booking-trial/reservations/reservation/payment", 404},
		{"POST", "/api/v1/local/booking-trial/reservations/reservation/contract", 404},
	} {
		r := httptest.NewRecorder()
		mux.ServeHTTP(r, httptest.NewRequest(tc.method, tc.path, nil))
		if r.Code != tc.status {
			t.Fatal(tc.path, r.Code)
		}
	}
}
