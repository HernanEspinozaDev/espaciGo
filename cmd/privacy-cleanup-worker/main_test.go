package main

import (
	"bytes"
	"context"
	"errors"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadConfigAllowsOnlyTheDedicatedDatabaseRole(t *testing.T) {
	dir := t.TempDir()
	passwordPath := filepath.Join(dir, "password")
	if err := os.WriteFile(passwordPath, []byte("synthetic"), 0o600); err != nil {
		t.Fatal(err)
	}
	values := map[string]string{
		"DATABASE_HOST": "127.0.0.1", "DATABASE_NAME": "espacigo", "DATABASE_USER": "worker_login",
		"DATABASE_PASSWORD_FILE": passwordPath, "BLOB_STORE_BACKEND": "filesystem", "M03_EVIDENCE_DIR": dir,
	}
	getenv := func(key string) string { return values[key] }
	values["DATABASE_SET_ROLE"] = "espacigo_privacy_cleanup_worker"
	if _, err := loadConfig(getenv); err != nil {
		t.Fatalf("dedicated worker role rejected: %v", err)
	}
	values["DATABASE_SET_ROLE"] = "espacigo_runtime"
	if _, err := loadConfig(getenv); err == nil {
		t.Fatal("broad application role was accepted")
	}
}

func TestLoadConfigRejectsInvalidOrUnsafeOptions(t *testing.T) {
	dir := t.TempDir()
	passwordPath := filepath.Join(dir, "password")
	if err := os.WriteFile(passwordPath, []byte("synthetic-password"), 0o600); err != nil {
		t.Fatal(err)
	}
	base := func() map[string]string {
		return map[string]string{
			"DATABASE_HOST":          "127.0.0.1",
			"DATABASE_NAME":          "espacigo",
			"DATABASE_USER":          "worker_login",
			"DATABASE_PASSWORD_FILE": passwordPath,
			"DATABASE_SET_ROLE":      "espacigo_privacy_cleanup_worker",
			"BLOB_STORE_BACKEND":     "filesystem",
			"M03_EVIDENCE_DIR":       dir,
		}
	}

	tests := []struct {
		name        string
		modify      func(m map[string]string)
		errContains string
	}{
		{
			name:        "invalid connection mode",
			modify:      func(m map[string]string) { m["DATABASE_CONNECTION_MODE"] = "proxy" },
			errContains: "invalid database connection mode",
		},
		{
			name:        "direct mode missing host",
			modify:      func(m map[string]string) { m["DATABASE_HOST"] = "" },
			errContains: "database host is required",
		},
		{
			name:        "invalid port non-numeric",
			modify:      func(m map[string]string) { m["DATABASE_PORT"] = "not-a-port" },
			errContains: "invalid database port",
		},
		{
			name:        "invalid port zero",
			modify:      func(m map[string]string) { m["DATABASE_PORT"] = "0" },
			errContains: "invalid database port",
		},
		{
			name:        "missing database name",
			modify:      func(m map[string]string) { m["DATABASE_NAME"] = "" },
			errContains: "database name, user and password file are required",
		},
		{
			name:        "missing database user",
			modify:      func(m map[string]string) { m["DATABASE_USER"] = "" },
			errContains: "database name, user and password file are required",
		},
		{
			name:        "missing password file",
			modify:      func(m map[string]string) { m["DATABASE_PASSWORD_FILE"] = "" },
			errContains: "database name, user and password file are required",
		},
		{
			name:        "nonexistent password file",
			modify:      func(m map[string]string) { m["DATABASE_PASSWORD_FILE"] = filepath.Join(dir, "nonexistent") },
			errContains: "database password file is unavailable",
		},
		{
			name: "empty password file",
			modify: func(m map[string]string) {
				emptyFile := filepath.Join(dir, "empty")
				_ = os.WriteFile(emptyFile, []byte("   \n"), 0o600)
				m["DATABASE_PASSWORD_FILE"] = emptyFile
			},
			errContains: "database password file is empty",
		},
		{
			name:        "invalid SSL mode",
			modify:      func(m map[string]string) { m["DATABASE_SSLMODE"] = "trust-all" },
			errContains: "invalid database SSL mode",
		},
		{
			name:        "cert without key",
			modify:      func(m map[string]string) { m["DATABASE_SSLCERT"] = "/some/cert.pem"; m["DATABASE_SSLKEY"] = "" },
			errContains: "database client certificate and key must be configured together",
		},
		{
			name:        "key without cert",
			modify:      func(m map[string]string) { m["DATABASE_SSLCERT"] = ""; m["DATABASE_SSLKEY"] = "/some/key.pem" },
			errContains: "database client certificate and key must be configured together",
		},
		{
			name:        "invalid storage backend",
			modify:      func(m map[string]string) { m["BLOB_STORE_BACKEND"] = "s3" },
			errContains: "invalid private evidence store backend",
		},
		{
			name:        "gcs backend without bucket",
			modify:      func(m map[string]string) { m["BLOB_STORE_BACKEND"] = "gcs"; m["CLOUD_STORAGE_BUCKET"] = "" },
			errContains: "Cloud Storage bucket is required",
		},
		{
			name:        "filesystem backend without evidence directory",
			modify:      func(m map[string]string) { m["BLOB_STORE_BACKEND"] = "filesystem"; m["M03_EVIDENCE_DIR"] = "" },
			errContains: "private evidence directory is required",
		},
		{
			name:        "cleanup interval too small",
			modify:      func(m map[string]string) { m["PRIVACY_CLEANUP_INTERVAL"] = "500ms" },
			errContains: "privacy cleanup interval must be between 1s and 1h",
		},
		{
			name:        "cleanup interval too large",
			modify:      func(m map[string]string) { m["PRIVACY_CLEANUP_INTERVAL"] = "2h" },
			errContains: "privacy cleanup interval must be between 1s and 1h",
		},
		{
			name:        "cleanup interval unparseable",
			modify:      func(m map[string]string) { m["PRIVACY_CLEANUP_INTERVAL"] = "invalid" },
			errContains: "privacy cleanup interval must be between 1s and 1h",
		},
		{
			name: "cloudsql mode invalid instance name",
			modify: func(m map[string]string) {
				m["DATABASE_CONNECTION_MODE"] = "cloudsql"
				m["CLOUD_SQL_INSTANCE_CONNECTION_NAME"] = "invalid-name"
			},
			errContains: "invalid Cloud SQL instance name",
		},
		{
			name: "cloudsql mode with TLS settings conflicting",
			modify: func(m map[string]string) {
				m["DATABASE_CONNECTION_MODE"] = "cloudsql"
				m["CLOUD_SQL_INSTANCE_CONNECTION_NAME"] = "my-project:us-central1:my-instance"
				m["DATABASE_SSLMODE"] = "require"
			},
			errContains: "Cloud SQL connector cannot be combined with direct TLS settings",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := base()
			tt.modify(m)
			getenv := func(key string) string { return m[key] }
			_, err := loadConfig(getenv)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.errContains)
			}
			if !strings.Contains(err.Error(), tt.errContains) {
				t.Fatalf("expected error containing %q, got %v", tt.errContains, err)
			}
		})
	}
}

