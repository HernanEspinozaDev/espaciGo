package publiccatalogpg

import (
	"context"
	"database/sql"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/publiccatalog"
)

func TestListAdapterBindsFilterCursorAndTypedAttributeArguments(t *testing.T) {
	start := time.Date(2030, time.January, 10, 12, 0, 0, 0, time.UTC)
	end := start.Add(90 * time.Minute)
	minUnit, maxUnit := int64(1000), int64(5000)
	minTotal, maxTotal := int64(2000), int64(10000)
	minArea, minHeight, maxHeight, usage := "10.5", "1.25", "3.5", "privada"
	lastTotal := int64(7500)
	cursor := &publiccatalog.Cursor{LastEstimatedTotalCLP: &lastTotal, LastSpaceID: "11111111-1111-4111-8111-111111111111"}
	filters := publiccatalog.Filters{
		CategoryCode: "oficina", RateUnit: "hora", MinUnitPriceCLP: &minUnit, MaxUnitPriceCLP: &maxUnit,
		Query: "sala", RegionCode: "13", CommuneCode: "13101", MinAreaM2: minArea,
		Attributes: map[string]bool{"wifi": true, "proyector": false}, AttributeVersion: 3,
		TypedAttributes: map[string]publiccatalog.AttributePredicate{
			"altura": {Type: "number", Minimum: &minHeight, Maximum: &maxHeight},
			"equipo": {Type: "enum_list", Values: []string{"wifi", "proyector"}},
			"uso":    {Type: "enum", Value: &usage},
		},
		StartAt: &start, EndAt: &end, MinTotalCLP: &minTotal, MaxTotalCLP: &maxTotal, Sort: "estimated_total_asc",
	}
	pool := &coverSourcePool{row: []any{
		"11111111-1111-4111-8111-111111111111", "oficina", "Oficina", "Sala", "Sala sintética", float64(20), "hora", int64(2500), "CLP", 1, "sala",
		sql.NullString{String: "America/Santiago", Valid: true},
		sql.NullString{String: "13", Valid: true}, sql.NullString{String: "Metropolitana", Valid: true},
		sql.NullString{String: "13101", Valid: true}, sql.NullString{String: "Santiago", Valid: true},
		sql.NullString{String: "22222222-2222-4222-8222-222222222222", Valid: true},
		sql.NullString{String: "media", Valid: true}, sql.NullInt64{Int64: 7500, Valid: true}, float64(0.5), true, false,
	}}
	repo := &Repository{pool: pool}

	items, err := repo.List(context.Background(), filters, cursor, 11)
	if err != nil || len(items) != 1 {
		t.Fatalf("List() items=%#v err=%v", items, err)
	}
	item := items[0]
	if item.SpaceID != "11111111-1111-4111-8111-111111111111" || item.TimeZone != "America/Santiago" || item.EstimatedTotalCLP == nil || *item.EstimatedTotalCLP != 7500 || item.CoverImageID == nil || *item.CoverImageID != "22222222-2222-4222-8222-222222222222" || len(item.Images) != 1 || item.Images[0].Source != "media" || !item.Verification.IdentityVerified || item.Verification.SpaceVerified {
		t.Fatalf("List() mapped item=%+v", item)
	}

	wantArgs := []any{
		"oficina", "hora", &minUnit, &maxUnit, "sala", "13", "13101", minArea, 3,
		[]byte(`{"proyector":false,"wifi":true}`), start, end, &minTotal, &maxTotal,
		lastTotal, cursor.LastSpaceID,
		"altura", minHeight, maxHeight, "equipo", []string{"wifi", "proyector"}, "uso", usage, 11,
	}
	if !reflect.DeepEqual(pool.args, wantArgs) {
		t.Fatalf("List() args=\n%#v\nwant=\n%#v", pool.args, wantArgs)
	}
}

