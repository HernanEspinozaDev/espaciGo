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

const identityOwnerCreateActorID = "88888888-8888-4888-8888-888888888888"
const identityOwnerCreateDeclarationID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
const identityOwnerCreateBody = `{"purpose":"identity","document_format_declared":"new","capture_consent_given":true,"capture_consent_version":"identity-capture-local-v1"}`

type identityOwnerCreateAuth struct {
	mu       sync.Mutex
	calls    int
	secret   identity.Secret
	role     identity.Role
	activity identity.ActivityKind
}

func (a *identityOwnerCreateAuth) Authorize(_ context.Context, secret identity.Secret, role identity.Role, activity identity.ActivityKind) (identity.Principal, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.calls++
	a.secret, a.role, a.activity = secret, role, activity
	return identity.Principal{AccountID: identityOwnerCreateActorID}, nil
}

type identityOwnerCreateRepository struct {
	mu                        sync.Mutex
	snapshot                  dv.Snapshot
	creates                   int
	actor                     string
	purpose                   dv.Purpose
	spaceID                   string
	loads, lists, transitions int
}

func (r *identityOwnerCreateRepository) CreateCase(ctx context.Context, actor string, purpose dv.Purpose, spaceID string, create func(dv.Authorization) (dv.Snapshot, error)) (dv.Snapshot, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.creates++
	r.actor, r.purpose, r.spaceID = actor, purpose, spaceID
	if err := ctx.Err(); err != nil {
		return dv.Snapshot{}, err
	}
	x, err := create(dv.Authorization{ActorID: actor, SubjectID: actor, IdentityDeclarationRevisionID: identityOwnerCreateDeclarationID})
	if err != nil {
		return dv.Snapshot{}, err
	}
	x.Revision = 1
	r.snapshot = x
	return x, nil
}
func (r *identityOwnerCreateRepository) LoadOwnCase(context.Context, string, string) (dv.Snapshot, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.loads++
	return dv.Snapshot{}, dv.ErrNotFound
}
func (r *identityOwnerCreateRepository) ListOwnCases(context.Context, string) ([]dv.Snapshot, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lists++
	return nil, nil
}
func (r *identityOwnerCreateRepository) Transition(context.Context, string, dv.Purpose, string, uint64, dv.TransitionAudit, func(*dv.Snapshot, dv.Authorization) error) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.transitions++
	return dv.ErrInvalid
}

func TestNextBFFCreatesOwnerIdentityCaseThroughRegisteredGoRoute(t *testing.T) {
	frontendRoot := strings.TrimSpace(os.Getenv("ESPACIGO_FRONTEND_ROOT"))
	if frontendRoot == "" {
		t.Skip("set ESPACIGO_FRONTEND_ROOT to authorized local Next checkout")
	}
	frontendRoot, err := filepath.Abs(frontendRoot)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(frontendRoot, "tooling", "identity-owner-create-bff-go-integration.mjs")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(frontendRoot, "node_modules", "typescript")); err != nil {
		t.Fatal(err)
	}
	repo := &identityOwnerCreateRepository{}
	service, err := dv.NewService(repo)
	if err != nil {
		t.Fatal(err)
	}
	service.SetClock(func() time.Time { return time.Date(2026, 11, 12, 17, 0, 0, 0, time.UTC) })
	auth := &identityOwnerCreateAuth{}
	mux := http.NewServeMux()
	RegisterOwnerCasesRoutes(mux, NewOwnerCasesHandler(auth, service))
	var mu sync.Mutex
	requests := 0
	var method, requestPath, authorization, cookie, contentType, body string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, e := io.ReadAll(r.Body)
		if e != nil {
			http.Error(w, "bad request", 400)
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
	cmd := exec.CommandContext(ctx, "node", "--test", "tooling/identity-owner-create-bff-go-integration.mjs")
	cmd.Dir = frontendRoot
	cmd.Env = identityBFFChildEnvironment(server.URL + "/api/v1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("owner create Next-to-Go characterization failed: %v\n%s", err, output)
	}
	t.Logf("Node output:\n%s", output)
	mu.Lock()
	count := requests
	gotMethod, gotPath, gotAuth, gotCookie, gotType, gotBody := method, requestPath, authorization, cookie, contentType, body
	mu.Unlock()
	if count != 1 || gotMethod != http.MethodPost || gotPath != ownerCasesRoute || gotAuth != "Bearer synthetic-session" || gotCookie != "" || gotType != "application/json" || gotBody != identityOwnerCreateBody {
		t.Fatalf("request count/method/path/auth/cookie/type/body=%d %q %q %q %q %q %q", count, gotMethod, gotPath, gotAuth, gotCookie, gotType, gotBody)
	}
	auth.mu.Lock()
	authOK := auth.calls == 1 && auth.secret == "synthetic-session" && auth.role == "" && auth.activity == identity.UserOperation
	auth.mu.Unlock()
	if !authOK {
		t.Fatalf("unexpected authorization call: %+v", auth)
	}
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if repo.creates != 1 || repo.actor != identityOwnerCreateActorID || repo.purpose != dv.PurposeIdentity || repo.spaceID != "" || repo.loads != 0 || repo.lists != 0 || repo.transitions != 0 {
		t.Fatalf("repository create/scope/reads/transitions=%d/%q/%q/%q/%d/%d/%d", repo.creates, repo.actor, repo.purpose, repo.spaceID, repo.loads, repo.lists, repo.transitions)
	}
	if dv.ValidateID(repo.snapshot.ID) != nil || repo.snapshot.SubjectID != identityOwnerCreateActorID || repo.snapshot.Purpose != dv.PurposeIdentity || repo.snapshot.SpaceID != "" || repo.snapshot.Revision != 1 || repo.snapshot.State != dv.CaseDraft || repo.snapshot.IdentityDeclarationRevisionID != identityOwnerCreateDeclarationID || repo.snapshot.DocumentFormatDeclared != dv.DocumentFormatNew || repo.snapshot.CaptureConsentVersion != dv.IdentityCaptureConsentVersion || repo.snapshot.CaptureConsentedAt.IsZero() {
		t.Fatalf("unexpected stored snapshot: %+v", repo.snapshot)
	}
}
