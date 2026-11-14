package publiccatalogpg

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/HernanEspinozaDev/espaciGo/internal/publiccatalog"
	"github.com/jackc/pgx/v5"
)

func TestDiscoveryRepositoryScansSyntheticRowsAndForwardsArguments(t *testing.T) {
	ctx := context.Background()
	pool := &coverSourcePool{row: []any{"oficina", "Oficina", 2}}
	repo := &Repository{pool: pool}

	categories, err := repo.Categories(ctx)
	if err != nil || len(categories) != 1 || categories[0] != (publiccatalog.Category{Code: "oficina", Name: "Oficina", Order: 2}) {
		t.Fatalf("Categories()=%#v err=%v", categories, err)
	}
	if len(pool.args) != 0 {
		t.Fatalf("Categories() args=%#v, want none", pool.args)
	}

	pool.row = []any{"13", "Metropolitana", "13101", "Santiago"}
	locations, err := repo.Locations(ctx, "Santiago", "13", 25)
	if err != nil || len(locations) != 1 || locations[0] != (publiccatalog.Location{RegionCode: "13", RegionName: "Metropolitana", CommuneCode: "13101", CommuneName: "Santiago"}) {
		t.Fatalf("Locations()=%#v err=%v", locations, err)
	}
	if want := []any{"13", "Santiago", 25}; !reflect.DeepEqual(pool.args, want) {
		t.Fatalf("Locations() args=%#v, want %#v", pool.args, want)
	}

	pool.row = []any{
		"altura", "Altura", "number", 1, "m", []byte(`["baja","alta"]`),
		sql.NullString{String: "0.5", Valid: true}, sql.NullString{String: "10", Valid: true}, sql.NullString{Valid: false}, 3,
	}
	filters, err := repo.AttributeFilters(ctx, "oficina")
	if err != nil || len(filters) != 1 {
		t.Fatalf("AttributeFilters()=%#v err=%v", filters, err)
	}
	filter := filters[0]
	if filter.Code != "altura" || filter.Label != "Altura" || filter.Type != "number" || filter.Order != 1 || filter.Unit != "m" || filter.ProfileVersion != 3 || !reflect.DeepEqual(filter.Options, []string{"baja", "alta"}) {
		t.Fatalf("AttributeFilters() result=%+v", filter)
	}
	if filter.Minimum == nil || *filter.Minimum != json.Number("0.5") || filter.Maximum == nil || *filter.Maximum != json.Number("10") || filter.Step != nil {
		t.Fatalf("AttributeFilters() numeric bounds min=%v max=%v step=%v", filter.Minimum, filter.Maximum, filter.Step)
	}
	if want := []any{"oficina"}; !reflect.DeepEqual(pool.args, want) {
		t.Fatalf("AttributeFilters() args=%#v, want %#v", pool.args, want)
	}
}

func TestDiscoveryRepositoryPropagatesQueryScanAndRowsErrors(t *testing.T) {
	failure := errors.New("synthetic query failure")
	repo := &Repository{pool: &coverSourcePool{queryErr: failure}}
	if _, err := repo.Categories(context.Background()); !errors.Is(err, failure) {
		t.Fatalf("Categories() query error=%v, want synthetic failure", err)
	}

	pool := &coverSourcePool{row: []any{"oficina", 17, 2}}
	repo = &Repository{pool: pool}
	if _, err := repo.Categories(context.Background()); err == nil {
		t.Fatal("Categories() accepted a non-string category name")
	}

	pool.row = []any{"oficina", "Oficina", 2}
	pool.rowsErr = failure
	if _, err := repo.Categories(context.Background()); !errors.Is(err, failure) {
		t.Fatalf("Categories() row-stream error=%v, want synthetic failure", err)
	}
}

func TestDiscoveryRepositoryRejectsInvalidAttributeOptionsJSON(t *testing.T) {
	pool := &coverSourcePool{row: []any{
		"uso", "Uso", "enum", 1, "", []byte(`{"not":"an array"}`),
		sql.NullString{Valid: false}, sql.NullString{Valid: false}, sql.NullString{Valid: false}, 1,
	}}
	repo := &Repository{pool: pool}
	if _, err := repo.AttributeFilters(context.Background(), "oficina"); err == nil {
		t.Fatal("AttributeFilters() accepted object options instead of an array")
	}
}

type discoveryQueryPool struct {
	*coverSourcePool
	query string
	args  []any
	value bool
	err   error
}

