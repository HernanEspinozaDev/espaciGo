package documentverification

import (
	"context"
	"errors"
	"testing"
)

type reviewerAccessListErrorReader struct {
	*reviewerAccessRepo
	err   error
	actor string
	calls int
}

func (r *reviewerAccessListErrorReader) ListReviewerCases(_ context.Context, actor string) ([]Snapshot, error) {
	r.calls++
	r.actor = actor
	return []Snapshot{{ID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"}}, r.err
}

func TestReviewerAccessListDiscardsPartialResultsOnRepositoryError(t *testing.T) {
	const actor = "66666666-6666-4666-8666-666666666666"
	wantErr := errors.New("synthetic reviewer list failure")
	repository := &reviewerAccessListErrorReader{
		reviewerAccessRepo: &reviewerAccessRepo{},
		err:                wantErr,
	}
	cases, err := NewService(&testRepository{})
	if err != nil {
		t.Fatal(err)
	}
	store := &reviewerAccessStore{}
	service, err := NewReviewerAccessService(repository, cases, store)
	if err != nil {
		t.Fatal(err)
	}

	got, err := service.List(context.Background(), actor)
	if !errors.Is(err, wantErr) || got != nil {
		t.Fatalf("cases=%v err=%v, want nil result and original repository error", got, err)
	}
	if repository.calls != 1 || repository.actor != actor {
		t.Fatalf("reader calls=%d actor=%q, want one call for authenticated reviewer", repository.calls, repository.actor)
	}
	if store.identityReads != 0 || store.spaceReads != 0 {
		t.Fatalf("document reads identity/space=%d/%d, want none for list error", store.identityReads, store.spaceReads)
	}
}

func TestReviewerAccessListRejectsInvalidActorBeforeReadingRepository(t *testing.T) {
	repository := &reviewerAccessListErrorReader{reviewerAccessRepo: &reviewerAccessRepo{}}
	cases, err := NewService(&testRepository{})
	if err != nil {
		t.Fatal(err)
	}
	store := &reviewerAccessStore{}
	service, err := NewReviewerAccessService(repository, cases, store)
	if err != nil {
		t.Fatal(err)
	}

	got, err := service.List(context.Background(), "invalid-actor-id")
	if !errors.Is(err, ErrInvalid) || got != nil {
		t.Fatalf("cases=%v err=%v, want nil result and ErrInvalid", got, err)
	}
	if repository.calls != 0 || store.identityReads != 0 || store.spaceReads != 0 {
		t.Fatalf("reader calls=%d document reads identity/space=%d/%d, want none", repository.calls, store.identityReads, store.spaceReads)
	}
}
