package migrator

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// TestRunnerV84AuthorityRecoveryPostgres exercises the V84 temporary-authority
// lifecycle only in the marked, isolated migration-CI PostgreSQL cluster.
func TestRunnerV84AuthorityRecoveryPostgres(t *testing.T) {
	if os.Getenv("TEST_MIGRATOR_CI_ISOLATED") != "1" {
		t.Skip("requires the disposable migration CI harness")
	}
	u, err := url.Parse(os.Getenv("TEST_DATABASE_URL"))
	if err != nil || u == nil || u.Scheme != "postgres" || u.Host != "localhost" || u.Path != "/postgres" || u.User == nil || u.User.Username() != "postgres" || !filepath.IsAbs(u.Query().Get("host")) {
		t.Fatal("disposable CI requires its private Unix socket")
	}
	marker := filepath.Join(u.Query().Get("host"), ".espacigo-migration-ci")
	if info, err := os.Lstat(marker); err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
		t.Fatal("disposable CI marker is missing")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	adminURL := os.Getenv("TEST_DATABASE_URL")
	admin := connectTestDB(t, ctx, adminURL)
	defer admin.Close(context.Background())
	var db string
	if err := admin.QueryRow(ctx, `SELECT current_database()`).Scan(&db); err != nil || db != "postgres" {
		t.Fatalf("CI admin connection must target postgres: database=%q err=%v", db, err)
	}
	const target = "espacigo_dms_stage_20261020"
	var preexisting, preexistingRoles bool
	if err := admin.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_catalog.pg_database WHERE datname=$1), EXISTS(SELECT 1 FROM pg_catalog.pg_roles WHERE rolname IN ('espacigo_migrator_login','espacigo_api_login','espacigo_schema_owner','espacigo_document_verification_fn','espacigo_runtime'))`, target).Scan(&preexisting, &preexistingRoles); err != nil || preexisting || preexistingRoles {
		t.Fatalf("fixed-name fixture and its roles must not already exist: database=%t roles=%t err=%v", preexisting, preexistingRoles, err)
	}
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{target}.Sanitize()); err != nil {
		t.Fatalf("create exclusive test database (must not already exist): %v", err)
	}
	var ownedOID uint32
	defer func() {
		cleanup := connectTestDB(t, context.Background(), adminURL)
		defer cleanup.Close(context.Background())
		var observedOID uint32
		var owner string
		if err := cleanup.QueryRow(context.Background(), `SELECT oid,pg_catalog.pg_get_userbyid(datdba) FROM pg_catalog.pg_database WHERE datname=$1`, target).Scan(&observedOID, &owner); err != nil || ownedOID == 0 || observedOID != ownedOID || owner != "postgres" {
			t.Errorf("refuse cleanup of unproven/replaced database: err=%v", err)
			return
		}
		if _, err := cleanup.Exec(context.Background(), "DROP DATABASE "+pgx.Identifier{target}.Sanitize()+" WITH (FORCE)"); err != nil {
			t.Errorf("drop owned test database: %v", err)
			return
		}
		if _, err := cleanup.Exec(context.Background(), `DROP ROLE IF EXISTS espacigo_migrator_login, espacigo_api_login, espacigo_schema_owner, espacigo_document_verification_fn, espacigo_runtime`); err != nil {
			t.Errorf("drop fixture roles after database cleanup: %v", err)
		}
	}()
	if err := admin.QueryRow(ctx, `SELECT oid FROM pg_catalog.pg_database WHERE datname=$1`, target).Scan(&ownedOID); err != nil {
		t.Fatalf("record newly created fixture database identity: %v", err)
	}
	databaseURL, err := url.Parse(adminURL)
	if err != nil {
		t.Fatal(err)
	}
	databaseURL.Path = "/" + target
	targetURL := databaseURL.String()
	all, err := DiscoverMigrations(filepath.Join("..", "..", "db", "migrations"))
	if err != nil || len(all) != 84 || inventoryFingerprint(all) != v84AuthorityInventory {
		t.Fatalf("pinned V84 migration inventory unavailable: %v", err)
	}
	prefix := func(last int) string {
		t.Helper()
		dir := t.TempDir()
		for _, m := range all[:last] {
			if err := os.WriteFile(filepath.Join(dir, m.Name), m.SQL, 0o600); err != nil {
				t.Fatal(err)
			}
		}
		return dir
	}
	conn := connectTestDB(t, ctx, targetURL)
	defer conn.Close(context.Background())
	mustExec := func(sql string) {
		t.Helper()
		if _, err := conn.Exec(ctx, sql); err != nil {
			t.Fatal(err)
		}
	}
	mustExec(`CREATE ROLE espacigo_runtime NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT;
