package media

import (
	"context"
	"errors"
	"testing"
	"time"
)

const (
	removeCASTestOwner = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	removeCASTestSpace = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	removeCASTestPhoto = "cccccccc-cccc-4ccc-8ccc-cccccccccccc"
)

type removeSpaceCASRepositoryStub struct {
	Repository
	current time.Time
	next    time.Time
	removed bool
}

func (r *removeSpaceCASRepositoryStub) RemoveSpacePhotoCAS(_ context.Context, _, _, _ string, expected, _ time.Time) (time.Time, error) {
	if !expected.Equal(r.current) {
		return time.Time{}, ErrPrecondition
	}
	r.removed = true
	return r.next, nil
}

type removeSpaceCASStoreStub struct{ ObjectStore }
type removeSpaceCASIDsStub struct{}

func (removeSpaceCASIDsStub) ID() (string, error) { return removeCASTestPhoto, nil }

func newRemoveSpaceCASTestService(t *testing.T, repo *removeSpaceCASRepositoryStub) *Service {
	t.Helper()
	svc, err := NewService(repo, &removeSpaceCASStoreStub{}, removeSpaceCASIDsStub{}, func() time.Time { return repo.next })
	if err != nil {
		t.Fatal(err)
	}
	return svc
}

func TestRemoveSpaceCASReturnsRenewedRevision(t *testing.T) {
	current := time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC)
	next := current.Add(time.Second)
	repo := &removeSpaceCASRepositoryStub{current: current, next: next}
	svc := newRemoveSpaceCASTestService(t, repo)

	updated, err := svc.RemoveSpaceCAS(context.Background(), removeCASTestOwner, removeCASTestSpace, removeCASTestPhoto, current.Format(time.RFC3339Nano))
	if err != nil {
		t.Fatal(err)
	}
	if !repo.removed || !updated.Equal(next) {
		t.Fatalf("removed=%v updated=%s; want removed and %s", repo.removed, updated.Format(time.RFC3339Nano), next.Format(time.RFC3339Nano))
	}
}

func TestRemoveSpaceCASStaleRevisionDoesNotMutate(t *testing.T) {
	current := time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC)
	repo := &removeSpaceCASRepositoryStub{current: current, next: current.Add(time.Second)}
	svc := newRemoveSpaceCASTestService(t, repo)

	_, err := svc.RemoveSpaceCAS(context.Background(), removeCASTestOwner, removeCASTestSpace, removeCASTestPhoto, current.Add(-time.Second).Format(time.RFC3339Nano))
	if !errors.Is(err, ErrPrecondition) {
		t.Fatalf("err=%v; want ErrPrecondition", err)
	}
	if repo.removed {
		t.Fatal("stale delete mutated the media photo")
	}
}
