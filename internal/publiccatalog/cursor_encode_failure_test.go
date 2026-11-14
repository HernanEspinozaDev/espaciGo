package publiccatalog

import (
	"context"
	"math"
	"testing"
)

func TestListDoesNotReturnPartialPageWhenCursorEncodingFails(t *testing.T) {
	rows := []Space{
		{SpaceID: "00000000-0000-4000-8000-000000000001", RelevanceRank: math.NaN()},
		{SpaceID: "00000000-0000-4000-8000-000000000002", RelevanceRank: 0.5},
	}
	service, err := New(memoryRepository{rows: rows}, []byte("01234567890123456789012345678901"))
	if err != nil {
		t.Fatal(err)
	}

	page, err := service.List(context.Background(), Filters{
		PageSize: 1, PageSizeSet: true, Sort: "relevance",
	})
	if err == nil {
		t.Fatal("expected cursor encoding failure for a non-finite repository rank")
	}
	if len(page.Items) != 0 || page.NextCursor != "" {
		t.Fatalf("partial page escaped after cursor encoding failure: %+v", page)
	}
}
