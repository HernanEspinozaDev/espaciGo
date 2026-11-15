package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
)

func TestReviewerIdentityApprovalWithUndeterminedFormatDoesNotMutate(t *testing.T) {
	for _, declaredFormat := range []dv.DocumentFormat{dv.DocumentFormatNew, dv.DocumentFormatUnsure} {
		t.Run(string(declaredFormat), func(t *testing.T) {
			const ownerID = "88888888-8888-4888-8888-888888888888"
			const declarationRevisionID = "99999999-9999-4999-8999-999999999999"
			consentedAt := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
			repo := &revokeLifecycleRepository{
				snapshot: dv.Snapshot{
					ID: reviewerID, SubjectID: ownerID, Purpose: dv.PurposeIdentity,
					State: dv.CaseSubmitted, Revision: 7,
					IdentityDeclarationRevisionID: declarationRevisionID,
					CaptureConsentVersion:         dv.IdentityCaptureConsentVersion,
					CaptureConsentedAt:            consentedAt,
					DocumentFormatDeclared:        declaredFormat,
					Files: []dv.File{
						{ID: "11111111-1111-4111-8111-111111111111", EvidenceRole: dv.EvidenceRoleIdentityFront, State: dv.FileClean, Size: 100, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{1}},
						{ID: "22222222-2222-4222-8222-222222222222", EvidenceRole: dv.EvidenceRoleIdentityBack, State: dv.FileClean, Size: 100, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{2}},
						{ID: "33333333-3333-4333-8333-333333333333", EvidenceRole: dv.EvidenceRoleIdentitySelfie, State: dv.FileClean, Size: 100, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{3}},
					},
				},
				authorization: dv.Authorization{
					ActorID: reviewerActor,
					ReviewerGrants: []dv.ReviewerGrant{{
						ReviewerID: reviewerActor, Capability: dv.CapabilityIdentity, ScopeID: ownerID,
					}},
				},
			}
			beforeSnapshot := repo.snapshot
			beforeAudit := repo.audit
			domain, err := dv.NewService(repo)
			if err != nil {
				t.Fatal(err)
			}
			domain.SetClock(func() time.Time { return time.Date(2026, 11, 11, 17, 0, 0, 0, time.UTC) })
			access, err := dv.NewReviewerAccessService(repo, domain, revokeLifecycleDocumentStore{})
			if err != nil {
				t.Fatal(err)
			}

			body, err := json.Marshal(struct {
				Decision       dv.Decision                 `json:"decision"`
				Reason         dv.ReasonCode               `json:"reason_code"`
				Revision       uint64                      `json:"expected_revision"`
				ObservedFormat dv.DocumentFormat           `json:"document_format_observed"`
				ObservedExpiry string                      `json:"document_expires_at_observed"`
				Checklist      *dv.IdentityReviewChecklist `json:"identity_review_checklist"`
			}{
				Decision: dv.DecisionApprove, Revision: 7, ObservedFormat: dv.DocumentFormatUndetermined,
				ObservedExpiry: "2031-04-05",
				Checklist: &dv.IdentityReviewChecklist{
					NamesMatch: boolPointer(true), RUTMatches: boolPointer(true), BirthDateAndMajorityMatch: boolPointer(true),
					PortraitMatchesSelfie: boolPointer(true), EvidenceLegible: boolPointer(true), DocumentUnexpired: boolPointer(true),
					IssueDateAndFormatVerified: boolPointer(true),
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			request := reviewerReq(http.MethodPost, reviewerCasesRoute+"/"+reviewerID+"/review", string(body))
			response := httptest.NewRecorder()
			NewReviewerCasesHandler(reviewerAuth{}, access).ServeHTTP(response, request)

			if response.Code != http.StatusBadRequest || response.Body.String() != "invalid request\n" {
				t.Fatalf("status=%d body=%q, want invalid request", response.Code, response.Body.String())
			}
			if repo.transitionCalls != 1 || repo.transitionActor != reviewerActor || repo.reviewerLoadCalls != 1 || repo.loadActor != reviewerActor {
				t.Fatalf("transition calls/actor=%d/%q, loads/actor=%d/%q", repo.transitionCalls, repo.transitionActor, repo.reviewerLoadCalls, repo.loadActor)
			}
			if !reflect.DeepEqual(repo.snapshot, beforeSnapshot) || repo.audit != beforeAudit {
				t.Fatalf("approval with undetermined format mutated state: snapshot=%+v audit=%+v", repo.snapshot, repo.audit)
			}
		})
	}
}

func boolPointer(value bool) *bool { return &value }
