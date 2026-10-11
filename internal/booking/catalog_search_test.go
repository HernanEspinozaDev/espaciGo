package booking

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestSortCatalogByRawDistanceThenEstimateThenID(t *testing.T) {
	items := []CatalogItem{
		{SpaceID: "dddddddd-dddd-4ddd-8ddd-dddddddddddd", DistanceMeters: 1000.9, EstimatedTotal: int64PtrTest(6000)},
		{SpaceID: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", DistanceMeters: 1000.0, EstimatedTotal: int64PtrTest(9000)},
		{SpaceID: "cccccccc-cccc-4ccc-8ccc-cccccccccccc", DistanceMeters: 1000.0, EstimatedTotal: int64PtrTest(8000)},
		{SpaceID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", DistanceMeters: 1000.0, EstimatedTotal: int64PtrTest(8000)},
	}
	sortCatalogByGeo(items)
	want := []string{
		"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
		"cccccccc-cccc-4ccc-8ccc-cccccccccccc",
		"bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb",
		"dddddddd-dddd-4ddd-8ddd-dddddddddddd",
	}
	for i := range want {
		if items[i].SpaceID != want[i] {
			t.Fatalf("sorted[%d]=%s, want %s; full order=%+v", i, items[i].SpaceID, want[i], items)
		}
	}
}

func int64PtrTest(v int64) *int64 { return &v }

func TestCatalogCursorFingerprintCanonicalizesEquivalentTimesAndMapKeys(t *testing.T) {
	startA := time.Date(2035, 1, 1, 12, 0, 0, 0, time.FixedZone("one", 3600))
	endA := startA.Add(time.Hour)
	startB, endB := startA.UTC(), endA.UTC()
	a, err := catalogFilterFingerprint(CatalogFilter{StartAt: &startA, EndAt: &endA, Attributes: map[string]any{"alpha": true, "beta": "x"}}, 5, "price")
	if err != nil {
		t.Fatal(err)
	}
	b, err := catalogFilterFingerprint(CatalogFilter{StartAt: &startB, EndAt: &endB, Attributes: map[string]any{"beta": "x", "alpha": true}}, 5, "price")
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Fatalf("equivalent filters fingerprint differs: %s %s", a, b)
	}
}

func TestCatalogCursorFingerprintBindsSearchTextAndMinimumArea(t *testing.T) {
	area := 42.5
	a, err := catalogFilterFingerprint(CatalogFilter{SearchText: "  bodega   Norte ", MinAreaM2: &area}, 5, "category")
	if err != nil {
		t.Fatal(err)
	}
	b, err := catalogFilterFingerprint(CatalogFilter{SearchText: "bodega Norte", MinAreaM2: &area}, 5, "category")
	if err != nil || a != b {
		t.Fatalf("whitespace-normalized search fingerprints differ: %q %q err=%v", a, b, err)
	}
	otherArea := 42.51
	c, err := catalogFilterFingerprint(CatalogFilter{SearchText: "bodega Norte", MinAreaM2: &otherArea}, 5, "category")
	if err != nil || a == c {
		t.Fatalf("minimum area did not change cursor binding: %q %q err=%v", a, c, err)
	}
	d, err := catalogFilterFingerprint(CatalogFilter{SearchText: "bodega sur", MinAreaM2: &area}, 5, "category")
	if err != nil || a == d {
		t.Fatalf("search text did not change cursor binding: %q %q err=%v", a, d, err)
	}
}

func TestSyntheticMapPositionsAreRelativeAndDoNotExposeCoordinates(t *testing.T) {
	items := []CatalogItem{
		{SpaceID: "a", HasMapPoint: true, MapLongitude: -70.67, MapLatitude: -33.45},
		{SpaceID: "b", HasMapPoint: true, MapLongitude: -70.66, MapLatitude: -33.46},
		{SpaceID: "c"},
	}
	assignSyntheticMapPositions(items)
	if items[0].MapPosition == nil || items[0].MapPosition.X != 0 || items[0].MapPosition.Y != 0 || items[1].MapPosition == nil || items[1].MapPosition.X != 100 || items[1].MapPosition.Y != 100 || items[2].MapPosition != nil {
		t.Fatalf("synthetic map positions=%+v", items)
	}
	raw, err := json.Marshal(items)
	if err != nil || strings.Contains(string(raw), "latitude") || strings.Contains(string(raw), "longitude") || strings.Contains(string(raw), "MapLongitude") || strings.Contains(string(raw), "-70.67") {
		t.Fatalf("serialized synthetic map exposed stored coordinates: %s err=%v", raw, err)
	}
}
