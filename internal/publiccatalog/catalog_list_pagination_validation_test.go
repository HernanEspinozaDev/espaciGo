package publiccatalog

import (
	"context"
	"errors"
	"testing"
)

func TestListRejectsInvalidPageSizeAndSortBeforeRepository(t *testing.T) {
	for _, tc := range []struct {
		name    string
		filters Filters
	}{
		{name: "explicit zero page size", filters: Filters{PageSize: 0, PageSizeSet: true}},
		{name: "negative page size", filters: Filters{PageSize: -1, PageSizeSet: true}},
		{name: "page size over maximum", filters: Filters{PageSize: MaxPageSize + 1, PageSizeSet: true}},
		{name: "unknown sort", filters: Filters{Sort: "newest"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &discoveryRepository{}
			service := newDiscoveryService(t, repo)
			if _, err := service.List(context.Background(), tc.filters); !errors.Is(err, ErrInvalid) || repo.listCalls != 0 {
				t.Fatalf("List() error=%v calls=%d, want ErrInvalid/0", err, repo.listCalls)
			}
		})
	}
}
