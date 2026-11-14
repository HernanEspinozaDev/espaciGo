package publiccatalog

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type detailRecorder struct {
	memoryRepository
	calls   int
	spaceID string
	result  Space
	err     error
}

func (r *detailRecorder) Detail(_ context.Context, spaceID string) (Space, error) {
	r.calls++
	r.spaceID = spaceID
	return r.result, r.err
}

func TestDetailRejectsMalformedSpaceIDBeforeRepositoryRead(t *testing.T) {
	repo := &detailRecorder{}
	service, err := New(repo, []byte("01234567890123456789012345678901"))
	if err != nil {
		t.Fatal(err)
	}

	for _, spaceID := range []string{"", "not-a-uuid", "11111111-1111-4111-8111-11111111111x"} {
		t.Run(spaceID, func(t *testing.T) {
			_, err := service.Detail(context.Background(), spaceID)
			if !errors.Is(err, ErrNotFound) {
				t.Fatalf("Detail(%q) error=%v, want ErrNotFound", spaceID, err)
			}
			if repo.calls != 0 {
				t.Fatalf("malformed ID reached repository %d times", repo.calls)
			}
		})
	}
}

func TestDetailCanonicalizesSpaceIDBeforeRepositoryRead(t *testing.T) {
	const canonicalID = "abcdefab-cdef-4abc-8def-abcdefabcdef"
	const suppliedID = "ABCDEFAB-CDEF-4ABC-8DEF-ABCDEFABCDEF"
	repo := &detailRecorder{result: Space{SpaceID: canonicalID, Title: "Sala sintética"}}
	service, err := New(repo, []byte("01234567890123456789012345678901"))
	if err != nil {
		t.Fatal(err)
	}

	got, err := service.Detail(context.Background(), suppliedID)
	if err != nil {
		t.Fatalf("Detail(%q): %v", suppliedID, err)
	}
	if repo.calls != 1 || repo.spaceID != strings.ToLower(suppliedID) {
		t.Fatalf("repository calls=%d ID=%q, want one read with canonical ID %q", repo.calls, repo.spaceID, canonicalID)
	}
	if got.SpaceID != canonicalID || got.Title != "Sala sintética" {
		t.Fatalf("detail=%+v, want the synthetic public listing", got)
	}
}
