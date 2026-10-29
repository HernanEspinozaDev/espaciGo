package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOwnerWeeklyHoursRoutesOverrideSpacesPrefix(t *testing.T) {
	mux := http.NewServeMux()
	mux.Handle("/api/v1/spaces/", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))
	ownerHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
		_, _ = w.Write([]byte(r.Method + ":" + r.PathValue("spaceID")))
	})
	registerOwnerWeeklyHoursRoutes(mux, ownerHandler)

	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodOptions} {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/v1/spaces/aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa/weekly-hours", nil)
			res := httptest.NewRecorder()
			mux.ServeHTTP(res, req)
			if res.Code != http.StatusNoContent || res.Body.String() != method+":aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa" {
				t.Fatalf("status=%d body=%q", res.Code, res.Body.String())
			}
		})
	}

	res := httptest.NewRecorder()
	mux.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/v1/spaces/aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", nil))
	if res.Code != http.StatusTeapot {
		t.Fatalf("unrelated spaces route status=%d", res.Code)
	}
}
