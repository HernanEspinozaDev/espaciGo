package rentalflowhttp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
)

func TestPrivateRentalRoutesRejectMalformedAuthenticatedAccountID(t *testing.T) {
	cases := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{
			name:   "quote",
			method: http.MethodPost,
			path:   "/api/v1/rental-quotes",
			body:   `{"space_id":"` + testSpaceID + `","start_at":"2030-01-02T13:00:00Z","end_at":"2030-01-02T14:00:00Z"}`,
		},
		{
			name:   "contract preview",
			method: http.MethodPost,
			path:   "/api/v1/rental-quotes/" + testQuoteID + "/contract-preview",
		},
	}
	invalidAccounts := []struct {
		name string
		id   string
	}{
		{name: "malformed text", id: "not-a-uuid"},
		{name: "invalid variant nibble", id: "11111111-1111-4111-1111-111111111111"},
		{name: "unsupported version nibble", id: "11111111-1111-9111-8111-111111111111"},
	}
	for _, tc := range cases {
		for _, invalidAccount := range invalidAccounts {
			t.Run(tc.name+"/"+invalidAccount.name, func(t *testing.T) {
				auth := &authStub{principal: identity.Principal{AccountID: invalidAccount.id}}
				quote := &quoteStub{}
				preview := &previewStub{}
				handler := newTestHandler(auth, &availabilityStub{}, quote, preview)
				request := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
				request.Header.Set("Authorization", "Bearer synthetic-token")
				if tc.body != "" {
					request.Header.Set("Content-Type", "application/json")
				}
				response := httptest.NewRecorder()
				handler.ServeHTTP(response, request)

				var envelope struct {
					Error struct {
						Code string `json:"code"`
					} `json:"error"`
				}
				if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
					t.Fatalf("invalid error envelope: %v body=%s", err, response.Body.String())
				}
				if response.Code != http.StatusUnauthorized || envelope.Error.Code != "unauthenticated" ||
					response.Header().Get("WWW-Authenticate") != "Bearer" || response.Header().Get("Cache-Control") != "no-store" ||
					auth.calls != 1 || quote.calls != 0 || preview.calls != 0 {
					t.Fatalf("status=%d code=%q challenge=%q cache=%q auth=%d quote=%d preview=%d body=%s",
						response.Code, envelope.Error.Code, response.Header().Get("WWW-Authenticate"), response.Header().Get("Cache-Control"),
						auth.calls, quote.calls, preview.calls, response.Body.String())
				}
			})
		}
	}
}
