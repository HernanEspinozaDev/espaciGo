//go:build bffintegration

package http

import (
	"context"
	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
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
)

const identityOwnerSubmitOwnerID = "88888888-8888-4888-8888-888888888888"
const identityOwnerSubmitCaseID = "99999999-9999-4999-8999-999999999999"
const identityOwnerSubmitDeclarationID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"

type identityOwnerSubmitAuth struct {
	mu       sync.Mutex
	calls    int
	secret   identity.Secret
	role     identity.Role
	activity identity.ActivityKind
}

func (a *identityOwnerSubmitAuth) Authorize(_ context.Context, secret identity.Secret, role identity.Role, activity identity.ActivityKind) (identity.Principal, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.calls++
	a.secret, a.role, a.activity = secret, role, activity
	return identity.Principal{AccountID: identityOwnerSubmitOwnerID}, nil
}

type identityOwnerSubmitRepository struct {
	mu                 sync.Mutex
	snapshot           dv.Snapshot
	audit              dv.TransitionAudit
	loads, transitions int
}

func (r *identityOwnerSubmitRepository) CreateCase(context.Context, string, dv.Purpose, string, func(dv.Authorization) (dv.Snapshot, error)) (dv.Snapshot, error) {
	return dv.Snapshot{}, dv.ErrInvalid
}
func (r *identityOwnerSubmitRepository) LoadOwnCase(_ context.Context, actor, id string) (dv.Snapshot, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.loads++
	if actor != identityOwnerSubmitOwnerID || id != r.snapshot.ID {
		return dv.Snapshot{}, dv.ErrNotFound
	}
	return r.snapshot, nil
}
func (r *identityOwnerSubmitRepository) ListOwnCases(context.Context, string) ([]dv.Snapshot, error) {
	return nil, dv.ErrInvalid
}
func (r *identityOwnerSubmitRepository) Transition(ctx context.Context, actor string, purpose dv.Purpose, id string, revision uint64, audit dv.TransitionAudit, change func(*dv.Snapshot, dv.Authorization) error) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.transitions++
	if err := ctx.Err(); err != nil {
		return err
	}
	if actor != identityOwnerSubmitOwnerID || actor != r.snapshot.SubjectID || purpose != dv.PurposeIdentity || purpose != r.snapshot.Purpose || id != r.snapshot.ID || revision != r.snapshot.Revision {
		return dv.ErrConflict
	}
	candidate := r.snapshot
	authorization := dv.Authorization{
		ActorID:                       actor,
		SubjectID:                     actor,
		IdentityDeclarationRevisionID: r.snapshot.IdentityDeclarationRevisionID,
	}
	if err := change(&candidate, authorization); err != nil {
		return err
	}
	candidate.Revision++
	r.snapshot, r.audit = candidate, audit
	return nil
}
func TestNextBFFSubmitsOwnerIdentityCaseThroughActualGoService(t *testing.T) {
	frontendRoot := strings.TrimSpace(os.Getenv("ESPACIGO_FRONTEND_ROOT"))
	if frontendRoot == "" {
		t.Skip("set ESPACIGO_FRONTEND_ROOT to authorized local Next checkout")
	}
	frontendRoot, err := filepath.Abs(frontendRoot)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(frontendRoot, "tooling", "identity-owner-submit-bff-go-integration.mjs")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(frontendRoot, "node_modules", "typescript")); err != nil {
		t.Fatal(err)
	}
	scenario := os.Getenv("ESPACIGO_OWNER_SUBMIT_SCENARIO")
	if scenario == "" {
		scenario = "success"
	}
	if scenario != "success" && scenario != "stale" && scenario != "retry" {
		t.Fatalf("unsupported owner submit scenario %q", scenario)
	}
	initialRevision := uint64(7)
	if scenario == "stale" || scenario == "retry" {
		initialRevision = 8
	}
	initialState := dv.CaseDraft
	initialAudit := dv.TransitionAudit{}
	if scenario == "retry" {
		initialState = dv.CaseSubmitted
		initialAudit = dv.TransitionAudit{Action: "case_transitioned", ResultCode: string(dv.CaseSubmitted)}
	}
	repo := &identityOwnerSubmitRepository{snapshot: dv.Snapshot{
		ID:                            identityOwnerSubmitCaseID,
		SubjectID:                     identityOwnerSubmitOwnerID,
		Purpose:                       dv.PurposeIdentity,
		State:                         initialState,
		Revision:                      initialRevision,
		IdentityDeclarationRevisionID: identityOwnerSubmitDeclarationID,
		CaptureConsentVersion:         dv.IdentityCaptureConsentVersion,
		CaptureConsentedAt:            time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC),
		DocumentFormatDeclared:        dv.DocumentFormatNew,
		Files: []dv.File{
			{ID: "11111111-1111-4111-8111-111111111111", EvidenceRole: dv.EvidenceRoleIdentityFront, State: dv.FileClean, Size: 100, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{1}},
			{ID: "22222222-2222-4222-8222-222222222222", EvidenceRole: dv.EvidenceRoleIdentityBack, State: dv.FileClean, Size: 100, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{2}},
			{ID: "33333333-3333-4333-8333-333333333333", EvidenceRole: dv.EvidenceRoleIdentitySelfie, State: dv.FileClean, Size: 100, MIME: "image/jpeg", Pages: 1, ContentSHA256: [32]byte{3}},
		},
	}, audit: initialAudit}
	service, err := dv.NewService(repo)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	authenticator := &identityOwnerSubmitAuth{}
	RegisterOwnerCasesRoutes(mux, NewOwnerCasesHandler(authenticator, service))
	var mu sync.Mutex
	requestCount := 0
	var method, requestPath, auth, cookie, match, contentType, body string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, e := io.ReadAll(r.Body)
		if e != nil {
			http.Error(w, "bad request", 400)
			return
		}
		_ = r.Body.Close()
		mu.Lock()
		requestCount++
		method, requestPath, auth, cookie, match, contentType, body = r.Method, r.URL.Path, r.Header.Get("Authorization"), r.Header.Get("Cookie"), r.Header.Get("If-Match"), r.Header.Get("Content-Type"), string(b)
		mu.Unlock()
		r.Body = io.NopCloser(strings.NewReader(string(b)))
		mux.ServeHTTP(w, r)
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", "--test", "tooling/identity-owner-submit-bff-go-integration.mjs")
	cmd.Dir = frontendRoot
	cmd.Env = identityBFFChildEnvironment(server.URL + "/api/v1")
	cmd.Env = append(cmd.Env, "ESPACIGO_OWNER_SUBMIT_SCENARIO="+scenario)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("owner submit Next-to-Go characterization failed: %v\n%s", err, output)
	}
	t.Logf("Node integration output:\n%s", output)
	mu.Lock()
	gotRequestCount := requestCount
	gotMethod, gotPath, gotAuth, gotCookie, gotMatch, gotType, gotBody := method, requestPath, auth, cookie, match, contentType, body
	mu.Unlock()
	if gotRequestCount != 1 || gotMethod != http.MethodPost || gotPath != ownerCasesRoute+"/"+identityOwnerSubmitCaseID+"/submit" || gotAuth != "Bearer synthetic-session" || gotCookie != "" || gotMatch != `"7"` || gotType != "" || gotBody != "" {
		t.Fatalf("Go requests=%d method/path/auth/cookie/If-Match/type/body=%q %q %q %q %q %q %q", gotRequestCount, gotMethod, gotPath, gotAuth, gotCookie, gotMatch, gotType, gotBody)
	}
	authenticator.mu.Lock()
	if authenticator.calls != 1 || authenticator.secret != identity.Secret("synthetic-session") || authenticator.role != "" || authenticator.activity != identity.UserOperation {
		t.Fatalf("auth calls=%d secret=%q role=%q activity=%d", authenticator.calls, authenticator.secret, authenticator.role, authenticator.activity)
	}
	authenticator.mu.Unlock()
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if scenario == "success" {
		if repo.snapshot.State != dv.CaseSubmitted || repo.snapshot.Revision != 8 || repo.audit.Action != "case_transitioned" || repo.audit.ResultCode != string(dv.CaseSubmitted) || repo.loads != 2 || repo.transitions != 1 {
			t.Fatalf("snapshot=%+v audit=%+v loads=%d transitions=%d", repo.snapshot, repo.audit, repo.loads, repo.transitions)
		}
		return
	}
	if scenario == "retry" {
		if repo.snapshot.State != dv.CaseSubmitted || repo.snapshot.Revision != 8 || repo.audit != initialAudit || repo.loads != 1 || repo.transitions != 0 {
			t.Fatalf("immediate-successor replay mutated repository: snapshot=%+v audit=%+v loads=%d transitions=%d", repo.snapshot, repo.audit, repo.loads, repo.transitions)
		}
		return
	}
	if repo.snapshot.State != dv.CaseDraft || repo.snapshot.Revision != 8 || repo.audit != (dv.TransitionAudit{}) || repo.loads != 1 || repo.transitions != 0 {
		t.Fatalf("stale conflict mutated repository: snapshot=%+v audit=%+v loads=%d transitions=%d", repo.snapshot, repo.audit, repo.loads, repo.transitions)
	}
}
