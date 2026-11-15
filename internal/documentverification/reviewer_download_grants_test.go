package documentverification

import (
	"context"
	"crypto/sha256"
	"errors"
	"testing"
	"time"
)

func TestReviewerDownloadRejectsInvalidGrantsBeforeReadingStore(t *testing.T) {
	const (
		actor   = "66666666-6666-4666-8666-666666666666"
		subject = "77777777-7777-4777-8777-777777777777"
		other   = "88888888-8888-4888-8888-888888888888"
		caseID  = "99999999-9999-4999-8999-999999999999"
		fileID  = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	)
	body := []byte("clean synthetic identity evidence")
	validFile := File{
		ID: fileID, State: FileClean, Size: int64(len(body)), MIME: "image/png",
		ContentSHA256: sha256.Sum256(body), RetentionAt: time.Date(2031, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	for _, tc := range []struct {
		name          string
		subjectID     string
		authorization Authorization
	}{
		{name: "missing grant", subjectID: subject, authorization: Authorization{ActorID: actor}},
		{name: "foreign reviewer grant", subjectID: subject, authorization: Authorization{ActorID: actor, ReviewerGrants: []ReviewerGrant{{ReviewerID: other, Capability: CapabilityIdentity, ScopeID: subject}}}},
		{name: "wrong scope", subjectID: subject, authorization: Authorization{ActorID: actor, ReviewerGrants: []ReviewerGrant{{ReviewerID: actor, Capability: CapabilityIdentity, ScopeID: other}}}},
		{name: "wrong capability", subjectID: subject, authorization: Authorization{ActorID: actor, ReviewerGrants: []ReviewerGrant{{ReviewerID: actor, Capability: CapabilitySpace, ScopeID: subject}}}},
		{name: "dual-purpose grants", subjectID: subject, authorization: Authorization{ActorID: actor, ReviewerGrants: []ReviewerGrant{{ReviewerID: actor, Capability: CapabilityIdentity, ScopeID: subject}, {ReviewerID: actor, Capability: CapabilitySpace, ScopeID: other}}}},
		{name: "self review", subjectID: actor, authorization: Authorization{ActorID: actor, ReviewerGrants: []ReviewerGrant{{ReviewerID: actor, Capability: CapabilityIdentity, ScopeID: actor}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &reviewerAccessRepo{
				x: Snapshot{ID: caseID, SubjectID: tc.subjectID, Purpose: PurposeIdentity, State: CaseInReview, Files: []File{validFile}},
				a: tc.authorization,
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
				t.Fatalf("invalid grant reached private store: content=%q mime=%q reads=(identity:%d space:%d) err=%v", got.Content, got.MIME, store.identityReads, store.spaceReads, err)
			}
		})
	}
}
