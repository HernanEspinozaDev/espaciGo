package documentverification

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestReviewerIdentityDetailDoesNotReadDeclarationAfterGrantRevocation(t *testing.T) {
	const (
		actorID    = "11111111-1111-4111-8111-111111111111"
		subjectID  = "22222222-2222-4222-8222-222222222222"
		caseID     = "33333333-3333-4333-8333-333333333333"
		revisionID = "44444444-4444-4444-8444-444444444444"
	)
	declaration := ReviewerIdentityDeclaration{
		FirstName:   "Synthetic",
		LastName:    "Assigned Reviewer",
		RUT:         "synthetic-rut",
		DateOfBirth: "1990-02-03",
	}
	newService := func(repo *reviewerAccessRepo, reads *int) *ReviewerAccessService {
		t.Helper()
		cases, err := NewService(&testRepository{})
		if err != nil {
			t.Fatal(err)
		}
		reader := ReviewerIdentityDeclarationReaderFunc(func(_ context.Context, accountID, id string) (ReviewerIdentityDeclaration, error) {
			*reads++
			if accountID != subjectID || id != revisionID {
				t.Fatalf("declaration reader scope=(%q,%q)", accountID, id)
			}
			return declaration, nil
		})
		service, err := NewReviewerAccessServiceWithIdentityDeclarationReader(repo, cases, &reviewerAccessStore{}, reader)
		if err != nil {
			t.Fatal(err)
		}
		return service
	}
	newRepo := func(revokeAfterFirst bool) *reviewerAccessRepo {
		return &reviewerAccessRepo{
			x: Snapshot{
				ID: caseID, SubjectID: subjectID, Purpose: PurposeIdentity,
				State: CaseInReview, Revision: 4,
				IdentityDeclarationRevisionID: revisionID,
				DocumentFormatDeclared:        DocumentFormatNew,
			},
			a: Authorization{
				ActorID: actorID,
				ReviewerGrants: []ReviewerGrant{{
					ReviewerID: actorID, Capability: CapabilityIdentity, ScopeID: subjectID,
				}},
			},
			revokeAfterFirst: revokeAfterFirst,
		}
	}

	t.Run("current assignment permits allowlisted declaration", func(t *testing.T) {
		repo := newRepo(false)
		readerCalls := 0
		detail, err := newService(repo, &readerCalls).LoadDetail(context.Background(), actorID, caseID)
		if err != nil || detail.IdentityDeclaration == nil || *detail.IdentityDeclaration != declaration || readerCalls != 1 || repo.readCalls != 3 {
			t.Fatalf("detail=%+v reader calls=%d authorization reads=%d err=%v", detail, readerCalls, repo.readCalls, err)
		}
	})

	t.Run("revocation between checks prevents declaration read", func(t *testing.T) {
		repo := newRepo(true)
		readerCalls := 0
		detail, err := newService(repo, &readerCalls).LoadDetail(context.Background(), actorID, caseID)
		if !errors.Is(err, ErrNotFound) || !reflect.DeepEqual(detail, ReviewerCaseDetail{}) || readerCalls != 0 || repo.readCalls != 2 {
			t.Fatalf("revoked assignment exposed detail=%+v reader calls=%d authorization reads=%d err=%v", detail, readerCalls, repo.readCalls, err)
		}
	})
}
