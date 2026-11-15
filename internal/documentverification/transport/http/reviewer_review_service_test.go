package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
)

func TestReviewerIdentityNeedsEvidenceHTTPUsesActualServiceAndReadsBack(t *testing.T) {
	for _, test := range []struct {
		name           string
		declaredFormat dv.DocumentFormat
		observedFormat dv.DocumentFormat
		formatVerified bool
	}{
		{name: "known format", declaredFormat: dv.DocumentFormatNew, observedFormat: dv.DocumentFormatNew, formatVerified: true},
		{name: "uncertain format requests evidence", declaredFormat: dv.DocumentFormatUnsure, observedFormat: dv.DocumentFormatUndetermined, formatVerified: false},
	} {
		t.Run(test.name, func(t *testing.T) {
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
					DocumentFormatDeclared:        test.declaredFormat,
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
			domain, err := dv.NewService(repo)
			if err != nil {
				t.Fatal(err)
			}
			domain.SetClock(func() time.Time { return time.Date(2026, 11, 11, 17, 0, 0, 0, time.UTC) })
			access, err := dv.NewReviewerAccessService(repo, domain, revokeLifecycleDocumentStore{})
			if err != nil {
				t.Fatal(err)
			}
			checklist := &dv.IdentityReviewChecklist{
				NamesMatch: boolPointer(false), RUTMatches: boolPointer(true), BirthDateAndMajorityMatch: boolPointer(true),
				PortraitMatchesSelfie: boolPointer(true), EvidenceLegible: boolPointer(true), DocumentUnexpired: boolPointer(true),
				IssueDateAndFormatVerified: boolPointer(test.formatVerified),
			}
			body, err := json.Marshal(struct {
				Decision       dv.Decision                 `json:"decision"`
				Reason         dv.ReasonCode               `json:"reason_code"`
				Revision       uint64                      `json:"expected_revision"`
				ObservedFormat dv.DocumentFormat           `json:"document_format_observed"`
				Checklist      *dv.IdentityReviewChecklist `json:"identity_review_checklist"`
			}{
				Decision: dv.DecisionNeedsEvidence, Reason: dv.ReasonEvidenceIncomplete, Revision: 7,
				ObservedFormat: test.observedFormat, Checklist: checklist,
			})
			if err != nil {
				t.Fatal(err)
			}
			request := reviewerReq(http.MethodPost, reviewerCasesRoute+"/"+reviewerID+"/review", string(body))
			response := httptest.NewRecorder()
			mux := http.NewServeMux()
			RegisterReviewerCasesRoutes(mux, NewReviewerCasesHandler(reviewerAuth{}, access))
			mux.ServeHTTP(response, request)

			if response.Code != http.StatusOK || response.Header().Get("ETag") != `"8"` {
				t.Fatalf("status=%d ETag=%q body=%q", response.Code, response.Header().Get("ETag"), response.Body.String())
			}
			var result reviewerCaseDTO
			if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if result.ID != reviewerID || result.State != dv.CaseNeedsEvidence || result.Revision != 8 ||
				result.DocumentFormatDeclared != test.declaredFormat || result.DocumentFormatObserved != test.observedFormat || result.DocumentExpiresAtObserved != nil {
				t.Fatalf("review response=%+v", result)
			}
			if repo.snapshot.State != dv.CaseNeedsEvidence || repo.snapshot.Revision != 8 || repo.snapshot.ReviewerID != reviewerActor ||
				repo.snapshot.DocumentFormatDeclared != test.declaredFormat || repo.snapshot.DocumentFormatObserved != test.observedFormat || !repo.snapshot.DocumentExpiresAt.IsZero() {
				t.Fatalf("service transition=%+v", repo.snapshot)
			}
			if repo.audit.Action != "review_decision" || repo.audit.ResultCode != string(dv.CaseNeedsEvidence) ||
				repo.audit.ReasonCode != dv.ReasonEvidenceIncomplete || repo.audit.DocumentFormatObserved != test.observedFormat ||
				!repo.audit.DocumentExpiresAtObserved.IsZero() || repo.audit.IdentityReviewChecklist == nil ||
				!repo.audit.IdentityReviewChecklist.Complete() || repo.audit.IdentityReviewChecklist.AllPassed() {
				t.Fatalf("review audit=%+v", repo.audit)
			}
			if repo.transitionCalls != 1 || repo.transitionActor != reviewerActor || repo.reviewerLoadCalls != 2 || repo.loadActor != reviewerActor {
				t.Fatalf("transition calls/actor=%d/%q, load calls/actor=%d/%q", repo.transitionCalls, repo.transitionActor, repo.reviewerLoadCalls, repo.loadActor)
			}
		})
	}
}
