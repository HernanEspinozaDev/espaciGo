package spaces

import (
	"context"
	"errors"
	"testing"
	"time"
)

type contentRevisionProbe struct {
	Repository
	called   bool
	expected time.Time
	err      error
}

func (r *contentRevisionProbe) UpdatePublishedOwnIfRevision(_ context.Context, owner, id string, expected time.Time, in PublishedContentInput) (Draft, error) {
	r.called = true
	r.expected = expected
	return Draft{ID: id}, r.err
}
func TestPublishedContentRevisionRejectsZeroBeforeRepository(t *testing.T) {
	r := &contentRevisionProbe{}
	s, _ := NewService(r)
	title := "Valid title"
	_, err := s.UpdatePublishedOwnIfRevision(context.Background(), revisionOwner, revisionID, time.Time{}, PublishedContentInput{Title: &title})
	if !errors.Is(err, ErrInvalid) || r.called {
		t.Fatalf("zero reached repo=%t error=%v", r.called, err)
	}
}
func TestPublishedContentRevisionPropagatesStale(t *testing.T) {
	r := &contentRevisionProbe{err: ErrPreconditionFailed}
	s, _ := NewService(r)
	title := "Valid title"
	now := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	_, err := s.UpdatePublishedOwnIfRevision(context.Background(), revisionOwner, revisionID, now, PublishedContentInput{Title: &title})
	if !errors.Is(err, ErrPreconditionFailed) || !r.called || !r.expected.Equal(now) {
		t.Fatalf("incorrect delegate %+v %v", r, err)
	}
}
func TestPublishedContentRevisionNoUnversionedFallback(t *testing.T) {
	s, _ := NewService(legacyRevisionRepo{})
	title := "Valid title"
	_, err := s.UpdatePublishedOwnIfRevision(context.Background(), revisionOwner, revisionID, time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC), PublishedContentInput{Title: &title})
	if err == nil {
		t.Fatal("accepted legacy unversioned writer")
	}
}
