//go:build bffintegration

package verificationhttp

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

	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
	"github.com/HernanEspinozaDev/espaciGo/internal/verification"
	"github.com/HernanEspinozaDev/espaciGo/internal/verification/declarationcrypto"
)

const declarationBFFOwner = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
const declarationBFFID = "11111111-1111-4111-8111-000000000001"
const declarationBFFConsent = "id-consent-20261108"
const declarationBFFConfiguredConsent = "id-consent-test-v2"

type declarationBFFAuth struct {
	mu    sync.Mutex
	calls int
	token identity.Secret
}

func (a *declarationBFFAuth) Authorize(_ context.Context, token identity.Secret, _ identity.Role, _ identity.ActivityKind) (identity.Principal, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.calls++
	a.token = token
	if token != "synthetic-session" {
		return identity.Principal{}, identity.ErrUnauthorized
	}
	return identity.Principal{AccountID: declarationBFFOwner}, nil
}

type declarationBFFRepository struct {
	mu           sync.Mutex
	rows         map[string]verification.DeclarationRevision
	writes, gets int
}

func (r *declarationBFFRepository) CreateRevision(_ context.Context, in verification.DeclarationRevisionInput, seal verification.DeclarationSealer) (verification.DeclarationRevision, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rev := int64(1)
	for _, old := range r.rows {
		if old.AccountID == in.AccountID && old.Revision >= rev {
			rev = old.Revision + 1
		}
	}
	key, ciphertext, err := seal(rev)
	if err != nil {
		return verification.DeclarationRevision{}, err
	}
	row := verification.DeclarationRevision{ID: in.ID, AccountID: in.AccountID, Revision: rev, KeyVersion: key, Ciphertext: ciphertext, SchemaVersion: in.SchemaVersion, ConsentVersion: in.ConsentVersion, ConsentGivenAt: in.ConsentGivenAt, RetentionAt: in.RetentionAt, CreatedAt: in.ConsentGivenAt}
	r.rows[row.ID] = row
	r.writes++
	return row, nil
}
func (r *declarationBFFRepository) GetRevision(_ context.Context, account, id string) (verification.DeclarationRevision, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.gets++
	row, ok := r.rows[id]
	if !ok || row.AccountID != account {
		return verification.DeclarationRevision{}, verification.ErrNotFound
	}
	return row, nil
}
func (r *declarationBFFRepository) GetCurrent(_ context.Context, account string) (verification.DeclarationRevision, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.gets++
	var found verification.DeclarationRevision
	for _, row := range r.rows {
		if row.AccountID == account && row.Revision > found.Revision {
			found = row
		}
	}
	if found.ID == "" {
		return verification.DeclarationRevision{}, verification.ErrNotFound
	}
	return found, nil
}

type declarationBFFIDs struct {
	mu    sync.Mutex
	calls int
}

func (g *declarationBFFIDs) ID() (string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.calls++
	return declarationBFFID, nil
}

