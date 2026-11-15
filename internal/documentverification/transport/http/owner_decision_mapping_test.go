package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
)

func TestOwnerDecisionMessageUsesOnlyApprovedIdentityMappings(t *testing.T) {
	const correctionFallback = "Se requieren antecedentes adicionales para continuar. Revisa la sección de documentos y completa lo que falta antes de enviar nuevamente."
	const needsEvidenceCopy = "Faltan antecedentes para continuar con la revisión. Revisa los documentos solicitados y completa lo que falta antes de enviar nuevamente."

	for _, tc := range []struct {
		name    string
		purpose documentverification.Purpose
		state   documentverification.CaseState
		reason  documentverification.ReasonCode
		want    string
	}{
		{"needs-evidence allowed code", documentverification.PurposeIdentity, documentverification.CaseNeedsEvidence, documentverification.ReasonEvidenceIncomplete, needsEvidenceCopy},
		{"needs-evidence internal code falls back", documentverification.PurposeIdentity, documentverification.CaseNeedsEvidence, documentverification.ReasonMalwareDetected, correctionFallback},
		{"needs-evidence unknown code falls back", documentverification.PurposeIdentity, documentverification.CaseNeedsEvidence, documentverification.ReasonCode("future_internal_reason_v2"), correctionFallback},
		{"needs-evidence absent code omits decision copy", documentverification.PurposeIdentity, documentverification.CaseNeedsEvidence, "", ""},
		{"rejected code is not exposed", documentverification.PurposeIdentity, documentverification.CaseRejected, documentverification.ReasonDocumentUnreadable, ""},
		{"revocation reason is not exposed", documentverification.PurposeIdentity, documentverification.CaseRevoked, documentverification.ReasonOwnerRequest, ""},
		{"space reason is not exposed", documentverification.PurposeSpace, documentverification.CaseNeedsEvidence, documentverification.ReasonEvidenceIncomplete, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			snapshot := documentverification.Snapshot{
				ID: ownerCasesID, SubjectID: casesActor, Purpose: tc.purpose,
				State: tc.state, Revision: 18, OwnerDecisionReason: tc.reason,
			}
			if tc.purpose == documentverification.PurposeSpace {
				snapshot.SpaceID = "88888888-8888-4888-8888-888888888888"
			}
			f := &casesServiceFake{loaded: snapshot}
			req := httptest.NewRequest(http.MethodGet, "/api/v1/document-verification/cases/"+ownerCasesID, nil)
			req.Header.Set("Authorization", "Bearer synthetic-session")
			w := httptest.NewRecorder()
			casesHandler(f).ServeHTTP(w, req)
			if w.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}
			var dto map[string]json.RawMessage
			if err := json.Unmarshal(w.Body.Bytes(), &dto); err != nil {
				t.Fatal(err)
			}
			got, present := dto["owner_decision_message"]
			if tc.want == "" {
				if present {
					t.Errorf("owner DTO unexpectedly included a decision message: %s", got)
				}
			} else if !present || string(got) != `"`+tc.want+`"` {
				t.Errorf("owner_decision_message=%s, present=%v, want %q", got, present, tc.want)
			}
			if strings.Contains(w.Body.String(), string(tc.reason)) && tc.reason != "" {
				t.Errorf("owner DTO leaked internal reason code %q: %s", tc.reason, w.Body.String())
			}
		})
	}
}