type declarationPurgeStub struct {
	calls   int
	deleted int64
	err     error
}

func (s *declarationPurgeStub) PurgeExpiredRevisions(context.Context) (int64, error) {
	s.calls++
	return s.deleted, s.err
}

func TestDeclarationPurgeRunsOnlyOnceWithDedicatedRole(t *testing.T) {
	ctx := context.Background()
	stub := &declarationPurgeStub{deleted: 7}
	deleted, err := purgeIdentityDeclarationBatch(ctx, "", stub)
	if err != nil || deleted != 0 || stub.calls != 0 {
		t.Fatalf("unconfigured role must skip purge: deleted=%d calls=%d err=%v", deleted, stub.calls, err)
	}
	deleted, err = purgeIdentityDeclarationBatch(ctx, "espacigo_privacy_cleanup_worker", stub)
	if err != nil || deleted != 7 || stub.calls != 1 {
		t.Fatalf("dedicated role must invoke one bounded purge: deleted=%d calls=%d err=%v", deleted, stub.calls, err)
	}
}

type ackPurgeStub struct {
	calls   int
	deleted int64
	err     error
}

func (s *ackPurgeStub) PurgeDeliveredSessionOutbox(context.Context) (int64, error) {
	s.calls++
	return s.deleted, s.err
}

