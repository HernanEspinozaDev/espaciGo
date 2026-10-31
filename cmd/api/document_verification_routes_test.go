package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRegisterDocumentVerificationRoutesMountsOwnerDownload(t *testing.T) {
	mux := http.NewServeMux()
	download := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(r.PathValue("caseID") + "/" + r.PathValue("fileID")))
	})
	registerDocumentVerificationRoutes(mux, nil, download)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/document-verification/space-cases/99999999-9999-4999-8999-999999999999/files/aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa/content", nil)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK || response.Body.String() != "99999999-9999-4999-8999-999999999999/aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa" {
		t.Fatalf("download route response=%d body=%q", response.Code, response.Body.String())
	}
}

func TestDocumentVerificationProfileAllowsOnlyOwnerDownloadRouteShape(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	handler := restrictDocumentVerificationRoutes(next)
	path := "/api/v1/document-verification/space-cases/99999999-9999-4999-8999-999999999999/files/aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa/content"
	for _, tc := range []struct {
		name   string
		method string
		path   string
		want   int
	}{
		{name: "owner download", method: http.MethodGet, path: path, want: http.StatusNoContent},
		{name: "owner download head denied", method: http.MethodHead, path: path, want: http.StatusNotFound},
		{name: "owner download malformed case id", method: http.MethodGet, path: "/api/v1/document-verification/space-cases/case/files/aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa/content", want: http.StatusNotFound},
		{name: "owner download malformed file id", method: http.MethodGet, path: "/api/v1/document-verification/space-cases/99999999-9999-4999-8999-999999999999/files/file/content", want: http.StatusNotFound},
		{name: "wrong method", method: http.MethodPost, path: path, want: http.StatusNotFound},
		{name: "extra path segment", method: http.MethodGet, path: path + "/extra", want: http.StatusNotFound},
		{name: "trailing slash", method: http.MethodGet, path: path + "/", want: http.StatusNotFound},
		{name: "neighbor route", method: http.MethodGet, path: "/api/v1/document-verification/space-cases/case", want: http.StatusNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(tc.method, tc.path, nil)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != tc.want {
				t.Fatalf("status=%d want=%d", response.Code, tc.want)
			}
		})
	}
}
