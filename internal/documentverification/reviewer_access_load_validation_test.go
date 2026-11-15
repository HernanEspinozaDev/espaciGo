package documentverification

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

type reviewerLoadValidationRepository struct {
	*reviewerAccessRepo
	loadCalls int
	actor     string
	caseID    string
	loadErr   error
}

func (r *reviewerLoadValidationRepository) LoadReviewerCase(_ context.Context, actor, caseID string) (Snapshot, error) {
	r.loadCalls++
	r.actor, r.caseID = actor, caseID
	return r.x, r.loadErr
}

func TestReviewerAccessLoadValidatesActorBeforeRepositoryRead(t *testing.T) {
	const (
		actorID        = "11111111-1111-4111-8111-111111111111"
		subjectID      = "22222222-2222-4222-8222-222222222222"
		caseID         = "33333333-3333-4333-8333-333333333333"
		invalidActorID = "malformed-actor-id"
	)
	snapshot := Snapshot{
		ID: caseID, SubjectID: subjectID, Purpose: PurposeIdentity, State: CaseInReview, Revision: 4,
	}
	repository := &reviewerLoadValidationRepository{reviewerAccessRepo: &reviewerAccessRepo{x: snapshot}}
	cases, err := NewService(&testRepository{})
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewReviewerAccessService(repository, cases, &reviewerAccessStore{})
	if err != nil {
		t.Fatal(err)
	}

	got, err := service.Load(context.Background(), actorID, caseID)
	if err != nil || !reflect.DeepEqual(got, snapshot) || repository.loadCalls != 1 || repository.actor != actorID || repository.caseID != caseID {
		t.Fatalf("valid load=%+v calls=%d actor=%q case=%q err=%v", got, repository.loadCalls, repository.actor, repository.caseID, err)
	}

	repository.loadCalls = 0
	repository.actor, repository.caseID = "", ""
	got, err = service.Load(context.Background(), invalidActorID, caseID)
	if !errors.Is(err, ErrInvalid) || !reflect.DeepEqual(got, Snapshot{}) || repository.loadCalls != 0 || repository.actor != "" || repository.caseID != "" {
		t.Fatalf("invalid actor load=%+v calls=%d actor=%q case=%q err=%v", got, repository.loadCalls, repository.actor, repository.caseID, err)
	}
}

func TestReviewerAccessLoadPropagatesRepositoryErrorWithoutSnapshot(t *testing.T) {
	const (
		actorID   = "11111111-1111-4111-8111-111111111111"
		subjectID = "22222222-2222-4222-8222-222222222222"
		caseID    = "33333333-3333-4333-8333-333333333333"
	)
	snapshot := Snapshot{
		ID: caseID, SubjectID: subjectID, Purpose: PurposeIdentity, State: CaseInReview, Revision: 4,
	}
	repository := &reviewerLoadValidationRepository{reviewerAccessRepo: &reviewerAccessRepo{x: snapshot}}
	cases, err := NewService(&testRepository{})
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewReviewerAccessService(repository, cases, &reviewerAccessStore{})
	if err != nil {
		t.Fatal(err)
	}

	control, err := service.Load(context.Background(), actorID, caseID)
	if err != nil || !reflect.DeepEqual(control, snapshot) || repository.loadCalls != 1 {
		t.Fatalf("valid control=%+v calls=%d err=%v", control, repository.loadCalls, err)
	}

	wantErr := errors.New("synthetic reviewer case read failure")
	repository.loadErr = wantErr
	repository.loadCalls = 0
	repository.actor, repository.caseID = "", ""
	got, err := service.Load(context.Background(), actorID, caseID)
	if !errors.Is(err, wantErr) || !reflect.DeepEqual(got, Snapshot{}) || repository.loadCalls != 1 || repository.actor != actorID || repository.caseID != caseID {
		t.Fatalf("failed load=%+v calls=%d actor=%q case=%q err=%v", got, repository.loadCalls, repository.actor, repository.caseID, err)
	}
}