func TestDeliveredSessionOutboxPurgeOnlyDedicatedRole(t *testing.T) {
	stub := &ackPurgeStub{deleted: 100}
	for _, role := range []string{"", "espacigo_runtime", "other"} {
		n, err := purgeDeliveredSessionOutbox(context.Background(), role, stub)
		if err != nil || n != 0 || stub.calls != 0 {
			t.Fatalf("role %q: n=%d calls=%d err=%v", role, n, stub.calls, err)
		}
	}
	n, err := purgeDeliveredSessionOutbox(context.Background(), "espacigo_privacy_cleanup_worker", stub)
	if err != nil || n != 100 || stub.calls != 1 {
		t.Fatalf("worker: n=%d calls=%d err=%v", n, stub.calls, err)
	}
}

func TestDeliveredSessionOutboxPurgeFailsClosedAndPropagates(t *testing.T) {
	if _, err := purgeDeliveredSessionOutbox(context.Background(), "espacigo_privacy_cleanup_worker", nil); err == nil {
		t.Fatal("nil purger must fail closed")
	}
	want := errors.New("synthetic ACK purge error")
	stub := &ackPurgeStub{err: want}
	if _, err := purgeDeliveredSessionOutbox(context.Background(), "espacigo_privacy_cleanup_worker", stub); !errors.Is(err, want) || stub.calls != 1 {
		t.Fatalf("calls=%d err=%v", stub.calls, err)
	}
}

func TestDeclarationPurgeReturnsRepositoryErrors(t *testing.T) {
	want := errors.New("synthetic purge error")
	stub := &declarationPurgeStub{err: want}
	if _, err := purgeIdentityDeclarationBatch(context.Background(), "espacigo_privacy_cleanup_worker", stub); !errors.Is(err, want) || stub.calls != 1 {
		t.Fatalf("purge error not propagated: calls=%d err=%v", stub.calls, err)
	}
}

type cycleDecl struct {
	calls int
	ctx   context.Context
	n     int64
	err   error
}

func (s *cycleDecl) PurgeExpiredRevisions(ctx context.Context) (int64, error) {
	s.calls++
	s.ctx = ctx
	return s.n, s.err
}

type cycleAck struct {
	calls int
	ctx   context.Context
	n     int64
	err   error
}

func (s *cycleAck) PurgeDeliveredSessionOutbox(ctx context.Context) (int64, error) {
	s.calls++
	s.ctx = ctx
	return s.n, s.err
}

