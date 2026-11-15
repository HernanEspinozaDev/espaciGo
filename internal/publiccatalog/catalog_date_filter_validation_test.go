package publiccatalog

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestListRequiresOrderedCompleteDateWindowBeforeRepository(t *testing.T) {
	start := time.Date(2030, time.January, 10, 12, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	validRepo := &discoveryRepository{}
	validService := newDiscoveryService(t, validRepo)
	if _, err := validService.List(context.Background(), Filters{StartAt: &start, EndAt: &end}); err != nil {
		t.Fatalf("valid date window rejected: %v", err)
	}
	if validRepo.listCalls != 1 || validRepo.lastListFilters.StartAt == nil || !validRepo.lastListFilters.StartAt.Equal(start) || validRepo.lastListFilters.EndAt == nil || !validRepo.lastListFilters.EndAt.Equal(end) {
		t.Fatalf("valid window calls=%d start=%v end=%v", validRepo.listCalls, validRepo.lastListFilters.StartAt, validRepo.lastListFilters.EndAt)
	}

	before := start.Add(-time.Hour)
	cases := []struct {
		name    string
		filters Filters
	}{
		{name: "start without end", filters: Filters{StartAt: &start}},
		{name: "end without start", filters: Filters{EndAt: &end}},
		{name: "empty interval", filters: Filters{StartAt: &start, EndAt: &start}},
		{name: "reversed interval", filters: Filters{StartAt: &start, EndAt: &before}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &discoveryRepository{}
			service := newDiscoveryService(t, repo)
			if _, err := service.List(context.Background(), tc.filters); !errors.Is(err, ErrInvalid) || repo.listCalls != 0 {
				t.Fatalf("List() error=%v list_calls=%d, want ErrInvalid/0", err, repo.listCalls)
			}
		})
	}
}
