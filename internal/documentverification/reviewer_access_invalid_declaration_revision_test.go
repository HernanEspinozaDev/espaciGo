package documentverification

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestReviewerIdentityDetailRejectsMalformedDeclarationRevisionBeforeRead(t *testing.T) {
	const (
		actorID      = "11111111-1111-4111-8111-111111111111"
		subjectID    = "22222222-2222-4222-8222-222222222222"
		caseID       = "33333333-3333-4333-8333-333333333333"
		revisionID   = "44444444-4444-4444-8444-444444444444"
		invalidRevID = "malformed-revision-id"
	)
	declaration := ReviewerIdentityDeclaration{
		FirstName: "Synthetic", LastName: "Reviewer Fixture", RUT: "synthetic-rut", DateOfBirth: "synthetic-date",
	}
	snapshot := Snapshot{
		ID: caseID, SubjectID: subjectID, Purpose: PurposeIdentity, State: CaseInReview,
		Revision: 4, IdentityDeclarationRevisionID: revisionID, DocumentFormatDeclared: DocumentFormatNew,
	}
	authorization := Authorization{
		ActorID:        actorID,
		ReviewerGrants: []ReviewerGrant{{ReviewerID: actorID, Capability: CapabilityIdentity, ScopeID: subjectID}},
	}
	newService := func(repo *reviewerAccessRepo, store *reviewerAccessStore, reader ReviewerIdentityDeclarationReader) *ReviewerAccessService {
		t.Helper()
		cases, err := NewService(&testRepository{})
		if err != nil {
			t.Fatal(err)
		}
		service, err := NewReviewerAccessServiceWithIdentityDeclarationReader(repo, cases, store, reader)
		if err != nil {
			t.Fatal(err)
		}
		return service
	}

	controlRepo := &reviewerAccessRepo{x: snapshot, a: authorization}
	controlStore := &reviewerAccessStore{}
	controlReaderCalls := 0
	control := newService(controlRepo, controlStore, ReviewerIdentityDeclarationReaderFunc(func(_ context.Context, owner, id string) (ReviewerIdentityDeclaration, error) {
		controlReaderCalls++
		if owner != subjectID || id != revisionID {
			t.Fatalf("control declaration scope=(%q,%q)", owner, id)
		}
		return declaration, nil
	}))
	controlDetail, err := control.LoadDetail(context.Background(), actorID, caseID)
	if err != nil || controlDetail.IdentityDeclaration == nil || *controlDetail.IdentityDeclaration != declaration || controlReaderCalls != 1 || controlRepo.readCalls != 3 {
		t.Fatalf("valid control detail=%+v reader=%d repository reads=%d err=%v", controlDetail, controlReaderCalls, controlRepo.readCalls, err)
	}

	invalidSnapshot := snapshot
	invalidSnapshot.IdentityDeclarationRevisionID = invalidRevID
	invalidRepo := &reviewerAccessRepo{x: invalidSnapshot, a: authorization}
	invalidStore := &reviewerAccessStore{}
	invalidReaderCalls := 0
	service := newService(invalidRepo, invalidStore, ReviewerIdentityDeclarationReaderFunc(func(context.Context, string, string) (ReviewerIdentityDeclaration, error) {
		invalidReaderCalls++
		return declaration, errors.New("unexpected declaration read for malformed revision")
	}))

	detail, err := service.LoadDetail(context.Background(), actorID, caseID)
	if !errors.Is(err, ErrDocumentStorage) || !reflect.DeepEqual(detail, ReviewerCaseDetail{}) || invalidRepo.readCalls != 1 || invalidReaderCalls != 0 || invalidStore.identityReads != 0 || invalidStore.spaceReads != 0 {
		t.Fatalf("malformed revision exposed detail=%+v repository reads=%d declaration reads=%d store=%+v err=%v", detail, invalidRepo.readCalls, invalidReaderCalls, invalidStore, err)
	}
}

func TestReviewerIdentityDetailRejectsMalformedActorBeforeRepositoryRead(t *testing.T) {
	const (
		actorID        = "11111111-1111-4111-8111-111111111111"
		subjectID      = "22222222-2222-4222-8222-222222222222"
		caseID         = "33333333-3333-4333-8333-333333333333"
		revisionID     = "44444444-4444-4444-8444-444444444444"
		invalidActorID = "malformed-actor-id"
	)
	declaration := ReviewerIdentityDeclaration{
		FirstName: "Synthetic", LastName: "Reviewer Fixture", RUT: "synthetic-rut", DateOfBirth: "synthetic-date",
	}
	snapshot := Snapshot{
		ID: caseID, SubjectID: subjectID, Purpose: PurposeIdentity, State: CaseInReview,
		Revision: 4, IdentityDeclarationRevisionID: revisionID, DocumentFormatDeclared: DocumentFormatNew,
	}
	authorization := Authorization{
		ActorID:        actorID,
		ReviewerGrants: []ReviewerGrant{{ReviewerID: actorID, Capability: CapabilityIdentity, ScopeID: subjectID}},
	}
	newService := func(repo *reviewerAccessRepo, store *reviewerAccessStore, reader ReviewerIdentityDeclarationReader) *ReviewerAccessService {
		t.Helper()
		cases, err := NewService(&testRepository{})
		if err != nil {
			t.Fatal(err)
		}
		service, err := NewReviewerAccessServiceWithIdentityDeclarationReader(repo, cases, store, reader)
		if err != nil {
			t.Fatal(err)
		}
		return service
	}

	controlRepo := &reviewerAccessRepo{x: snapshot, a: authorization}
	controlStore := &reviewerAccessStore{}
	controlReaderCalls := 0
	control := newService(controlRepo, controlStore, ReviewerIdentityDeclarationReaderFunc(func(context.Context, string, string) (ReviewerIdentityDeclaration, error) {
		controlReaderCalls++
		return declaration, nil
	}))
	controlDetail, err := control.LoadDetail(context.Background(), actorID, caseID)
	if err != nil || controlDetail.IdentityDeclaration == nil || *controlDetail.IdentityDeclaration != declaration || controlReaderCalls != 1 || controlRepo.readCalls != 3 {
		t.Fatalf("valid control detail=%+v reader=%d repository reads=%d err=%v", controlDetail, controlReaderCalls, controlRepo.readCalls, err)
	}

	invalidRepo := &reviewerAccessRepo{x: snapshot, a: authorization}
	invalidStore := &reviewerAccessStore{}
	invalidReaderCalls := 0
	service := newService(invalidRepo, invalidStore, ReviewerIdentityDeclarationReaderFunc(func(context.Context, string, string) (ReviewerIdentityDeclaration, error) {
		invalidReaderCalls++
		return declaration, nil
	}))

	detail, err := service.LoadDetail(context.Background(), invalidActorID, caseID)
	if !errors.Is(err, ErrInvalid) || !reflect.DeepEqual(detail, ReviewerCaseDetail{}) || invalidRepo.readCalls != 0 || invalidReaderCalls != 0 || invalidStore.identityReads != 0 || invalidStore.spaceReads != 0 {
		t.Fatalf("malformed actor exposed detail=%+v repository reads=%d declaration reads=%d store=%+v err=%v", detail, invalidRepo.readCalls, invalidReaderCalls, invalidStore, err)
	}
}
