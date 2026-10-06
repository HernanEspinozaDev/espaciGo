package booking

import "testing"

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