func TestRetentionCycleIndependentlyRunsBothAndLogsNoErrors(t *testing.T) {
	ctx := context.WithValue(context.Background(), struct{}{}, "forwarded")
	first := errors.New("synthetic secret-ish first failure")
	second := errors.New("synthetic second failure")
	d := &cycleDecl{err: first}
	a := &cycleAck{err: second}
	var out bytes.Buffer
	runIdentityRetentionCycle(ctx, "espacigo_privacy_cleanup_worker", d, a, log.New(&out, "", 0))
	if d.calls != 1 || a.calls != 1 || d.ctx != ctx || a.ctx != ctx {
		t.Fatalf("calls/contexts: d=%+v a=%+v", d, a)
	}
	if strings.Contains(out.String(), first.Error()) || strings.Contains(out.String(), second.Error()) {
		t.Fatalf("error leaked: %q", out.String())
	}
}
func TestRetentionCycleOtherRoleSkipsBoth(t *testing.T) {
	d := &cycleDecl{}
	a := &cycleAck{}
	runIdentityRetentionCycle(context.Background(), "other", d, a, log.New(&bytes.Buffer{}, "", 0))
	if d.calls+a.calls != 0 {
		t.Fatal("purger called")
	}
}
func TestRetentionCycleNilPurgersLogGenericFailures(t *testing.T) {
	var out bytes.Buffer
	runIdentityRetentionCycle(context.Background(), "espacigo_privacy_cleanup_worker", nil, nil, log.New(&out, "", 0))
	if strings.Contains(out.String(), "requires") || strings.Contains(out.String(), "nil") {
		t.Fatalf("non-generic log: %s", out.String())
	}
}
func TestSchedulerImmediateThenTicksAndStops(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	calls := 0
	done := make(chan struct{})
	go func() {
		runScheduler(ctx, 10*time.Millisecond, func() {
			calls++
			if calls == 3 {
				cancel()
			}
		})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("scheduler did not stop")
	}
	if calls != 3 {
		t.Fatalf("cycles=%d", calls)
	}
}
func TestSchedulerCancelledBeforeStartDoesNotCall(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls := 0
	runScheduler(ctx, time.Millisecond, func() { calls++ })
	if calls != 0 {
		t.Fatalf("calls=%d", calls)
	}
}
func TestSchedulerCancellationDuringCallbackDoesNotRepeat(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	entered := make(chan struct{})
	release := make(chan struct{})
	done := make(chan struct{})
	calls := 0
	go func() {
		runScheduler(ctx, time.Millisecond, func() { calls++; close(entered); <-release })
		close(done)
	}()
	<-entered
	cancel()
	close(release)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("scheduler did not stop")
	}
	if calls != 1 {
		t.Fatalf("calls=%d", calls)
	}
}
func TestRetentionCycleFullBatchLogsBacklog(t *testing.T) {
	d := &cycleDecl{n: 100}
	a := &cycleAck{n: 100}
	var out bytes.Buffer
	runIdentityRetentionCycle(context.Background(), "espacigo_privacy_cleanup_worker", d, a, log.New(&out, "", 0))
	if strings.Count(out.String(), "batch limit") != 2 {
		t.Fatalf("log=%q", out.String())
	}
}

func TestRetentionFailureRecoversNextCycleAndACKStillRuns(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	d := &cycleDecl{err: errors.New("synthetic first failure")}
	a := &cycleAck{}
	cycles := 0
	runScheduler(ctx, time.Millisecond, func() {
		cycles++
		if cycles == 2 {
			d.err = nil
			cancel()
		}
		runIdentityRetentionCycle(ctx, "espacigo_privacy_cleanup_worker", d, a, log.New(&bytes.Buffer{}, "", 0))
	})
	if cycles != 2 || d.calls != 2 || a.calls != 2 {
		t.Fatalf("cycles=%d declaration=%d ACK=%d", cycles, d.calls, a.calls)
	}
}
func TestSchedulerInvalidIntervalFailsClosed(t *testing.T) {
	calls := 0
	runScheduler(context.Background(), 0, func() { calls++ })
	if calls != 0 {
		t.Fatalf("calls=%d", calls)
	}
}

