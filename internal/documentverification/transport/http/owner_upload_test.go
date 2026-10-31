package http

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
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
	content       []byte
	receipt       documentverification.UploadReceipt
	err           error
	calls         int
}

func (f *ownerUploadFake) Upload(_ context.Context, actor, caseID string, revision uint64, content []byte) (documentverification.UploadReceipt, error) {
	f.calls++
	f.actor, f.caseID, f.revision = actor, caseID, revision
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
		File:     documentverification.File{ID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", State: documentverification.FilePending, Size: int64(len(content)), MIME: "image/png", Pages: 1},
		Revision: 4,
	}}
	handler := NewOwnerCasesHandlerWithUpload(ownerUploadAuth{}, ownerUploadCases{}, fake)
	request := ownerUploadRequest(t, "/api/v1/document-verification/cases/"+ownerUploadCase+"/files", `"3"`, content)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if fake.calls != 1 || fake.actor != ownerUploadActor || fake.caseID != ownerUploadCase || fake.revision != 3 || !bytes.Equal(fake.content, content) {
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
	for _, key := range []string{"id", "state", "size", "mime", "pages"} {
		if _, ok := file[key]; !ok {
			t.Errorf("uploaded file DTO missing key %q", key)
		}
	}
	if len(file) != 5 {
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
