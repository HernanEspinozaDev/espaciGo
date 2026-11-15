package publiccatalog

import (
	"context"
	"strings"
	"testing"
)

type cursorCountingRepository struct {
	memoryRepository
	listCalls int
}

func (r *cursorCountingRepository) List(ctx context.Context, filters Filters, after *Cursor, limit int) ([]Space, error) {
	r.listCalls++
	return r.memoryRepository.List(ctx, filters, after, limit)
}

func TestListRejectsTamperedCursorBeforeRepositoryRead(t *testing.T) {
	repo := &cursorCountingRepository{memoryRepository: memoryRepository{rows: []Space{
		{SpaceID: "00000000-0000-4000-8000-000000000001", CategoryOrder: 1, TitleKey: "a"},
		{SpaceID: "00000000-0000-4000-8000-000000000002", CategoryOrder: 1, TitleKey: "b"},
	}}}
	service, err := New(repo, []byte("01234567890123456789012345678901"))
	if err != nil {
		t.Fatal(err)
	}
	filters := Filters{PageSize: 1, PageSizeSet: true}
	first, err := service.List(context.Background(), filters)
	if err != nil || first.NextCursor == "" {
		t.Fatalf("first page cursor=%q error=%v", first.NextCursor, err)
	}
	if repo.listCalls != 1 {
		t.Fatalf("initial page repository calls=%d, want 1", repo.listCalls)
	}

	separator := strings.LastIndex(first.NextCursor, ".")
	if separator < 0 || separator == len(first.NextCursor)-1 {
		t.Fatalf("generated cursor has no signature segment: %q", first.NextCursor)
	}
	mutated := []byte(first.NextCursor)
	if mutated[separator+1] == 'A' {
		mutated[separator+1] = 'B'
	} else {
		mutated[separator+1] = 'A'
	}
	filters.Cursor = string(mutated)
	if _, err := service.List(context.Background(), filters); err != ErrInvalid {
		t.Fatalf("tampered cursor error=%v, want ErrInvalid", err)
	}
	if repo.listCalls != 1 {
		t.Fatalf("tampered cursor reached repository: calls=%d, want 1 total", repo.listCalls)
	}
}
