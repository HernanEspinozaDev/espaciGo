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
	loadErr   error
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
	if f.loadErr != nil {
		return f.loaded, f.loadErr
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

func TestOwnerCasesSubmitRejectsUncleanFilesAsBadRequest(t *testing.T) {
	loaded := casesSnapshot()
	f := &casesServiceFake{loaded: loaded, submitErr: documentverification.ErrFilesNotClean}
	req := httptest.NewRequest("POST", "/api/v1/document-verification/cases/"+ownerCasesID+"/submit", nil)
	req.Header.Set("Authorization", "Bearer synthetic-session")
	req.Header.Set("If-Match", `"3"`)
	w := httptest.NewRecorder()
	casesHandler(f).ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("unclean-file submit status=%d body=%s, want 400", w.Code, w.Body.String())
	}
	if len(f.calls) != 2 || f.calls[0] != "load" || f.calls[1] != "submit" {
		t.Fatalf("unclean-file submit calls=%v, want load then submit", f.calls)
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

func TestOwnerCasesHandlerSubmissionReplayConflictAndErrorBoundaries(t *testing.T) {
	// 1. Replay of lost successful submission returns 200 with new ETag without calling Submit
	snapSubmitted := casesSnapshot()
	snapSubmitted.State = documentverification.CaseSubmitted
	snapSubmitted.Revision = 4
	fReplay := &casesServiceFake{loaded: snapSubmitted}
	reqReplay := httptest.NewRequest(http.MethodPost, "/api/v1/document-verification/cases/"+ownerCasesID+"/submit", nil)
	reqReplay.Header.Set("Authorization", "Bearer synthetic-session")
	reqReplay.Header.Set("If-Match", `"3"`)
	wReplay := httptest.NewRecorder()
	casesHandler(fReplay).ServeHTTP(wReplay, reqReplay)
	if wReplay.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for submission replay, got %d (body: %s)", wReplay.Code, wReplay.Body.String())
	}
	if wReplay.Header().Get("ETag") != `"4"` {
		t.Fatalf("expected ETag \"4\", got %q", wReplay.Header().Get("ETag"))
	}
	if len(fReplay.calls) != 1 || fReplay.calls[0] != "load" {
		t.Fatalf("expected only load call, got %v", fReplay.calls)
	}

	// 2. Conflict when revision does not match and is not the immediate replay successor
	snapStale := casesSnapshot()
	snapStale.Revision = 5
	fConflict := &casesServiceFake{loaded: snapStale}
	reqConflict := httptest.NewRequest(http.MethodPost, "/api/v1/document-verification/cases/"+ownerCasesID+"/submit", nil)
	reqConflict.Header.Set("Authorization", "Bearer synthetic-session")
	reqConflict.Header.Set("If-Match", `"3"`)
	wConflict := httptest.NewRecorder()
	casesHandler(fConflict).ServeHTTP(wConflict, reqConflict)
	if wConflict.Code != http.StatusConflict || !strings.Contains(wConflict.Body.String(), "case revision conflict") {
		t.Fatalf("expected 409 case revision conflict, got %d (body: %s)", wConflict.Code, wConflict.Body.String())
	}

	// 3. Submit error propagation from service
	for _, tc := range []struct {
		name     string
		svcErr   error
		wantCode int
	}{
		{"conflict", documentverification.ErrConflict, http.StatusConflict},
		{"not found", documentverification.ErrNotFound, http.StatusNotFound},
		{"invalid", documentverification.ErrInvalid, http.StatusBadRequest},
	} {
		t.Run("submit_err_"+tc.name, func(t *testing.T) {
			fErr := &casesServiceFake{loaded: casesSnapshot(), submitErr: tc.svcErr}
			req := httptest.NewRequest(http.MethodPost, "/api/v1/document-verification/cases/"+ownerCasesID+"/submit", nil)
			req.Header.Set("Authorization", "Bearer synthetic-session")
			req.Header.Set("If-Match", `"3"`)
			w := httptest.NewRecorder()
			casesHandler(fErr).ServeHTTP(w, req)
			if w.Code != tc.wantCode {
				t.Fatalf("expected status %d, got %d (body: %s)", tc.wantCode, w.Code, w.Body.String())
			}
		})
	}

	// 4. Create case with expiration timestamp populated
	expiry := time.Date(2032, 5, 1, 0, 0, 0, 0, time.UTC)
	snapCreated := casesSnapshot()
	snapCreated.DocumentExpiresAt = expiry
	fCreate := &casesServiceFake{started: snapCreated}
	bodyCreate := `{"purpose":"space","space_id":"88888888-8888-4888-8888-888888888888","document_expires_at":"2032-05-01T00:00:00Z"}`
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/v1/document-verification/cases", strings.NewReader(bodyCreate))
	reqCreate.Header.Set("Authorization", "Bearer synthetic-session")
	reqCreate.Header.Set("Content-Type", "application/json")
	wCreate := httptest.NewRecorder()
	casesHandler(fCreate).ServeHTTP(wCreate, reqCreate)
	if wCreate.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d (body: %s)", wCreate.Code, wCreate.Body.String())
	}
	if wCreate.Header().Get("ETag") != `"3"` {
		t.Fatalf("expected ETag \"3\", got %q", wCreate.Header().Get("ETag"))
	}

	// 5. List error propagation
	fListErr := &casesServiceFake{err: documentverification.ErrNotFound}
	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/document-verification/cases", nil)
	reqList.Header.Set("Authorization", "Bearer synthetic-session")
	wList := httptest.NewRecorder()
	casesHandler(fListErr).ServeHTTP(wList, reqList)
	if wList.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for list error, got %d (body: %s)", wList.Code, wList.Body.String())
	}

	// 6. ownerSnapshotDTO purpose and validation boundaries
	t.Run("dto_identity_valid", func(t *testing.T) {
		snapIdentity := documentverification.Snapshot{
			ID:        ownerCasesID,
			SubjectID: casesActor,
			Purpose:   documentverification.PurposeIdentity,
			State:     documentverification.CaseDraft,
			Revision:  1,
		}
		fIdentity := &casesServiceFake{loaded: snapIdentity}
		req := httptest.NewRequest(http.MethodGet, "/api/v1/document-verification/cases/"+ownerCasesID, nil)
		req.Header.Set("Authorization", "Bearer synthetic-session")
		w := httptest.NewRecorder()
		casesHandler(fIdentity).ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 for valid identity case, got %d (body: %s)", w.Code, w.Body.String())
		}
	})

	t.Run("dto_identity_with_space_id_rejected", func(t *testing.T) {
		snapIdentityWithSpace := documentverification.Snapshot{
			ID:        ownerCasesID,
			SubjectID: casesActor,
			Purpose:   documentverification.PurposeIdentity,
			SpaceID:   "88888888-8888-4888-8888-888888888888",
			State:     documentverification.CaseDraft,
			Revision:  1,
		}
		fBad := &casesServiceFake{loaded: snapIdentityWithSpace}
		req := httptest.NewRequest(http.MethodGet, "/api/v1/document-verification/cases/"+ownerCasesID, nil)
		req.Header.Set("Authorization", "Bearer synthetic-session")
		w := httptest.NewRecorder()
		casesHandler(fBad).ServeHTTP(w, req)
		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404 for identity case with space_id, got %d (body: %s)", w.Code, w.Body.String())
		}
	})

	t.Run("dto_space_with_invalid_space_id_rejected", func(t *testing.T) {
		snapSpaceBadSpaceID := documentverification.Snapshot{
			ID:        ownerCasesID,
			SubjectID: casesActor,
			Purpose:   documentverification.PurposeSpace,
			SpaceID:   "invalid-uuid",
			State:     documentverification.CaseDraft,
			Revision:  1,
		}
		fBad := &casesServiceFake{loaded: snapSpaceBadSpaceID}
		req := httptest.NewRequest(http.MethodGet, "/api/v1/document-verification/cases/"+ownerCasesID, nil)
		req.Header.Set("Authorization", "Bearer synthetic-session")
		w := httptest.NewRecorder()
		casesHandler(fBad).ServeHTTP(w, req)
		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404 for space case with invalid space_id, got %d (body: %s)", w.Code, w.Body.String())
		}
	})

	t.Run("dto_unknown_purpose_rejected", func(t *testing.T) {
		snapUnknown := documentverification.Snapshot{
			ID:        ownerCasesID,
			SubjectID: casesActor,
			Purpose:   documentverification.Purpose("unknown-purpose"),
			State:     documentverification.CaseDraft,
			Revision:  1,
		}
		fBad := &casesServiceFake{loaded: snapUnknown}
		req := httptest.NewRequest(http.MethodGet, "/api/v1/document-verification/cases/"+ownerCasesID, nil)
		req.Header.Set("Authorization", "Bearer synthetic-session")
		w := httptest.NewRecorder()
		casesHandler(fBad).ServeHTTP(w, req)
		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404 for unknown purpose, got %d (body: %s)", w.Code, w.Body.String())
		}
	})

	// 7. RegisterOwnerCasesRoutes mounts mux endpoints correctly
	mux := http.NewServeMux()
	RegisterOwnerCasesRoutes(mux, casesHandler(&casesServiceFake{
		listed: []documentverification.Snapshot{casesSnapshot()},
		loaded: casesSnapshot(),
	}))
	reqMux := httptest.NewRequest(http.MethodGet, "/api/v1/document-verification/cases", nil)
	reqMux.Header.Set("Authorization", "Bearer synthetic-session")
	wMux := httptest.NewRecorder()
	mux.ServeHTTP(wMux, reqMux)
	if wMux.Code != http.StatusOK {
		t.Fatalf("expected 200 from mounted mux route, got %d (body: %s)", wMux.Code, wMux.Body.String())
	}
	reqMuxPrefix := httptest.NewRequest(http.MethodGet, "/api/v1/document-verification/cases/"+ownerCasesID, nil)
	reqMuxPrefix.Header.Set("Authorization", "Bearer synthetic-session")
	wMuxPrefix := httptest.NewRecorder()
	mux.ServeHTTP(wMuxPrefix, reqMuxPrefix)
	if wMuxPrefix.Code != http.StatusOK {
		t.Fatalf("expected 200 from mounted mux prefix route, got %d (body: %s)", wMuxPrefix.Code, wMuxPrefix.Body.String())
	}
}

