package documentverificationpg

import (
	"testing"
	"time"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
)

func TestReviewerGrantMatchesExactReviewerCapabilityAndScope(t *testing.T) {
	actor := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	subject := "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	space := "cccccccc-cccc-4ccc-8ccc-cccccccccccc"
	superseded := time.Unix(1, 0)
	cases := []struct {
		name   string
		x      dv.Snapshot
		grants []dv.ReviewerGrant
		want   bool
	}{
		{"identity exact", dv.Snapshot{Purpose: dv.PurposeIdentity, SubjectID: subject}, []dv.ReviewerGrant{{ReviewerID: actor, Capability: dv.CapabilityIdentity, ScopeID: subject}}, true},
		{"identity wrong subject", dv.Snapshot{Purpose: dv.PurposeIdentity, SubjectID: subject}, []dv.ReviewerGrant{{ReviewerID: actor, Capability: dv.CapabilityIdentity, ScopeID: space}}, false},
		{"space exact space ID", dv.Snapshot{Purpose: dv.PurposeSpace, SubjectID: subject, SpaceID: space}, []dv.ReviewerGrant{{ReviewerID: actor, Capability: dv.CapabilitySpace, ScopeID: space}}, true},
		{"space scope cannot be owner", dv.Snapshot{Purpose: dv.PurposeSpace, SubjectID: subject, SpaceID: space}, []dv.ReviewerGrant{{ReviewerID: actor, Capability: dv.CapabilitySpace, ScopeID: subject}}, false},
		{"grant for another reviewer", dv.Snapshot{Purpose: dv.PurposeIdentity, SubjectID: subject}, []dv.ReviewerGrant{{ReviewerID: subject, Capability: dv.CapabilityIdentity, ScopeID: subject}}, false},
		{"unsupported purpose", dv.Snapshot{Purpose: "unknown", SubjectID: subject}, []dv.ReviewerGrant{{ReviewerID: actor, Capability: dv.CapabilityIdentity, ScopeID: subject}}, false},
		{"incompatible capabilities", dv.Snapshot{Purpose: dv.PurposeIdentity, SubjectID: subject}, []dv.ReviewerGrant{{ReviewerID: actor, Capability: dv.CapabilityIdentity, ScopeID: subject}, {ReviewerID: actor, Capability: dv.CapabilitySpace, ScopeID: space}}, false},
		{"self review", dv.Snapshot{Purpose: dv.PurposeIdentity, SubjectID: actor}, []dv.ReviewerGrant{{ReviewerID: actor, Capability: dv.CapabilityIdentity, ScopeID: actor}}, false},
		{"foreign actor grant mixed with valid", dv.Snapshot{Purpose: dv.PurposeIdentity, SubjectID: subject}, []dv.ReviewerGrant{{ReviewerID: actor, Capability: dv.CapabilityIdentity, ScopeID: subject}, {ReviewerID: subject, Capability: dv.CapabilityIdentity, ScopeID: subject}}, false},
		{"superseded case", dv.Snapshot{Purpose: dv.PurposeIdentity, SubjectID: subject, SupersededAt: &superseded}, []dv.ReviewerGrant{{ReviewerID: actor, Capability: dv.CapabilityIdentity, ScopeID: subject}}, false},
		{"malformed extra scope", dv.Snapshot{Purpose: dv.PurposeIdentity, SubjectID: subject}, []dv.ReviewerGrant{{ReviewerID: actor, Capability: dv.CapabilityIdentity, ScopeID: subject}, {ReviewerID: actor, Capability: dv.CapabilityIdentity, ScopeID: "malformed"}}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := reviewerGrantMatches(tc.grants, actor, tc.x); got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}
