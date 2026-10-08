package m02local

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
)

type m02AuthStub struct{}

func (m02AuthStub) Authorize(_ context.Context, _ identity.Secret, _ identity.Role, _ identity.ActivityKind) (identity.Principal, error) {
	return identity.Principal{AccountID: "11111111-1111-4111-8111-111111111111"}, nil
}

type m02AuthErrorStub struct{}

func (m02AuthErrorStub) Authorize(context.Context, identity.Secret, identity.Role, identity.ActivityKind) (identity.Principal, error) {
	return identity.Principal{}, errors.New("denied")
}

func TestM02PhotoEndpointRejectsClientProvidedImageBytes(t *testing.T) {
	h := NewHandler(m02AuthStub{}, nil, nil)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/profile/photo", strings.NewReader("arbitrary-upload"))
	req.Header.Set("Authorization", "Bearer local-test")
	req.Header.Set("Idempotency-Key", "photo-key-123")
	res := httptest.NewRecorder()
	h.ServeHTTP(res, req)
	if res.Code != http.StatusUnprocessableEntity || !strings.Contains(res.Body.String(), "validation_error") {
		t.Fatalf("client bytes accepted: status=%d body=%s", res.Code, res.Body.String())
	}
}

func TestM02PhotoAndPayoutErrorsUseCommonHTTPContract(t *testing.T) {
	tests := []struct {
		name       string
		auth       Authenticator
		path       string
		body       string
		wantStatus int
		wantCode   string
	}{
		{name: "photo unauthorized", auth: m02AuthErrorStub{}, path: "/api/v1/profile/photo", wantStatus: http.StatusUnauthorized, wantCode: "unauthenticated"},
		{name: "payout unauthorized", auth: m02AuthErrorStub{}, path: "/api/v1/payout-account", wantStatus: http.StatusUnauthorized, wantCode: "unauthenticated"},
		{name: "photo rejects client image", auth: m02AuthStub{}, path: "/api/v1/profile/photo", body: "bytes", wantStatus: http.StatusUnprocessableEntity, wantCode: "validation_error"},
		{name: "payout rejects client payload", auth: m02AuthStub{}, path: "/api/v1/payout-account", body: "payload", wantStatus: http.StatusUnprocessableEntity, wantCode: "validation_error"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := NewHandler(tc.auth, nil, nil)
			req := httptest.NewRequest(http.MethodPut, tc.path, strings.NewReader(tc.body))
			req.Header.Set("Authorization", "Bearer local-test")
			res := httptest.NewRecorder()
			h.ServeHTTP(res, req)
			if res.Code != tc.wantStatus {
				t.Fatalf("status=%d body=%s", res.Code, res.Body.String())
			}
			requestID := res.Header().Get("X-Request-ID")
			if requestID == "" {
				t.Fatal("missing X-Request-ID")
			}
			var envelope struct {
				Error map[string]string `json:"error"`
			}
			if err := json.Unmarshal(res.Body.Bytes(), &envelope); err != nil {
				t.Fatalf("decode error: %v", err)
			}
			for _, field := range []string{"code", "message", "request_id"} {
				if envelope.Error[field] == "" {
					t.Errorf("missing error.%s: %#v", field, envelope.Error)
				}
			}
			if envelope.Error["code"] != tc.wantCode || envelope.Error["request_id"] != requestID {
				t.Errorf("error does not match contract/header: %#v header=%q", envelope.Error, requestID)
			}
			if tc.wantStatus == http.StatusUnauthorized && res.Header().Get("WWW-Authenticate") != "Bearer" {
				t.Errorf("WWW-Authenticate=%q, want Bearer", res.Header().Get("WWW-Authenticate"))
			}
		})
	}
}
