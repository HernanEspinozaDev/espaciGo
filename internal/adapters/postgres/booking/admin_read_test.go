package bookingpg

import (
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/booking"
)

func TestAdminReservationCursorBindsAccountFiltersAndPageSize(t *testing.T) {
	actor := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	id := "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	from1 := time.Date(2029, 12, 31, 21, 0, 0, 0, time.FixedZone("offset", -3*60*60))
	from2 := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	f1 := booking.AdminReservationFilter{ID: id, State: "pagada", CreatedFrom: &from1}
	f2 := booking.AdminReservationFilter{ID: id, State: "pagada", CreatedFrom: &from2}
	if adminFilterHash(f1) != adminFilterHash(f2) {
		t.Fatal("equivalent instants should normalize to the same cursor filter")
	}
	encoded := encodeAdminCursor(adminReservationCursor{Version: 1, Actor: actor, Filter: adminFilterHash(f1), CreatedAt: time.Date(2030, 1, 2, 0, 0, 0, 0, time.UTC), ID: id, PageSize: 25})
	if _, err := decodeAdminCursor(encoded, actor, adminFilterHash(f2), 25); err != nil {
		t.Fatalf("valid cursor rejected: %v", err)
	}
	for _, tc := range []struct {
		actor, filter string
		size          int
	}{{"cccccccc-cccc-4ccc-8ccc-cccccccccccc", adminFilterHash(f1), 25}, {actor, "different", 25}, {actor, adminFilterHash(f1), 10}} {
		if _, err := decodeAdminCursor(encoded, tc.actor, tc.filter, tc.size); err == nil {
			t.Errorf("accepted cursor with actor=%s filter=%s size=%d", tc.actor, tc.filter, tc.size)
		}
	}
	if _, err := decodeAdminCursor("not-base64", actor, adminFilterHash(f1), 25); err == nil {
		t.Fatal("accepted malformed cursor")
	}
}
