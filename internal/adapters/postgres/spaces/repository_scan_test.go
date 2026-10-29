package spacespg

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
	"time"
)

type scanRow struct{ values []any }

func (r scanRow) Scan(dest ...any) error {
	if len(dest) != len(r.values) {
		return fmt.Errorf("scan destination count %d does not match row value count %d", len(dest), len(r.values))
	}
	for i, value := range r.values {
		dv := reflect.ValueOf(dest[i]).Elem()
		if value == nil {
			continue
		}
		vv := reflect.ValueOf(value)
		if dv.Kind() == reflect.Ptr {
			ptr := reflect.New(dv.Type().Elem())
			ptr.Elem().Set(vv.Convert(dv.Type().Elem()))
			dv.Set(ptr)
		} else if vv.Type().AssignableTo(dv.Type()) {
			dv.Set(vv)
		} else {
			dv.Set(vv.Convert(dv.Type()))
		}
	}
	return nil
}

func TestScanPopulatesUpdatedAtButOmitsItFromJSON(t *testing.T) {
	want := time.Date(2026, time.March, 4, 5, 6, 7, 123, time.UTC)
	row := scanRow{values: []any{
		"space-id", "CAT", "Category", "Title", "Description", float64(12.5), int32(4),
		"Rules", "hour", int64(1000), "Address", "borrador", 2, []byte(`{"x":true}`),
		"America/Santiago", "", "RM", "13101", want,
	}}

	draft, err := scan(row)
	if err != nil {
		t.Fatalf("scan returned error: %v", err)
	}
	if !draft.UpdatedAt.Equal(want) {
		t.Fatalf("UpdatedAt = %v, want %v", draft.UpdatedAt, want)
	}
	encoded, err := json.Marshal(draft)
	if err != nil {
		t.Fatalf("marshal draft: %v", err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatalf("unmarshal JSON: %v", err)
	}
	if _, exists := fields["UpdatedAt"]; exists {
		t.Fatal("JSON unexpectedly includes UpdatedAt")
	}
}
