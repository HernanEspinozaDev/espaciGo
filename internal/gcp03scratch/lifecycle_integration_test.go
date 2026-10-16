package gcp03scratch

import (
	"context"
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
	if err != nil || parsed.Scheme != "postgres" || parsed.User == nil || parsed.User.Username() != "postgres" || parsed.Hostname() != "127.0.0.1" || parsed.Port() == "" || parsed.Path != "/"+DatabaseName || parsed.RawQuery != "sslmode=disable" || hasPassword || password != "" {
		t.Fatal("integration URL must be passwordless postgres on loopback, target espacigo_scratch, sslmode=disable")
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
	if err := databaseACLOnConnection(ctx, admin); err != nil {
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
	if err := retireOnConnection(ctx, admin, migrationDir); err != nil {
		t.Fatalf("retire migration login: %v", err)
	}
	if err := dbbootstrap.VerifyScratchMigratorRetired(ctx, admin); err != nil {
		t.Fatal(err)
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
