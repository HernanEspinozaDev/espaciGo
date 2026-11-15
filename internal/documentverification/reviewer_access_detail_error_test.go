package documentverification

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestReviewerIdentityDetailDiscardsDeclarationReturnedWithError(t *testing.T) {
	const (
		actorID    = "11111111-1111-4111-8111-111111111111"
		subjectID  = "22222222-2222-4222-8222-222222222222"
		caseID     = "33333333-3333-4333-8333-333333333333"
		revisionID = "44444444-4444-4444-8444-444444444444"
	)
	declaration := ReviewerIdentityDeclaration{
		FirstName: "Synthetic-Partial-Name", LastName: "Synthetic-Partial-Surname",
		RUT: "Synthetic-RUT-Fixture", DateOfBirth: "2040-02-03",
	}
	wantErr := errors.New("synthetic declaration read failure")
	repository := &reviewerAccessRepo{
		x: Snapshot{
			ID: caseID, SubjectID: subjectID, Purpose: PurposeIdentity, State: CaseInReview, Revision: 4,
			IdentityDeclarationRevisionID: revisionID, DocumentFormatDeclared: DocumentFormatNew,
		},
		a: Authorization{ActorID: actorID, ReviewerGrants: []ReviewerGrant{{
			ReviewerID: actorID, Capability: CapabilityIdentity, ScopeID: subjectID,
		}}},
	}
	store := &reviewerAccessStore{}
	cases, err := NewService(&testRepository{})
	if err != nil {
		t.Fatal(err)
	}
	declarationReads := 0
	service, err := NewReviewerAccessServiceWithIdentityDeclarationReader(
		repository, cases, store,
		ReviewerIdentityDeclarationReaderFunc(func(ctx context.Context, accountID, id string) (ReviewerIdentityDeclaration, error) {
			declarationReads++
			if err := ctx.Err(); err != nil {
				return ReviewerIdentityDeclaration{}, err
			}
			if accountID != subjectID || id != revisionID {
				t.Fatalf("declaration scope=(%q,%q), want subject and exact revision", accountID, id)
			}
			return declaration, wantErr
		}),
	)
	if err != nil {
		t.Fatal(err)
	}

	detail, err := service.LoadDetail(context.Background(), actorID, caseID)
	if !errors.Is(err, ErrDocumentStorage) || !reflect.DeepEqual(detail, ReviewerCaseDetail{}) ||
		declarationReads != 1 || repository.readCalls != 2 || store.identityReads != 0 || store.spaceReads != 0 {
		t.Fatalf("partial declaration escaped: detail=%+v reads=%d repository checks=%d store=%+v err=%v",
			detail, declarationReads, repository.readCalls, store, err)
	}
}
