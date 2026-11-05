package dbbootstrap

import (
	"context"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/migrator"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const runtimeACLReadbackF5Database = "espacigo_f5_gate_20261101"
const runtimeACLReadbackF5AdminMarker = "f5-confirmed:espacigo_f5_gate_20261101"

func assertF5Postgres18CatalogCoverage(t *testing.T, ctx context.Context, conn *pgx.Conn) {
	t.Helper()
	tx, err := conn.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		t.Fatal("could not start read-only PostgreSQL 18 catalog inventory")
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	readNames := func(query string) []string {
		rows, err := tx.Query(ctx, query)
		if err != nil {
			t.Fatalf("read PostgreSQL catalog inventory: %v", err)
		}
		defer rows.Close()
		var names []string
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				t.Fatalf("decode PostgreSQL catalog inventory: %v", err)
			}
			names = append(names, name)
		}
		if err := rows.Err(); err != nil {
			t.Fatalf("finish PostgreSQL catalog inventory: %v", err)
		}
		return names
	}
	aclColumns := readNames(`SELECT c.relname||'.'||a.attname
		FROM pg_catalog.pg_class c
		JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
		JOIN pg_catalog.pg_attribute a ON a.attrelid=c.oid
		WHERE n.nspname='pg_catalog' AND a.attnum>0 AND NOT a.attisdropped
		  AND a.atttypid='pg_catalog.aclitem[]'::pg_catalog.regtype
		ORDER BY c.relname,a.attnum`)
	wantACLColumns := []string{
		"pg_attribute.attacl", "pg_class.relacl", "pg_database.datacl", "pg_default_acl.defaclacl",
		"pg_foreign_data_wrapper.fdwacl", "pg_foreign_server.srvacl", "pg_init_privs.initprivs",
		"pg_language.lanacl", "pg_largeobject_metadata.lomacl", "pg_namespace.nspacl",
		"pg_parameter_acl.paracl", "pg_proc.proacl", "pg_tablespace.spcacl", "pg_type.typacl",
	}
	if strings.Join(aclColumns, "\n") != strings.Join(wantACLColumns, "\n") {
		t.Fatalf("PostgreSQL 18 aclitem[] catalog inventory changed: %v", aclColumns)
	}
	ownerColumns := readNames(`SELECT c.relname||'.'||a.attname
		FROM pg_catalog.pg_class c
		JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
		JOIN pg_catalog.pg_attribute a ON a.attrelid=c.oid
		WHERE n.nspname='pg_catalog' AND a.attnum>0 AND NOT a.attisdropped
		  AND a.atttypid='pg_catalog.oid'::pg_catalog.regtype
		  AND (a.attname LIKE '%owner' OR a.attname='datdba')
		ORDER BY c.relname,a.attnum`)
	wantOwnerColumns := []string{
		"pg_class.relowner", "pg_collation.collowner", "pg_conversion.conowner", "pg_database.datdba",
		"pg_event_trigger.evtowner", "pg_extension.extowner", "pg_foreign_data_wrapper.fdwowner",
		"pg_foreign_server.srvowner", "pg_language.lanowner", "pg_largeobject_metadata.lomowner",
		"pg_namespace.nspowner", "pg_opclass.opcowner", "pg_operator.oprowner", "pg_opfamily.opfowner",
		"pg_proc.proowner", "pg_publication.pubowner", "pg_statistic_ext.stxowner",
		"pg_subscription.subowner", "pg_tablespace.spcowner", "pg_ts_config.cfgowner",
		"pg_ts_dict.dictowner", "pg_type.typowner",
	}
	if strings.Join(ownerColumns, "\n") != strings.Join(wantOwnerColumns, "\n") {
		t.Fatalf("PostgreSQL 18 ownership catalog inventory changed: %v", ownerColumns)
	}
	defaultACLRoleColumns := readNames(`SELECT c.relname||'.'||a.attname
		FROM pg_catalog.pg_class c
		JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
		JOIN pg_catalog.pg_attribute a ON a.attrelid=c.oid
		WHERE n.nspname='pg_catalog' AND c.relname='pg_default_acl'
		  AND a.attname='defaclrole' AND a.attnum>0 AND NOT a.attisdropped
		  AND a.atttypid='pg_catalog.oid'::pg_catalog.regtype
		ORDER BY c.relname,a.attnum`)
	if strings.Join(defaultACLRoleColumns, "\n") != "pg_default_acl.defaclrole" {
		t.Fatalf("PostgreSQL default-privilege authority catalog inventory changed: %v", defaultACLRoleColumns)
	}
}

