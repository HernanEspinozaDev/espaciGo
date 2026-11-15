package documentverification

import (
	"context"
	"crypto/sha256"
	"errors"
	"testing"
	"time"
)

func TestReviewerDownloadRejectsExpiredRetentionBeforeReadingStore(t *testing.T) {
	const (
		actor   = "66666666-6666-4666-8666-666666666666"
		subject = "77777777-7777-4777-8777-777777777777"
		caseID  = "99999999-9999-4999-8999-999999999999"
		fileID  = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	)
	now := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	body := []byte("clean synthetic identity evidence")
	for _, tc := range []struct {
		name        string
		retentionAt time.Time
	}{
		{name: "expired before now", retentionAt: now.Add(-time.Nanosecond)},
		{name: "expires exactly now", retentionAt: now},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &reviewerAccessRepo{
				x: Snapshot{
					ID: caseID, SubjectID: subject, Purpose: PurposeIdentity, State: CaseInReview,
					Files: []File{{
						ID: fileID, State: FileClean, Size: int64(len(body)), MIME: "image/png",
						ContentSHA256: sha256.Sum256(body), RetentionAt: tc.retentionAt,
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
			access.SetClock(func() time.Time { return now })

			got, err := access.Download(context.Background(), actor, caseID, fileID)
			if !errors.Is(err, ErrNotFound) || len(got.Content) != 0 || got.MIME != "" || store.identityReads != 0 || store.spaceReads != 0 {
				t.Fatalf("expired evidence reached store: content=%q mime=%q reads=(identity:%d space:%d) err=%v", got.Content, got.MIME, store.identityReads, store.spaceReads, err)
			}
		})
	}
}
