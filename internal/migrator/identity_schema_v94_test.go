package migrator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIdentityV94DeliveredSessionOutboxRetirementContract(t *testing.T) {
	path := filepath.Join("..", "..", "db", "migrations", "V000094__identity_delivered_session_outbox_retirement.sql")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sql := string(b)
	for _, required := range []string{
		"CREATE FUNCTION public.purge_delivered_identity_session_outbox()",
		"RETURNS bigint", "SECURITY DEFINER", "SET search_path = pg_catalog",
		"session_user <> 'espacigo_migrator_login'", "current_user <> 'espacigo_schema_owner'",
		"current_schema() <> 'public'", "espacigo_privacy_cleanup_worker",
		"agregado_tipo", "didit_session", "didit.session_remote_deletion",
		"didit_session_deletion:", "entregada_en IS NOT NULL", "retirar_en IS NOT NULL",
		"retirar_en <= v_now", "o.retirar_en >= o.entregada_en + interval '30 days'",
		"cancelada_en IS NULL", "fallo_terminal_en IS NULL", "LIMIT 100",
		"FOR UPDATE OF o SKIP LOCKED", "GET DIAGNOSTICS v_deleted = ROW_COUNT",
		"REVOKE ALL ON FUNCTION public.purge_delivered_identity_session_outbox() FROM PUBLIC",
		"GRANT EXECUTE ON FUNCTION public.purge_delivered_identity_session_outbox()",
	} {
		if !strings.Contains(sql, required) {
			t.Errorf("V94 missing retirement contract %q", required)
		}
	}
	for _, forbidden := range []string{
		"CREATE TABLE", "identity_provider_attempt", "identity_provider_event",
		"http_", "net.http", "p_at", "p_limit", "GRANT DELETE", "GRANT ALL",
	} {
		if strings.Contains(sql, forbidden) {
			t.Errorf("V94 contains out-of-scope construct %q", forbidden)
		}
	}
	if strings.Count(sql, "clock_timestamp()") != 1 {
		t.Fatalf("V94 must capture the real clock exactly once, found %d calls", strings.Count(sql, "clock_timestamp()"))
	}
	if strings.Count(sql, "FOR UPDATE OF o SKIP LOCKED") != 1 {
		t.Fatal("V94 must lock only its bounded candidate set with SKIP LOCKED")
	}
	if strings.Contains(sql, "OR NOT EXISTS (\n        SELECT 1 FROM pg_catalog.pg_auth_members m") {
		t.Fatal("V94 must not require worker LOGIN membership to install a worker-only function")
	}
	if strings.Contains(sql, "c.relowner FROM pg_catalog.pg_class c WHERE c.oid=relation) <> actor") {
		t.Fatal("V94 must not require transferring the existing outbox to its function owner")
	}
	for _, required := range []string{"pg_catalog.has_column_privilege", "'UPDATE'", "'SELECT'", "'postgres'"} {
		if !strings.Contains(sql, required) {
			t.Errorf("V94 must check existing table owner and explicit definer privileges: missing %q", required)
		}
	}
}
