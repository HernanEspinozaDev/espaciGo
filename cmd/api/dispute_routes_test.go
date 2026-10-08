package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAssembledRouterKeepsBookingRoutesAndDispatchesSpecificDisputeRoutes(t *testing.T) {
	mux := http.NewServeMux()
	var bookingPaths, disputePaths []string
	mux.Handle("/api/v1/local/booking-trial/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bookingPaths = append(bookingPaths, r.Method+" "+r.URL.Path)
		w.WriteHeader(http.StatusNoContent)
	}))
	registerDisputeRoutes(mux, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		disputePaths = append(disputePaths, r.Method+" "+r.URL.Path)
		w.WriteHeader(http.StatusAccepted)
	}))

	cases := []struct {
		method string
		path   string
		status int
	}{
		{http.MethodGet, "/api/v1/local/booking-trial/reservations/00000000-0000-4000-8000-000000000001", http.StatusNoContent},
		{http.MethodPost, "/api/v1/local/booking-trial/reservations/00000000-0000-4000-8000-000000000001/cancel", http.StatusNoContent},
		{http.MethodGet, "/api/v1/local/booking-trial/reservations/00000000-0000-4000-8000-000000000001/disputes", http.StatusAccepted},
		{http.MethodPost, "/api/v1/local/booking-trial/reservations/00000000-0000-4000-8000-000000000001/disputes", http.StatusAccepted},
		{http.MethodGet, "/api/v1/local/booking-trial/disputes/00000000-0000-4000-8000-000000000002/history", http.StatusAccepted},
		{http.MethodGet, "/api/v1/admin/disputes", http.StatusAccepted},
		{http.MethodPost, "/api/v1/admin/disputes/00000000-0000-4000-8000-000000000002/close", http.StatusAccepted},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, req)
		if response.Code != tc.status {
			t.Errorf("%s %s status=%d, want %d", tc.method, tc.path, response.Code, tc.status)
		}
	}
	if len(bookingPaths) != 2 || len(disputePaths) != 5 {
		t.Fatalf("router dispatched booking=%v dispute=%v", bookingPaths, disputePaths)
	}
}
