package documentverification

import (
	"context"
	"crypto/sha256"
	"errors"
	"testing"
	"time"
)

func TestReviewerSpaceDownloadRejectsUntrustedSpaceOwnerBeforeStoreRead(t *testing.T) {
	const (
		actor   = "66666666-6666-4666-8666-666666666666"
		subject = "77777777-7777-4777-8777-777777777777"
		other   = "88888888-8888-4888-8888-888888888888"
		spaceID = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
		caseID  = "99999999-9999-4999-8999-999999999999"
		fileID  = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	)
	body := []byte("synthetic assigned space evidence")
	file := File{
		ID: fileID, State: FileClean, Size: int64(len(body)), MIME: "application/pdf",
		ContentSHA256: sha256.Sum256(body), RetentionAt: time.Date(2031, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	for _, tc := range []struct {
		name       string
		spaceOwner string
	}{
		{name: "reviewer is also the space owner", spaceOwner: actor},
		{name: "authorization owner differs from case subject", spaceOwner: other},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repository := &reviewerAccessRepo{
				x: Snapshot{
					ID: caseID, SubjectID: subject, SpaceID: spaceID, Purpose: PurposeSpace,
					State: CaseInReview, Files: []File{file},
				},
				a: Authorization{
					ActorID: actor, SpaceOwnerID: tc.spaceOwner,
					ReviewerGrants: []ReviewerGrant{{ReviewerID: actor, Capability: CapabilitySpace, ScopeID: spaceID}},
				},
			}
			store := &reviewerAccessStore{content: body}
			cases, err := NewService(&testRepository{})
			if err != nil {
				t.Fatal(err)
			}
			service, err := NewReviewerAccessService(repository, cases, store)
			if err != nil {
				t.Fatal(err)
			}
			service.SetClock(func() time.Time { return time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC) })

			got, err := service.Download(context.Background(), actor, caseID, fileID)
			if !errors.Is(err, ErrNotFound) || len(got.Content) != 0 || got.MIME != "" {
				t.Fatalf("untrusted space-owner download=%+v err=%v, want empty document and ErrNotFound", got, err)
			}
			if repository.readCalls != 1 || store.spaceReads != 0 || store.identityReads != 0 {
				t.Fatalf("repository reads=%d store identity/space=%d/%d, want one scoped check and no document read", repository.readCalls, store.identityReads, store.spaceReads)
			}
		})
	}
}
