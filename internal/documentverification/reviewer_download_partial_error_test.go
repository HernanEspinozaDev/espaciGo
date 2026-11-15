package documentverification

import (
	"context"
	"crypto/sha256"
	"errors"
	"reflect"
	"testing"
	"time"
)

type reviewerDownloadPartialErrorStore struct {
	content       []byte
	err           error
	identityReads int
	spaceReads    int
	ownerID       string
	caseID        string
	fileID        string
}

func (s *reviewerDownloadPartialErrorStore) ReadIdentityDocument(_ context.Context, ownerID, caseID, fileID string) ([]byte, error) {
	s.identityReads++
	s.ownerID, s.caseID, s.fileID = ownerID, caseID, fileID
	return s.content, s.err
}

func (s *reviewerDownloadPartialErrorStore) ReadSpaceDocument(context.Context, string, string, string, string) ([]byte, error) {
	s.spaceReads++
	return nil, ErrNotFound
}

func TestReviewerDownloadDiscardsContentReturnedWithStoreError(t *testing.T) {
	const (
		actorID   = "66666666-6666-4666-8666-666666666666"
		subjectID = "77777777-7777-4777-8777-777777777777"
		caseID    = "99999999-9999-4999-8999-999999999999"
		fileID    = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	)
	content := []byte("synthetic partial private identity document")
	now := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	repository := &reviewerAccessRepo{
		x: Snapshot{
			ID: caseID, SubjectID: subjectID, Purpose: PurposeIdentity, State: CaseInReview, Revision: 4,
			Files: []File{{
				ID: fileID, EvidenceRole: EvidenceRoleIdentityFront, State: FileClean,
				Size: int64(len(content)), MIME: "image/jpeg", ContentSHA256: sha256.Sum256(content),
				RetentionAt: now.Add(time.Hour),
			}},
		},
		a: Authorization{ActorID: actorID, ReviewerGrants: []ReviewerGrant{{
			ReviewerID: actorID, Capability: CapabilityIdentity, ScopeID: subjectID,
		}}},
	}
	wantStorageErr := errors.New("synthetic partial identity storage read")
	store := &reviewerDownloadPartialErrorStore{content: content, err: wantStorageErr}
	cases, err := NewService(&testRepository{})
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewReviewerAccessService(repository, cases, store)
	if err != nil {
		t.Fatal(err)
	}
	service.SetClock(func() time.Time { return now })

	got, err := service.Download(context.Background(), actorID, caseID, fileID)
	if !errors.Is(err, ErrDocumentStorage) || !reflect.DeepEqual(got, DownloadedDocument{}) {
		t.Fatalf("download=%+v err=%v, want zero document and ErrDocumentStorage", got, err)
	}
	if store.identityReads != 1 || store.spaceReads != 0 || store.ownerID != subjectID || store.caseID != caseID || store.fileID != fileID {
		t.Fatalf("storage scope/calls=(%q,%q,%q)/(%d,%d), want one identity read of exact owner/case/file", store.ownerID, store.caseID, store.fileID, store.identityReads, store.spaceReads)
	}
	if repository.readCalls != 1 {
		t.Fatalf("repository checks=%d, want one authorized case read", repository.readCalls)
	}
}