func TestListAdapterBuildsCursorPredicatesAndArgumentsForSorts(t *testing.T) {
	const spaceID = "11111111-1111-4111-8111-111111111111"
	estimatedTotal := int64(7500)
	lastCategory, lastPrice, lastRank := 2, int64(2500), float64(0.75)
	lastTitle := "sala"
	cursor := &publiccatalog.Cursor{
		LastCategoryOrder: lastCategory, LastUnitPriceCLP: lastPrice, LastEstimatedTotalCLP: &estimatedTotal,
		LastRelevanceRank: lastRank, LastTitleKey: lastTitle, LastSpaceID: spaceID,
	}
	cases := []struct {
		name, sort, predicate, order string
		cursorArgs                   []any
	}{
		{name: "unit price ascending", sort: "unit_price_asc", predicate: `AND (precio_base_clp,space_id::uuid) > ($15::bigint,$16::uuid)`, order: `ORDER BY precio_base_clp,space_id::uuid`, cursorArgs: []any{lastPrice, spaceID}},
		{name: "unit price descending", sort: "unit_price_desc", predicate: `AND (precio_base_clp,space_id::uuid) < ($15::bigint,$16::uuid)`, order: `ORDER BY precio_base_clp DESC,space_id::uuid DESC`, cursorArgs: []any{lastPrice, spaceID}},
		{name: "estimated total descending", sort: "estimated_total_desc", predicate: `AND (estimated_total < $15::bigint OR (estimated_total=$15::bigint AND space_id::uuid>$16::uuid) OR estimated_total IS NULL)`, order: `ORDER BY estimated_total DESC NULLS LAST,space_id::uuid`, cursorArgs: []any{estimatedTotal, spaceID}},
		{name: "relevance", sort: "relevance", predicate: `AND (relevance_rank < $15::real OR (relevance_rank=$15::real AND space_id::uuid>$16::uuid))`, order: `ORDER BY relevance_rank DESC,space_id::uuid`, cursorArgs: []any{lastRank, spaceID}},
		{name: "title ascending", sort: "title_asc", predicate: `AND (title_key COLLATE "C",space_id::uuid) > ($15::text COLLATE "C",$16::uuid)`, order: `ORDER BY title_key COLLATE "C",space_id::uuid`, cursorArgs: []any{lastTitle, spaceID}},
		{name: "title descending", sort: "title_desc", predicate: `AND (title_key COLLATE "C",space_id::uuid) < ($15::text COLLATE "C",$16::uuid)`, order: `ORDER BY title_key COLLATE "C" DESC,space_id::uuid DESC`, cursorArgs: []any{lastTitle, spaceID}},
		{name: "default category and title", predicate: `AND (category_order,title_key COLLATE "C",space_id::uuid) > ($15::smallint,$16::text COLLATE "C",$17::uuid)`, order: `ORDER BY category_order,title_key COLLATE "C",space_id::uuid`, cursorArgs: []any{lastCategory, lastTitle, spaceID}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pool := &coverSourcePool{row: listCursorFixtureRow()}
			repo := &Repository{pool: pool}
			if _, err := repo.List(context.Background(), publiccatalog.Filters{Sort: tc.sort}, cursor, 7); err != nil {
				t.Fatalf("List(): %v", err)
			}
			if !strings.Contains(pool.query, tc.predicate) || !strings.Contains(pool.query, tc.order) {
				t.Fatalf("query misses cursor/order contract: %s", pool.query)
			}
			cursorEnd := len(pool.args) - 1
			cursorStart := cursorEnd - len(tc.cursorArgs)
			if cursorStart < 14 || !reflect.DeepEqual(pool.args[cursorStart:cursorEnd], tc.cursorArgs) || pool.args[cursorEnd] != 7 {
				t.Fatalf("query args=%#v, cursor args=%#v limit=7", pool.args, tc.cursorArgs)
			}
		})
	}

	pool := &coverSourcePool{row: listCursorFixtureRow()}
	repo := &Repository{pool: pool}
	if _, err := repo.List(context.Background(), publiccatalog.Filters{Sort: "estimated_total_asc"}, &publiccatalog.Cursor{LastSpaceID: spaceID}, 7); err != nil {
		t.Fatalf("List() with nil last estimate: %v", err)
	}
	if !strings.Contains(pool.query, `AND estimated_total IS NULL AND space_id::uuid > $15::uuid`) || !reflect.DeepEqual(pool.args[14:], []any{spaceID, 7}) {
		t.Fatalf("nil-estimate cursor query=%q args=%#v", pool.query, pool.args)
	}
}

func listCursorFixtureRow() []any {
	return []any{
		"11111111-1111-4111-8111-111111111111", "oficina", "Oficina", "Sala", "Sala sintética", float64(20), "hora", int64(2500), "CLP", 1, "sala",
		sql.NullString{String: "America/Santiago", Valid: true},
		sql.NullString{Valid: false}, sql.NullString{Valid: false}, sql.NullString{Valid: false}, sql.NullString{Valid: false},
		sql.NullString{Valid: false}, sql.NullString{Valid: false}, sql.NullInt64{Valid: false}, float64(0), true, false,
	}
}

func TestListAdapterBuildsIntegerAttributeIntegralPredicate(t *testing.T) {
	minimum, maximum := "1", "4"
	pool := &coverSourcePool{row: listCursorFixtureRow()}
	repo := &Repository{pool: pool}
	filters := publiccatalog.Filters{
		AttributeVersion: 3,
		TypedAttributes: map[string]publiccatalog.AttributePredicate{
			"capacidad": {Type: "integer", Minimum: &minimum, Maximum: &maximum},
		},
	}
	if _, err := repo.List(context.Background(), filters, nil, 5); err != nil {
		t.Fatalf("List(): %v", err)
	}
	if !strings.Contains(pool.query, "=trunc(") || !strings.Contains(pool.query, "filter_profile_version=3") {
		t.Fatalf("integer filter query misses integral/profile predicates: %s", pool.query)
	}
	wantArgs := []any{"capacidad", minimum, maximum, 5}
	if got := pool.args[len(pool.args)-len(wantArgs):]; !reflect.DeepEqual(got, wantArgs) {
		t.Fatalf("integer filter args=%#v, want %#v", got, wantArgs)
	}
}

func TestListAdapterRejectsUnknownTypedAttributeBeforeQuery(t *testing.T) {
	pool := &coverSourcePool{row: listCursorFixtureRow()}
	repo := &Repository{pool: pool}
	filters := publiccatalog.Filters{
		TypedAttributes: map[string]publiccatalog.AttributePredicate{
			"capacidad": {Type: "untrusted-sql-type"},
		},
	}
	_, err := repo.List(context.Background(), filters, nil, 5)
	if pool.query != "" {
		t.Fatalf("unknown attribute type reached database query: %q", pool.query)
	}
	if err != publiccatalog.ErrInvalid {
		t.Fatalf("List() error=%v, want ErrInvalid", err)
	}
}
