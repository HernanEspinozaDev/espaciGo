package publiccatalog

import (
	"context"
	"errors"
	"testing"
)

type listErrorRepository struct {
	memoryRepository
	failure error
	calls   int
}

func (r *listErrorRepository) List(context.Context, Filters, *Cursor, int) ([]Space, error) {
	r.calls++
	return []Space{{SpaceID: "00000000-0000-4000-8000-000000000001", Title: "synthetic partial result"}}, r.failure
}

func TestListDiscardsPartialRowsWhenRepositoryReturnsError(t *testing.T) {
	failure := errors.New("synthetic catalog query failure")
	repo := &listErrorRepository{failure: failure}
	service, err := New(repo, []byte("01234567890123456789012345678901"))
	if err != nil {
		t.Fatal(err)
	}
	page, err := service.List(context.Background(), Filters{})
	if !errors.Is(err, failure) || len(page.Items) != 0 || page.NextCursor != "" || repo.calls != 1 {
		t.Fatalf("List() page=%+v error=%v repository_calls=%d", page, err, repo.calls)
	}
}
