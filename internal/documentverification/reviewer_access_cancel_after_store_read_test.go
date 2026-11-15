package documentverification

import (
	"context"
	"crypto/sha256"
	"errors"
	"testing"
	"time"
)

type reviewerAccessCancelAfterReadStore struct {
	reviewerAccessStore
	cancel context.CancelFunc
}

func (s *reviewerAccessCancelAfterReadStore) ReadIdentityDocument(_ context.Context, ownerID, caseID, fileID string) ([]byte, error) {
	s.identityReads++
	s.owner, s.caseID, s.fileID = ownerID, caseID, fileID
	s.cancel()
	return s.content, nil
}

func TestReviewerDownloadFailsClosedWhenContextCancelsAfterStoreRead(t *testing.T) {
	const (
		actorID   = "66666666-6666-4666-8666-666666666666"
		subjectID = "77777777-7777-4777-8777-777777777777"
		caseID    = "99999999-9999-4999-8999-999999999999"
		fileID    = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	)
	content := []byte("synthetic private identity evidence")
	repo := &reviewerAccessRepo{
		x: Snapshot{
			ID: caseID, SubjectID: subjectID, Purpose: PurposeIdentity, State: CaseInReview,
			Files: []File{{
				ID: fileID, State: FileClean, Size: int64(len(content)), MIME: "image/png",
				ContentSHA256: sha256.Sum256(content), RetentionAt: time.Date(2031, 1, 1, 0, 0, 0, 0, time.UTC),
			}},
		},
		a: Authorization{
			ActorID: actorID,
			ReviewerGrants: []ReviewerGrant{{
				ReviewerID: actorID, Capability: CapabilityIdentity, ScopeID: subjectID,
			}},
		},
	}
	cases, err := NewService(&testRepository{})
	if err != nil {
		t.Fatal(err)
	}

	controlStore := &reviewerAccessStore{content: content}
	control, err := NewReviewerAccessService(repo, cases, controlStore)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := control.Download(context.Background(), actorID, caseID, fileID); err != nil || string(got.Content) != string(content) {
		t.Fatalf("valid control fixture did not download: bytes=%q err=%v", got.Content, err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	store := &reviewerAccessCancelAfterReadStore{reviewerAccessStore: reviewerAccessStore{content: content}, cancel: cancel}
	access, err := NewReviewerAccessService(repo, cases, store)
	if err != nil {
		t.Fatal(err)
	}
	got, err := access.Download(ctx, actorID, caseID, fileID)
	if !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatalf("store fake did not cancel the request context: %v", ctx.Err())
	}
	if !errors.Is(err, ErrDocumentStorage) || len(got.Content) != 0 || got.MIME != "" {
		t.Fatalf("canceled download exposed a result: bytes=%q MIME=%q err=%v", got.Content, got.MIME, err)
	}
	if store.identityReads != 1 {
		t.Fatalf("identity store reads=%d, want exactly one", store.identityReads)
	}
}

type reviewerAccessCancelAfterCallbackRepo struct {
	*reviewerAccessRepo
	cancel          context.CancelFunc
	cancelAfterCall int
}

func (r *reviewerAccessCancelAfterCallbackRepo) WithReviewerCaseForRead(ctx context.Context, actor, id string, read func(context.Context, Snapshot, Authorization) error) error {
	err := r.reviewerAccessRepo.WithReviewerCaseForRead(ctx, actor, id, read)
	if err == nil && (r.cancelAfterCall == 0 || r.reviewerAccessRepo.readCalls == r.cancelAfterCall) {
		r.cancel()
	}
	return err
}

func TestReviewerDownloadFailsClosedWhenContextCancelsAfterReaderReturns(t *testing.T) {
	const (
		actorID   = "66666666-6666-4666-8666-666666666666"
		subjectID = "77777777-7777-4777-8777-777777777777"
		caseID    = "99999999-9999-4999-8999-999999999999"
		fileID    = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	)
	content := []byte("synthetic private identity evidence")
	baseRepo := &reviewerAccessRepo{
		x: Snapshot{
			ID: caseID, SubjectID: subjectID, Purpose: PurposeIdentity, State: CaseInReview,
			Files: []File{{
				ID: fileID, State: FileClean, Size: int64(len(content)), MIME: "image/png",
				ContentSHA256: sha256.Sum256(content), RetentionAt: time.Date(2031, 1, 1, 0, 0, 0, 0, time.UTC),
			}},
		},
		a: Authorization{
			ActorID: actorID,
			ReviewerGrants: []ReviewerGrant{{
				ReviewerID: actorID, Capability: CapabilityIdentity, ScopeID: subjectID,
			}},
		},
	}
	cases, err := NewService(&testRepository{})
	if err != nil {
		t.Fatal(err)
	}

	controlStore := &reviewerAccessStore{content: content}
	control, err := NewReviewerAccessService(baseRepo, cases, controlStore)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := control.Download(context.Background(), actorID, caseID, fileID); err != nil || string(got.Content) != string(content) {
		t.Fatalf("valid control fixture did not download: bytes=%q err=%v", got.Content, err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	repo := &reviewerAccessCancelAfterCallbackRepo{reviewerAccessRepo: baseRepo, cancel: cancel}
	store := &reviewerAccessStore{content: content}
	access, err := NewReviewerAccessService(repo, cases, store)
	if err != nil {
		t.Fatal(err)
	}
	got, err := access.Download(ctx, actorID, caseID, fileID)
	if !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatalf("repository fake did not cancel the context after the callback: %v", ctx.Err())
	}
	if !errors.Is(err, ErrDocumentStorage) || len(got.Content) != 0 || got.MIME != "" {
		t.Fatalf("canceled download exposed a result: bytes=%q MIME=%q err=%v", got.Content, got.MIME, err)
	}
	if store.identityReads != 1 {
		t.Fatalf("identity store reads=%d, want exactly one", store.identityReads)
	}
}
