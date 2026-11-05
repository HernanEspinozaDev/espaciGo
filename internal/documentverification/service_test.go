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
	state           Snapshot
	auth            Authorization
	err             error
	calls           int
	createActor     string
	transitionActor string
	listActor       string
	loadActor       string
}

func (r *memoryRepository) CreateCase(_ context.Context, actor string, p Purpose, sid string, build func(Authorization) (Snapshot, error)) (Snapshot, error) {
	r.calls++
	r.createActor = actor
	if r.err != nil {
		return Snapshot{}, r.err
	}
	x, e := build(r.auth)
	if e != nil {
		return Snapshot{}, e
	}
	x.Revision = 1
	r.state = x
	return x, nil
}
func (r *memoryRepository) Transition(_ context.Context, actor string, p Purpose, id string, rev uint64, _ TransitionAudit, fn func(*Snapshot, Authorization) error) error {
	r.calls++
	r.transitionActor = actor
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

func (r *memoryRepository) ListOwnCases(_ context.Context, actor string) ([]Snapshot, error) {
	r.listActor = actor
	if actor != r.state.SubjectID {
		return nil, ErrNotFound
	}
	return []Snapshot{r.state}, nil
}

func (r *memoryRepository) LoadOwnCase(_ context.Context, actor, id string) (Snapshot, error) {
	r.loadActor = actor
	if actor != r.state.SubjectID || id != r.state.ID {
		return Snapshot{}, ErrNotFound
	}
	return r.state, nil
}

func TestServicePassesAuthenticatedActorToRepository(t *testing.T) {
	s, r, _ := fixture(t, PurposeIdentity)
	r.auth.SubjectID = actor
	if err := s.Create(context.Background(), actor, CreateInput{Purpose: PurposeIdentity, DocumentExpiresAt: time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatal(err)
	}
	if r.createActor != actor {
		t.Fatalf("create repository actor=%q, want %q", r.createActor, actor)
	}
	s, r, x := fixture(t, PurposeIdentity)
	r.auth.ActorID = subject
	r.auth.SubjectID = subject
	r.state.State = CaseDraft
	if err := s.Submit(context.Background(), subject, x.ID, PurposeIdentity, 0); err != nil {
		t.Fatal(err)
	}
	if r.transitionActor != subject {
		t.Fatalf("transition repository actor=%q, want %q", r.transitionActor, subject)
	}
}

func TestServiceStartReturnsCreatedCaseSnapshot(t *testing.T) {
	s, r, _ := fixture(t, PurposeIdentity)
	r.auth.SubjectID = actor
	expires := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	created, err := s.Start(context.Background(), actor, CreateInput{Purpose: PurposeIdentity, DocumentExpiresAt: expires})
	if err != nil {
		t.Fatal(err)
	}
	if ValidateID(created.ID) != nil || created.Purpose != PurposeIdentity || created.SubjectID != actor || created.State != CaseDraft || created.Revision != 1 {
		t.Fatalf("unexpected created snapshot: %+v", created)
	}
}

func TestServiceStartPreservesDeclaredDocumentExpiry(t *testing.T) {
	s, r, _ := fixture(t, PurposeIdentity)
	r.auth.SubjectID = actor
	s.SetClock(func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) })
	expires := time.Date(2027, 9, 15, 0, 0, 0, 0, time.UTC)
	created, err := s.Start(context.Background(), actor, CreateInput{Purpose: PurposeIdentity, DocumentExpiresAt: expires})
	if err != nil {
		t.Fatal(err)
	}
	if !created.DocumentExpiresAt.Equal(expires) {
		t.Fatalf("document expiry=%v, want %v", created.DocumentExpiresAt, expires)
	}
}

func TestAddFilePersistsOnlyPendingOwnerEvidenceWithinQuota(t *testing.T) {
	s, r, x := fixture(t, PurposeIdentity)
	r.state.State = CaseDraft
	r.auth.ActorID = subject
	r.auth.SubjectID = subject
	file := File{ID: "66666666-6666-4666-8666-666666666666", State: FilePending, Size: 128, MIME: "image/png", Pages: 1, ContentSHA256: [32]byte{1}}
	if err := s.AddFile(context.Background(), subject, x.ID, PurposeIdentity, 0, file); err != nil {
		t.Fatal(err)
	}
	if len(r.state.Files) != 2 || r.state.Files[1] != file || r.state.Revision != 1 {
		t.Fatalf("unexpected pending owner evidence state: %+v", r.state)
	}
	if err := s.AddFile(context.Background(), actor, x.ID, PurposeIdentity, 1, File{ID: "77777777-7777-4777-8777-777777777777", State: FilePending, Size: 1, MIME: "image/png", Pages: 1, ContentSHA256: [32]byte{1}}); !errors.Is(err, ErrCapability) {
		t.Fatalf("non-owner upload error=%v, want ErrCapability", err)
	}
}