// This opt-in gate executes the catalog readback against the authorized local
// F5 target. It is intentionally separate from unit tests and never mutates DB.
func TestVerifyRuntimeACLReadbackF5Gate(t *testing.T) {
	_, config := f5ReadbackTestConfig(t)

	ctx := context.Background()
	conn, err := pgx.ConnectConfig(ctx, config)
	if err != nil {
		t.Fatal("could not connect to the exact F5 ACL readback target")
	}
	defer conn.Close(context.Background())
	var database, sessionUser, currentUser string
	if err := conn.QueryRow(ctx, "SELECT current_database(), session_user, current_user").Scan(&database, &sessionUser, &currentUser); err != nil ||
		database != runtimeACLReadbackF5Database || sessionUser != "postgres" || currentUser != "postgres" {
		t.Fatal("F5 ACL readback database identity mismatch")
	}

	tx, err := conn.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		t.Fatal("could not start the read-only F5 ACL inspection transaction")
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	migrations, err := migrator.DiscoverMigrations(filepath.Join("..", "..", "db", "migrations"))
	if err != nil {
		t.Fatal(err)
	}
	rows, err := tx.Query(ctx, `SELECT version,name,checksum FROM public.schema_migrations ORDER BY version`)
	if err != nil {
		t.Fatal("could not read the F5 applied migration ledger")
	}
	var sources []RuntimeACLSource
	index := 0
	for rows.Next() {
		var version int64
		var name, checksum string
		if err := rows.Scan(&version, &name, &checksum); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		if index >= len(migrations) || migrations[index].Version != version || migrations[index].Name != name || migrations[index].Checksum != checksum {
			rows.Close()
			t.Fatalf("F5 ledger/source mismatch at ordered entry %d", index)
		}
		sources = append(sources, RuntimeACLSource{Version: int(version), SQL: migrations[index].SQL})
		index++
	}
	rowsErr := rows.Err()
	rows.Close()
	if rowsErr != nil {
		t.Fatal("could not finish reading the F5 migration ledger")
	}
	if index == 0 {
		t.Fatal(errors.New("F5 migration ledger is empty"))
	}
	if err := VerifyRuntimeACLReadbackF5(ctx, tx, sources); err != nil {
		t.Fatalf("complete F5 runtime ACL catalog readback failed: %v", err)
	}
}

func f5ReadbackTestConfig(t *testing.T) (string, *pgx.ConnConfig) {
	t.Helper()
	dsn := os.Getenv("ESPACIGO_RUNTIME_ACL_READBACK_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set the explicit F5 ACL readback DSN to run the PostgreSQL gate")
	}
	parsedDSN, err := url.Parse(dsn)
	if err != nil || (parsedDSN.Scheme != "postgres" && parsedDSN.Scheme != "postgresql") ||
		parsedDSN.Host != "127.0.0.1:15434" || parsedDSN.Path != "/"+runtimeACLReadbackF5Database ||
		parsedDSN.User == nil || parsedDSN.User.Username() != "postgres" {
		t.Fatal("F5 ACL readback gate requires a postgres URI for the exact loopback target")
	}
	if _, hasInlinePassword := parsedDSN.User.Password(); hasInlinePassword {
		t.Fatal("F5 ACL readback gate rejects inline passwords; use the private PGPASSFILE")
	}
	config, err := pgx.ParseConfig(dsn)
	if err != nil || config.Host != "127.0.0.1" || config.Port != 15434 ||
		config.Database != runtimeACLReadbackF5Database || config.User != "postgres" {
		t.Fatal("F5 ACL readback gate parsed an unexpected target identity")
	}
	passfile := os.Getenv("PGPASSFILE")
	if !filepath.IsAbs(passfile) || filepath.Clean(passfile) == "/dev/null" {
		t.Fatal("F5 ACL readback gate requires an absolute private passfile")
	}
	info, err := os.Lstat(passfile)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
		t.Fatal("F5 ACL readback gate requires a regular passfile with mode 0600")
	}
	if os.Getenv("PGPASSWORD") != "" || os.Getenv("PGSERVICE") != "" || os.Getenv("PGSERVICEFILE") != "" {
		t.Fatal("F5 ACL readback gate rejects alternate credential sources")
	}
	return dsn, config
}

