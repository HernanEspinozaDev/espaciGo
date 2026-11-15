package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/identitydeclaration"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
	"github.com/HernanEspinozaDev/espaciGo/internal/migrator"
	"github.com/HernanEspinozaDev/espaciGo/internal/verification"
	verificationhttp "github.com/HernanEspinozaDev/espaciGo/internal/verification/transport/http"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type runtimeDeclarationRepo struct {
	rows map[string]verification.DeclarationRevision
}

func (r *runtimeDeclarationRepo) CreateRevision(_ context.Context, in verification.DeclarationRevisionInput, seal verification.DeclarationSealer) (verification.DeclarationRevision, error) {
	rev := int64(1)
	for _, row := range r.rows {
		if row.AccountID == in.AccountID && row.Revision >= rev {
			rev = row.Revision + 1
		}
	}
	version, ciphertext, err := seal(rev)
	if err != nil {
		return verification.DeclarationRevision{}, err
	}
	row := verification.DeclarationRevision{ID: in.ID, AccountID: in.AccountID, Revision: rev, KeyVersion: version, Ciphertext: ciphertext, SchemaVersion: in.SchemaVersion, ConsentVersion: in.ConsentVersion, ConsentGivenAt: in.ConsentGivenAt, RetentionAt: in.RetentionAt, CreatedAt: in.ConsentGivenAt}
	r.rows[row.ID] = row
	return row, nil
}
func (r *runtimeDeclarationRepo) GetRevision(_ context.Context, account, id string) (verification.DeclarationRevision, error) {
	v, ok := r.rows[id]
	if !ok || v.AccountID != account {
		return verification.DeclarationRevision{}, verification.ErrNotFound
	}
	return v, nil
}
func (r *runtimeDeclarationRepo) GetCurrent(_ context.Context, account string) (verification.DeclarationRevision, error) {
	var best verification.DeclarationRevision
	for _, v := range r.rows {
		if v.AccountID == account && v.Revision > best.Revision {
			best = v
		}
	}
	if best.ID == "" {
		return best, verification.ErrNotFound
	}
	return best, nil
}

type runtimeDeclarationAuth struct{ accountID string }

func (a runtimeDeclarationAuth) Authorize(context.Context, identity.Secret, identity.Role, identity.ActivityKind) (identity.Principal, error) {
	accountID := a.accountID
	if accountID == "" {
		accountID = "123e4567-e89b-42d3-a456-426614174000"
	}
	return identity.Principal{AccountID: accountID, Roles: []identity.Role{"usuario"}}, nil
}
func env(values map[string]string) func(string) string {
	return func(k string) string { return values[k] }
}
func runtimeHandler(t *testing.T, values map[string]string, repo verification.DeclarationRevisionRepository) http.Handler {
	return runtimeHandlerForAccount(t, values, repo, "")
}
func runtimeHandlerForAccount(t *testing.T, values map[string]string, repo verification.DeclarationRevisionRepository, accountID string) http.Handler {
	t.Helper()
	svc, err := newRuntimeIdentityDeclarationService(env(values), repo, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	return verificationhttp.NewHandlerWithDeclaration(runtimeDeclarationAuth{accountID: accountID}, nil, nil, nil, svc)
}
func requestRuntime(h http.Handler, method, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "/api/v1/verifications/identity-declaration", strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer synthetic")
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}
func writeKeyring(t *testing.T, path, active string, keys map[string]string) {
	t.Helper()
	b, _ := json.Marshal(map[string]any{"current": active, "keys": keys})
	if err := os.WriteFile(path, b, 0600); err != nil {
		t.Fatal(err)
	}
}
func TestValidateIdentityDeclarationRuntimeDSN(t *testing.T) {
	valid := "postgresql://espacigo_api_login@127.0.0.1:5432/espacigo_dms_stage_20261020"
	if err := validateIdentityDeclarationRuntimeDSN(valid); err != nil {
		t.Fatalf("valid native application DSN rejected: %v", err)
	}
	parsed, err := pgxpool.ParseConfig(valid)
	if err != nil {
		t.Fatalf("pgx rejected the allowlisted native application DSN: %v", err)
	}
	if parsed.ConnConfig.User != "espacigo_api_login" || parsed.ConnConfig.Database != "espacigo_dms_stage_20261020" || parsed.ConnConfig.Password != "" || parsed.ConnConfig.Port != 5432 || parsed.ConnConfig.Host != "127.0.0.1" {
		t.Fatal("pgx did not preserve the pinned native application target")
	}
	for _, tc := range []struct{ name, dsn string }{
		{"superuser", "postgresql://postgres@/espacigo_dms_stage_20261020?host=/var/run/postgresql&port=5432"},
		{"wrong socket", "postgresql://espacigo_api_login@/espacigo_dms_stage_20261020?host=/tmp&port=5432"},
		{"wrong port", "postgresql://espacigo_api_login@/espacigo_dms_stage_20261020?host=/var/run/postgresql&port=5433"},
		{"wrong database", "postgresql://espacigo_api_login@/postgres?host=/var/run/postgresql&port=5432"},
		{"wrong user", "postgresql://other@/espacigo_dms_stage_20261020?host=/var/run/postgresql&port=5432"},
		{"embedded password", "postgresql://espacigo_api_login:secret@/espacigo_dms_stage_20261020?host=/var/run/postgresql&port=5432"},
		{"query override", valid + "&sslmode=disable"},
		{"duplicate host", valid + "&host=/tmp"},
		{"fragment", valid + "#anything"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateIdentityDeclarationRuntimeDSN(tc.dsn); err == nil {
				t.Fatal("expected DSN rejection")
			}
		})
	}
}

