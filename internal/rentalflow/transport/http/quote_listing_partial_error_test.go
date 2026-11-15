package rentalflowhttp

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
	"github.com/HernanEspinozaDev/espaciGo/internal/rentalflow"
)

type partialErrorPublicListingReader struct {
	backend *syntheticFlowBackend
	err     error
}

func (r *partialErrorPublicListingReader) PublicListing(ctx context.Context, spaceID string) (rentalflow.Listing, error) {
	listing, err := r.backend.PublicListing(ctx, spaceID)
	if err != nil {
		return listing, err
	}
	return listing, r.err
}

func TestSyntheticHTTPQuoteDiscardsListingReturnedWithRepositoryError(t *testing.T) {
	const ownerID = "44444444-4444-4444-8444-444444444444"
	const renterID = "33333333-3333-4333-8333-333333333333"
	const title = "Título sintético reservado"
	cases := []struct {
		name       string
		repoErr    error
		wantStatus int
	}{
		{name: "listing disappeared", repoErr: rentalflow.ErrNotFound, wantStatus: http.StatusNotFound},
		{name: "operational failure", repoErr: errors.New("synthetic private listing repository failure"), wantStatus: http.StatusServiceUnavailable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Date(2030, time.January, 1, 12, 0, 0, 0, time.UTC)
			start := time.Date(2030, time.January, 2, 13, 0, 0, 0, time.UTC)
			backend := &syntheticFlowBackend{
				listing: rentalflow.Listing{
					ID: testSpaceID, OwnerID: ownerID, State: "activa", Title: title,
					RateUnit: "hora", UnitPriceCLP: 12000, Currency: "CLP", RateVersion: 2, TimeZone: "America/Santiago",
				},
				interval: rentalflow.Interval{StartAt: start, EndAt: start.Add(time.Hour)}, available: true,
			}
			listings := &partialErrorPublicListingReader{backend: backend, err: tc.repoErr}
			quotes, err := rentalflow.NewQuoteService(listings, backend, backend, func() time.Time { return now })
			if err != nil {
				t.Fatal(err)
			}
			handler := New(&authStub{principal: identity.Principal{AccountID: renterID}}, nil, quotes, nil, nil)
			body := `{"space_id":"` + testSpaceID + `","start_at":"` + start.Format(time.RFC3339) + `","end_at":"` + start.Add(time.Hour).Format(time.RFC3339) + `"}`
			request := httptest.NewRequest(http.MethodPost, "/api/v1/rental-quotes", strings.NewReader(body))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Authorization", "Bearer synthetic-session")
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)

			if response.Code != tc.wantStatus || response.Header().Get("Cache-Control") != "no-store" ||
				strings.Contains(response.Body.String(), `"data"`) || strings.Contains(response.Body.String(), tc.repoErr.Error()) {
				t.Fatalf("partial listing error response status=%d cache=%q body=%s, want %d/no-store without data", response.Code, response.Header().Get("Cache-Control"), response.Body.String(), tc.wantStatus)
			}
			for _, forbidden := range []string{title, ownerID, renterID, testQuoteID, "12000", "rate_version"} {
				if strings.Contains(errorDisclosureBody(t, response.Body.Bytes()), forbidden) {
					t.Fatalf("listing error response disclosed %q: %s", forbidden, response.Body.String())
				}
			}
			if backend.listingReads != 1 || backend.availabilityChecks != 0 || backend.quoteWrites != 0 {
				t.Fatalf("listing error continued downstream: listing=%d calendar=%d quote writes=%d", backend.listingReads, backend.availabilityChecks, backend.quoteWrites)
			}
		})
	}
}
