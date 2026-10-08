package m02local

import (
	"context"
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
