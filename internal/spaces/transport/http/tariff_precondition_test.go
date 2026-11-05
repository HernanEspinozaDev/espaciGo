package spaceshttp

import (
	"context"
	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/credentials"
	"github.com/HernanEspinozaDev/espaciGo/internal/pricing"
	"github.com/HernanEspinozaDev/espaciGo/internal/spaces"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func (p *priceRepo) UpdateRateIfRevision(ctx context.Context, owner, id string, expected time.Time, in pricing.RateInput) (pricing.Rate, time.Time, error) {
	if owner != ownerA || id != draftID {
		return pricing.Rate{}, time.Time{}, pricing.ErrNotFound
	}
	if !expected.Equal(p.revision) {
		return pricing.Rate{}, time.Time{}, pricing.ErrPreconditionFailed
	}
	rate, err := p.UpdateRate(ctx, owner, id, in)
	if err == nil {
		p.revision = expected.Add(time.Microsecond)
	}
	return rate, p.revision, err
}
func TestTariffPreconditions(t *testing.T) {
	for _, tc := range []struct {
		name, tag, body string
		status          int
	}{
		{"missing-before-body", "", "{", 428},
		{"malformed", "*", `{"rate_unit":"hora","base_price":9000}`, 400},
		{"wrong-resource", draftETag(spaces.Draft{ID: ownerB, UpdatedAt: time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)}), `{"rate_unit":"hora","base_price":9000}`, 412},
		{"stale", draftETag(spaces.Draft{ID: draftID, UpdatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}), `{"rate_unit":"hora","base_price":9000}`, 412},
		{"current", "current", `{"rate_unit":"hora","base_price":9000}`, 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
			repo := &priceRepo{revision: now, rate: pricing.Rate{SpaceID: draftID, Version: 1, Unit: "hora", Amount: 8000, Currency: "CLP", CreatedAt: now}}
			svc, _ := pricing.NewService(repo, fakeCalendar{}, credentials.Generator{})
			ss, _ := spaces.NewService(&memoryRepo{})
			h := NewHandlerWithPricing(fakeAuth{}, ss, nil, fakeCalendar{}, svc)
			req := httptest.NewRequest(http.MethodPut, "/api/v1/spaces/"+draftID+"/tariff", strings.NewReader(tc.body))
			req.Header.Set("Authorization", "Bearer landlord")
			req.Header.Set("Content-Type", "application/json")
			if tc.tag == "current" {
				req.Header.Set("If-Match", draftETag(spaces.Draft{ID: draftID, UpdatedAt: now}))
			} else if tc.tag != "" {
				req.Header.Set("If-Match", tc.tag)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)
			if w.Code != tc.status {
				t.Fatalf("status=%d want=%d body=%s", w.Code, tc.status, w.Body.String())
			}
			if tc.status != 200 && (repo.rate.Version != 1 || repo.rate.Amount != 8000) {
				t.Fatal("rejected tariff mutated")
			}
			if tc.status == 200 && w.Header().Get("ETag") != draftETag(spaces.Draft{ID: draftID, UpdatedAt: repo.revision}) {
				t.Fatal("renewed ETag missing")
			}
		})
	}
}
