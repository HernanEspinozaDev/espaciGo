package publiccatalog

import (
	"context"
	"errors"
	"testing"
)

func TestListValidatesEnumListOptionsAndNormalizesTheirOrder(t *testing.T) {
	profile := []AttributeFilter{{Code: "equipment", Type: "enum_list", Options: []string{"wifi", "projector"}, ProfileVersion: 4}}

	t.Run("valid options reach repository in canonical order", func(t *testing.T) {
		repo := &discoveryRepository{categoryActive: true, filters: profile}
		service := newDiscoveryService(t, repo)
		_, err := service.List(context.Background(), Filters{
			CategoryCode: "oficina",
			TypedAttributes: map[string]AttributePredicate{
				"equipment": {Values: []string{"wifi", "projector"}},
			},
		})
		got := repo.lastListFilters.TypedAttributes["equipment"].Values
		if err != nil || repo.listCalls != 1 || len(got) != 2 || got[0] != "projector" || got[1] != "wifi" {
			t.Fatalf("List() error=%v list_calls=%d values=%v", err, repo.listCalls, got)
		}
	})

	for _, tc := range []struct {
		name   string
		values []string
	}{
		{name: "unknown option", values: []string{"wifi", "foreign"}},
		{name: "duplicate option", values: []string{"wifi", "wifi"}},
		{name: "empty option set", values: nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &discoveryRepository{categoryActive: true, filters: profile}
			service := newDiscoveryService(t, repo)
			_, err := service.List(context.Background(), Filters{
				CategoryCode: "oficina",
				TypedAttributes: map[string]AttributePredicate{
					"equipment": {Values: tc.values},
				},
			})
			if !errors.Is(err, ErrInvalid) || repo.listCalls != 0 {
				t.Fatalf("List() error=%v list_calls=%d, want ErrInvalid/0", err, repo.listCalls)
			}
		})
	}
}
