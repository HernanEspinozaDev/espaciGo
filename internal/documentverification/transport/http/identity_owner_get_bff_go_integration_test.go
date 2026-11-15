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
	"testing"
	"time"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
)

func TestNextBFFGetsOwnIdentityCaseThroughRegisteredGoRoute(t *testing.T) {
	frontendRoot := strings.TrimSpace(os.Getenv("ESPACIGO_FRONTEND_ROOT"))
	if frontendRoot == "" {
		t.Skip("set ESPACIGO_FRONTEND_ROOT to authorized local Next checkout")
	}
	frontendRoot, err := filepath.Abs(frontendRoot)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(frontendRoot, "tooling", "identity-owner-get-bff-go-integration.mjs")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(frontendRoot, "node_modules", "typescript")); err != nil {
		t.Fatal(err)
	}

	repo := &identityOwnerSubmitRepository{snapshot: dv.Snapshot{
		ID: identityOwnerSubmitCaseID, SubjectID: identityOwnerSubmitOwnerID, Purpose: dv.PurposeIdentity,
		State: dv.CaseNeedsEvidence, Revision: 8, IdentityDeclarationRevisionID: identityOwnerSubmitDeclarationID,
		CaptureConsentVersion: dv.IdentityCaptureConsentVersion, CaptureConsentedAt: time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC),
		DocumentFormatDeclared: dv.DocumentFormatNew, OwnerDecisionReason: dv.ReasonEvidenceIncomplete,
		DocumentExpiresAt: time.Date(2030, 1, 2, 0, 0, 0, 0, time.UTC),
		ReviewerID:        "77777777-7777-4777-8777-777777777777",
		Files: []dv.File{
			{ID: "11111111-1111-4111-8111-111111111111", EvidenceRole: dv.EvidenceRoleIdentityFront, State: dv.FileClean, Size: 100, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{1}},
			{ID: "22222222-2222-4222-8222-222222222222", EvidenceRole: dv.EvidenceRoleIdentityBack, State: dv.FileClean, Size: 100, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{2}},
			{ID: "33333333-3333-4333-8333-333333333333", EvidenceRole: dv.EvidenceRoleIdentitySelfie, State: dv.FileClean, Size: 100, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{3}},
		},
	}}
	service, err := dv.NewService(repo)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	auth := &identityOwnerSubmitAuth{}
	RegisterOwnerCasesRoutes(mux, NewOwnerCasesHandler(auth, service))
	var captureMu sync.Mutex
	var requestCount int
	var method, path, authorization, cookie, contentType, body string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, e := io.ReadAll(r.Body)
		if e != nil {
			http.Error(w, "bad request", 400)
			return
		}
		_ = r.Body.Close()
		captureMu.Lock()
		requestCount++
		method, path, authorization, cookie, contentType, body = r.Method, r.URL.Path, r.Header.Get("Authorization"), r.Header.Get("Cookie"), r.Header.Get("Content-Type"), string(b)
		captureMu.Unlock()
		r.Body = io.NopCloser(strings.NewReader(string(b)))
		mux.ServeHTTP(w, r)
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", "--test", "tooling/identity-owner-get-bff-go-integration.mjs")
	cmd.Dir = frontendRoot
	cmd.Env = identityBFFChildEnvironment(server.URL + "/api/v1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("owner GET Next-to-Go characterization failed: %v\n%s", err, output)
	}
	t.Logf("Node output:\n%s", output)
	captureMu.Lock()
	gotRequestCount := requestCount
	gotMethod, gotPath, gotAuthorization, gotCookie, gotContentType, gotBody := method, path, authorization, cookie, contentType, body
	captureMu.Unlock()
	if gotRequestCount != 1 || gotMethod != http.MethodGet || gotPath != ownerCasesRoute+"/"+identityOwnerSubmitCaseID || gotAuthorization != "Bearer synthetic-session" || gotCookie != "" || gotContentType != "" || gotBody != "" {
		t.Fatalf("request count/method/path/auth/cookie/type/body=%d %q %q %q %q %q %q", gotRequestCount, gotMethod, gotPath, gotAuthorization, gotCookie, gotContentType, gotBody)
	}
	auth.mu.Lock()
	defer auth.mu.Unlock()
	if auth.calls != 1 || auth.secret != "synthetic-session" || auth.role != "" || auth.activity != identity.UserOperation {
		t.Fatalf("auth calls/secret/role/activity=%d %q %q %v", auth.calls, auth.secret, auth.role, auth.activity)
	}
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if repo.loads != 1 || repo.transitions != 0 || repo.snapshot.ID != identityOwnerSubmitCaseID {
		t.Fatalf("repository loads/transitions=%d/%d", repo.loads, repo.transitions)
	}
}
