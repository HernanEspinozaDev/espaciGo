package publiccatalog

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestListNormalizesSearchQueryAndRejectsUnsafeQueriesBeforeRepository(t *testing.T) {
	t.Run("normalizes Unicode and whitespace before listing", func(t *testing.T) {
		repo := &discoveryRepository{}
		service := newDiscoveryService(t, repo)
		_, err := service.List(context.Background(), Filters{Query: "  Cafe\u0301\t Centro  "})
		if err != nil || repo.listCalls != 1 || repo.lastListFilters.Query != "Café Centro" {
			t.Fatalf("List() error=%v list_calls=%d normalized_query=%q", err, repo.listCalls, repo.lastListFilters.Query)
		}
	})

	t.Run("accepts the 120 rune boundary", func(t *testing.T) {
		repo := &discoveryRepository{}
		service := newDiscoveryService(t, repo)
		query := strings.Repeat("a", 120)
		if _, err := service.List(context.Background(), Filters{Query: query}); err != nil || repo.listCalls != 1 || repo.lastListFilters.Query != query {
			t.Fatalf("List() error=%v list_calls=%d query_length=%d", err, repo.listCalls, len(repo.lastListFilters.Query))
		}
	})

	for _, tc := range []struct {
		name  string
		query string
	}{
		{name: "exceeds rune limit", query: strings.Repeat("á", 121)},
		{name: "contains control character", query: "Santiago\x00Centro"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &discoveryRepository{}
			service := newDiscoveryService(t, repo)
			if _, err := service.List(context.Background(), Filters{Query: tc.query}); !errors.Is(err, ErrInvalid) || repo.listCalls != 0 {
				t.Fatalf("List() error=%v list_calls=%d", err, repo.listCalls)
			}
		})
	}
}
