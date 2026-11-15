package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
)

const reviewerID = "99999999-9999-4999-8999-999999999999"
const reviewerActor = "66666666-6666-4666-8666-666666666666"
const reviewerFile = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"

type reviewerFake struct {
	calls       []string
	actor       string
	x           dv.Snapshot
	d           dv.DownloadedDocument
	err         error
	reviewArgs  []any
	revokeActor string
	revokeID    string
	declaration map[string]string
}

type reviewerLoadSequenceFake struct {
	*reviewerFake
	loads []dv.Snapshot
}

type reviewerIdentityConflictFake struct {
	*reviewerFake
}

func (f *reviewerIdentityConflictFake) ReviewIdentity(_ context.Context, actor, _ string, decision dv.Decision, reason dv.ReasonCode, format dv.DocumentFormat, expiry time.Time, checklist *dv.IdentityReviewChecklist, until time.Time, revision uint64) error {
	f.calls = append(f.calls, "review_identity")
	f.actor = actor
	f.reviewArgs = []any{decision, reason, format, expiry, checklist, until, revision}
	return dv.ErrConflict
}

func (f *reviewerLoadSequenceFake) Load(_ context.Context, actor, _ string) (dv.Snapshot, error) {
	f.calls = append(f.calls, "load")
	f.actor = actor
	if len(f.loads) == 0 {
		return dv.Snapshot{}, dv.ErrNotFound
	}
	result := f.loads[0]
	f.loads = f.loads[1:]
	return result, f.err
}

func (f *reviewerFake) List(_ context.Context, a string) ([]dv.Snapshot, error) {
	f.calls = append(f.calls, "list")
	f.actor = a
	return []dv.Snapshot{f.x}, f.err
}
func (f *reviewerFake) Load(_ context.Context, a, id string) (dv.Snapshot, error) {
	f.calls = append(f.calls, "load")
	f.actor = a
	return f.x, f.err
}
func (f *reviewerFake) LoadDetail(_ context.Context, a, id string) (dv.ReviewerCaseDetail, error) {
	f.calls = append(f.calls, "load_detail")
	f.actor = a
	detail := dv.ReviewerCaseDetail{Snapshot: f.x}
	if len(f.declaration) > 0 {
		detail.IdentityDeclaration = &dv.ReviewerIdentityDeclaration{
			FirstName: f.declaration["first_name"], LastName: f.declaration["last_name"],
			RUT: f.declaration["rut"], DateOfBirth: f.declaration["date_of_birth"],
		}
	}
	return detail, f.err
}
func (f *reviewerFake) Review(_ context.Context, a, id string, c dv.Capability, d dv.Decision, r dv.ReasonCode, u time.Time, v uint64) error {
	f.calls = append(f.calls, "review")
	f.actor = a
	f.reviewArgs = []any{c, d, r, u, v}
	return f.err
}
func (f *reviewerFake) ReviewIdentity(_ context.Context, a, id string, d dv.Decision, r dv.ReasonCode, format dv.DocumentFormat, observedExpiry time.Time, checklist *dv.IdentityReviewChecklist, until time.Time, rev uint64) error {
	f.calls = append(f.calls, "review_identity")
	f.actor = a
	f.reviewArgs = []any{d, r, format, observedExpiry, checklist, until, rev}
	return f.err
}
func (f *reviewerFake) Revoke(_ context.Context, a, id string, p dv.Purpose, r dv.ReasonCode, v uint64) error {
	f.calls = append(f.calls, "revoke")
	f.actor = a
	f.revokeActor = a
	f.revokeID = id
	f.reviewArgs = []any{p, r, v}
	return f.err
}
func (f *reviewerFake) Download(_ context.Context, a, c, id string) (dv.DownloadedDocument, error) {
	f.calls = append(f.calls, "download")
	f.actor = a
	return f.d, f.err
}

type reviewerAuth struct{ err error }

func (a reviewerAuth) Authorize(context.Context, identity.Secret, identity.Role, identity.ActivityKind) (identity.Principal, error) {
	return identity.Principal{AccountID: reviewerActor}, a.err
}
func reviewerHandler(f *reviewerFake, authErr error) http.Handler {
	return NewReviewerCasesHandler(reviewerAuth{authErr}, f)
}
func reviewerReq(method, path, body string) *http.Request {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer session")
	return r
}
func TestReviewerCasesFiveEndpoints(t *testing.T) {
	x := dv.Snapshot{ID: reviewerID, SubjectID: "88888888-8888-4888-8888-888888888888", Purpose: dv.PurposeIdentity, State: dv.CaseInReview, Revision: 7, Files: []dv.File{{ID: reviewerFile, State: dv.FileClean, Size: 4, MIME: "application/pdf"}}}
	tests := []struct {
		name, method, path, body, call string
		status                         int
	}{{"list", "GET", reviewerCasesRoute, "", "list", 200}, {"case", "GET", reviewerCasesRoute + "/" + reviewerID, "", "load_detail", 200}, {"pdf", "GET", reviewerCasesRoute + "/" + reviewerID + "/files/" + reviewerFile + "/content", "", "download", 200}, {"review", "POST", reviewerCasesRoute + "/" + reviewerID + "/review", `{"capability":"identity","decision":"approve","reason_code":"verified","expected_revision":7}`, "review", 200}, {"revoke", "POST", reviewerCasesRoute + "/" + reviewerID + "/revoke", `{"purpose":"identity","reason_code":"solicitud_titular","expected_revision":7}`, "revoke", 200}}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := &reviewerFake{x: x, d: dv.DownloadedDocument{MIME: "application/pdf", Content: []byte("%PDF")}}
			w := httptest.NewRecorder()
			reviewerHandler(f, nil).ServeHTTP(w, reviewerReq(tc.method, tc.path, tc.body))
			if w.Code != tc.status {
				t.Fatalf("status=%d calls=%v body=%s", w.Code, f.calls, w.Body.String())
			}
			if f.actor != reviewerActor || len(f.calls) == 0 || (tc.call == "review" && (len(f.calls) != 3 || f.calls[0] != "load" || f.calls[1] != "review" || f.calls[2] != "load")) || (tc.call != "review" && f.calls[0] != tc.call) {
				t.Fatalf("actor=%q calls=%v", f.actor, f.calls)
			}
			if tc.name == "pdf" && (w.Header().Get("Content-Type") != "application/pdf" || w.Body.String() != "%PDF" || w.Header().Get("Cache-Control") != "private, no-store" || w.Header().Get("X-Content-Type-Options") != "nosniff") {
				t.Fatalf("download headers=%v body=%q", w.Header(), w.Body.String())
			}
			if tc.name == "review" || tc.name == "revoke" {
				wantCalls := []string{"revoke", "load"}
				if tc.name == "review" {
					wantCalls = []string{"load", "review", "load"}
				}
				if len(f.calls) != len(wantCalls) || w.Header().Get("ETag") != `"7"` {
					t.Fatalf("post then metadata load: calls=%v etag=%q", f.calls, w.Header().Get("ETag"))
				}
				for i := range wantCalls {
					if f.calls[i] != wantCalls[i] {
						t.Fatalf("calls=%v, want %v", f.calls, wantCalls)
					}
				}
			}
		})
	}
}

