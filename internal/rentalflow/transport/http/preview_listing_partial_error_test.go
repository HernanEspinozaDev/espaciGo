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

func TestSyntheticHTTPPreviewDiscardsListingReturnedWithRepositoryError(t *testing.T) {
	const ownerID = "44444444-4444-4444-8444-444444444444"
	const renterID = "33333333-3333-4333-8333-333333333333"
	const title = "Título sintético reservado"
	const ownerName = "Propietario sintético privado"
	const renterName = "Arrendatario sintético privado"
	cases := []struct {
		name       string
		repoErr    error
		cancel     bool
		wantStatus int
	}{
		{name: "listing disappeared after quote", repoErr: rentalflow.ErrNotFound, wantStatus: http.StatusConflict},
		{name: "cancellation concurrent with missing listing", repoErr: rentalflow.ErrNotFound, cancel: true, wantStatus: http.StatusServiceUnavailable},
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
				quote: rentalflow.Quote{
					ID: testQuoteID, SpaceID: testSpaceID, OwnerID: ownerID, RenterID: renterID, SpaceTitle: title,
					RateVersion: 2, RateUnit: "hora", UnitPriceCLP: 12000, Currency: "CLP", Units: 1, SubtotalCLP: 12000,
					StartAt: start, EndAt: start.Add(time.Hour), TimeZone: "America/Santiago",
					CreatedAt: now, ExpiresAt: now.Add(15 * time.Minute),
				},
			}
			ctx := context.Background()
			cancel := func() {}
			if tc.cancel {
				ctx, cancel = context.WithCancel(ctx)
			}
			defer cancel()
			baseListings := &partialErrorPublicListingReader{backend: backend, err: tc.repoErr}
			var listings rentalflow.PublicListingReader = baseListings
			if tc.cancel {
				listings = &cancelAfterPartialListingErrorReader{base: baseListings, cancel: cancel}
			}
			gate := &syntheticIdentityGate{ownerID: ownerID, renterID: renterID}
			identities := &syntheticIdentityReader{quoteID: testQuoteID, parties: map[string]rentalflow.ContractIdentity{
				ownerID:  {SubjectID: ownerID, LegalName: ownerName, RUT: "11.111.111-1"},
				renterID: {SubjectID: renterID, LegalName: renterName, RUT: "22.222.222-2"},
			}}
			previews, err := rentalflow.NewContractPreviewService(listings, backend, backend, gate, identities, func() time.Time { return now })
			if err != nil {
				t.Fatal(err)
			}
			handler := New(&authStub{principal: identity.Principal{AccountID: renterID}}, nil, nil, previews, nil)
			request := httptest.NewRequest(http.MethodPost, "/api/v1/rental-quotes/"+testQuoteID+"/contract-preview", nil).WithContext(ctx)
			request.Header.Set("Authorization", "Bearer synthetic-session")
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)

			if response.Code != tc.wantStatus || response.Header().Get("Cache-Control") != "no-store" ||
				strings.Contains(response.Body.String(), `"data"`) || strings.Contains(response.Body.String(), tc.repoErr.Error()) {
				t.Fatalf("partial listing error response status=%d cache=%q body=%s, want %d/no-store without data", response.Code, response.Header().Get("Cache-Control"), response.Body.String(), tc.wantStatus)
			}
			for _, forbidden := range []string{title, ownerID, renterID, ownerName, renterName, "11.111.111-1", "22.222.222-2", "12000"} {
				if strings.Contains(errorDisclosureBody(t, response.Body.Bytes()), forbidden) {
					t.Fatalf("listing error response disclosed %q: %s", forbidden, response.Body.String())
				}
			}
			if backend.listingReads != 1 || backend.availabilityChecks != 0 || gate.calls != 0 || identities.calls != 0 {
				t.Fatalf("listing error continued downstream: listing=%d calendar=%d gate=%d legal reads=%d", backend.listingReads, backend.availabilityChecks, gate.calls, identities.calls)
			}
		})
	}
}
