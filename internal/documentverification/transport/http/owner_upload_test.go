package http

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
)

const ownerUploadActor = "66666666-6666-4666-8666-666666666666"
const ownerUploadCase = "99999999-9999-4999-8999-999999999999"

type ownerUploadAuth struct{}

func (ownerUploadAuth) Authorize(context.Context, identity.Secret, identity.Role, identity.ActivityKind) (identity.Principal, error) {
	return identity.Principal{AccountID: ownerUploadActor}, nil
}

type ownerUploadCases struct{}

func (ownerUploadCases) Start(context.Context, string, documentverification.CreateInput) (documentverification.Snapshot, error) {
	return documentverification.Snapshot{}, nil
}
func (ownerUploadCases) ListOwnCases(context.Context, string) ([]documentverification.Snapshot, error) {
	return nil, nil
}
func (ownerUploadCases) LoadOwnCase(context.Context, string, string) (documentverification.Snapshot, error) {
	return documentverification.Snapshot{}, nil
}
func (ownerUploadCases) Submit(context.Context, string, string, documentverification.Purpose, uint64) error {
	return nil
}

type ownerUploadFake struct {
	actor, caseID string
	revision      uint64
	role          documentverification.EvidenceRole
	content       []byte
	receipt       documentverification.UploadReceipt
	err           error
	calls         int
}

func (f *ownerUploadFake) Upload(_ context.Context, actor, caseID string, revision uint64, role documentverification.EvidenceRole, content []byte) (documentverification.UploadReceipt, error) {
	f.calls++
	f.actor, f.caseID, f.revision = actor, caseID, revision
	f.role = role
	f.content = append([]byte(nil), content...)
	return f.receipt, f.err
}

func ownerUploadRequest(t *testing.T, path, etag string, content []byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	part, err := mw.CreateFormFile("file", "ignored-original-name.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatal(err)
	}
	rolePart, err := mw.CreateFormField("evidence_role")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rolePart.Write([]byte(string(documentverification.EvidenceRoleIdentityFront))); err != nil {
		t.Fatal(err)
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, path, &body)
	req.Header.Set("Authorization", "Bearer synthetic-session")
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if etag != "" {
		req.Header.Set("If-Match", etag)
	}
	return req
}

func TestOwnerCaseUploadUsesAuthenticatedActorAndRevision(t *testing.T) {
	content := []byte("synthetic document bytes")
	fake := &ownerUploadFake{receipt: documentverification.UploadReceipt{
		File:     documentverification.File{ID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", EvidenceRole: documentverification.EvidenceRoleIdentityFront, State: documentverification.FilePending, Size: int64(len(content)), MIME: "image/png", Pages: 1},
		Revision: 4,
	}}
	handler := NewOwnerCasesHandlerWithUpload(ownerUploadAuth{}, ownerUploadCases{}, fake)
	request := ownerUploadRequest(t, "/api/v1/document-verification/cases/"+ownerUploadCase+"/files", `"3"`, content)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if fake.calls != 1 || fake.actor != ownerUploadActor || fake.caseID != ownerUploadCase || fake.revision != 3 || fake.role != documentverification.EvidenceRoleIdentityFront || !bytes.Equal(fake.content, content) {
		t.Fatalf("upload call=%+v", fake)
	}
	if response.Header().Get("ETag") != `"4"` || response.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("headers=%v", response.Header())
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode upload response: %v", err)
	}
	if len(payload) != 2 || payload["file"] == nil || payload["revision"] == nil {
		t.Fatalf("upload response keys=%v", payload)
	}
	var file map[string]json.RawMessage
	if err := json.Unmarshal(payload["file"], &file); err != nil {
		t.Fatalf("decode uploaded file DTO: %v", err)
	}
	fileKeys := []string{"id", "evidence_role", "state", "size", "mime", "pages"}
	for _, key := range fileKeys {
		if _, ok := file[key]; !ok {
			t.Errorf("uploaded file DTO missing key %q", key)
		}
	}
	if string(file["evidence_role"]) != `"identity_front"` {
		t.Errorf("uploaded file evidence role=%s, want identity_front", file["evidence_role"])
	}
	if len(file) != len(fileKeys) {
		t.Errorf("uploaded file DTO has unexpected keys: %v", file)
	}
	for _, secret := range []string{"ignored-original-name.png", "object_key", "ContentSHA256", "content_sha256"} {
		if bytes.Contains(response.Body.Bytes(), []byte(secret)) {
			t.Fatalf("response leaked %q: %s", secret, response.Body.String())
		}
	}
}

