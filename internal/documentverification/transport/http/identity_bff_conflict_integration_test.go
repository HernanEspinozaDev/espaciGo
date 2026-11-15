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

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
)

func TestNextBFFSanitizesStaleReviewerIdentityDecisionConflict(t *testing.T) {
	frontendRoot := strings.TrimSpace(os.Getenv("ESPACIGO_FRONTEND_ROOT"))
	if frontendRoot == "" {
		t.Skip("set ESPACIGO_FRONTEND_ROOT to the authorized local Next checkout")
	}
	frontendRoot, err := filepath.Abs(frontendRoot)
	if err != nil {
		t.Fatal("resolve frontend checkout path")
	}
	integrationScript := filepath.Join(frontendRoot, "tooling", "reviewer-decision-conflict-bff-go-integration.mjs")
	if _, err := os.Stat(integrationScript); err != nil {
		t.Fatalf("reviewer conflict integration script unavailable: %v", err)
	}
	if _, err := os.Stat(filepath.Join(frontendRoot, "node_modules", "typescript")); err != nil {
		t.Fatalf("frontend TypeScript dependency unavailable: %v", err)
	}

	const ownerID = "88888888-8888-4888-8888-888888888888"
	now := time.Date(2026, 11, 12, 17, 0, 0, 0, time.UTC)
	repo := &revokeLifecycleRepository{
		snapshot: dv.Snapshot{
			ID: identityBFFIntegrationCaseID, SubjectID: ownerID, Purpose: dv.PurposeIdentity,
			State: dv.CaseNeedsEvidence, Revision: 8,
			IdentityDeclarationRevisionID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
			CaptureConsentVersion:         dv.IdentityCaptureConsentVersion,
			CaptureConsentedAt:            time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC),
			DocumentFormatDeclared:        dv.DocumentFormatNew,
			Files: []dv.File{
				{ID: "11111111-1111-4111-8111-111111111111", EvidenceRole: dv.EvidenceRoleIdentityFront, State: dv.FileClean, Size: 100, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{1}},
				{ID: "22222222-2222-4222-8222-222222222222", EvidenceRole: dv.EvidenceRoleIdentityBack, State: dv.FileClean, Size: 100, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{2}},
				{ID: "33333333-3333-4333-8333-333333333333", EvidenceRole: dv.EvidenceRoleIdentitySelfie, State: dv.FileClean, Size: 100, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{3}},
			},
		},
		authorization: dv.Authorization{ActorID: identityBFFIntegrationActorID, ReviewerGrants: []dv.ReviewerGrant{{ReviewerID: identityBFFIntegrationActorID, Capability: dv.CapabilityIdentity, ScopeID: ownerID}}},
	}
	cases, err := dv.NewService(repo)
	if err != nil {
		t.Fatal(err)
	}
	cases.SetClock(func() time.Time { return now })
	access, err := dv.NewReviewerAccessService(repo, cases, revokeLifecycleDocumentStore{})
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	RegisterReviewerCasesRoutes(mux, NewReviewerCasesHandler(reviewerAuth{}, access))

	var authObserved, cookieObserved atomic.Bool
	var mu sync.Mutex
	var method, requestPath, contentType, requestBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "Bearer synthetic-session" {
			authObserved.Store(true)
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
		mu.Lock()
		method, requestPath, contentType, requestBody = r.Method, r.URL.Path, r.Header.Get("Content-Type"), string(body)
		mu.Unlock()
		mux.ServeHTTP(w, r)
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", "--test", "tooling/reviewer-decision-conflict-bff-go-integration.mjs")
	cmd.Dir = frontendRoot
	cmd.Env = identityBFFChildEnvironment(server.URL + "/api/v1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("reviewer conflict Next-to-Go characterization failed: %v\n%s", err, output)
	}
	t.Logf("Node integration test output:\n%s", output)

	mu.Lock()
	gotMethod, gotPath, gotType, gotBody := method, requestPath, contentType, requestBody
	mu.Unlock()
	if !authObserved.Load() || cookieObserved.Load() {
		t.Fatalf("Go request authorization observed=%t cookie observed=%t", authObserved.Load(), cookieObserved.Load())
	}
	if gotMethod != http.MethodPost || gotPath != reviewerCasesRoute+"/"+identityBFFIntegrationCaseID+"/review" || gotType != "application/json" {
		t.Fatalf("Go received method/path/content-type %q %q %q", gotMethod, gotPath, gotType)
	}
	if gotBody != identityBFFReviewerDecisionPayload {
		t.Fatalf("Go received unexpected reviewer request body %q", gotBody)
	}
	if repo.snapshot.State != dv.CaseNeedsEvidence || repo.snapshot.Revision != 8 || repo.snapshot.ReviewerID != "" || repo.audit.Action != "" {
		t.Fatalf("conflict changed fake snapshot/audit: snapshot=%+v audit=%+v", repo.snapshot, repo.audit)
	}
	if repo.transitionCalls != 1 || repo.transitionActor != identityBFFIntegrationActorID || repo.reviewerLoadCalls != 1 || repo.loadActor != identityBFFIntegrationActorID {
		t.Fatalf("transition=%d actor=%q loads=%d load actor=%q; want one attempt and only initial load", repo.transitionCalls, repo.transitionActor, repo.reviewerLoadCalls, repo.loadActor)
	}
}
