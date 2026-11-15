package documentverification

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestSubmitPersistsStateAndChecksOwner(t *testing.T) {
	s, r, x := fixture(t, PurposeIdentity)
	r.state.State = CaseDraft
	r.state.SpaceID = ""
	r.auth.ActorID = subject
	if err := s.Submit(context.Background(), subject, x.ID, PurposeIdentity, 0); err != nil {
		t.Fatal(err)
	}
	if r.state.State != CaseSubmitted || r.state.Revision != 1 {
		t.Fatalf("submission not persisted: %+v", r.state)
	}
}

func TestTypedIdentitySubmitRejectsMissingEvidenceRoles(t *testing.T) {
	s, r, x := fixture(t, PurposeIdentity)
	r.state.State = CaseDraft
	r.state.IdentityDeclarationRevisionID = declarationRevisionID
	r.state.DocumentFormatDeclared = DocumentFormatLegacy
	r.state.CaptureConsentVersion = IdentityCaptureConsentVersion
	r.state.CaptureConsentedAt = time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	r.state.Files = []File{{ID: fileID, EvidenceRole: EvidenceRoleIdentityFront, State: FileClean, Size: 100, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{1}}}
	r.auth.ActorID = subject
	before := r.state
	if err := s.Submit(context.Background(), subject, x.ID, PurposeIdentity, 0); !errors.Is(err, ErrEvidenceIncomplete) {
		t.Fatalf("submit with incomplete roles error=%v, want ErrEvidenceIncomplete", err)
	}
	if r.state.State != before.State || r.state.Revision != before.Revision {
		t.Fatal("incomplete identity evidence changed case state")
	}
}

func TestTypedIdentityUploadReplacesMatchingRoleSlot(t *testing.T) {
	s, r, x := fixture(t, PurposeIdentity)
	r.state.State = CaseDraft
	r.state.IdentityDeclarationRevisionID = declarationRevisionID
	r.state.DocumentFormatDeclared = DocumentFormatNew
	r.state.CaptureConsentVersion = IdentityCaptureConsentVersion
	r.state.CaptureConsentedAt = time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	r.state.Files = []File{
		{ID: fileID, EvidenceRole: EvidenceRoleIdentityFront, State: FileClean, Size: 100, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{1}},
		{ID: "66666666-6666-4666-8666-666666666666", EvidenceRole: EvidenceRoleIdentityBack, State: FileClean, Size: 100, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{1}},
		{ID: "77777777-7777-4777-8777-777777777777", EvidenceRole: EvidenceRoleIdentitySelfie, State: FileClean, Size: 100, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{1}},
	}
	r.auth.ActorID = subject
	r.auth.SubjectID = subject
	replacement := File{ID: "99999999-9999-4999-8999-999999999999", EvidenceRole: EvidenceRoleIdentityFront, State: FilePending, Size: 120, MIME: "image/png", Pages: 1, ContentSHA256: [32]byte{2}}
	if err := s.AddFile(context.Background(), subject, x.ID, PurposeIdentity, 0, replacement); err != nil {
		t.Fatalf("replace front-side slot: %v", err)
	}
	if len(r.state.Files) != 3 || r.state.Files[0].ID != replacement.ID || r.state.Files[0].State != FilePending || r.state.Revision != 1 {
		t.Fatalf("replacement did not preserve exactly three current slots: %+v", r.state)
	}
}

func TestSubmitAfterEvidenceRequestClearsPriorReviewer(t *testing.T) {
	s, r, x := fixture(t, PurposeIdentity)
	r.state.State = CaseNeedsEvidence
	r.state.ReviewerID = actor
	r.state.Revision = 3
	r.auth.ActorID = subject
	if err := s.Submit(context.Background(), subject, x.ID, PurposeIdentity, 3); err != nil {
		t.Fatal(err)
	}
	if r.state.State != CaseSubmitted || r.state.ReviewerID != "" {
		t.Fatalf("resubmission retained reviewer or wrong state: %+v", r.state)
	}
}

func TestCreateCannotCreateIdentityForAnotherAccount(t *testing.T) {
	s, _, _ := fixture(t, PurposeIdentity)
	if err := s.Create(context.Background(), actor, identityCreateInput(DocumentFormatNew)); !errors.Is(err, ErrCapability) {
		t.Fatalf("foreign subject accepted: %v", err)
	}
}
func TestIdentityRejectsSpaceBindingAndEmptyFiles(t *testing.T) {
	s, _, _ := fixture(t, PurposeIdentity)
	if err := s.Create(context.Background(), actor, CreateInput{Purpose: PurposeIdentity, SpaceID: space}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("identity bound to space: %v", err)
	}
	if err := ValidateQuota(PurposeIdentity, []File{{ID: fileID, State: FileClean, MIME: "image/jpeg", Size: 0, Pages: 1}}); err == nil {
		t.Fatal("empty inspected file accepted")
	}
}
func TestSubmitUncleanFilesCannotAdvance(t *testing.T) {
	for _, state := range []FileState{FilePending, FileBlocked, FileError} {
		t.Run(string(state), func(t *testing.T) {
			s, r, x := fixture(t, PurposeSpace)
			r.state.State = CaseDraft
			r.auth.ActorID = subject
			r.state.Files[0].State = state
			if err := s.Submit(context.Background(), subject, x.ID, x.Purpose, 0); !errors.Is(err, ErrFilesNotClean) {
				t.Fatalf("err=%v", err)
			}
			if r.state.State != CaseDraft || r.state.Revision != 0 {
				t.Fatal("rejected submit mutated")
			}
		})
	}
}

