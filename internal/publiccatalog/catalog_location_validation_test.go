package publiccatalog

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestListRejectsMalformedCategoryAndLocationFiltersBeforeRepository(t *testing.T) {
	validRepo := &discoveryRepository{categoryActive: true, regionActive: true, communeActive: true}
	validService := newDiscoveryService(t, validRepo)
	if _, err := validService.List(context.Background(), Filters{CategoryCode: "oficina", RegionCode: "13", CommuneCode: "13101"}); err != nil {
		t.Fatalf("valid control filters rejected: %v", err)
	}
	if validRepo.categoryCalls != 1 || validRepo.regionCalls != 1 || validRepo.communeCalls != 1 || validRepo.listCalls != 1 {
		t.Fatalf("valid control calls category/region/commune/list=%d/%d/%d/%d", validRepo.categoryCalls, validRepo.regionCalls, validRepo.communeCalls, validRepo.listCalls)
	}

	cases := []struct {
		name    string
		filters Filters
	}{
		{name: "category surrounding whitespace", filters: Filters{CategoryCode: " oficina"}},
		{name: "category too long", filters: Filters{CategoryCode: strings.Repeat("a", 81)}},
		{name: "malformed region", filters: Filters{RegionCode: "1x"}},
		{name: "non-ASCII region digits", filters: Filters{RegionCode: "１3"}},
		{name: "malformed commune", filters: Filters{RegionCode: "13", CommuneCode: "13x01"}},
		{name: "commune without region", filters: Filters{CommuneCode: "13101"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &discoveryRepository{categoryActive: true, regionActive: true, communeActive: true}
			service := newDiscoveryService(t, repo)
			_, err := service.List(context.Background(), tc.filters)
			if !errors.Is(err, ErrInvalid) || repo.categoryCalls != 0 || repo.regionCalls != 0 || repo.communeCalls != 0 || repo.listCalls != 0 {
				t.Fatalf("List() error=%v calls category/region/commune/list=%d/%d/%d/%d", err, repo.categoryCalls, repo.regionCalls, repo.communeCalls, repo.listCalls)
			}
		})
	}
}
