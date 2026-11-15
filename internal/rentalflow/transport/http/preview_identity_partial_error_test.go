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

type partialContractIdentityErrorReader struct {
	quoteID      string
	parties      map[string]rentalflow.ContractIdentity
	errorSubject string
	calls        []string
}

func (r *partialContractIdentityErrorReader) CurrentForRentalContract(_ context.Context, _, quoteID, subjectID string, _ time.Time) (rentalflow.ContractIdentity, error) {
	r.calls = append(r.calls, subjectID)
	identity, ok := r.parties[subjectID]
	if quoteID != r.quoteID || !ok {
		return rentalflow.ContractIdentity{}, rentalflow.ErrNotFound
	}
	if subjectID == r.errorSubject {
		return identity, errors.New("synthetic private legal identity read failure")
	}
	return identity, nil
}

func TestSyntheticHTTPPreviewDiscardsIdentityReturnedWithReaderError(t *testing.T) {
	const ownerID = "44444444-4444-4444-8444-444444444444"
	const renterID = "33333333-3333-4333-8333-333333333333"
	const title = "Título sintético reservado"
	const ownerName = "Propietario sintético privado"
	const renterName = "Arrendatario sintético privado"
	cases := []struct {
		name          string
		errorSubject  string
		wantReadOrder []string
	}{
		{name: "owner identity returned with error", errorSubject: ownerID, wantReadOrder: []string{ownerID}},
		{name: "renter identity returned with error after owner", errorSubject: renterID, wantReadOrder: []string{ownerID, renterID}},
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
			gate := &syntheticIdentityGate{ownerID: ownerID, renterID: renterID}
			reader := &partialContractIdentityErrorReader{
				quoteID: testQuoteID, errorSubject: tc.errorSubject,
				parties: map[string]rentalflow.ContractIdentity{
					ownerID:  {SubjectID: ownerID, LegalName: ownerName, RUT: "11.111.111-1"},
					renterID: {SubjectID: renterID, LegalName: renterName, RUT: "22.222.222-2"},
				},
			}
			previews, err := rentalflow.NewContractPreviewService(backend, backend, backend, gate, reader, func() time.Time { return now })
			if err != nil {
				t.Fatal(err)
			}
			handler := New(&authStub{principal: identity.Principal{AccountID: renterID}}, nil, nil, previews, nil)
			request := httptest.NewRequest(http.MethodPost, "/api/v1/rental-quotes/"+testQuoteID+"/contract-preview", nil)
			request.Header.Set("Authorization", "Bearer synthetic-session")
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)

			if response.Code != http.StatusServiceUnavailable || response.Header().Get("Cache-Control") != "no-store" ||
				strings.Contains(response.Body.String(), `"data"`) || strings.Contains(response.Body.String(), "synthetic private legal identity read failure") {
				t.Fatalf("partial identity error response status=%d cache=%q body=%s, want generic 503/no-store without data", response.Code, response.Header().Get("Cache-Control"), response.Body.String())
			}
			for _, forbidden := range []string{title, ownerID, renterID, ownerName, renterName, "11.111.111-1", "22.222.222-2", "11111111-1", "22222222-2", "12000"} {
				if strings.Contains(errorDisclosureBody(t, response.Body.Bytes()), forbidden) {
					t.Fatalf("identity reader error disclosed %q: %s", forbidden, response.Body.String())
				}
			}
			if gate.calls != 1 || backend.listingReads != 1 || backend.availabilityChecks != 1 || len(reader.calls) != len(tc.wantReadOrder) {
				t.Fatalf("unexpected reads after partial identity error: gate=%d listing=%d calendar=%d legal=%v", gate.calls, backend.listingReads, backend.availabilityChecks, reader.calls)
			}
			for i, subject := range tc.wantReadOrder {
				if reader.calls[i] != subject {
					t.Fatalf("legal identity read order=%v, want %v", reader.calls, tc.wantReadOrder)
				}
			}
		})
	}
}
