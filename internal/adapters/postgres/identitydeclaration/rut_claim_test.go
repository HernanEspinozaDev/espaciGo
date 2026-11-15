package identitydeclaration

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/verification"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

const rutClaimPostgresDSNEnv = "ESPACIGO_RUT_CLAIM_TEST_DATABASE_URL"

func resolveRutClaimDSN(t *testing.T) string {
	t.Helper()
	return nativeTestDSNFromEnv(os.Getenv, rutClaimPostgresDSNEnv)
}

func TestRutClaimStoreRejectsInvalidInputsWithoutPool(t *testing.T) {
	store := NewRutClaimStore(nil)
	now := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	if err := store.Claim(context.Background(), "bad", "bad", "v1:abc", now.Add(time.Hour), now); !errors.Is(err, verification.ErrInvalid) {
		t.Fatalf("Claim() error = %v, want ErrInvalid", err)
	}
	if err := store.MarkVerified(context.Background(), "bad", "bad", "v1:abc", now); !errors.Is(err, verification.ErrInvalid) {
		t.Fatalf("MarkVerified() error = %v, want ErrInvalid", err)
	}
	if err := (*RutClaimStore)(nil).Claim(context.Background(), "", "", "", now, now); !errors.Is(err, verification.ErrInvalid) {
		t.Fatalf("nil store Claim() error = %v, want ErrInvalid", err)
	}
}

func TestValidRutClaimInputRequiresVersionedHMACDigest(t *testing.T) {
	account, declaration := "85000000-0000-4000-8000-000000000001", "85000000-0000-4000-8000-000000000002"
	valid := "v1:" + "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	if !validRutClaimInput(account, declaration, valid) {
		t.Fatal("valid versioned HMAC digest rejected")
	}
	for _, fingerprint := range []string{"v1:abc", "v1:" + stringsRepeat("a", 64) + "\n", ":" + stringsRepeat("a", 64), "v1:" + stringsRepeat("g", 64)} {
		if validRutClaimInput(account, declaration, fingerprint) {
			t.Fatalf("invalid fingerprint accepted: %q", fingerprint)
		}
	}
}

func stringsRepeat(s string, n int) string {
	out := ""
	for i := 0; i < n; i++ {
		out += s
	}
	return out
}

func TestRutClaimPostgresClaimLifecycle(t *testing.T) {
	dsn := resolveRutClaimDSN(t)
	if dsn == "" {
		t.Skip("native PostgreSQL lifecycle test not explicitly enabled")
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ConnConfig.Host != "127.0.0.1" || cfg.ConnConfig.Port != 5432 || cfg.ConnConfig.Database != "espacigo_dms_stage_20261020" {
		t.Fatalf("target mismatch: host=%s port=%d db=%s", cfg.ConnConfig.Host, cfg.ConnConfig.Port, cfg.ConnConfig.Database)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	var dbName string
	if err := pool.QueryRow(ctx, "SELECT current_database()").Scan(&dbName); err != nil || dbName != "espacigo_dms_stage_20261020" {
		t.Fatalf("target identity mismatch (database=%q, error=%v)", dbName, err)
	}
	_, _ = pool.Exec(ctx, "SET ROLE espacigo_schema_owner")
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "RESET ROLE")
	})
	store := NewRutClaimStore(pool)

	// Use two existing synthetic user accounts in the database
	accountA := "89c59651-d486-4f07-8847-f9f00507e3b7"
	accountB := "b8e52ff2-e2cc-4abd-a4ac-2c5f950b2a28"
	declA := ensureTestDeclarationRevision(t, ctx, pool, accountA)
	declB := ensureTestDeclarationRevision(t, ctx, pool, accountB)

	t.Cleanup(func() {
		cleanupCtx, cCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cCancel()
		_, _ = pool.Exec(cleanupCtx, "DELETE FROM public.identity_rut_claim WHERE account_id IN ($1::uuid, $2::uuid)", accountA, accountB)
		_, _ = pool.Exec(cleanupCtx, "DELETE FROM public.identity_declaration_revision WHERE id IN ($1::uuid, $2::uuid)", declA, declB)
	})

	// Clean existing claims for these accounts
	_, _ = pool.Exec(ctx, "DELETE FROM public.identity_rut_claim WHERE account_id IN ($1::uuid, $2::uuid)", accountA, accountB)

	fp := "v1:" + stringsRepeat("a", 64)
	now := time.Now().UTC().Truncate(time.Microsecond)
	expires := now.Add(time.Minute)
	if err := store.Claim(ctx, accountA, declA, fp, expires, now); err != nil {
		t.Fatalf("first claim failed: %v", err)
	}
	if err := NewRutClaimStore(pool).Claim(ctx, accountA, declA, fp, expires, now); err != nil {
		t.Fatalf("claim did not survive store recreation: %v", err)
	}
	if err := store.Claim(ctx, accountB, declB, fp, expires, now); !errors.Is(err, verification.ErrConflict) {
		t.Fatalf("cross-account claim error=%v, want neutral conflict", err)
	}
	if err := store.MarkVerified(ctx, accountA, declA, fp, now.Add(time.Second)); err != nil {
		t.Fatalf("MarkVerified failed: %v", err)
	}
	if err := store.MarkVerified(ctx, accountA, declA, fp, now.Add(2*time.Second)); err != nil {
		t.Fatalf("same verified claim was not idempotent: %v", err)
	}
	store.RevokeVerified(ctx, accountA, now.Add(3*time.Second))
	if err := store.Claim(ctx, accountB, declB, fp, now.Add(time.Hour), now.Add(4*time.Second)); !errors.Is(err, verification.ErrConflict) {
		t.Fatalf("verified fingerprint was released: %v", err)
	}
	otherFP := "v1:" + stringsRepeat("b", 64)
	if err := store.Claim(ctx, accountB, declB, otherFP, now.Add(time.Second), now); err != nil {
		t.Fatalf("pending claim failed: %v", err)
	}
	// Reclaim by same account B with different expiration succeeds
	if err := store.Claim(ctx, accountB, declB, otherFP, now.Add(time.Hour), now.Add(2*time.Second)); err != nil {
		t.Fatalf("reclaim by same account failed: %v", err)
	}
}

func ensureTestDeclarationRevision(t *testing.T, ctx context.Context, pool *pgxpool.Pool, accountID string) string {
	t.Helper()
	declID := uuid.NewString()
	now := time.Now().UTC().Truncate(time.Microsecond)
	_, err := pool.Exec(ctx, `
INSERT INTO public.identity_declaration_revision (
    id, account_id, revision, key_version, ciphertext, schema_version,
    consent_version, consent_given_at, retention_at
) VALUES (
    $1::uuid, $2::uuid, 1, 'v1', decode(repeat('00', 29), 'hex'), '1',
    'n05', $3::timestamptz, $3::timestamptz + interval '1 day'
)
`, declID, accountID, now)
	if err != nil {
		t.Fatalf("failed to create declaration fixture for account %s: %v", accountID, err)
	}
	return declID
}
