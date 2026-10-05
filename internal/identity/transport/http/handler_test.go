package identityhttp

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTransportRejectsUnsafeRequestsBeforeCallingService(t *testing.T) {
	handler := NewHandler(nil, nil, []string{"http://localhost:8081"})
	cases := []struct {
		method, path, body, content, origin string
		status                              int
	}{
		{"GET", "/api/v1/auth/session", "", "", "", 401},
		{"POST", "/api/v1/auth/login", "{broken", "application/json", "", 400},
		{"POST", "/api/v1/auth/login", "{} {}", "application/json", "", 400},
		{"POST", "/api/v1/auth/login", `{"role":"administrador"}`, "application/json", "", 400},
		{"POST", "/api/v1/auth/login", "{}", "text/plain", "", 415},
		{"GET", "/api/v1/auth/session?access_token=synthetic", "", "", "", 400},
		{"GET", "/api/v1/auth/session", "", "", "http://other.invalid", 403},
		{"DELETE", "/api/v1/auth/register", "", "", "", 405},
	}
	for _, test := range cases {
		r := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
		r.Header.Set("Content-Type", test.content)
		r.Header.Set("Origin", test.origin)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != test.status {
			t.Fatalf("%s: status %d want %d", test.path, w.Code, test.status)
		}
		if !strings.Contains(w.Body.String(), `"request_id"`) || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("missing safe error envelope")
		}
		if test.status == 401 && w.Header().Get("WWW-Authenticate") != "Bearer" {
			t.Fatal("missing Bearer challenge")
		}
	}
	r := httptest.NewRequest("OPTIONS", "/api/v1/auth/login", nil)
	r.Header.Set("Origin", "http://localhost:8081")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != 204 || !strings.Contains(w.Header().Get("Access-Control-Allow-Headers"), "Authorization") {
		t.Fatal("CORS preflight failed")
	}
}
