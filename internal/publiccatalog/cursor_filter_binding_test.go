package publiccatalog

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestListRejectsCursorWhenBoundFiltersChange(t *testing.T) {
	start := time.Date(2030, time.January, 2, 13, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	for _, tc := range []struct {
		name   string
		setup  func(*Filters)
		mutate func(*Filters)
	}{
		{name: "search query", mutate: func(filters *Filters) { filters.Query = "estudio" }},
		{name: "rental window", mutate: func(filters *Filters) {
			changedStart := filters.StartAt.Add(time.Minute)
			filters.StartAt = &changedStart
		}},
		{name: "category", setup: func(filters *Filters) { filters.CategoryCode = "oficina" }, mutate: func(filters *Filters) { filters.CategoryCode = "auditorio" }},
		{name: "region", setup: func(filters *Filters) { filters.RegionCode = "13" }, mutate: func(filters *Filters) { filters.RegionCode = "05" }},
		{
			name: "typed attribute",
			setup: func(filters *Filters) {
				value := "privada"
				filters.CategoryCode = "oficina"
				filters.TypedAttributes = map[string]AttributePredicate{"uso": {Value: &value}}
			},
			mutate: func(filters *Filters) {
				value := "compartida"
				filters.TypedAttributes["uso"] = AttributePredicate{Value: &value}
			},
		},
		{
			name: "legacy boolean attribute",
			setup: func(filters *Filters) {
				filters.CategoryCode = "oficina"
				filters.Attributes = map[string]bool{"wifi": true}
			},
			mutate: func(filters *Filters) { filters.Attributes["wifi"] = false },
		},
		{
			name: "estimated total lower bound",
			setup: func(filters *Filters) {
				minimum := int64(5000)
				filters.MinTotalCLP = &minimum
				filters.Sort = "estimated_total_asc"
			},
			mutate: func(filters *Filters) {
				minimum := int64(9000)
				filters.MinTotalCLP = &minimum
			},
		},
		{name: "minimum area", setup: func(filters *Filters) { filters.MinAreaM2 = "10" }, mutate: func(filters *Filters) { filters.MinAreaM2 = "12" }},
		{
			name: "commune",
			setup: func(filters *Filters) {
				filters.RegionCode = "13"
				filters.CommuneCode = "13101"
			},
			mutate: func(filters *Filters) { filters.CommuneCode = "13102" },
		},
		{
			name: "minimum unit price",
			setup: func(filters *Filters) {
				minimum := int64(5000)
				filters.RateUnit = "hora"
				filters.MinUnitPriceCLP = &minimum
			},
			mutate: func(filters *Filters) {
				minimum := int64(6000)
				filters.MinUnitPriceCLP = &minimum
			},
		},
		{
			name: "maximum unit price",
			setup: func(filters *Filters) {
				maximum := int64(10000)
				filters.RateUnit = "hora"
				filters.MaxUnitPriceCLP = &maximum
			},
			mutate: func(filters *Filters) {
				maximum := int64(9000)
				filters.MaxUnitPriceCLP = &maximum
			},
		},
		{
			name: "maximum estimated total",
			setup: func(filters *Filters) {
				maximum := int64(10000)
				filters.MaxTotalCLP = &maximum
				filters.Sort = "estimated_total_desc"
			},
			mutate: func(filters *Filters) {
				maximum := int64(9000)
				filters.MaxTotalCLP = &maximum
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &cursorCountingRepository{memoryRepository: memoryRepository{rows: []Space{
				{SpaceID: "00000000-0000-4000-8000-000000000001", TitleKey: "a", RateUnit: "hora", BasePriceCLP: 8000, AreaM2: 20, TimeZone: "UTC"},
				{SpaceID: "00000000-0000-4000-8000-000000000002", TitleKey: "b", RateUnit: "hora", BasePriceCLP: 8000, AreaM2: 20, TimeZone: "UTC"},
			}}}
			service, err := New(repo, []byte("01234567890123456789012345678901"))
			if err != nil {
				t.Fatal(err)
			}
			filters := Filters{PageSize: 1, PageSizeSet: true, Query: "sala", StartAt: &start, EndAt: &end}
			if tc.setup != nil {
				tc.setup(&filters)
			}
			first, err := service.List(context.Background(), filters)
			if err != nil || first.NextCursor == "" || repo.listCalls != 1 {
				t.Fatalf("first page=%+v error=%v list_calls=%d", first, err, repo.listCalls)
			}

			changed := filters
			changed.Cursor = first.NextCursor
			tc.mutate(&changed)
			if _, err = service.List(context.Background(), changed); !errors.Is(err, ErrInvalid) || repo.listCalls != 1 {
				t.Fatalf("changed filters accepted cursor: error=%v list_calls=%d", err, repo.listCalls)
			}
		})
	}
}

type changingAttributeProfileRepository struct {
	*cursorCountingRepository
	profileVersion int
	profileReads   int
}

func (r *changingAttributeProfileRepository) AttributeFilters(_ context.Context, category string) ([]AttributeFilter, error) {
	r.profileReads++
	if category != "oficina" {
		return nil, ErrNotFound
	}
	return []AttributeFilter{{Code: "uso", Type: "enum", Options: []string{"privada", "compartida"}, ProfileVersion: r.profileVersion}}, nil
}

func TestListRejectsCursorWhenAttributeProfileVersionChanges(t *testing.T) {
	rows := []Space{
		{SpaceID: "00000000-0000-4000-8000-000000000001", TitleKey: "a"},
		{SpaceID: "00000000-0000-4000-8000-000000000002", TitleKey: "b"},
	}
	repo := &changingAttributeProfileRepository{
		cursorCountingRepository: &cursorCountingRepository{memoryRepository: memoryRepository{rows: rows}},
		profileVersion:           1,
	}
	service, err := New(repo, []byte("01234567890123456789012345678901"))
	if err != nil {
		t.Fatal(err)
	}
	value := "privada"
	filters := Filters{
		CategoryCode: "oficina", PageSize: 1, PageSizeSet: true,
		TypedAttributes: map[string]AttributePredicate{"uso": {Value: &value}},
	}
	first, err := service.List(context.Background(), filters)
	if err != nil || first.NextCursor == "" || repo.listCalls != 1 || repo.profileReads != 1 {
		t.Fatalf("first page=%+v error=%v list_calls=%d profile_reads=%d", first, err, repo.listCalls, repo.profileReads)
	}

	repo.profileVersion = 2
	filters.Cursor = first.NextCursor
	if _, err = service.List(context.Background(), filters); !errors.Is(err, ErrInvalid) || repo.listCalls != 1 || repo.profileReads != 2 {
		t.Fatalf("cursor from stale attribute profile accepted: error=%v list_calls=%d profile_reads=%d", err, repo.listCalls, repo.profileReads)
	}
}
