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

const reviewerDetailBFFCaseID = "99999999-9999-4999-8999-999999999999"
const reviewerDetailBFFActorID = "66666666-6666-4666-8666-666666666666"
const reviewerDetailBFFSubjectID = "11111111-1111-4111-8111-111111111111"
const reviewerDetailBFFRevisionID = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"

type reviewerDetailBFFAuth struct {
	mu    sync.Mutex
	calls int
	token identity.Secret
	role  identity.Role
	kind  identity.ActivityKind
}

func (a *reviewerDetailBFFAuth) Authorize(_ context.Context, token identity.Secret, role identity.Role, kind identity.ActivityKind) (identity.Principal, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.calls++
	a.token = token
	a.role = role
	a.kind = kind
	return identity.Principal{AccountID: reviewerDetailBFFActorID}, nil
}

type reviewerDetailBFFRepo struct {
	mu            sync.Mutex
	snapshot      dv.Snapshot
	authorization dv.Authorization
	actors, cases []string
}

func (*reviewerDetailBFFRepo) CreateCase(context.Context, string, dv.Purpose, string, func(dv.Authorization) (dv.Snapshot, error)) (dv.Snapshot, error) {
	return dv.Snapshot{}, dv.ErrInvalid
}
func (*reviewerDetailBFFRepo) Transition(context.Context, string, dv.Purpose, string, uint64, dv.TransitionAudit, func(*dv.Snapshot, dv.Authorization) error) error {
	return dv.ErrCapability
}
func (*reviewerDetailBFFRepo) ListReviewerCases(context.Context, string) ([]dv.Snapshot, error) {
	return nil, dv.ErrNotFound
}
func (*reviewerDetailBFFRepo) LoadReviewerCase(context.Context, string, string) (dv.Snapshot, error) {
	return dv.Snapshot{}, dv.ErrNotFound
}
func (r *reviewerDetailBFFRepo) WithReviewerCaseForRead(ctx context.Context, actor, id string, read func(context.Context, dv.Snapshot, dv.Authorization) error) error {
	r.mu.Lock()
	r.actors = append(r.actors, actor)
	r.cases = append(r.cases, id)
	snap, auth := r.snapshot, r.authorization
	r.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	return read(ctx, snap, auth)
}

type reviewerDetailBFFStore struct {
	mu    sync.Mutex
	reads int
}

func (s *reviewerDetailBFFStore) ReadIdentityDocument(context.Context, string, string, string) ([]byte, error) {
	s.mu.Lock()
	s.reads++
	s.mu.Unlock()
	return nil, dv.ErrNotFound
}
func (s *reviewerDetailBFFStore) ReadSpaceDocument(context.Context, string, string, string, string) ([]byte, error) {
	s.mu.Lock()
	s.reads++
	s.mu.Unlock()
	return nil, dv.ErrNotFound
}

func TestNextBFFGetsReviewerDetailFromRegisteredGoHandler(t *testing.T) {
	root := strings.TrimSpace(os.Getenv("ESPACIGO_FRONTEND_ROOT"))
	if root == "" {
		t.Skip("set ESPACIGO_FRONTEND_ROOT to authorized local Next checkout")
	}
	root, err := filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(root, "tooling", "reviewer-detail-bff-go-integration.mjs")
	if _, err = os.Stat(script); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(root, "node_modules", "typescript")); err != nil {
		t.Fatal(err)
	}
	repo := &reviewerDetailBFFRepo{snapshot: dv.Snapshot{ID: reviewerDetailBFFCaseID, SubjectID: reviewerDetailBFFSubjectID, Purpose: dv.PurposeIdentity, State: dv.CaseInReview, Revision: 5, IdentityDeclarationRevisionID: reviewerDetailBFFRevisionID, DocumentFormatDeclared: dv.DocumentFormatNew}, authorization: dv.Authorization{ActorID: reviewerDetailBFFActorID, ReviewerGrants: []dv.ReviewerGrant{{ReviewerID: reviewerDetailBFFActorID, Capability: dv.CapabilityIdentity, ScopeID: reviewerDetailBFFSubjectID}}}}
	service, err := dv.NewService(repo)
	if err != nil {
		t.Fatal(err)
	}
	store := &reviewerDetailBFFStore{}
	var decMu sync.Mutex
	decCalls := 0
	decSubject, decRevision := "", ""
	reader := dv.ReviewerIdentityDeclarationReaderFunc(func(_ context.Context, subject, revision string) (dv.ReviewerIdentityDeclaration, error) {
		decMu.Lock()
		defer decMu.Unlock()
		decCalls++
		decSubject, decRevision = subject, revision
		return dv.ReviewerIdentityDeclaration{FirstName: "SyntheticFirst", LastName: "SyntheticLast", RUT: "11.111.111-1", DateOfBirth: "1980-01-02"}, nil
	})
	access, err := dv.NewReviewerAccessServiceWithIdentityDeclarationReader(repo, service, store, reader)
	if err != nil {
		t.Fatal(err)
	}
	auth := &reviewerDetailBFFAuth{}
	mux := http.NewServeMux()
	RegisterReviewerCasesRoutes(mux, NewReviewerCasesHandler(auth, access))
	var mu sync.Mutex
	calls := 0
	var method, requestPath, authorization, cookie, contentType, body string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, e := io.ReadAll(r.Body)
		if e != nil {
			http.Error(w, "synthetic test read failure", 400)
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
	cmd := exec.CommandContext(ctx, "node", "--test", "tooling/reviewer-detail-bff-go-integration.mjs")
	cmd.Dir = root
	cmd.Env = identityBFFChildEnvironment(server.URL + "/api/v1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("reviewer detail BFF→Go characterization failed: %v\n%s", err, out)
	}
	t.Logf("Node integration output:\n%s", out)
	mu.Lock()
	defer mu.Unlock()
	if calls != 1 || method != http.MethodGet || requestPath != reviewerCasesRoute+"/"+reviewerDetailBFFCaseID || authorization != "Bearer synthetic-reviewer-session" || cookie != "" || contentType != "" || body != "" {
		t.Fatalf("upstream calls=%d method=%q path=%q auth=%q cookie=%q content-type=%q body=%q", calls, method, requestPath, authorization, cookie, contentType, body)
	}
	auth.mu.Lock()
	defer auth.mu.Unlock()
	if auth.calls != 1 || auth.token != "synthetic-reviewer-session" || auth.role != "" || auth.kind != identity.UserOperation {
		t.Fatalf("auth calls=%d token=%q role=%q activity=%v", auth.calls, auth.token, auth.role, auth.kind)
	}
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if len(repo.actors) != 2 || len(repo.cases) != 2 {
		t.Fatalf("scoped reads actors=%v cases=%v", repo.actors, repo.cases)
	}
	for i := range repo.actors {
		if repo.actors[i] != reviewerDetailBFFActorID || repo.cases[i] != reviewerDetailBFFCaseID {
			t.Fatalf("read %d actor=%q case=%q", i, repo.actors[i], repo.cases[i])
		}
	}
	decMu.Lock()
	defer decMu.Unlock()
	if decCalls != 1 || decSubject != reviewerDetailBFFSubjectID || decRevision != reviewerDetailBFFRevisionID {
		t.Fatalf("declaration reads=%d subject=%q revision=%q", decCalls, decSubject, decRevision)
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.reads != 0 {
		t.Fatalf("document store reads=%d", store.reads)
	}
}
