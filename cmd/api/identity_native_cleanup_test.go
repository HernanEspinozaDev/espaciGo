package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/verification"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type nativeDeclarationIntent struct {
	AccountID     string `json:"account_id"`
	DeclarationID string `json:"declaration_id"`
	CorrelationID string `json:"correlation_id"`
}
type nativeDeclarationIntentRepository struct {
	delegate      verification.DeclarationRevisionRepository
	journalPath   string
	intentWritten bool
	trustedIntent nativeDeclarationIntent
}

func (r *nativeDeclarationIntentRepository) CreateRevision(ctx context.Context, in verification.DeclarationRevisionInput, seal verification.DeclarationSealer) (verification.DeclarationRevision, error) {
	if r.delegate == nil || !canonicalNativeUUID(in.AccountID) || !canonicalNativeUUID(in.ID) || in.CorrelationID == "" || strings.TrimSpace(in.CorrelationID) != in.CorrelationID || len(in.CorrelationID) > 120 {
		return verification.DeclarationRevision{}, errors.New("native declaration intent inputs are not canonical")
	}
	_, err := r.delegate.GetRevision(ctx, in.AccountID, in.ID)
	if err == nil {
		return verification.DeclarationRevision{}, errors.New("native declaration ID already exists for account")
	}
	if !errors.Is(err, verification.ErrNotFound) {
		return verification.DeclarationRevision{}, fmt.Errorf("native declaration ID preflight failed: %w", err)
	}
	intent := nativeDeclarationIntent{AccountID: in.AccountID, DeclarationID: in.ID, CorrelationID: in.CorrelationID}
	if err := persistNativeDeclarationIntent(r.journalPath, intent); err != nil {
		return verification.DeclarationRevision{}, err
	}
	r.intentWritten = true
	r.trustedIntent = intent
	return r.delegate.CreateRevision(ctx, in, seal)
}
func (r nativeDeclarationIntentRepository) GetRevision(ctx context.Context, accountID, id string) (verification.DeclarationRevision, error) {
	return r.delegate.GetRevision(ctx, accountID, id)
}
func (r nativeDeclarationIntentRepository) GetCurrent(ctx context.Context, accountID string) (verification.DeclarationRevision, error) {
	return r.delegate.GetCurrent(ctx, accountID)
}
func canonicalNativeUUID(value string) bool {
	id, err := uuid.Parse(value)
	return err == nil && id.String() == value
}
func persistNativeDeclarationIntent(path string, intent nativeDeclarationIntent) error {
	dir := filepath.Dir(path)
	if !filepath.IsAbs(path) {
		return errors.New("native declaration intent path must be absolute")
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	owner, ok := info.Sys().(*syscall.Stat_t)
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0700 || !ok || owner.Uid != uint32(os.Getuid()) {
		return errors.New("native declaration intent directory must be operator-owned mode 0700 without symlinks")
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return fmt.Errorf("create native declaration intent without overwrite: %w", err)
	}
	if err := file.Chmod(0600); err != nil {
		_ = file.Close()
		return err
	}
	if err := json.NewEncoder(file).Encode(intent); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	dirFile, err := os.Open(dir)
	if err != nil {
		return err
	}
	if err := dirFile.Sync(); err != nil {
		_ = dirFile.Close()
		return err
	}
	return dirFile.Close()
}

type intentBoundaryRepo struct {
	rows        map[string]verification.DeclarationRevision
	writeErr    error
	writes      int
	beforeWrite func() error
}

func (r *intentBoundaryRepo) CreateRevision(_ context.Context, in verification.DeclarationRevisionInput, _ verification.DeclarationSealer) (verification.DeclarationRevision, error) {
	if r.beforeWrite != nil {
		if err := r.beforeWrite(); err != nil {
			return verification.DeclarationRevision{}, err
		}
	}
	r.writes++
	if r.writeErr != nil {
		return verification.DeclarationRevision{}, r.writeErr
	}
	row := verification.DeclarationRevision{ID: in.ID, AccountID: in.AccountID}
	r.rows[in.ID] = row
	return row, nil
}
func (r *intentBoundaryRepo) GetRevision(_ context.Context, accountID, id string) (verification.DeclarationRevision, error) {
	row, ok := r.rows[id]
	if !ok || row.AccountID != accountID {
		return verification.DeclarationRevision{}, verification.ErrNotFound
	}
	return row, nil
}
func (r *intentBoundaryRepo) GetCurrent(context.Context, string) (verification.DeclarationRevision, error) {
	return verification.DeclarationRevision{}, verification.ErrNotFound
}
func nativeIntentInput() verification.DeclarationRevisionInput {
	return verification.DeclarationRevisionInput{ID: "123e4567-e89b-42d3-a456-426614174001", AccountID: "123e4567-e89b-42d3-a456-426614174000", CorrelationID: "trusted-correlation"}
}
func nativeIntentPath(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "private")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(dir, "intent.json")
}
func TestNativeDeclarationTrackedRepositoryPersistsIntentBeforeWrite(t *testing.T) {
	repo := &intentBoundaryRepo{rows: map[string]verification.DeclarationRevision{}}
	tracked := nativeDeclarationIntentRepository{delegate: repo, journalPath: nativeIntentPath(t)}
	in := nativeIntentInput()
	repo.beforeWrite = func() error {
		if err := validateNativeIdentityPrivateFile(tracked.journalPath); err != nil {
			return errors.New("private intent missing at database write boundary")
		}
		b, err := os.ReadFile(tracked.journalPath)
		if err != nil {
			return err
		}
		var got nativeDeclarationIntent
		if json.Unmarshal(b, &got) != nil || got.AccountID != in.AccountID || got.DeclarationID != in.ID || got.CorrelationID != in.CorrelationID {
			return errors.New("trusted intent not durable before delegate write")
		}
		return nil
	}
	if _, err := tracked.CreateRevision(context.Background(), in, nil); err != nil {
		t.Fatal(err)
	}
	if repo.writes != 1 {
		t.Fatalf("writes=%d", repo.writes)
	}
	b, err := os.ReadFile(tracked.journalPath)
	if err != nil {
		t.Fatal(err)
	}
	var got nativeDeclarationIntent
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.AccountID != in.AccountID || got.DeclarationID != in.ID || got.CorrelationID != in.CorrelationID {
		t.Fatalf("intent mismatch: %+v", got)
	}
	info, err := os.Stat(tracked.journalPath)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("journal mode/stat: %v %v", info, err)
	}
}
func TestNativeDeclarationTrackedRepositoryNeverWritesWithoutJournal(t *testing.T) {
	for _, tc := range []struct {
		name     string
		existing bool
	}{{"occupied journal", true}, {"journal persistence error", false}} {
		t.Run(tc.name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "private")
			if err := os.Mkdir(root, 0700); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(root, "intent.json")
			if tc.existing {
				if err := os.WriteFile(path, []byte("sentinel"), 0600); err != nil {
					t.Fatal(err)
				}
			} else {
				blocker := filepath.Join(root, "not-a-directory")
				if err := os.WriteFile(blocker, []byte("x"), 0600); err != nil {
					t.Fatal(err)
				}
				path = filepath.Join(blocker, "intent.json")
			}
			repo := &intentBoundaryRepo{rows: map[string]verification.DeclarationRevision{}}
			tracked := nativeDeclarationIntentRepository{delegate: repo, journalPath: path}
			if _, err := tracked.CreateRevision(context.Background(), nativeIntentInput(), nil); err == nil {
				t.Fatal("expected journal failure")
			}
			if repo.writes != 0 {
				t.Fatalf("writes=%d", repo.writes)
			}
			if tc.existing {
				b, _ := os.ReadFile(path)
				if string(b) != "sentinel" {
					t.Fatal("journal overwritten")
				}
			}
		})
	}
}
func TestNativeDeclarationTrackedRepositoryRejectsExistingID(t *testing.T) {
	in := nativeIntentInput()
	repo := &intentBoundaryRepo{rows: map[string]verification.DeclarationRevision{in.ID: {ID: in.ID, AccountID: in.AccountID}}}
	tracked := nativeDeclarationIntentRepository{delegate: repo, journalPath: nativeIntentPath(t)}
	if _, err := tracked.CreateRevision(context.Background(), in, nil); err == nil {
		t.Fatal("expected existing ID rejection")
	}
	if repo.writes != 0 {
		t.Fatal("write called for existing ID")
	}
}
func TestNativeDeclarationTrackedRepositoryKeepsIntentAfterUncertainWrite(t *testing.T) {
	writeErr := errors.New("commit outcome uncertain")
	repo := &intentBoundaryRepo{rows: map[string]verification.DeclarationRevision{}, writeErr: writeErr}
	tracked := nativeDeclarationIntentRepository{delegate: repo, journalPath: nativeIntentPath(t)}
	if _, err := tracked.CreateRevision(context.Background(), nativeIntentInput(), nil); !errors.Is(err, writeErr) {
		t.Fatalf("err=%v", err)
	}
	if _, err := os.Stat(tracked.journalPath); err != nil {
		t.Fatalf("intent absent: %v", err)
	}
}

