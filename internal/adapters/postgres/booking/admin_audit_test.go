package bookingpg

import (
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/booking"
)

func TestAdminAuditCursorBindsAccountFiltersPageSizeAndSnapshot(t *testing.T) {
	filter := booking.AdminAuditFilter{From: time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC), Until: time.Date(2030, 1, 2, 0, 0, 0, 0, time.UTC), Action: "identity.credencial_cambiar"}
	hash := auditFilterHash(filter)
	cursor := auditCursor{Version: 1, Actor: "11111111-1111-4111-8111-111111111111", Filter: hash, PageSize: 25, SnapshotAt: filter.Until, LastAt: filter.From, LastID: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"}
	encoded, err := encodeAuditCursor(cursor)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeAuditCursor(encoded, cursor.Actor, hash, 25); err != nil {
		t.Fatalf("valid cursor rejected: %v", err)
	}
	for _, tc := range []struct {
		name, actor, filter string
		size                int
	}{{"account", "22222222-2222-4222-8222-222222222222", hash, 25}, {"filters", cursor.Actor, "different", 25}, {"page size", cursor.Actor, hash, 10}} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := decodeAuditCursor(encoded, tc.actor, tc.filter, tc.size); err == nil {
				t.Fatal("cursor accepted with changed binding")
			}
		})
	}
	if _, err := decodeAuditCursor("not-a-url-token", cursor.Actor, hash, 25); err == nil {
		t.Fatal("malformed cursor accepted")
	}
}
