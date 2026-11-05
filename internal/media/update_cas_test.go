package media

import (
	"context"
	"errors"
	"testing"
	"time"
)

type updateCASTestRepo struct {
	Repository
	photo    Photo
	revision time.Time
	called   bool
}

func (r *updateCASTestRepo) UpdateSpacePhotoCAS(_ context.Context, _, _, _ string, alt *string, visible *bool, position *int, expected, at time.Time) (Photo, time.Time, error) {
	r.called = true
	if !expected.Equal(r.revision) {
		return Photo{}, time.Time{}, ErrPrecondition
	}
	if alt != nil {
		r.photo.AltText = *alt
	}
	if visible != nil {
		r.photo.Visible = *visible
	}
	if position != nil {
		r.photo.Position = *position
	}
	r.revision = at
	return r.photo, at, nil
}

type updateCASTestStore struct{ ObjectStore }
type updateCASTestIDs struct{}

func (updateCASTestIDs) ID() (string, error) { return "", nil }

func TestUpdateSpaceCASUsesExpectedRevisionAndReturnsRenewedPhoto(t *testing.T) {
	current := time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC)
	next := current.Add(time.Second)
	photo := Photo{ID: removeCASTestPhoto, SpaceID: removeCASTestSpace, Position: 1}
	repo := &updateCASTestRepo{photo: photo, revision: current}
	svc, err := NewService(repo, updateCASTestStore{}, updateCASTestIDs{}, func() time.Time { return next })
	if err != nil {
		t.Fatal(err)
	}
	alt := "Updated"
	visible := false
	position := 2
	got, revision, err := svc.UpdateSpaceCAS(context.Background(), removeCASTestOwner, removeCASTestSpace, removeCASTestPhoto, &alt, &visible, &position, current.Format(time.RFC3339Nano))
	if err != nil {
		t.Fatal(err)
	}
	if !repo.called || got.AltText != alt || got.Visible || got.Position != position || !revision.Equal(next) {
		t.Fatalf("called=%v photo=%+v revision=%s", repo.called, got, revision)
	}
}

func TestUpdateSpaceCASStaleRevisionDoesNotMutate(t *testing.T) {
	current := time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC)
	repo := &updateCASTestRepo{photo: Photo{ID: removeCASTestPhoto, Position: 1}, revision: current}
	svc, err := NewService(repo, updateCASTestStore{}, updateCASTestIDs{}, func() time.Time { return current.Add(time.Second) })
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = svc.UpdateSpaceCAS(context.Background(), removeCASTestOwner, removeCASTestSpace, removeCASTestPhoto, nil, nil, nil, current.Add(-time.Second).Format(time.RFC3339Nano))
	if !errors.Is(err, ErrPrecondition) {
		t.Fatalf("err=%v; want ErrPrecondition", err)
	}
	if repo.photo.Position != 1 {
		t.Fatal("stale CAS mutated photo")
	}
}
