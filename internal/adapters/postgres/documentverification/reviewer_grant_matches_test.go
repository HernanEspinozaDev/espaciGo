package documentverificationpg

import (
	"testing"
	"time"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
)

func TestReviewerGrantMatchesRequiresExactPurposeScope(t *testing.T) {
	const (
		actor    = "11111111-1111-4111-8111-111111111111"
		subjectA = "22222222-2222-4222-8222-222222222222"
		subjectB = "33333333-3333-4333-8333-333333333333"
		spaceA   = "44444444-4444-4444-8444-444444444444"
		spaceB   = "55555555-5555-4555-8555-555555555555"
		caseID   = "66666666-6666-4666-8666-666666666666"
	)

	identityCase := dv.Snapshot{ID: caseID, Purpose: dv.PurposeIdentity, SubjectID: subjectA}
	spaceCase := dv.Snapshot{ID: caseID, Purpose: dv.PurposeSpace, SubjectID: subjectA, SpaceID: spaceA}
	identityGrant := dv.ReviewerGrant{ReviewerID: actor, Capability: dv.CapabilityIdentity, ScopeID: subjectA}
	spaceGrant := dv.ReviewerGrant{ReviewerID: actor, Capability: dv.CapabilitySpace, ScopeID: spaceA}
	at := time.Unix(1, 0)
	supersededCase := identityCase
	supersededCase.SupersededAt = &at

	cases := []struct {
		name   string
		grants []dv.ReviewerGrant
		actor  string
		case_  dv.Snapshot
		want   bool
	}{
		{name: "exact identity assignment", grants: []dv.ReviewerGrant{identityGrant}, actor: actor, case_: identityCase, want: true},
		{name: "identity scope mismatch", grants: []dv.ReviewerGrant{{ReviewerID: actor, Capability: dv.CapabilityIdentity, ScopeID: subjectB}}, actor: actor, case_: identityCase},
		{name: "grant belongs to another reviewer", grants: []dv.ReviewerGrant{{ReviewerID: subjectB, Capability: dv.CapabilityIdentity, ScopeID: subjectA}}, actor: actor, case_: identityCase},
		{name: "space capability cannot authorize identity", grants: []dv.ReviewerGrant{{ReviewerID: actor, Capability: dv.CapabilitySpace, ScopeID: subjectA}}, actor: actor, case_: identityCase},
		{name: "mixed capabilities fail closed", grants: []dv.ReviewerGrant{identityGrant, spaceGrant}, actor: actor, case_: identityCase},
		{name: "additional exact identity assignments remain separately scoped", grants: []dv.ReviewerGrant{identityGrant, {ReviewerID: actor, Capability: dv.CapabilityIdentity, ScopeID: subjectB}}, actor: actor, case_: identityCase, want: true},
		{name: "exact space assignment", grants: []dv.ReviewerGrant{spaceGrant}, actor: actor, case_: spaceCase, want: true},
		{name: "space scope mismatch", grants: []dv.ReviewerGrant{{ReviewerID: actor, Capability: dv.CapabilitySpace, ScopeID: spaceB}}, actor: actor, case_: spaceCase},
		{name: "identity capability cannot authorize space", grants: []dv.ReviewerGrant{{ReviewerID: actor, Capability: dv.CapabilityIdentity, ScopeID: spaceA}}, actor: actor, case_: spaceCase},
		{name: "self review denied", grants: []dv.ReviewerGrant{{ReviewerID: subjectA, Capability: dv.CapabilityIdentity, ScopeID: subjectA}}, actor: subjectA, case_: dv.Snapshot{ID: caseID, Purpose: dv.PurposeIdentity, SubjectID: subjectA}},
		{name: "superseded case denied", grants: []dv.ReviewerGrant{identityGrant}, actor: actor, case_: supersededCase},
		{name: "unknown purpose denied", grants: []dv.ReviewerGrant{identityGrant}, actor: actor, case_: dv.Snapshot{ID: caseID, Purpose: dv.Purpose("unknown"), SubjectID: subjectA}},
		{name: "malformed unrelated grant invalidates input", grants: []dv.ReviewerGrant{identityGrant, {ReviewerID: actor, Capability: dv.Capability("unknown"), ScopeID: subjectB}}, actor: actor, case_: identityCase},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := reviewerGrantMatches(tc.grants, tc.actor, tc.case_); got != tc.want {
				t.Fatalf("reviewerGrantMatches()=%t, want %t", got, tc.want)
			}
		})
	}
}
