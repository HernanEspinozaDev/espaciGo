package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/migrator"
	"github.com/jackc/pgx/v5"
)

const testDumpSHA256 = "1d31721c1a3fee15903f3a5cfbb348e5803b567727187c0a8871313b94f7faee"

// TestTransactionalRestoreAtomicityFromPrivateW6Bundle exercises the exact
// restore SQL assembled by this command against disposable PostgreSQL 18.
// scripts/test-gcp04-w6-restore-atomicity.sh provisions that isolated server.
func TestTransactionalRestoreAtomicityFromPrivateW6Bundle(t *testing.T) {
	if os.Getenv("GCP04_W6_TEST_EPHEMERAL") != "1" {
		t.Skip("requires scripts/test-gcp04-w6-restore-atomicity.sh disposable PostgreSQL fixture")
	}
	bundleDir := os.Getenv("GCP04_W6_TEST_BUNDLE")
	adminDSN := os.Getenv("GCP04_W6_TEST_ADMIN_DSN")
	if bundleDir == "" || adminDSN == "" {
		t.Fatal("private W6 bundle and disposable PostgreSQL admin DSN are required")
	}
	parsedAdmin, err := url.Parse(adminDSN)
	if err != nil || parsedAdmin.Scheme != "postgres" || parsedAdmin.User == nil || parsedAdmin.User.Username() != "postgres" || parsedAdmin.Hostname() != "127.0.0.1" || parsedAdmin.Path != "/postgres" || parsedAdmin.RawQuery != "sslmode=disable" {
		t.Fatal("admin DSN must use the loopback-only disposable fixture")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Minute)
	defer cancel()
	admin, err := pgx.Connect(ctx, adminDSN)
	if err != nil {
		t.Fatalf("connect to disposable PostgreSQL: %v", err)
	}
	defer admin.Close(context.Background())
	var version int
	if err := admin.QueryRow(ctx, `SELECT current_setting('server_version_num')::int`).Scan(&version); err != nil || version < 180000 || version >= 190000 {
		t.Fatalf("fixture must be PostgreSQL 18; version=%d err=%v", version, err)
	}
	assertNamesAvailable(t, ctx, admin)

	t.Cleanup(func() { cleanupFixture(t, adminDSN) })
	if err := createFixtureRolesAndTarget(ctx, admin, adminDSN); err != nil {
		t.Fatalf("create disposable V1-V59 target roles/database: %v", err)
	}

	root, migrationsDir := repositoryMigrationsDir(t)
	migrations, err := migrator.DiscoverMigrations(migrationsDir)
	if err != nil || len(migrations) < 64 {
		t.Fatalf("discover canonical migrations: count=%d err=%v", len(migrations), err)
	}
	v59Dir := filepath.Join(t.TempDir(), "migrations-v59")
	if err := os.Mkdir(v59Dir, 0700); err != nil {
		t.Fatal(err)
	}
	for _, migration := range migrations[:59] {
		if err := os.WriteFile(filepath.Join(v59Dir, filepath.Base(migration.Path)), migration.SQL, 0600); err != nil {
			t.Fatal(err)
		}
	}
	targetURL := *parsedAdmin
	targetURL.User = url.User("espacigo_migrator_login")
	targetURL.Path = "/espacigo"
	if _, err := migrator.Run(ctx, targetURL.String(), v59Dir); err != nil {
		t.Fatalf("apply real V1-V59 fixture migrations: %v", err)
	}
	adminTarget, err := pgx.Connect(ctx, strings.Replace(adminDSN, "/postgres?", "/espacigo?", 1))
	if err != nil {
		t.Fatalf("connect to fixture target database: %v", err)
	}
	defer adminTarget.Close(context.Background())
	if _, err := adminTarget.Exec(ctx, `REASSIGN OWNED BY espacigo_migrator_login TO espacigo_schema_owner`); err != nil {
		t.Fatalf("assign fixture objects to the reviewed schema owner: %v", err)
	}
	if _, err := adminTarget.Exec(ctx, `REVOKE CREATE ON SCHEMA public FROM espacigo_migrator_login`); err != nil {
		t.Fatal("revoke temporary fixture migration privilege")
	}

	manifestBytes, files := readPrivateTestBundle(t, bundleDir)
	manifestHash := hexDigest(manifestBytes)
	cfg := config{mode: "restore", expectedManifest: manifestHash, expectedDump: testDumpSHA256}
	verifiedManifest, verifiedFiles, err := readAndVerifyBundleAt(cfg, bundleDir)
	if err != nil || !bytes.Equal(manifestBytes, verifiedManifest) {
		t.Fatalf("validate retained bundle pins: %v", err)
	}
	manifestValue := new(manifest)
	if err := json.Unmarshal(verifiedManifest, manifestValue); err != nil {
		t.Fatalf("decode reviewed manifest: %v", err)
	}
	if err := verifyArchiveTOC(ctx, verifiedFiles); err != nil {
		t.Fatalf("verify retained archive TOC: %v", err)
	}
	script, err := buildScript(*manifestValue, "restore", verifiedFiles)
	if err != nil {
		t.Fatalf("build restore guard script: %v", err)
	}
	if err := generateRestoreSQL(ctx, verifiedFiles, *manifestValue, &script); err != nil {
		t.Fatalf("generate restore SQL through production pg_restore path: %v", err)
	}
	if !bytes.Equal(files["baseline.dump"], verifiedFiles["baseline.dump"]) {
		t.Fatal("bundle changed between validation and SQL generation")
	}

	privateTemp := createPrivateRestoreTestDir(t, root)
	failureSQL := bytes.Replace(script,
		[]byte("END $gcp04_postcheck$;\nCOMMIT;"),
		[]byte("END $gcp04_postcheck$;\nSELECT 1/0;\nCOMMIT;"), 1)
	if bytes.Equal(failureSQL, script) {
		t.Fatal("could not place the injected failure immediately before COMMIT")
	}
	failurePath := filepath.Join(privateTemp, "rollback.sql")
	if err := os.WriteFile(failurePath, failureSQL, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := runRestorePSQL(ctx, targetURL.String(), failurePath, privateTemp); err == nil {
		t.Fatal("injected pre-COMMIT failure unexpectedly succeeded")
	} else if !strings.Contains(err.Error(), "22012") || !strings.Contains(err.Error(), "division by zero") {
		t.Fatalf("rollback was not caused by the intended pre-COMMIT SQLSTATE 22012 fault: %v", err)
	}
	assertV59RollbackState(t, ctx, adminTarget, manifestValue.Migrations.SourceLedger[:59])
	roleBefore, err := gadmRoleFingerprint(ctx, adminTarget)
	if err != nil {
		t.Fatalf("read GADM role after rollback: %v", err)
	}

	successPath := filepath.Join(privateTemp, "restore.sql")
	if err := os.WriteFile(successPath, script, 0600); err != nil {
		t.Fatal(err)
	}
	output, err := runRestorePSQL(ctx, targetURL.String(), successPath, privateTemp)
	if err != nil {
		t.Fatalf("run production restore SQL successfully: %v", err)
	}
	if _, err := parseSQLReport(output); err != nil {
		t.Fatalf("decode successful restore report: %v", err)
	}
	assertV64RestoreState(t, ctx, adminTarget, manifestValue.Migrations.SourceLedger)
	roleAfter, err := gadmRoleFingerprint(ctx, adminTarget)
	if err != nil || roleAfter != roleBefore {
		t.Fatalf("GADM role changed during restore: before=%s after=%s err=%v", roleBefore, roleAfter, err)
	}
}

func assertNamesAvailable(t *testing.T, ctx context.Context, conn *pgx.Conn) {
	t.Helper()
	var dbExists bool
	var roles int
	if err := conn.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_catalog.pg_database WHERE datname='espacigo')`).Scan(&dbExists); err != nil {
		t.Fatal(err)
	}
	if err := conn.QueryRow(ctx, `SELECT count(*) FROM pg_catalog.pg_roles WHERE rolname IN ('espacigo_schema_owner','espacigo_runtime','espacigo_privacy_cleanup_worker','espacigo_gadm_bootstrap','espacigo_db_observer','espacigo_migrator_login')`).Scan(&roles); err != nil {
		t.Fatal(err)
	}
	if dbExists || roles != 0 {
		t.Fatalf("refusing non-empty PostgreSQL fixture: target_db_exists=%t reserved_roles=%d", dbExists, roles)
	}
}

func createFixtureRolesAndTarget(ctx context.Context, admin *pgx.Conn, adminDSN string) error {
	for _, statement := range []string{
		`CREATE ROLE espacigo_schema_owner NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT`,
		`CREATE ROLE espacigo_runtime NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT`,
		`CREATE ROLE espacigo_privacy_cleanup_worker NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT`,
		`CREATE ROLE espacigo_gadm_bootstrap NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT`,
		`CREATE ROLE espacigo_db_observer NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT`,
		`COMMENT ON ROLE espacigo_gadm_bootstrap IS 'NOLOGIN GADM local bootstrap group; temporary isolated login membership only.'`,
		`CREATE ROLE espacigo_migrator_login LOGIN INHERIT NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS`,
		`GRANT espacigo_schema_owner TO espacigo_migrator_login WITH ADMIN FALSE, INHERIT TRUE, SET TRUE`,
		`CREATE DATABASE espacigo`,
	} {
		if _, err := admin.Exec(ctx, statement); err != nil {
			return err
		}
	}
	for _, statement := range []string{
		`GRANT CONNECT ON DATABASE espacigo TO espacigo_schema_owner`,
		`GRANT TEMPORARY ON DATABASE espacigo TO espacigo_schema_owner`,
		`REVOKE TEMPORARY ON DATABASE espacigo FROM PUBLIC`,
	} {
		if _, err := admin.Exec(ctx, statement); err != nil {
			return err
		}
	}
	target, err := pgx.Connect(ctx, strings.Replace(adminDSN, "/postgres?", "/espacigo?", 1))
	if err != nil {
		return err
	}
	defer target.Close(context.Background())
	for _, statement := range []string{
		`CREATE EXTENSION IF NOT EXISTS btree_gist`,
		`CREATE EXTENSION IF NOT EXISTS postgis`,
		`CREATE EXTENSION IF NOT EXISTS unaccent`,
		`GRANT USAGE, CREATE ON SCHEMA public TO espacigo_schema_owner`,
		`GRANT CREATE ON SCHEMA public TO espacigo_migrator_login`,
	} {
		if _, err := target.Exec(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}

func repositoryMigrationsDir(t *testing.T) (string, string) {
	t.Helper()
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test source path")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(sourceFile), "..", ".."))
	return root, filepath.Join(root, "db", "migrations")
}

func readPrivateTestBundle(t *testing.T, directory string) ([]byte, bundleFiles) {
	t.Helper()
	info, err := os.Lstat(directory)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0700 || !strings.Contains(filepath.Base(directory), "gcp04-w4-baseline-") {
		t.Fatal("W6 fixture must be the retained private bundle with mode 0700")
	}
	files := bundleFiles{}
	for _, name := range []string{"baseline.manifest.json", "baseline.dump", "baseline.toc.original", "baseline.toc.restore", "baseline.acl.restore.sql"} {
		path := filepath.Join(directory, name)
		fileInfo, err := os.Lstat(path)
		if err != nil || !fileInfo.Mode().IsRegular() || fileInfo.Mode()&os.ModeSymlink != 0 || fileInfo.Mode().Perm() != 0600 {
			t.Fatalf("private W6 fixture %s must be a mode 0600 regular file", name)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read private W6 fixture %s: %v", name, err)
		}
		files[name] = data
	}
	if hexDigest(files["baseline.dump"]) != testDumpSHA256 {
		t.Fatal("private W6 fixture dump SHA-256 differs from the reviewed retained archive")
	}
	return files["baseline.manifest.json"], files
}

func createPrivateRestoreTestDir(t *testing.T, root string) string {
	t.Helper()
	parent := filepath.Join(root, ".local", "testing")
	if err := os.MkdirAll(parent, 0700); err != nil {
		t.Fatalf("create private test-artifact directory: %v", err)
	}
	parentInfo, err := os.Lstat(parent)
	if err != nil || !parentInfo.IsDir() || parentInfo.Mode()&os.ModeSymlink != 0 || parentInfo.Mode().Perm() != 0700 {
		t.Fatal("private test-artifact directory must be a real mode 0700 directory")
	}
	directory, err := os.MkdirTemp(parent, "gcp04-w6-restore-atomicity-")
	if err != nil {
		t.Fatalf("create private restore SQL workspace: %v", err)
	}
	if err := os.Chmod(directory, 0700); err != nil {
		t.Fatalf("protect private restore SQL workspace: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(directory) })
	return directory
}

func runRestorePSQL(ctx context.Context, databaseURL, sqlPath, privateDir string) ([]byte, error) {
	passfile := filepath.Join(privateDir, "empty.pgpass")
	if err := os.WriteFile(passfile, nil, 0600); err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, "psql", "-X", "-q", "-A", "-t", "-v", "ON_ERROR_STOP=1", "-v", "VERBOSITY=verbose", "--dbname="+databaseURL, "--file="+sqlPath)
	cmd.Env = testPostgresEnvironment(os.Environ(), passfile)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return stdout.Bytes(), fmt.Errorf("psql restore failed (%s): %w", strings.TrimSpace(stderr.String()), err)
	}
	return stdout.Bytes(), nil
}

func testPostgresEnvironment(source []string, passfile string) []string {
	blocked := map[string]bool{"PGHOST": true, "PGPORT": true, "PGDATABASE": true, "PGUSER": true, "PGPASSWORD": true, "PGPASSFILE": true, "PGSERVICE": true, "PGSERVICEFILE": true, "PGOPTIONS": true, "PGSSLMODE": true}
	out := make([]string, 0, len(source)+1)
	for _, entry := range source {
		key, _, ok := strings.Cut(entry, "=")
		if !ok || !blocked[key] {
			out = append(out, entry)
		}
	}
	return append(out, "PGPASSFILE="+passfile)
}

func assertV59RollbackState(t *testing.T, ctx context.Context, conn *pgx.Conn, expected []migrationRow) {
	t.Helper()
	rows, err := conn.Query(ctx, `SELECT version, name, checksum FROM public.schema_migrations ORDER BY version`)
	if err != nil {
		t.Fatal(err)
	}
	index := 0
	for rows.Next() {
		var actual migrationRow
		if err := rows.Scan(&actual.Version, &actual.Name, &actual.Checksum); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		if index >= len(expected) || actual != expected[index] {
			rows.Close()
			t.Fatalf("rollback changed V1-V59 ledger at row %d: got=%+v", index+1, actual)
		}
		index++
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		t.Fatal(err)
	}
	rows.Close()
	var v60TriggerFunctionExists bool
	if err := conn.QueryRow(ctx, `SELECT to_regprocedure('public.reject_gadm_bootstrap_identity_change()') IS NOT NULL`).Scan(&v60TriggerFunctionExists); err != nil {
		t.Fatal(err)
	}
	if index != 59 || v60TriggerFunctionExists {
		t.Fatalf("pre-COMMIT failure did not roll back clean/restore: ledger_rows=%d V60_trigger_function_exists=%t", index, v60TriggerFunctionExists)
	}
}

func assertV64RestoreState(t *testing.T, ctx context.Context, conn *pgx.Conn, expected []migrationRow) {
	t.Helper()
	rows, err := conn.Query(ctx, `SELECT version, name, checksum FROM public.schema_migrations ORDER BY version`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	index := 0
	for rows.Next() {
		var actual migrationRow
		if err := rows.Scan(&actual.Version, &actual.Name, &actual.Checksum); err != nil {
			t.Fatal(err)
		}
		if index >= len(expected) || actual != expected[index] {
			t.Fatalf("restored ledger mismatch at row %d: got=%+v", index+1, actual)
		}
		index++
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if index != 64 {
		t.Fatalf("restored ledger has %d rows, want 64", index)
	}
	var publicDatabaseACL, publicSchemaACL, observerACL, v60FunctionExists bool
	if err := conn.QueryRow(ctx, `
SELECT
  NOT EXISTS (SELECT 1 FROM pg_catalog.pg_database d CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(d.datacl,pg_catalog.acldefault('d',d.datdba))) a WHERE d.datname='espacigo' AND a.grantee=0 AND a.privilege_type IN ('CREATE','TEMPORARY')),
  (NOT EXISTS (SELECT 1 FROM pg_catalog.pg_namespace n CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(n.nspacl,pg_catalog.acldefault('n',n.nspowner))) a WHERE n.nspname='public' AND a.grantee=0 AND a.privilege_type='CREATE') AND EXISTS (SELECT 1 FROM pg_catalog.pg_namespace n CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(n.nspacl,pg_catalog.acldefault('n',n.nspowner))) a WHERE n.nspname='public' AND a.grantee=0 AND a.privilege_type='USAGE')),
	(has_database_privilege('espacigo_db_observer','espacigo','CONNECT') AND NOT has_database_privilege('espacigo_db_observer','espacigo','TEMPORARY') AND NOT has_database_privilege('espacigo_db_observer','espacigo','CREATE') AND has_schema_privilege('espacigo_db_observer','public','USAGE') AND NOT has_schema_privilege('espacigo_db_observer','public','CREATE') AND has_table_privilege('espacigo_db_observer','public.schema_migrations','SELECT') AND NOT has_table_privilege('espacigo_db_observer','public.schema_migrations','INSERT') AND NOT has_table_privilege('espacigo_db_observer','public.schema_migrations','UPDATE') AND NOT has_table_privilege('espacigo_db_observer','public.schema_migrations','DELETE') AND NOT has_table_privilege('espacigo_db_observer','public.schema_migrations','TRUNCATE')),
	to_regprocedure('public.reject_gadm_bootstrap_identity_change()') IS NOT NULL`).Scan(&publicDatabaseACL, &publicSchemaACL, &observerACL, &v60FunctionExists); err != nil {
		t.Fatal(err)
	}
	if !publicDatabaseACL || !publicSchemaACL || !observerACL || !v60FunctionExists {
		t.Fatalf("restored ACL/schema check failed: database=%t public_schema=%t observer=%t V60_function=%t", publicDatabaseACL, publicSchemaACL, observerACL, v60FunctionExists)
	}
	var usuarioRows int
	if err := conn.QueryRow(ctx, `SELECT count(*) FROM public.usuario`).Scan(&usuarioRows); err != nil || usuarioRows != 0 {
		t.Fatalf("restored usuario rows=%d err=%v", usuarioRows, err)
	}
}

func gadmRoleFingerprint(ctx context.Context, conn *pgx.Conn) (string, error) {
	var fingerprint string
	err := conn.QueryRow(ctx, `SELECT json_build_object(
  'role', (SELECT json_build_object('login',rolcanlogin,'super',rolsuper,'createdb',rolcreatedb,'createrole',rolcreaterole,'replication',rolreplication,'bypassrls',rolbypassrls,'inherit',rolinherit,'connlimit',rolconnlimit,'comment',shobj_description(oid,'pg_authid')) FROM pg_catalog.pg_roles WHERE rolname='espacigo_gadm_bootstrap'),
  'memberships', (SELECT count(*) FROM pg_catalog.pg_auth_members m JOIN pg_catalog.pg_roles r ON r.oid=m.roleid JOIN pg_catalog.pg_roles u ON u.oid=m.member WHERE r.rolname='espacigo_gadm_bootstrap' OR u.rolname='espacigo_gadm_bootstrap'),
  'settings', (SELECT count(*) FROM pg_catalog.pg_db_role_setting s JOIN pg_catalog.pg_roles r ON r.oid=s.setrole WHERE r.rolname='espacigo_gadm_bootstrap'))::text`).Scan(&fingerprint)
	return fingerprint, err
}

func cleanupFixture(t *testing.T, adminDSN string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	admin, err := pgx.Connect(ctx, adminDSN)
	if err != nil {
		t.Errorf("connect to dispose test database: %v", err)
		return
	}
	defer admin.Close(context.Background())
	_, _ = admin.Exec(ctx, `DROP DATABASE IF EXISTS espacigo WITH (FORCE)`)
	_, _ = admin.Exec(ctx, `REVOKE espacigo_schema_owner FROM espacigo_migrator_login`)
	_, _ = admin.Exec(ctx, `DROP ROLE IF EXISTS espacigo_migrator_login, espacigo_db_observer, espacigo_gadm_bootstrap, espacigo_privacy_cleanup_worker, espacigo_runtime, espacigo_schema_owner`)
}
