//go:build bffintegration

package http

import (
	"context"
	"crypto/sha256"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
)

type identityOwnerUploadInspector struct {
	mu      sync.Mutex
	calls   int
	purpose dv.Purpose
	id      string
	content []byte
}

func (i *identityOwnerUploadInspector) Inspect(p dv.Purpose, id string, raw []byte) (dv.File, []byte, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.calls++
	i.purpose = p
	i.id = id
	i.content = append([]byte(nil), raw...)
	normalized := append([]byte(nil), raw...)
	return dv.File{ID: id, State: dv.FilePending, Size: int64(len(normalized)), MIME: "image/png", Pages: 1, ContentSHA256: sha256.Sum256(normalized)}, normalized, nil
}

type identityOwnerUploadStore struct {
	mu                                             sync.Mutex
	puts, spacePuts, identityDeletes, spaceDeletes int
	subject, caseID, fileID                        string
	content                                        []byte
}

func (s *identityOwnerUploadStore) PutIdentityDocument(_ context.Context, subject, caseID, fileID string, content []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.puts++
	s.subject = subject
	s.caseID = caseID
	s.fileID = fileID
	s.content = append([]byte(nil), content...)
	return nil
}
func (s *identityOwnerUploadStore) PutSpaceDocument(context.Context, string, string, string, string, []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.spacePuts++
	return nil
}
func (s *identityOwnerUploadStore) DeleteIdentityDocument(context.Context, string, string, string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.identityDeletes++
	return nil
}
func (s *identityOwnerUploadStore) DeleteSpaceDocument(context.Context, string, string, string, string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.spaceDeletes++
	return nil
}

