package publiccatalog

import (
	"context"
	"strings"
	"testing"
)

func TestListRejectsOversizedSignedCursorBeforeRepository(t *testing.T) {
	repo := &discoveryRepository{}
	service := newDiscoveryService(t, repo)
	filters := Filters{PageSize: DefaultPageSize, Sort: "default"}
	cursor := Cursor{
		Version: cursorVersion, Fingerprint: filtersFingerprint(filters), Sort: filters.Sort,
		PageSize: filters.PageSize, LastTitleKey: strings.Repeat("x", 5000),
		LastSpaceID: "00000000-0000-4000-8000-000000000001",
	}
	token, err := service.encodeCursor(cursor)
	if err != nil || len(token) <= 4096 {
		t.Fatalf("fixture cursor length=%d error=%v, want valid signed token >4096 bytes", len(token), err)
	}
	filters.Cursor = token
	if _, err := service.List(context.Background(), filters); err != ErrInvalid || repo.listCalls != 0 {
		t.Fatalf("List() error=%v calls=%d, want ErrInvalid/0", err, repo.listCalls)
	}
}
