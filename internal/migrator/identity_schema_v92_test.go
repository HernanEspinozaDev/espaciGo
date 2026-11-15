package migrator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIdentityV92ExpiredLifecyclePurgeContract(t *testing.T) {
	path := filepath.Join("..", "..", "db", "migrations", "V000092__identity_expired_lifecycle_purge.sql")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sql := string(b)
	for _, required := range []string{
		"CREATE OR REPLACE FUNCTION public.purge_expired_identity_declaration_revisions()",
		"RETURNS bigint", "SECURITY DEFINER", "SET search_path = pg_catalog",
		"LIMIT 100", "pg_try_advisory_xact_lock", "FOR UPDATE OF r NOWAIT",
		"provider_session_id", "char_length(dedup_key) > 120", "didit_session_deletion:",
		"identity_provider_event", "identity_provider_attempt", "identity_rut_claim",
		"outbox_evento_local", "EXCEPTION WHEN lock_not_available", "verified_at IS NULL",
		"retention_at <=", "REVOKE ALL ON FUNCTION", "FROM espacigo_runtime",
		"session_user <> 'espacigo_migrator_login'", "actor <> 'espacigo_schema_owner'",
		"hashtextextended(d.account_id::text, 0)", "hashtextextended(d.account_id::text, 1)",
		"FOR UPDATE OF x NOWAIT", "FOR UPDATE OF e NOWAIT", "FOR UPDATE OF c NOWAIT",
		"SET declaration_id=NULL", "verified_at IS NOT NULL", "o.agregado_id=a.id",
		"V92 requires the exact worker-only predecessor ACL",
	} {
		if !strings.Contains(sql, required) {
			t.Errorf("V92 missing purge contract %q", required)
		}
	}
	if strings.Contains(sql, "http_") || strings.Contains(sql, "net.http") {
		t.Fatal("V92 must not perform remote calls")
	}
	// This is a source-contract guard only. PostgreSQL acceptance is separate.
	previous := -1
	for _, fragment := range []string{
		"FOR UPDATE OF x NOWAIT", "FOR UPDATE OF e NOWAIT", "FOR UPDATE OF c NOWAIT", "FOR UPDATE OF r NOWAIT",
		"INSERT INTO public.outbox_evento_local", "DELETE FROM public.identity_provider_event",
		"DELETE FROM public.identity_provider_attempt", "DELETE FROM public.identity_rut_claim",
		"DELETE FROM public.identity_declaration_revision",
	} {
		at := strings.Index(sql, fragment)
		if at < 0 || at <= previous {
			t.Fatalf("V92 lock/outbox/dependency order invalid at %q", fragment)
		}
		previous = at
	}
	start := strings.Index(sql, "IF NOT pg_catalog.pg_try_advisory_xact_lock")
	if start < 0 {
		t.Fatal("missing group try-lock boundary")
	}
	end := strings.Index(sql[start:], "END IF;")
	if end < 0 || !strings.Contains(sql[start:start+end], "RAISE lock_not_available;") {
		t.Fatal("busy advisory namespace must unwind partial group locks")
	}
}