func TestServiceReadsOnlyAuthenticatedOwnersCases(t *testing.T) {
	s, r, x := fixture(t, PurposeIdentity)
	list, err := s.ListOwnCases(context.Background(), subject)
	if err != nil || len(list) != 1 || list[0].ID != x.ID || r.listActor != subject {
		t.Fatalf("own case list=%+v actor=%q err=%v", list, r.listActor, err)
	}
	loaded, err := s.LoadOwnCase(context.Background(), subject, x.ID)
	if err != nil || loaded.ID != x.ID || r.loadActor != subject {
		t.Fatalf("own case=%+v actor=%q err=%v", loaded, r.loadActor, err)
	}
	if _, err := s.LoadOwnCase(context.Background(), actor, x.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("foreign owner read error=%v, want ErrNotFound", err)
	}
}

func fixture(t *testing.T, p Purpose) (*Service, *memoryRepository, Snapshot) {
	t.Helper()
	r := &memoryRepository{auth: Authorization{ActorID: actor, SubjectID: subject, SpaceOwnerID: subject, ReviewerGrants: []ReviewerGrant{{ReviewerID: actor, Capability: CapabilityIdentity, ScopeID: subject}}, IdentityCurrent: true}}
	s, _ := NewService(r)
	x := Snapshot{Purpose: p, ID: caseID, SubjectID: subject, SpaceID: space, State: CaseSubmitted, Files: []File{{ID: fileID, State: FileClean, Size: 100, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{1}}}, DocumentExpiresAt: time.Date(2028, 1, 1, 0, 0, 0, 0, time.UTC)}
	if p == PurposeIdentity {
		x.SpaceID = ""
	}
	r.state = x
	return s, r, x
}
func TestCreateDerivesAuthorizationAndRejectsWrongOwner(t *testing.T) {
	s, r, _ := fixture(t, PurposeIdentity)
	r.auth.SubjectID = actor
	if e := s.Create(context.Background(), actor, CreateInput{Purpose: PurposeIdentity, DocumentExpiresAt: time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)}); e != nil {
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

func TestReviewRejectionRequiresAReasonCode(t *testing.T) {
	s, r, x := fixture(t, PurposeIdentity)
	r.auth.ActorID = actor
	before := r.state
	err := s.Review(context.Background(), actor, x.ID, CapabilityIdentity, DecisionReject, time.Time{}, 0)
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("reasonless rejection error=%v, want ErrInvalid", err)
	}
	if r.state.State != before.State || r.state.Revision != before.Revision {
		t.Fatal("reasonless rejection mutated the case")
	}
}

func TestRevokeRejectsFreeTextReason(t *testing.T) {
	s, r, x := fixture(t, PurposeIdentity)
	r.state.State = CaseApproved
	r.state.VerifiedAt = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	r.state.VerifiedUntil = time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)
	r.auth.ReviewerGrants = []ReviewerGrant{{ReviewerID: actor, Capability: CapabilityIdentity, ScopeID: subject}}
	before := r.state
	err := s.Revoke(context.Background(), actor, x.ID, PurposeIdentity, "texto libre con datos", 0)
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("free-text revocation error=%v, want ErrInvalid", err)
	}
	if r.state.State != before.State || r.state.Revision != before.Revision {
		t.Fatal("free-text revocation mutated the case")
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
		e := ValidateQuota(tc.p, []File{{ID: fileID, State: FileClean, Size: tc.size, MIME: tc.mime, Pages: tc.pages, ContentSHA256: [32]byte{1}}})
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
	err := s.ReviewWithReason(context.Background(), actor, x.ID, CapabilitySpace, DecisionReject, ReasonDocumentInvalid, time.Time{}, 0)
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
	err := s.ReviewWithReason(context.Background(), subject, x.ID, CapabilitySpace, DecisionReject, ReasonDocumentInvalid, time.Time{}, 0)
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
	err := s.Revoke(context.Background(), actor, x.ID, PurposeSpace, ReasonOwnerRequest, 0)
	if !errors.Is(err, ErrSelfReview) {
		t.Fatalf("got %v", err)
	}
	if r.state.State != before.State || r.state.Revision != before.Revision {
		t.Fatal("mutated")
	}
}
