package documentverification

import (
	"context"
	"errors"
	"testing"
)

type reviewerLoadCancellationRepository struct {
	snapshot  Snapshot
	loadCalls int
	onLoad    func()
}

func (*reviewerLoadCancellationRepository) ListReviewerCases(context.Context, string) ([]Snapshot, error) {
	return nil, nil
}

func (r *reviewerLoadCancellationRepository) LoadReviewerCase(context.Context, string, string) (Snapshot, error) {
	r.loadCalls++
	if r.onLoad != nil {
		r.onLoad()
	}
	return r.snapshot, nil
}

func (*reviewerLoadCancellationRepository) WithReviewerCaseForRead(context.Context, string, string, func(context.Context, Snapshot, Authorization) error) error {
	return ErrNotFound
}

func TestReviewerAccessLoadSkipsRepositoryWhenContextAlreadyCanceled(t *testing.T) {
	repo := &reviewerLoadCancellationRepository{snapshot: Snapshot{
		ID: "99999999-9999-4999-8999-999999999999", SubjectID: "77777777-7777-4777-8777-777777777777",
		Purpose: PurposeIdentity, State: CaseInReview, Revision: 4,
	}}
	service := newReviewerListCancelService(t, repo)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	got, err := service.Load(ctx, "66666666-6666-4666-8666-666666666666", repo.snapshot.ID)
	if !errors.Is(err, ErrDocumentStorage) || got.ID != "" || repo.loadCalls != 0 {
		t.Fatalf("snapshot=%+v err=%v load calls=%d", got, err, repo.loadCalls)
	}
}

func TestReviewerAccessLoadDiscardsSnapshotWhenContextCancelsDuringRead(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	repo := &reviewerLoadCancellationRepository{
		snapshot: Snapshot{
			ID: "99999999-9999-4999-8999-999999999999", SubjectID: "77777777-7777-4777-8777-777777777777",
			Purpose: PurposeIdentity, State: CaseInReview, Revision: 4,
		},
		onLoad: cancel,
	}
	service := newReviewerListCancelService(t, repo)

	got, err := service.Load(ctx, "66666666-6666-4666-8666-666666666666", repo.snapshot.ID)
	if !errors.Is(err, ErrDocumentStorage) || got.ID != "" || repo.loadCalls != 1 || ctx.Err() != context.Canceled {
		t.Fatalf("snapshot=%+v err=%v context err=%v load calls=%d", got, err, ctx.Err(), repo.loadCalls)
	}
}
