package gcp03scratch

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/dbbootstrap"
	"github.com/HernanEspinozaDev/espaciGo/internal/migrator"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// TestScratchLifecyclePostgres18 exercises the real fixed bootstrap, V1-V61,
// final grants, and retirement SQL against a disposable local PostgreSQL 18
// instance. The opt-in guard deliberately rejects a persistent Compose port.
func TestScratchLifecyclePostgres18(t *testing.T) {
	connectionURL := os.Getenv("GCP03SCRATCH_TEST_DATABASE_URL")
	if connectionURL == "" {
		t.Skip("set GCP03SCRATCH_TEST_DATABASE_URL to an isolated PostgreSQL 18 endpoint")
	}
	parsed, err := url.Parse(connectionURL)
	password, hasPassword := "", false
	if err == nil && parsed.User != nil {
		password, hasPassword = parsed.User.Password()
	}
	if err != nil || parsed.Scheme != "postgres" || parsed.User == nil || parsed.User.Username() != "gcp03_lifecycle_admin" || parsed.Hostname() != "127.0.0.1" || parsed.Port() == "" || parsed.Path != "/"+DatabaseName || parsed.RawQuery != "sslmode=disable" || hasPassword || password != "" {
		t.Fatal("integration URL must be passwordless gcp03_lifecycle_admin on loopback, target espacigo_scratch, sslmode=disable")
	}
	var port int
	if _, err := fmt.Sscan(parsed.Port(), &port); err != nil || port <= 1024 || port == 5432 {
		t.Fatal("integration URL must use an isolated high loopback port")
	}
	if _, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", parsed.Port()), 250*time.Millisecond); err != nil {
		t.Fatalf("isolated PostgreSQL endpoint is not reachable: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	admin, err := pgx.Connect(ctx, connectionURL)
	if err != nil {
		t.Fatalf("connect to isolated PostgreSQL endpoint: %v", err)
	}
	defer admin.Close(context.Background())
	var version int
	if err := admin.QueryRow(ctx, "SELECT current_setting('server_version_num')::int / 10000").Scan(&version); err != nil || version != 18 {
		t.Fatalf("integration requires PostgreSQL 18, got %d (%v)", version, err)
	}
	var database string
	if err := admin.QueryRow(ctx, "SELECT current_database()").Scan(&database); err != nil || database != DatabaseName {
		t.Fatal("integration endpoint is not the exact scratch database")
	}
	var fresh bool
	if err := admin.QueryRow(ctx, `SELECT
		to_regclass('public.schema_migrations') IS NULL
		AND to_regclass('public.usuario') IS NULL
		AND to_regclass('public.espacio') IS NULL
		AND to_regclass('public.reserva') IS NULL
		AND NOT EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname IN (
			'espacigo_scratch_migrator', 'espacigo_runtime', 'espacigo_gadm_bootstrap'
		))`).Scan(&fresh); err != nil || !fresh {
		t.Fatalf("integration requires a fresh disposable database with no application schema or scratch roles (%v)", err)
	}
	for _, extension := range []string{"btree_gist", "postgis", "unaccent"} {
		if _, err := admin.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS "+pgx.Identifier{extension}.Sanitize()); err != nil {
			t.Fatalf("install locally available extension %s: %v", extension, err)
		}
	}
	if _, err := admin.Exec(ctx, `CREATE ROLE espacigo_scratch_migrator LOGIN CREATEROLE NOSUPERUSER NOCREATEDB NOINHERIT`); err != nil {
		t.Fatal("create isolated migration login")
	}
	if _, err := admin.Exec(ctx, `GRANT CONNECT ON DATABASE espacigo_scratch TO espacigo_scratch_migrator`); err != nil {
		t.Fatal("grant isolated login database connect")
	}
	if err := dbbootstrap.VerifyScratchBootstrapRolesCreated(ctx, admin); err == nil {
		t.Fatal("bootstrap role check unexpectedly passed before bootstrap")
	}

	migratorURL, err := url.Parse(connectionURL)
	if err != nil {
		t.Fatal(err)
	}
	migratorURL.User = url.User(MigratorLogin)
	migratorURL.RawQuery = "sslmode=disable"
	migratorConn, err := pgx.Connect(ctx, migratorURL.String())
	if err != nil {
		t.Fatal("connect as isolated migrator")
	}
	if err := bootstrapOnConnection(ctx, migratorConn); err != nil {
		t.Fatal(err)
	}
	if err := migratorConn.Close(ctx); err != nil {
		t.Fatal(err)
	}
	if err := dbbootstrap.VerifyScratchBootstrapRolesCreated(ctx, admin); err != nil {
		t.Fatalf("verify PostgreSQL 18 role autogrants: %v", err)
	}
	// This local postgres REVOKE models the Cloud SQL Admin users.update
	// reconciliation step; the Cloud SQL API is not exercised by this test.
	if _, err := admin.Exec(ctx, `REVOKE espacigo_runtime FROM espacigo_scratch_migrator; REVOKE espacigo_gadm_bootstrap FROM espacigo_scratch_migrator`); err != nil {
		t.Fatal("model Cloud SQL creator membership reconciliation")
	}
	if err := dbbootstrap.VerifyScratchBootstrap(ctx, admin); err != nil {
		t.Fatalf("verify reconciled bootstrap roles: %v", err)
	}
	// Prepare Cloud SQL's fixed owner login as a restricted non-superuser and
	// database owner. The separate lifecycle admin remains only a local fixture
	// control role for migration/extension setup.
	if _, err := admin.Exec(ctx, `CREATE ROLE postgres LOGIN NOSUPERUSER CREATEROLE`); err != nil {
		t.Fatal("create restricted disposable postgres owner login")
	}
	if _, err := admin.Exec(ctx, `ALTER DATABASE espacigo_scratch OWNER TO postgres`); err != nil {
		t.Fatal("assign disposable database to restricted owner login")
	}
	ownerURL, err := url.Parse(connectionURL)
	if err != nil {
		t.Fatal(err)
	}
	ownerURL.User = url.User(DatabaseOwnerLogin)
	ownerConn := mustConnect(t, ctx, ownerURL.String())
	defer ownerConn.Close(context.Background())
	if err := databaseACLOnConnection(ctx, ownerConn); err != nil {
		t.Fatal(err)
	}
	if err := dbbootstrap.VerifyScratchDatabaseACL(ctx, admin); err != nil {
		t.Fatal(err)
	}

	migratorConn = mustConnect(t, ctx, migratorURL.String())
	if err := dbbootstrap.VerifyScratchBootstrap(ctx, migratorConn); err != nil {
		t.Fatalf("migrator preflight: %v", err)
	}
	if _, err := migratorConn.Exec(ctx, `CREATE TEMP TABLE gcp03_temp_acl_probe (id integer)`); err != nil {
		t.Fatalf("migrator-only TEMP and schema CREATE grants: %v", err)
	}
	if _, err := migratorConn.Exec(ctx, `CREATE TABLE public.gcp03_schema_acl_probe (id integer)`); err != nil {
		t.Fatalf("migrator-only schema CREATE grant: %v", err)
	}
	if _, err := migratorConn.Exec(ctx, `DROP TABLE public.gcp03_schema_acl_probe`); err != nil {
		t.Fatal(err)
	}
	if err := migratorConn.Close(ctx); err != nil {
		t.Fatal(err)
	}
	migrationDir := filepath.Join(filepath.Dir(sourceFile(t)), "..", "..", "db", "migrations")
	// The local root connection models Cloud SQL's cloudsqlsuperuser capability
	// for migrations. The actual Cloud SQL built-in role is service-managed and
	// cannot be faithfully recreated by a vanilla local PostgreSQL cluster.
	if result, err := migrator.Run(ctx, connectionURL, migrationDir); err != nil {
		t.Fatalf("run real V1-V61 migrations: %v", err)
	} else if len(result.Applied) != MigrationCount {
		t.Fatalf("expected %d applied migrations, got %d", MigrationCount, len(result.Applied))
	}
	if err := finalizeOnConnection(ctx, admin, migrationDir); err != nil {
		t.Fatalf("verify and grant runtime permissions: %v", err)
	}
	// The lifecycle runner above uses the disposable bootstrap admin to model
	// Cloud SQL's managed migration capability. Transfer this one metadata table
	// to its intended migration owner before exercising the ownership failure;
	// this does not claim the local runner reproduces Cloud SQL role semantics.
	if _, err := admin.Exec(ctx, `ALTER TABLE public.schema_migrations OWNER TO espacigo_scratch_migrator`); err != nil {
		t.Fatal("model migrator ownership of migration history")
	}
	observerConn := mustConnect(t, ctx, migratorURL.String())
	if _, err := observerConn.Exec(ctx, `SET default_transaction_read_only = on`); err != nil {
		t.Fatalf("enforce read-only observer session: %v", err)
	}
	var readOnly string
	if err := observerConn.QueryRow(ctx, `SHOW transaction_read_only`).Scan(&readOnly); err != nil || readOnly != "on" {
		t.Fatalf("observer session is not read-only (%v)", err)
	}
	observation, err := dbbootstrap.ObserveScratch(ctx, observerConn, dbbootstrap.RoleMembershipLocalPostgres18)
	if err != nil {
		t.Fatalf("run fixed observer against disposable PostgreSQL 18/PostGIS: %v", err)
	}
	if !observation.Passed() {
		t.Fatalf("fixed observer reported failed checks: %+v", observation)
	}
	if len(observation.RuntimeChecks) != 16 || observation.PostGISFullVersion == "" ||
		observation.Spatial.ExpectedIndex != "public.reserva_ensayo_local_ubicacion_gist" {
		t.Fatalf("observer omitted required local inventory: %+v", observation)
	}
	for _, membership := range observation.Memberships {
		if membership.Member == MigratorLogin && membership.GrantedRole == "cloudsqlsuperuser" {
			t.Fatal("local PostgreSQL fixture falsely reported Cloud SQL-managed membership")
		}
	}
	if err := observerConn.Close(ctx); err != nil {
		t.Fatal("close observer login connection")
	}
	var ownerAttrs struct {
		Superuser    bool
		CreateRole   bool
		HistoryOwner string
	}
	if err := ownerConn.QueryRow(ctx, `SELECT r.rolsuper, r.rolcreaterole, pg_catalog.pg_get_userbyid(c.relowner)
		FROM pg_catalog.pg_roles r
		JOIN pg_catalog.pg_class c ON c.relname = 'schema_migrations'
		JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace AND n.nspname = 'public'
		WHERE r.rolname = 'postgres'`).Scan(&ownerAttrs.Superuser, &ownerAttrs.CreateRole, &ownerAttrs.HistoryOwner); err != nil {
		t.Fatal("inspect restricted postgres lifecycle fixture")
	}
	if ownerAttrs.Superuser || !ownerAttrs.CreateRole || ownerAttrs.HistoryOwner != MigratorLogin {
		t.Fatalf("owner fixture does not model non-superuser/non-owner postgres: %+v", ownerAttrs)
	}
	if err := migrator.VerifyAllApplied(ctx, ownerConn, migrationDir, MigrationCount); err == nil {
		t.Fatal("pre-grant owner migration-history verification unexpectedly passed")
	} else {
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
			t.Fatalf("pre-grant failure = %v, want permission denied 42501", err)
		}
	}

	migratorConn = mustConnect(t, ctx, migratorURL.String())
	if err := grantOwnerHistoryOnConnection(ctx, migratorConn, migrationDir); err != nil {
		t.Fatalf("grant exact migration-history read to restricted owner: %v", err)
	}
	if err := migratorConn.Close(ctx); err != nil {
		t.Fatal("close owner-history grant phase")
	}
	if err := migrator.VerifyAllApplied(ctx, ownerConn, migrationDir, MigrationCount); err != nil {
		t.Fatalf("post-grant owner migration-history verification: %v", err)
	}
	var historyCount int
	if err := ownerConn.QueryRow(ctx, `SELECT count(*) FROM public.schema_migrations`).Scan(&historyCount); err != nil || historyCount != MigrationCount {
		t.Fatalf("owner history read returned %d rows (%v), want %d", historyCount, err, MigrationCount)
	}
	if err := retireOnConnection(ctx, ownerConn, migrationDir); err == nil {
		t.Fatal("restricted non-superuser postgres unexpectedly altered the migrator role")
	} else {
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "42501" || !strings.Contains(err.Error(), "permission denied to alter role") {
			t.Fatalf("restricted owner retirement failure = %v, want local ALTER ROLE permission limitation", err)
		}
	}
	// Vanilla PostgreSQL's CREATEROLE does not model Cloud SQL's managed
	// cloudsqlsuperuser behavior for ALTER ROLE. Promote only the disposable
	// fixture owner after recording that limitation, then exercise the fixed
	// retirement SQL and final ACL checks. This is SQL/ACL coverage, not proof
	// that the managed Cloud SQL role can perform ALTER ROLE.
	if _, err := admin.Exec(ctx, `ALTER ROLE postgres SUPERUSER`); err != nil {
		t.Fatal("enable disposable owner for fixed retirement SQL syntax/ACL check")
	}
	if err := ownerConn.Close(ctx); err != nil {
		t.Fatal("close restricted owner fixture session")
	}
	ownerConn = mustConnect(t, ctx, ownerURL.String())
	if err := retireOnConnection(ctx, ownerConn, migrationDir); err != nil {
		t.Fatalf("retire migration login: %v", err)
	}
	if err := dbbootstrap.VerifyScratchMigratorRetired(ctx, ownerConn); err != nil {
		t.Fatal(err)
	}
	var exactACL bool
	if err := ownerConn.QueryRow(ctx, `SELECT has_table_privilege('postgres', 'public.schema_migrations', 'SELECT')
		AND NOT has_table_privilege('espacigo_runtime', 'public.schema_migrations', 'SELECT')
		AND NOT EXISTS (
			SELECT 1 FROM pg_catalog.aclexplode(coalesce(c.relacl, pg_catalog.acldefault('r', c.relowner))) acl
			JOIN pg_catalog.pg_roles r ON r.oid = acl.grantee
			WHERE r.rolname = 'postgres' AND acl.privilege_type = 'SELECT' AND acl.is_grantable
		)
		FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'public' AND c.relname = 'schema_migrations'`).Scan(&exactACL); err != nil || !exactACL {
		t.Fatalf("post-retirement history ACL is not restricted as reviewed (%v)", err)
	}
}

func mustConnect(t *testing.T, ctx context.Context, connectionURL string) *pgx.Conn {
	t.Helper()
	conn, err := pgx.Connect(ctx, connectionURL)
	if err != nil {
		t.Fatal("connect to isolated test database")
	}
	return conn
}

func sourceFile(t *testing.T) string {
	t.Helper()
	_, path, _, ok := runtime.Caller(0)
	if !ok || strings.TrimSpace(path) == "" {
		t.Fatal("resolve integration test source path")
	}
	return path
}