func TestReviewerRevokeUsesAuthenticatedPrincipalAndReadsBackRevokedCase(t *testing.T) {
	const subjectID = "88888888-8888-4888-8888-888888888888"
	f := &reviewerFake{x: dv.Snapshot{
		ID: reviewerID, SubjectID: subjectID, Purpose: dv.PurposeIdentity,
		State: dv.CaseRevoked, Revision: 8,
	}}
	w := httptest.NewRecorder()
	body := `{"purpose":"identity","reason_code":"solicitud_titular","expected_revision":7}`
	reviewerHandler(f, nil).ServeHTTP(w, reviewerReq("POST", reviewerCasesRoute+"/"+reviewerID+"/revoke", body))

	if w.Code != http.StatusOK || f.revokeActor != reviewerActor || f.revokeID != reviewerID {
		t.Fatalf("status=%d revoke actor=%q id=%q calls=%v body=%s", w.Code, f.revokeActor, f.revokeID, f.calls, w.Body.String())
	}
	if len(f.reviewArgs) != 3 || f.reviewArgs[0] != dv.PurposeIdentity || f.reviewArgs[1] != dv.ReasonOwnerRequest || f.reviewArgs[2] != uint64(7) {
		t.Fatalf("revoke arguments=%v, want identity/solicitud_titular/revision 7", f.reviewArgs)
	}
	if len(f.calls) != 2 || f.calls[0] != "revoke" || f.calls[1] != "load" || w.Header().Get("ETag") != `"8"` {
		t.Fatalf("revocation readback calls=%v etag=%q", f.calls, w.Header().Get("ETag"))
	}
	var response struct {
		State    dv.CaseState `json:"state"`
		Revision uint64       `json:"revision"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.State != dv.CaseRevoked || response.Revision != 8 {
		t.Fatalf("revocation readback state/revision=%q/%d", response.State, response.Revision)
	}
}

func TestReviewerRevokeDenialDoesNotReadBackCase(t *testing.T) {
	f := &reviewerFake{err: dv.ErrCapability}
	w := httptest.NewRecorder()
	body := `{"purpose":"identity","reason_code":"solicitud_titular","expected_revision":7}`
	reviewerHandler(f, nil).ServeHTTP(w, reviewerReq("POST", reviewerCasesRoute+"/"+reviewerID+"/revoke", body))

	if w.Code != http.StatusNotFound || len(f.calls) != 1 || f.calls[0] != "revoke" || strings.Contains(w.Body.String(), dv.ErrCapability.Error()) {
		t.Fatalf("denied revocation status=%d calls=%v body=%q", w.Code, f.calls, w.Body.String())
	}
}

func TestTypedIdentityReviewUsesObservedValuesAndReviewerDTOOnly(t *testing.T) {
	const expiryDate = "2031-04-05"
	expiry, err := observedExpiryBoundary(expiryDate)
	if err != nil {
		t.Fatal(err)
	}
	x := dv.Snapshot{ID: reviewerID, SubjectID: "88888888-8888-4888-8888-888888888888", Purpose: dv.PurposeIdentity, State: dv.CaseInReview, Revision: 7, DocumentFormatDeclared: dv.DocumentFormatNew, DocumentFormatObserved: dv.DocumentFormatLegacy, DocumentExpiresAt: expiry}
	f := &reviewerFake{x: x}
	body := `{"decision":"approve","expected_revision":7,"document_format_observed":"legacy","document_expires_at_observed":"2031-04-05","identity_review_checklist":{"names_match":true,"rut_matches":true,"birth_date_and_majority_match":true,"portrait_matches_selfie":true,"evidence_legible":true,"document_unexpired":true,"issue_date_and_format_verified":true}}`
	w := httptest.NewRecorder()
	reviewerHandler(f, nil).ServeHTTP(w, reviewerReq("POST", reviewerCasesRoute+"/"+reviewerID+"/review", body))
	if w.Code != 200 || len(f.calls) != 3 || f.calls[1] != "review_identity" {
		t.Fatalf("status=%d calls=%v body=%s", w.Code, f.calls, w.Body.String())
	}
	var dto map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &dto); err != nil {
		t.Fatal(err)
	}
	if dto["document_format_observed"] != string(dv.DocumentFormatLegacy) || dto["document_expires_at_observed"] != expiryDate || dto["document_expires_at"] != nil {
		t.Fatalf("reviewer DTO=%v", dto)
	}
}

func TestTypedIdentityReviewUsesSantiagoCivilExpiryDate(t *testing.T) {
	location, err := time.LoadLocation("America/Santiago")
	if err != nil {
		t.Fatal(err)
	}
	const expiryDate = "2031-04-05"
	wantBoundary := time.Date(2031, 4, 5, 0, 0, 0, 0, location).AddDate(0, 0, 1).UTC()
	f := &reviewerFake{x: dv.Snapshot{
		ID: reviewerID, SubjectID: "88888888-8888-4888-8888-888888888888",
		Purpose: dv.PurposeIdentity, State: dv.CaseInReview, Revision: 7,
		DocumentFormatDeclared: dv.DocumentFormatNew, DocumentExpiresAt: wantBoundary,
	}}
	body := `{"decision":"approve","expected_revision":7,"document_format_observed":"new","document_expires_at_observed":"2031-04-05","identity_review_checklist":{"names_match":true,"rut_matches":true,"birth_date_and_majority_match":true,"portrait_matches_selfie":true,"evidence_legible":true,"document_unexpired":true,"issue_date_and_format_verified":true}}`
	w := httptest.NewRecorder()
	reviewerHandler(f, nil).ServeHTTP(w, reviewerReq("POST", reviewerCasesRoute+"/"+reviewerID+"/review", body))
	if w.Code != http.StatusOK || len(f.calls) != 3 || f.calls[1] != "review_identity" {
		t.Fatalf("status=%d calls=%v body=%s", w.Code, f.calls, w.Body.String())
	}
	if len(f.reviewArgs) < 4 {
		t.Fatalf("review arguments=%v", f.reviewArgs)
	}
	gotBoundary, ok := f.reviewArgs[3].(time.Time)
	if !ok || !gotBoundary.Equal(wantBoundary) {
		t.Fatalf("observed expiry boundary=%v, want %v", f.reviewArgs[3], wantBoundary)
	}
	var dto map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &dto); err != nil {
		t.Fatal(err)
	}
	if dto["document_expires_at_observed"] != expiryDate {
		t.Fatalf("reviewer DTO expiry=%v, want civil date %q", dto["document_expires_at_observed"], expiryDate)
	}
}

func TestReviewerIdentityReviewConflictReturnsNeutral409WithoutReadback(t *testing.T) {
	base := &reviewerFake{x: dv.Snapshot{
		ID: reviewerID, SubjectID: "88888888-8888-4888-8888-888888888888",
		Purpose: dv.PurposeIdentity, State: dv.CaseInReview, Revision: 7,
		DocumentFormatDeclared: dv.DocumentFormatNew,
	}}
	service := &reviewerIdentityConflictFake{reviewerFake: base}
	body := `{"decision":"approve","expected_revision":7,"document_format_observed":"new","document_expires_at_observed":"2031-04-05","identity_review_checklist":{"names_match":true,"rut_matches":true,"birth_date_and_majority_match":true,"portrait_matches_selfie":true,"evidence_legible":true,"document_unexpired":true,"issue_date_and_format_verified":true}}`
	w := httptest.NewRecorder()
	NewReviewerCasesHandler(reviewerAuth{}, service).ServeHTTP(w, reviewerReq("POST", reviewerCasesRoute+"/"+reviewerID+"/review", body))

	if w.Code != http.StatusConflict || w.Body.String() != "case revision conflict\n" {
		t.Fatalf("status=%d body=%q, want neutral 409 revision conflict", w.Code, w.Body.String())
	}
	if w.Header().Get("ETag") != "" || w.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("conflict response headers=%v", w.Header())
	}
	if len(base.calls) != 2 || base.calls[0] != "load" || base.calls[1] != "review_identity" || base.actor != reviewerActor {
		t.Fatalf("conflict calls=%v actor=%q, want preflight then one review and no readback", base.calls, base.actor)
	}
}

func TestReviewerDecisionReadbackRejectsMismatchedCaseOrPurpose(t *testing.T) {
	const otherCaseID = "11111111-1111-4111-8111-111111111111"
	initial := dv.Snapshot{
		ID: reviewerID, SubjectID: "88888888-8888-4888-8888-888888888888",
		Purpose: dv.PurposeIdentity, State: dv.CaseInReview, Revision: 7,
		DocumentFormatDeclared: dv.DocumentFormatNew,
	}
	tests := []struct {
		name     string
		readback dv.Snapshot
	}{
		{
			name: "case id mismatch",
			readback: dv.Snapshot{
				ID: otherCaseID, SubjectID: initial.SubjectID,
				Purpose: dv.PurposeIdentity, State: dv.CaseApproved, Revision: 8,
			},
		},
		{
			name: "unsupported purpose",
			readback: dv.Snapshot{
				ID: reviewerID, SubjectID: initial.SubjectID,
				Purpose: dv.Purpose("unsupported"), State: dv.CaseApproved, Revision: 8,
			},
		},
		{
			name: "different allowed purpose",
			readback: dv.Snapshot{
				ID: reviewerID, SubjectID: initial.SubjectID,
				Purpose: dv.PurposeSpace, State: dv.CaseApproved, Revision: 8,
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			base := &reviewerFake{}
			service := &reviewerLoadSequenceFake{
				reviewerFake: base,
				loads:        []dv.Snapshot{initial, tc.readback},
			}
			body := `{"decision":"approve","expected_revision":7,"document_format_observed":"new","document_expires_at_observed":"2031-04-05","identity_review_checklist":{"names_match":true,"rut_matches":true,"birth_date_and_majority_match":true,"portrait_matches_selfie":true,"evidence_legible":true,"document_unexpired":true,"issue_date_and_format_verified":true}}`
			w := httptest.NewRecorder()
			NewReviewerCasesHandler(reviewerAuth{}, service).ServeHTTP(w, reviewerReq("POST", reviewerCasesRoute+"/"+reviewerID+"/review", body))

			if w.Code != http.StatusNotFound || w.Header().Get("ETag") != "" {
				t.Fatalf("invalid readback status=%d etag=%q body=%s", w.Code, w.Header().Get("ETag"), w.Body.String())
			}
			if len(base.calls) != 3 || base.calls[0] != "load" || base.calls[1] != "review_identity" || base.calls[2] != "load" || base.actor != reviewerActor {
				t.Fatalf("decision/readback calls=%v actor=%q", base.calls, base.actor)
			}
			if strings.Contains(w.Body.String(), otherCaseID) {
				t.Fatalf("readback disclosed a different case ID: %s", w.Body.String())
			}
		})
	}
}

func TestReviewerDecisionRejectsMismatchedCaseOrUnsupportedPurposeBeforeWrite(t *testing.T) {
	const otherCaseID = "11111111-1111-4111-8111-111111111111"
	initial := dv.Snapshot{
		ID: reviewerID, SubjectID: "88888888-8888-4888-8888-888888888888",
		Purpose: dv.PurposeIdentity, State: dv.CaseInReview, Revision: 7,
		DocumentFormatDeclared: dv.DocumentFormatNew,
	}
	tests := []struct {
		name   string
		loaded dv.Snapshot
		body   string
	}{
		{
			name: "case id mismatch",
			loaded: dv.Snapshot{
				ID: otherCaseID, SubjectID: initial.SubjectID,
				Purpose: dv.PurposeIdentity, State: dv.CaseInReview, Revision: 7,
				DocumentFormatDeclared: dv.DocumentFormatNew,
			},
			body: `{"decision":"approve","expected_revision":7,"document_format_observed":"new","document_expires_at_observed":"2031-04-05","identity_review_checklist":{"names_match":true,"rut_matches":true,"birth_date_and_majority_match":true,"portrait_matches_selfie":true,"evidence_legible":true,"document_unexpired":true,"issue_date_and_format_verified":true}}`,
		},
		{
			name: "unsupported purpose",
			loaded: dv.Snapshot{
				ID: reviewerID, SubjectID: initial.SubjectID,
				Purpose: dv.Purpose("unsupported"), State: dv.CaseInReview, Revision: 7,
			},
			body: `{"capability":"identity","decision":"approve","reason_code":"verified","expected_revision":7}`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			base := &reviewerFake{}
			service := &reviewerLoadSequenceFake{reviewerFake: base, loads: []dv.Snapshot{tc.loaded}}
			w := httptest.NewRecorder()
			NewReviewerCasesHandler(reviewerAuth{}, service).ServeHTTP(w, reviewerReq("POST", reviewerCasesRoute+"/"+reviewerID+"/review", tc.body))

			if w.Code != http.StatusNotFound || w.Header().Get("ETag") != "" {
				t.Fatalf("invalid preflight status=%d etag=%q body=%s", w.Code, w.Header().Get("ETag"), w.Body.String())
			}
			if len(base.calls) != 1 || base.calls[0] != "load" {
				t.Fatalf("invalid preflight reached mutation/readback: calls=%v", base.calls)
			}
		})
	}
}

func TestReviewerRevokeReadbackRejectsDifferentAllowedPurpose(t *testing.T) {
	base := &reviewerFake{}
	service := &reviewerLoadSequenceFake{
		reviewerFake: base,
		loads: []dv.Snapshot{{
			ID: reviewerID, SubjectID: "88888888-8888-4888-8888-888888888888",
			Purpose: dv.PurposeSpace, State: dv.CaseRevoked, Revision: 8,
		}},
	}
	w := httptest.NewRecorder()
	body := `{"purpose":"identity","reason_code":"solicitud_titular","expected_revision":7}`
	NewReviewerCasesHandler(reviewerAuth{}, service).ServeHTTP(w, reviewerReq("POST", reviewerCasesRoute+"/"+reviewerID+"/revoke", body))

	if w.Code != http.StatusNotFound || w.Header().Get("ETag") != "" {
		t.Fatalf("purpose-mismatched revoke readback status=%d etag=%q body=%s", w.Code, w.Header().Get("ETag"), w.Body.String())
	}
	if len(base.calls) != 2 || base.calls[0] != "revoke" || base.calls[1] != "load" {
		t.Fatalf("revoke readback calls=%v", base.calls)
	}
}

func TestObservedExpiryBoundaryUsesFirstValidLocalTimeAfterDSTGap(t *testing.T) {
	const civilDate = "2026-09-05"
	got, err := observedExpiryBoundary(civilDate)
	if err != nil {
		t.Fatal(err)
	}
	location, err := time.LoadLocation(observedExpiryLocation)
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 9, 6, 1, 0, 0, 0, location).UTC()
	if !got.Equal(want) {
		t.Fatalf("expiry boundary=%s local=%s, want first valid instant %s", got, got.In(location), want.In(location))
	}
	if observed := observedExpiryDate(got); observed != civilDate {
		t.Fatalf("DST boundary round-trip date=%q, want %q", observed, civilDate)
	}
}

func TestReviewerRejectsTimestampForCivilObservedExpiry(t *testing.T) {
	f := &reviewerFake{x: dv.Snapshot{
		ID: reviewerID, SubjectID: "88888888-8888-4888-8888-888888888888",
		Purpose: dv.PurposeIdentity, State: dv.CaseInReview, Revision: 7,
		DocumentFormatDeclared: dv.DocumentFormatNew,
	}}
	body := `{"decision":"approve","expected_revision":7,"document_format_observed":"new","document_expires_at_observed":"2031-04-05T23:59:59.999Z","identity_review_checklist":{"names_match":true,"rut_matches":true,"birth_date_and_majority_match":true,"portrait_matches_selfie":true,"evidence_legible":true,"document_unexpired":true,"issue_date_and_format_verified":true}}`
	w := httptest.NewRecorder()
	reviewerHandler(f, nil).ServeHTTP(w, reviewerReq("POST", reviewerCasesRoute+"/"+reviewerID+"/review", body))
	if w.Code != http.StatusBadRequest || len(f.calls) != 1 || f.calls[0] != "load" {
		t.Fatalf("timestamp expiry was not rejected before decision: status=%d calls=%v body=%s", w.Code, f.calls, w.Body.String())
	}
}

func TestObservedExpiryBoundaryRejectsMalformedCivilDates(t *testing.T) {
	for _, value := range []string{"2031-02-29", "2031-4-05", "2031-04-05T23:59:59.999Z"} {
		t.Run(value, func(t *testing.T) {
			if _, err := observedExpiryBoundary(value); err != dv.ErrInvalid {
				t.Fatalf("observedExpiryBoundary(%q) error=%v, want ErrInvalid", value, err)
			}
		})
	}
}

func TestTypedIdentityReviewRequiresObservedFormat(t *testing.T) {
	f := &reviewerFake{x: dv.Snapshot{ID: reviewerID, Purpose: dv.PurposeIdentity, DocumentFormatDeclared: dv.DocumentFormatUnsure}}
	w := httptest.NewRecorder()
	reviewerHandler(f, nil).ServeHTTP(w, reviewerReq("POST", reviewerCasesRoute+"/"+reviewerID+"/review", `{"decision":"reject","reason_code":"documento_invalido","expected_revision":7}`))
	if w.Code != 400 || len(f.calls) != 1 || f.calls[0] != "load" {
		t.Fatalf("status=%d calls=%v body=%s", w.Code, f.calls, w.Body.String())
	}
}

func TestIdentityApprovalRequiresReviewerChecklist(t *testing.T) {
	f := &reviewerFake{x: dv.Snapshot{
		ID: reviewerID, SubjectID: "88888888-8888-4888-8888-888888888888",
		Purpose: dv.PurposeIdentity, State: dv.CaseInReview, Revision: 7,
		DocumentFormatDeclared: dv.DocumentFormatNew,
	}}
	body := `{"capability":"review_identity","decision":"approve","expected_revision":7,"document_format_observed":"new","document_expires_at_observed":"2031-04-05"}`
	w := httptest.NewRecorder()
	reviewerHandler(f, nil).ServeHTTP(w, reviewerReq("POST", reviewerCasesRoute+"/"+reviewerID+"/review", body))
	if w.Code != http.StatusBadRequest || len(f.calls) != 1 || f.calls[0] != "load" {
		t.Fatalf("incomplete review reached decision: status=%d calls=%v body=%s", w.Code, f.calls, w.Body.String())
	}
}

func TestSpaceReviewRejectsIdentityChecklistBeforeMutation(t *testing.T) {
	f := &reviewerFake{x: dv.Snapshot{
		ID: reviewerID, SubjectID: "88888888-8888-4888-8888-888888888888",
		Purpose: dv.PurposeSpace, State: dv.CaseInReview, Revision: 7,
	}}
	body := `{"capability":"review_space","decision":"approve","reason_code":"verified","expected_revision":7,"document_format_observed":"new","document_expires_at_observed":"2031-04-05","identity_review_checklist":{"names_match":true,"rut_matches":true,"birth_date_and_majority_match":true,"portrait_matches_selfie":true,"evidence_legible":true,"document_unexpired":true,"issue_date_and_format_verified":true}}`
	w := httptest.NewRecorder()
	reviewerHandler(f, nil).ServeHTTP(w, reviewerReq("POST", reviewerCasesRoute+"/"+reviewerID+"/review", body))

	if w.Code != http.StatusBadRequest || w.Body.String() != "invalid request\n" || w.Header().Get("ETag") != "" {
		t.Fatalf("space review with identity checklist status=%d etag=%q body=%q", w.Code, w.Header().Get("ETag"), w.Body.String())
	}
	if w.Header().Get("Cache-Control") != "private, no-store" || f.actor != reviewerActor || len(f.calls) != 1 || f.calls[0] != "load" || len(f.reviewArgs) != 0 {
		t.Fatalf("identity-only payload reached mutation/readback: headers=%v actor=%q calls=%v args=%v", w.Header(), f.actor, f.calls, f.reviewArgs)
	}
}

func TestAssignedIdentityDetailIncludesOnlyAllowlistedDeclaration(t *testing.T) {
	f := &reviewerFake{
		x:           dv.Snapshot{ID: reviewerID, SubjectID: "88888888-8888-4888-8888-888888888888", Purpose: dv.PurposeIdentity, State: dv.CaseInReview, Revision: 7, IdentityDeclarationRevisionID: "77777777-7777-4777-8777-777777777777", DocumentFormatDeclared: dv.DocumentFormatNew},
		declaration: map[string]string{"first_name": "Synthetic", "last_name": "Reviewer Fixture", "rut": "11.222.333-9", "date_of_birth": "1990-02-03"},
	}
	w := httptest.NewRecorder()
	reviewerHandler(f, nil).ServeHTTP(w, reviewerReq("GET", reviewerCasesRoute+"/"+reviewerID, ""))
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if f.actor != reviewerActor || len(f.calls) != 1 || f.calls[0] != "load_detail" {
		t.Fatalf("principal/detail calls actor=%q calls=%v", f.actor, f.calls)
	}
	var dto map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &dto); err != nil {
		t.Fatal(err)
	}
	declaration, ok := dto["identity_declaration"].(map[string]any)
	if !ok || declaration["first_name"] != "Synthetic" || declaration["last_name"] != "Reviewer Fixture" || declaration["rut"] != "11.222.333-9" || declaration["date_of_birth"] != "1990-02-03" || len(declaration) != 4 {
		t.Fatalf("assigned detail declaration=%v, want the four allowlisted synthetic fields", dto["identity_declaration"])
	}
}

func TestAssignedIdentityDetailPreservesDeclaredFormatAndFileDTOValues(t *testing.T) {
	const observedDate = "2031-04-05"
	expiry, err := observedExpiryBoundary(observedDate)
	if err != nil {
		t.Fatal(err)
	}
	const content = "synthetic identity front image"
	f := &reviewerFake{
		x: dv.Snapshot{
			ID: reviewerID, SubjectID: "88888888-8888-4888-8888-888888888888",
			Purpose: dv.PurposeIdentity, State: dv.CaseInReview, Revision: 7,
			IdentityDeclarationRevisionID: "77777777-7777-4777-8777-777777777777",
			DocumentFormatDeclared:        dv.DocumentFormatNew,
			DocumentFormatObserved:        dv.DocumentFormatLegacy,
			DocumentExpiresAt:             expiry,
			Files: []dv.File{{
				ID: reviewerFile, EvidenceRole: dv.EvidenceRoleIdentityFront,
				State: dv.FileClean, Size: int64(len(content)), MIME: "image/jpeg", Pages: 1,
				RetentionAt: time.Date(2031, 10, 1, 0, 0, 0, 0, time.UTC),
			}},
		},
		declaration: map[string]string{
			"first_name": "Synthetic", "last_name": "Reviewer Fixture",
			"rut": "11.222.333-9", "date_of_birth": "1990-02-03",
		},
	}
	w := httptest.NewRecorder()
	reviewerHandler(f, nil).ServeHTTP(w, reviewerReq("GET", reviewerCasesRoute+"/"+reviewerID, ""))
	if w.Code != http.StatusOK || w.Header().Get("ETag") != `"7"` || w.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("status=%d etag=%q cache=%q body=%s", w.Code, w.Header().Get("ETag"), w.Header().Get("Cache-Control"), w.Body.String())
	}
	if f.actor != reviewerActor || len(f.calls) != 1 || f.calls[0] != "load_detail" {
		t.Fatalf("principal/detail calls actor=%q calls=%v", f.actor, f.calls)
	}
	var dto map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &dto); err != nil {
		t.Fatal(err)
	}
	wantCase := map[string]any{
		"id": reviewerID, "purpose": string(dv.PurposeIdentity),
		"state": string(dv.CaseInReview), "revision": float64(7),
		"document_format_declared":     string(dv.DocumentFormatNew),
		"document_format_observed":     string(dv.DocumentFormatLegacy),
		"document_expires_at_observed": observedDate,
	}
	if len(dto) != len(wantCase)+2 {
		t.Fatalf("reviewer detail keys=%v, want only allowlisted case, files, and declaration fields", dto)
	}
	for key, want := range wantCase {
		if got := dto[key]; got != want {
			t.Errorf("reviewer detail %s=%v, want %v", key, got, want)
		}
	}
	if _, ok := dto["document_expires_at"]; ok {
		t.Errorf("identity reviewer DTO exposed timestamp field document_expires_at: %v", dto["document_expires_at"])
	}
	files, ok := dto["files"].([]any)
	if !ok || len(files) != 1 {
		t.Fatalf("reviewer files=%v, want one metadata item", dto["files"])
	}
	file, ok := files[0].(map[string]any)
	if !ok {
		t.Fatalf("reviewer file DTO=%T, want object", files[0])
	}
	wantFile := map[string]any{
		"id": reviewerFile, "evidence_role": string(dv.EvidenceRoleIdentityFront),
		"state": string(dv.FileClean), "size": float64(len(content)),
		"mime": "image/jpeg", "pages": float64(1),
	}
	if len(file) != len(wantFile) {
		t.Fatalf("reviewer file keys=%v, want only allowlisted metadata", file)
	}
	for key, want := range wantFile {
		if got := file[key]; got != want {
			t.Errorf("reviewer file %s=%v, want %v", key, got, want)
		}
	}
	declaration, ok := dto["identity_declaration"].(map[string]any)
	if !ok || len(declaration) != 4 || declaration["first_name"] != "Synthetic" || declaration["last_name"] != "Reviewer Fixture" || declaration["rut"] != "11.222.333-9" || declaration["date_of_birth"] != "1990-02-03" {
		t.Fatalf("reviewer declaration=%v, want the four allowlisted synthetic fields", dto["identity_declaration"])
	}
}

func TestReviewerDetailRejectsMismatchedCaseFromService(t *testing.T) {
	const returnedCaseID = "11111111-1111-4111-8111-111111111111"
	f := &reviewerFake{
		x: dv.Snapshot{
			ID: returnedCaseID, SubjectID: "88888888-8888-4888-8888-888888888888",
			Purpose: dv.PurposeIdentity, State: dv.CaseInReview, Revision: 7,
		},
		declaration: map[string]string{
			"first_name": "Synthetic Private", "last_name": "Wrong Case",
			"rut": "11.222.333-9", "date_of_birth": "1990-02-03",
		},
	}
	w := httptest.NewRecorder()
	reviewerHandler(f, nil).ServeHTTP(w, reviewerReq("GET", reviewerCasesRoute+"/"+reviewerID, ""))

	if w.Code != http.StatusNotFound || len(f.calls) != 1 || f.calls[0] != "load_detail" {
		t.Fatalf("mismatched case status=%d calls=%v body=%s", w.Code, f.calls, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "Synthetic Private") || strings.Contains(w.Body.String(), "11.222.333-9") || w.Header().Get("ETag") != "" {
		t.Fatalf("mismatched case leaked detail, identity data, or ETag: headers=%v body=%s", w.Header(), w.Body.String())
	}
}

func TestReviewerDetailRejectsUnsupportedPurposeFromService(t *testing.T) {
	f := &reviewerFake{
		x: dv.Snapshot{
			ID: reviewerID, SubjectID: "88888888-8888-4888-8888-888888888888",
			Purpose: dv.Purpose("unsupported"), State: dv.CaseInReview, Revision: 7,
		},
		declaration: map[string]string{
			"first_name": "Synthetic Private", "last_name": "Unsupported Purpose",
			"rut": "11.222.333-9", "date_of_birth": "1990-02-03",
		},
	}
	w := httptest.NewRecorder()
	reviewerHandler(f, nil).ServeHTTP(w, reviewerReq("GET", reviewerCasesRoute+"/"+reviewerID, ""))

	if w.Code != http.StatusNotFound || len(f.calls) != 1 || f.calls[0] != "load_detail" {
		t.Fatalf("unsupported purpose status=%d calls=%v body=%s", w.Code, f.calls, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "Synthetic Private") || strings.Contains(w.Body.String(), "11.222.333-9") || w.Header().Get("ETag") != "" {
		t.Fatalf("unsupported purpose leaked detail, identity data, or ETag: headers=%v body=%s", w.Header(), w.Body.String())
	}
}

func TestReviewerDetailDoesNotExposeIdentityDeclarationForSpaceCase(t *testing.T) {
	f := &reviewerFake{
		x:           dv.Snapshot{ID: reviewerID, SubjectID: "88888888-8888-4888-8888-888888888888", Purpose: dv.PurposeSpace, State: dv.CaseInReview, Revision: 7},
		declaration: map[string]string{"first_name": "Synthetic Private", "last_name": "Must Stay Hidden", "rut": "11.222.333-9", "date_of_birth": "1990-02-03"},
	}
	w := httptest.NewRecorder()
	reviewerHandler(f, nil).ServeHTTP(w, reviewerReq("GET", reviewerCasesRoute+"/"+reviewerID, ""))
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var dto map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &dto); err != nil {
		t.Fatal(err)
	}
	wantKeys := []string{"id", "purpose", "state", "revision", "files"}
	if len(dto) != len(wantKeys) || dto["id"] != reviewerID || dto["purpose"] != string(dv.PurposeSpace) || dto["state"] != string(dv.CaseInReview) || dto["revision"] != float64(7) {
		t.Fatalf("space detail DTO=%v, want only the five public case fields", dto)
	}
	if _, ok := dto["files"].([]any); !ok {
		t.Fatalf("space detail files=%T, want array", dto["files"])
	}
	for _, key := range wantKeys {
		if _, ok := dto[key]; !ok {
			t.Errorf("space detail DTO missing allowed key %q", key)
		}
	}
	body := w.Body.String()
	for _, privateValue := range []string{"identity_declaration", "Synthetic Private", "Must Stay Hidden", "11.222.333-9", "1990-02-03"} {
		if strings.Contains(body, privateValue) {
			t.Errorf("space detail disclosed private value %q: body=%s", privateValue, body)
		}
	}
}

func TestReviewerListDoesNotExposeIdentityDeclaration(t *testing.T) {
	f := &reviewerFake{
		x:           dv.Snapshot{ID: reviewerID, SubjectID: "88888888-8888-4888-8888-888888888888", Purpose: dv.PurposeIdentity, State: dv.CaseInReview, Revision: 7},
		declaration: map[string]string{"first_name": "Synthetic", "last_name": "List Must Not Reveal", "rut": "11.222.333-9", "date_of_birth": "1990-02-03"},
	}
	w := httptest.NewRecorder()
	reviewerHandler(f, nil).ServeHTTP(w, reviewerReq("GET", reviewerCasesRoute, ""))
	if w.Code != http.StatusOK || strings.Contains(w.Body.String(), "identity_declaration") || strings.Contains(w.Body.String(), "List Must Not Reveal") {
		t.Fatalf("reviewer list disclosed declaration: status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestReviewerCasesRejectsOutOfScopeAndWithdrawn(t *testing.T) {
	for _, e := range []error{dv.ErrNotFound, dv.ErrCapability, dv.ErrTransition} {
		f := &reviewerFake{err: e}
		w := httptest.NewRecorder()
		reviewerHandler(f, nil).ServeHTTP(w, reviewerReq("GET", reviewerCasesRoute+"/"+reviewerID, ""))
		if w.Code != 404 || strings.Contains(w.Body.String(), e.Error()) {
			t.Fatalf("err %v: status=%d body=%q", e, w.Code, w.Body.String())
		}
	}
}
func TestReviewerCasesStrictRoutingJSONAndAuth(t *testing.T) {
	tests := []struct {
		name, method, path, body string
		authErr                  error
		want                     int
	}{{"invalid UUID", "GET", reviewerCasesRoute + "/bad", "", nil, 404}, {"extra path", "GET", reviewerCasesRoute + "/" + reviewerID + "/review/x", "", nil, 404}, {"query", "GET", reviewerCasesRoute + "?x=1", "", nil, 404}, {"wrong verb", "DELETE", reviewerCasesRoute + "/" + reviewerID, "", nil, 405}, {"unknown JSON", "POST", reviewerCasesRoute + "/" + reviewerID + "/review", `{"caller":"` + reviewerActor + `"}`, nil, 400}, {"trailing JSON", "POST", reviewerCasesRoute + "/" + reviewerID + "/review", `{} {}`, nil, 400}, {"missing auth", "GET", reviewerCasesRoute, "", nil, 401}, {"auth denied", "GET", reviewerCasesRoute, "", identity.ErrForbidden, 404}}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := &reviewerFake{}
			r := reviewerReq(tc.method, tc.path, tc.body)
			if tc.name == "missing auth" {
				r.Header.Del("Authorization")
			}
			w := httptest.NewRecorder()
			reviewerHandler(f, tc.authErr).ServeHTTP(w, r)
			if w.Code != tc.want || len(f.calls) != 0 {
				t.Fatalf("status=%d want=%d calls=%v body=%s", w.Code, tc.want, f.calls, w.Body.String())
			}
		})
	}
}
func TestReviewerCasesDownloadRejectsUnknownMIME(t *testing.T) {
	f := &reviewerFake{d: dv.DownloadedDocument{MIME: "text/html", Content: []byte("secret")}}
	w := httptest.NewRecorder()
	reviewerHandler(f, nil).ServeHTTP(w, reviewerReq("GET", reviewerCasesRoute+"/"+reviewerID+"/files/"+reviewerFile+"/content", ""))
	if w.Code != 404 || strings.Contains(w.Body.String(), "secret") {
		t.Fatalf("status=%d body=%q", w.Code, w.Body.String())
	}
}
func TestReviewerCasesMutationDoesNotAcceptCallerActor(t *testing.T) {
	f := &reviewerFake{x: dv.Snapshot{ID: reviewerID, Revision: 8}}
	w := httptest.NewRecorder()
	reviewerHandler(f, nil).ServeHTTP(w, reviewerReq("POST", reviewerCasesRoute+"/"+reviewerID+"/revoke", `{"purpose":"identity","reason_code":"solicitud_titular","expected_revision":7,"actor":"88888888-8888-4888-8888-888888888888"}`))
	if w.Code != 400 || len(f.calls) != 0 {
		t.Fatalf("status=%d calls=%v", w.Code, f.calls)
	}
}

type reviewerSpaceErrorFake struct {
	*reviewerFake
	reviewErr error
}

func (f *reviewerSpaceErrorFake) Review(_ context.Context, a, id string, c dv.Capability, d dv.Decision, r dv.ReasonCode, u time.Time, v uint64) error {
	f.calls = append(f.calls, "review")
	return f.reviewErr
}

func TestReviewerCasesHandlerSecurityBoundariesAndSpaceReview(t *testing.T) {
	// 1. observedExpiryDate returns UTC date format when boundary is not at local dayStart
	midday := time.Date(2031, 4, 5, 12, 0, 0, 0, time.UTC)
	if got := observedExpiryDate(midday); got != "2031-04-05" {
		t.Fatalf("observedExpiryDate(%v) = %q, want 2031-04-05", midday, got)
	}

	// 2. Nil handler receiver and nil dependencies return 503 Service Unavailable
	req := reviewerReq(http.MethodGet, reviewerCasesRoute, "")
	w := httptest.NewRecorder()
	(*ReviewerCasesHandler)(nil).ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("nil receiver: expected 503, got %d", w.Code)
	}

	w = httptest.NewRecorder()
	NewReviewerCasesHandler(nil, &reviewerFake{}).ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("nil auth: expected 503, got %d", w.Code)
	}

	w = httptest.NewRecorder()
	NewReviewerCasesHandler(reviewerAuth{}, nil).ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("nil service: expected 503, got %d", w.Code)
	}

	// 3. Unauthorized error (non-forbidden) returns 401 with WWW-Authenticate: Bearer
	w = httptest.NewRecorder()
	reviewerHandler(&reviewerFake{}, identity.ErrUnauthorized).ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized || w.Header().Get("WWW-Authenticate") != "Bearer" {
		t.Fatalf("expected 401 with WWW-Authenticate: Bearer, got %d (headers: %v)", w.Code, w.Header())
	}

	// 4. URL path without slash prefix after route prefix returns 404
	reqBadPrefix := reviewerReq(http.MethodGet, reviewerCasesRoute+"-suffix", "")
	w = httptest.NewRecorder()
	reviewerHandler(&reviewerFake{}, nil).ServeHTTP(w, reqBadPrefix)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for bad path prefix, got %d", w.Code)
	}

	// 5. Collection route with method POST returns 405 Method Not Allowed with Allow: GET
	reqPostColl := reviewerReq(http.MethodPost, reviewerCasesRoute, "{}")
	w = httptest.NewRecorder()
	reviewerHandler(&reviewerFake{}, nil).ServeHTTP(w, reqPostColl)
	if w.Code != http.StatusMethodNotAllowed || w.Header().Get("Allow") != http.MethodGet {
		t.Fatalf("expected 405 Method Not Allowed with Allow: GET, got %d (headers: %v)", w.Code, w.Header())
	}

	// 6. Space case review returns mapped error (409 Conflict, 404 Not Found, 400 Invalid, 503 Unavailable)
	for _, tc := range []struct {
		name     string
		err      error
		wantCode int
	}{
		{"conflict", dv.ErrConflict, http.StatusConflict},
		{"transition", dv.ErrTransition, http.StatusNotFound},
		{"not found", dv.ErrNotFound, http.StatusNotFound},
		{"capability", dv.ErrCapability, http.StatusNotFound},
		{"invalid", dv.ErrInvalid, http.StatusBadRequest},
		{"storage failure", dv.ErrDocumentStorage, http.StatusServiceUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base := &reviewerFake{
				x: dv.Snapshot{
					ID:       reviewerID,
					Purpose:  dv.PurposeSpace,
					State:    dv.CaseInReview,
					Revision: 3,
				},
			}
			spaceFake := &reviewerSpaceErrorFake{reviewerFake: base, reviewErr: tc.err}
			body := `{"decision":"approve","capability":"can_publish_spaces","expected_revision":3}`
			wSpace := httptest.NewRecorder()
			NewReviewerCasesHandler(reviewerAuth{}, spaceFake).ServeHTTP(wSpace, reviewerReq(http.MethodPost, reviewerCasesRoute+"/"+reviewerID+"/review", body))
			if wSpace.Code != tc.wantCode {
				t.Fatalf("space review err %v: expected %d, got %d (body: %s)", tc.err, tc.wantCode, wSpace.Code, wSpace.Body.String())
			}
		})
	}

	// 7. Space case review rejects unexpected identity checklist
	baseSpace := &reviewerFake{
		x: dv.Snapshot{
			ID:       reviewerID,
			Purpose:  dv.PurposeSpace,
			State:    dv.CaseInReview,
			Revision: 3,
		},
	}
	invalidBody := `{"decision":"approve","expected_revision":3,"identity_review_checklist":{"names_match":true}}`
	wBadChecklist := httptest.NewRecorder()
	reviewerHandler(baseSpace, nil).ServeHTTP(wBadChecklist, reviewerReq(http.MethodPost, reviewerCasesRoute+"/"+reviewerID+"/review", invalidBody))
	if wBadChecklist.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unexpected checklist on space case, got %d (body: %s)", wBadChecklist.Code, wBadChecklist.Body.String())
	}

	// 8. Review with canceled context returns 503 Service Unavailable
	ctxCanceled, cancel := context.WithCancel(context.Background())
	cancel()
	reviewBody := `{"decision":"approve","capability":"can_publish_spaces","expected_revision":3}`
	reqReviewCanceled := reviewerReq(http.MethodPost, reviewerCasesRoute+"/"+reviewerID+"/review", reviewBody)
	reqReviewCanceled = reqReviewCanceled.WithContext(ctxCanceled)
	wReviewCanceled := httptest.NewRecorder()
	reviewerHandler(baseSpace, nil).ServeHTTP(wReviewCanceled, reqReviewCanceled)
	if wReviewCanceled.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 for canceled review context, got %d", wReviewCanceled.Code)
	}

	// 9. Revoke with canceled context returns 503 Service Unavailable
	reqRevokeCanceled := reviewerReq(http.MethodPost, reviewerCasesRoute+"/"+reviewerID+"/revoke", `{"purpose":"space","reason_code":"document_invalid","expected_revision":3}`)
	reqRevokeCanceled = reqRevokeCanceled.WithContext(ctxCanceled)
	wRevokeCanceled := httptest.NewRecorder()
	reviewerHandler(baseSpace, nil).ServeHTTP(wRevokeCanceled, reqRevokeCanceled)
	if wRevokeCanceled.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 for canceled revoke context, got %d", wRevokeCanceled.Code)
	}
}