func TestF5MigrationAdvisoryLockSerializesSessions(t *testing.T) {
	dsn, config := f5ReadbackTestConfig(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	first, err := pgx.ConnectConfig(ctx, config)
	if err != nil {
		t.Fatal("could not connect to the exact F5 lock-test target")
	}
	defer first.Close(context.Background())
	if err := verifyF5MigrationAdvisoryLock(ctx, dsn, first); err != nil {
		t.Fatalf("F5 migration advisory-lock integration: %v", err)
	}
}

func verifyF5MigrationAdvisoryLock(ctx context.Context, dsn string, first *pgx.Conn) error {
	lockCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	secondConfig, err := pgx.ParseConfig(dsn)
	if err != nil {
		return errors.New("parse second F5 lock-test connection")
	}
	second, err := pgx.ConnectConfig(lockCtx, secondConfig)
	if err != nil {
		return errors.New("connect second F5 lock-test session")
	}
	defer second.Close(context.Background())
	release, err := migrator.AcquireMigrationAdvisoryLock(lockCtx, first)
	if err != nil {
		return errors.New("acquire production migration advisory lock")
	}
	released := false
	defer func() {
		if !released {
			_ = release(context.Background())
		}
	}()
	var lockClass, lockObject int32
	if err := first.QueryRow(lockCtx, `SELECT classid::int4,objid::int4 FROM pg_catalog.pg_locks WHERE pid=pg_backend_pid() AND locktype='advisory' AND granted AND objsubid=2`).Scan(&lockClass, &lockObject); err != nil {
		return errors.New("read production advisory-lock key")
	}
	var acquired bool
	if err := second.QueryRow(lockCtx, `SELECT pg_catalog.pg_try_advisory_lock($1,$2)`, lockClass, lockObject).Scan(&acquired); err != nil || acquired {
		return errors.New("second session did not observe migration lock contention")
	}
	if err := release(lockCtx); err != nil {
		return errors.New("release production migration advisory lock")
	}
	released = true
	if err := second.QueryRow(lockCtx, `SELECT pg_catalog.pg_try_advisory_lock($1,$2)`, lockClass, lockObject).Scan(&acquired); err != nil || !acquired {
		return errors.New("second session could not acquire released migration lock")
	}
	var unlocked bool
	if err := second.QueryRow(lockCtx, `SELECT pg_catalog.pg_advisory_unlock($1,$2)`, lockClass, lockObject).Scan(&unlocked); err != nil || !unlocked {
		return errors.New("second session could not release migration lock")
	}
	return nil
}

// This preview verifies the pre-V81 role/ACL contract, executes V81 and the
// complete V82 migration through the explicitly authorized F5 admin path,
// then reads back the post-V81/V82 catalog state in a transaction rolled back
// at the end. It does not prove command commits.
func TestRuntimeACLReadbackF5RollbackPreview(t *testing.T) {
	dsn := os.Getenv("ESPACIGO_RUNTIME_ACL_READBACK_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set the explicit F5 ACL readback DSN to run the PostgreSQL rollback preview")
	}
	parsedDSN, err := url.Parse(dsn)
	if err != nil || (parsedDSN.Scheme != "postgres" && parsedDSN.Scheme != "postgresql") ||
		parsedDSN.Host != "127.0.0.1:15434" || parsedDSN.Path != "/"+runtimeACLReadbackF5Database ||
		parsedDSN.User == nil || parsedDSN.User.Username() != "postgres" {
		t.Fatal("F5 ACL preview requires a postgres URI for the exact loopback target")
	}
	if _, hasInlinePassword := parsedDSN.User.Password(); hasInlinePassword {
		t.Fatal("F5 ACL preview rejects inline passwords; use the private PGPASSFILE")
	}
	config, err := pgx.ParseConfig(dsn)
	if err != nil || config.Host != "127.0.0.1" || config.Port != 15434 ||
		config.Database != runtimeACLReadbackF5Database || config.User != "postgres" {
		t.Fatal("F5 ACL preview parsed an unexpected target identity")
	}
	passfile := os.Getenv("PGPASSFILE")
	info, err := os.Lstat(passfile)
	if err != nil || !filepath.IsAbs(passfile) || filepath.Clean(passfile) == "/dev/null" ||
		!info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
		t.Fatal("F5 ACL preview requires an absolute regular mode-0600 private passfile")
	}
	if os.Getenv("PGPASSWORD") != "" || os.Getenv("PGSERVICE") != "" || os.Getenv("PGSERVICEFILE") != "" {
		t.Fatal("F5 ACL preview rejects alternate credential sources")
	}

	ctx := context.Background()
	conn, err := pgx.ConnectConfig(ctx, config)
	if err != nil {
		t.Fatal("could not connect to the exact F5 ACL preview target")
	}
	defer conn.Close(context.Background())
	var database, sessionUser, currentUser string
	if err := conn.QueryRow(ctx, "SELECT current_database(), session_user, current_user").Scan(&database, &sessionUser, &currentUser); err != nil ||
		database != runtimeACLReadbackF5Database || sessionUser != "postgres" || currentUser != "postgres" {
		t.Fatal("F5 ACL preview database identity mismatch")
	}
	assertF5Postgres18CatalogCoverage(t, ctx, conn)

	migrations, err := migrator.DiscoverMigrations(filepath.Join("..", "..", "db", "migrations"))
	if err != nil {
		t.Fatal(err)
	}
	rows, err := conn.Query(ctx, `SELECT version,name,checksum FROM public.schema_migrations ORDER BY version`)
	if err != nil {
		t.Fatal("could not read the F5 applied migration ledger")
	}
	var sources []RuntimeACLSource
	index := 0
	for rows.Next() {
		var version int64
		var name, checksum string
		if err := rows.Scan(&version, &name, &checksum); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		if index >= len(migrations) || migrations[index].Version != version || migrations[index].Name != name || migrations[index].Checksum != checksum {
			rows.Close()
			t.Fatalf("F5 ledger/source mismatch at ordered entry %d", index)
		}
		sources = append(sources, RuntimeACLSource{Version: int(version), SQL: migrations[index].SQL})
		index++
	}
	rowsErr := rows.Err()
	rows.Close()
	if rowsErr != nil || index != 80 {
		t.Fatal("could not verify the complete V1-V80 F5 migration ledger")
	}

	tx, err := conn.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		t.Fatal("could not start the rollback-only F5 ACL preview transaction")
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	roleBootstrapSQL, err := os.ReadFile(filepath.Join("..", "..", "db", "repair", "f5_pre_v81_role_contract.sql"))
	if err != nil {
		t.Fatalf("read guarded F5 pre-V81 role bootstrap: %v", err)
	}
	if err := verifyRuntimeACLF5PublicBaseline(ctx, tx); err != nil {
		t.Fatalf("read the approved PUBLIC/default baseline before the pre-V81 role bootstrap: %v", err)
	}
	roleBootstrapDriftFixtures := []struct {
		name, setupSQL, wantError string
	}{
		{
			name:      "unexpected PUBLIC ledger grant",
			setupSQL:  `GRANT SELECT ON TABLE public.schema_migrations TO PUBLIC`,
			wantError: "approved PUBLIC/default ACL baseline",
		},
		{
			name:      "unexpected default ACL",
			setupSQL:  `ALTER DEFAULT PRIVILEGES FOR ROLE postgres IN SCHEMA public GRANT SELECT ON TABLES TO PUBLIC`,
			wantError: "approved PUBLIC/default ACL baseline",
		},
		{
			name:      "unexpected non-PUBLIC ledger grant",
			setupSQL:  `GRANT SELECT ON TABLE public.schema_migrations TO espacigo_privacy_cleanup_worker`,
			wantError: "unexpected direct ACL on migration ledger",
		},
		{
			name:      "unexpected owner ledger ACL delta",
			setupSQL:  `REVOKE SELECT ON TABLE public.schema_migrations FROM postgres`,
			wantError: "unexpected direct ACL on migration ledger",
		},
	}
	for _, fixture := range roleBootstrapDriftFixtures {
		if _, err := tx.Exec(ctx, `SAVEPOINT pre_v81_role_bootstrap_drift_probe`); err != nil {
			t.Fatalf("start %s pre-V81 contract probe: %v", fixture.name, err)
		}
		if _, err := tx.Exec(ctx, fixture.setupSQL); err != nil {
			t.Fatalf("create transactional %s fixture: %v", fixture.name, err)
		}
		_, bootstrapErr := tx.Conn().PgConn().Exec(ctx, string(roleBootstrapSQL)).ReadAll()
		if bootstrapErr == nil || !strings.Contains(bootstrapErr.Error(), fixture.wantError) {
			t.Fatalf("pre-V81 role bootstrap must reject %s, got %v", fixture.name, bootstrapErr)
		}
		if _, err := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT pre_v81_role_bootstrap_drift_probe`); err != nil {
			t.Fatalf("rollback %s pre-V81 contract probe: %v", fixture.name, err)
		}
		if _, err := tx.Exec(ctx, `RELEASE SAVEPOINT pre_v81_role_bootstrap_drift_probe`); err != nil {
			t.Fatalf("release %s pre-V81 contract probe: %v", fixture.name, err)
		}
	}
	if err := verifyRuntimeACLF5PublicBaseline(ctx, tx); err != nil {
		t.Fatalf("negative pre-V81 fixtures changed the transaction's approved PUBLIC/default baseline: %v", err)
	}
	if _, err := tx.Conn().PgConn().Exec(ctx, string(roleBootstrapSQL)).ReadAll(); err != nil {
		t.Fatalf("guarded F5 pre-V81 role bootstrap failed in rollback preview: %v", err)
	}
	if err := ApplyRuntimeACLFromMigrationsF5(ctx, tx, sources); err != nil {
		t.Fatalf("apply source-derived V1-V80 ACLs in preview transaction: %v", err)
	}
	if err := GrantRuntimePermissions(ctx, tx); err != nil {
		t.Fatalf("apply shared runtime ACL allowlist in preview transaction: %v", err)
	}
	if err := VerifyRuntimeACLReadbackF5(ctx, tx, sources); err != nil {
		t.Fatalf("complete F5 runtime ACL catalog readback failed in rollback preview: %v", err)
	}
	v82SQL, err := os.ReadFile(filepath.Join("..", "..", "db", "migrations", "V000082__document_verification_retention_precedence.sql"))
	if err != nil {
		t.Fatalf("read V82 source for transactional authority probe: %v", err)
	}
	v82Statements, err := splitACLStatements(string(v82SQL))
	v82FunctionStatementIndex := -1
	for i, statement := range v82Statements {
		if strings.HasPrefix(strings.TrimSpace(statement), "CREATE OR REPLACE FUNCTION public.document_verification_space_owner_changed()") {
			v82FunctionStatementIndex = i
			break
		}
	}
	if err != nil || v82FunctionStatementIndex < 0 {
		t.Fatalf("extract V82 function replacement statement: statements=%d err=%v", len(v82Statements), err)
	}
	lockConn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal("could not open the second F5 session for V82 catalog-lock recovery probe")
	}
	defer lockConn.Close(context.Background())
	lockTx, err := lockConn.Begin(ctx)
	if err != nil {
		t.Fatal("could not start the concurrent F5 catalog-lock transaction")
	}
	lockTxOpen := true
	defer func() {
		if lockTxOpen {
			_ = lockTx.Rollback(context.Background())
		}
	}()
	var lockedFunctionOID uint32
	if err := lockTx.QueryRow(ctx, `SELECT p.oid FROM pg_catalog.pg_proc p WHERE p.oid='public.document_verification_space_owner_changed()'::regprocedure FOR UPDATE`).Scan(&lockedFunctionOID); err != nil || lockedFunctionOID == 0 {
		t.Fatal("could not acquire the concurrent V82 function-row lock")
	}
	if _, err := tx.Exec(ctx, `SAVEPOINT v82_concurrent_catalog_lock_probe`); err != nil {
		t.Fatalf("start V82 concurrent catalog-lock probe: %v", err)
	}
	if _, err := tx.Exec(ctx, `SET LOCAL lock_timeout='250ms'`); err != nil {
		t.Fatalf("set bounded V82 catalog-lock probe timeout: %v", err)
	}
	_, v82LockErr := tx.Exec(ctx, v82Statements[v82FunctionStatementIndex])
	var lockPGErr *pgconn.PgError
	if !errors.As(v82LockErr, &lockPGErr) || lockPGErr.Code != "55P03" {
		t.Fatalf("V82 function replacement should fail on the controlled concurrent catalog-row lock, got %v", v82LockErr)
	}
	if _, err := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT v82_concurrent_catalog_lock_probe`); err != nil {
		t.Fatalf("recover the V82 preview transaction after catalog-lock failure: %v", err)
	}
	if _, err := tx.Exec(ctx, `RELEASE SAVEPOINT v82_concurrent_catalog_lock_probe`); err != nil {
		t.Fatalf("release V82 concurrent catalog-lock savepoint: %v", err)
	}
	if err := lockTx.Rollback(ctx); err != nil {
		t.Fatalf("release concurrent V82 catalog-row lock: %v", err)
	}
	lockTxOpen = false
	var v82FunctionOwner string
	var schemaOwnerCanCreate bool
	if err := tx.QueryRow(ctx, `SELECT owner.rolname,pg_catalog.has_schema_privilege('espacigo_schema_owner','public','CREATE') FROM pg_catalog.pg_proc p JOIN pg_catalog.pg_roles owner ON owner.oid=p.proowner WHERE p.oid='public.document_verification_space_owner_changed()'::regprocedure`).Scan(&v82FunctionOwner, &schemaOwnerCanCreate); err != nil {
		t.Fatalf("read V82 function ownership/authority: %v", err)
	}
	if v82FunctionOwner != "postgres" || schemaOwnerCanCreate {
		t.Fatalf("unexpected V82 precondition state: owner=%q schema_create=%t", v82FunctionOwner, schemaOwnerCanCreate)
	}
	if _, err := tx.Exec(ctx, `SAVEPOINT v82_owner_authority_probe`); err != nil {
		t.Fatalf("start V82 owner authority probe: %v", err)
	}
	if _, err := tx.Exec(ctx, `SET SESSION AUTHORIZATION espacigo_migrator_login`); err != nil {
		t.Fatalf("set V82 probe session identity: %v", err)
	}
	if _, err := tx.Exec(ctx, `SET ROLE espacigo_schema_owner`); err != nil {
		t.Fatalf("set V82 probe current role: %v", err)
	}
	_, ownerErr := tx.Exec(ctx, v82Statements[v82FunctionStatementIndex])
	var pgErr *pgconn.PgError
	if !errors.As(ownerErr, &pgErr) || pgErr.Code != "42501" || !strings.Contains(ownerErr.Error(), "permission denied for schema public") {
		t.Fatalf("V82 must fail closed on public-schema CREATE authority under the restricted identity; got %v", ownerErr)
	}
	if _, err := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT v82_owner_authority_probe`); err != nil {
		t.Fatalf("rollback V82 owner authority probe: %v", err)
	}
	if _, err := tx.Exec(ctx, `RELEASE SAVEPOINT v82_owner_authority_probe`); err != nil {
		t.Fatalf("release V82 owner authority probe: %v", err)
	}
	v81SQL, err := os.ReadFile(filepath.Join("..", "..", "db", "migrations", "V000081__document_verification_runtime_scope_helpers.sql"))
	if err != nil {
		t.Fatalf("read V81 source for transactional precondition verification: %v", err)
	}
	v81Statements, err := splitACLStatements(string(v81SQL))
	if err != nil || len(v81Statements) == 0 || !strings.HasPrefix(strings.TrimSpace(v81Statements[0]), "DO ") {
		t.Fatalf("extract V81 precondition block: statements=%d err=%v", len(v81Statements), err)
	}
	var v81HelperExists bool
	if err := tx.QueryRow(ctx, `SELECT to_regprocedure('document_verification_private.document_verification_lock_active_actor(uuid)') IS NOT NULL`).Scan(&v81HelperExists); err != nil {
		t.Fatalf("inspect V81 helper before its precondition: %v", err)
	}
	if v81HelperExists {
		t.Fatal("V81-created helper function unexpectedly exists in the pre-V81 contract")
	}
	if _, err := tx.Exec(ctx, `SAVEPOINT v81_confirmation_probe`); err != nil {
		t.Fatalf("start V81 confirmation-context probe: %v", err)
	}
	if _, err := tx.Exec(ctx, `SELECT pg_catalog.set_config('application_name', 'unconfirmed-preview', true)`); err != nil {
		t.Fatalf("set negative V81 confirmation context: %v", err)
	}
	_, unconfirmedV81Err := tx.Exec(ctx, v81Statements[0])
	if !errors.As(unconfirmedV81Err, &pgErr) || pgErr.Code != "P0001" || !strings.Contains(unconfirmedV81Err.Error(), "confirmed F5 versioned migration context") {
		t.Fatalf("V81 must reject an unconfirmed administrative SQL session, got %v", unconfirmedV81Err)
	}
	if _, err := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT v81_confirmation_probe`); err != nil {
		t.Fatalf("rollback unconfirmed V81 probe: %v", err)
	}
	if _, err := tx.Exec(ctx, `RELEASE SAVEPOINT v81_confirmation_probe`); err != nil {
		t.Fatalf("release unconfirmed V81 probe: %v", err)
	}
	if _, err := tx.Exec(ctx, "SELECT pg_catalog.set_config('application_name', $1, true)", runtimeACLReadbackF5AdminMarker); err != nil {
		t.Fatalf("set the exact confirmed V81 preview context: %v", err)
	}
	if _, err := tx.Exec(ctx, `SAVEPOINT v81_ledger_acl_probe`); err != nil {
		t.Fatalf("start V81 direct-ledger-ACL probe: %v", err)
	}
	if _, err := tx.Exec(ctx, `GRANT UPDATE ON TABLE public.schema_migrations TO espacigo_schema_owner`); err != nil {
		t.Fatalf("create rollback-only V81 excessive-ledger-privilege fixture: %v", err)
	}
	_, excessiveLedgerPrivilegeErr := tx.Exec(ctx, v81Statements[0])
	if !errors.As(excessiveLedgerPrivilegeErr, &pgErr) || pgErr.Code != "P0001" || !strings.Contains(excessiveLedgerPrivilegeErr.Error(), "V81 prerequisite privilege contract") {
		t.Fatalf("V81 must reject an out-of-contract direct ledger privilege, got %v", excessiveLedgerPrivilegeErr)
	}
	if _, err := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT v81_ledger_acl_probe`); err != nil {
		t.Fatalf("rollback V81 direct-ledger-ACL probe: %v", err)
	}
	if _, err := tx.Exec(ctx, `RELEASE SAVEPOINT v81_ledger_acl_probe`); err != nil {
		t.Fatalf("release V81 direct-ledger-ACL probe: %v", err)
	}
	if _, err := tx.Exec(ctx, v81Statements[0]); err != nil {
		t.Fatalf("V81 precondition rejected the required pre-migration role contract: %v", err)
	}
	for i := 1; i < len(v81Statements); i++ {
		statement := v81Statements[i]
		if _, err := tx.Exec(ctx, statement); err != nil {
			t.Fatalf("V81 statement %d failed under the explicitly authorized F5 administrative identity: %v", i+1, err)
		}
	}
	if _, err := tx.Exec(ctx, `RESET ROLE`); err != nil {
		t.Fatalf("reset V81 preview role before administrative readback: %v", err)
	}
	if _, err := tx.Exec(ctx, `RESET SESSION AUTHORIZATION`); err != nil {
		t.Fatalf("reset V81 preview session identity before administrative readback: %v", err)
	}
	sources = append(sources, RuntimeACLSource{Version: 81, SQL: v81SQL})
	if err := VerifyRuntimeACLReadbackF5(ctx, tx, sources); err != nil {
		t.Fatalf("complete F5 catalog readback after transactional V81: %v", err)
	}
	if _, err := tx.Exec(ctx, `SAVEPOINT v82_confirmation_probe`); err != nil {
		t.Fatalf("start V82 confirmation-context probe: %v", err)
	}
	if _, err := tx.Exec(ctx, `SELECT pg_catalog.set_config('application_name', 'unconfirmed-preview', true)`); err != nil {
		t.Fatalf("set negative V82 confirmation context: %v", err)
	}
	_, unconfirmedV82Err := tx.Exec(ctx, v82Statements[0])
	if !errors.As(unconfirmedV82Err, &pgErr) || pgErr.Code != "P0001" || !strings.Contains(unconfirmedV82Err.Error(), "confirmed F5 versioned migration context") {
		t.Fatalf("V82 must reject an unconfirmed administrative SQL session, got %v", unconfirmedV82Err)
	}
	if _, err := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT v82_confirmation_probe`); err != nil {
		t.Fatalf("rollback unconfirmed V82 probe: %v", err)
	}
	if _, err := tx.Exec(ctx, `RELEASE SAVEPOINT v82_confirmation_probe`); err != nil {
		t.Fatalf("release unconfirmed V82 probe: %v", err)
	}
	if _, err := tx.Exec(ctx, "SELECT pg_catalog.set_config('application_name', $1, true)", runtimeACLReadbackF5AdminMarker); err != nil {
		t.Fatalf("set the exact confirmed V82 preview context: %v", err)
	}
	for i, statement := range v82Statements {
		if _, err := tx.Exec(ctx, statement); err != nil {
			t.Fatalf("V82 statement %d failed under the explicitly authorized F5 administrative identity: %v", i+1, err)
		}
	}
	var v82LedgerRole string
	if err := tx.QueryRow(ctx, `SELECT current_user`).Scan(&v82LedgerRole); err != nil || v82LedgerRole != "espacigo_schema_owner" {
		t.Fatalf("V82 must return to schema_owner before the runner records the migration: role=%q err=%v", v82LedgerRole, err)
	}
	if _, err := tx.Exec(ctx, `RESET ROLE`); err != nil {
		t.Fatalf("reset the V82 preview role before administrative readback: %v", err)
	}
	sources = append(sources, RuntimeACLSource{Version: 82, SQL: v82SQL})
	if err := VerifyRuntimeACLReadbackF5(ctx, tx, sources); err != nil {
		t.Fatalf("F5 PUBLIC/default baseline changed after V82 branch: %v", err)
	}
	if _, err := tx.Exec(ctx, `RESET ROLE`); err != nil {
		t.Fatalf("reset F5 migration preview role: %v", err)
	}
	if _, err := tx.Exec(ctx, `RESET SESSION AUTHORIZATION`); err != nil {
		t.Fatalf("reset F5 migration preview session identity: %v", err)
	}
	systemACLFixtures := []struct {
		name, grantSQL string
	}{
		{"relation", `GRANT SELECT ON TABLE pg_catalog.pg_class TO espacigo_runtime`},
		{"column", `GRANT SELECT (relname) ON TABLE pg_catalog.pg_class TO espacigo_runtime`},
		{"function", `GRANT EXECUTE ON FUNCTION pg_catalog.abs(integer) TO espacigo_runtime`},
		{"schema", `GRANT USAGE ON SCHEMA pg_catalog TO espacigo_runtime`},
		{"type", `GRANT USAGE ON TYPE pg_catalog.int4 TO espacigo_runtime`},
	}
	for _, fixture := range systemACLFixtures {
		if _, err := tx.Exec(ctx, `SAVEPOINT system_acl_probe`); err != nil {
			t.Fatalf("start %s catalog ACL probe: %v", fixture.name, err)
		}
		if _, err := tx.Exec(ctx, fixture.grantSQL); err != nil {
			t.Fatalf("create %s system-catalog ACL fixture: %v", fixture.name, err)
		}
		if err := rejectUnmodeledRuntimeACLClasses(ctx, tx, true); err == nil {
			t.Errorf("runtime direct %s ACL on a system catalog must fail closed", fixture.name)
		}
		if _, err := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT system_acl_probe`); err != nil {
			t.Fatalf("rollback %s catalog ACL probe: %v", fixture.name, err)
		}
		if _, err := tx.Exec(ctx, `RELEASE SAVEPOINT system_acl_probe`); err != nil {
			t.Fatalf("release %s catalog ACL probe: %v", fixture.name, err)
		}
	}
	var largeObjectOID uint32
	if err := tx.QueryRow(ctx, `SELECT pg_catalog.lo_create(0)`).Scan(&largeObjectOID); err != nil {
		t.Fatalf("create transactional large-object ownership fixture: %v", err)
	}
	if _, err := tx.Exec(ctx, `ALTER LARGE OBJECT `+strconv.FormatUint(uint64(largeObjectOID), 10)+` OWNER TO espacigo_runtime`); err != nil {
		t.Fatalf("assign transactional large-object fixture to runtime: %v", err)
	}
	if err := VerifyRuntimeACLReadbackF5(ctx, tx, sources); err == nil || err.Error() != "runtime role owns a database, schema, or catalog object" {
		t.Fatalf("complete F5 readback must reject runtime-owned large objects, got %v", err)
	}
	if err := rejectUnmodeledRuntimeACLClasses(ctx, tx, false); err != nil {
		t.Fatalf("unmodeled ACL catalogs were not clean before the default-role fixture: %v", err)
	}
	if _, err := tx.Exec(ctx, `ALTER DEFAULT PRIVILEGES FOR ROLE espacigo_runtime IN SCHEMA public GRANT SELECT ON TABLES TO espacigo_api_login`); err != nil {
		t.Fatalf("create default-ACL authority fixture for runtime: %v", err)
	}
	var runtimeDefaultACLExists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (
		SELECT 1 FROM pg_catalog.pg_default_acl d
		CROSS JOIN LATERAL pg_catalog.aclexplode(d.defaclacl) a
		WHERE d.defaclrole='espacigo_runtime'::regrole AND d.defaclobjtype='r'
		  AND a.grantee='espacigo_api_login'::regrole AND a.privilege_type='SELECT')`).Scan(&runtimeDefaultACLExists); err != nil || !runtimeDefaultACLExists {
		t.Fatalf("runtime-owned API table default ACL fixture was not created: exists=%t err=%v", runtimeDefaultACLExists, err)
	}
	if err := rejectUnmodeledRuntimeACLClasses(ctx, tx, false); err == nil {
		t.Fatal("runtime-owned default privileges must be rejected even when their ACL grantee is another limited role")
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatalf("could not roll back F5 ACL preview: %v", err)
	}
	if err := verifyRuntimeACLF5PublicBaseline(ctx, conn); err != nil {
		t.Fatalf("rollback preview changed the persistent F5 PUBLIC/default baseline: %v", err)
	}
}
