package publiccatalogpg

import (
	"context"
	"database/sql"
	"testing"

	"github.com/HernanEspinozaDev/espaciGo/internal/publiccatalog"
)

func TestListAdapterKeepsAbsentPublicMetadataNil(t *testing.T) {
	pool := &coverSourcePool{row: []any{
		"11111111-1111-4111-8111-111111111111", "oficina", "Oficina", "Sala", "Descripción sintética", float64(20), "dia", int64(5000), "CLP", 1, "sala",
		sql.NullString{},
		sql.NullString{}, sql.NullString{}, sql.NullString{}, sql.NullString{},
		sql.NullString{}, sql.NullString{String: "media", Valid: true}, sql.NullInt64{}, float64(0), true, false,
	}}
	repo := &Repository{pool: pool}
	items, err := repo.List(context.Background(), publiccatalog.Filters{}, nil, 5)
	if err != nil || len(items) != 1 {
		t.Fatalf("List() items=%#v err=%v", items, err)
	}
	item := items[0]
	if item.TimeZone != "" || item.RegionCode != nil || item.RegionName != nil || item.CommuneCode != nil || item.CommuneName != nil || item.CoverImageID != nil || item.EstimatedTotalCLP != nil {
		t.Fatalf("absent metadata became populated: %+v", item)
	}
	if item.Images == nil || len(item.Images) != 0 || item.Features == nil || len(item.Features) != 0 {
		t.Fatalf("empty public collections must be non-nil: images=%#v features=%#v", item.Images, item.Features)
	}
}