func TestOwnerCasesHandlerCorruptStateReloadAndRouteBoundaries(t *testing.T) {
	// 1. Submit pre-check fails if LoadOwnCase returns error
	fLoadErr := &casesServiceFake{err: documentverification.ErrNotFound}
	reqSubmitLoadErr := httptest.NewRequest(http.MethodPost, "/api/v1/document-verification/cases/"+ownerCasesID+"/submit", nil)
	reqSubmitLoadErr.Header.Set("Authorization", "Bearer synthetic-session")
	reqSubmitLoadErr.Header.Set("If-Match", `"3"`)
	wSubmitLoadErr := httptest.NewRecorder()
	casesHandler(fLoadErr).ServeHTTP(wSubmitLoadErr, reqSubmitLoadErr)
	if wSubmitLoadErr.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for submit load error, got %d", wSubmitLoadErr.Code)
	}

	// 2. Submit pre-check fails if LoadOwnCase returns corrupt snapshot
	snapCorrupt := casesSnapshot()
	snapCorrupt.SubjectID = "different-actor"
	fPreCorrupt := &casesServiceFake{loaded: snapCorrupt}
	reqPreCorrupt := httptest.NewRequest(http.MethodPost, "/api/v1/document-verification/cases/"+ownerCasesID+"/submit", nil)
	reqPreCorrupt.Header.Set("Authorization", "Bearer synthetic-session")
	reqPreCorrupt.Header.Set("If-Match", `"3"`)
	wPreCorrupt := httptest.NewRecorder()
	casesHandler(fPreCorrupt).ServeHTTP(wPreCorrupt, reqPreCorrupt)
	if wPreCorrupt.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for submit corrupt pre-snapshot, got %d", wPreCorrupt.Code)
	}

	// 3. Submit post-reload fails if LoadOwnCase returns error after Submit succeeded
	fPostReloadErr := &casesServiceFake{
		loadQueue: []documentverification.Snapshot{casesSnapshot()},
		loadErr:   documentverification.ErrNotFound,
	}
	reqPostReloadErr := httptest.NewRequest(http.MethodPost, "/api/v1/document-verification/cases/"+ownerCasesID+"/submit", nil)
	reqPostReloadErr.Header.Set("Authorization", "Bearer synthetic-session")
	reqPostReloadErr.Header.Set("If-Match", `"3"`)
	wPostReloadErr := httptest.NewRecorder()
	casesHandler(fPostReloadErr).ServeHTTP(wPostReloadErr, reqPostReloadErr)
	if wPostReloadErr.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for submit post-reload error, got %d", wPostReloadErr.Code)
	}

	// 4. Submit post-reload fails if reload snapshot is corrupt
	snapCorruptReload := casesSnapshot()
	snapCorruptReload.SubjectID = "different-actor"
	fPostReloadCorrupt := &casesServiceFake{
		loadQueue: []documentverification.Snapshot{casesSnapshot(), snapCorruptReload},
	}
	reqPostReloadCorrupt := httptest.NewRequest(http.MethodPost, "/api/v1/document-verification/cases/"+ownerCasesID+"/submit", nil)
	reqPostReloadCorrupt.Header.Set("Authorization", "Bearer synthetic-session")
	reqPostReloadCorrupt.Header.Set("If-Match", `"3"`)
	wPostReloadCorrupt := httptest.NewRecorder()
	casesHandler(fPostReloadCorrupt).ServeHTTP(wPostReloadCorrupt, reqPostReloadCorrupt)
	if wPostReloadCorrupt.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for submit post-reload corrupt snapshot, got %d", wPostReloadCorrupt.Code)
	}

	// 5. Create fails with 404 if Start returns snapshot with subject mismatch
	snapStartCorrupt := casesSnapshot()
	snapStartCorrupt.SubjectID = "different-actor"
	fStartCorrupt := &casesServiceFake{started: snapStartCorrupt}
	bodyCreate := `{"purpose":"space","space_id":"88888888-8888-4888-8888-888888888888"}`
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/v1/document-verification/cases", strings.NewReader(bodyCreate))
	reqCreate.Header.Set("Authorization", "Bearer synthetic-session")
	reqCreate.Header.Set("Content-Type", "application/json")
	wCreate := httptest.NewRecorder()
	casesHandler(fStartCorrupt).ServeHTTP(wCreate, reqCreate)
	if wCreate.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for create corrupt snapshot, got %d", wCreate.Code)
	}

	// 6. List fails with 404 if a returned snapshot is corrupt
	snapListCorrupt := casesSnapshot()
	snapListCorrupt.SubjectID = "different-actor"
	fListCorrupt := &casesServiceFake{listed: []documentverification.Snapshot{snapListCorrupt}}
	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/document-verification/cases", nil)
	reqList.Header.Set("Authorization", "Bearer synthetic-session")
	wList := httptest.NewRecorder()
	casesHandler(fListCorrupt).ServeHTTP(wList, reqList)
	if wList.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for list corrupt snapshot, got %d", wList.Code)
	}

	// 7. Route prefix mismatch returns 404
	reqBadPrefix := httptest.NewRequest(http.MethodGet, "/api/v1/document-verification/other-cases", nil)
	reqBadPrefix.Header.Set("Authorization", "Bearer synthetic-session")
	wBadPrefix := httptest.NewRecorder()
	casesHandler(&casesServiceFake{}).ServeHTTP(wBadPrefix, reqBadPrefix)
	if wBadPrefix.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for bad route prefix, got %d", wBadPrefix.Code)
	}

	// 8. ownerSnapshotDTO direct validation edge cases
	if _, err := ownerSnapshotDTO(casesSnapshot(), "", ""); !errors.Is(err, documentverification.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for empty actor, got %v", err)
	}
	badIDSnap := casesSnapshot()
	badIDSnap.ID = "invalid-uuid"
	if _, err := ownerSnapshotDTO(badIDSnap, casesActor, ""); !errors.Is(err, documentverification.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for invalid snapshot ID, got %v", err)
	}
	if _, err := ownerSnapshotDTO(casesSnapshot(), casesActor, "mismatched-uuid"); !errors.Is(err, documentverification.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for requested ID mismatch, got %v", err)
	}
}
