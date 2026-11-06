package migrator

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// Opt-in to the already authorized schema-only clone, never a persistent DB.
func TestSpacesCloudRangeGuardLocalPostgres(t *testing.T) {
	dsn := os.Getenv("TEST_SPACES_CLOUD_RANGE_LOCAL_URL")
	if dsn == "" {
		t.Skip("requires the authorized isolated spaces-cloud clone")
	}
	u, err := url.Parse(dsn)
	if err != nil || u.Host != "127.0.0.1:15432" || u.Path != "/espacigo_dms_stage_20261020" || u.User == nil || u.User.Username() != "postgres" || u.Query().Get("sslmode") != "require" {
		t.Fatal("wrong local rehearsal connection")
	}
	if _, embedded := u.User.Password(); embedded {
		t.Fatal("use private local PGPASSFILE")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())
	if _, err := conn.Exec(ctx, "BEGIN READ ONLY"); err != nil {
		t.Fatal(err)
	}
	defer conn.Exec(context.Background(), "ROLLBACK")
	var safe bool
	if err := conn.QueryRow(ctx, `SELECT current_database()='espacigo_dms_stage_20261020'
 AND session_user='postgres' AND current_user='postgres'
 AND inet_server_addr()='127.0.0.1'::inet
 AND NOT (SELECT rolsuper FROM pg_roles WHERE rolname=current_user)
 AND EXISTS (SELECT 1 FROM pg_roles WHERE rolname='local_delta_bootstrap' AND rolsuper)
 AND (SELECT count(*)=0 FROM public.usuario)
 AND (SELECT count(*)=81 AND max(version)=81 FROM public.schema_migrations)`).Scan(&safe); err != nil {
		t.Fatal(err)
	}
	if !safe {
		t.Fatal("not the unchanged isolated V81 zero-account rehearsal")
	}
	snapshot := func() string {
		t.Helper()
		var value string
		if err := conn.QueryRow(ctx, `SELECT md5(string_agg(version::text||'|'||name||'|'||checksum,';' ORDER BY version))
     ||':'||(SELECT md5(pg_get_functiondef('public.document_verification_space_owner_changed()'::regprocedure)))
     FROM public.schema_migrations`).Scan(&value); err != nil {
			t.Fatal(err)
		}
		return value
	}
	before := snapshot()
	source := filepath.Join("..", "..", "db", "migrations")
	// This must reject an insufficient prefix under the actual advisory lock.
	if _, err := RunWithRoleInRange(ctx, dsn, source, "", 82, 83); err == nil || !strings.Contains(err.Error(), "below required minimum") {
		t.Fatalf("wrong bounded rejection: %v", err)
	}
	if after := snapshot(); after != before {
		t.Fatal("rejection changed ledger or function")
	}
	var available bool
	if err := conn.QueryRow(ctx, "SELECT pg_try_advisory_lock($1,$2)", advisoryLockClass, advisoryLockObject).Scan(&available); err != nil {
		t.Fatal(err)
	}
	if !available {
		t.Fatal("bounded rejection leaked migration advisory lock")
	}
	if _, err := conn.Exec(ctx, "SELECT pg_advisory_unlock($1,$2)", advisoryLockClass, advisoryLockObject); err != nil {
		t.Fatal(err)
	}
	t.Log("real PostgreSQL rejection before DDL, unchanged V81/function, advisory lock released")
}
