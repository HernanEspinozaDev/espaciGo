package rentalflowhttp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/HernanEspinozaDev/espaciGo/internal/rentalflow"
)

func TestWriteDomainErrorDistinguishesIdentityGateFromAvailabilityConflict(t *testing.T) {
	cases := []struct {
		name string
		err  error
		code string
	}{
		{name: "identity gate", err: rentalflow.ErrIdentityRequired, code: "identity_required"},
		{name: "availability changed", err: rentalflow.ErrConflict, code: "conflict"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			writeDomainError(response, test.err)
			var envelope struct {
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
				t.Fatal(err)
			}
			if response.Code != http.StatusConflict || envelope.Error.Code != test.code {
				t.Fatalf("status=%d code=%q, want 409 %q", response.Code, envelope.Error.Code, test.code)
			}
		})
	}
}
