package rentalflowhttp

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
	"github.com/HernanEspinozaDev/espaciGo/internal/rentalflow"
)

func TestQuoteHandlerAcceptsRFC3339OffsetTimestamps(t *testing.T) {
	inputZone := time.FixedZone("synthetic", 5*60*60+30*60)
	start := time.Date(2030, time.May, 11, 20, 30, 0, 0, inputZone)
	end := start.Add(time.Hour)
	request := struct {
		SpaceID string `json:"space_id"`
		StartAt string `json:"start_at"`
		EndAt   string `json:"end_at"`
	}{SpaceID: testSpaceID, StartAt: start.Format(time.RFC3339), EndAt: end.Format(time.RFC3339)}
	body, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}

	quote := &quoteStub{result: rentalflow.Quote{ID: testQuoteID, SpaceID: testSpaceID}}
	auth := &authStub{principal: identity.Principal{AccountID: testActorID}}
	h := newTestHandler(auth, &availabilityStub{}, quote, &previewStub{})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/rental-quotes", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer token")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusCreated || auth.calls != 1 || quote.calls != 1 {
		t.Fatalf("status=%d auth_calls=%d quote_calls=%d body=%s", w.Code, auth.calls, quote.calls, w.Body.String())
	}
	if quote.request.StartAt != request.StartAt || quote.request.EndAt != request.EndAt {
		t.Fatalf("dispatched timestamps=(%q, %q), want original RFC3339 values (%q, %q)", quote.request.StartAt, quote.request.EndAt, request.StartAt, request.EndAt)
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("Cache-Control=%q, want no-store", w.Header().Get("Cache-Control"))
	}
}
