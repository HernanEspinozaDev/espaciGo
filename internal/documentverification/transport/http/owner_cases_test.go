package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
)

const ownerCasesID = "99999999-9999-4999-8999-999999999999"
const casesActor = "66666666-6666-4666-8666-666666666666"

type casesServiceFake struct {
	started   documentverification.Snapshot
	loaded    documentverification.Snapshot
	loadQueue []documentverification.Snapshot
	listed    []documentverification.Snapshot
	err       error
	submitErr error
	actor     string
	rev       uint64
	purpose   documentverification.Purpose
	calls     []string
}

func (f *casesServiceFake) Start(_ context.Context, actor string, in documentverification.CreateInput) (documentverification.Snapshot, error) {
	f.actor = actor
	f.calls = append(f.calls, "start")
	return f.started, f.err
}
func (f *casesServiceFake) ListOwnCases(_ context.Context, actor string) ([]documentverification.Snapshot, error) {
	f.actor = actor
	f.calls = append(f.calls, "list")
	return f.listed, f.err
}
func (f *casesServiceFake) LoadOwnCase(_ context.Context, actor, id string) (documentverification.Snapshot, error) {
	f.actor = actor
	f.calls = append(f.calls, "load")
	if len(f.loadQueue) > 0 {
		x := f.loadQueue[0]
		f.loadQueue = f.loadQueue[1:]
		return x, nil
	}
	return f.loaded, f.err
}
func (f *casesServiceFake) Submit(_ context.Context, actor, id string, p documentverification.Purpose, rev uint64) error {
	f.actor = actor
	f.rev = rev
	f.purpose = p
	f.calls = append(f.calls, "submit")
	if f.submitErr != nil {
		return f.submitErr
	}
	return f.err
}

type casesAuthFake struct {
	principal identity.Principal
	err       error
}