func TestOwnerCaseUploadRejectsInvalidPreconditionsAndOversizedFile(t *testing.T) {
	tests := []struct {
		name string
		etag string
		body []byte
		want int
	}{
		{name: "missing precondition", body: []byte("small"), want: http.StatusPreconditionRequired},
		{name: "malformed precondition", etag: `W/"3"`, body: []byte("small"), want: http.StatusBadRequest},
		{name: "noncanonical revision", etag: `"03"`, body: []byte("small"), want: http.StatusBadRequest},
		{name: "oversized file", etag: `"3"`, body: bytes.Repeat([]byte("x"), documentverification.MaxUploadBytes+1), want: http.StatusRequestEntityTooLarge},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fake := &ownerUploadFake{}
			handler := NewOwnerCasesHandlerWithUpload(ownerUploadAuth{}, ownerUploadCases{}, fake)
			request := ownerUploadRequest(t, "/api/v1/document-verification/cases/"+ownerUploadCase+"/files", tc.etag, tc.body)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != tc.want {
				t.Fatalf("status=%d want=%d body=%s", response.Code, tc.want, response.Body.String())
			}
			if fake.calls != 0 {
				t.Fatalf("invalid request reached upload service: %+v", fake)
			}
		})
	}
}

func TestOwnerCaseUploadRejectsDuplicateIfMatchHeaders(t *testing.T) {
	fake := &ownerUploadFake{}
	handler := NewOwnerCasesHandlerWithUpload(ownerUploadAuth{}, ownerUploadCases{}, fake)
	request := ownerUploadRequest(t, "/api/v1/document-verification/cases/"+ownerUploadCase+"/files", `"3"`, []byte("small"))
	request.Header.Add("If-Match", `"3"`)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest || fake.calls != 0 {
		t.Fatalf("status=%d calls=%d body=%s", response.Code, fake.calls, response.Body.String())
	}
}

func TestOwnerCaseUploadRejectsNonMultipartContentType(t *testing.T) {
	fake := &ownerUploadFake{}
	handler := NewOwnerCasesHandlerWithUpload(ownerUploadAuth{}, ownerUploadCases{}, fake)
	request := ownerUploadRequest(t, "/api/v1/document-verification/cases/"+ownerUploadCase+"/files", `"3"`, []byte("small"))
	request.Header.Set("Content-Type", "application/octet-stream")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnsupportedMediaType || fake.calls != 0 {
		t.Fatalf("status=%d calls=%d body=%s", response.Code, fake.calls, response.Body.String())
	}
}

