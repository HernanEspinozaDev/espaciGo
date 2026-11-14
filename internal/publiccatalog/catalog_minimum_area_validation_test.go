package publiccatalog

import (
	"context"
	"errors"
	"testing"
)

func TestListValidatesMinimumAreaBeforeRepository(t *testing.T) {
	t.Run("accepts positive canonical area", func(t *testing.T) {
		repo := &discoveryRepository{}
		service := newDiscoveryService(t, repo)
		_, err := service.List(context.Background(), Filters{MinAreaM2: "12.50"})
		if err != nil || repo.listCalls != 1 || repo.lastListFilters.MinAreaM2 != "12.50" {
			t.Fatalf("List() error=%v calls=%d min_area=%q", err, repo.listCalls, repo.lastListFilters.MinAreaM2)
		}
	})

	for _, value := range []string{"0", "0.00", "01", "1.234", "1e3"} {
		t.Run("rejects_"+value, func(t *testing.T) {
			repo := &discoveryRepository{}
			service := newDiscoveryService(t, repo)
			if _, err := service.List(context.Background(), Filters{MinAreaM2: value}); !errors.Is(err, ErrInvalid) || repo.listCalls != 0 {
				t.Fatalf("List() error=%v calls=%d, want ErrInvalid/0", err, repo.listCalls)
			}
		})
	}
}