func (a casesAuthFake) Authorize(_ context.Context, _ identity.Secret, _ identity.Role, _ identity.ActivityKind) (identity.Principal, error) {
	return a.principal, a.err
}
func casesSnapshot() documentverification.Snapshot {
	return documentverification.Snapshot{ID: ownerCasesID, SubjectID: casesActor, Purpose: documentverification.PurposeSpace, SpaceID: "88888888-8888-4888-8888-888888888888", State: documentverification.CaseDraft, Revision: 3, DocumentExpiresAt: time.Date(2030, 1, 2, 0, 0, 0, 0, time.UTC), Files: []documentverification.File{{ID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", State: documentverification.FilePending, Size: 12, MIME: "image/png", Pages: 1, ContentSHA256: [32]byte{1}}}}
}
func casesHandler(f *casesServiceFake) http.Handler {
	return NewOwnerCasesHandler(casesAuthFake{principal: identity.Principal{AccountID: casesActor}}, f)
}

func assertOwnerCaseDTOKeys(t *testing.T, body []byte) {
	t.Helper()
	var object map[string]json.RawMessage
	if err := json.Unmarshal(body, &object); err != nil {
		t.Fatalf("decode owner case DTO: %v", err)
	}
	for _, key := range []string{"id", "purpose", "space_id", "state", "revision", "document_expires_at", "files"} {
		if _, ok := object[key]; !ok {
			t.Errorf("owner case DTO missing key %q", key)
		}
	}
	if len(object) != 7 {
		t.Errorf("owner case DTO has unexpected keys: %v", object)
	}
	var files []map[string]json.RawMessage
	if err := json.Unmarshal(object["files"], &files); err != nil {
		t.Fatalf("decode owner file DTOs: %v", err)
	}
	for _, file := range files {
		for _, key := range []string{"id", "state", "size", "mime", "pages"} {
			if _, ok := file[key]; !ok {
				t.Errorf("owner file DTO missing key %q", key)
			}
		}
		if len(file) != 5 {
			t.Errorf("owner file DTO has unexpected keys: %v", file)
		}
	}
}

func assertOwnerCaseListDTOKeys(t *testing.T, body []byte) {
	t.Helper()
	var entries []json.RawMessage
	if err := json.Unmarshal(body, &entries); err != nil {
		t.Fatalf("decode owner case list: %v", err)
	}
	for _, entry := range entries {
		assertOwnerCaseDTOKeys(t, entry)
	}
}

func TestOwnerCasesCreateUsesAuthenticatedActorAndAllowlistedDTO(t *testing.T) {
	f := &casesServiceFake{started: casesSnapshot()}
	req := httptest.NewRequest("POST", "/api/v1/document-verification/cases", strings.NewReader(`{"purpose":"space","space_id":"88888888-8888-4888-8888-888888888888"}`))
	req.Header.Set("Authorization", "Bearer synthetic-session")
	w := httptest.NewRecorder()
	casesHandler(f).ServeHTTP(w, req)
	if w.Code != 201 || f.actor != casesActor {
		t.Fatalf("status=%d actor=%q body=%s", w.Code, f.actor, w.Body.String())
	}
	assertOwnerCaseDTOKeys(t, w.Body.Bytes())
	for _, secret := range []string{"ContentSHA256", "content_sha256", "original_filename", "object_key", "ReviewerID"} {
		if strings.Contains(w.Body.String(), secret) {
			t.Fatalf("leaked %s: %s", secret, w.Body.String())
		}
	}
	if w.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatal(w.Header())
	}
}
func TestOwnerCasesListReadAndSubmitBindActorAndRevision(t *testing.T) {
	for _, method := range []string{"GET", "READ", "SUBMIT"} {
		t.Run(method, func(t *testing.T) {
			f := &casesServiceFake{loaded: casesSnapshot(), listed: []documentverification.Snapshot{casesSnapshot()}}
			if method == "SUBMIT" {
				before, after := casesSnapshot(), casesSnapshot()
				after.Revision = 4
				f.loadQueue = []documentverification.Snapshot{before, after}
			}
			path := "/api/v1/document-verification/cases"
			if method != "GET" {
				path += "/" + ownerCasesID
			}
			if method == "SUBMIT" {
				path += "/submit"
			}
			verb := http.MethodGet
			if method == "SUBMIT" {
				verb = http.MethodPost
			}
			req := httptest.NewRequest(verb, path, nil)
			req.Header.Set("Authorization", "Bearer synthetic-session")
			if method == "SUBMIT" {
				req.Header.Set("If-Match", `"3"`)
			}
			w := httptest.NewRecorder()
			casesHandler(f).ServeHTTP(w, req)
			if w.Code < 200 || w.Code >= 300 || f.actor != casesActor {
				t.Fatalf("status=%d actor=%q body=%s", w.Code, f.actor, w.Body.String())
			}
			if method == "SUBMIT" && (f.rev != 3 || f.purpose != documentverification.PurposeSpace) {
				t.Fatalf("submit rev=%d purpose=%s", f.rev, f.purpose)
			}
			switch method {
			case "GET":
				assertOwnerCaseListDTOKeys(t, w.Body.Bytes())
			case "READ", "SUBMIT":
				assertOwnerCaseDTOKeys(t, w.Body.Bytes())
			}
			if method == "SUBMIT" && w.Header().Get("ETag") != `"4"` {
				t.Fatalf("successful submit ETag=%q", w.Header().Get("ETag"))
			}
			if w.Header().Get("Cache-Control") != "private, no-store" {
				t.Fatal(w.Header())
			}
		})
	}
}
func TestOwnerCasesRejectsMissingOrStaleETag(t *testing.T) {
	for _, tc := range []struct {
		name, etag string
		loadedRev  uint64
		err        error
		want       int
	}{{"missing", "", 3, nil, 428}, {"stale", `"2"`, 3, nil, 409}, {"repository conflict", `"3"`, 3, documentverification.ErrConflict, 409}} {
		t.Run(tc.name, func(t *testing.T) {
			loaded := casesSnapshot()
			loaded.Revision = tc.loadedRev
			f := &casesServiceFake{loaded: loaded, err: tc.err}
			if tc.name == "repository conflict" {
				f.err = nil
				f.submitErr = tc.err
			}
			req := httptest.NewRequest("POST", "/api/v1/document-verification/cases/"+ownerCasesID+"/submit", nil)
			req.Header.Set("Authorization", "Bearer synthetic-session")
			req.Header.Set("If-Match", tc.etag)
			w := httptest.NewRecorder()
			casesHandler(f).ServeHTTP(w, req)
			if w.Code != tc.want {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}
			if w.Header().Get("Cache-Control") != "private, no-store" {
				t.Fatalf("error response cache headers=%v", w.Header())
			}
			if tc.name == "stale" && (len(f.calls) != 1 || f.calls[0] != "load") {
				t.Fatalf("stale ETag must not submit the case; calls=%v", f.calls)
			}
			if tc.name == "repository conflict" {
				if len(f.calls) != 2 || f.calls[0] != "load" || f.calls[1] != "submit" || f.rev != tc.loadedRev {
					t.Fatalf("repository CAS conflict was not reached with loaded revision: calls=%v rev=%d", f.calls, f.rev)
				}
			}
		})
	}
}
func TestOwnerCasesAuthMalformedJSONAndHiddenNotFound(t *testing.T) {
	for _, tc := range []struct {
		name, auth, body string
		authErr, svcErr  error
		want             int
	}{{"auth", "", `{}`, nil, nil, 401}, {"bad json", "Bearer synthetic-session", `{"purpose":"identity","unexpected":true}`, nil, nil, 400}, {"hidden not found", "Bearer synthetic-session", "", nil, documentverification.ErrNotFound, 404}, {"sanitized service error", "Bearer synthetic-session", `{}`, nil, errors.New("private backend diagnostic"), 503}} {
		t.Run(tc.name, func(t *testing.T) {
			f := &casesServiceFake{err: tc.svcErr}
			h := NewOwnerCasesHandler(casesAuthFake{principal: identity.Principal{AccountID: casesActor}, err: tc.authErr}, f)
			method, path := "POST", "/api/v1/document-verification/cases"
			if tc.name == "hidden not found" {
				method, path = "GET", "/api/v1/document-verification/cases/"+ownerCasesID
			}
			req := httptest.NewRequest(method, path, strings.NewReader(tc.body))
			if tc.auth != "" {
				req.Header.Set("Authorization", tc.auth)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)
			if w.Code != tc.want {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}
			if tc.name == "sanitized service error" && strings.Contains(w.Body.String(), "private backend diagnostic") {
				t.Fatalf("internal error leaked: %s", w.Body.String())
			}
			if w.Header().Get("Cache-Control") != "private, no-store" {
				t.Fatalf("error response cache headers=%v", w.Header())
			}
		})
	}
}

func TestOwnerCasesCreateRejectsOversizedJSONWith413(t *testing.T) {
	f := &casesServiceFake{started: casesSnapshot()}
	body := `{"purpose":"identity"}` + strings.Repeat(" ", 16<<10)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/document-verification/cases", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer synthetic-session")
	w := httptest.NewRecorder()
	casesHandler(f).ServeHTTP(w, req)
	if w.Code != http.StatusRequestEntityTooLarge || len(f.calls) != 0 {
		t.Fatalf("status=%d calls=%v body=%s", w.Code, f.calls, w.Body.String())
	}
	if w.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("oversized response cache headers=%v", w.Header())
	}
}