func TestTypedIdentitySubmitRejectsEveryNonCleanRole(t *testing.T) {
	roles := []EvidenceRole{EvidenceRoleIdentityFront, EvidenceRoleIdentityBack, EvidenceRoleIdentitySelfie}
	states := []FileState{FilePending, FileBlocked, FileError}
	for roleIndex, role := range roles {
		for _, state := range states {
			t.Run(string(role)+"/"+string(state), func(t *testing.T) {
				s, r, x := fixture(t, PurposeIdentity)
				r.state.State = CaseDraft
				r.state.IdentityDeclarationRevisionID = declarationRevisionID
				r.state.DocumentFormatDeclared = DocumentFormatLegacy
				r.state.CaptureConsentVersion = IdentityCaptureConsentVersion
				r.state.CaptureConsentedAt = time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
				r.state.Files = []File{
					{ID: fileID, EvidenceRole: roles[0], State: FileClean, Size: 100, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{1}},
					{ID: "66666666-6666-4666-8666-666666666666", EvidenceRole: roles[1], State: FileClean, Size: 100, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{2}},
					{ID: "77777777-7777-4777-8777-777777777777", EvidenceRole: roles[2], State: FileClean, Size: 100, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{3}},
				}
				r.state.Files[roleIndex].State = state
				r.auth.ActorID = subject
				before := r.state
				before.Files = append([]File(nil), r.state.Files...)
				beforeAudit := r.audit

				err := s.Submit(context.Background(), subject, x.ID, PurposeIdentity, 0)
				if !errors.Is(err, ErrFilesNotClean) {
					t.Fatalf("submit with %s in %s state error=%v, want ErrFilesNotClean", role, state, err)
				}
				if !reflect.DeepEqual(r.state, before) || !reflect.DeepEqual(r.audit, beforeAudit) {
					t.Fatalf("rejected submit with %s in %s state mutated case or audit", role, state)
				}
			})
		}
	}
}

func TestReviewAuthorizationAndCurrentIdentity(t *testing.T) {
	now := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name   string
		change func(*memoryRepository)
		want   error
	}{
		{"no-capability", func(r *memoryRepository) { r.auth.ReviewerGrants = nil }, ErrCapability},
		{"self-review", func(r *memoryRepository) { r.state.SubjectID = actor; r.auth.SpaceOwnerID = actor }, ErrSelfReview},
		{"identity-revoked", func(r *memoryRepository) { r.auth.IdentityCurrent = false }, ErrIdentityNotCurrent},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, r, x := fixture(t, PurposeSpace)
			r.auth.ReviewerGrants = []ReviewerGrant{{ReviewerID: actor, Capability: CapabilitySpace, ScopeID: space}}
			s.SetClock(func() time.Time { return now })
			tc.change(r)
			if err := s.Review(context.Background(), actor, x.ID, CapabilitySpace, DecisionApprove, time.Time{}, 0); !errors.Is(err, tc.want) {
				t.Fatalf("got %v want %v", err, tc.want)
			}
			if r.state.State != CaseSubmitted || r.state.Revision != 0 {
				t.Fatal("rejected approval mutated")
			}
		})
	}
}
func TestReviewCapsValidityAndRequestedExpiry(t *testing.T) {
	now := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	for _, p := range []Purpose{PurposeIdentity, PurposeSpace} {
		t.Run(string(p), func(t *testing.T) {
			s, r, x := fixture(t, p)
			s.SetClock(func() time.Time { return now })
			r.state.DocumentExpiresAt = now.AddDate(4, 0, 0)
			cap := CapabilityIdentity
			months := 24
			if p == PurposeSpace {
				r.auth.ReviewerGrants = []ReviewerGrant{{ReviewerID: actor, Capability: CapabilitySpace, ScopeID: space}}
				cap = CapabilitySpace
				months = 12
			}
			if p == PurposeIdentity {
				r.state.SpaceID = ""
			}
			if err := s.Review(context.Background(), actor, x.ID, cap, DecisionApprove, now.AddDate(5, 0, 0), 0); err != nil {
				t.Fatal(err)
			}
			if !r.state.VerifiedUntil.Equal(now.AddDate(0, months, 0)) {
				t.Fatal("validity exceeded policy")
			}
		})
	}
}
func TestRevokeUsesClockAndRequiresCapability(t *testing.T) {
	now := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	s, r, x := fixture(t, PurposeSpace)
	s.SetClock(func() time.Time { return now })
	r.state.State = CaseApproved
	r.auth.ReviewerGrants = nil
	if err := s.Revoke(context.Background(), actor, x.ID, x.Purpose, ReasonOwnerRequest, 0); !errors.Is(err, ErrCapability) {
		t.Fatalf("got %v", err)
	}
	r.auth.ReviewerGrants = []ReviewerGrant{{ReviewerID: actor, Capability: CapabilitySpace, ScopeID: space}}
	if err := s.Revoke(context.Background(), actor, x.ID, x.Purpose, ReasonOwnerRequest, 0); err != nil {
		t.Fatal(err)
	}
	if r.state.State != CaseRevoked || r.state.RevokedAt == nil || !r.state.RevokedAt.Equal(now) {
		t.Fatal("revocation not immediate")
	}
}

