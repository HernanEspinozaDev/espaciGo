package gcp03origin

import "testing"

func TestEvaluateRuntimeMembershipPostcheck(t *testing.T) {
	tests := []struct {
		name           string
		roleReady      bool
		effectiveSet   bool
		memberships    []RuntimeMembershipGrant
		wantStage      string
		wantPassed     bool
		wantGrantor    string
		wantMembership bool
		wantRowCount   int
	}{
		{
			name:         "zero rows is explicit",
			roleReady:    true,
			effectiveSet: false,
			memberships:  []RuntimeMembershipGrant{},
			wantStage:    "postcheck_no_membership",
		},
		{
			name:         "one safe row accepts its actual inherited grantor",
			roleReady:    true,
			effectiveSet: true,
			memberships: []RuntimeMembershipGrant{{
				Grantor: "membership_admin", AdminOption: false, InheritOption: false, SetOption: true,
			}},
			wantPassed:     true,
			wantGrantor:    "membership_admin",
			wantMembership: true,
			wantRowCount:   1,
		},
		{
			name:         "multiple rows are preserved and rejected",
			roleReady:    true,
			effectiveSet: true,
			memberships: []RuntimeMembershipGrant{
				{Grantor: "fixture_control", AdminOption: false, InheritOption: true, SetOption: true},
				{Grantor: "membership_admin", AdminOption: false, InheritOption: false, SetOption: true},
			},
			wantStage:    "postcheck_multiple_memberships",
			wantRowCount: 2,
		},
		{
			name:         "unsafe membership options are rejected and reported",
			roleReady:    true,
			effectiveSet: true,
			memberships: []RuntimeMembershipGrant{{
				Grantor: "membership_admin", AdminOption: true, InheritOption: false, SetOption: true,
			}},
			wantStage:    "postcondition",
			wantGrantor:  "membership_admin",
			wantRowCount: 1,
		},
		{
			name:         "ineffective SET membership is rejected",
			roleReady:    true,
			effectiveSet: false,
			memberships: []RuntimeMembershipGrant{{
				Grantor: "membership_admin", AdminOption: false, InheritOption: false, SetOption: true,
			}},
			wantStage:      "postcondition",
			wantGrantor:    "membership_admin",
			wantMembership: true,
			wantRowCount:   1,
		},
		{
			name:         "unsafe runtime role is rejected",
			roleReady:    false,
			effectiveSet: true,
			memberships: []RuntimeMembershipGrant{{
				Grantor: "membership_admin", AdminOption: false, InheritOption: false, SetOption: true,
			}},
			wantStage:      "postcondition",
			wantGrantor:    "membership_admin",
			wantMembership: true,
			wantRowCount:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report := MembershipReport{
				RuntimeRoleReady: tt.roleReady,
				EffectiveSet:     tt.effectiveSet,
				Memberships:      append([]RuntimeMembershipGrant(nil), tt.memberships...),
			}
			stage := evaluateRuntimeMembershipPostcheck(&report)
			if stage != tt.wantStage {
				t.Fatalf("stage = %q, want %q; report=%+v", stage, tt.wantStage, report)
			}
			if report.Passed != tt.wantPassed || report.Grantor != tt.wantGrantor || report.MembershipSet != tt.wantMembership || report.MembershipRowCount != tt.wantRowCount {
				t.Fatalf("report = %+v; want passed=%t grantor=%q membership=%t rows=%d", report, tt.wantPassed, tt.wantGrantor, tt.wantMembership, tt.wantRowCount)
			}
			if len(report.Memberships) != len(tt.memberships) {
				t.Fatalf("reported membership rows = %d, want %d", len(report.Memberships), len(tt.memberships))
			}
			for i := range tt.memberships {
				if report.Memberships[i] != tt.memberships[i] {
					t.Fatalf("reported membership[%d] = %+v, want %+v", i, report.Memberships[i], tt.memberships[i])
				}
			}
		})
	}
}
