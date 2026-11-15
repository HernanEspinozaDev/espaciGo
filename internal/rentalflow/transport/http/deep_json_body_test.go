package rentalflowhttp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
)

func TestQuoteRejectsDeeplyNestedJSONWithoutDispatch(t *testing.T) {
	const nesting = 8000
	body := `{"space_id":` + strings.Repeat("[", nesting) + "0" + strings.Repeat("]", nesting) + `,"start_at":"2030-01-02T13:00:00Z","end_at":"2030-01-02T14:00:00Z"}`
	if len(body) > 16*1024 || !json.Valid([]byte(body)) {
		t.Fatalf("synthetic JSON fixture is invalid or exceeds request limit: len=%d", len(body))
	}

	auth := &authStub{principal: identity.Principal{AccountID: testActorID}}
	quote := &quoteStub{}
	h := newTestHandler(auth, &availabilityStub{}, quote, &previewStub{})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/rental-quotes", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer token")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest || auth.calls != 1 || quote.calls != 0 {
		t.Fatalf("status=%d auth_calls=%d quote_calls=%d body=%s", w.Code, auth.calls, quote.calls, w.Body.String())
	}
	if w.Header().Get("Cache-Control") != "no-store" || !json.Valid(w.Body.Bytes()) {
		t.Fatalf("cache=%q invalid response envelope=%q", w.Header().Get("Cache-Control"), w.Body.String())
	}
}
