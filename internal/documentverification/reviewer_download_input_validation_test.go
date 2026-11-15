package documentverification

import (
	"context"
	"errors"
	"testing"
)

func TestReviewerDownloadRejectsInvalidInputsBeforeRepositoryOrStorage(t *testing.T) {
	const (
		actor  = "66666666-6666-4666-8666-666666666666"
		caseID = "99999999-9999-4999-8999-999999999999"
		fileID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	)
	for _, tc := range []struct {
		name   string
		actor  string
		caseID string
		fileID string
	}{
		{name: "invalid actor", actor: "invalid-actor-id", caseID: caseID, fileID: fileID},
		{name: "invalid case ID", actor: actor, caseID: "invalid-case-id", fileID: fileID},
		{name: "invalid file ID", actor: actor, caseID: caseID, fileID: "invalid-file-id"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repository := &reviewerAccessRepo{}
			store := &reviewerAccessStore{}
			cases, err := NewService(&testRepository{})
			if err != nil {
				t.Fatal(err)
			}
			service, err := NewReviewerAccessService(repository, cases, store)
			if err != nil {
				t.Fatal(err)
			}

			got, err := service.Download(context.Background(), tc.actor, tc.caseID, tc.fileID)
			if !errors.Is(err, ErrInvalid) || len(got.Content) != 0 || got.MIME != "" {
				t.Fatalf("invalid input download=%+v err=%v, want empty document and ErrInvalid", got, err)
			}
			if repository.readCalls != 0 || store.identityReads != 0 || store.spaceReads != 0 {
				t.Fatalf("invalid input reached repository/storage: repository reads=%d store identity/space=%d/%d", repository.readCalls, store.identityReads, store.spaceReads)
			}
		})
	}
}

func TestNilReviewerAccessServiceDownloadReturnsErrInvalid(t *testing.T) {
	const (
		actor  = "66666666-6666-4666-8666-666666666666"
		caseID = "99999999-9999-4999-8999-999999999999"
		fileID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	)
	var service *ReviewerAccessService
	got, err := service.Download(context.Background(), actor, caseID, fileID)
	if !errors.Is(err, ErrInvalid) || len(got.Content) != 0 || got.MIME != "" {
		t.Fatalf("nil service download=%+v err=%v, want empty document and ErrInvalid", got, err)
	}
}