func TestOwnerCasesCreateAndReadReturnStrongETag(t *testing.T) {
	t.Run("create", func(t *testing.T) {
		f := &casesServiceFake{started: casesSnapshot()}
		req := httptest.NewRequest(http.MethodPost, "/api/v1/document-verification/cases", strings.NewReader(`{"purpose":"space","space_id":"88888888-8888-4888-8888-888888888888"}`))
		req.Header.Set("Authorization", "Bearer synthetic-session")
		w := httptest.NewRecorder()
		casesHandler(f).ServeHTTP(w, req)
		if w.Code != http.StatusCreated || w.Header().Get("ETag") != `"3"` {
			t.Fatalf("status=%d ETag=%q body=%s", w.Code, w.Header().Get("ETag"), w.Body.String())
		}
	})

	t.Run("read", func(t *testing.T) {
		f := &casesServiceFake{loaded: casesSnapshot()}
		req := httptest.NewRequest(http.MethodGet, "/api/v1/document-verification/cases/"+ownerCasesID, nil)
		req.Header.Set("Authorization", "Bearer synthetic-session")
		w := httptest.NewRecorder()
		casesHandler(f).ServeHTTP(w, req)
		if w.Code != http.StatusOK || w.Header().Get("ETag") != `"3"` {
			t.Fatalf("status=%d ETag=%q body=%s", w.Code, w.Header().Get("ETag"), w.Body.String())
		}
	})
}