func TestValidateIdentityDeclarationCleanupDSN(t *testing.T) {
	valid := "postgresql://espacigo_migrator_login@127.0.0.1:5432/espacigo_dms_stage_20261020"
	if err := validateIdentityDeclarationCleanupDSN(valid); err != nil {
		t.Fatalf("valid local migrator DSN rejected: %v", err)
	}
	parsed, err := pgxpool.ParseConfig(valid)
	if err != nil {
		t.Fatalf("pgx rejected the allowlisted cleanup DSN: %v", err)
	}
	if parsed.ConnConfig.User != "espacigo_migrator_login" || parsed.ConnConfig.Database != identityDeclarationNativeTarget || parsed.ConnConfig.Password != "" || parsed.ConnConfig.Port != 5432 || parsed.ConnConfig.Host != "127.0.0.1" {
		t.Fatal("pgx did not preserve the pinned migrator cleanup target")
	}
	for _, tc := range []struct{ name, dsn string }{
		{"superuser", "postgresql://postgres@/espacigo_dms_stage_20261020?host=/var/run/postgresql&port=5432"},
		{"wrong user", "postgresql://other@/espacigo_dms_stage_20261020?host=/var/run/postgresql&port=5432"},
		{"wrong socket", "postgresql://espacigo_migrator_login@/espacigo_dms_stage_20261020?host=/tmp&port=5432"},
		{"wrong port", "postgresql://espacigo_migrator_login@/espacigo_dms_stage_20261020?host=/var/run/postgresql&port=5433"},
		{"wrong database", "postgresql://espacigo_migrator_login@/postgres?host=/var/run/postgresql&port=5432"},
		{"embedded password", "postgresql://espacigo_migrator_login:secret@/espacigo_dms_stage_20261020?host=/var/run/postgresql&port=5432"},
		{"query override", valid + "&sslmode=disable"},
		{"fragment", valid + "#anything"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateIdentityDeclarationCleanupDSN(tc.dsn); err == nil {
				t.Fatal("expected cleanup DSN rejection")
			}
		})
	}
}

func TestValidateIdentityDeclarationNativeLedgerMatchesSource(t *testing.T) {
	migrations, err := migrator.DiscoverMigrations(filepath.Join("..", "..", "db", "migrations"))
	if err != nil || len(migrations) < identityDeclarationNativeLedgerVersion {
		t.Fatalf("load source migration inventory: count=%d err=%v", len(migrations), err)
	}
	expected := migrations[:identityDeclarationNativeLedgerVersion]
	applied := make([]migrator.HistoryEntry, len(expected))
	for i, migration := range expected {
		applied[i] = migrator.HistoryEntry{Version: migration.Version, Name: migration.Name, Checksum: migration.Checksum}
	}
	if err := validateIdentityDeclarationNativeLedger(expected, applied); err != nil {
		t.Fatalf("matching source ledger rejected: %v", err)
	}
	for _, tc := range []struct {
		name   string
		mutate func([]migrator.Migration, []migrator.HistoryEntry) ([]migrator.Migration, []migrator.HistoryEntry)
	}{
		{"missing version", func(m []migrator.Migration, h []migrator.HistoryEntry) ([]migrator.Migration, []migrator.HistoryEntry) {
			return m, h[:84]
		}},
		{"extra version", func(m []migrator.Migration, h []migrator.HistoryEntry) ([]migrator.Migration, []migrator.HistoryEntry) {
			return m, append(h, migrator.HistoryEntry{Version: identityDeclarationNativeLedgerVersion + 1})
		}},
		{"version gap", func(m []migrator.Migration, h []migrator.HistoryEntry) ([]migrator.Migration, []migrator.HistoryEntry) {
			h[1].Version = 3
			return m, h
		}},
		{"renamed migration", func(m []migrator.Migration, h []migrator.HistoryEntry) ([]migrator.Migration, []migrator.HistoryEntry) {
			h[2].Name += ".renamed"
			return m, h
		}},
		{"checksum drift", func(m []migrator.Migration, h []migrator.HistoryEntry) ([]migrator.Migration, []migrator.HistoryEntry) {
			h[3].Checksum = strings.Repeat("0", 64)
			return m, h
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mutatedHistory := append([]migrator.HistoryEntry(nil), applied...)
			mutatedSource, mutatedHistory := tc.mutate(expected, mutatedHistory)
			if err := validateIdentityDeclarationNativeLedger(mutatedSource, mutatedHistory); err == nil {
				t.Fatal("expected source ledger mismatch rejection")
			}
		})
	}
}

func validateIdentityDeclarationRuntimeDSN(raw string) error {
	return validateIdentityDeclarationTCPDSN(raw, "espacigo_api_login")
}
func validateIdentityDeclarationCleanupDSN(raw string) error {
	return validateIdentityDeclarationTCPDSN(raw, "espacigo_migrator_login")
}
func validateIdentityDeclarationTCPDSN(raw, expectedUser string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "postgresql" || u.Host != "127.0.0.1:5432" || u.Path != "/"+identityDeclarationNativeTarget || u.RawPath != "" || u.RawQuery != "" || u.Fragment != "" || u.User == nil || u.User.Username() != expectedUser {
		return errors.New("native KYC integration DSN must pin TCP 127.0.0.1:5432, the exact database and login role")
	}
	if _, hasPassword := u.User.Password(); hasPassword {
		return errors.New("native KYC integration DSN must use an external passfile, not an embedded password")
	}
	return nil
}

func validateIdentityDeclarationNativeLedger(source []migrator.Migration, applied []migrator.HistoryEntry) error {
	if len(source) < identityDeclarationNativeLedgerVersion || len(applied) != identityDeclarationNativeLedgerVersion {
		return errors.New("native KYC requires exactly the source-backed V1..V96 ledger prefix")
	}
	for i := 0; i < identityDeclarationNativeLedgerVersion; i++ {
		version := int64(i + 1)
		if source[i].Version != version || applied[i].Version != version {
			return errors.New("native KYC migration ledger has a gap or unexpected version")
		}
		if source[i].Name != applied[i].Name {
			return errors.New("native KYC applied migration name differs from source")
		}
		if source[i].Checksum != applied[i].Checksum {
			return errors.New("native KYC applied migration checksum differs from source")
		}
	}
	return nil
}

func TestIdentityDeclarationRuntimeFailClosedWhenUnconfigured(t *testing.T) {
	repo := &runtimeDeclarationRepo{rows: map[string]verification.DeclarationRevision{}}
	h := runtimeHandler(t, nil, repo)
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		w := requestRuntime(h, method, `{"consent_given":true}`)
		if w.Code != 503 || !strings.Contains(w.Body.String(), "service_unavailable") {
			t.Fatalf("%s status=%d body=%s", method, w.Code, w.Body.String())
		}
	}
}
func TestIdentityDeclarationRuntimeRejectsInvalidKeyrings(t *testing.T) {
	dir := t.TempDir()
	valid := filepath.Join(dir, "keyring.json")
	key := hex.EncodeToString(make([]byte, 32))
	writeKeyring(t, valid, "v1", map[string]string{"v1": key})
	badJSON := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(badJSON, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	wide := filepath.Join(dir, "wide.json")
	if err := os.WriteFile(wide, []byte(`{"current":"v1","keys":{"v1":"`+key+`"}}`), 0644); err != nil {
		t.Fatal(err)
	}
	sym := filepath.Join(dir, "link.json")
	if err := os.Symlink(valid, sym); err != nil {
		t.Fatal(err)
	}
	cases := []string{"relative.json", sym, wide, badJSON}
	for _, path := range cases {
		t.Run(filepath.Base(path), func(t *testing.T) {
			_, err := loadIdentityDeclarationKeyring(env(map[string]string{"IDENTITY_DECLARATION_KEYRING_FILE": path}))
			if err == nil || err.Error() != "identity declaration keyring configuration is invalid" {
				t.Fatalf("error=%v", err)
			}
		})
	}
}
func TestRuntimeIdentityDeclarationServiceRequiresRutHMACSecret(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "keyring.json")
	writeKeyring(t, path, "v1", map[string]string{"v1": hex.EncodeToString(make([]byte, 32))})
	for _, secret := range []string{"", "too-short"} {
		values := map[string]string{"IDENTITY_DECLARATION_KEYRING_FILE": path, "IDENTITY_RUT_HMAC_SECRET": secret}
		if svc, err := newRuntimeIdentityDeclarationService(env(values), &runtimeDeclarationRepo{rows: map[string]verification.DeclarationRevision{}}, time.Now); err == nil || svc != nil {
			t.Fatalf("accepted invalid runtime secret %q", secret)
		}
	}
}

