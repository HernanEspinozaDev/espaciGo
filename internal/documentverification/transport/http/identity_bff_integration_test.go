//go:build bffintegration

package http

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
)

const identityBFFIntegrationCaseID = "99999999-9999-4999-8999-999999999999"
const identityBFFIntegrationActorID = "66666666-6666-4666-8666-666666666666"
const identityBFFIntegrationPayload = "synthetic-identity"

type identityBFFIntegrationUpload struct {
	mu       sync.Mutex
	calls    int
	actor    string
	caseID   string
	revision uint64
	role     documentverification.EvidenceRole
	content  []byte
}

func (f *identityBFFIntegrationUpload) Upload(_ context.Context, actor, caseID string, revision uint64, role documentverification.EvidenceRole, content []byte) (documentverification.UploadReceipt, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.actor = actor
	f.caseID = caseID
	f.revision = revision
	f.role = role
	f.content = append([]byte(nil), content...)
	return documentverification.UploadReceipt{
		File: documentverification.File{
			ID:           "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
			EvidenceRole: documentverification.EvidenceRoleIdentityFront,
			State:        documentverification.FilePending,
			Size:         int64(len(content)),
			MIME:         "image/png",
			Pages:        1,
		},
		Revision: 4,
	}, nil
}

func (f *identityBFFIntegrationUpload) snapshot() (int, string, string, uint64, documentverification.EvidenceRole, []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls, f.actor, f.caseID, f.revision, f.role, append([]byte(nil), f.content...)
}

func TestNextBFFForwardsIdentityMultipartToGoOwnerUploadHandler(t *testing.T) {
	frontendRoot := strings.TrimSpace(os.Getenv("ESPACIGO_FRONTEND_ROOT"))
	if frontendRoot == "" {
		t.Skip("set ESPACIGO_FRONTEND_ROOT to the authorized local Next checkout")
	}
	frontendRoot, err := filepath.Abs(frontendRoot)
	if err != nil {
		t.Fatal("resolve frontend checkout path")
	}
	integrationScript := filepath.Join(frontendRoot, "tooling", "identity-bff-go-integration.mjs")
	if _, err := os.Stat(integrationScript); err != nil {
		t.Fatalf("integration script is unavailable in the frontend checkout: %v", err)
	}
	if _, err := os.Stat(filepath.Join(frontendRoot, "node_modules", "typescript")); err != nil {
		t.Fatalf("frontend TypeScript dependency is unavailable: %v", err)
	}

	upload := &identityBFFIntegrationUpload{}
	goHandler := NewOwnerCasesHandlerWithUpload(ownerUploadAuth{}, ownerUploadCases{}, upload)
	var authorizationObserved atomic.Bool
	var cookieObserved atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "Bearer synthetic-session" {
			authorizationObserved.Store(true)
		}
		if r.Header.Get("Cookie") != "" {
			cookieObserved.Store(true)
		}
		goHandler.ServeHTTP(w, r)
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", "--test", "tooling/identity-bff-go-integration.mjs")
	cmd.Dir = frontendRoot
	cmd.Env = identityBFFChildEnvironment(server.URL + "/api/v1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Next-to-Go characterization failed: %v\n%s", err, output)
	}
	t.Logf("Node integration test output:\n%s", output)

	if !authorizationObserved.Load() {
		t.Fatal("Go handler did not receive the expected synthetic Bearer authorization")
	}
	if cookieObserved.Load() {
		t.Fatal("BFF forwarded a Cookie header to the Go handler")
	}
	calls, actor, caseID, revision, role, content := upload.snapshot()
	if calls != 1 || actor != identityBFFIntegrationActorID || caseID != identityBFFIntegrationCaseID || revision != 3 || role != documentverification.EvidenceRoleIdentityFront || string(content) != identityBFFIntegrationPayload {
		t.Fatalf("Go upload service received an unexpected call shape (calls=%d, revision=%d, role=%q, bytes=%d)", calls, revision, role, len(content))
	}
}

func identityBFFChildEnvironment(apiBaseURL string) []string {
	var env []string
	for _, name := range []string{"PATH", "HOME", "TMPDIR", "LANG", "LC_ALL"} {
		if value, ok := os.LookupEnv(name); ok {
			env = append(env, name+"="+value)
		}
	}
	return append(env, "ESPACIGO_TEST_API_BASE_URL="+apiBaseURL)
}

