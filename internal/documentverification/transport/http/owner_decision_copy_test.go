package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
)

func TestOwnerIdentityDecisionMessageIsOmittedWithoutDecisionEvent(t *testing.T) {
	for _, tc := range []struct {
		name, state, want string
	}{
		{"needs evidence without decision event", string(documentverification.CaseNeedsEvidence), ""},
		{"rejected", string(documentverification.CaseRejected), ""},
		{"revoked", string(documentverification.CaseRevoked), ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			snapshot := documentverification.Snapshot{
				ID: ownerCasesID, SubjectID: casesActor, Purpose: documentverification.PurposeIdentity,
				State: documentverification.CaseState(tc.state), Revision: 18,
				ReviewerID: "reviewer-private-synthetic", IdentityDeclarationRevisionID: "declaration-private-synthetic",
			}
			for _, request := range []struct {
				name, path string
			}{
				{"detail", "/api/v1/document-verification/cases/" + ownerCasesID},
				{"list", "/api/v1/document-verification/cases"},
			} {
				t.Run(request.name, func(t *testing.T) {
					f := &casesServiceFake{loaded: snapshot, listed: []documentverification.Snapshot{snapshot}}
					req := httptest.NewRequest(http.MethodGet, request.path, nil)
					req.Header.Set("Authorization", "Bearer synthetic-session")
					w := httptest.NewRecorder()
					casesHandler(f).ServeHTTP(w, req)
					if w.Code != http.StatusOK {
						t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
					}
					var dto map[string]json.RawMessage
					if request.name == "list" {
						var rows []map[string]json.RawMessage
						if err := json.Unmarshal(w.Body.Bytes(), &rows); err != nil || len(rows) != 1 {
							t.Fatalf("decode owner list: rows=%d err=%v body=%s", len(rows), err, w.Body.String())
						}
						dto = rows[0]
					} else if err := json.Unmarshal(w.Body.Bytes(), &dto); err != nil {
						t.Fatal(err)
					}
					got, present := dto["owner_decision_message"]
					if tc.want == "" {
						if present {
							t.Errorf("owner_decision_message unexpectedly present: %s", got)
						}
					} else if !present || string(got) != `"`+tc.want+`"` {
						t.Errorf("owner_decision_message=%s, present=%v, want %q", got, present, tc.want)
					}
					body := w.Body.String()
					for _, privateValue := range []string{"reviewer-private-synthetic", "declaration-private-synthetic", "documento_ilegible", "documento_invalido", "documento_vencido", "antecedentes_incompletos", "malware_detectado"} {
						if strings.Contains(body, privateValue) {
							t.Errorf("owner response leaked private/internal value %q: %s", privateValue, body)
						}
					}
				})
			}
		})
	}
}
