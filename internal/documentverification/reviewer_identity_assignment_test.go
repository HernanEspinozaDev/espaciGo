package documentverification

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestTypedIdentityReviewRequiresExactCurrentAssignment(t *testing.T) {
	const otherReviewer = "99999999-9999-4999-8999-999999999999"
	const otherScope = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"

	cases := []struct {
		name   string
		grants []ReviewerGrant
		want   error
	}{
		{name: "missing assignment", want: ErrCapability},
		{
			name: "wrong subject scope",
			grants: []ReviewerGrant{{
				ReviewerID: actor,
				Capability: CapabilityIdentity,
				ScopeID:    otherScope,
			}},
			want: ErrCapability,
		},
		{
			name: "assignment belongs to another reviewer",
			grants: []ReviewerGrant{{
				ReviewerID: otherReviewer,
				Capability: CapabilityIdentity,
				ScopeID:    subject,
			}},
			want: ErrCapability,
		},
		{
			name: "space capability cannot authorize identity",
			grants: []ReviewerGrant{{
				ReviewerID: actor,
				Capability: CapabilitySpace,
				ScopeID:    subject,
			}},
			want: ErrCapability,
		},
		{
			name: "exact identity assignment",
			grants: []ReviewerGrant{{
				ReviewerID: actor,
				Capability: CapabilityIdentity,
				ScopeID:    subject,
			}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			service, repo, _ := fixture(t, PurposeIdentity)
			service.SetClock(func() time.Time { return time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC) })
			repo.state.Revision = 4
			repo.state.IdentityDeclarationRevisionID = declarationRevisionID
			repo.state.DocumentFormatDeclared = DocumentFormatNew
			repo.state.CaptureConsentVersion = IdentityCaptureConsentVersion
			repo.state.CaptureConsentedAt = time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
			repo.state.DocumentExpiresAt = time.Time{}
			repo.state.Files = []File{
				{ID: "11111111-1111-4111-8111-111111111121", EvidenceRole: EvidenceRoleIdentityFront, State: FileClean, Size: 4, MIME: "image/png", Pages: 1, ContentSHA256: [32]byte{1}},
				{ID: "11111111-1111-4111-8111-111111111122", EvidenceRole: EvidenceRoleIdentityBack, State: FileClean, Size: 4, MIME: "image/png", Pages: 1, ContentSHA256: [32]byte{2}},
				{ID: "11111111-1111-4111-8111-111111111123", EvidenceRole: EvidenceRoleIdentitySelfie, State: FileClean, Size: 4, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{3}},
			}
			repo.auth.ReviewerGrants = append([]ReviewerGrant(nil), tc.grants...)
			beforeState := repo.state
			beforeAudit := repo.audit

			err := service.ReviewIdentityWithReason(
				context.Background(),
				actor,
				caseID,
				DecisionApprove,
				"",
				DocumentFormatNew,
				time.Date(2030, 5, 1, 0, 0, 0, 0, time.UTC),
				passedIdentityReviewChecklist(),
				time.Time{},
				4,
			)
			if tc.want != nil {
				if !errors.Is(err, tc.want) {
					t.Fatalf("review error=%v, want %v", err, tc.want)
				}
				if !reflect.DeepEqual(repo.state, beforeState) || !reflect.DeepEqual(repo.audit, beforeAudit) {
					t.Fatal("rejected identity review mutated the case or audit")
				}
				return
			}
			if err != nil {
				t.Fatalf("exact identity assignment rejected: %v", err)
			}
			if repo.state.State != CaseApproved || repo.state.Revision != 5 || repo.audit.Action != "review_decision" {
				t.Fatalf("exact identity review was not applied: case=%+v audit=%+v", repo.state, repo.audit)
			}
		})
	}
}