func TestNextBFFUploadsOwnerIdentityThroughActualUploadService(t *testing.T) {
	frontendRoot := strings.TrimSpace(os.Getenv("ESPACIGO_FRONTEND_ROOT"))
	if frontendRoot == "" {
		t.Skip("set ESPACIGO_FRONTEND_ROOT to the authorized local Next checkout")
	}
	var err error
	frontendRoot, err = filepath.Abs(frontendRoot)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(frontendRoot, "tooling", "identity-bff-go-integration.mjs")); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(frontendRoot, "node_modules", "typescript")); err != nil {
		t.Fatal(err)
	}

	repo := &identityOwnerSubmitRepository{snapshot: dv.Snapshot{
		ID: identityBFFIntegrationCaseID, SubjectID: identityOwnerSubmitOwnerID, Purpose: dv.PurposeIdentity,
		State: dv.CaseDraft, Revision: 3, IdentityDeclarationRevisionID: identityOwnerSubmitDeclarationID,
		CaptureConsentVersion: dv.IdentityCaptureConsentVersion, CaptureConsentedAt: time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC),
		DocumentFormatDeclared: dv.DocumentFormatNew,
	}}
	cases, err := dv.NewService(repo)
	if err != nil {
		t.Fatal(err)
	}
	inspector := &identityOwnerUploadInspector{}
	store := &identityOwnerUploadStore{}
	uploader, err := dv.NewUploadService(cases, inspector, store)
	if err != nil {
		t.Fatal(err)
	}
	auth := &identityOwnerSubmitAuth{}
	mux := http.NewServeMux()
	RegisterOwnerCasesRoutes(mux, NewOwnerCasesHandlerWithUpload(auth, cases, uploader))
	var requestMu sync.Mutex
	requests := 0
	var method, path, authorization, cookie, ifMatch, contentType string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestMu.Lock()
		requests++
		method, path, authorization, cookie, ifMatch, contentType = r.Method, r.URL.Path, r.Header.Get("Authorization"), r.Header.Get("Cookie"), r.Header.Get("If-Match"), r.Header.Get("Content-Type")
		requestMu.Unlock()
		mux.ServeHTTP(w, r)
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", "--test", "tooling/identity-bff-go-integration.mjs")
	cmd.Dir = frontendRoot
	cmd.Env = identityBFFChildEnvironment(server.URL + "/api/v1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Next-to-Go upload characterization failed: %v\n%s", err, output)
	}
	t.Logf("Node integration test output:\n%s", output)
	requestMu.Lock()
	gotRequests, gotMethod, gotPath, gotAuth, gotCookie, gotMatch, gotType := requests, method, path, authorization, cookie, ifMatch, contentType
	requestMu.Unlock()
	if gotRequests != 1 || gotMethod != http.MethodPost || gotPath != ownerCasesRoute+"/"+identityBFFIntegrationCaseID+"/files" || gotAuth != "Bearer synthetic-session" || gotCookie != "" || gotMatch != `"3"` || !strings.HasPrefix(gotType, "multipart/form-data; boundary=") {
		t.Fatalf("request count/method/path/auth/cookie/If-Match/content-type=%d %q %q %q %q %q %q", gotRequests, gotMethod, gotPath, gotAuth, gotCookie, gotMatch, gotType)
	}
	auth.mu.Lock()
	if auth.calls != 1 || auth.secret != identity.Secret("synthetic-session") || auth.role != "" || auth.activity != identity.UserOperation {
		t.Fatalf("auth calls=%d secret=%q role=%q activity=%d", auth.calls, auth.secret, auth.role, auth.activity)
	}
	auth.mu.Unlock()
	repo.mu.Lock()
	if repo.snapshot.Revision != 4 || len(repo.snapshot.Files) != 1 || repo.snapshot.Files[0].EvidenceRole != dv.EvidenceRoleIdentityFront || repo.snapshot.Files[0].State != dv.FilePending || repo.snapshot.Files[0].ID == "" || repo.loads != 1 || repo.transitions != 1 {
		t.Fatalf("repository snapshot=%+v loads=%d transitions=%d", repo.snapshot, repo.loads, repo.transitions)
	}
	persistedFile := repo.snapshot.Files[0]
	repo.mu.Unlock()
	inspector.mu.Lock()
	if inspector.calls != 1 || inspector.purpose != dv.PurposeIdentity || inspector.id != persistedFile.ID || string(inspector.content) != identityBFFIntegrationPayload {
		t.Fatalf("inspector calls=%d purpose=%q id=%q bytes=%q", inspector.calls, inspector.purpose, inspector.id, inspector.content)
	}
	inspector.mu.Unlock()
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.puts != 1 || store.spacePuts != 0 || store.identityDeletes != 0 || store.spaceDeletes != 0 || store.subject != identityOwnerSubmitOwnerID || store.caseID != identityBFFIntegrationCaseID || store.fileID != persistedFile.ID || string(store.content) != identityBFFIntegrationPayload || persistedFile.Size != int64(len(store.content)) {
		t.Fatalf("store puts=%d spacePuts=%d deletes=%d/%d scope=%q/%q/%q bytes=%q", store.puts, store.spacePuts, store.identityDeletes, store.spaceDeletes, store.subject, store.caseID, store.fileID, store.content)
	}
}

type identityOwnerUploadCancelStore struct {
	base                      *identityOwnerUploadStore
	cancel                    context.CancelFunc
	stored                    bool
	deleteContextErr          error
	deleteHasDeadline         bool
	putSubject, putCase       string
	putFile                   string
	deleteSubject, deleteCase string
	deleteFile                string
}

func (s *identityOwnerUploadCancelStore) PutIdentityDocument(ctx context.Context, subject, caseID, fileID string, content []byte) error {
	if err := s.base.PutIdentityDocument(ctx, subject, caseID, fileID, content); err != nil {
		return err
	}
	s.putSubject, s.putCase, s.putFile = subject, caseID, fileID
	s.stored = true
	s.cancel()
	return nil
}

func (s *identityOwnerUploadCancelStore) PutSpaceDocument(ctx context.Context, ownerID, spaceID, caseID, fileID string, content []byte) error {
	return s.base.PutSpaceDocument(ctx, ownerID, spaceID, caseID, fileID, content)
}

