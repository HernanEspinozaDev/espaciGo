package publiccatalog

import (
	"context"
	"errors"
	"testing"
)

func TestListAllowsOnlyCanonicalRentalRateUnits(t *testing.T) {
	for _, unit := range []string{"hora", "dia", "mes"} {
		t.Run("accepts_"+unit, func(t *testing.T) {
			repo := &discoveryRepository{}
			service := newDiscoveryService(t, repo)
			_, err := service.List(context.Background(), Filters{RateUnit: unit})
			if err != nil || repo.listCalls != 1 || repo.lastListFilters.RateUnit != unit {
				t.Fatalf("List() error=%v calls=%d forwarded_unit=%q", err, repo.listCalls, repo.lastListFilters.RateUnit)
			}
		})
	}

	for _, tc := range []struct {
		name string
		unit string
	}{
		{name: "unsupported unit", unit: "semana"},
		{name: "wrong case", unit: "Hora"},
	} {
		t.Run("rejects_"+tc.name, func(t *testing.T) {
			repo := &discoveryRepository{}
			service := newDiscoveryService(t, repo)
			if _, err := service.List(context.Background(), Filters{RateUnit: tc.unit}); !errors.Is(err, ErrInvalid) || repo.listCalls != 0 {
				t.Fatalf("List() error=%v calls=%d, want ErrInvalid/0", err, repo.listCalls)
			}
		})
	}
}
