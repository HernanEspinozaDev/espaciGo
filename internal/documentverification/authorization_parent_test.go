package documentverification

import (
	"context"
	"errors"
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
func TestCreateCannotCreateIdentityForAnotherAccount(t *testing.T) {
	s, _, _ := fixture(t, PurposeIdentity)
	if err := s.Create(context.Background(), actor, CreateInput{Purpose: PurposeIdentity}); !errors.Is(err, ErrCapability) {
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
	if err := s.Revoke(context.Background(), actor, x.ID, x.Purpose, "synthetic reason", 0); !errors.Is(err, ErrCapability) {
		t.Fatalf("got %v", err)
	}
	r.auth.ReviewerGrants = []ReviewerGrant{{ReviewerID: actor, Capability: CapabilitySpace, ScopeID: space}}
	if err := s.Revoke(context.Background(), actor, x.ID, x.Purpose, "synthetic reason", 0); err != nil {
		t.Fatal(err)
	}
	if r.state.State != CaseRevoked || r.state.RevokedAt == nil || !r.state.RevokedAt.Equal(now) {
		t.Fatal("revocation not immediate")
	}
}
