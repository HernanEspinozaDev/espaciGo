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

type partialErrorPreviewQuoteRepository struct {
	*syntheticFlowBackend
	calls  int
	getErr error
	cancel context.CancelFunc
}

func (r *partialErrorPreviewQuoteRepository) GetForParticipant(ctx context.Context, actorID, quoteID string) (rentalflow.Quote, error) {
	r.calls++
	quote, err := r.syntheticFlowBackend.GetForParticipant(ctx, actorID, quoteID)
	if err != nil {
		return quote, err
	}
	if r.cancel != nil {
		r.cancel()
	}
	return quote, r.getErr
}

func TestSyntheticHTTPPreviewMapsRepositoryOutcomesWithoutReturningPartialSnapshot(t *testing.T) {
	const ownerID = "44444444-4444-4444-8444-444444444444"
	const renterID = "33333333-3333-4333-8333-333333333333"
	const title = "Título sintético reservado"
	const ownerName = "Propietario sintético privado"
	const renterName = "Arrendatario sintético privado"
	cases := []struct {
		name       string
		getErr     error
		cancel     bool
		wantStatus int
	}{
		{name: "operational failure", getErr: errors.New("synthetic private quote repository failure"), wantStatus: http.StatusServiceUnavailable},
		{name: "quote not found", getErr: rentalflow.ErrNotFound, wantStatus: http.StatusNotFound},
		{name: "cancellation concurrent with not found", getErr: rentalflow.ErrNotFound, cancel: true, wantStatus: http.StatusServiceUnavailable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
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
			quotes := &partialErrorPreviewQuoteRepository{syntheticFlowBackend: backend, getErr: tc.getErr}
			if tc.cancel {
				quotes.cancel = cancel
			}
			gate := &syntheticIdentityGate{ownerID: ownerID, renterID: renterID}
			identities := &syntheticIdentityReader{quoteID: testQuoteID, parties: map[string]rentalflow.ContractIdentity{
				ownerID:  {SubjectID: ownerID, LegalName: ownerName, RUT: "11.111.111-1"},
				renterID: {SubjectID: renterID, LegalName: renterName, RUT: "22.222.222-2"},
			}}
			previews, err := rentalflow.NewContractPreviewService(backend, backend, quotes, gate, identities, func() time.Time { return now })
			if err != nil {
				t.Fatal(err)
			}
			handler := New(&authStub{principal: identity.Principal{AccountID: renterID}}, nil, nil, previews, nil)
			request := httptest.NewRequest(http.MethodPost, "/api/v1/rental-quotes/"+testQuoteID+"/contract-preview", nil).WithContext(ctx)
			request.Header.Set("Authorization", "Bearer synthetic-session")
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)

			if response.Code != tc.wantStatus || response.Header().Get("Cache-Control") != "no-store" ||
				strings.Contains(response.Body.String(), `"data"`) || strings.Contains(response.Body.String(), tc.getErr.Error()) {
				t.Fatalf("partial quote repository error status=%d cache=%q body=%s, want %d/no-store without partial data", response.Code, response.Header().Get("Cache-Control"), response.Body.String(), tc.wantStatus)
			}
			for _, forbidden := range []string{title, ownerID, renterID, ownerName, renterName, "11111111-1", "22222222-2", "12000"} {
				if strings.Contains(errorDisclosureBody(t, response.Body.Bytes()), forbidden) {
					t.Fatalf("quote repository error disclosed %q: %s", forbidden, response.Body.String())
				}
			}
			if quotes.calls != 1 || backend.listingReads != 0 || backend.availabilityChecks != 0 || gate.calls != 0 || identities.calls != 0 {
				t.Fatalf("partial quote snapshot reached downstream reads: quote=%d listing=%d availability=%d gate=%d identities=%d", quotes.calls, backend.listingReads, backend.availabilityChecks, gate.calls, identities.calls)
			}
		})
	}
}
