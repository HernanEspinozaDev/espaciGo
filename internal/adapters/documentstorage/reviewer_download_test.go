package documentstorage

import (
	"context"
	"crypto/sha256"
	"errors"
	"testing"
	"time"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
)

const (
	reviewerDownloadActorID = "88888888-8888-4888-8888-888888888888"
	reviewerDownloadOwnerID = "11111111-1111-4111-8111-111111111111"
	reviewerDownloadCaseID  = "33333333-3333-4333-8333-333333333333"
	reviewerDownloadFileID  = "44444444-4444-4444-8444-444444444444"
)

type reviewerDownloadDomainRepository struct{}

func (reviewerDownloadDomainRepository) CreateCase(context.Context, string, dv.Purpose, string, func(dv.Authorization) (dv.Snapshot, error)) (dv.Snapshot, error) {
	return dv.Snapshot{}, errors.New("unexpected CreateCase")
}

func (reviewerDownloadDomainRepository) Transition(context.Context, string, dv.Purpose, string, uint64, dv.TransitionAudit, func(*dv.Snapshot, dv.Authorization) error) error {
	return errors.New("unexpected Transition")
}

type reviewerDownloadReader struct {
	snapshot dv.Snapshot
	auth     dv.Authorization
}

func (r reviewerDownloadReader) ListReviewerCases(context.Context, string) ([]dv.Snapshot, error) {
	return nil, nil
}

func (r reviewerDownloadReader) LoadReviewerCase(context.Context, string, string) (dv.Snapshot, error) {
	return r.snapshot, nil
}

func (r reviewerDownloadReader) WithReviewerCaseForRead(ctx context.Context, actor, caseID string, read func(context.Context, dv.Snapshot, dv.Authorization) error) error {
	if actor != reviewerDownloadActorID || caseID != reviewerDownloadCaseID {
		return dv.ErrNotFound
	}
	return read(ctx, r.snapshot, r.auth)
}

func TestReviewerDownloadWithEncryptedStoreFailsClosedOnInFlightCancellation(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2031, 4, 5, 12, 0, 0, 0, time.UTC)
	payload := []byte("synthetic identity evidence for reviewer download")
	blobs := &memoryBlobs{}
	store, err := New(blobs, testKey)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.PutIdentityDocument(ctx, reviewerDownloadOwnerID, reviewerDownloadCaseID, reviewerDownloadFileID, payload); err != nil {
		t.Fatal(err)
	}

	reader := reviewerDownloadReader{
		snapshot: dv.Snapshot{
			ID: reviewerDownloadCaseID, SubjectID: reviewerDownloadOwnerID,
			Purpose: dv.PurposeIdentity, State: dv.CaseSubmitted,
			Files: []dv.File{{
				ID: reviewerDownloadFileID, State: dv.FileClean, Size: int64(len(payload)),
				MIME: "image/png", ContentSHA256: sha256.Sum256(payload), RetentionAt: now.Add(time.Hour),
			}},
		},
		auth: dv.Authorization{
			ActorID: reviewerDownloadActorID,
			ReviewerGrants: []dv.ReviewerGrant{{
				ReviewerID: reviewerDownloadActorID, Capability: dv.CapabilityIdentity, ScopeID: reviewerDownloadOwnerID,
			}},
		},
	}
	cases, err := dv.NewService(reviewerDownloadDomainRepository{})
	if err != nil {
		t.Fatal(err)
	}
	access, err := dv.NewReviewerAccessService(reader, cases, store)
	if err != nil {
		t.Fatal(err)
	}
	access.SetClock(func() time.Time { return now })

	// Positive control: the real reviewer service accepts the exact synthetic grant
	// and the encrypted adapter returns only the authenticated plaintext.
	got, err := access.Download(ctx, reviewerDownloadActorID, reviewerDownloadCaseID, reviewerDownloadFileID)
	if err != nil || string(got.Content) != string(payload) || got.MIME != "image/png" {
		t.Fatalf("valid reviewer download=%q MIME=%q err=%v", got.Content, got.MIME, err)
	}

	readCtx, cancel := context.WithCancel(ctx)
	blobs.getHook = cancel
	got, err = access.Download(readCtx, reviewerDownloadActorID, reviewerDownloadCaseID, reviewerDownloadFileID)
	if !errors.Is(err, dv.ErrDocumentStorage) {
		t.Fatalf("canceled reviewer download error=%v, want neutral storage failure", err)
	}
	if len(got.Content) != 0 || got.MIME != "" {
		t.Fatalf("canceled reviewer download exposed content: bytes=%d MIME=%q", len(got.Content), got.MIME)
	}
}
