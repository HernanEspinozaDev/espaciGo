package gallery

import (
	"context"
	"errors"
	"testing"
	"time"
)

type casRepo struct {
	Repository
	got     time.Time
	enabled bool
	calls   int
	err     error
}

func (r *casRepo) SetPublicInCatalogCAS(_ context.Context, _, _, _ string, enabled bool, expected time.Time) (Photo, time.Time, error) {
	r.calls++
	r.got = expected
	r.enabled = enabled
	return Photo{}, expected, r.err
}

type stubRepo struct{}

func (stubRepo) ReserveCandidate(context.Context, string, string, string, time.Time) error {
	return nil
}
func (stubRepo) BeginCandidate(context.Context, string, string, string) (CandidateWriter, error) {
	return nil, nil
}
func (stubRepo) QueueCandidateCleanup(context.Context, string, time.Time) error { return nil }
func (stubRepo) ClaimCandidateCleanup(context.Context, int) ([]string, error)   { return nil, nil }
func (stubRepo) CompleteCandidateCleanup(context.Context, string) error         { return nil }
func (stubRepo) FailCandidateCleanup(context.Context, string, time.Time) error  { return nil }
func (stubRepo) List(context.Context, string, string) ([]Photo, error)          { return nil, nil }
func (stubRepo) Get(context.Context, string, string, string) (Photo, error)     { return Photo{}, nil }
func (stubRepo) Remove(context.Context, string, string, string) (Removal, error) {
	return Removal{}, nil
}
func (stubRepo) SelectCover(context.Context, string, string, string) (Cover, error) {
	return Cover{}, nil
}
func (stubRepo) SetPublicInCatalog(context.Context, string, string, string, bool) (Photo, error) {
	return Photo{}, nil
}
func (stubRepo) PendingCleanup(context.Context, int) ([]string, error)    { return nil, nil }
func (stubRepo) CompleteCleanup(context.Context, string, time.Time) error { return nil }
func (stubRepo) FailCleanup(context.Context, string, time.Time) error     { return nil }
func testService(t *testing.T, repo Repository) *Service {
	t.Helper()
	s, e := NewService(repo, nilFileStore{}, ids{}, time.Now)
	if e != nil {
		t.Fatal(e)
	}
	return s
}

type nilFileStore struct{}

func (nilFileStore) Put(context.Context, string, []byte) error   { return nil }
func (nilFileStore) Get(context.Context, string) ([]byte, error) { return nil, nil }
func (nilFileStore) Delete(context.Context, string) error        { return nil }

type ids struct{}

func (ids) ID() (string, error) { return "", nil }

const ownerID = "123e4567-e89b-42d3-a456-426614174000"

func TestSetPublicInCatalogCASPassesCanonicalRevision(t *testing.T) {
	rev := "2026-10-28T12:00:00Z"
	repo := &casRepo{}
	s := testService(t, repo)
	_, got, err := s.SetPublicInCatalogCAS(context.Background(), ownerID, ownerID, ownerID, true, rev)
	if err != nil || got.UTC().Format(time.RFC3339Nano) != rev || !repo.enabled || repo.got.UTC().Format(time.RFC3339Nano) != rev {
		t.Fatalf("CAS result/revision: %v %v %v", got, repo, err)
	}
}
func TestSetPublicInCatalogCASRejectsWhitespacePaddedIDs(t *testing.T) {
	cases := []struct {
		name, owner, spaceID, photoID string
	}{
		{name: "owner", owner: " " + ownerID, spaceID: ownerID, photoID: ownerID},
		{name: "space", owner: ownerID, spaceID: ownerID + " ", photoID: ownerID},
		{name: "photo", owner: ownerID, spaceID: ownerID, photoID: "\t" + ownerID},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &casRepo{}
			s := testService(t, repo)
			_, _, err := s.SetPublicInCatalogCAS(context.Background(), tc.owner, tc.spaceID, tc.photoID, true, "2026-10-28T12:00:00Z")
			if !errors.Is(err, ErrNotFound) {
				t.Fatalf("got error %v; want %v", err, ErrNotFound)
			}
			if repo.calls != 0 {
				t.Fatalf("repository CAS calls=%d; want 0 for invalid UUID token", repo.calls)
			}
		})
	}
}

func TestSetPublicInCatalogCASRejectsUnsupportedRepository(t *testing.T) {
	s := testService(t, stubRepo{})
	_, _, err := s.SetPublicInCatalogCAS(context.Background(), ownerID, ownerID, ownerID, true, "2026-10-28T12:00:00Z")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("got %v", err)
	}
}