func TestIdentityDeclarationRuntimeRestartReadbackAndRotation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "keyring.json")
	k1 := hex.EncodeToString(make([]byte, 32))
	k2 := hex.EncodeToString([]byte("12345678901234567890123456789012"))
	writeKeyring(t, path, "v1", map[string]string{"v1": k1})
	repo := &runtimeDeclarationRepo{rows: map[string]verification.DeclarationRevision{}}
	values := map[string]string{"IDENTITY_DECLARATION_KEYRING_FILE": path, "IDENTITY_RUT_HMAC_SECRET": strings.Repeat("h", 32)}
	at := time.Date(2026, 11, 8, 12, 0, 0, 0, time.FixedZone("x", 3600))
	svc, err := newRuntimeIdentityDeclarationService(env(values), repo, func() time.Time { return at })
	if err != nil {
		t.Fatal(err)
	}
	_ = svc
	if got := identityDeclarationRetentionPolicy(at); !got.Equal(at.UTC().Add(30 * 24 * time.Hour)) {
		t.Fatalf("retention=%s", got)
	}
	h := runtimeHandler(t, values, repo)
	minorBody := `{"first_name":"Minor","last_name":"Person","date_of_birth":"2010-05-01","rut":"11.222.333-9","id_country":"CHL","document_type":"ID","consent_given":true,"consent_version":"id-consent-20261108"}`
	if w := requestRuntime(h, http.MethodPost, minorBody); w.Code != http.StatusUnprocessableEntity || len(repo.rows) != 0 {
		t.Fatalf("minor POST status=%d rows=%d body=%s", w.Code, len(repo.rows), w.Body.String())
	}
	body := `{"first_name":"Synthetic","last_name":"Person","date_of_birth":"1990-01-02","rut":"11.222.333-9","id_country":"CHL","document_type":"ID","consent_given":true,"consent_version":"id-consent-20261108"}`
	if w := requestRuntime(h, http.MethodPost, body); w.Code != 201 {
		t.Fatalf("POST status=%d body=%s", w.Code, w.Body.String())
	}
	if repo.rows == nil || len(repo.rows) != 1 {
		t.Fatal("synthetic repository did not retain declaration")
	}
	// A newly constructed service/handler models a process restart over durable storage.
	restarted := runtimeHandler(t, values, repo)
	w := requestRuntime(restarted, http.MethodGet, "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Synthetic") {
		t.Fatalf("restart GET status=%d body=%s", w.Code, w.Body.String())
	}
	writeKeyring(t, path, "v2", map[string]string{"v1": k1, "v2": k2})
	rotated := runtimeHandler(t, values, repo)
	w = requestRuntime(rotated, http.MethodGet, "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Synthetic") {
		t.Fatalf("rotated GET status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestIdentityDeclarationRuntimeUsesConfiguredConsentVersion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "keyring.json")
	writeKeyring(t, path, "v1", map[string]string{"v1": hex.EncodeToString(make([]byte, 32))})
	const configuredVersion = "id-consent-test-v2"
	values := map[string]string{
		"IDENTITY_DECLARATION_KEYRING_FILE":    path,
		"IDENTITY_RUT_HMAC_SECRET":             strings.Repeat("h", 32),
		"IDENTITY_DECLARATION_CONSENT_VERSION": configuredVersion,
	}
	repo := &runtimeDeclarationRepo{rows: map[string]verification.DeclarationRevision{}}
	handler := runtimeHandler(t, values, repo)

	staleBody := `{"first_name":"Synthetic","last_name":"Person","date_of_birth":"1990-01-02","rut":"11.222.333-9","id_country":"CHL","document_type":"ID","consent_given":true,"consent_version":"id-consent-20261108"}`
	if response := requestRuntime(handler, http.MethodPost, staleBody); response.Code != http.StatusUnprocessableEntity || len(repo.rows) != 0 {
		t.Fatalf("stale POST status=%d rows=%d body=%s", response.Code, len(repo.rows), response.Body.String())
	}

	configuredBody := `{"first_name":"Synthetic","last_name":"Person","date_of_birth":"1990-01-02","rut":"11.222.333-9","id_country":"CHL","document_type":"ID","consent_given":true,"consent_version":"id-consent-test-v2"}`
	if response := requestRuntime(handler, http.MethodPost, configuredBody); response.Code != http.StatusCreated {
		t.Fatalf("configured POST status=%d body=%s", response.Code, response.Body.String())
	}
	if len(repo.rows) != 1 {
		t.Fatalf("configured POST rows=%d, want 1", len(repo.rows))
	}
	for _, row := range repo.rows {
		if row.ConsentVersion != configuredVersion {
			t.Fatalf("stored consent version=%q, want configured version", row.ConsentVersion)
		}
	}

	response := requestRuntime(handler, http.MethodGet, "")
	if response.Code != http.StatusOK {
		t.Fatalf("GET status=%d body=%s", response.Code, response.Body.String())
	}
	var current struct {
		ConsentVersion string `json:"consent_version"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &current); err != nil {
		t.Fatal(err)
	}
	if current.ConsentVersion != configuredVersion {
		t.Fatalf("readback consent version=%q, want configured version", current.ConsentVersion)
	}
}

type fakeRutClaimStore struct {
	claims       map[string]string
	claimedCount int
}

func (f *fakeRutClaimStore) Claim(_ context.Context, accountID, _, fingerprint string, _, _ time.Time) error {
	if f.claims == nil {
		f.claims = make(map[string]string)
	}
	f.claimedCount++
	if owner, exists := f.claims[fingerprint]; exists && owner != accountID {
		return verification.ErrConflict
	}
	f.claims[fingerprint] = accountID
	return nil
}
func (f *fakeRutClaimStore) MarkVerified(_ context.Context, _, _, _ string, _ time.Time) error {
	return nil
}
func (f *fakeRutClaimStore) RevokeVerified(_ context.Context, _ string, _ time.Time) {}

type fakeSessionDeletionQueue struct {
	enqueued []string
}

func (q *fakeSessionDeletionQueue) EnqueueSessionDeletion(_ context.Context, sessionID string) error {
	q.enqueued = append(q.enqueued, sessionID)
	return nil
}

func (q *fakeSessionDeletionQueue) PendingSessionDeletions(_ context.Context) ([]string, error) {
	return []string{"pending-didit-sess-1"}, nil
}

func (q *fakeSessionDeletionQueue) AcknowledgeSessionDeletion(_ context.Context, _ string) error {
	return nil
}

type runtimeAttemptRepo struct{}

func (r *runtimeAttemptRepo) PrepareAttempt(_ context.Context, in verification.PrepareAttemptInput) (verification.ProviderAttempt, bool, error) {
	return verification.ProviderAttempt{
		ID:              in.ID,
		AccountID:       in.AccountID,
		DeclarationID:   in.DeclarationID,
		Provider:        in.Provider,
		Environment:     in.Environment,
		ApplicationID:   in.ApplicationID,
		WorkflowID:      in.WorkflowID,
		WorkflowVersion: in.WorkflowVersion,
		IdempotencyKey:  in.IdempotencyKey,
		CorrelationID:   in.CorrelationID,
		Status:          "CREATED",
		CreatedAt:       in.Now,
		UpdatedAt:       in.Now,
		RetentionAt:     in.RetentionAt,
	}, true, nil
}
func (r *runtimeAttemptRepo) MarkAttemptSessionReady(_ context.Context, _, _ string, _ time.Time) (verification.ProviderAttempt, error) {
	return verification.ProviderAttempt{}, nil
}
func (r *runtimeAttemptRepo) MarkAttemptFailed(_ context.Context, _ string, _ time.Time) error {
	return nil
}
func (r *runtimeAttemptRepo) GetAttemptForAccount(_ context.Context, _, _ string, _ time.Time, _ time.Duration) (verification.ProviderAttempt, error) {
	return verification.ProviderAttempt{}, nil
}
func (r *runtimeAttemptRepo) FindAttemptForWebhook(_ context.Context, _, _, _, _, _ string) (verification.ProviderAttempt, error) {
	return verification.ProviderAttempt{}, nil
}
func (r *runtimeAttemptRepo) InsertOrGetWebhookEvent(_ context.Context, _ verification.WebhookEventRecordInput) (verification.ProviderEvent, bool, error) {
	return verification.ProviderEvent{}, true, nil
}
func (r *runtimeAttemptRepo) SaveAttemptDecisionAndCompleteEvent(_ context.Context, _, _, _, _, _ string, _ []byte, _ time.Time) (verification.ProviderAttempt, error) {
	return verification.ProviderAttempt{}, nil
}
func (r *runtimeAttemptRepo) CommitWebhookDecision(_ context.Context, _ verification.CommitWebhookDecisionInput) (verification.ProviderAttempt, error) {
	return verification.ProviderAttempt{}, nil
}
func (r *runtimeAttemptRepo) UpdateAttemptStatusAndCompleteEvent(_ context.Context, _, _, _, _ string, _ time.Time) (verification.ProviderAttempt, error) {
	return verification.ProviderAttempt{}, nil
}
func (r *runtimeAttemptRepo) CompleteWebhookEvent(_ context.Context, _, _ string, _ time.Time) error {
	return nil
}

func TestLegacyDiditTestFactoryWiresProvidedRutClaimStore(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "keyring.json")
	writeKeyring(t, path, "v1", map[string]string{"v1": hex.EncodeToString(make([]byte, 32))})
	values := map[string]string{
		"IDENTITY_DECLARATION_KEYRING_FILE": path,
		"DIDIT_API_KEY":                     "test-api-key",
		"DIDIT_WORKFLOW_ID":                 "test-workflow-id",
		"IDENTITY_RUT_HMAC_SECRET":          strings.Repeat("s", 32),
	}
	repo := &runtimeDeclarationRepo{rows: make(map[string]verification.DeclarationRevision)}
	declSvc, err := newRuntimeIdentityDeclarationService(env(values), repo, time.Now)
	if err != nil || declSvc == nil {
		t.Fatalf("newRuntimeIdentityDeclarationService failed: %v", err)
	}
	customStore := verification.NewMemoryRutClaimRegistry()
	svc, err := newTestDiditService(env(values), &runtimeAttemptRepo{}, declSvc, time.Now, customStore, &fakeSessionDeletionQueue{})
	if err != nil || svc == nil {
		t.Fatalf("newTestDiditService failed: %v", err)
	}
}

func TestLegacyDiditTestFactoryWiresProvidedSessionDeletionQueue(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "keyring.json")
	writeKeyring(t, path, "v1", map[string]string{"v1": hex.EncodeToString(make([]byte, 32))})
	values := map[string]string{
		"IDENTITY_DECLARATION_KEYRING_FILE": path,
		"DIDIT_API_KEY":                     "test-api-key",
		"DIDIT_WORKFLOW_ID":                 "test-workflow-id",
		"IDENTITY_RUT_HMAC_SECRET":          strings.Repeat("s", 32),
	}
	repo := &runtimeDeclarationRepo{rows: make(map[string]verification.DeclarationRevision)}
	declSvc, err := newRuntimeIdentityDeclarationService(env(values), repo, time.Now)
	if err != nil || declSvc == nil {
		t.Fatalf("newRuntimeIdentityDeclarationService failed: %v", err)
	}
	customQueue := &fakeSessionDeletionQueue{}
	svc, err := newTestDiditService(env(values), &runtimeAttemptRepo{}, declSvc, time.Now, verification.NewMemoryRutClaimRegistry(), customQueue)
	if err != nil || svc == nil {
		t.Fatalf("newTestDiditService failed: %v", err)
	}
	pending := svc.PendingRemoteDeletions()
	if len(pending) != 1 || pending[0] != "pending-didit-sess-1" {
		t.Fatalf("expected rehydrated pending deletions from queue, got %v", pending)
	}
}

func TestLegacyDiditTestFactoryStaysDisabledWithoutBothDurableStores(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "keyring.json")
	writeKeyring(t, path, "v1", map[string]string{"v1": hex.EncodeToString(make([]byte, 32))})
	values := map[string]string{
		"IDENTITY_DECLARATION_KEYRING_FILE": path,
		"DIDIT_API_KEY":                     "test-api-key",
		"DIDIT_WORKFLOW_ID":                 "test-workflow-id",
		"IDENTITY_RUT_HMAC_SECRET":          strings.Repeat("s", 32),
	}
	repo := &runtimeDeclarationRepo{rows: make(map[string]verification.DeclarationRevision)}
	declSvc, err := newRuntimeIdentityDeclarationService(env(values), repo, time.Now)
	if err != nil || declSvc == nil {
		t.Fatalf("newRuntimeIdentityDeclarationService failed: %v", err)
	}
	for _, stores := range [][]any{
		{verification.NewMemoryRutClaimRegistry()},
		{&fakeSessionDeletionQueue{}},
		{},
	} {
		svc, err := newTestDiditService(env(values), &runtimeAttemptRepo{}, declSvc, time.Now, stores...)
		if err != nil || svc != nil {
			t.Fatalf("Didit runtime must remain disabled without both durable stores; service=%v err=%v", svc, err)
		}
	}
}

const identityDeclarationNativeTestDSNEnv = "IDENTITY_DECLARATION_NATIVE_TEST_DSN"
const identityDeclarationNativeCleanupDSNEnv = "IDENTITY_DECLARATION_NATIVE_CLEANUP_DSN"
const identityDeclarationNativeTestAccountEnv = "IDENTITY_DECLARATION_NATIVE_TEST_ACCOUNT_ID"
const identityDeclarationNativeOtherAccountEnv = "IDENTITY_DECLARATION_NATIVE_OTHER_ACCOUNT_ID"
const identityDeclarationNativeTarget = "espacigo_dms_stage_20261020"
const identityDeclarationNativeTargetOID = int64(16388)
const identityDeclarationNativeV90Checksum = "23bbacde2cf992f912926f84945c701422b097598e698b2e8f38158b61622592"
const identityDeclarationNativeV91Checksum = "6ceee080736bd965a27226e3810d28ede0ecf2deb52477716a504e2b425c1250"
const identityDeclarationNativeV92Checksum = "bf7db344af0752ac479b4422dfa0e770cd11623c3be2b51c4449dba89c93cc03"
const identityDeclarationNativeV93Checksum = "885d3f926ffc7d3420f2391a257ca2ed68663e71d4cd54f4cf310c79bd8b7292"
const identityDeclarationNativeV94Checksum = "a4ee15a45ab352527e93c00e82763056b57b3750064483b8b90aa6891c0672a9"
const identityDeclarationNativeV95Checksum = "351a9bbdbd890e34b3b5db46ecffa971d49b1f5d4519d9bf7f2b1eae1e897275"
const identityDeclarationNativeV96Checksum = "a206e74995db47112c6b382c33ffab5badcab2ec9b9859af0fbd018478cc7ca5"
const identityDeclarationNativePeerDSN = "postgresql://postgres@/espacigo_dms_stage_20261020?host=/var/run/postgresql&port=5432"
const identityDeclarationNativeLedgerVersion = 96

func identityDeclarationIntentJournalPath(t *testing.T) string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("..", "..", ".local", "testing", "kyc-native", "declaration-test-intent.json"))
	if err != nil {
		t.Fatal("native declaration intent path could not be resolved")
	}
	return path
}

func TestIdentityDeclarationRuntimePostgreSQLSaveRestartReadback(t *testing.T) {
	rawDSN := os.Getenv(identityDeclarationNativeTestDSNEnv)
	cleanupDSN := os.Getenv(identityDeclarationNativeCleanupDSNEnv)
	accountID := os.Getenv(identityDeclarationNativeTestAccountEnv)
	otherAccountID := os.Getenv(identityDeclarationNativeOtherAccountEnv)
	if rawDSN == "" && cleanupDSN == "" && accountID == "" && otherAccountID == "" {
		t.Skip("native KYC integration requires an explicitly authorized local synthetic account")
	}
	if rawDSN == "" || cleanupDSN == "" || accountID == "" || otherAccountID == "" {
		t.Fatal("native KYC integration requires the exact local app/cleanup DSNs and two authorized account IDs")
	}
	if err := validateIdentityDeclarationRuntimeDSN(rawDSN); err != nil {
		t.Fatal(err)
	}
	if err := validateIdentityDeclarationCleanupDSN(cleanupDSN); err != nil {
		t.Fatal(err)
	}
	parsedID, err := uuid.Parse(accountID)
	if err != nil || parsedID.String() != accountID || !canonicalNativeUUID(otherAccountID) || otherAccountID == accountID {
		t.Fatal("authorized KYC isolation account IDs must be canonical and distinct")
	}
	policyPath, err := filepath.Abs(filepath.Join("..", "..", ".local", "testing", "test-account-policy.json"))
	if err != nil {
		t.Fatal("native KYC policy path could not be resolved")
	}
	if err := validateNativeIdentityPrivateFile(policyPath); err != nil {
		t.Fatal(err)
	}
	policyBytes, err := os.ReadFile(policyPath)
	if err != nil {
		t.Fatal("native KYC private policy could not be loaded")
	}
	if err := validateNativeIdentityAccountPair(policyBytes, accountID, otherAccountID); err != nil {
		t.Fatal(err)
	}
	appPassfile, err := filepath.Abs(filepath.Join("..", "..", ".local", "testing", "kyc-native", "api.pgpass"))
	if err != nil {
		t.Fatal("native KYC app passfile path could not be resolved")
	}
	if _, err := parseNativeIdentityConfig(rawDSN, appPassfile); err != nil {
		t.Fatal(err)
	}
	cleanupConfig, err := parseNativeIdentityConfig(cleanupDSN, "/home/nandev/.local/share/espacigo-native-secrets/pgpass")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	sourceMigrations, err := migrator.DiscoverMigrations(filepath.Join("..", "..", "db", "migrations"))
	if err != nil || len(sourceMigrations) < identityDeclarationNativeLedgerVersion {
		t.Fatal("native KYC source migration inventory is incomplete or unreadable")
	}
	if err := readNativeIdentityPreflight(ctx, sourceMigrations, identityDeclarationNativeLedgerVersion); err != nil {
		t.Fatal(err)
	}
	if sourceMigrations[94-1].Version != 94 || sourceMigrations[94-1].Checksum != identityDeclarationNativeV94Checksum {
		t.Fatal("native KYC V94 checksum differs from its pinned reviewed value")
	}

	cleanupConfig.MaxConns = 1
	cleanupConfig.AfterConnect = func(connectCtx context.Context, conn *pgx.Conn) error {
		if _, roleErr := conn.Exec(connectCtx, `SET ROLE espacigo_schema_owner`); roleErr != nil {
			return roleErr
		}
		var session, current string
		if roleErr := conn.QueryRow(connectCtx, `SELECT session_user, current_user`).Scan(&session, &current); roleErr != nil {
			return roleErr
		}
		if session != "espacigo_migrator_login" || current != "espacigo_schema_owner" {
			return errors.New("native KYC cleanup role identity mismatch")
		}
		return nil
	}
	cleanupPool, err := pgxpool.NewWithConfig(ctx, cleanupConfig)
	if err != nil {
		t.Fatal("native KYC cleanup-role pool could not be opened")
	}
	t.Cleanup(cleanupPool.Close)
	if err := cleanupPool.QueryRow(ctx, `SELECT 1`).Scan(new(int)); err != nil {
		t.Fatal("native KYC authorized cleanup role could not execute a query")
	}
	var cleanupAuthorized bool
	if err := cleanupPool.QueryRow(ctx, `SELECT has_table_privilege(current_user,'public.evento_auditoria_local','DELETE') AND has_schema_privilege(current_user,'public','CREATE') AND bool_and(has_column_privilege(current_user,'public.evento_auditoria_local',c,'SELECT')) AND has_column_privilege(current_user,'public.outbox_evento_local','clave_deduplicacion','SELECT') AND has_table_privilege(current_user,'public.outbox_evento_local','DELETE') FROM unnest(ARRAY['actor_id','recurso_tipo','recurso_id','accion','correlacion_id']) AS cols(c)`).Scan(&cleanupAuthorized); err != nil || !cleanupAuthorized {
		t.Fatal("native KYC integration requires prepared temporary fixture-cleanup authority; no declaration written")
	}

	newPool := func() *pgxpool.Pool {
		config, configErr := parseNativeIdentityConfig(rawDSN, appPassfile)
		if configErr != nil {
			t.Fatal("native KYC runtime DSN could not be parsed")
		}
		config.MaxConns = 1
		config.AfterConnect = func(connectCtx context.Context, conn *pgx.Conn) error {
			if _, roleErr := conn.Exec(connectCtx, `SET ROLE espacigo_runtime`); roleErr != nil {
				return roleErr
			}
			var session, current string
			if roleErr := conn.QueryRow(connectCtx, `SELECT session_user, current_user`).Scan(&session, &current); roleErr != nil {
				return roleErr
			}
			if session != "espacigo_api_login" || current != "espacigo_runtime" {
				return errors.New("native KYC runtime role identity mismatch")
			}
			return nil
		}
		pool, poolErr := pgxpool.NewWithConfig(ctx, config)
		if poolErr != nil {
			t.Fatal("native KYC runtime-role pool could not be opened")
		}
		return pool
	}
	appPool := newPool()
	t.Cleanup(appPool.Close)
	restartedPool := newPool()
	t.Cleanup(restartedPool.Close)
	if err := appPool.QueryRow(ctx, `SELECT 1`).Scan(new(int)); err != nil {
		t.Fatal("native KYC runtime role could not execute a query")
	}
	accountPreflight, err := appPool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		t.Fatal("native KYC account read-only preflight could not start")
	}
	defer accountPreflight.Rollback(ctx)
	for _, fixtureID := range []string{accountID, otherAccountID} {
		var accountExists bool
		if err := accountPreflight.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM public.usuario WHERE id=$1::uuid AND estado='activo')`, fixtureID).Scan(&accountExists); err != nil || !accountExists {
			t.Fatal("authorized synthetic KYC account is absent, inactive or unreadable under the runtime role")
		}
	}
	var primaryHasRevision bool
	if err := accountPreflight.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM public.identity_declaration_revision WHERE account_id=$1::uuid)`, accountID).Scan(&primaryHasRevision); err != nil || primaryHasRevision {
		t.Fatal("primary fixture must have no prior declaration; preserve its existing data")
	}
	var secondaryHasRevision bool
	if err := accountPreflight.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM public.identity_declaration_revision WHERE account_id=$1::uuid)`, otherAccountID).Scan(&secondaryHasRevision); err != nil || secondaryHasRevision {
		t.Fatal("secondary fixture must be readable and have no prior declaration; preserve its data")
	}
	if err := accountPreflight.Rollback(ctx); err != nil {
		t.Fatal("native KYC account read-only preflight could not close")
	}

	keyPath := filepath.Join(t.TempDir(), "identity-keyring.json")
	key := hex.EncodeToString([]byte("12345678901234567890123456789012"))
	writeKeyring(t, keyPath, "v1", map[string]string{"v1": key})
	values := map[string]string{"IDENTITY_DECLARATION_KEYRING_FILE": keyPath}
	trackedRepo := &nativeDeclarationIntentRepository{delegate: identitydeclaration.New(appPool), journalPath: identityDeclarationIntentJournalPath(t)}
	firstHandler := runtimeHandlerForAccount(t, values, trackedRepo, accountID)
	secondHandler := runtimeHandlerForAccount(t, values, identitydeclaration.New(restartedPool), accountID)
	otherPool := newPool()
	t.Cleanup(otherPool.Close)
	otherHandler := runtimeHandlerForAccount(t, values, identitydeclaration.New(otherPool), otherAccountID)
	journalPath := identityDeclarationIntentJournalPath(t)
	if _, err := os.Lstat(journalPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("native KYC intent journal already exists or cannot be checked; preserve it and do not POST")
	}
	t.Cleanup(func() {
		if !trackedRepo.intentWritten {
			return
		}
		journalPath := identityDeclarationIntentJournalPath(t)
		if err := validateNativeIdentityPrivateFile(journalPath); err != nil {
			t.Errorf("native KYC intent journal metadata is invalid or missing")
			return
		}
		intentBytes, readErr := os.ReadFile(journalPath)
		if readErr != nil {
			t.Errorf("native KYC intent journal could not be read: %v", readErr)
			return
		}
		var intent nativeDeclarationIntent
		if err := json.Unmarshal(intentBytes, &intent); err != nil {
			t.Errorf("native KYC intent journal is invalid: %v", err)
			return
		}
		if intent.AccountID != accountID || intent != trackedRepo.trustedIntent {
			t.Errorf("native KYC journal differs from the trusted write intent")
			return
		}
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cleanupCancel()
		if err := cleanupNativeIdentityDeclarationFixture(cleanupCtx, cleanupPool, intent.AccountID, intent.CorrelationID, intent.DeclarationID); err != nil {
			t.Errorf("native KYC synthetic fixture cleanup failed: %v", err)
			return
		}
		if err := os.Remove(journalPath); err != nil {
			t.Errorf("native KYC intent journal could not be retired after cleanup: %v", err)
			return
		}
		dir, err := os.Open(filepath.Dir(journalPath))
		if err == nil {
			err = dir.Sync()
			_ = dir.Close()
		}
		if err != nil {
			t.Errorf("native KYC intent journal removal could not be synced: %v", err)
		}
	})

	body := `{"first_name":"Synthetic","last_name":"Person","date_of_birth":"1990-01-02","rut":"11.222.333-9","id_country":"CHL","document_type":"ID","consent_given":true,"consent_version":"id-consent-20261108"}`
	savedResponse := requestRuntime(firstHandler, http.MethodPost, body)
	if savedResponse.Code != http.StatusCreated {
		t.Fatalf("native KYC save returned status %d", savedResponse.Code)
	}
	var saved verificationhttp.IdentityDeclarationResponse
	if err := json.Unmarshal(savedResponse.Body.Bytes(), &saved); err != nil {
		t.Fatal("native KYC save response was not valid JSON")
	}
	if saved.ID == "" || saved.Revision < 1 || saved.FirstName != "Synthetic" || saved.LastName != "Person" || saved.RUT != "11222333-9" {
		t.Fatal("native KYC save response did not match the synthetic declaration")
	}
	var ciphertext []byte
	if err := appPool.QueryRow(ctx, `SELECT ciphertext FROM public.identity_declaration_revision WHERE id=$1::uuid AND account_id=$2::uuid`, saved.ID, accountID).Scan(&ciphertext); err != nil {
		t.Fatal("native KYC encrypted declaration was not persisted")
	}
	if len(ciphertext) < 29 || bytes.Contains(ciphertext, []byte("Synthetic")) || bytes.Contains(ciphertext, []byte("11222333-9")) {
		t.Fatal("native KYC database row did not contain opaque encrypted payload")
	}

	readback := requestRuntime(secondHandler, http.MethodGet, "")
	if readback.Code != http.StatusOK {
		t.Fatalf("native KYC restart readback returned status %d", readback.Code)
	}
	var loaded verificationhttp.IdentityDeclarationResponse
	if err := json.Unmarshal(readback.Body.Bytes(), &loaded); err != nil {
		t.Fatal("native KYC restart readback was not valid JSON")
	}
	if loaded.ID != saved.ID || loaded.Revision != saved.Revision || loaded.FirstName != saved.FirstName || loaded.LastName != saved.LastName || loaded.DateOfBirth != saved.DateOfBirth || loaded.RUT != saved.RUT || loaded.ConsentVersion != saved.ConsentVersion {
		t.Fatal("native KYC restart readback differs from the persisted declaration")
	}
	crossRead := requestRuntime(otherHandler, http.MethodGet, "")
	if crossRead.Code != http.StatusNotFound {
		t.Fatalf("native KYC cross-account read returned status %d, expected 404", crossRead.Code)
	}
	for _, privateField := range []string{saved.ID, saved.FirstName, saved.LastName, saved.DateOfBirth, saved.RUT, "11.222.333-9"} {
		if privateField != "" && bytes.Contains(crossRead.Body.Bytes(), []byte(privateField)) {
			t.Fatal("native KYC cross-account response leaked primary declaration data")
		}
	}
	keptResponse := requestRuntime(secondHandler, http.MethodGet, "")
	var kept verificationhttp.IdentityDeclarationResponse
	if keptResponse.Code != http.StatusOK || json.Unmarshal(keptResponse.Body.Bytes(), &kept) != nil || kept != loaded {
		t.Fatal("primary declaration changed or disappeared after cross-account read")
	}
	runNativeN06AtomicRecoveryTest(t, ctx, appPool, cleanupPool, newPool, values, accountID, saved.ID)
}