func TestOwnerCaseUploadAllowsOmittedRoleForSpaceCases(t *testing.T) {
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	file, err := mw.CreateFormFile("file", "evidence.png")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = file.Write([]byte("synthetic"))
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/document-verification/cases/"+ownerUploadCase+"/files", &body)
	req.Header.Set("Authorization", "Bearer synthetic-session")
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("If-Match", `"3"`)
	fake := &ownerUploadFake{receipt: documentverification.UploadReceipt{File: documentverification.File{ID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", State: documentverification.FilePending, Size: 9, MIME: "image/png", Pages: 1}, Revision: 4}}
	response := httptest.NewRecorder()
	NewOwnerCasesHandlerWithUpload(ownerUploadAuth{}, ownerUploadCases{}, fake).ServeHTTP(response, req)
	if response.Code != http.StatusCreated || fake.calls != 1 || fake.role != "" {
		t.Fatalf("file-only space-compatible upload status=%d calls=%d role=%q body=%s", response.Code, fake.calls, fake.role, response.Body.String())
	}
	var payload struct {
		File map[string]json.RawMessage `json:"file"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode file-only response: %v", err)
	}
	if _, ok := payload.File["evidence_role"]; ok {
		t.Fatalf("space-compatible file-only response unexpectedly included evidence_role: %v", payload.File)
	}
}

func TestOwnerCaseUploadRejectsAdditionalMultipartParts(t *testing.T) {
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	file, err := mw.CreateFormFile("file", "synthetic.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write([]byte("synthetic")); err != nil {
		t.Fatal(err)
	}
	extra, err := mw.CreateFormField("metadata")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := extra.Write([]byte("must not be accepted")); err != nil {
		t.Fatal(err)
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/document-verification/cases/"+ownerUploadCase+"/files", &body)
	request.Header.Set("Authorization", "Bearer synthetic-session")
	request.Header.Set("Content-Type", mw.FormDataContentType())
	request.Header.Set("If-Match", `"3"`)
	fake := &ownerUploadFake{}
	response := httptest.NewRecorder()
	NewOwnerCasesHandlerWithUpload(ownerUploadAuth{}, ownerUploadCases{}, fake).ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest || fake.calls != 0 {
		t.Fatalf("status=%d calls=%d body=%s", response.Code, fake.calls, response.Body.String())
	}
}

type ownerUploadAuthWithErr struct {
	err error
}

func (a ownerUploadAuthWithErr) Authorize(context.Context, identity.Secret, identity.Role, identity.ActivityKind) (identity.Principal, error) {
	if a.err != nil {
		return identity.Principal{}, a.err
	}
	return identity.Principal{AccountID: ownerUploadActor}, nil
}

func TestOwnerCaseUploadQuotaEmptyFileAndSecurityBoundaries(t *testing.T) {
	const path = "/api/v1/document-verification/cases/" + ownerUploadCase + "/files"

	// 1. Quota reached returns 409 Conflict
	fakeQuota := &ownerUploadFake{err: documentverification.ErrQuota}
	reqQuota := ownerUploadRequest(t, path, `"3"`, []byte("synthetic content"))
	resQuota := httptest.NewRecorder()
	NewOwnerCasesHandlerWithUpload(ownerUploadAuth{}, ownerUploadCases{}, fakeQuota).ServeHTTP(resQuota, reqQuota)
	if resQuota.Code != http.StatusConflict || !bytes.Contains(resQuota.Body.Bytes(), []byte("document limit reached")) {
		t.Fatalf("expected 409 document limit reached, got %d (body: %s)", resQuota.Code, resQuota.Body.String())
	}

	// 2. Empty file returns 400 Bad Request
	fakeEmpty := &ownerUploadFake{}
	reqEmpty := ownerUploadRequest(t, path, `"3"`, []byte{})
	resEmpty := httptest.NewRecorder()
	NewOwnerCasesHandlerWithUpload(ownerUploadAuth{}, ownerUploadCases{}, fakeEmpty).ServeHTTP(resEmpty, reqEmpty)
	if resEmpty.Code != http.StatusBadRequest || !bytes.Contains(resEmpty.Body.Bytes(), []byte("empty file")) || fakeEmpty.calls != 0 {
		t.Fatalf("expected 400 empty file, got %d calls=%d (body: %s)", resEmpty.Code, fakeEmpty.calls, resEmpty.Body.String())
	}

	// 3. Missing upload service (upload == nil) returns 503
	reqNoUpload := ownerUploadRequest(t, path, `"3"`, []byte("valid content"))
	resNoUpload := httptest.NewRecorder()
	NewOwnerCasesHandler(ownerUploadAuth{}, ownerUploadCases{}).ServeHTTP(resNoUpload, reqNoUpload)
	if resNoUpload.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when upload service is nil, got %d", resNoUpload.Code)
	}

	// 4. Nil handler receiver or nil auth/service returns 503
	resNil := httptest.NewRecorder()
	(*OwnerCasesHandler)(nil).ServeHTTP(resNil, reqQuota)
	if resNil.Code != http.StatusServiceUnavailable {
		t.Fatalf("nil receiver: expected 503, got %d", resNil.Code)
	}

	resNilAuth := httptest.NewRecorder()
	NewOwnerCasesHandlerWithUpload(nil, ownerUploadCases{}, fakeQuota).ServeHTTP(resNilAuth, reqQuota)
	if resNilAuth.Code != http.StatusServiceUnavailable {
		t.Fatalf("nil auth: expected 503, got %d", resNilAuth.Code)
	}

	resNilCases := httptest.NewRecorder()
	NewOwnerCasesHandlerWithUpload(ownerUploadAuth{}, nil, fakeQuota).ServeHTTP(resNilCases, reqQuota)
	if resNilCases.Code != http.StatusServiceUnavailable {
		t.Fatalf("nil cases service: expected 503, got %d", resNilCases.Code)
	}

	// 5. Auth unauthorized returns 401 with WWW-Authenticate: Bearer
	resUnauth := httptest.NewRecorder()
	NewOwnerCasesHandlerWithUpload(ownerUploadAuthWithErr{err: identity.ErrUnauthorized}, ownerUploadCases{}, fakeQuota).ServeHTTP(resUnauth, reqQuota)
	if resUnauth.Code != http.StatusUnauthorized || resUnauth.Header().Get("WWW-Authenticate") != "Bearer" {
		t.Fatalf("expected 401 with WWW-Authenticate: Bearer, got %d (headers: %v)", resUnauth.Code, resUnauth.Header())
	}

	// 6. Auth forbidden returns 404
	resForbidden := httptest.NewRecorder()
	NewOwnerCasesHandlerWithUpload(ownerUploadAuthWithErr{err: identity.ErrForbidden}, ownerUploadCases{}, fakeQuota).ServeHTTP(resForbidden, reqQuota)
	if resForbidden.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for forbidden auth, got %d", resForbidden.Code)
	}

	// 7. Invalid UUID in upload path returns 404
	reqBadID := ownerUploadRequest(t, "/api/v1/document-verification/cases/not-a-valid-uuid/files", `"3"`, []byte("content"))
	resBadID := httptest.NewRecorder()
	NewOwnerCasesHandlerWithUpload(ownerUploadAuth{}, ownerUploadCases{}, fakeQuota).ServeHTTP(resBadID, reqBadID)
	if resBadID.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for invalid case ID, got %d", resBadID.Code)
	}

	// 8. Missing file part (only role part sent) returns 400
	var roleOnlyBody bytes.Buffer
	mwRoleOnly := multipart.NewWriter(&roleOnlyBody)
	roleOnlyPart, err := mwRoleOnly.CreateFormField("evidence_role")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = roleOnlyPart.Write([]byte(string(documentverification.EvidenceRoleIdentityFront)))
	_ = mwRoleOnly.Close()
	reqNoFile := httptest.NewRequest(http.MethodPost, path, &roleOnlyBody)
	reqNoFile.Header.Set("Authorization", "Bearer synthetic-session")
	reqNoFile.Header.Set("Content-Type", mwRoleOnly.FormDataContentType())
	reqNoFile.Header.Set("If-Match", `"3"`)
	resNoFile := httptest.NewRecorder()
	NewOwnerCasesHandlerWithUpload(ownerUploadAuth{}, ownerUploadCases{}, fakeQuota).ServeHTTP(resNoFile, reqNoFile)
	if resNoFile.Code != http.StatusBadRequest || !bytes.Contains(resNoFile.Body.Bytes(), []byte("file part is required")) {
		t.Fatalf("expected 400 file part is required, got %d (body: %s)", resNoFile.Code, resNoFile.Body.String())
	}

	// 9. Method not allowed on files (GET) returns 405 with Allow: POST
	reqGetFiles := httptest.NewRequest(http.MethodGet, path, nil)
	reqGetFiles.Header.Set("Authorization", "Bearer synthetic-session")
	resGetFiles := httptest.NewRecorder()
	NewOwnerCasesHandlerWithUpload(ownerUploadAuth{}, ownerUploadCases{}, fakeQuota).ServeHTTP(resGetFiles, reqGetFiles)
	if resGetFiles.Code != http.StatusMethodNotAllowed || resGetFiles.Header().Get("Allow") != http.MethodPost {
		t.Fatalf("expected 405 Method Not Allowed with Allow: POST, got %d (headers: %v)", resGetFiles.Code, resGetFiles.Header())
	}

	// 10. Method not allowed on submit (GET) returns 405 with Allow: POST
	reqGetSubmit := httptest.NewRequest(http.MethodGet, "/api/v1/document-verification/cases/"+ownerUploadCase+"/submit", nil)
	reqGetSubmit.Header.Set("Authorization", "Bearer synthetic-session")
	resGetSubmit := httptest.NewRecorder()
	NewOwnerCasesHandlerWithUpload(ownerUploadAuth{}, ownerUploadCases{}, fakeQuota).ServeHTTP(resGetSubmit, reqGetSubmit)
	if resGetSubmit.Code != http.StatusMethodNotAllowed || resGetSubmit.Header().Get("Allow") != http.MethodPost {
		t.Fatalf("expected 405 Method Not Allowed with Allow: POST, got %d (headers: %v)", resGetSubmit.Code, resGetSubmit.Header())
	}
}

func TestOwnerCaseUploadMultipartReaderErrorsAndTypeValidation(t *testing.T) {
	const path = "/api/v1/document-verification/cases/" + ownerUploadCase + "/files"

	// 1. Non-multipart content type returns 415
	reqNotMultipart := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
	reqNotMultipart.Header.Set("Authorization", "Bearer synthetic-session")
	reqNotMultipart.Header.Set("Content-Type", "application/json")
	reqNotMultipart.Header.Set("If-Match", `"3"`)
	resNotMultipart := httptest.NewRecorder()
	NewOwnerCasesHandlerWithUpload(ownerUploadAuth{}, ownerUploadCases{}, &ownerUploadFake{}).ServeHTTP(resNotMultipart, reqNotMultipart)
	if resNotMultipart.Code != http.StatusUnsupportedMediaType || !bytes.Contains(resNotMultipart.Body.Bytes(), []byte("multipart form required")) {
		t.Fatalf("expected 415 multipart form required, got %d (body: %s)", resNotMultipart.Code, resNotMultipart.Body.String())
	}

	// 2. Multipart without boundary returns 400 invalid multipart request
	reqNoBoundary := httptest.NewRequest(http.MethodPost, path, strings.NewReader("bad multipart"))
	reqNoBoundary.Header.Set("Authorization", "Bearer synthetic-session")
	reqNoBoundary.Header.Set("Content-Type", "multipart/form-data")
	reqNoBoundary.Header.Set("If-Match", `"3"`)
	resNoBoundary := httptest.NewRecorder()
	NewOwnerCasesHandlerWithUpload(ownerUploadAuth{}, ownerUploadCases{}, &ownerUploadFake{}).ServeHTTP(resNoBoundary, reqNoBoundary)
	if resNoBoundary.Code != http.StatusBadRequest || !bytes.Contains(resNoBoundary.Body.Bytes(), []byte("invalid multipart request")) {
		t.Fatalf("expected 400 invalid multipart request, got %d (body: %s)", resNoBoundary.Code, resNoBoundary.Body.String())
	}

	// 3. Evidence role exceeding 64 bytes returns 400 invalid evidence role
	var longRoleBody bytes.Buffer
	mwLongRole := multipart.NewWriter(&longRoleBody)
	rolePart, err := mwLongRole.CreateFormField("evidence_role")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = rolePart.Write([]byte(strings.Repeat("a", 65)))
	filePart, err := mwLongRole.CreateFormFile("file", "test.png")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = filePart.Write([]byte("valid content"))
	_ = mwLongRole.Close()
	reqLongRole := httptest.NewRequest(http.MethodPost, path, &longRoleBody)
	reqLongRole.Header.Set("Authorization", "Bearer synthetic-session")
	reqLongRole.Header.Set("Content-Type", mwLongRole.FormDataContentType())
	reqLongRole.Header.Set("If-Match", `"3"`)
	resLongRole := httptest.NewRecorder()
	NewOwnerCasesHandlerWithUpload(ownerUploadAuth{}, ownerUploadCases{}, &ownerUploadFake{}).ServeHTTP(resLongRole, reqLongRole)
	if resLongRole.Code != http.StatusBadRequest || !bytes.Contains(resLongRole.Body.Bytes(), []byte("invalid evidence role")) {
		t.Fatalf("expected 400 invalid evidence role for oversized role, got %d (body: %s)", resLongRole.Code, resLongRole.Body.String())
	}

	// 4. Evidence role with filename returns 400 invalid evidence role
	var roleWithFilenameBody bytes.Buffer
	mwRoleFile := multipart.NewWriter(&roleWithFilenameBody)
	rfPart, err := mwRoleFile.CreateFormFile("evidence_role", "role.txt")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = rfPart.Write([]byte(string(documentverification.EvidenceRoleIdentityFront)))
	_ = mwRoleFile.Close()
	reqRoleFile := httptest.NewRequest(http.MethodPost, path, &roleWithFilenameBody)
	reqRoleFile.Header.Set("Authorization", "Bearer synthetic-session")
	reqRoleFile.Header.Set("Content-Type", mwRoleFile.FormDataContentType())
	reqRoleFile.Header.Set("If-Match", `"3"`)
	resRoleFile := httptest.NewRecorder()
	NewOwnerCasesHandlerWithUpload(ownerUploadAuth{}, ownerUploadCases{}, &ownerUploadFake{}).ServeHTTP(resRoleFile, reqRoleFile)
	if resRoleFile.Code != http.StatusBadRequest || !bytes.Contains(resRoleFile.Body.Bytes(), []byte("invalid evidence role")) {
		t.Fatalf("expected 400 invalid evidence role when filename present, got %d (body: %s)", resRoleFile.Code, resRoleFile.Body.String())
	}

	// 5. File part with empty filename returns 400 invalid file part
	var emptyFilenameBody bytes.Buffer
	mwEmptyFilename := multipart.NewWriter(&emptyFilenameBody)
	ffPart, err := mwEmptyFilename.CreateFormField("file")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = ffPart.Write([]byte("some content"))
	_ = mwEmptyFilename.Close()
	reqEmptyFilename := httptest.NewRequest(http.MethodPost, path, &emptyFilenameBody)
	reqEmptyFilename.Header.Set("Authorization", "Bearer synthetic-session")
	reqEmptyFilename.Header.Set("Content-Type", mwEmptyFilename.FormDataContentType())
	reqEmptyFilename.Header.Set("If-Match", `"3"`)
	resEmptyFilename := httptest.NewRecorder()
	NewOwnerCasesHandlerWithUpload(ownerUploadAuth{}, ownerUploadCases{}, &ownerUploadFake{}).ServeHTTP(resEmptyFilename, reqEmptyFilename)
	if resEmptyFilename.Code != http.StatusBadRequest || !bytes.Contains(resEmptyFilename.Body.Bytes(), []byte("invalid file part")) {
		t.Fatalf("expected 400 invalid file part for empty filename, got %d (body: %s)", resEmptyFilename.Code, resEmptyFilename.Body.String())
	}

	// 6. Upload service non-quota error propagates through ownerError (e.g. ErrConflict -> 409)
	fakeErr := &ownerUploadFake{err: documentverification.ErrConflict}
	reqErr := ownerUploadRequest(t, path, `"3"`, []byte("synthetic-payload"))
	resErr := httptest.NewRecorder()
	NewOwnerCasesHandlerWithUpload(ownerUploadAuth{}, ownerUploadCases{}, fakeErr).ServeHTTP(resErr, reqErr)
	if resErr.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict from upload service error, got %d (body: %s)", resErr.Code, resErr.Body.String())
	}

	// 7. Direct uploadFile with invalid UUID returns 404
	h := &OwnerCasesHandler{
		auth:    ownerUploadAuth{},
		service: ownerUploadCases{},
		upload:  &ownerUploadFake{},
	}
	reqBadID := httptest.NewRequest(http.MethodPost, "/upload", nil)
	wBadID := httptest.NewRecorder()
	h.uploadFile(wBadID, reqBadID, "actor", "not-a-uuid")
	if wBadID.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for invalid UUID in uploadFile, got %d", wBadID.Code)
	}

	// 8. writeOwnerJSON returns 503 on unencodable value
	wUnencodable := httptest.NewRecorder()
	writeOwnerJSON(wUnencodable, http.StatusOK, make(chan int))
	if wUnencodable.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 for unencodable JSON, got %d", wUnencodable.Code)
	}
}