func TestNativeDeclarationTrackedRepositoryRejectsSymlinkDirectory(t *testing.T) {
	path := nativeIntentPath(t)
	link := filepath.Join(t.TempDir(), "redirect")
	if err := os.Symlink(filepath.Dir(path), link); err != nil {
		t.Fatal(err)
	}
	repo := &intentBoundaryRepo{rows: map[string]verification.DeclarationRevision{}}
	tracked := nativeDeclarationIntentRepository{delegate: repo, journalPath: filepath.Join(link, "intent.json")}
	_, err := tracked.CreateRevision(context.Background(), nativeIntentInput(), nil)
	if err == nil || repo.writes != 0 {
		t.Fatalf("symlink directory accepted: error=%v writes=%d", err, repo.writes)
	}
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("journal escaped through symlink directory")
	}
}

func TestNativeDeclarationCleanupRejectsMissingAuthorityOrScope(t *testing.T) {
	in := nativeIntentInput()
	if err := cleanupNativeIdentityDeclarationFixture(context.Background(), nil, in.AccountID, in.CorrelationID, in.ID); err == nil {
		t.Fatal("missing cleanup pool reported success")
	}
}

func TestNativeIdentityCleanupPersistedIntent(t *testing.T) {
	if os.Getenv("IDENTITY_DECLARATION_NATIVE_CLEANUP_JOURNAL") != "1" {
		t.Skip("set explicit journal-cleanup opt-in to recover one authorized synthetic fixture")
	}
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal("repository root could not be resolved")
	}
	policyPath := filepath.Join(root, ".local", "testing", "test-account-policy.json")
	journalPath := filepath.Join(root, ".local", "testing", "kyc-native", "declaration-test-intent.json")
	if err := validateNativeIdentityPrivateFile(policyPath); err != nil {
		t.Fatal("private fixture policy metadata is invalid")
	}
	if err := validateNativeIdentityPrivateFile(journalPath); err != nil {
		t.Fatal("trusted cleanup journal metadata is invalid; preserve it")
	}
	policy, err := os.ReadFile(policyPath)
	if err != nil {
		t.Fatal("private fixture policy could not be read")
	}
	data, err := os.ReadFile(journalPath)
	if err != nil {
		t.Fatal("trusted cleanup journal could not be read")
	}
	var intent nativeDeclarationIntent
	if err := json.Unmarshal(data, &intent); err != nil || !canonicalNativeUUID(intent.AccountID) || !canonicalNativeUUID(intent.DeclarationID) || intent.CorrelationID == "" || strings.TrimSpace(intent.CorrelationID) != intent.CorrelationID || len(intent.CorrelationID) > 120 {
		t.Fatal("trusted cleanup journal scope is invalid; preserve it")
	}
	if err := validateNativeIdentityPolicy(policy, intent.AccountID); err != nil {
		t.Fatal("journal account is not an authorized verified synthetic fixture")
	}
	passfile := "/home/nandev/.local/share/espacigo-native-secrets/pgpass"
	config, err := parseNativeIdentityConfig(os.Getenv(identityDeclarationNativeCleanupDSNEnv), passfile)
	if err != nil {
		t.Fatal("authorized cleanup connection configuration is invalid")
	}
	config.MaxConns = 1
	config.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		if _, err := conn.Exec(ctx, `SET ROLE espacigo_schema_owner`); err != nil {
			return err
		}
		var session, current string
		if err := conn.QueryRow(ctx, `SELECT session_user,current_user`).Scan(&session, &current); err != nil {
			return err
		}
		if session != "espacigo_migrator_login" || current != "espacigo_schema_owner" {
			return errors.New("cleanup role identity mismatch")
		}
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal("authorized cleanup pool could not be opened")
	}
	defer pool.Close()
	if err := cleanupNativeIdentityDeclarationFixture(ctx, pool, intent.AccountID, intent.CorrelationID, intent.DeclarationID); err != nil {
		var sqlError interface{ SQLState() string }
		if errors.As(err, &sqlError) {
			t.Fatalf("trusted synthetic fixture cleanup failed (SQLSTATE=%s); preserve journal", sqlError.SQLState())
		}
		t.Fatal("trusted synthetic fixture cleanup failed; preserve journal")
	}
	if err := os.Remove(journalPath); err != nil {
		t.Fatalf("verified cleanup journal could not be retired: %v", err)
	}
	dir, err := os.Open(filepath.Dir(journalPath))
	if err != nil {
		t.Fatal("journal directory could not be opened for sync")
	}
	if err := dir.Sync(); err != nil {
		_ = dir.Close()
		t.Fatal("journal retirement could not be synced")
	}
	if err := dir.Close(); err != nil {
		t.Fatal("journal directory sync handle could not be closed")
	}
}

