package documentverification

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
)

const actor = "11111111-1111-4111-8111-111111111111"
const subject = "22222222-2222-4222-8222-222222222222"
const space = "33333333-3333-4333-8333-333333333333"
const caseID = "55555555-5555-4555-8555-555555555555"
const fileID = "44444444-4444-4444-8444-444444444444"
const declarationRevisionID = "88888888-8888-4888-8888-888888888888"

type memoryRepository struct {
	state           Snapshot
	auth            Authorization
	audit           TransitionAudit
	err             error
	calls           int
	createActor     string
	transitionActor string
	listActor       string
	loadActor       string
}

type cancelBeforeTransitionCallbackRepository struct {
	*memoryRepository
	cancel context.CancelFunc
}

func (r *cancelBeforeTransitionCallbackRepository) Transition(ctx context.Context, actor string, p Purpose, id string, rev uint64, audit TransitionAudit, fn func(*Snapshot, Authorization) error) error {
	r.cancel()
	return r.memoryRepository.Transition(ctx, actor, p, id, rev, audit, fn)
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
func (r *memoryRepository) Transition(_ context.Context, actor string, p Purpose, id string, rev uint64, audit TransitionAudit, fn func(*Snapshot, Authorization) error) error {
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
	r.audit = audit
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
	if err := s.Create(context.Background(), actor, identityCreateInput(DocumentFormatNew)); err != nil {
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

func TestStartRequiresDeclaredDocumentFormat(t *testing.T) {
	s, r, _ := fixture(t, PurposeIdentity)
	r.auth.SubjectID = actor
	if _, err := s.Start(context.Background(), actor, CreateInput{Purpose: PurposeIdentity}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("missing declared format error=%v, want ErrInvalid", err)
	}
	if r.calls != 0 {
		t.Fatalf("repository calls=%d, want validation before persistence", r.calls)
	}
}

func TestIdentityStartRequiresSeparateCaptureConsent(t *testing.T) {
	s, r, _ := fixture(t, PurposeIdentity)
	r.auth.SubjectID = actor
	for _, tc := range []struct {
		name string
		edit func(*CreateInput)
	}{
		{name: "missing", edit: func(*CreateInput) {}},
		{name: "unchecked", edit: func(in *CreateInput) { in.CaptureConsentVersion = IdentityCaptureConsentVersion }},
		{name: "unknown version", edit: func(in *CreateInput) { in.CaptureConsentGiven = true; in.CaptureConsentVersion = "identity-capture-v0" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := identityCreateInput(DocumentFormatNew)
			input.CaptureConsentGiven = false
			tc.edit(&input)
			if _, err := s.Start(context.Background(), actor, input); !errors.Is(err, ErrInvalid) {
				t.Fatalf("capture consent error=%v, want ErrInvalid", err)
			}
		})
	}
	if r.calls != 0 {
		t.Fatalf("repository calls=%d, want consent validation before persistence", r.calls)
	}
}

func TestStartRequiresServerDeclarationRevision(t *testing.T) {
	s, r, _ := fixture(t, PurposeIdentity)
	r.auth.SubjectID = actor
	r.auth.IdentityDeclarationRevisionID = ""
	before := r.state
	_, err := s.Start(context.Background(), actor, identityCreateInput(DocumentFormatLegacy))
	if !errors.Is(err, ErrDeclarationRequired) {
		t.Fatalf("missing server declaration error=%v, want ErrDeclarationRequired", err)
	}
	if r.state.ID != before.ID || r.state.Revision != before.Revision {
		t.Fatal("case changed when there was no saved declaration")
	}
}

func TestServiceStartReturnsCreatedCaseSnapshot(t *testing.T) {
	s, r, _ := fixture(t, PurposeIdentity)
	r.auth.SubjectID = actor
	created, err := s.Start(context.Background(), actor, identityCreateInput(DocumentFormatNew))
	if err != nil {
		t.Fatal(err)
	}
	if ValidateID(created.ID) != nil || created.Purpose != PurposeIdentity || created.SubjectID != actor || created.State != CaseDraft || created.Revision != 1 {
		t.Fatalf("unexpected created snapshot: %+v", created)
	}
	if created.IdentityDeclarationRevisionID != r.auth.IdentityDeclarationRevisionID || created.DocumentFormatDeclared != DocumentFormatNew ||
		created.CaptureConsentVersion != IdentityCaptureConsentVersion || created.CaptureConsentedAt.IsZero() || !created.DocumentExpiresAt.IsZero() {
		t.Fatalf("identity case did not capture server declaration and declared format: %+v", created)
	}
}

func TestIdentityStartAcceptsLegacyDeclaredFormat(t *testing.T) {
	s, r, _ := fixture(t, PurposeIdentity)
	r.auth.SubjectID = actor

	created, err := s.Start(context.Background(), actor, identityCreateInput(DocumentFormatLegacy))
	if err != nil {
		t.Fatalf("start with legacy declared format: %v", err)
	}
	if created.Purpose != PurposeIdentity || created.SubjectID != actor || created.State != CaseDraft ||
		created.DocumentFormatDeclared != DocumentFormatLegacy ||
		created.IdentityDeclarationRevisionID != r.auth.IdentityDeclarationRevisionID ||
		created.CaptureConsentVersion != IdentityCaptureConsentVersion || created.CaptureConsentedAt.IsZero() ||
		!created.DocumentExpiresAt.IsZero() {
		t.Fatalf("legacy identity case was not created with its authoritative fields: %+v", created)
	}
	if r.calls != 1 || r.state.ID != created.ID || r.state.DocumentFormatDeclared != DocumentFormatLegacy {
		t.Fatalf("legacy identity case not persisted to memory repository: calls=%d state=%+v", r.calls, r.state)
	}
}

func TestIdentityStartRecordsServerConsentTime(t *testing.T) {
	s, r, _ := fixture(t, PurposeIdentity)
	r.auth.SubjectID = actor
	now := time.Date(2026, 11, 10, 12, 30, 0, 0, time.UTC)
	s.SetClock(func() time.Time { return now })
	created, err := s.Start(context.Background(), actor, identityCreateInput(DocumentFormatUnsure))
	if err != nil {
		t.Fatal(err)
	}
	if created.CaptureConsentVersion != IdentityCaptureConsentVersion || !created.CaptureConsentedAt.Equal(now) {
		t.Fatalf("captured consent=%q at %s, want %q at %s", created.CaptureConsentVersion, created.CaptureConsentedAt, IdentityCaptureConsentVersion, now)
	}
}

func TestIdentityStartRejectsApplicantDeclaredExpiry(t *testing.T) {
	s, r, _ := fixture(t, PurposeIdentity)
	r.auth.SubjectID = actor
	expires := time.Date(2027, 9, 15, 0, 0, 0, 0, time.UTC)
	input := identityCreateInput(DocumentFormatNew)
	input.DocumentExpiresAt = expires
	_, err := s.Start(context.Background(), actor, input)
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("identity expiry supplied by applicant error=%v, want ErrInvalid", err)
	}
	if r.calls != 0 {
		t.Fatalf("repository calls=%d, want validation before persistence", r.calls)
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
	r := &memoryRepository{auth: Authorization{ActorID: actor, SubjectID: subject, SpaceOwnerID: subject, IdentityDeclarationRevisionID: declarationRevisionID, ReviewerGrants: []ReviewerGrant{{ReviewerID: actor, Capability: CapabilityIdentity, ScopeID: subject}}, IdentityCurrent: true}}
	s, _ := NewService(r)
	x := Snapshot{Purpose: p, ID: caseID, SubjectID: subject, SpaceID: space, State: CaseSubmitted, Files: []File{{ID: fileID, State: FileClean, Size: 100, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{1}}}, DocumentExpiresAt: time.Date(2028, 1, 1, 0, 0, 0, 0, time.UTC)}
	if p == PurposeIdentity {
		x.SpaceID = ""
	}
	r.state = x
	return s, r, x
}

func identityCreateInput(format DocumentFormat) CreateInput {
	return CreateInput{
		Purpose:                PurposeIdentity,
		DocumentFormatDeclared: format,
		CaptureConsentGiven:    true,
		CaptureConsentVersion:  IdentityCaptureConsentVersion,
	}
}

func TestClosedIdentityCaseReleasesDeclarationLinkWithoutLosingConsent(t *testing.T) {
	x := Snapshot{
		ID: caseID, Purpose: PurposeIdentity, SubjectID: subject, State: CaseApproved, Revision: 2,
		IdentityDeclarationRevisionID: declarationRevisionID,
		DocumentFormatDeclared:        DocumentFormatNew,
		CaptureConsentVersion:         IdentityCaptureConsentVersion,
		CaptureConsentedAt:            time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC),
		DocumentFormatObserved:        DocumentFormatNew,
		DocumentExpiresAt:             time.Date(2028, 11, 10, 0, 0, 0, 0, time.UTC),
		VerifiedAt:                    time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC),
		VerifiedUntil:                 time.Date(2027, 11, 10, 12, 0, 0, 0, time.UTC),
	}
	if !validSnapshot(x) {
		t.Fatal("approved identity case with its linked declaration is invalid")
	}
	x.State = CaseRevoked
	x.IdentityDeclarationRevisionID = ""
	if !validSnapshot(x) {
		t.Fatal("closed identity case must retain consent metadata after releasing its declaration link")
	}
	x.State = CaseSubmitted
	if validSnapshot(x) {
		t.Fatal("active identity case without its declaration link was accepted")
	}
}

func TestCreateDerivesAuthorizationAndRejectsWrongOwner(t *testing.T) {
	s, r, _ := fixture(t, PurposeIdentity)
	r.auth.SubjectID = actor
	if e := s.Create(context.Background(), actor, identityCreateInput(DocumentFormatNew)); e != nil {
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

func TestTypedIdentityReviewRequiresReviewerObservation(t *testing.T) {
	s, r, x := fixture(t, PurposeIdentity)
	r.state.IdentityDeclarationRevisionID = declarationRevisionID
	r.state.DocumentFormatDeclared = DocumentFormatNew
	r.state.Files = []File{
		{ID: fileID, EvidenceRole: EvidenceRoleIdentityFront, State: FileClean, Size: 100, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{1}},
		{ID: "66666666-6666-4666-8666-666666666666", EvidenceRole: EvidenceRoleIdentityBack, State: FileClean, Size: 100, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{1}},
		{ID: "77777777-7777-4777-8777-777777777777", EvidenceRole: EvidenceRoleIdentitySelfie, State: FileClean, Size: 100, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{1}},
	}
	if err := s.ReviewWithReason(context.Background(), actor, x.ID, CapabilityIdentity, DecisionApprove, "", time.Time{}, 0); !errors.Is(err, ErrInvalid) {
		t.Fatalf("identity decision without reviewer observation error=%v, want ErrInvalid", err)
	}
	if r.state.State != CaseSubmitted || r.state.Revision != 0 {
		t.Fatal("unobserved identity review changed case")
	}
}

func TestTypedIdentityApprovalRequiresReviewerChecklist(t *testing.T) {
	s, r, x := fixture(t, PurposeIdentity)
	r.state.IdentityDeclarationRevisionID = declarationRevisionID
	r.state.DocumentFormatDeclared = DocumentFormatNew
	r.state.Files = []File{
		{ID: fileID, EvidenceRole: EvidenceRoleIdentityFront, State: FileClean, Size: 100, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{1}},
		{ID: "66666666-6666-4666-8666-666666666666", EvidenceRole: EvidenceRoleIdentityBack, State: FileClean, Size: 100, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{1}},
		{ID: "77777777-7777-4777-8777-777777777777", EvidenceRole: EvidenceRoleIdentitySelfie, State: FileClean, Size: 100, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{1}},
	}
	before := r.state
	err := s.ReviewIdentityWithReason(context.Background(), actor, x.ID, DecisionApprove, "", DocumentFormatNew, time.Date(2029, 9, 15, 0, 0, 0, 0, time.UTC), nil, time.Time{}, 0)
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("identity approval without checklist error=%v, want ErrInvalid", err)
	}
	if r.state.State != before.State || r.state.Revision != before.Revision || r.state.DocumentFormatObserved != before.DocumentFormatObserved {
		t.Fatal("approval without checklist mutated the case")
	}
}

func TestTypedIdentityApprovalRejectsEveryFailedChecklistItem(t *testing.T) {
	for _, field := range []string{
		"names_match",
		"rut_matches",
		"birth_date_and_majority_match",
		"portrait_matches_selfie",
		"evidence_legible",
		"document_unexpired",
		"issue_date_and_format_verified",
	} {
		t.Run(field, func(t *testing.T) {
			s, r, x := fixture(t, PurposeIdentity)
			r.state.IdentityDeclarationRevisionID = declarationRevisionID
			r.state.DocumentFormatDeclared = DocumentFormatNew
			r.state.Files = []File{
				{ID: fileID, EvidenceRole: EvidenceRoleIdentityFront, State: FileClean, Size: 100, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{1}},
				{ID: "66666666-6666-4666-8666-666666666666", EvidenceRole: EvidenceRoleIdentityBack, State: FileClean, Size: 100, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{1}},
				{ID: "77777777-7777-4777-8777-777777777777", EvidenceRole: EvidenceRoleIdentitySelfie, State: FileClean, Size: 100, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{1}},
			}
			checklist := passedIdentityReviewChecklist()
			no := false
			switch field {
			case "names_match":
				checklist.NamesMatch = &no
			case "rut_matches":
				checklist.RUTMatches = &no
			case "birth_date_and_majority_match":
				checklist.BirthDateAndMajorityMatch = &no
			case "portrait_matches_selfie":
				checklist.PortraitMatchesSelfie = &no
			case "evidence_legible":
				checklist.EvidenceLegible = &no
			case "document_unexpired":
				checklist.DocumentUnexpired = &no
			case "issue_date_and_format_verified":
				checklist.IssueDateAndFormatVerified = &no
			default:
				t.Fatalf("unknown checklist field %q", field)
			}
			before := r.state
			before.Files = append([]File(nil), r.state.Files...)
			if r.state.RevokedAt != nil {
				value := *r.state.RevokedAt
				before.RevokedAt = &value
			}
			if r.state.SupersededAt != nil {
				value := *r.state.SupersededAt
				before.SupersededAt = &value
			}
			beforeAudit := r.audit
			err := s.ReviewIdentityWithReason(context.Background(), actor, x.ID, DecisionApprove, "", DocumentFormatNew, time.Date(2029, 9, 15, 0, 0, 0, 0, time.UTC), checklist, time.Time{}, 0)
			if !errors.Is(err, ErrInvalid) {
				t.Fatalf("identity approval with failed %s error=%v, want ErrInvalid", field, err)
			}
			if !reflect.DeepEqual(r.state, before) || !reflect.DeepEqual(r.audit, beforeAudit) {
				t.Fatalf("approval with failed %s checklist item mutated state or audit: case=%+v audit=%+v", field, r.state, r.audit)
			}
		})
	}
}

func TestTypedIdentityApprovalUsesReviewerObservedMetadata(t *testing.T) {
	s, r, x := fixture(t, PurposeIdentity)
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	s.SetClock(func() time.Time { return now })
	r.state.DocumentExpiresAt = time.Date(2027, 9, 15, 0, 0, 0, 0, time.UTC)
	r.state.IdentityDeclarationRevisionID = declarationRevisionID
	r.state.DocumentFormatDeclared = DocumentFormatNew
	r.state.Files = []File{
		{ID: fileID, EvidenceRole: EvidenceRoleIdentityFront, State: FileClean, Size: 100, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{1}},
		{ID: "66666666-6666-4666-8666-666666666666", EvidenceRole: EvidenceRoleIdentityBack, State: FileClean, Size: 100, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{1}},
		{ID: "77777777-7777-4777-8777-777777777777", EvidenceRole: EvidenceRoleIdentitySelfie, State: FileClean, Size: 100, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{1}},
	}
	observedExpiry := time.Date(2029, 9, 15, 0, 0, 0, 0, time.UTC)
	checklist := passedIdentityReviewChecklist()
	if err := s.ReviewIdentityWithReason(context.Background(), actor, x.ID, DecisionApprove, "", DocumentFormatLegacy, observedExpiry, checklist, time.Time{}, 0); err != nil {
		t.Fatalf("approve typed identity with reviewer observation: %v", err)
	}
	wantUntil := now.AddDate(0, 24, 0)
	if r.state.State != CaseApproved || r.state.DocumentFormatDeclared != DocumentFormatNew || r.state.DocumentFormatObserved != DocumentFormatLegacy || !r.state.DocumentExpiresAt.Equal(observedExpiry) || !r.state.VerifiedUntil.Equal(wantUntil) {
		t.Fatalf("reviewer observation not applied correctly: %+v", r.state)
	}
	if r.audit.DocumentFormatObserved != DocumentFormatLegacy || !r.audit.DocumentExpiresAtObserved.Equal(observedExpiry) || r.audit.IdentityReviewChecklist == nil || !r.audit.IdentityReviewChecklist.AllPassed() {
		t.Fatalf("audit reviewer observations not captured: %+v", r.audit)
	}
}

func TestTypedIdentityApprovalHonorsExclusiveObservedExpiryBoundary(t *testing.T) {
	boundary := time.Date(2031, 4, 6, 3, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name    string
		now     time.Time
		approve bool
	}{
		{name: "valid until just before boundary", now: boundary.Add(-time.Nanosecond), approve: true},
		{name: "expired at exclusive boundary", now: boundary, approve: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, r, x := fixture(t, PurposeIdentity)
			s.SetClock(func() time.Time { return tc.now })
			r.state.IdentityDeclarationRevisionID = declarationRevisionID
			r.state.DocumentFormatDeclared = DocumentFormatNew
			r.state.CaptureConsentVersion = IdentityCaptureConsentVersion
			r.state.CaptureConsentedAt = time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
			r.auth.IdentityDeclarationRevisionID = declarationRevisionID
			r.state.Files = []File{
				{ID: fileID, EvidenceRole: EvidenceRoleIdentityFront, State: FileClean, Size: 100, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{1}},
				{ID: "66666666-6666-4666-8666-666666666666", EvidenceRole: EvidenceRoleIdentityBack, State: FileClean, Size: 100, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{2}},
				{ID: "77777777-7777-4777-8777-777777777777", EvidenceRole: EvidenceRoleIdentitySelfie, State: FileClean, Size: 100, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{3}},
			}
			before := r.state
			before.Files = append([]File(nil), r.state.Files...)
			beforeAudit := r.audit

			err := s.ReviewIdentityWithReason(context.Background(), actor, x.ID, DecisionApprove, "", DocumentFormatNew, boundary, passedIdentityReviewChecklist(), time.Time{}, 0)
			if tc.approve {
				if err != nil || r.state.State != CaseApproved || !r.state.VerifiedUntil.Equal(boundary) {
					t.Fatalf("approval before boundary: err=%v state=%q verified_until=%s", err, r.state.State, r.state.VerifiedUntil)
				}
				return
			}
			if !errors.Is(err, ErrInvalid) {
				t.Fatalf("approval at exclusive boundary error=%v, want ErrInvalid", err)
			}
			if !reflect.DeepEqual(r.state, before) || !reflect.DeepEqual(r.audit, beforeAudit) {
				t.Fatalf("expired approval mutated state or audit: state=%+v audit=%+v", r.state, r.audit)
			}
		})
	}
}

func passedIdentityReviewChecklist() *IdentityReviewChecklist {
	yes := true
	return &IdentityReviewChecklist{
		NamesMatch: &yes, RUTMatches: &yes, BirthDateAndMajorityMatch: &yes,
		PortraitMatchesSelfie: &yes, EvidenceLegible: &yes, DocumentUnexpired: &yes,
		IssueDateAndFormatVerified: &yes,
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

func TestRevokeRejectsInvalidReasonsBeforeRepository(t *testing.T) {
	for _, reason := range []ReasonCode{"", "unsupported_reason", "texto libre sintetico"} {
		t.Run(string(reason), func(t *testing.T) {
			s, r, x := fixture(t, PurposeIdentity)
			r.state.State = CaseApproved
			r.state.VerifiedAt = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
			r.state.VerifiedUntil = time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)
			r.auth.ReviewerGrants = []ReviewerGrant{{ReviewerID: actor, Capability: CapabilityIdentity, ScopeID: subject}}
			before := r.state
			beforeCalls := r.calls
			beforeAudit := r.audit
			err := s.Revoke(context.Background(), actor, x.ID, PurposeIdentity, reason, 0)
			if !errors.Is(err, ErrInvalid) {
				t.Fatalf("reason %q error=%v, want ErrInvalid", reason, err)
			}
			if r.calls != beforeCalls {
				t.Fatalf("invalid reason reached repository: calls=%d, want %d", r.calls, beforeCalls)
			}
			if r.state.State != before.State || r.state.Revision != before.Revision || r.audit != beforeAudit {
				t.Fatal("invalid reason mutated the case or audit")
			}
		})
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

func TestServiceConstructorQuotaAndSnapshotValidationBoundaries(t *testing.T) {
	// 1. NewService nil check
	if _, err := NewService(nil); !errors.Is(err, ErrInvalid) {
		t.Fatalf("NewService(nil) = %v, want ErrInvalid", err)
	}

	// 2. ValidateQuota boundaries
	if err := ValidateQuota(Purpose("unknown"), nil); !errors.Is(err, ErrInvalid) {
		t.Fatalf("ValidateQuota unknown purpose err = %v, want ErrInvalid", err)
	}

	validFile := File{
		ID: fileID, State: FileClean, Size: 1024, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{1},
	}

	// File count limits: max 3 for identity, max 10 for space
	makeFiles := func(count int) []File {
		var list []File
		for i := 0; i < count; i++ {
			list = append(list, File{
				ID:            fmt.Sprintf("44444444-4444-4444-8444-%012d", i+1),
				State:         FileClean,
				Size:          1024,
				MIME:          "image/jpeg",
				Pages:         1,
				ContentSHA256: [32]byte{byte(i + 1)},
			})
		}
		return list
	}
	if err := ValidateQuota(PurposeIdentity, makeFiles(3)); err != nil {
		t.Fatalf("ValidateQuota identity 3 files err = %v, want nil", err)
	}
	if err := ValidateQuota(PurposeIdentity, makeFiles(4)); !errors.Is(err, ErrQuota) {
		t.Fatalf("ValidateQuota identity 4 files err = %v, want ErrQuota", err)
	}
	if err := ValidateQuota(PurposeSpace, makeFiles(10)); err != nil {
		t.Fatalf("ValidateQuota space 10 files err = %v, want nil", err)
	}
	if err := ValidateQuota(PurposeSpace, makeFiles(11)); !errors.Is(err, ErrQuota) {
		t.Fatalf("ValidateQuota space 11 files err = %v, want ErrQuota", err)
	}

	// Duplicate file ID
	dupFiles := []File{validFile, validFile}
	if err := ValidateQuota(PurposeIdentity, dupFiles); !errors.Is(err, ErrInvalid) {
		t.Fatalf("ValidateQuota dup files err = %v, want ErrInvalid", err)
	}

	// Zero content SHA256
	zeroSHA := validFile
	zeroSHA.ContentSHA256 = [32]byte{}
	if err := ValidateQuota(PurposeIdentity, []File{zeroSHA}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("ValidateQuota zero SHA err = %v, want ErrInvalid", err)
	}

	// Invalid file state
	badState := validFile
	badState.State = FileState("corrupt")
	if err := ValidateQuota(PurposeIdentity, []File{badState}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("ValidateQuota bad file state err = %v, want ErrInvalid", err)
	}

	// File size <= 0 or > 10MB, and exactly 10MB
	zeroSize := validFile
	zeroSize.Size = 0
	if err := ValidateQuota(PurposeIdentity, []File{zeroSize}); !errors.Is(err, ErrQuota) {
		t.Fatalf("ValidateQuota zero size err = %v, want ErrQuota", err)
	}
	negSize := validFile
	negSize.Size = -1
	if err := ValidateQuota(PurposeIdentity, []File{negSize}); !errors.Is(err, ErrQuota) {
		t.Fatalf("ValidateQuota neg size err = %v, want ErrQuota", err)
	}
	exactMaxSize := validFile
	exactMaxSize.Size = 10 * 1024 * 1024
	if err := ValidateQuota(PurposeIdentity, []File{exactMaxSize}); err != nil {
		t.Fatalf("ValidateQuota exact 10MB err = %v, want nil", err)
	}
	hugeSize := validFile
	hugeSize.Size = 10*1024*1024 + 1
	if err := ValidateQuota(PurposeIdentity, []File{hugeSize}); !errors.Is(err, ErrQuota) {
		t.Fatalf("ValidateQuota huge size err = %v, want ErrQuota", err)
	}

	// Image with pages 0, 1, or > 1
	zeroPagesImg := validFile
	zeroPagesImg.Pages = 0
	if err := ValidateQuota(PurposeIdentity, []File{zeroPagesImg}); err != nil {
		t.Fatalf("ValidateQuota image with 0 pages err = %v, want nil", err)
	}
	badPages := validFile
	badPages.Pages = 2
	if err := ValidateQuota(PurposeIdentity, []File{badPages}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("ValidateQuota image with 2 pages err = %v, want ErrInvalid", err)
	}

	// PDF with pages < 1 or > 5 (identity), and exactly 5
	zeroPagePDF := validFile
	zeroPagePDF.MIME = "application/pdf"
	zeroPagePDF.Pages = 0
	if err := ValidateQuota(PurposeIdentity, []File{zeroPagePDF}); !errors.Is(err, ErrQuota) {
		t.Fatalf("ValidateQuota PDF 0 pages err = %v, want ErrQuota", err)
	}
	fivePagePDF := validFile
	fivePagePDF.MIME = "application/pdf"
	fivePagePDF.Pages = 5
	if err := ValidateQuota(PurposeIdentity, []File{fivePagePDF}); err != nil {
		t.Fatalf("ValidateQuota identity PDF 5 pages err = %v, want nil", err)
	}
	sixPagePDF := validFile
	sixPagePDF.MIME = "application/pdf"
	sixPagePDF.Pages = 6
	if err := ValidateQuota(PurposeIdentity, []File{sixPagePDF}); !errors.Is(err, ErrQuota) {
		t.Fatalf("ValidateQuota identity PDF 6 pages err = %v, want ErrQuota", err)
	}

	// Space PDF with pages > 20, and exactly 20
	twentyPagePDF := validFile
	twentyPagePDF.MIME = "application/pdf"
	twentyPagePDF.Pages = 20
	if err := ValidateQuota(PurposeSpace, []File{twentyPagePDF}); err != nil {
		t.Fatalf("ValidateQuota space PDF 20 pages err = %v, want nil", err)
	}
	twentyOnePagePDF := validFile
	twentyOnePagePDF.MIME = "application/pdf"
	twentyOnePagePDF.Pages = 21
	if err := ValidateQuota(PurposeSpace, []File{twentyOnePagePDF}); !errors.Is(err, ErrQuota) {
		t.Fatalf("ValidateQuota space PDF 21 pages err = %v, want ErrQuota", err)
	}

	// Unknown MIME type
	textMime := validFile
	textMime.MIME = "text/plain"
	if err := ValidateQuota(PurposeIdentity, []File{textMime}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("ValidateQuota text/plain err = %v, want ErrInvalid", err)
	}

	// 3. validSnapshot boundaries
	now := time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)
	baseSnap := Snapshot{
		ID:                            caseID,
		SubjectID:                     subject,
		Purpose:                       PurposeIdentity,
		State:                         CaseInReview,
		Revision:                      1,
		IdentityDeclarationRevisionID: declarationRevisionID,
		DocumentFormatDeclared:        DocumentFormatNew,
		CaptureConsentVersion:         IdentityCaptureConsentVersion,
		CaptureConsentedAt:            now,
		Files: []File{
			{ID: fileID, EvidenceRole: EvidenceRoleIdentityFront, State: FileClean, Size: 1024, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{1}},
		},
	}
	if !validSnapshot(baseSnap) {
		t.Fatal("baseSnap should be valid")
	}

	// Invalid purpose
	snapBadPurpose := baseSnap
	snapBadPurpose.Purpose = Purpose("unknown")
	if validSnapshot(snapBadPurpose) {
		t.Fatal("expected invalid for unknown purpose")
	}

	// SpaceID on identity
	snapSpaceOnIdentity := baseSnap
	snapSpaceOnIdentity.SpaceID = space
	if validSnapshot(snapSpaceOnIdentity) {
		t.Fatal("expected invalid for SpaceID on identity case")
	}

	// PurposeSpace with invalid SpaceID
	snapSpaceBadID := baseSnap
	snapSpaceBadID.Purpose = PurposeSpace
	snapSpaceBadID.SpaceID = "not-uuid"
	snapSpaceBadID.IdentityDeclarationRevisionID = ""
	snapSpaceBadID.DocumentFormatDeclared = ""
	snapSpaceBadID.CaptureConsentVersion = ""
	snapSpaceBadID.CaptureConsentedAt = time.Time{}
	snapSpaceBadID.Files = []File{validFile}
	if validSnapshot(snapSpaceBadID) {
		t.Fatal("expected invalid for space case with invalid SpaceID")
	}

	// PurposeSpace valid
	snapSpaceBadID.SpaceID = space
	if !validSnapshot(snapSpaceBadID) {
		t.Fatal("expected valid for space case with valid SpaceID")
	}

	// Superseded on identity
	snapSupersededIdentity := baseSnap
	snapSupersededIdentity.SupersededAt = &now
	if validSnapshot(snapSupersededIdentity) {
		t.Fatal("expected invalid for superseded identity case")
	}

	// Superseded on space is valid
	snapSpaceBadID.SupersededAt = &now
	if !validSnapshot(snapSpaceBadID) {
		t.Fatal("expected valid for superseded space case")
	}

	// Invalid declared document format
	snapBadDeclared := baseSnap
	snapBadDeclared.DocumentFormatDeclared = DocumentFormat("bad")
	if validSnapshot(snapBadDeclared) {
		t.Fatal("expected invalid for bad declared format")
	}

	// Invalid observed document format
	snapBadObserved := baseSnap
	snapBadObserved.DocumentFormatObserved = DocumentFormat("bad")
	if validSnapshot(snapBadObserved) {
		t.Fatal("expected invalid for bad observed format")
	}

	// Consent version set without timestamp
	snapConsentNoTime := baseSnap
	snapConsentNoTime.CaptureConsentedAt = time.Time{}
	if validSnapshot(snapConsentNoTime) {
		t.Fatal("expected invalid for consent version without timestamp")
	}

	// Consent timestamp without version
	snapConsentNoVersion := baseSnap
	snapConsentNoVersion.CaptureConsentVersion = ""
	if validSnapshot(snapConsentNoVersion) {
		t.Fatal("expected invalid for consent timestamp without version")
	}

	// Wrong consent version
	snapConsentWrongVer := baseSnap
	snapConsentWrongVer.CaptureConsentVersion = "v999"
	if validSnapshot(snapConsentWrongVer) {
		t.Fatal("expected invalid for wrong consent version")
	}

	// PurposeSpace with identity declaration metadata
	snapSpaceWithDecl := snapSpaceBadID
	snapSpaceWithDecl.SupersededAt = nil
	snapSpaceWithDecl.IdentityDeclarationRevisionID = declarationRevisionID
	if validSnapshot(snapSpaceWithDecl) {
		t.Fatal("expected invalid for space case with identity declaration ID")
	}

	// Files with PDF and evidence role
	snapPDFWithRole := baseSnap
	snapPDFWithRole.Files = []File{
		{ID: fileID, EvidenceRole: EvidenceRoleIdentityFront, State: FileClean, Size: 1024, MIME: "application/pdf", Pages: 1, ContentSHA256: [32]byte{1}},
	}
	if validSnapshot(snapPDFWithRole) {
		t.Fatal("expected invalid for PDF file with evidence role")
	}

	// Files with duplicate evidence role
	snapDupRole := baseSnap
	snapDupRole.Files = []File{
		{ID: fileID, EvidenceRole: EvidenceRoleIdentityFront, State: FileClean, Size: 1024, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{1}},
		{ID: "66666666-6666-4666-8666-666666666666", EvidenceRole: EvidenceRoleIdentityFront, State: FileClean, Size: 1024, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{2}},
	}
	if validSnapshot(snapDupRole) {
		t.Fatal("expected invalid for duplicate evidence role")
	}

	// Files with invalid evidence role
	snapInvalidRole := baseSnap
	snapInvalidRole.Files = []File{
		{ID: fileID, EvidenceRole: EvidenceRole("invalid_role"), State: FileClean, Size: 1024, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{1}},
	}
	if validSnapshot(snapInvalidRole) {
		t.Fatal("expected invalid for invalid evidence role")
	}

	// Files with empty evidence role when declared format is set
	snapEmptyRole := baseSnap
	snapEmptyRole.Files = []File{
		{ID: fileID, EvidenceRole: "", State: FileClean, Size: 1024, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{1}},
	}
	if validSnapshot(snapEmptyRole) {
		t.Fatal("expected invalid for empty evidence role with declared format")
	}

	// Malformed snapshot ID and SubjectID
	snapBadID := baseSnap
	snapBadID.ID = "not-a-uuid"
	if validSnapshot(snapBadID) {
		t.Fatal("expected invalid for malformed snapshot ID")
	}
	snapBadSubject := baseSnap
	snapBadSubject.SubjectID = "not-a-uuid"
	if validSnapshot(snapBadSubject) {
		t.Fatal("expected invalid for malformed snapshot SubjectID")
	}

	// Active state without declaration link and terminal state with declaration link
	snapActiveNoDecl := baseSnap
	snapActiveNoDecl.IdentityDeclarationRevisionID = ""
	if validSnapshot(snapActiveNoDecl) {
		t.Fatal("expected invalid for active identity case without declaration link")
	}
	snapTerminalWithDecl := baseSnap
	snapTerminalWithDecl.State = CaseRejected
	snapTerminalWithDecl.IdentityDeclarationRevisionID = declarationRevisionID
	if validSnapshot(snapTerminalWithDecl) {
		t.Fatal("expected invalid for rejected identity case with active declaration link")
	}

	// Snapshot with quota failure (oversized file)
	snapOversize := baseSnap
	snapOversize.Files = []File{
		{ID: fileID, EvidenceRole: EvidenceRoleIdentityFront, State: FileClean, Size: 11 * 1024 * 1024, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{1}},
	}
	if validSnapshot(snapOversize) {
		t.Fatal("expected invalid for snapshot violating file size quota")
	}
}

type minimalRepoNoReader struct{}

func (*minimalRepoNoReader) CreateCase(context.Context, string, Purpose, string, func(Authorization) (Snapshot, error)) (Snapshot, error) {
	return Snapshot{}, nil
}

func (*minimalRepoNoReader) Transition(context.Context, string, Purpose, string, uint64, TransitionAudit, func(*Snapshot, Authorization) error) error {
	return nil
}

type errQueryRepo struct {
	minimalRepoNoReader
	err error
}

func (e *errQueryRepo) ListOwnCases(context.Context, string) ([]Snapshot, error) {
	return nil, e.err
}

func (e *errQueryRepo) LoadOwnCase(context.Context, string, string) (Snapshot, error) {
	return Snapshot{}, e.err
}

func TestServiceTransitionsAndQueryBoundaries(t *testing.T) {
	// 1. Pure helper function boundaries (negative and positive branches)
	if p := purposeForCap(Capability("unknown")); p != "" {
		t.Fatalf("purposeForCap(unknown) = %q, want empty", p)
	}
	if p := purposeForCap(CapabilityIdentity); p != PurposeIdentity {
		t.Fatalf("purposeForCap(Identity) = %q, want %q", p, PurposeIdentity)
	}
	if p := purposeForCap(CapabilitySpace); p != PurposeSpace {
		t.Fatalf("purposeForCap(Space) = %q, want %q", p, PurposeSpace)
	}

	if scope := reviewerScopeID(Purpose("unknown"), Snapshot{}); scope != "" {
		t.Fatalf("reviewerScopeID(unknown) = %q, want empty", scope)
	}
	if scope := reviewerScopeID(PurposeIdentity, Snapshot{SubjectID: subject}); scope != subject {
		t.Fatalf("reviewerScopeID(Identity) = %q, want %q", scope, subject)
	}
	if scope := reviewerScopeID(PurposeSpace, Snapshot{SpaceID: space}); scope != space {
		t.Fatalf("reviewerScopeID(Space) = %q, want %q", scope, space)
	}

	if validReviewReason(Decision("unknown"), ReasonDocumentInvalid) {
		t.Fatal("expected validReviewReason=false for unknown decision")
	}
	if validReviewReason(DecisionApprove, ReasonDocumentInvalid) {
		t.Fatal("expected validReviewReason=false for approve with reason")
	}
	if !validReviewReason(DecisionApprove, "") {
		t.Fatal("expected validReviewReason=true for approve with empty reason")
	}
	if validReviewReason(DecisionNeedsEvidence, ReasonDocumentInvalid) {
		t.Fatal("expected validReviewReason=false for needs_evidence with invalid reason code")
	}
	if !validReviewReason(DecisionNeedsEvidence, ReasonEvidenceIncomplete) {
		t.Fatal("expected validReviewReason=true for needs_evidence with ReasonEvidenceIncomplete")
	}
	for _, rejectReason := range []ReasonCode{ReasonDocumentUnreadable, ReasonDocumentInvalid, ReasonDocumentExpired, ReasonOwnershipChanged} {
		if !validReviewReason(DecisionReject, rejectReason) {
			t.Fatalf("expected validReviewReason=true for reject with %q", rejectReason)
		}
	}

	if validRevokeReason(ReasonCode("unknown")) {
		t.Fatal("expected validRevokeReason=false for unknown reason")
	}
	for _, revokeReason := range []ReasonCode{ReasonDocumentInvalid, ReasonDocumentExpired, ReasonOwnershipChanged, ReasonConflictOfInterest, ReasonOwnerRequest} {
		if !validRevokeReason(revokeReason) {
			t.Fatalf("expected validRevokeReason=true for %q", revokeReason)
		}
	}

	if allClean(nil) || allClean([]File{}) {
		t.Fatal("expected allClean=false for empty files")
	}
	if !allClean([]File{{State: FileClean}}) {
		t.Fatal("expected allClean=true for clean files")
	}

	// 2. Query boundaries on nil receivers / invalid inputs
	var nilSvc *Service
	if _, err := nilSvc.ListOwnCases(context.Background(), subject); !errors.Is(err, ErrInvalid) {
		t.Fatalf("nil.ListOwnCases err=%v, want ErrInvalid", err)
	}
	if _, err := nilSvc.LoadOwnCase(context.Background(), subject, caseID); !errors.Is(err, ErrInvalid) {
		t.Fatalf("nil.LoadOwnCase err=%v, want ErrInvalid", err)
	}
	if _, err := nilSvc.Start(context.Background(), subject, CreateInput{}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("nil.Start err=%v, want ErrInvalid", err)
	}
	if err := nilSvc.Submit(context.Background(), subject, caseID, PurposeIdentity, 1); !errors.Is(err, ErrInvalid) {
		t.Fatalf("nil.Submit err=%v, want ErrInvalid", err)
	}
	if err := nilSvc.AddFile(context.Background(), subject, caseID, PurposeIdentity, 1, File{}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("nil.AddFile err=%v, want ErrInvalid", err)
	}

	// 3. Repo without OwnCaseReader interface
	svcNoReader, err := NewService(&minimalRepoNoReader{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svcNoReader.ListOwnCases(context.Background(), subject); !errors.Is(err, ErrInvalid) {
		t.Fatalf("ListOwnCases without OwnCaseReader err=%v, want ErrInvalid", err)
	}
	if _, err := svcNoReader.LoadOwnCase(context.Background(), subject, caseID); !errors.Is(err, ErrInvalid) {
		t.Fatalf("LoadOwnCase without OwnCaseReader err=%v, want ErrInvalid", err)
	}

	// 4. Cancelled context propagation on queries
	s, r, _ := fixture(t, PurposeIdentity)
	ctxCancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.ListOwnCases(ctxCancelled, subject); !errors.Is(err, context.Canceled) {
		t.Fatalf("ListOwnCases cancelled ctx err=%v, want context.Canceled", err)
	}
	if _, err := s.LoadOwnCase(ctxCancelled, subject, caseID); !errors.Is(err, context.Canceled) {
		t.Fatalf("LoadOwnCase cancelled ctx err=%v, want context.Canceled", err)
	}

	// 5. Repository error propagation on queries
	expErr := errors.New("repo failure")
	svcWithErr, err := NewService(&errQueryRepo{err: expErr})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svcWithErr.ListOwnCases(context.Background(), subject); !errors.Is(err, expErr) {
		t.Fatalf("ListOwnCases repo err=%v, want expErr", err)
	}
	if _, err := svcWithErr.LoadOwnCase(context.Background(), subject, caseID); !errors.Is(err, expErr) {
		t.Fatalf("LoadOwnCase repo err=%v, want expErr", err)
	}

	// 6. AddFile validations
	now := time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)
	validDraft := Snapshot{
		ID:                            caseID,
		SubjectID:                     subject,
		Purpose:                       PurposeIdentity,
		State:                         CaseDraft,
		Revision:                      1,
		IdentityDeclarationRevisionID: declarationRevisionID,
		DocumentFormatDeclared:        DocumentFormatLegacy,
		CaptureConsentVersion:         IdentityCaptureConsentVersion,
		CaptureConsentedAt:            now,
		Files: []File{
			{ID: fileID, EvidenceRole: EvidenceRoleIdentityFront, State: FileClean, Size: 100, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{1}},
		},
	}
	r.state = validDraft
	r.auth.ActorID = subject

	// File not pending
	cleanFile := File{ID: fileID, State: FileClean, Size: 100, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{1}}
	if err := s.AddFile(context.Background(), subject, caseID, PurposeIdentity, 1, cleanFile); !errors.Is(err, ErrInvalid) {
		t.Fatalf("AddFile non-pending err=%v, want ErrInvalid", err)
	}
	// EvidenceRole set on Space case
	pendingWithRole := File{ID: fileID, EvidenceRole: EvidenceRoleIdentityFront, State: FilePending, Size: 100, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{1}}
	if err := s.AddFile(context.Background(), subject, caseID, PurposeSpace, 1, pendingWithRole); !errors.Is(err, ErrInvalid) {
		t.Fatalf("AddFile role on Space err=%v, want ErrInvalid", err)
	}
	// Identity case replacement of existing evidence role
	replacementFile := File{ID: uuid.NewString(), EvidenceRole: EvidenceRoleIdentityFront, State: FilePending, Size: 200, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{2}}
	if err := s.AddFile(context.Background(), subject, caseID, PurposeIdentity, r.state.Revision, replacementFile); err != nil {
		t.Fatalf("AddFile replacement failed: %v", err)
	}
	if len(r.state.Files) != 1 || r.state.Files[0].ID != replacementFile.ID {
		t.Fatalf("expected replacement in r.state.Files, got %+v", r.state.Files)
	}

	// 7. Submit validations
	// Identity draft with no files -> ErrEvidenceIncomplete
	r.state.Files = nil
	if err := s.Submit(context.Background(), subject, caseID, PurposeIdentity, r.state.Revision); !errors.Is(err, ErrEvidenceIncomplete) {
		t.Fatalf("Submit identity no files err=%v, want ErrEvidenceIncomplete", err)
	}
	// Space draft with no files -> ErrFilesNotClean
	r.state.Purpose = PurposeSpace
	r.state.SpaceID = space
	r.state.DocumentFormatDeclared = ""
	r.state.IdentityDeclarationRevisionID = ""
	r.state.CaptureConsentVersion = ""
	r.state.CaptureConsentedAt = time.Time{}
	if err := s.Submit(context.Background(), subject, caseID, PurposeSpace, r.state.Revision); !errors.Is(err, ErrFilesNotClean) {
		t.Fatalf("Submit space no files err=%v, want ErrFilesNotClean", err)
	}
	r.state.Purpose = PurposeIdentity
	r.state.SpaceID = ""
	r.state.DocumentFormatDeclared = DocumentFormatLegacy
	r.state.IdentityDeclarationRevisionID = declarationRevisionID
	r.state.CaptureConsentVersion = IdentityCaptureConsentVersion
	r.state.CaptureConsentedAt = now
	// Incomplete identity files (missing selfie)
	r.state.Files = []File{
		{ID: uuid.NewString(), EvidenceRole: EvidenceRoleIdentityFront, State: FileClean, Size: 100, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{1}},
		{ID: uuid.NewString(), EvidenceRole: EvidenceRoleIdentityBack, State: FileClean, Size: 100, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{2}},
	}
	if err := s.Submit(context.Background(), subject, caseID, PurposeIdentity, r.state.Revision); !errors.Is(err, ErrEvidenceIncomplete) {
		t.Fatalf("Submit missing selfie err=%v, want ErrEvidenceIncomplete", err)
	}
	// Non-clean file
	r.state.Files = append(r.state.Files, File{ID: uuid.NewString(), EvidenceRole: EvidenceRoleIdentitySelfie, State: FilePending, Size: 100, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{3}})
	if err := s.Submit(context.Background(), subject, caseID, PurposeIdentity, r.state.Revision); !errors.Is(err, ErrFilesNotClean) {
		t.Fatalf("Submit with pending file err=%v, want ErrFilesNotClean", err)
	}
	// All clean -> Submit succeeds
	r.state.Files[2].State = FileClean
	if err := s.Submit(context.Background(), subject, caseID, PurposeIdentity, r.state.Revision); err != nil {
		t.Fatalf("expected Submit to succeed, got %v", err)
	}
	if r.state.State != CaseSubmitted {
		t.Fatalf("expected state to transition to CaseSubmitted, got %q", r.state.State)
	}

	// Submit on non-draft/non-needs_evidence state -> ErrTransition
	if err := s.Submit(context.Background(), subject, caseID, PurposeIdentity, r.state.Revision); !errors.Is(err, ErrTransition) {
		t.Fatalf("Submit on submitted case err=%v, want ErrTransition", err)
	}

	// Submit from CaseNeedsEvidence -> CaseSubmitted
	r.state.State = CaseNeedsEvidence
	if err := s.Submit(context.Background(), subject, caseID, PurposeIdentity, r.state.Revision); err != nil {
		t.Fatalf("Submit from CaseNeedsEvidence failed: %v", err)
	}
	if r.state.State != CaseSubmitted {
		t.Fatalf("expected state to transition to CaseSubmitted from NeedsEvidence, got %q", r.state.State)
	}

	// Submit with PDF in identity photo role fails validSnapshot in transact -> ErrNotFound
	r.state.State = CaseDraft
	r.state.Files[0].MIME = "application/pdf"
	if err := s.Submit(context.Background(), subject, caseID, PurposeIdentity, r.state.Revision); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Submit with PDF in photo role err=%v, want ErrNotFound", err)
	}
	r.state.Files[0].MIME = "image/jpeg"
}
