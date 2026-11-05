package spaces

import (
	"context"
	"errors"
	"testing"
	"time"
)

const revisionOwner = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
const revisionID = "11111111-1111-4111-8111-111111111111"

type revisionRepo struct {
	Repository
	expected time.Time
	input    Input
	owner    string
	id       string
	called   bool
	result   Draft
	err      error
}

func (r *revisionRepo) GetOwn(context.Context, string, string) (Draft, error) {
	return Draft{CategoryCode: "oficina", AttributeSchemaVersion: 3}, nil
}
func (r *revisionRepo) Profile(_ context.Context, category string, version int) (Profile, error) {
	if category != "oficina" || version != 3 {
		return Profile{}, ErrNotFound
	}
	return Profile{CategoryCode: category, SchemaVersion: 3}, nil
}
func (r *revisionRepo) UpdateOwnIfRevision(_ context.Context, owner, id string, expected time.Time, input Input) (Draft, error) {
	r.owner, r.id, r.called = owner, id, true
	r.expected, r.input = expected, input
	return r.result, r.err
}

func TestUpdateOwnIfRevisionPassesNormalizedInputAndExpectedTimestamp(t *testing.T) {
	revision := time.Date(2026, 4, 2, 3, 4, 5, 123, time.FixedZone("offset", 3600))
	want := Draft{ID: revisionID, Title: "Actualizado"}
	repo := &revisionRepo{result: want}
	svc, err := NewService(repo)
	if err != nil {
		t.Fatal(err)
	}
	in := validRevisionInput()
	got, err := svc.UpdateOwnIfRevision(context.Background(), revisionOwner, revisionID, revision, in)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != want.ID || got.Title != want.Title {
		t.Fatalf("got %#v, want %#v", got, want)
	}
	if !repo.called || repo.owner != revisionOwner || repo.id != revisionID {
		t.Fatalf("repository call got called=%v owner=%q id=%q", repo.called, repo.owner, repo.id)
	}
	if !repo.expected.Equal(revision) || repo.expected != revision {
		t.Fatalf("timestamp changed: got %#v want %#v", repo.expected, revision)
	}
	if repo.input.AttributeSchemaVersion != 3 || repo.input.Attributes == nil {
		t.Fatalf("input not normalized: %#v", repo.input)
	}
}

func TestUpdateOwnIfRevisionRejectsInvalidInputWithoutRepositoryCall(t *testing.T) {
	repo := &revisionRepo{}
	svc, err := NewService(repo)
	if err != nil {
		t.Fatal(err)
	}
	input := validRevisionInput()
	input.Title = ""

	_, err = svc.UpdateOwnIfRevision(context.Background(), revisionOwner, revisionID, time.Now(), input)
	if err == nil || !errors.Is(err, ErrInvalid) {
		t.Fatalf("expected validation error, got %v", err)
	}
	if repo.called {
		t.Fatal("version-aware repository method was called for invalid input")
	}
}

func TestUpdateOwnIfRevisionPropagatesPreconditionFailure(t *testing.T) {
	repo := &revisionRepo{err: ErrPreconditionFailed}
	svc, err := NewService(repo)
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.UpdateOwnIfRevision(context.Background(), revisionOwner, revisionID, time.Now(), validRevisionInput())
	if !errors.Is(err, ErrPreconditionFailed) {
		t.Fatalf("got %v", err)
	}
}

type legacyRevisionRepo struct{ Repository }

func (legacyRevisionRepo) GetOwn(context.Context, string, string) (Draft, error) {
	return Draft{CategoryCode: "oficina", AttributeSchemaVersion: 3}, nil
}
func (legacyRevisionRepo) Profile(_ context.Context, category string, version int) (Profile, error) {
	return Profile{CategoryCode: category, SchemaVersion: version}, nil
}

func TestUpdateOwnIfRevisionFailsClosedWithoutVersionedRepository(t *testing.T) {
	svc, err := NewService(legacyRevisionRepo{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.UpdateOwnIfRevision(context.Background(), revisionOwner, revisionID, time.Now(), validRevisionInput())
	if err == nil || errors.Is(err, ErrInvalid) {
		t.Fatalf("expected internal error, got %v", err)
	}
}

func validRevisionInput() Input {
	input := validInput()
	input.Title = "Actualizado"
	return input
}
