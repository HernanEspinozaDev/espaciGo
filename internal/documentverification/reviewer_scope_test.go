package documentverification

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestReviewRequiresPersonPurposeAndExactResourceScope(t *testing.T) {
	otherScope := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	for _, tc := range []struct {
		name       string
		purpose    Purpose
		capability Capability
		wrongScope string
	}{
		{name: "identity subject scope", purpose: PurposeIdentity, capability: CapabilityIdentity, wrongScope: otherScope},
		{name: "space ID scope", purpose: PurposeSpace, capability: CapabilitySpace, wrongScope: otherScope},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, r, x := fixture(t, tc.purpose)
			r.auth.ActorID = actor
			r.auth.ReviewerGrants = []ReviewerGrant{{ReviewerID: actor, Capability: tc.capability, ScopeID: tc.wrongScope}}
			if err := s.ReviewWithReason(context.Background(), actor, x.ID, tc.capability, DecisionReject, ReasonDocumentInvalid, time.Time{}, 0); !errors.Is(err, ErrCapability) {
				t.Fatalf("wrong scope accepted: %v", err)
			}
			if r.state.Revision != 0 || r.state.State != CaseSubmitted {
				t.Fatal("wrong-scope review mutated case")
			}

			scope := x.SubjectID
			if tc.purpose == PurposeSpace {
				scope = x.SpaceID
			}
			r.auth.ReviewerGrants = []ReviewerGrant{{ReviewerID: actor, Capability: tc.capability, ScopeID: scope}}
			if err := s.ReviewWithReason(context.Background(), actor, x.ID, tc.capability, DecisionReject, ReasonDocumentInvalid, time.Time{}, 0); err != nil {
				t.Fatalf("exact assigned scope rejected: %v", err)
			}
			if r.state.State != CaseRejected || r.state.Revision != 1 {
				t.Fatal("authorized scoped review did not persist")
			}
		})
	}
}

func TestRevokeRequiresPersonPurposeAndExactResourceScope(t *testing.T) {
	s, r, x := fixture(t, PurposeSpace)
	r.state.State = CaseApproved
	r.auth.ActorID = actor
	r.auth.SpaceOwnerID = subject
	r.auth.ReviewerGrants = []ReviewerGrant{{ReviewerID: actor, Capability: CapabilitySpace, ScopeID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"}}
	if err := s.Revoke(context.Background(), actor, x.ID, PurposeSpace, ReasonOwnerRequest, 0); !errors.Is(err, ErrCapability) {
		t.Fatalf("wrong-scope revocation accepted: %v", err)
	}
	if r.state.State != CaseApproved || r.state.Revision != 0 {
		t.Fatal("wrong-scope revocation mutated case")
	}
	r.auth.ReviewerGrants = []ReviewerGrant{{ReviewerID: actor, Capability: CapabilitySpace, ScopeID: x.SpaceID}}
	if err := s.Revoke(context.Background(), actor, x.ID, PurposeSpace, ReasonOwnerRequest, 0); err != nil {
		t.Fatalf("exact assigned scope rejected: %v", err)
	}
	if r.state.State != CaseRevoked || r.state.Revision != 1 {
		t.Fatal("authorized scoped revocation did not persist")
	}
}

func TestSpaceRevokeRejectsHistoricalOwnerAfterTransfer(t *testing.T) {
	s, r, x := fixture(t, PurposeSpace)
	r.state.State = CaseApproved
	r.auth.ActorID = subject
	r.auth.SpaceOwnerID = actor
	r.auth.ReviewerGrants = []ReviewerGrant{{ReviewerID: subject, Capability: CapabilitySpace, ScopeID: x.SpaceID}}
	before := r.state
	if err := s.Revoke(context.Background(), subject, x.ID, PurposeSpace, ReasonOwnerRequest, 0); !errors.Is(err, ErrConflict) {
		t.Fatalf("old owner revocation err=%v, want conflict requiring new case", err)
	}
	if r.state.State != before.State || r.state.Revision != before.Revision {
		t.Fatal("stale-owner revocation mutated case")
	}
}

func TestReviewerCannotUseUnapprovedDualCapabilityGrant(t *testing.T) {
	s, r, x := fixture(t, PurposeSpace)
	r.auth.ActorID = actor
	r.auth.ReviewerGrants = []ReviewerGrant{
		{ReviewerID: actor, Capability: CapabilitySpace, ScopeID: x.SpaceID},
		{ReviewerID: actor, Capability: CapabilityIdentity, ScopeID: x.SubjectID},
	}
	if err := s.ReviewWithReason(context.Background(), actor, x.ID, CapabilitySpace, DecisionReject, ReasonDocumentInvalid, time.Time{}, 0); !errors.Is(err, ErrCapability) {
		t.Fatalf("dual-capability reviewer accepted without policy approval: %v", err)
	}
	if r.state.State != CaseSubmitted || r.state.Revision != 0 {
		t.Fatal("dual-capability denial mutated case")
	}
}

func TestReviewerAssignmentIsBoundToAuthenticatedPerson(t *testing.T) {
	s, r, x := fixture(t, PurposeSpace)
	r.auth.ActorID = subject
	r.auth.ReviewerGrants = []ReviewerGrant{{ReviewerID: actor, Capability: CapabilitySpace, ScopeID: x.SpaceID}}
	if err := s.ReviewWithReason(context.Background(), actor, x.ID, CapabilitySpace, DecisionReject, ReasonDocumentInvalid, time.Time{}, 0); !errors.Is(err, ErrCapability) {
		t.Fatalf("grant from a different actor accepted: %v", err)
	}
}

func TestReviewerGrantMustMatchAuthenticatedPerson(t *testing.T) {
	s, r, x := fixture(t, PurposeSpace)
	r.auth.ActorID = actor
	r.auth.ReviewerGrants = []ReviewerGrant{{ReviewerID: subject, Capability: CapabilitySpace, ScopeID: x.SpaceID}}
	if err := s.ReviewWithReason(context.Background(), actor, x.ID, CapabilitySpace, DecisionReject, ReasonDocumentInvalid, time.Time{}, 0); !errors.Is(err, ErrCapability) {
		t.Fatalf("grant assigned to another person accepted: %v", err)
	}
	if r.state.State != CaseSubmitted || r.state.Revision != 0 {
		t.Fatal("mismatched-person grant mutated case")
	}
}
