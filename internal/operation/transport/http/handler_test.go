package operationhttp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
)

type testAuthenticator struct{ deny bool }

func (a testAuthenticator) Authorize(context.Context, identity.Secret, identity.Role, identity.ActivityKind) (identity.Principal, error) {
	if a.deny {
		return identity.Principal{}, identity.ErrUnauthorized
	}
	return identity.Principal{AccountID: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"}, nil
}

func TestRecordRejectsClientSelectedKindAndUsesCommonErrorHeaders(t *testing.T) {
	handler := NewHandler(testAuthenticator{}, nil, []string{"http://localhost:8081"})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/local/booking-trial/reservations/aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa/check-in", strings.NewReader(`{"kind":"checkout","comments":"x"}`))
	request.Header.Set("Authorization", "Bearer test")
	request.Header.Set("Origin", "http://localhost:8081")
	request.Header.Set("Idempotency-Key", "strict-kind-key")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if response.Header().Get("Access-Control-Allow-Origin") != "http://localhost:8081" {
		t.Fatalf("CORS header=%q", response.Header().Get("Access-Control-Allow-Origin"))
	}
	if response.Header().Get("X-Request-ID") == "" || !strings.Contains(response.Body.String(), response.Header().Get("X-Request-ID")) {
		t.Fatalf("request ID header/body mismatch: %v %s", response.Header(), response.Body.String())
	}
}

func TestUnauthorizedOperationReturnsBearerChallenge(t *testing.T) {
	handler := NewHandler(testAuthenticator{deny: true}, nil, nil)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/local/booking-trial/reservations/aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa/operations", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized || response.Header().Get("WWW-Authenticate") != "Bearer" {
		t.Fatalf("status=%d challenge=%q", response.Code, response.Header().Get("WWW-Authenticate"))
	}
	if response.Header().Get("X-Request-ID") == "" || !strings.Contains(response.Body.String(), response.Header().Get("X-Request-ID")) {
		t.Fatalf("request ID missing/mismatched: %v %s", response.Header(), response.Body.String())
	}
}