func cleanupNativeIdentityDeclarationFixture(ctx context.Context, pool *pgxpool.Pool, accountID, correlationID, intentDeclarationID string) error {
	if pool == nil || accountID == "" || correlationID == "" || intentDeclarationID == "" {
		return errors.New("native KYC cleanup requires a pool and complete trusted intent")
	}
	if !canonicalNativeUUID(accountID) || !canonicalNativeUUID(intentDeclarationID) || strings.TrimSpace(correlationID) != correlationID || len(correlationID) == 0 || len(correlationID) > 120 {
		return errors.New("native KYC cleanup intent is not canonical")
	}
	var sessionUser, currentUser string
	if err := pool.QueryRow(ctx, `SELECT session_user, current_user`).Scan(&sessionUser, &currentUser); err != nil {
		return err
	}
	if sessionUser != "espacigo_migrator_login" || currentUser != "espacigo_schema_owner" {
		return errors.New("native KYC fixture cleanup requires the authorized migrator principal and schema-owner role")
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var revisionAccountID string
	revisionErr := tx.QueryRow(ctx, `SELECT account_id::text FROM public.identity_declaration_revision WHERE id=$1::uuid FOR UPDATE`, intentDeclarationID).Scan(&revisionAccountID)
	if revisionErr != nil && !errors.Is(revisionErr, pgx.ErrNoRows) {
		return revisionErr
	}
	if revisionErr == nil && revisionAccountID != accountID {
		return errors.New("native KYC trusted declaration ID belongs to a different account")
	}
	// Audit is immutable for this path; locking the owned revision is enough.
	// FOR UPDATE on audit would require widening the temporary cleanup ACL.
	rows, err := tx.Query(ctx, `SELECT actor_id::text, recurso_tipo, recurso_id::text, accion, correlacion_id
 FROM public.evento_auditoria_local
 WHERE correlacion_id=$1
    OR (actor_id=$2::uuid AND recurso_tipo='identity_declaration' AND recurso_id=$3::uuid)`, correlationID, accountID, intentDeclarationID)
	if err != nil {
		return err
	}
	var auditIDs []string
	for rows.Next() {
		var actorID, resourceType, resourceID, action, auditCorrelationID string
		if err := rows.Scan(&actorID, &resourceType, &resourceID, &action, &auditCorrelationID); err != nil {
			rows.Close()
			return err
		}
		if actorID != accountID || resourceType != "identity_declaration" || action != "kyc.identity_declaration.save" || resourceID != intentDeclarationID || auditCorrelationID != correlationID {
			rows.Close()
			return errors.New("native KYC correlation audit does not match the trusted intent scope")
		}
		auditIDs = append(auditIDs, resourceID)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	if len(auditIDs) > 1 {
		return errors.New("native KYC correlation matched multiple declaration rows")
	}
	if errors.Is(revisionErr, pgx.ErrNoRows) {
		if len(auditIDs) == 0 {
			var artifacts nativeKYCNoRevisionArtifactCounts
			if err := tx.QueryRow(ctx, `SELECT
 (SELECT count(*) FROM public.identity_declaration_revision WHERE account_id=$1::uuid),
 (SELECT count(*) FROM public.identity_provider_attempt WHERE account_id=$1::uuid),
 (SELECT count(*) FROM public.identity_provider_event e JOIN public.identity_provider_attempt a ON a.id=e.attempt_id WHERE a.account_id=$1::uuid),
 (SELECT count(*) FROM public.identity_rut_claim WHERE account_id=$1::uuid),
 (SELECT count(*) FROM public.evento_auditoria_local WHERE actor_id=$1::uuid AND recurso_tipo='identity_declaration'),
 (SELECT count(*) FROM public.outbox_evento_local o WHERE o.agregado_tipo='didit_session' AND o.tipo='didit.session_remote_deletion' AND EXISTS (SELECT 1 FROM public.identity_provider_attempt a WHERE a.account_id=$1::uuid AND (o.agregado_id=a.id OR o.clave_deduplicacion='didit_session_deletion:' || a.provider_session_id)))`, accountID).Scan(
				&artifacts.Revisions, &artifacts.ProviderAttempts, &artifacts.ProviderEvents,
				&artifacts.RutClaims, &artifacts.DeclarationAudit, &artifacts.FixtureOutbox,
			); err != nil {
				return err
			}
			if nativeKYCNoRevisionHasAnyArtifact(artifacts) {
				return errors.New("native KYC artifacts exist without the trusted declaration and audit; preserve for read-only reconciliation")
			}
			return tx.Commit(ctx)
		}
	}
	if revisionErr == nil && len(auditIDs) == 1 && auditIDs[0] != intentDeclarationID {
		return errors.New("native KYC trusted intent, revision and audit identify different declarations")
	}
	attemptRows, err := tx.Query(ctx, `SELECT id::text, COALESCE(provider_session_id,'') FROM public.identity_provider_attempt WHERE declaration_id=$1::uuid AND account_id=$2::uuid ORDER BY id`, intentDeclarationID, accountID)
	if err != nil {
		return err
	}
	attemptIDs := make([]string, 0)
	sessionKeys := make([]string, 0)
	for attemptRows.Next() {
		var attemptID, sessionID string
		if err := attemptRows.Scan(&attemptID, &sessionID); err != nil {
			attemptRows.Close()
			return err
		}
		if !canonicalNativeUUID(attemptID) {
			attemptRows.Close()
			return errors.New("native KYC attempt ID is not canonical; preserve cleanup journal")
		}
		attemptIDs = append(attemptIDs, attemptID)
		if sessionID != "" {
			key := "didit_session_deletion:" + sessionID
			if strings.TrimSpace(sessionID) != sessionID || len(key) > 120 {
				attemptRows.Close()
				return errors.New("native KYC provider session key is not canonical; preserve cleanup journal")
			}
			sessionKeys = append(sessionKeys, key)
		}
	}
	if err := attemptRows.Err(); err != nil {
		attemptRows.Close()
		return err
	}
	attemptRows.Close()
	var unexpectedOutbox int64
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM public.outbox_evento_local o
 WHERE (o.agregado_id::text=ANY($1::text[]) OR o.clave_deduplicacion=ANY($2::text[]))
   AND NOT (o.agregado_tipo='didit_session' AND o.tipo='didit.session_remote_deletion' AND o.clave_deduplicacion=ANY($2::text[]))`, attemptIDs, sessionKeys).Scan(&unexpectedOutbox); err != nil {
		return err
	}
	if unexpectedOutbox != 0 {
		return errors.New("native KYC outbox rows do not match the trusted attempt/session scope; preserve journal")
	}
	if _, err := tx.Exec(ctx, `DELETE FROM public.outbox_evento_local WHERE agregado_tipo='didit_session' AND tipo='didit.session_remote_deletion' AND clave_deduplicacion=ANY($1::text[])`, sessionKeys); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM public.identity_provider_event WHERE attempt_id IN (SELECT id FROM public.identity_provider_attempt WHERE declaration_id=$1::uuid AND account_id=$2::uuid)`, intentDeclarationID, accountID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM public.identity_provider_attempt WHERE declaration_id=$1::uuid AND account_id=$2::uuid`, intentDeclarationID, accountID); err != nil {
		return err
	}
	if len(auditIDs) == 1 {
		if _, err := tx.Exec(ctx, `DELETE FROM public.evento_auditoria_local WHERE actor_id=$1::uuid AND recurso_tipo='identity_declaration' AND recurso_id=$2::uuid AND accion='kyc.identity_declaration.save' AND correlacion_id=$3`, accountID, intentDeclarationID, correlationID); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, `DELETE FROM public.identity_rut_claim WHERE declaration_id=$1::uuid AND account_id=$2::uuid`, intentDeclarationID, accountID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM public.identity_declaration_revision WHERE id=$1::uuid AND account_id=$2::uuid`, intentDeclarationID, accountID); err != nil {
		return err
	}
	var remainingEvents, remainingAttempts, remainingClaims, remainingRevisions, remainingAudit, remainingOutbox int64
	if err := tx.QueryRow(ctx, `SELECT (SELECT count(*) FROM public.identity_provider_event e JOIN public.identity_provider_attempt a ON a.id=e.attempt_id WHERE a.account_id=$1::uuid),(SELECT count(*) FROM public.identity_provider_attempt WHERE account_id=$1::uuid),(SELECT count(*) FROM public.identity_rut_claim WHERE account_id=$1::uuid),(SELECT count(*) FROM public.identity_declaration_revision WHERE account_id=$1::uuid),(SELECT count(*) FROM public.evento_auditoria_local WHERE actor_id=$1::uuid AND recurso_tipo='identity_declaration'),(SELECT count(*) FROM public.outbox_evento_local WHERE agregado_id::text=ANY($2::text[]) OR clave_deduplicacion=ANY($3::text[]))`, accountID, attemptIDs, sessionKeys).Scan(&remainingEvents, &remainingAttempts, &remainingClaims, &remainingRevisions, &remainingAudit, &remainingOutbox); err != nil {
		return err
	}
	if remainingEvents+remainingAttempts+remainingClaims+remainingRevisions+remainingAudit+remainingOutbox != 0 {
		return errors.New("native KYC child, claim or declaration fixture remained after cleanup")
	}
	return tx.Commit(ctx)
}