CREATE ROLE espacigo_api_login LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT;
CREATE ROLE espacigo_migrator_login LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT;
GRANT espacigo_runtime TO espacigo_api_login WITH ADMIN FALSE, INHERIT FALSE, SET TRUE;`)
	if _, err := Run(ctx, targetURL, prefix(80)); err != nil {
		t.Fatalf("apply fresh V1-V80 fixture: %v", err)
	}
	mustExec(`CREATE ROLE espacigo_schema_owner NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT;
CREATE ROLE espacigo_document_verification_fn NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT;
CREATE SCHEMA document_verification_private AUTHORIZATION espacigo_document_verification_fn;
GRANT USAGE ON SCHEMA public TO espacigo_document_verification_fn, espacigo_schema_owner;
GRANT SELECT(id,estado), UPDATE(id) ON public.usuario TO espacigo_document_verification_fn;
GRANT SELECT(id,propietario_id), UPDATE(id) ON public.espacio TO espacigo_document_verification_fn;
GRANT espacigo_schema_owner TO espacigo_migrator_login WITH ADMIN FALSE, INHERIT FALSE, SET TRUE;
GRANT espacigo_document_verification_fn TO espacigo_schema_owner WITH ADMIN FALSE, INHERIT FALSE, SET TRUE;
GRANT SELECT,INSERT ON public.schema_migrations TO espacigo_schema_owner;
GRANT CONNECT ON DATABASE espacigo_dms_stage_20261020 TO espacigo_migrator_login, espacigo_api_login;`)
	mustExec(`REVOKE ALL ON DATABASE espacigo_dms_stage_20261020 FROM PUBLIC`)
	migrationURL := *databaseURL
	migrationURL.User = url.User("espacigo_migrator_login")
	if _, err := RunWithRole(ctx, migrationURL.String(), prefix(81), "espacigo_schema_owner"); err != nil {
		t.Fatalf("apply V81 source fixture: %v", err)
	}
	mustExec(`GRANT CREATE ON SCHEMA public TO espacigo_schema_owner;
ALTER FUNCTION public.document_verification_space_owner_changed() OWNER TO espacigo_schema_owner;
GRANT UPDATE(id) ON public.document_verification_reviewer_grant TO espacigo_schema_owner WITH GRANT OPTION;`)
	if _, err := RunWithRole(ctx, migrationURL.String(), prefix(83), "espacigo_schema_owner"); err != nil {
		t.Fatalf("apply guarded V82/V83 fixture: %v", err)
	}
	mustExec(`ALTER FUNCTION public.document_verification_space_owner_changed() OWNER TO postgres;
