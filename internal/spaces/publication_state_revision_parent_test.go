package spaces

import (
	"context"
	"errors"
	"testing"
	"time"
)

type publicationRevisionProbe struct {
	Repository
	called                        bool
	owner, id, state, correlation string
	expected                      time.Time
	err                           error
}

func (r *publicationRevisionProbe) SetPublicationStateIfRevision(_ context.Context, owner, id, state, correlation string, expected time.Time) (Draft, error) {
	r.called = true
	r.owner = owner
	r.id = id
	r.state = state
	r.correlation = correlation
	r.expected = expected
	return Draft{ID: id, State: state, UpdatedAt: expected.Add(time.Microsecond)}, r.err
}
func TestPublicationRevisionServicePassesIdentityAndRevision(t *testing.T) {
	r := &publicationRevisionProbe{}
	s, _ := NewService(r)
	rev := time.Date(2026, 10, 28, 12, 0, 0, 0, time.FixedZone("offset", 3600))
	d, e := s.SetPublicationStateIfRevision(context.Background(), revisionOwner, revisionID, "activa", "synthetic-correlation", rev)
	if e != nil || !r.called || r.owner != revisionOwner || r.id != revisionID || r.state != "activa" || r.correlation != "synthetic-correlation" || !r.expected.Equal(rev) || r.expected.Location() != time.UTC || d.State != "activa" {
		t.Fatalf("incorrect CAS delegation: result=%+v error=%v probe=%+v", d, e, r)
	}
}
func TestPublicationRevisionServiceFailsClosedWithoutVersionedRepository(t *testing.T) {
	s, _ := NewService(legacyRevisionRepo{})
	_, e := s.SetPublicationStateIfRevision(context.Background(), revisionOwner, revisionID, "activa", "synthetic", time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC))
	if e == nil {
		t.Fatal("legacy writer used without CAS")
	}
}
func TestPublicationRevisionServicePropagatesStaleAndRejectsZero(t *testing.T) {
	r := &publicationRevisionProbe{err: ErrPreconditionFailed}
	s, _ := NewService(r)
	_, e := s.SetPublicationStateIfRevision(context.Background(), revisionOwner, revisionID, "activa", "synthetic", time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC))
	if !errors.Is(e, ErrPreconditionFailed) {
		t.Fatalf("got %v", e)
	}
	r.called = false
	_, e = s.SetPublicationStateIfRevision(context.Background(), revisionOwner, revisionID, "activa", "synthetic", time.Time{})
	if !errors.Is(e, ErrInvalid) || r.called {
		t.Fatal("zero revision reached repository")
	}
}
