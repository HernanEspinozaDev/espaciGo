package identityhttp

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/devauth"
)

func TestInternalClientIPSeparatesQuotaAndIgnoresUntrustedForwardedFor(t *testing.T) {
	limiter := devauth.NewIPLimiter(1)
	at := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	allow := func(ip string) bool {
		req := httptest.NewRequest("POST", "/api/v1/auth/verification/reissue", strings.NewReader("{}"))
		req.RemoteAddr = "10.0.0.9:8080"
		if ip != "" {
			req.Header.Set("X-Espacigo-Client-IP", ip)
		}
		// This is untrusted at the API boundary and must not affect the quota key.
		req.Header.Set("X-Forwarded-For", "198.51.100.77")
		clientIP, err := clientIPFromRequest(req)
		if err != nil {
			t.Fatalf("derive client IP: %v", err)
		}
		allowed, err := limiter.AllowVerification(context.Background(), clientIP, at)
		if err != nil {
			t.Fatalf("apply quota: %v", err)
		}
		return allowed
	}
	if !allow("203.0.113.10") || !allow("203.0.113.11") {
		t.Fatal("distinct visitor IPs unexpectedly shared the rate-limit quota")
	}
	if allow("203.0.113.10") {
		t.Fatal("repeated request from one visitor bypassed its rate-limit quota")
	}
	// Without the internal SSR context, a forged XFF value cannot change the
	// direct peer key or create a fresh quota bucket.
	if !allow("") || allow("") {
		t.Fatal("untrusted X-Forwarded-For changed the direct peer quota")
	}
}

func TestTransportRejectsUnsafeRequestsBeforeCallingService(t *testing.T) {
	handler := NewHandler(nil, nil, []string{"http://localhost:4200"})
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
	r.Header.Set("Origin", "http://localhost:4200")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != 204 || !strings.Contains(w.Header().Get("Access-Control-Allow-Headers"), "Authorization") {
		t.Fatal("CORS preflight failed")
	}
}

func TestAccountInputErrorsAreSafeAndActionableBeforeServiceLookup(t *testing.T) {
	handler := NewHandler(nil, nil, nil)
	for _, test := range []struct{ path, body, field string }{
		{"login", `{"email":"invalid-email","password":""}`, `"field":"email"`},
		{"register", `{"email":"fixture@example.invalid","password":"weak","terms_version_ids":[]}`, `"field":"terms_version_ids"`},
	} {
		req := httptest.NewRequest("POST", "/api/v1/auth/"+test.path, strings.NewReader(test.body))
		req.Header.Set("Content-Type", "application/json")
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		if res.Code != 422 || !strings.Contains(res.Body.String(), test.field) || strings.Contains(res.Body.String(), "fixture@example.invalid") || strings.Contains(res.Body.String(), "weak") {
			t.Fatalf("unsafe or missing validation: status %d body %s", res.Code, res.Body.String())
		}
	}
}