REVOKE UPDATE(id) ON public.document_verification_reviewer_grant FROM espacigo_schema_owner CASCADE;
GRANT UPDATE(id) ON public.document_verification_reviewer_grant TO espacigo_runtime;
REVOKE CREATE ON SCHEMA public FROM espacigo_schema_owner;`)
	before, err := readHistory(ctx, conn)
	if err != nil || len(before) != 83 {
		t.Fatalf("V83 fixture ledger: rows=%d err=%v", len(before), err)
	}
	migrationDir := prefix(84)
	inspect, err := manageV84AuthorityWithConn(ctx, conn, migrationDir, "inspect", "")
	if err != nil || inspect.AppliedCount != 83 || !inspect.AuthorityClosed || !validHex(inspect.BaselineSHA256, 64) {
		t.Fatalf("inspect V83 authority baseline: report=%+v err=%v", inspect, err)
	}
	assertV84TemporaryPrivileges(t, ctx, conn, false)

	t.Run("failure_cleanup", func(t *testing.T) {
		if _, err := manageV84AuthorityWithConn(ctx, conn, migrationDir, "prepare", ""); err != nil {
			t.Fatal(err)
		}
		assertV84TemporaryPrivileges(t, ctx, conn, true)
		if _, err := manageV84AuthorityWithConn(ctx, conn, migrationDir, "finalize", "0000000000000000000000000000000000000000000000000000000000000000"); err == nil {
			t.Fatal("wrong-baseline finalization unexpectedly succeeded")
		}
		assertV84TemporaryPrivileges(t, ctx, conn, false)
		unchanged, err := readHistory(ctx, conn)
		if err != nil || len(unchanged) != 83 {
			t.Fatalf("failure cleanup changed ledger: rows=%d err=%v", len(unchanged), err)
		}
	})

	t.Run("apply_and_finalize", func(t *testing.T) {
		if _, err := manageV84AuthorityWithConn(ctx, conn, migrationDir, "prepare", ""); err != nil {
			t.Fatal(err)
		}
		assertV84TemporaryPrivileges(t, ctx, conn, true)
		result, err := RunWithRole(ctx, migrationURL.String(), migrationDir, "espacigo_schema_owner")
		if err != nil || len(result.Applied) != 1 || result.Applied[0] != 84 {
			t.Fatalf("run original V84 as dedicated migrator: applied=%v err=%v", result.Applied, err)
		}
		if _, err := manageV84AuthorityWithConn(ctx, conn, migrationDir, "finalize", inspect.BaselineSHA256); err != nil {
			t.Fatalf("finalize V84: %v", err)
		}
		independent := connectTestDB(t, ctx, targetURL)
		defer independent.Close(context.Background())
		plan, err := Plan(ctx, targetURL, migrationDir, "espacigo_schema_owner")
		if err != nil || !plan.ReadOnly || len(plan.Applied) != 84 || len(plan.Pending) != 0 {
			t.Fatalf("independent V84 ledger postcheck: plan=%+v err=%v", plan, err)
		}
		assertV84TemporaryPrivileges(t, ctx, independent, false)
		var empty bool
		if err := independent.QueryRow(ctx, `SELECT NOT EXISTS(SELECT 1 FROM public.usuario)`).Scan(&empty); err != nil || !empty {
			t.Fatalf("fixture unexpectedly contains account data: %v", err)
		}
	})
}

func assertV84TemporaryPrivileges(t *testing.T, ctx context.Context, conn *pgx.Conn, expected bool) {
	t.Helper()
	var create, references, grantOption, tableReferences bool
	err := conn.QueryRow(ctx, `SELECT has_schema_privilege('espacigo_schema_owner','public','CREATE'), has_column_privilege('espacigo_schema_owner','public.usuario','id','REFERENCES'), has_schema_privilege('espacigo_schema_owner','public','CREATE WITH GRANT OPTION') OR has_column_privilege('espacigo_schema_owner','public.usuario','id','REFERENCES WITH GRANT OPTION'), has_table_privilege('espacigo_schema_owner','public.usuario','REFERENCES')`).Scan(&create, &references, &grantOption, &tableReferences)
	if err != nil {
		t.Fatal(err)
	}
	if create != expected || references != expected || grantOption || tableReferences {
		t.Fatalf("V84 temporary privileges: schema-create=%t column-references=%t table-references=%t grant-option=%t expected=%t", create, references, tableReferences, grantOption, expected)
	}
}