func TestOwnerCasesSubmitRejectsNonSingleStrongIfMatch(t *testing.T) {
	tests := []struct {
		name string
		tags []string
	}{{"weak", []string{`W/"3"`}}, {"wildcard", []string{"*"}}, {"list", []string{`"3", "4"`}}, {"duplicate headers", []string{`"3"`, `"3"`}}, {"noncanonical revision", []string{`"03"`}}}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := &casesServiceFake{loaded: casesSnapshot()}
			req := httptest.NewRequest(http.MethodPost, "/api/v1/document-verification/cases/"+ownerCasesID+"/submit", nil)
			req.Header.Set("Authorization", "Bearer synthetic-session")
			for _, tag := range tc.tags {
				req.Header.Add("If-Match", tag)
			}
			w := httptest.NewRecorder()
			casesHandler(f).ServeHTTP(w, req)
			if w.Code != http.StatusBadRequest || len(f.calls) != 0 {
				t.Fatalf("status=%d calls=%v body=%s", w.Code, f.calls, w.Body.String())
			}
			if w.Header().Get("Cache-Control") != "private, no-store" {
				t.Fatalf("error response cache headers=%v", w.Header())
			}
		})
	}
}

func TestOwnerCasesRouteShapeAndMethodAllowlist(t *testing.T) {
	tests := []struct {
		name, method, path string
		want               int
		allow              string
	}{
		{"query string rejected", http.MethodGet, "/api/v1/document-verification/cases/" + ownerCasesID + "?include=files", http.StatusNotFound, ""},
		{"nested case read", http.MethodGet, "/api/v1/document-verification/cases/" + ownerCasesID + "/nested", http.StatusNotFound, ""},
		{"invalid case id", http.MethodGet, "/api/v1/document-verification/cases/not-a-uuid", http.StatusNotFound, ""},
		{"extra submit segment", http.MethodPost, "/api/v1/document-verification/cases/" + ownerCasesID + "/submit/extra", http.StatusNotFound, ""},
		{"extra files segment", http.MethodPost, "/api/v1/document-verification/cases/" + ownerCasesID + "/files/extra", http.StatusNotFound, ""},
		{"unsupported case method", http.MethodDelete, "/api/v1/document-verification/cases/" + ownerCasesID, http.StatusMethodNotAllowed, "GET"},
		{"unsupported files method", http.MethodGet, "/api/v1/document-verification/cases/" + ownerCasesID + "/files", http.StatusMethodNotAllowed, "POST"},
		{"unsupported collection method", http.MethodDelete, "/api/v1/document-verification/cases", http.StatusMethodNotAllowed, "GET, POST"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := &casesServiceFake{loaded: casesSnapshot()}
			req := httptest.NewRequest(tc.method, tc.path, nil)
			req.Header.Set("Authorization", "Bearer synthetic-session")
			w := httptest.NewRecorder()
			casesHandler(f).ServeHTTP(w, req)
			if w.Code != tc.want || len(f.calls) != 0 {
				t.Fatalf("status=%d want=%d calls=%v body=%s", w.Code, tc.want, f.calls, w.Body.String())
			}
			if got := w.Header().Get("Allow"); got != tc.allow {
				t.Fatalf("Allow=%q want=%q", got, tc.allow)
			}
			if w.Header().Get("Cache-Control") != "private, no-store" {
				t.Fatalf("cache headers=%v", w.Header())
			}
		})
	}
}
