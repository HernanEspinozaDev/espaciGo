package health

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type pingFunc func(context.Context) error

func (f pingFunc) Ping(ctx context.Context) error { return f(ctx) }

func TestLivenessDoesNotDependOnDatabase(t *testing.T) {
	calls := 0
	handler := NewHandler(pingFunc(func(context.Context) error {
		calls++
		return errors.New("database should not be checked for liveness")
	}), nil)

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/health/live", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if calls != 0 {
		t.Fatalf("database ping calls = %d, want 0", calls)
	}
	if got := response.Body.String(); got != "{\"status\":\"live\"}\n" {
		t.Fatalf("body = %q, want generic liveness JSON", got)
	}
}

func TestReadinessReportsUnavailableWithoutLeakingDatabaseError(t *testing.T) {
	secretLikeError := "connection refused at postgres://synthetic-secret@database"
	handler := NewHandler(pingFunc(func(context.Context) error {
		return errors.New(secretLikeError)
	}), nil)
	request := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusServiceUnavailable)
	}
	if strings.Contains(response.Body.String(), secretLikeError) || strings.Contains(response.Body.String(), "synthetic-secret") {
		t.Fatalf("response leaked database failure details: %q", response.Body.String())
	}
	if got := response.Body.String(); got != "{\"status\":\"unavailable\"}\n" {
		t.Fatalf("body = %q, want generic unavailable JSON", got)
	}
}

func TestReadinessReturnsReadyWhenDatabasePingSucceeds(t *testing.T) {
	calls := 0
	handler := NewHandler(pingFunc(func(context.Context) error {
		calls++
		return nil
	}), nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/health/ready", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if calls != 1 {
		t.Fatalf("database ping calls = %d, want 1", calls)
	}
	if got := response.Body.String(); got != "{\"status\":\"ready\"}\n" {
		t.Fatalf("body = %q, want generic readiness JSON", got)
	}
}

func TestReadinessCORSAllowsOnlyConfiguredMockOrigin(t *testing.T) {
	allowedOrigin := "http://localhost:8081"
	handler := NewHandler(pingFunc(func(context.Context) error { return nil }), []string{allowedOrigin})

	allowed := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	request.Header.Set("Origin", allowedOrigin)
	handler.ServeHTTP(allowed, request)
	if got := allowed.Header().Get("Access-Control-Allow-Origin"); got != allowedOrigin {
		t.Fatalf("allowed origin header = %q, want %q", got, allowedOrigin)
	}
	if got := allowed.Header().Get("Vary"); got != "Origin" {
		t.Fatalf("Vary = %q, want Origin", got)
	}

	denied := httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	request.Header.Set("Origin", "https://unexpected.example")
	handler.ServeHTTP(denied, request)
	if got := denied.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("unexpected origin received Access-Control-Allow-Origin %q", got)
	}
}

func TestReadinessAllowsCorsPreflightFromMock(t *testing.T) {
	allowedOrigin := "http://localhost:8081"
	handler := NewHandler(pingFunc(func(context.Context) error { return nil }), []string{allowedOrigin})
	request := httptest.NewRequest(http.MethodOptions, "/health/ready", nil)
	request.Header.Set("Origin", allowedOrigin)
	request.Header.Set("Access-Control-Request-Method", http.MethodGet)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
	}
	if got := response.Header().Get("Access-Control-Allow-Origin"); got != allowedOrigin {
		t.Fatalf("Access-Control-Allow-Origin = %q, want %q", got, allowedOrigin)
	}
	if got := response.Header().Get("Access-Control-Allow-Methods"); got != http.MethodGet {
		t.Fatalf("Access-Control-Allow-Methods = %q, want GET", got)
	}
}
