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

const identityOwnerListActorID = "88888888-8888-4888-8888-888888888888"
const identityOwnerListCaseID = "99999999-9999-4999-8999-999999999999"
const identityOwnerListDeclarationID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"

type identityOwnerListAuth struct {
	mu       sync.Mutex
	calls    int
	secret   identity.Secret
	role     identity.Role
	activity identity.ActivityKind
}

func (a *identityOwnerListAuth) Authorize(_ context.Context, secret identity.Secret, role identity.Role, activity identity.ActivityKind) (identity.Principal, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.calls++
	a.secret, a.role, a.activity = secret, role, activity
	return identity.Principal{AccountID: identityOwnerListActorID}, nil
}

type identityOwnerListRepository struct {
	mu                          sync.Mutex
	snapshot                    dv.Snapshot
	listCalls                   int
	listActor                   string
	loads, creates, transitions int
}

func (r *identityOwnerListRepository) CreateCase(context.Context, string, dv.Purpose, string, func(dv.Authorization) (dv.Snapshot, error)) (dv.Snapshot, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.creates++
	return dv.Snapshot{}, dv.ErrInvalid
}
func (r *identityOwnerListRepository) LoadOwnCase(context.Context, string, string) (dv.Snapshot, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.loads++
	return dv.Snapshot{}, dv.ErrNotFound
}
func (r *identityOwnerListRepository) ListOwnCases(ctx context.Context, actor string) ([]dv.Snapshot, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.listCalls++
	r.listActor = actor
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if actor != identityOwnerListActorID {
		return nil, dv.ErrNotFound
	}
	return []dv.Snapshot{r.snapshot}, nil
}
func (r *identityOwnerListRepository) Transition(context.Context, string, dv.Purpose, string, uint64, dv.TransitionAudit, func(*dv.Snapshot, dv.Authorization) error) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.transitions++
	return dv.ErrInvalid
}

func TestNextBFFListsOwnIdentityCasesThroughRegisteredGoRoute(t *testing.T) {
	frontendRoot := strings.TrimSpace(os.Getenv("ESPACIGO_FRONTEND_ROOT"))
	if frontendRoot == "" {
		t.Skip("set ESPACIGO_FRONTEND_ROOT to authorized local Next checkout")
	}
	frontendRoot, err := filepath.Abs(frontendRoot)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(frontendRoot, "tooling", "identity-owner-list-bff-go-integration.mjs")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(frontendRoot, "node_modules", "typescript")); err != nil {
		t.Fatal(err)
	}

	repo := &identityOwnerListRepository{snapshot: dv.Snapshot{
		ID: identityOwnerListCaseID, SubjectID: identityOwnerListActorID, Purpose: dv.PurposeIdentity,
		State: dv.CaseNeedsEvidence, Revision: 8, IdentityDeclarationRevisionID: identityOwnerListDeclarationID,
		CaptureConsentVersion: dv.IdentityCaptureConsentVersion, CaptureConsentedAt: time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC),
		DocumentFormatDeclared: dv.DocumentFormatNew, OwnerDecisionReason: dv.ReasonEvidenceIncomplete,
		DocumentExpiresAt: time.Date(2030, 1, 2, 0, 0, 0, 0, time.UTC), ReviewerID: "77777777-7777-4777-8777-777777777777",
		Files: []dv.File{{ID: "11111111-1111-4111-8111-111111111111", EvidenceRole: dv.EvidenceRoleIdentityFront, State: dv.FileClean, Size: 100, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{1}}},
	}}
	service, err := dv.NewService(repo)
	if err != nil {
		t.Fatal(err)
	}
	auth := &identityOwnerListAuth{}
	mux := http.NewServeMux()
	RegisterOwnerCasesRoutes(mux, NewOwnerCasesHandler(auth, service))
	var mu sync.Mutex
	requests := 0
	var method, requestPath, authorization, cookie, contentType, body string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, readErr := io.ReadAll(r.Body)
		if readErr != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		_ = r.Body.Close()
		mu.Lock()
		requests++
		method, requestPath, authorization, cookie, contentType, body = r.Method, r.URL.Path, r.Header.Get("Authorization"), r.Header.Get("Cookie"), r.Header.Get("Content-Type"), string(b)
		mu.Unlock()
		r.Body = io.NopCloser(strings.NewReader(string(b)))
		mux.ServeHTTP(w, r)
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", "--test", "tooling/identity-owner-list-bff-go-integration.mjs")
	cmd.Dir = frontendRoot
	cmd.Env = identityBFFChildEnvironment(server.URL + "/api/v1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("owner list Next-to-Go characterization failed: %v\n%s", err, output)
	}
	t.Logf("Node output:\n%s", output)
	mu.Lock()
	count := requests
	gotMethod, gotPath, gotAuth, gotCookie, gotType, gotBody := method, requestPath, authorization, cookie, contentType, body
	mu.Unlock()
	if count != 1 || gotMethod != http.MethodGet || gotPath != ownerCasesRoute || gotAuth != "Bearer synthetic-session" || gotCookie != "" || gotType != "" || gotBody != "" {
		t.Fatalf("request count/method/path/auth/cookie/type/body=%d %q %q %q %q %q %q", count, gotMethod, gotPath, gotAuth, gotCookie, gotType, gotBody)
	}
	auth.mu.Lock()
	authOK := auth.calls == 1 && auth.secret == identity.Secret("synthetic-session") && auth.role == "" && auth.activity == identity.UserOperation
	auth.mu.Unlock()
	if !authOK {
		t.Fatal("unexpected synthetic authorization call")
	}
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if repo.listCalls != 1 || repo.listActor != identityOwnerListActorID || repo.loads != 0 || repo.creates != 0 || repo.transitions != 0 || repo.snapshot.ID != identityOwnerListCaseID {
		t.Fatalf("repository lists/actor/loads/creates/transitions=%d/%q/%d/%d/%d", repo.listCalls, repo.listActor, repo.loads, repo.creates, repo.transitions)
	}
}
