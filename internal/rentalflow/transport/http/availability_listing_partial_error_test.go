package rentalflowhttp

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/rentalflow"
)

type cancelAfterPartialListingErrorReader struct {
	base   *partialErrorPublicListingReader
	cancel context.CancelFunc
}

func (r *cancelAfterPartialListingErrorReader) PublicListing(ctx context.Context, spaceID string) (rentalflow.Listing, error) {
	listing, err := r.base.PublicListing(ctx, spaceID)
	r.cancel()
	return listing, err
}

func TestSyntheticHTTPAvailabilityDiscardsListingReturnedWithRepositoryError(t *testing.T) {
	const ownerID = "44444444-4444-4444-8444-444444444444"
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
			backend := &syntheticFlowBackend{listing: rentalflow.Listing{
				ID: testSpaceID, OwnerID: ownerID, State: "activa", Title: title,
				RateUnit: "hora", UnitPriceCLP: 12000, Currency: "CLP", RateVersion: 2, TimeZone: "America/Santiago",
			}}
			listings := &partialErrorPublicListingReader{backend: backend, err: tc.repoErr}
			availability, err := rentalflow.NewAvailabilityService(listings, backend, func() time.Time { return now })
			if err != nil {
				t.Fatal(err)
			}
			handler := New(nil, availability, nil, nil, nil)
			request := httptest.NewRequest(http.MethodGet,
				"/api/v1/spaces/"+testSpaceID+"/rental-availability?date=2030-01-02&duration=1", nil)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)

			if response.Code != tc.wantStatus || response.Header().Get("Cache-Control") != "no-store" ||
				strings.Contains(response.Body.String(), `"data"`) || strings.Contains(response.Body.String(), tc.repoErr.Error()) {
				t.Fatalf("partial listing error response status=%d cache=%q body=%s, want %d/no-store without data", response.Code, response.Header().Get("Cache-Control"), response.Body.String(), tc.wantStatus)
			}
			for _, forbidden := range []string{title, ownerID, "12000", "America/Santiago", "rate_version"} {
				if strings.Contains(errorDisclosureBody(t, response.Body.Bytes()), forbidden) {
					t.Fatalf("public listing error disclosed %q: %s", forbidden, response.Body.String())
				}
			}
			if backend.listingReads != 1 || backend.availabilityReads != 0 {
				t.Fatalf("listing error continued to calendar: listing=%d calendar=%d", backend.listingReads, backend.availabilityReads)
			}
		})
	}
}

func TestSyntheticHTTPAvailabilityCancellationPrecedesPartialListingNotFound(t *testing.T) {
	const ownerID = "44444444-4444-4444-8444-444444444444"
	const title = "Título sintético reservado"
	now := time.Date(2030, time.January, 1, 12, 0, 0, 0, time.UTC)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	backend := &syntheticFlowBackend{listing: rentalflow.Listing{
		ID: testSpaceID, OwnerID: ownerID, State: "activa", Title: title,
		RateUnit: "hora", UnitPriceCLP: 12000, Currency: "CLP", RateVersion: 2, TimeZone: "America/Santiago",
	}}
	base := &partialErrorPublicListingReader{backend: backend, err: rentalflow.ErrNotFound}
	listings := &cancelAfterPartialListingErrorReader{base: base, cancel: cancel}
	availability, err := rentalflow.NewAvailabilityService(listings, backend, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	handler := New(nil, availability, nil, nil, nil)
	request := httptest.NewRequest(http.MethodGet,
		"/api/v1/spaces/"+testSpaceID+"/rental-availability?date=2030-01-02&duration=1", nil).WithContext(ctx)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable || response.Header().Get("Cache-Control") != "no-store" ||
		strings.Contains(response.Body.String(), `"data"`) || strings.Contains(response.Body.String(), rentalflow.ErrNotFound.Error()) {
		t.Fatalf("canceled partial listing response status=%d cache=%q body=%s, want 503/no-store", response.Code, response.Header().Get("Cache-Control"), response.Body.String())
	}
	for _, forbidden := range []string{title, ownerID, "12000", "America/Santiago", "rate_version"} {
		if strings.Contains(errorDisclosureBody(t, response.Body.Bytes()), forbidden) {
			t.Fatalf("canceled listing response disclosed %q: %s", forbidden, response.Body.String())
		}
	}
	if backend.listingReads != 1 || backend.availabilityReads != 0 {
		t.Fatalf("canceled listing continued to calendar: listing=%d calendar=%d", backend.listingReads, backend.availabilityReads)
	}
}
