//go:build bffintegration

package http_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
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

	documentstorage "github.com/HernanEspinozaDev/espaciGo/internal/adapters/documentstorage"
	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
	transporthttp "github.com/HernanEspinozaDev/espaciGo/internal/documentverification/transport/http"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
)

const reviewerDownloadBFFCaseID = "99999999-9999-4999-8999-999999999999"
const reviewerDownloadBFFFileID = "22222222-2222-4222-8222-222222222222"
const reviewerDownloadBFFActorID = "66666666-6666-4666-8666-666666666666"
const reviewerDownloadBFFSubjectID = "11111111-1111-4111-8111-111111111111"

type reviewerDownloadBFFAuth struct {
	mu    sync.Mutex
	calls int
	token identity.Secret
	role  identity.Role
	kind  identity.ActivityKind
}

func (a *reviewerDownloadBFFAuth) Authorize(_ context.Context, token identity.Secret, role identity.Role, kind identity.ActivityKind) (identity.Principal, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.calls++
	a.token, a.role, a.kind = token, role, kind
	return identity.Principal{AccountID: reviewerDownloadBFFActorID}, nil
}

type reviewerDownloadBFFBlobs struct {
	mu      sync.Mutex
	objects map[string][]byte
	gets    int
}

func (b *reviewerDownloadBFFBlobs) Put(_ context.Context, id string, value []byte) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.objects == nil {
		b.objects = map[string][]byte{}
	}
	b.objects[id] = append([]byte(nil), value...)
	return nil
}
func (b *reviewerDownloadBFFBlobs) Get(_ context.Context, id string) ([]byte, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.gets++
	value, ok := b.objects[id]
	if !ok {
		return nil, os.ErrNotExist
	}
	return append([]byte(nil), value...), nil
}
func (b *reviewerDownloadBFFBlobs) Delete(_ context.Context, id string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.objects, id)
	return nil
}

type reviewerDownloadBFFRepository struct {
	mu            sync.Mutex
	snapshot      dv.Snapshot
	authorization dv.Authorization
	actors, cases []string
}

func (*reviewerDownloadBFFRepository) CreateCase(context.Context, string, dv.Purpose, string, func(dv.Authorization) (dv.Snapshot, error)) (dv.Snapshot, error) {
	return dv.Snapshot{}, dv.ErrInvalid
}
func (*reviewerDownloadBFFRepository) Transition(context.Context, string, dv.Purpose, string, uint64, dv.TransitionAudit, func(*dv.Snapshot, dv.Authorization) error) error {
	return dv.ErrCapability
}
func (*reviewerDownloadBFFRepository) ListReviewerCases(context.Context, string) ([]dv.Snapshot, error) {
	return nil, dv.ErrNotFound
}
func (*reviewerDownloadBFFRepository) LoadReviewerCase(context.Context, string, string) (dv.Snapshot, error) {
	return dv.Snapshot{}, dv.ErrNotFound
}
func (r *reviewerDownloadBFFRepository) WithReviewerCaseForRead(ctx context.Context, actor, caseID string, read func(context.Context, dv.Snapshot, dv.Authorization) error) error {
	r.mu.Lock()
	r.actors = append(r.actors, actor)
	r.cases = append(r.cases, caseID)
	snapshot, auth := r.snapshot, r.authorization
	r.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	return read(ctx, snapshot, auth)
}