type nativeKYCNoRevisionArtifactCounts struct {
	Revisions        int64
	ProviderAttempts int64
	ProviderEvents   int64
	RutClaims        int64
	DeclarationAudit int64
	FixtureOutbox    int64
}

func nativeKYCNoRevisionHasAnyArtifact(counts nativeKYCNoRevisionArtifactCounts) bool {
	return counts != (nativeKYCNoRevisionArtifactCounts{})
}

func TestNativeKYCNoRevisionArtifactGuard(t *testing.T) {
	cases := []struct {
		name   string
		counts nativeKYCNoRevisionArtifactCounts
		want   bool
	}{
		{name: "empty", want: false},
		{name: "unexpected revision", counts: nativeKYCNoRevisionArtifactCounts{Revisions: 1}, want: true},
		{name: "provider attempt", counts: nativeKYCNoRevisionArtifactCounts{ProviderAttempts: 1}, want: true},
		{name: "provider event", counts: nativeKYCNoRevisionArtifactCounts{ProviderEvents: 1}, want: true},
		{name: "RUT claim", counts: nativeKYCNoRevisionArtifactCounts{RutClaims: 1}, want: true},
		{name: "declaration audit", counts: nativeKYCNoRevisionArtifactCounts{DeclarationAudit: 1}, want: true},
		{name: "fixture-linked Didit outbox", counts: nativeKYCNoRevisionArtifactCounts{FixtureOutbox: 1}, want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := nativeKYCNoRevisionHasAnyArtifact(tc.counts)
			if got != tc.want {
				t.Fatalf("nativeKYCNoRevisionHasAnyArtifact(%+v)=%t, want %t", tc.counts, got, tc.want)
			}
		})
	}
}