func TestWorkerConfigDatabaseURL(t *testing.T) {
	cfg := workerConfig{
		dbUser:        "test_user",
		dbPassword:    "test_pass",
		dbHost:        "127.0.0.1",
		dbPort:        5432,
		dbName:        "espacigo_test",
		dbSSLMode:     "verify-full",
		dbSSLRootCert: "/etc/ssl/root.crt",
		dbSSLCert:     "/etc/ssl/client.crt",
		dbSSLKey:      "/etc/ssl/client.key",
	}

	rawURL := cfg.databaseURL()
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("failed to parse generated database URL: %v", err)
	}

	if parsed.Scheme != "postgres" {
		t.Errorf("scheme = %q, want postgres", parsed.Scheme)
	}
	if parsed.User.Username() != "test_user" {
		t.Errorf("username = %q, want test_user", parsed.User.Username())
	}
	pass, _ := parsed.User.Password()
	if pass != "test_pass" {
		t.Errorf("password = %q, want test_pass", pass)
	}
	if parsed.Host != "127.0.0.1:5432" {
		t.Errorf("host = %q, want 127.0.0.1:5432", parsed.Host)
	}
	if parsed.Path != "/espacigo_test" {
		t.Errorf("path = %q, want /espacigo_test", parsed.Path)
	}
	query := parsed.Query()
	if query.Get("sslmode") != "verify-full" {
		t.Errorf("sslmode = %q, want verify-full", query.Get("sslmode"))
	}
	if query.Get("sslrootcert") != "/etc/ssl/root.crt" {
		t.Errorf("sslrootcert = %q, want /etc/ssl/root.crt", query.Get("sslrootcert"))
	}
	if query.Get("sslcert") != "/etc/ssl/client.crt" {
		t.Errorf("sslcert = %q, want /etc/ssl/client.crt", query.Get("sslcert"))
	}
	if query.Get("sslkey") != "/etc/ssl/client.key" {
		t.Errorf("sslkey = %q, want /etc/ssl/client.key", query.Get("sslkey"))
	}
}

type mockSyntheticEvidenceCleaner struct {
	deletedID   string
	err         error
	hasDeadline bool
	deadline    time.Time
}

func (m *mockSyntheticEvidenceCleaner) Delete(ctx context.Context, id string) error {
	m.deletedID = id
	m.deadline, m.hasDeadline = ctx.Deadline()
	return m.err
}

func TestTimedCleanerDelete(t *testing.T) {
	mock := &mockSyntheticEvidenceCleaner{}
	cleaner := timedCleaner{store: mock}

	err := cleaner.Delete(context.Background(), "test-evidence-id-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mock.deletedID != "test-evidence-id-123" {
		t.Errorf("deletedID = %q, want test-evidence-id-123", mock.deletedID)
	}
	if !mock.hasDeadline {
		t.Error("expected deadline to be set on context")
	}
	remaining := time.Until(mock.deadline)
	if remaining <= 0 || remaining > maxDeleteDuration {
		t.Errorf("expected deadline bounded by maxDeleteDuration, got remaining=%v", remaining)
	}

	sentinel := errors.New("synthetic cleaner delete error")
	mockErr := &mockSyntheticEvidenceCleaner{err: sentinel}
	cleanerErr := timedCleaner{store: mockErr}
	err = cleanerErr.Delete(context.Background(), "test-evidence-id-456")
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected error %v, got %v", sentinel, err)
	}
}

func TestNewCleanerAndMediaStoreFilesystem(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	cfg := workerConfig{
		storageBackend: "filesystem",
		evidenceRoot:   dir,
	}

	cleaner, err := newCleaner(context.Background(), cfg)
	if err != nil {
		t.Fatalf("newCleaner filesystem error: %v", err)
	}
	if cleaner == nil {
		t.Fatal("expected non-nil cleaner")
	}

	mediaStore, err := newMediaStore(context.Background(), cfg)
	if err != nil {
		t.Fatalf("newMediaStore filesystem error: %v", err)
	}
	if mediaStore == nil {
		t.Fatal("expected non-nil media store")
	}

	// Test rejection of non-private evidence directory
	nonPrivateDir := t.TempDir()
	_ = os.Chmod(nonPrivateDir, 0o777)
	badPermCfg := workerConfig{storageBackend: "filesystem", evidenceRoot: nonPrivateDir}
	if _, err := newCleaner(context.Background(), badPermCfg); err == nil {
		t.Fatal("expected error for non-private directory in newCleaner")
	}

	badCfg := workerConfig{storageBackend: "unknown"}
	if _, err := newCleaner(context.Background(), badCfg); err == nil {
		t.Fatal("expected error for invalid backend in newCleaner")
	}
	if _, err := newMediaStore(context.Background(), badCfg); err == nil {
		t.Fatal("expected error for invalid backend in newMediaStore")
	}
}