func TestNextBFFDownloadsReviewerIdentityEvidenceFromRegisteredGoHandler(t *testing.T) {
	frontendRoot := strings.TrimSpace(os.Getenv("ESPACIGO_FRONTEND_ROOT"))
	if frontendRoot == "" {
		t.Fatal("ESPACIGO_FRONTEND_ROOT must select the local Next checkout")
	}
	frontendRoot, err := filepath.Abs(frontendRoot)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(frontendRoot, "tooling", "reviewer-download-bff-go-integration.mjs")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(frontendRoot, "node_modules", "typescript")); err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.SetRGBA(0, 0, color.RGBA{R: 17, G: 33, B: 65, A: 255})
	var encodedPNG bytes.Buffer
	if err := png.Encode(&encodedPNG, img); err != nil {
		t.Fatal(err)
	}
	payload := append([]byte(nil), encodedPNG.Bytes()...)
	now := time.Date(2031, 4, 5, 12, 0, 0, 0, time.UTC)
	blobs := &reviewerDownloadBFFBlobs{objects: make(map[string][]byte)}
	store, err := documentstorage.New(blobs, []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.PutIdentityDocument(context.Background(), reviewerDownloadBFFSubjectID, reviewerDownloadBFFCaseID, reviewerDownloadBFFFileID, payload); err != nil {
		t.Fatal(err)
	}
	file := dv.File{
		ID:            reviewerDownloadBFFFileID,
		State:         dv.FileClean,
		Size:          int64(len(payload)),
		MIME:          "image/png",
		ContentSHA256: sha256.Sum256(payload),
		RetentionAt:   now.Add(time.Hour),
	}
	snapshot := dv.Snapshot{
		ID:        reviewerDownloadBFFCaseID,
		SubjectID: reviewerDownloadBFFSubjectID,
		Purpose:   dv.PurposeIdentity,
		State:     dv.CaseInReview,
		Revision:  4,
		Files:     []dv.File{file},
	}
	grant := dv.ReviewerGrant{
		ReviewerID: reviewerDownloadBFFActorID,
		Capability: dv.CapabilityIdentity,
		ScopeID:    reviewerDownloadBFFSubjectID,
	}
	repo := &reviewerDownloadBFFRepository{
		snapshot:      snapshot,
		authorization: dv.Authorization{ActorID: reviewerDownloadBFFActorID, ReviewerGrants: []dv.ReviewerGrant{grant}},
	}
	service, err := dv.NewService(repo)
	if err != nil {
		t.Fatal(err)
	}
	access, err := dv.NewReviewerAccessService(repo, service, store)
	if err != nil {
		t.Fatal(err)
	}
	access.SetClock(func() time.Time { return now })
	auth := &reviewerDownloadBFFAuth{}
	mux := http.NewServeMux()
	transporthttp.RegisterReviewerCasesRoutes(mux, transporthttp.NewReviewerCasesHandler(auth, access))
	var requestMu sync.Mutex
	calls := 0
	method, requestPath, authorization, cookie, contentType, body := "", "", "", "", "", ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, readErr := io.ReadAll(r.Body)
		if readErr != nil {
			http.Error(w, "synthetic read failure", 400)
			return
		}
		_ = r.Body.Close()
		requestMu.Lock()
		calls++
		method = r.Method
		requestPath = r.URL.Path
		authorization = r.Header.Get("Authorization")
		cookie = r.Header.Get("Cookie")
		contentType = r.Header.Get("Content-Type")
		body = string(b)
		requestMu.Unlock()
		mux.ServeHTTP(w, r)
	}))
	defer server.Close()
	tmp := t.TempDir()
	cmd := exec.CommandContext(context.Background(), "node", "--test", "tooling/reviewer-download-bff-go-integration.mjs")
	cmd.Dir = frontendRoot
	cmd.Env = []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + os.Getenv("HOME"),
		"TMPDIR=" + tmp,
		"ESPACIGO_TEST_API_BASE_URL=" + server.URL + "/api/v1",
		"EXPECTED_REVIEWER_DOWNLOAD_BASE64=" + base64.StdEncoding.EncodeToString(payload),
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Next BFF→Go identity download characterization: %v\n%s", err, output)
	}
	t.Logf("Node test output:\n%s", output)
	requestMu.Lock()
	defer requestMu.Unlock()
	wantPath := "/api/v1/document-verification/reviewer/cases" + "/" + reviewerDownloadBFFCaseID + "/files/" + reviewerDownloadBFFFileID + "/content"
	if calls != 1 || method != http.MethodGet || requestPath != wantPath || authorization != "Bearer synthetic-reviewer-session" || cookie != "" || contentType != "" || body != "" {
		t.Fatalf(
			"upstream request calls=%d method=%q path=%q bearer_seen=%t cookie=%t content_type=%q body_len=%d",
			calls, method, requestPath, authorization != "", cookie != "", contentType, len(body),
		)
	}
	auth.mu.Lock()
	defer auth.mu.Unlock()
	if auth.calls != 1 || auth.token != "synthetic-reviewer-session" || auth.role != "" || auth.kind != identity.UserOperation {
		t.Fatalf("auth calls=%d role=%q kind=%v", auth.calls, auth.role, auth.kind)
	}
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if len(repo.actors) != 1 || len(repo.cases) != 1 {
		t.Fatalf("scoped repository reads=%d", len(repo.actors))
	}
	for i := range repo.actors {
		if repo.actors[i] != reviewerDownloadBFFActorID || repo.cases[i] != reviewerDownloadBFFCaseID {
			t.Fatalf("read %d escaped actor/case scope", i)
		}
	}
	blobs.mu.Lock()
	defer blobs.mu.Unlock()
	if blobs.gets != 1 {
		t.Fatalf("encrypted blob reads=%d", blobs.gets)
	}
}
