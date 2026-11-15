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

const reviewerQueueBFFCaseID = "99999999-9999-4999-8999-999999999999"
const reviewerQueueBFFActorID = "66666666-6666-4666-8666-666666666666"
const reviewerQueueBFFSubjectID = "11111111-1111-4111-8111-111111111111"
const reviewerQueueBFFDeclarationRevisionID = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"

type reviewerQueueBFFAuth struct {
	calls int
	token identity.Secret
	role  identity.Role
	kind  identity.ActivityKind
}

func (a *reviewerQueueBFFAuth) Authorize(_ context.Context, token identity.Secret, role identity.Role, kind identity.ActivityKind) (identity.Principal, error) {
	a.calls++
	a.token, a.role, a.kind = token, role, kind
	return identity.Principal{AccountID: reviewerQueueBFFActorID}, nil
}

type reviewerQueueBFFRepository struct {
	listed []dv.Snapshot
	actor  string
	calls  int
}

func (*reviewerQueueBFFRepository) CreateCase(context.Context, string, dv.Purpose, string, func(dv.Authorization) (dv.Snapshot, error)) (dv.Snapshot, error) {
	return dv.Snapshot{}, dv.ErrInvalid
}
func (*reviewerQueueBFFRepository) Transition(context.Context, string, dv.Purpose, string, uint64, dv.TransitionAudit, func(*dv.Snapshot, dv.Authorization) error) error {
	return dv.ErrCapability
}
func (r *reviewerQueueBFFRepository) ListReviewerCases(ctx context.Context, actor string) ([]dv.Snapshot, error) {
	r.calls++
	r.actor = actor
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return r.listed, nil
}
func (*reviewerQueueBFFRepository) LoadReviewerCase(context.Context, string, string) (dv.Snapshot, error) {
	return dv.Snapshot{}, dv.ErrNotFound
}
func (*reviewerQueueBFFRepository) WithReviewerCaseForRead(context.Context, string, string, func(context.Context, dv.Snapshot, dv.Authorization) error) error {
	return dv.ErrNotFound
}

type reviewerQueueBFFDocumentStore struct{}

func (reviewerQueueBFFDocumentStore) ReadIdentityDocument(context.Context, string, string, string) ([]byte, error) {
	return nil, dv.ErrNotFound
}
func (reviewerQueueBFFDocumentStore) ReadSpaceDocument(context.Context, string, string, string, string) ([]byte, error) {
	return nil, dv.ErrNotFound
}

func TestNextBFFGetsReviewerQueueFromRegisteredGoHandler(t *testing.T) {
	frontendRoot := strings.TrimSpace(os.Getenv("ESPACIGO_FRONTEND_ROOT"))
	if frontendRoot == "" {
		t.Skip("set ESPACIGO_FRONTEND_ROOT to the authorized local Next checkout")
	}
	frontendRoot, err := filepath.Abs(frontendRoot)
	if err != nil {
		t.Fatal("resolve frontend checkout path")
	}
	script := filepath.Join(frontendRoot, "tooling", "reviewer-queue-bff-go-integration.mjs")
	if _, err := os.Stat(script); err != nil {
		t.Fatalf("reviewer queue integration script unavailable: %v", err)
	}
	if _, err := os.Stat(filepath.Join(frontendRoot, "node_modules", "typescript")); err != nil {
		t.Fatalf("frontend TypeScript dependency unavailable: %v", err)
	}
	repo := &reviewerQueueBFFRepository{listed: []dv.Snapshot{
		{ID: reviewerQueueBFFCaseID, SubjectID: reviewerQueueBFFSubjectID, Purpose: dv.PurposeIdentity, State: dv.CaseSubmitted, Revision: 3, DocumentFormatDeclared: dv.DocumentFormatNew, IdentityDeclarationRevisionID: reviewerQueueBFFDeclarationRevisionID},
		{ID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", SubjectID: reviewerQueueBFFActorID, Purpose: dv.PurposeIdentity, State: dv.CaseSubmitted, Revision: 1},
		{ID: "cccccccc-cccc-4ccc-8ccc-cccccccccccc", SubjectID: reviewerQueueBFFSubjectID, Purpose: dv.PurposeIdentity, State: dv.CaseApproved, Revision: 2},
	}}
	domain, err := dv.NewService(reviewerQueueProjectionDomainRepo{})
	if err != nil {
		t.Fatal(err)
	}
	access, err := dv.NewReviewerAccessService(repo, domain, reviewerQueueBFFDocumentStore{})
	if err != nil {
		t.Fatal(err)
	}
	auth := &reviewerQueueBFFAuth{}
	mux := http.NewServeMux()
	RegisterReviewerCasesRoutes(mux, NewReviewerCasesHandler(auth, access))
	var mu sync.Mutex
	var calls int
	var method, requestPath, authorization, cookie, contentType, body string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, readErr := io.ReadAll(r.Body)
		if readErr != nil {
			http.Error(w, "synthetic test read failure", http.StatusBadRequest)
			return
		}
		_ = r.Body.Close()
		mu.Lock()
		calls++
		method, requestPath, authorization, cookie, contentType, body = r.Method, r.URL.Path, r.Header.Get("Authorization"), r.Header.Get("Cookie"), r.Header.Get("Content-Type"), string(b)
		mu.Unlock()
		mux.ServeHTTP(w, r)
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", "--test", "tooling/reviewer-queue-bff-go-integration.mjs")
	cmd.Dir = frontendRoot
	cmd.Env = identityBFFChildEnvironment(server.URL + "/api/v1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("reviewer queue Next-to-Go characterization failed: %v\n%s", err, output)
	}
	t.Logf("Node integration test output:\n%s", output)
	mu.Lock()
	defer mu.Unlock()
	if calls != 1 || method != http.MethodGet || requestPath != reviewerCasesRoute || authorization != "Bearer synthetic-reviewer-session" || cookie != "" || contentType != "" || body != "" {
		t.Fatalf("upstream request calls=%d method=%q path=%q auth=%q cookie=%q content-type=%q body=%q", calls, method, requestPath, authorization, cookie, contentType, body)
	}
	if auth.calls != 1 || auth.token != identity.Secret("synthetic-reviewer-session") || auth.role != "" || auth.kind != identity.UserOperation {
		t.Fatalf("auth calls=%d token=%q role=%q activity=%v", auth.calls, auth.token, auth.role, auth.kind)
	}
	if repo.calls != 1 || repo.actor != reviewerQueueBFFActorID {
		t.Fatalf("reader calls=%d actor=%q", repo.calls, repo.actor)
	}
}
