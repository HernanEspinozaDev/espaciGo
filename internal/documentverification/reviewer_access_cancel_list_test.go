package documentverification

import (
	"context"
	"errors"
	"testing"
)

type reviewerListCancelRepository struct {
	cases     []Snapshot
	listCalls int
	onList    func()
}

func (r *reviewerListCancelRepository) ListReviewerCases(context.Context, string) ([]Snapshot, error) {
	r.listCalls++
	if r.onList != nil {
		r.onList()
	}
	return r.cases, nil
}

func (r *reviewerListCancelRepository) LoadReviewerCase(context.Context, string, string) (Snapshot, error) {
	return Snapshot{}, ErrNotFound
}

func (r *reviewerListCancelRepository) WithReviewerCaseForRead(context.Context, string, string, func(context.Context, Snapshot, Authorization) error) error {
	return ErrNotFound
}

func newReviewerListCancelService(t *testing.T, repo ReviewerCaseReader) *ReviewerAccessService {
	t.Helper()
	cases, err := NewService(&testRepository{})
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewReviewerAccessService(repo, cases, &reviewerAccessStore{})
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func TestReviewerAccessListSkipsRepositoryWhenContextAlreadyCanceled(t *testing.T) {
	repo := &reviewerListCancelRepository{cases: []Snapshot{{
		ID: "99999999-9999-4999-8999-999999999999", SubjectID: "77777777-7777-4777-8777-777777777777",
		Purpose: PurposeIdentity, State: CaseInReview, Revision: 4,
	}}}
	service := newReviewerListCancelService(t, repo)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cases, err := service.List(ctx, "66666666-6666-4666-8666-666666666666")
	if !errors.Is(err, ErrDocumentStorage) || len(cases) != 0 || repo.listCalls != 0 {
		t.Fatalf("cases=%+v err=%v list calls=%d", cases, err, repo.listCalls)
	}
}

func TestReviewerAccessListReturnsNoCasesWhenContextCancelsAfterRead(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	repo := &reviewerListCancelRepository{
		cases: []Snapshot{{
			ID: "99999999-9999-4999-8999-999999999999", SubjectID: "77777777-7777-4777-8777-777777777777",
			Purpose: PurposeIdentity, State: CaseInReview, Revision: 4,
		}},
		onList: cancel,
	}
	service := newReviewerListCancelService(t, repo)

	cases, err := service.List(ctx, "66666666-6666-4666-8666-666666666666")
	if !errors.Is(err, ErrDocumentStorage) || len(cases) != 0 || repo.listCalls != 1 || ctx.Err() != context.Canceled {
		t.Fatalf("cases=%+v err=%v ctx err=%v list calls=%d", cases, err, ctx.Err(), repo.listCalls)
	}
}