func TestIdentityRevokeRequiresExactGrantAndAuditsTransition(t *testing.T) {
	now := time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name  string
		grant []ReviewerGrant
		want  error
	}{
		{name: "missing grant", want: ErrCapability},
		{
			name:  "wrong subject scope",
			grant: []ReviewerGrant{{ReviewerID: actor, Capability: CapabilityIdentity, ScopeID: "88888888-8888-4888-8888-888888888888"}},
			want:  ErrCapability,
		},
		{
			name:  "grant names another reviewer",
			grant: []ReviewerGrant{{ReviewerID: subject, Capability: CapabilityIdentity, ScopeID: subject}},
			want:  ErrCapability,
		},
		{
			name:  "exact grant",
			grant: []ReviewerGrant{{ReviewerID: actor, Capability: CapabilityIdentity, ScopeID: subject}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, r, x := fixture(t, PurposeIdentity)
			s.SetClock(func() time.Time { return now })
			r.state.State = CaseApproved
			r.state.Revision = 4
			r.state.IdentityDeclarationRevisionID = declarationRevisionID
			r.state.DocumentFormatDeclared = DocumentFormatNew
			r.state.DocumentFormatObserved = DocumentFormatNew
			r.state.CaptureConsentVersion = IdentityCaptureConsentVersion
			r.state.CaptureConsentedAt = now.AddDate(0, -1, 0)
			r.state.DocumentExpiresAt = now.AddDate(1, 0, 0)
			r.state.VerifiedAt = now.AddDate(0, -1, 0)
			r.state.VerifiedUntil = now.AddDate(0, 11, 0)
			r.state.Files = []File{
				{ID: fileID, EvidenceRole: EvidenceRoleIdentityFront, State: FileClean, Size: 100, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{1}},
				{ID: "cccccccc-cccc-4ccc-8ccc-cccccccccccc", EvidenceRole: EvidenceRoleIdentityBack, State: FileClean, Size: 100, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{2}},
				{ID: "dddddddd-dddd-4ddd-8ddd-dddddddddddd", EvidenceRole: EvidenceRoleIdentitySelfie, State: FileClean, Size: 100, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{3}},
			}
			r.auth.ReviewerGrants = tc.grant
			before := r.state
			before.Files = append([]File(nil), r.state.Files...)
			beforeAudit := r.audit
			if !validSnapshot(r.state) {
				t.Fatal("approved synthetic identity fixture is invalid")
			}

			err := s.Revoke(context.Background(), actor, x.ID, PurposeIdentity, ReasonDocumentInvalid, 4)
			if tc.want != nil {
				if !errors.Is(err, tc.want) {
					t.Fatalf("revoke error=%v, want %v", err, tc.want)
				}
				if !reflect.DeepEqual(r.state, before) || !reflect.DeepEqual(r.audit, beforeAudit) {
					t.Fatal("denied identity revocation mutated case or audit")
				}
				return
			}
			if err != nil {
				t.Fatalf("revoke with exact identity grant: %v", err)
			}
			if r.state.State != CaseRevoked || r.state.Revision != 5 || r.state.RevokedAt == nil ||
				!r.state.RevokedAt.Equal(now) || r.state.ReviewerID != actor || !r.state.VerifiedUntil.Equal(before.VerifiedUntil) {
				t.Fatalf("identity revocation transition=%+v", r.state)
			}
			if r.audit.Action != "revoked" || r.audit.ResultCode != string(CaseRevoked) || r.audit.ReasonCode != ReasonDocumentInvalid {
				t.Fatalf("identity revocation audit=%+v", r.audit)
			}
		})
	}
}
