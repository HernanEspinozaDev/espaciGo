package rentalflowhttp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
	"github.com/HernanEspinozaDev/espaciGo/internal/rentalflow"
)

func TestSyntheticHTTPQuoteCancellationPrecedesPartialListingNotFound(t *testing.T) {
	const ownerID = "44444444-4444-4444-8444-444444444444"
	const renterID = "33333333-3333-4333-8333-333333333333"
	const title = "Título sintético reservado"
	now := time.Date(2030, time.January, 1, 12, 0, 0, 0, time.UTC)
	start := time.Date(2030, time.January, 2, 13, 0, 0, 0, time.UTC)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	backend := &syntheticFlowBackend{
		listing: rentalflow.Listing{
			ID: testSpaceID, OwnerID: ownerID, State: "activa", Title: title,
			RateUnit: "hora", UnitPriceCLP: 12000, Currency: "CLP", RateVersion: 2, TimeZone: "America/Santiago",
		},
		interval: rentalflow.Interval{StartAt: start, EndAt: start.Add(time.Hour)}, available: true,
	}
	base := &partialErrorPublicListingReader{backend: backend, err: rentalflow.ErrNotFound}
	listings := &cancelAfterPartialListingErrorReader{base: base, cancel: cancel}
	quotes, err := rentalflow.NewQuoteService(listings, backend, backend, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	handler := New(&authStub{principal: identity.Principal{AccountID: renterID}}, nil, quotes, nil, nil)
	body := `{"space_id":"` + testSpaceID + `","start_at":"` + start.Format(time.RFC3339) + `","end_at":"` + start.Add(time.Hour).Format(time.RFC3339) + `"}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/rental-quotes", strings.NewReader(body)).WithContext(ctx)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer synthetic-session")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable || response.Header().Get("Cache-Control") != "no-store" ||
		strings.Contains(response.Body.String(), `"data"`) || strings.Contains(response.Body.String(), rentalflow.ErrNotFound.Error()) {
		t.Fatalf("canceled partial quote listing response status=%d cache=%q body=%s, want 503/no-store without data", response.Code, response.Header().Get("Cache-Control"), response.Body.String())
	}
	for _, forbidden := range []string{title, ownerID, renterID, "12000", "rate_version", testQuoteID} {
		if strings.Contains(errorDisclosureBody(t, response.Body.Bytes()), forbidden) {
			t.Fatalf("canceled quote listing response disclosed %q: %s", forbidden, response.Body.String())
		}
	}
	if backend.listingReads != 1 || backend.availabilityChecks != 0 || backend.quoteWrites != 0 {
		t.Fatalf("canceled quote listing continued downstream: listing=%d calendar=%d quote writes=%d", backend.listingReads, backend.availabilityChecks, backend.quoteWrites)
	}
}
