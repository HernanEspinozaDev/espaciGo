package documentverification

import (
	"context"
	"crypto/sha256"
	"errors"
	"testing"
	"time"
)

func TestReviewerDownloadRejectsContentWithMismatchedSizeOrDigest(t *testing.T) {
	const (
		actor   = "66666666-6666-4666-8666-666666666666"
		subject = "77777777-7777-4777-8777-777777777777"
		caseID  = "99999999-9999-4999-8999-999999999999"
		fileID  = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	)
	expected := []byte("synthetic expected identity evidence")
	differentDigest := append([]byte(nil), expected...)
	differentDigest[len(differentDigest)-1] ^= 1
	for _, tc := range []struct {
		name                string
		bytes               []byte
		wantSameSize        bool
		wantDifferentDigest bool
	}{
		{name: "size mismatch", bytes: []byte("different-length payload"), wantDifferentDigest: true},
		{name: "digest mismatch", bytes: differentDigest, wantSameSize: true, wantDifferentDigest: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if sameSize := len(tc.bytes) == len(expected); sameSize != tc.wantSameSize {
				t.Fatalf("fixture same-size=%t want=%t", sameSize, tc.wantSameSize)
			}
			if differentDigest := sha256.Sum256(tc.bytes) != sha256.Sum256(expected); differentDigest != tc.wantDifferentDigest {
				t.Fatalf("fixture different-digest=%t want=%t", differentDigest, tc.wantDifferentDigest)
			}
			repo := &reviewerAccessRepo{
				x: Snapshot{
					ID: caseID, SubjectID: subject, Purpose: PurposeIdentity, State: CaseInReview,
					Files: []File{{
						ID: fileID, State: FileClean, Size: int64(len(expected)), MIME: "image/png",
						ContentSHA256: sha256.Sum256(expected), RetentionAt: time.Date(2031, 1, 1, 0, 0, 0, 0, time.UTC),
					}},
				},
				a: Authorization{ActorID: actor, ReviewerGrants: []ReviewerGrant{{ReviewerID: actor, Capability: CapabilityIdentity, ScopeID: subject}}},
			}
			store := &reviewerAccessStore{content: tc.bytes}
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
			if !errors.Is(err, ErrDocumentStorage) || len(got.Content) != 0 || got.MIME != "" || store.identityReads != 1 || store.spaceReads != 0 {
				t.Fatalf("mismatched evidence was returned or routed incorrectly: content=%q mime=%q reads=(identity:%d space:%d) err=%v", got.Content, got.MIME, store.identityReads, store.spaceReads, err)
			}
		})
	}
}
