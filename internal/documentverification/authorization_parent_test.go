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
	if err := s.Create(context.Background(), actor, CreateInput{Purpose: PurposeIdentity, DocumentExpiresAt: time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)}); !errors.Is(err, ErrCapability) {
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

func TestReviewHonorsRequestedExpiryEarlierThanDocumentExpiry(t *testing.T) {
	now := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	s, r, x := fixture(t, PurposeIdentity)
	s.SetClock(func() time.Time { return now })
	requestedExpiry := now.AddDate(0, 6, 0)
	r.state.DocumentExpiresAt = requestedExpiry.Add(time.Nanosecond)
	if err := s.Review(context.Background(), actor, x.ID, CapabilityIdentity, DecisionApprove, requestedExpiry, 0); err != nil {
		t.Fatal(err)
	}
	if !r.state.VerifiedUntil.Equal(requestedExpiry) {
		t.Fatalf("verifiedUntil=%s; want earlier requested expiry %s", r.state.VerifiedUntil, requestedExpiry)
	}
}

func FuzzReviewUsesEarliestIdentityValidityBoundary(f *testing.F) {
	f.Add(uint16(0), uint16(30), uint16(0))
	f.Add(uint16(59), uint16(900), uint16(180))
	f.Add(uint16(365), uint16(4000), uint16(5000))
	f.Add(uint16(59), uint16(3000), uint16(5000))
	f.Fuzz(func(t *testing.T, dateSeed, documentDaysSeed, requestedDaysSeed uint16) {
		year := 2000 + int(dateSeed%40)
		dayOfYear := int(dateSeed % 366)
		now := time.Date(year, time.January, 1, 12, 0, 0, 0, time.UTC).AddDate(0, 0, dayOfYear)
		documentExpiry := now.AddDate(0, 0, int(documentDaysSeed%4000)+1)
		requestedDays := int(requestedDaysSeed % 5001)
		requestedExpiry := time.Time{}
		if requestedDays > 0 {
			requestedExpiry = now.AddDate(0, 0, requestedDays)
		}

		want := now.AddDate(0, 24, 0)
		if documentExpiry.Before(want) {
			want = documentExpiry
		}
		if !requestedExpiry.IsZero() && requestedExpiry.Before(want) {
			want = requestedExpiry
		}

		s, repository, x := fixture(t, PurposeIdentity)
		s.SetClock(func() time.Time { return now })
		repository.state.DocumentExpiresAt = documentExpiry
		if err := s.Review(context.Background(), actor, x.ID, CapabilityIdentity, DecisionApprove, requestedExpiry, 0); err != nil {
			t.Fatalf("valid expiry bounds rejected: doc=%s requested=%s err=%v", documentExpiry, requestedExpiry, err)
		}
		if !repository.state.VerifiedAt.Equal(now) || !repository.state.VerifiedUntil.Equal(want) {
			t.Fatalf("reviewed dates verifiedAt=%s verifiedUntil=%s; want %s and %s", repository.state.VerifiedAt, repository.state.VerifiedUntil, now, want)
		}
	})
}

func TestReviewRejectsRequestedExpiryAtCurrentInstantWithoutMutation(t *testing.T) {
	now := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	s, r, x := fixture(t, PurposeIdentity)
	s.SetClock(func() time.Time { return now })
	r.state.DocumentExpiresAt = now.AddDate(0, 6, 0)
	before := r.state
	if err := s.Review(context.Background(), actor, x.ID, CapabilityIdentity, DecisionApprove, now, 0); !errors.Is(err, ErrInvalid) {
		t.Fatalf("expiry at now must be rejected, got %v", err)
	}
	if r.state.State != before.State || r.state.Revision != before.Revision || r.state.ReviewerID != before.ReviewerID ||
		!r.state.VerifiedAt.Equal(before.VerifiedAt) || !r.state.VerifiedUntil.Equal(before.VerifiedUntil) {
		t.Fatalf("rejected expiry mutated verification state: before=%+v after=%+v", before, r.state)
	}
}

func TestReviewRejectsIdentityWithoutDocumentExpiry(t *testing.T) {
	now := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	s, r, x := fixture(t, PurposeIdentity)
	s.SetClock(func() time.Time { return now })
	r.state.DocumentExpiresAt = time.Time{}
	before := r.state
	if err := s.Review(context.Background(), actor, x.ID, CapabilityIdentity, DecisionApprove, time.Time{}, 0); !errors.Is(err, ErrInvalid) {
		t.Fatalf("identity without document expiry must not be approved: got %v", err)
	}
	if r.state.State != before.State || r.state.Revision != before.Revision || r.state.ReviewerID != before.ReviewerID ||
		!r.state.VerifiedAt.Equal(before.VerifiedAt) || !r.state.VerifiedUntil.Equal(before.VerifiedUntil) {
		t.Fatalf("missing document expiry mutated verification state: before=%+v after=%+v", before, r.state)
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
