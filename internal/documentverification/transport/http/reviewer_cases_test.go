package http

import (
	"context"
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
	calls      []string
	actor      string
	x          dv.Snapshot
	d          dv.DownloadedDocument
	err        error
	reviewArgs []any
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
func (f *reviewerFake) Review(_ context.Context, a, id string, c dv.Capability, d dv.Decision, r dv.ReasonCode, u time.Time, v uint64) error {
	f.calls = append(f.calls, "review")
	f.actor = a
	f.reviewArgs = []any{c, d, r, u, v}
	return f.err
}
func (f *reviewerFake) Revoke(_ context.Context, a, id string, p dv.Purpose, r dv.ReasonCode, v uint64) error {
	f.calls = append(f.calls, "revoke")
	f.actor = a
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
	}{{"list", "GET", reviewerCasesRoute, "", "list", 200}, {"case", "GET", reviewerCasesRoute + "/" + reviewerID, "", "load", 200}, {"pdf", "GET", reviewerCasesRoute + "/" + reviewerID + "/files/" + reviewerFile + "/content", "", "download", 200}, {"review", "POST", reviewerCasesRoute + "/" + reviewerID + "/review", `{"capability":"identity","decision":"approve","reason_code":"verified","expected_revision":7}`, "review", 200}, {"revoke", "POST", reviewerCasesRoute + "/" + reviewerID + "/revoke", `{"purpose":"identity","reason_code":"owner_request","expected_revision":7}`, "revoke", 200}}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := &reviewerFake{x: x, d: dv.DownloadedDocument{MIME: "application/pdf", Content: []byte("%PDF")}}
			w := httptest.NewRecorder()
			reviewerHandler(f, nil).ServeHTTP(w, reviewerReq(tc.method, tc.path, tc.body))
			if w.Code != tc.status || len(f.calls) != 2 && tc.call == "review" && false {
				t.Fatalf("status=%d calls=%v body=%s", w.Code, f.calls, w.Body.String())
			}
			if f.actor != reviewerActor || len(f.calls) == 0 || f.calls[0] != tc.call {
				t.Fatalf("actor=%q calls=%v", f.actor, f.calls)
			}
			if tc.name == "pdf" && (w.Header().Get("Content-Type") != "application/pdf" || w.Body.String() != "%PDF" || w.Header().Get("Cache-Control") != "private, no-store" || w.Header().Get("X-Content-Type-Options") != "nosniff") {
				t.Fatalf("download headers=%v body=%q", w.Header(), w.Body.String())
			}
			if tc.name == "review" || tc.name == "revoke" {
				if len(f.calls) != 2 || f.calls[1] != "load" || w.Header().Get("ETag") != "\"7\"" {
					t.Fatalf("post then metadata load: calls=%v etag=%q", f.calls, w.Header().Get("ETag"))
				}
			}
		})
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
	reviewerHandler(f, nil).ServeHTTP(w, reviewerReq("POST", reviewerCasesRoute+"/"+reviewerID+"/revoke", `{"purpose":"identity","reason_code":"owner_request","expected_revision":7,"actor":"88888888-8888-4888-8888-888888888888"}`))
	if w.Code != 400 || len(f.calls) != 0 {
		t.Fatalf("status=%d calls=%v", w.Code, f.calls)
	}
}
