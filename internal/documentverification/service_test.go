package documentverification

import (
	"context"
	"errors"
	"testing"
	"time"
)

const actor = "11111111-1111-4111-8111-111111111111"
const subject = "22222222-2222-4222-8222-222222222222"
const space = "33333333-3333-4333-8333-333333333333"
const caseID = "55555555-5555-4555-8555-555555555555"
const fileID = "44444444-4444-4444-8444-444444444444"

type memoryRepository struct {
	state Snapshot
	auth  Authorization
	err   error
	calls int
}

func (r *memoryRepository) CreateCase(_ context.Context, p Purpose, sid string, build func(Authorization) (Snapshot, error)) error {
	r.calls++
	if r.err != nil {
		return r.err
	}
	x, e := build(r.auth)
	if e == nil {
		r.state = x
	}
	return e
}
func (r *memoryRepository) Transition(_ context.Context, p Purpose, id string, rev uint64, fn func(*Snapshot, Authorization) error) error {
	r.calls++
	if r.err != nil {
		return r.err
	}
	if r.state.Purpose != p || r.state.ID != id {
		return ErrNotFound
	}
	if r.state.Revision != rev {
		return ErrConflict
	}
	x := r.state
	if e := fn(&x, r.auth); e != nil {
		return e
	}
	x.Revision++
	r.state = x
	return nil
}
func fixture(t *testing.T, p Purpose) (*Service, *memoryRepository, Snapshot) {
	t.Helper()
	r := &memoryRepository{auth: Authorization{ActorID: actor, SubjectID: subject, SpaceOwnerID: subject, ReviewerGrants: []ReviewerGrant{{ReviewerID: actor, Capability: CapabilityIdentity, ScopeID: subject}}, IdentityCurrent: true}}
	s, _ := NewService(r)
	x := Snapshot{Purpose: p, ID: caseID, SubjectID: subject, SpaceID: space, State: CaseSubmitted, Files: []File{{ID: fileID, State: FileClean, Size: 100, MIME: "image/jpeg", Pages: 1}}, DocumentExpiresAt: time.Date(2028, 1, 1, 0, 0, 0, 0, time.UTC)}
	if p == PurposeIdentity {
		x.SpaceID = ""
	}
	r.state = x
	return s, r, x
}
func TestCreateDerivesAuthorizationAndRejectsWrongOwner(t *testing.T) {
	s, r, _ := fixture(t, PurposeIdentity)
	r.auth.SubjectID = actor
	if e := s.Create(context.Background(), actor, CreateInput{Purpose: PurposeIdentity}); e != nil {
		t.Fatal(e)
	}
	if r.state.SubjectID != actor || r.state.ID == caseID || r.state.State != CaseDraft || len(r.state.Files) != 0 {
		t.Fatalf("unsafe create: %+v", r.state)
	}
	r.auth.SpaceOwnerID = subject
	if !errors.Is(s.Create(context.Background(), actor, CreateInput{Purpose: PurposeSpace, SpaceID: space}), ErrCapability) {
		t.Fatal("foreign owner accepted")
	}
}
func TestReviewUsesServerClockAndSnapshotExpiry(t *testing.T) {
	s, r, x := fixture(t, PurposeIdentity)
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s.SetClock(func() time.Time { return now })
	expiry := now.AddDate(0, 6, 0)
	r.state.DocumentExpiresAt = expiry
	if e := s.Review(context.Background(), actor, x.ID, CapabilityIdentity, DecisionApprove, time.Time{}, 0); e != nil {
		t.Fatal(e)
	}
	if !r.state.VerifiedAt.Equal(now) || !r.state.VerifiedUntil.Equal(expiry) {
		t.Fatalf("timestamps: %+v", r.state)
	}
}
func TestTransitionRejectsPurposeSnapshotMismatchAndRevision(t *testing.T) {
	s, r, x := fixture(t, PurposeIdentity)
	r.state.Purpose = PurposeSpace
	if !errors.Is(s.Submit(context.Background(), subject, x.ID, PurposeIdentity, 0), ErrNotFound) {
		t.Fatal("mismatch accepted")
	}
	r.state.Purpose = PurposeIdentity
	if !errors.Is(s.Submit(context.Background(), subject, x.ID, PurposeIdentity, 9), ErrConflict) {
		t.Fatal("stale revision accepted")
	}
}
func TestFileMetadataPolicy(t *testing.T) {
	for _, tc := range []struct {
		p     Purpose
		mime  string
		size  int64
		pages int
		bad   bool
	}{{PurposeIdentity, "image/jpeg", 10 * 1024 * 1024, 1, false}, {PurposeIdentity, "image/gif", 10, 1, true}, {PurposeIdentity, "application/pdf", 1, 6, true}, {PurposeSpace, "application/pdf", 1, 20, false}, {PurposeSpace, "image/png", 10*1024*1024 + 1, 1, true}} {
		e := ValidateQuota(tc.p, []File{{ID: fileID, State: FileClean, Size: tc.size, MIME: tc.mime, Pages: tc.pages}})
		if (e != nil) != tc.bad {
			t.Errorf("%+v => %v", tc, e)
		}
	}
}
func TestBackendErrorsPropagate(t *testing.T) {
	s, r, x := fixture(t, PurposeIdentity)
	backend := errors.New("backend unavailable")
	r.err = backend
	if !errors.Is(s.Submit(context.Background(), subject, x.ID, x.Purpose, 0), backend) {
		t.Fatal("backend error lost")
	}
}