func (s *identityOwnerUploadCancelStore) DeleteIdentityDocument(ctx context.Context, subject, caseID, fileID string) error {
	s.deleteContextErr = ctx.Err()
	_, s.deleteHasDeadline = ctx.Deadline()
	s.deleteSubject, s.deleteCase, s.deleteFile = subject, caseID, fileID
	if err := s.base.DeleteIdentityDocument(ctx, subject, caseID, fileID); err != nil {
		return err
	}
	s.stored = false
	return nil
}

func (s *identityOwnerUploadCancelStore) DeleteSpaceDocument(ctx context.Context, ownerID, spaceID, caseID, fileID string) error {
	return s.base.DeleteSpaceDocument(ctx, ownerID, spaceID, caseID, fileID)
}

func TestOwnerHTTPUploadCompensatesSuccessfulPutAfterRequestCancellation(t *testing.T) {
	repo := &identityOwnerSubmitRepository{snapshot: dv.Snapshot{
		ID: identityBFFIntegrationCaseID, SubjectID: identityOwnerSubmitOwnerID, Purpose: dv.PurposeIdentity,
		State: dv.CaseDraft, Revision: 3, IdentityDeclarationRevisionID: identityOwnerSubmitDeclarationID,
		CaptureConsentVersion: dv.IdentityCaptureConsentVersion, CaptureConsentedAt: time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC),
		DocumentFormatDeclared: dv.DocumentFormatNew,
	}}
	cases, err := dv.NewService(repo)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	store := &identityOwnerUploadCancelStore{base: &identityOwnerUploadStore{}, cancel: cancel}
	uploader, err := dv.NewUploadService(cases, &identityOwnerUploadInspector{}, store)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewOwnerCasesHandlerWithUpload(&identityOwnerSubmitAuth{}, cases, uploader)
	request := ownerUploadRequest(t, ownerCasesRoute+"/"+identityBFFIntegrationCaseID+"/files", `"3"`, []byte(identityBFFIntegrationPayload)).WithContext(ctx)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable || response.Body.String() != "temporarily unavailable\n" || response.Header().Get("ETag") != "" {
		t.Fatalf("status=%d etag=%q body=%q", response.Code, response.Header().Get("ETag"), response.Body.String())
	}
	if ctx.Err() != context.Canceled {
		t.Fatalf("request context error=%v, want cancellation", ctx.Err())
	}
	repo.mu.Lock()
	if repo.snapshot.Revision != 3 || len(repo.snapshot.Files) != 0 || repo.loads != 1 || repo.transitions != 0 {
		t.Fatalf("repository snapshot=%+v loads=%d transitions=%d", repo.snapshot, repo.loads, repo.transitions)
	}
	repo.mu.Unlock()
	store.base.mu.Lock()
	puts, identityDeletes := store.base.puts, store.base.identityDeletes
	spacePuts, spaceDeletes := store.base.spacePuts, store.base.spaceDeletes
	store.base.mu.Unlock()
	if puts != 1 || identityDeletes != 1 || spacePuts != 0 || spaceDeletes != 0 || store.stored {
		t.Fatalf("storage puts/deletes=%d/%d space=%d/%d stored=%v", puts, identityDeletes, spacePuts, spaceDeletes, store.stored)
	}
	if store.deleteContextErr != nil || !store.deleteHasDeadline || store.putSubject != store.deleteSubject || store.putCase != store.deleteCase || store.putFile != store.deleteFile || store.putSubject != identityOwnerSubmitOwnerID || store.putCase != identityBFFIntegrationCaseID || store.putFile == "" {
		t.Fatalf("cleanup ctx=%v deadline=%v put scope=%q/%q/%q delete scope=%q/%q/%q", store.deleteContextErr, store.deleteHasDeadline, store.putSubject, store.putCase, store.putFile, store.deleteSubject, store.deleteCase, store.deleteFile)
	}
}