func TestNextBFFOwnIdentityDeclarationPOSTThenGET(t *testing.T) {
	frontend := strings.TrimSpace(os.Getenv("ESPACIGO_FRONTEND_ROOT"))
	if frontend == "" {
		t.Fatal("ESPACIGO_FRONTEND_ROOT must identify authorized Next checkout")
	}
	frontend, err := filepath.Abs(frontend)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(frontend, "tooling", "identity-declaration-bff-go-integration.mjs")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(frontend, "node_modules", "typescript")); err != nil {
		t.Fatal(err)
	}
	repo := &declarationBFFRepository{rows: map[string]verification.DeclarationRevision{}}
	keyring, err := declarationcrypto.New("synthetic-key", map[string][]byte{"synthetic-key": make([]byte, 32)})
	if err != nil {
		t.Fatal(err)
	}
	ids := &declarationBFFIDs{}
	fixedNow := time.Date(2026, time.November, 13, 12, 0, 0, 0, time.UTC)
	service, err := verification.NewDeclarationService(repo, keyring, ids, func() time.Time { return fixedNow }, func(at time.Time) time.Time { return at.Add(24 * time.Hour) }, declarationBFFConfiguredConsent)
	if err != nil {
		t.Fatal(err)
	}
	auth := &declarationBFFAuth{}
	handler := NewHandlerWithDeclarationWithoutExternalProvider(auth, nil, nil, nil, service)
	var mu sync.Mutex
	var count int
	var methods, paths, authHeaders, cookies, contentTypes, bodies []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, readErr := io.ReadAll(r.Body)
		if readErr != nil {
			http.Error(w, "read failed", 500)
			return
		}
		_ = r.Body.Close()
		mu.Lock()
		count++
		methods = append(methods, r.Method)
		paths = append(paths, r.URL.Path)
		authHeaders = append(authHeaders, r.Header.Get("Authorization"))
		cookies = append(cookies, r.Header.Get("Cookie"))
		contentTypes = append(contentTypes, r.Header.Get("Content-Type"))
		bodies = append(bodies, string(body))
		mu.Unlock()
		r.Body = io.NopCloser(strings.NewReader(string(body)))
		handler.ServeHTTP(w, r)
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", "--test", "tooling/identity-declaration-bff-go-integration.mjs")
	cmd.Dir = frontend
	cmd.Env = identityBFFChildEnvironment(server.URL+"/api/v1", declarationBFFConfiguredConsent)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Next→Go declaration characterization failed: %v\n%s", err, output)
	}
	t.Logf("Node output:\n%s", output)
	mu.Lock()
	gotCount := count
	gotMethods := append([]string(nil), methods...)
	gotPaths := append([]string(nil), paths...)
	gotAuth := append([]string(nil), authHeaders...)
	gotCookies := append([]string(nil), cookies...)
	gotTypes := append([]string(nil), contentTypes...)
	gotBodies := append([]string(nil), bodies...)
	mu.Unlock()
	wantPath := "/api/v1/verifications/identity-declaration"
	if gotCount != 3 || strings.Join(gotMethods, ",") != "POST,GET,POST" || strings.Join(gotPaths, ",") != wantPath+","+wantPath+","+wantPath || strings.Join(gotAuth, ",") != "Bearer synthetic-session,Bearer synthetic-session,Bearer synthetic-session" || strings.Join(gotCookies, ",") != ",," || strings.Join(gotTypes, ",") != "application/json,,application/json" {
		t.Fatalf("upstream count/method/path/auth/cookie/type mismatch: %d %q %q %q %q %q", gotCount, gotMethods, gotPaths, gotAuth, gotCookies, gotTypes)
	}
	const wantBody = `{"first_name":"Synthetic","last_name":"Person","date_of_birth":"1990-01-02","rut":"11.222.333-9","id_country":"CHL","document_type":"ID","consent_given":true,"consent_version":"id-consent-test-v2"}`
	wantStaleBody := `{"first_name":"Synthetic","last_name":"Person","date_of_birth":"1990-01-02","rut":"11.222.333-9","id_country":"CHL","document_type":"ID","consent_given":true,"consent_version":"` + declarationBFFConsent + `"}`
	if gotBodies[0] != wantBody || gotBodies[1] != "" || gotBodies[2] != wantStaleBody {
		t.Fatalf("upstream body did not match exact contract fields")
	}
	auth.mu.Lock()
	authCalls, token := auth.calls, auth.token
	auth.mu.Unlock()
	if authCalls != 3 || token != "synthetic-session" {
		t.Fatalf("auth calls/token=%d/%q", authCalls, token)
	}
	repo.mu.Lock()
	writes, gets, rows := repo.writes, repo.gets, len(repo.rows)
	var storedAccount string
	var storedRevision int64
	var storedCiphertextBytes int
	for _, row := range repo.rows {
		storedAccount = row.AccountID
		storedRevision = row.Revision
		storedCiphertextBytes = len(row.Ciphertext)
	}
	repo.mu.Unlock()
	if writes != 1 || gets != 2 || rows != 1 || storedAccount != declarationBFFOwner || storedRevision != 1 || storedCiphertextBytes == 0 {
		t.Fatalf("repository writes/readbacks/rows/account/revision/ciphertext=%d/%d/%d/%q/%d/%d", writes, gets, rows, storedAccount, storedRevision, storedCiphertextBytes)
	}
	ids.mu.Lock()
	idCalls := ids.calls
	ids.mu.Unlock()
	if idCalls != 1 {
		t.Fatalf("synthetic ID calls=%d", idCalls)
	}
}

func identityBFFChildEnvironment(apiBaseURL, consentVersion string) []string {
	env := []string{"PATH=" + os.Getenv("PATH"), "HOME=" + os.Getenv("HOME"), "TMPDIR=" + os.Getenv("TMPDIR"), "ESPACIGO_FRONTEND_ROOT=" + os.Getenv("ESPACIGO_FRONTEND_ROOT"), "ESPACIGO_TEST_API_BASE_URL=" + apiBaseURL, "NEXT_PUBLIC_IDENTITY_DECLARATION_CONSENT_VERSION=" + consentVersion}
	return env
}
