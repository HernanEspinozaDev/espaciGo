package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSpacesOrder2RouteAllowlist(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	handler := restrictSpacesOrder2Routes(next)
	spaceID := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	blockID := "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	cases := []struct {
		method string
		path   string
		want   int
	}{
		{"GET", "/health/live", http.StatusNoContent},
		{"GET", "/health/ready", http.StatusNoContent},
		{"GET", "/openapi.yaml", http.StatusNoContent},
		{"GET", "/api/v1/auth/terms", http.StatusNoContent},
		{"POST", "/api/v1/auth/login", http.StatusNoContent},
		{"GET", "/api/v1/auth/session", http.StatusNoContent},
		{"POST", "/api/v1/auth/logout", http.StatusNoContent},
		{"GET", "/api/v1/spaces", http.StatusNoContent},
		{"GET", "/api/v1/spaces/" + spaceID, http.StatusNoContent}, // Editor selection loads the owner draft and ETag used by order-2 settings edits.
		{"GET", "/api/v1/spaces/" + spaceID + "/availability", http.StatusNoContent},
		{"PUT", "/api/v1/spaces/" + spaceID + "/availability", http.StatusNoContent},
		{"GET", "/api/v1/spaces/" + spaceID + "/availability/timezone", http.StatusNoContent},
		{"GET", "/api/v1/spaces/" + spaceID + "/weekly-hours", http.StatusNoContent},
		{"PUT", "/api/v1/spaces/" + spaceID + "/weekly-hours", http.StatusNoContent},
		{"GET", "/api/v1/spaces/" + spaceID + "/availability/blocks", http.StatusNoContent},
		{"POST", "/api/v1/spaces/" + spaceID + "/availability/blocks", http.StatusNoContent},
		{"DELETE", "/api/v1/spaces/" + spaceID + "/availability/blocks/" + blockID, http.StatusNoContent},
		{"POST", "/api/v1/spaces", http.StatusNotFound},
		{"POST", "/api/v1/auth/register", http.StatusNotFound},
		{"GET", "/api/v1/public/spaces", http.StatusNotFound},
		{"GET", "/api/v1/admin/global/", http.StatusNotFound},
		{"POST", "/api/v1/spaces/" + spaceID + "/gallery", http.StatusNotFound},
		{"DELETE", "/api/v1/spaces/" + spaceID + "/availability/blocks/not-a-uuid", http.StatusNotFound},
		{"GET", "/api/v1/spaces/not-a-uuid/availability", http.StatusNotFound},
		{"GET", "/api/v1/spaces/" + spaceID + "/availability/blocks/" + blockID, http.StatusNotFound},
		{"OPTIONS", "/health/live", http.StatusNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			res := httptest.NewRecorder()
			handler.ServeHTTP(res, httptest.NewRequest(tc.method, tc.path, nil))
			if res.Code != tc.want {
				t.Fatalf("status=%d, want %d", res.Code, tc.want)
			}
		})
	}
}

func TestRuntimeProfileHandlerWiresOrder2RouteRestriction(t *testing.T) {
	mux := http.NewServeMux()
	ready := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.Handle("GET /api/v1/spaces/{spaceID}", ready)
	mux.Handle("GET /api/v1/spaces/{spaceID}/weekly-hours", ready)
	mux.Handle("POST /api/v1/spaces", ready)
	mux.Handle("GET /api/v1/admin/global/", ready)
	handler := handlerForRuntimeProfile(runtimeProfileSpacesOrder2, mux)
	for _, tc := range []struct {
		method string
		path   string
		want   int
	}{
		{http.MethodGet, "/api/v1/spaces/aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", http.StatusNoContent},
		{http.MethodGet, "/api/v1/spaces/aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa/weekly-hours", http.StatusNoContent},
		{http.MethodPost, "/api/v1/spaces", http.StatusNotFound},
		{http.MethodGet, "/api/v1/admin/global/", http.StatusNotFound},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			res := httptest.NewRecorder()
			handler.ServeHTTP(res, httptest.NewRequest(tc.method, tc.path, nil))
			if res.Code != tc.want {
				t.Fatalf("status=%d, want %d", res.Code, tc.want)
			}
		})
	}
}