func (p *discoveryQueryPool) QueryRow(_ context.Context, query string, args ...any) pgx.Row {
	p.query = query
	p.args = append([]any(nil), args...)
	return discoveryQueryRow{value: p.value, err: p.err}
}

type discoveryQueryRow struct {
	value bool
	err   error
}

func (r discoveryQueryRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) != 1 {
		return errors.New("synthetic QueryRow expects exactly one destination")
	}
	value, ok := dest[0].(*bool)
	if !ok {
		return errors.New("synthetic QueryRow expects a bool destination")
	}
	*value = r.value
	return nil
}

func TestDiscoveryRepositoryExistenceLookupsBindCodesAndPropagateErrors(t *testing.T) {
	pool := &discoveryQueryPool{coverSourcePool: &coverSourcePool{}, value: true}
	repo := &Repository{pool: pool}
	cases := []struct {
		name, table string
		args        []any
		call        func() (bool, error)
	}{
		{name: "active category", table: "categoria_espacio", args: []any{"oficina"}, call: func() (bool, error) { return repo.HasActiveCategory(context.Background(), "oficina") }},
		{name: "active region", table: "region_publica", args: []any{"13"}, call: func() (bool, error) { return repo.HasRegion(context.Background(), "13") }},
		{name: "active commune", table: "comuna_publica", args: []any{"13101", "13"}, call: func() (bool, error) { return repo.HasCommune(context.Background(), "13", "13101") }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			active, err := tc.call()
			if err != nil || !active {
				t.Fatalf("lookup active=%v err=%v", active, err)
			}
			if !strings.Contains(pool.query, tc.table) || !reflect.DeepEqual(pool.args, tc.args) {
				t.Fatalf("query=%q args=%#v, expected table %q args %#v", pool.query, pool.args, tc.table, tc.args)
			}
		})
	}

	failure := errors.New("synthetic existence lookup failure")
	pool.err = failure
	if _, err := repo.HasRegion(context.Background(), "13"); !errors.Is(err, failure) {
		t.Fatalf("HasRegion() error=%v, want propagated row error", err)
	}
}

func TestDiscoveryRepositoryPropagatesLocationAndFilterErrors(t *testing.T) {
	failure := errors.New("synthetic repository stream failure")
	locationRow := []any{"13", "Metropolitana", "13101", "Santiago"}
	filterRow := []any{
		"altura", "Altura", "number", 1, "m", []byte(`["baja"]`),
		sql.NullString{String: "0", Valid: true}, sql.NullString{String: "10", Valid: true}, sql.NullString{Valid: false}, 3,
	}
	cases := []struct {
		name     string
		row      []any
		queryErr error
		rowsErr  error
		call     func(*Repository) error
		wantErr  error
	}{
		{name: "location query", queryErr: failure, wantErr: failure, call: func(r *Repository) error {
			_, err := r.Locations(context.Background(), "Santiago", "13", 10)
			return err
		}},
		{name: "location scan", row: []any{13, "Metropolitana", "13101", "Santiago"}, call: func(r *Repository) error {
			_, err := r.Locations(context.Background(), "Santiago", "13", 10)
			return err
		}},
		{name: "location stream", row: locationRow, rowsErr: failure, wantErr: failure, call: func(r *Repository) error {
			_, err := r.Locations(context.Background(), "Santiago", "13", 10)
			return err
		}},
		{name: "attribute filter query", queryErr: failure, wantErr: failure, call: func(r *Repository) error { _, err := r.AttributeFilters(context.Background(), "oficina"); return err }},
		{name: "attribute filter scan", row: []any{17, "Altura", "number", 1, "m", []byte(`[]`), sql.NullString{Valid: false}, sql.NullString{Valid: false}, sql.NullString{Valid: false}, 3}, call: func(r *Repository) error { _, err := r.AttributeFilters(context.Background(), "oficina"); return err }},
		{name: "attribute filter stream", row: filterRow, rowsErr: failure, wantErr: failure, call: func(r *Repository) error { _, err := r.AttributeFilters(context.Background(), "oficina"); return err }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pool := &coverSourcePool{row: tc.row, queryErr: tc.queryErr, rowsErr: tc.rowsErr}
			err := tc.call(&Repository{pool: pool})
			if err == nil || (tc.wantErr != nil && !errors.Is(err, tc.wantErr)) {
				t.Fatalf("error=%v, want non-nil and errors.Is(_, %v)", err, tc.wantErr)
			}
		})
	}
}