func TestNextBFFForwardsReviewerIdentityDecisionToGoHandler(t *testing.T) {
	frontendRoot := strings.TrimSpace(os.Getenv("ESPACIGO_FRONTEND_ROOT"))
	if frontendRoot == "" {
		t.Skip("set ESPACIGO_FRONTEND_ROOT to the authorized local Next checkout")
	}
	frontendRoot, err := filepath.Abs(frontendRoot)
	if err != nil {
		t.Fatal("resolve frontend checkout path")
	}
	integrationScript := filepath.Join(frontendRoot, "tooling", "reviewer-decision-bff-go-integration.mjs")
	if _, err := os.Stat(integrationScript); err != nil {
		t.Fatalf("reviewer integration script is unavailable in the frontend checkout: %v", err)
	}
	if _, err := os.Stat(filepath.Join(frontendRoot, "node_modules", "typescript")); err != nil {
		t.Fatalf("frontend TypeScript dependency is unavailable: %v", err)
	}

	const ownerID = "88888888-8888-4888-8888-888888888888"
	now := time.Date(2026, 11, 12, 17, 0, 0, 0, time.UTC)
	consentedAt := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	repo := &revokeLifecycleRepository{
		snapshot: documentverification.Snapshot{
			ID: identityBFFIntegrationCaseID, SubjectID: ownerID, Purpose: documentverification.PurposeIdentity,
			State: documentverification.CaseSubmitted, Revision: 7,
			IdentityDeclarationRevisionID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
			CaptureConsentVersion:         documentverification.IdentityCaptureConsentVersion,
			CaptureConsentedAt:            consentedAt,
			DocumentFormatDeclared:        documentverification.DocumentFormatNew,
			Files: []documentverification.File{
				{ID: "11111111-1111-4111-8111-111111111111", EvidenceRole: documentverification.EvidenceRoleIdentityFront, State: documentverification.FileClean, Size: 100, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{1}},
				{ID: "22222222-2222-4222-8222-222222222222", EvidenceRole: documentverification.EvidenceRoleIdentityBack, State: documentverification.FileClean, Size: 100, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{2}},
				{ID: "33333333-3333-4333-8333-333333333333", EvidenceRole: documentverification.EvidenceRoleIdentitySelfie, State: documentverification.FileClean, Size: 100, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{3}},
			},
		},
		authorization: documentverification.Authorization{
			ActorID: identityBFFIntegrationActorID,
			ReviewerGrants: []documentverification.ReviewerGrant{{
				ReviewerID: identityBFFIntegrationActorID,
				Capability: documentverification.CapabilityIdentity,
				ScopeID:    ownerID,
			}},
		},
	}
	cases, err := documentverification.NewService(repo)
	if err != nil {
		t.Fatal(err)
	}
	cases.SetClock(func() time.Time { return now })
	access, err := documentverification.NewReviewerAccessService(repo, cases, revokeLifecycleDocumentStore{})
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	RegisterReviewerCasesRoutes(mux, NewReviewerCasesHandler(reviewerAuth{}, access))

	var authorizationObserved atomic.Bool
	var cookieObserved atomic.Bool
	var requestMu sync.Mutex
	var receivedMethod, receivedPath, receivedContentType, receivedBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "Bearer synthetic-session" {
			authorizationObserved.Store(true)
		}
		if r.Header.Get("Cookie") != "" {
			cookieObserved.Store(true)
		}
		body, readErr := io.ReadAll(r.Body)
		if readErr != nil {
			http.Error(w, "invalid test request", http.StatusBadRequest)
			return
		}
		_ = r.Body.Close()
		r.Body = io.NopCloser(strings.NewReader(string(body)))
		requestMu.Lock()
		receivedMethod, receivedPath = r.Method, r.URL.Path
		receivedContentType, receivedBody = r.Header.Get("Content-Type"), string(body)
		requestMu.Unlock()
		mux.ServeHTTP(w, r)
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", "--test", "tooling/reviewer-decision-bff-go-integration.mjs")
	cmd.Dir = frontendRoot
	cmd.Env = identityBFFChildEnvironment(server.URL + "/api/v1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("reviewer Next-to-Go characterization failed: %v\n%s", err, output)
	}
	t.Logf("Node integration test output:\n%s", output)

	requestMu.Lock()
	method, path, contentType, body := receivedMethod, receivedPath, receivedContentType, receivedBody
	requestMu.Unlock()
	if !authorizationObserved.Load() || cookieObserved.Load() {
		t.Fatalf("Go request authorization observed=%t cookie observed=%t", authorizationObserved.Load(), cookieObserved.Load())
	}
	if method != http.MethodPost || path != reviewerCasesRoute+"/"+identityBFFIntegrationCaseID+"/review" || contentType != "application/json" {
		t.Fatalf("Go received method/path/content-type %q %q %q", method, path, contentType)
	}
	if body != identityBFFReviewerDecisionPayload {
		t.Fatalf("Go received unexpected reviewer request body %q", body)
	}
	if repo.snapshot.State != documentverification.CaseNeedsEvidence || repo.snapshot.Revision != 8 || repo.snapshot.ReviewerID != identityBFFIntegrationActorID ||
		repo.snapshot.DocumentFormatObserved != documentverification.DocumentFormatNew || !repo.snapshot.DocumentExpiresAt.IsZero() {
		t.Fatalf("reviewer in-memory transition=%+v", repo.snapshot)
	}
	if repo.audit.Action != "review_decision" || repo.audit.ResultCode != string(documentverification.CaseNeedsEvidence) ||
		repo.audit.ReasonCode != documentverification.ReasonEvidenceIncomplete || repo.audit.DocumentFormatObserved != documentverification.DocumentFormatNew ||
		repo.audit.IdentityReviewChecklist == nil || !repo.audit.IdentityReviewChecklist.Complete() || repo.audit.IdentityReviewChecklist.AllPassed() {
		t.Fatalf("reviewer in-memory audit=%+v", repo.audit)
	}
	if repo.transitionCalls != 1 || repo.transitionActor != identityBFFIntegrationActorID || repo.reviewerLoadCalls != 2 || repo.loadActor != identityBFFIntegrationActorID {
		t.Fatalf("reviewer transition calls/actor=%d/%q, reads/actor=%d/%q", repo.transitionCalls, repo.transitionActor, repo.reviewerLoadCalls, repo.loadActor)
	}
}

const identityBFFReviewerDecisionPayload = `{"decision":"needs_evidence","reason_code":"antecedentes_incompletos","expected_revision":7,"document_format_observed":"new","identity_review_checklist":{"names_match":false,"rut_matches":true,"birth_date_and_majority_match":true,"portrait_matches_selfie":true,"evidence_legible":true,"document_unexpired":true,"issue_date_and_format_verified":true}}`