func TestSpaceReviewRejectsCurrentOwnerEvenWhenHistoricalSubjectDiffers(t *testing.T) {
	s, r, x := fixture(t, PurposeSpace)
	r.auth.ActorID = actor
	r.auth.SpaceOwnerID = actor
	r.auth.ReviewerGrants = []ReviewerGrant{{ReviewerID: actor, Capability: CapabilitySpace, ScopeID: space}}
	r.state.SubjectID = subject
	before := r.state
	err := s.Review(context.Background(), actor, x.ID, CapabilitySpace, DecisionReject, time.Time{}, 0)
	if !errors.Is(err, ErrSelfReview) {
		t.Fatalf("got %v want %v", err, ErrSelfReview)
	}
	if r.state.State != before.State || r.state.Revision != before.Revision {
		t.Fatal("owner review mutated case")
	}
}

func TestSpaceCaseHistoricalOwnerMismatchCannotSubmit(t *testing.T) {
	s, r, x := fixture(t, PurposeSpace)
	r.state.State = CaseDraft
	r.state.SubjectID = actor
	r.auth.ActorID = subject
	r.auth.SubjectID = subject
	r.auth.SpaceOwnerID = subject
	before := r.state
	err := s.Submit(context.Background(), actor, x.ID, PurposeSpace, 0)
	if !errors.Is(err, ErrCapability) {
		t.Fatalf("got %v want capability denial", err)
	}
	if r.state.State != before.State || r.state.Revision != before.Revision {
		t.Fatal("mutated")
	}
}
func TestSpaceReviewRejectsStaleHistoricalOwner(t *testing.T) {
	s, r, x := fixture(t, PurposeSpace)
	r.auth.ReviewerGrants = []ReviewerGrant{{ReviewerID: subject, Capability: CapabilitySpace, ScopeID: space}}
	r.auth.SpaceOwnerID = actor
	r.auth.ActorID = subject
	before := r.state
	err := s.Review(context.Background(), subject, x.ID, CapabilitySpace, DecisionReject, time.Time{}, 0)
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("got %v want %v", err, ErrConflict)
	}
	if r.state.State != before.State || r.state.Revision != before.Revision {
		t.Fatal("mutated")
	}
}
func TestSpaceRevokeRejectsCurrentOwner(t *testing.T) {
	s, r, x := fixture(t, PurposeSpace)
	r.state.State = CaseApproved
	r.auth.ReviewerGrants = []ReviewerGrant{{ReviewerID: actor, Capability: CapabilitySpace, ScopeID: space}}
	r.auth.SpaceOwnerID = actor
	before := r.state
	err := s.Revoke(context.Background(), actor, x.ID, PurposeSpace, "reason", 0)
	if !errors.Is(err, ErrSelfReview) {
		t.Fatalf("got %v", err)
	}
	if r.state.State != before.State || r.state.Revision != before.Revision {
		t.Fatal("mutated")
	}
}
