package documentverification

import (
	"context"
	"crypto/sha256"
	"errors"
	"testing"
	"time"
)

func TestReviewerDownloadRejectsSupersededCaseBeforeReadingStore(t *testing.T) {
	const (
		actor   = "66666666-6666-4666-8666-666666666666"
		subject = "77777777-7777-4777-8777-777777777777"
		caseID  = "99999999-9999-4999-8999-999999999999"
		fileID  = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	)
	body := []byte("synthetic superseded identity evidence")
	supersededAt := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	repo := &reviewerAccessRepo{
		x: Snapshot{
			ID: caseID, SubjectID: subject, Purpose: PurposeIdentity, State: CaseInReview,
			SupersededAt: &supersededAt,
			Files: []File{{
				ID: fileID, State: FileClean, Size: int64(len(body)), MIME: "image/png",
				ContentSHA256: sha256.Sum256(body), RetentionAt: time.Date(2031, 1, 1, 0, 0, 0, 0, time.UTC),
			}},
		},
		a: Authorization{ActorID: actor, ReviewerGrants: []ReviewerGrant{{ReviewerID: actor, Capability: CapabilityIdentity, ScopeID: subject}}},
	}
	store := &reviewerAccessStore{content: body}
	cases, err := NewService(&testRepository{})
	if err != nil {
		t.Fatal(err)
	}
	access, err := NewReviewerAccessService(repo, cases, store)
	if err != nil {
		t.Fatal(err)
	}
	access.SetClock(func() time.Time { return time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC) })

	got, err := access.Download(context.Background(), actor, caseID, fileID)
	if !errors.Is(err, ErrNotFound) || len(got.Content) != 0 || got.MIME != "" || store.identityReads != 0 || store.spaceReads != 0 {
		t.Fatalf("superseded case evidence reached store: content=%q mime=%q reads=(identity:%d space:%d) err=%v", got.Content, got.MIME, store.identityReads, store.spaceReads, err)
	}
}
