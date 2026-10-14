package publiccatalog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

type memoryRepository struct{ rows []Space }

func (m memoryRepository) Categories(context.Context) ([]Category, error) {
	return []Category{{Code: "oficina", Name: "Oficina", Order: 1}}, nil
}
func (m memoryRepository) HasActiveCategory(context.Context, string) (bool, error) { return true, nil }
func (m memoryRepository) AttributeFilters(context.Context, string) ([]AttributeFilter, error) {
	min, max, step := json.Number("0.01"), json.Number("99.99"), json.Number("0.01")
	return []AttributeFilter{{Code: "wifi", Type: "boolean", ProfileVersion: 1}, {Code: "altura", Type: "number", Minimum: &min, Maximum: &max, Step: &step, ProfileVersion: 1}, {Code: "uso", Type: "enum", Options: []string{"privada", "compartida"}, ProfileVersion: 1}, {Code: "equipo", Type: "enum_list", Options: []string{"wifi", "proyector"}, ProfileVersion: 1}}, nil
}
func (m memoryRepository) HasRegion(context.Context, string) (bool, error)          { return true, nil }
func (m memoryRepository) HasCommune(context.Context, string, string) (bool, error) { return true, nil }
func (m memoryRepository) Locations(context.Context, string, string, int) ([]Location, error) {
	return nil, nil
}
func (m memoryRepository) Detail(context.Context, string) (Space, error) { return Space{}, ErrNotFound }
func (m memoryRepository) List(_ context.Context, f Filters, after *Cursor, limit int) ([]Space, error) {
	start := 0
	if after != nil {
		for i, v := range m.rows {
			if v.SpaceID == after.LastSpaceID {
				start = i + 1
				break
			}
		}
	}
	end := start + limit
	if end > len(m.rows) {
		end = len(m.rows)
	}
	return append([]Space(nil), m.rows[start:end]...), nil
}

func TestListCursorIsStableAndBoundToFilters(t *testing.T) {
	rows := []Space{{SpaceID: "00000000-0000-0000-0000-000000000001", CategoryOrder: 1, TitleKey: "a", BasePriceCLP: 6000}, {SpaceID: "00000000-0000-0000-0000-000000000002", CategoryOrder: 1, TitleKey: "b", BasePriceCLP: 7000}, {SpaceID: "00000000-0000-0000-0000-000000000003", CategoryOrder: 1, TitleKey: "c", BasePriceCLP: 8000}}
	svc, err := New(memoryRepository{rows}, []byte("01234567890123456789012345678901"))
	if err != nil {
		t.Fatal(err)
	}
	page, err := svc.List(context.Background(), Filters{PageSize: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 2 || page.NextCursor == "" {
		t.Fatalf("unexpected first page: %#v", page)
	}
	second, err := svc.List(context.Background(), Filters{PageSize: 2, Cursor: page.NextCursor})
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Items) != 1 || second.Items[0].SpaceID != rows[2].SpaceID {
		t.Fatalf("unexpected next page: %#v", second)
	}
	if _, err = svc.List(context.Background(), Filters{PageSize: 2, RateUnit: "hora", Cursor: page.NextCursor}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("cursor accepted with changed filters: %v", err)
	}
}

func TestListRejectsUnitPriceComparisonsWithoutModality(t *testing.T) {
	svc, _ := New(memoryRepository{}, []byte("01234567890123456789012345678901"))
	if _, err := svc.List(context.Background(), Filters{Sort: "unit_price_asc"}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("expected invalid request, got %v", err)
	}
	price := int64(1 << 62)
	if _, err := svc.List(context.Background(), Filters{RateUnit: "dia", MinUnitPriceCLP: &price}); err != nil {
		t.Fatalf("valid int64 rejected: %v", err)
	}
}

func TestListAcceptsStableTitleSorts(t *testing.T) {
	svc, _ := New(memoryRepository{}, []byte("01234567890123456789012345678901"))
	for _, order := range []string{"title_asc", "title_desc"} {
		if _, err := svc.List(context.Background(), Filters{Sort: order, PageSize: 20}); err != nil {
			t.Fatalf("%s rejected: %v", order, err)
		}
	}
}

func TestTypedPredicatesAreProfileValidatedAndCursorVersioned(t *testing.T) {
	svc, _ := New(memoryRepository{}, []byte("01234567890123456789012345678901"))
	min := "1.23"
	valid := Filters{CategoryCode: "oficina", PageSize: 20, TypedAttributes: map[string]AttributePredicate{"altura": {Minimum: &min}, "uso": {Value: stringPointer("privada")}, "equipo": {Values: []string{"wifi", "proyector"}}}}
	if _, err := svc.List(context.Background(), valid); err != nil {
		t.Fatalf("valid typed predicates rejected: %v", err)
	}
	for _, predicate := range []AttributePredicate{{Minimum: stringPointer("1.234")}, {Minimum: stringPointer("100")}, {Value: stringPointer("foreign")}, {Values: []string{"wifi", "wifi"}}} {
		f := Filters{CategoryCode: "oficina", PageSize: 20, TypedAttributes: map[string]AttributePredicate{"altura": predicate}}
		if _, err := svc.List(context.Background(), f); !errors.Is(err, ErrInvalid) {
			t.Fatalf("invalid predicate accepted: %#v / %v", predicate, err)
		}
	}
	if _, err := svc.List(context.Background(), Filters{CategoryCode: "oficina", TypedAttributes: map[string]AttributePredicate{"altura": {Type: "integer", Minimum: &min}}}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("type-mismatched predicate accepted: %v", err)
	}
	if _, err := svc.List(context.Background(), Filters{TypedAttributes: map[string]AttributePredicate{"altura": {Minimum: &min}}}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("filter without category accepted: %v", err)
	}
	tooMany := Filters{TypedAttributes: make(map[string]AttributePredicate, 17)}
	for i := 0; i < 17; i++ {
		tooMany.TypedAttributes[fmt.Sprintf("attribute_%02d", i)] = AttributePredicate{Minimum: &min}
	}
	if err := validate(&tooMany); !errors.Is(err, ErrInvalid) {
		t.Fatalf("more than 16 typed fields accepted: %v", err)
	}
	cursor, err := svc.encodeCursor(Cursor{Version: cursorVersion, LastSpaceID: "00000000-0000-0000-0000-000000000001"})
	if err != nil || !strings.HasPrefix(cursor, "v3.") {
		t.Fatalf("typed cursor version missing: %q %v", cursor, err)
	}
	if _, err := svc.decodeCursor(strings.Replace(cursor, "v3.", "v2.", 1)); !errors.Is(err, ErrInvalid) {
		t.Fatalf("old cursor unexpectedly accepted: %v", err)
	}
}

func stringPointer(value string) *string { return &value }
