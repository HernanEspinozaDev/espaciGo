package migrator

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestNativeKYCAuthorityWindowScriptsAreVersionedAndTargetPinned(t *testing.T) {
	root := filepath.Join("..", "..", "db", "repair")
	scripts := []string{
		"prepare_native_kyc_v95.sql",
		"finalize_native_kyc_v95.sql",
		"prepare_native_kyc_v96.sql",
		"finalize_native_kyc_v96.sql",
	}
	for _, name := range scripts {
		t.Run(name, func(t *testing.T) {
			body, err := os.ReadFile(filepath.Join(root, name))
			if err != nil {
				t.Fatalf("read versioned authority-window script: %v", err)
			}
			sql := string(body)
			for _, required := range []string{
				"'espacigo_dms_stage_20261020'",
				"<>16388",
				"current_setting('port')<>'5432'",
				"current_setting('server_version_num')<>'180007'",
				"datdba",
				"session_user<>'postgres'",
				"current_user<>'postgres'",
				"pg_advisory_xact_lock",
			} {
				if !strings.Contains(sql, required) {
					t.Errorf("%s must pin %q", name, required)
				}
			}
			for _, forbidden := range []string{"GRANT ALL", "ALTER ROLE", "DROP TABLE", "TRUNCATE", "DELETE FROM public."} {
				if strings.Contains(strings.ToUpper(sql), strings.ToUpper(forbidden)) {
					t.Errorf("%s must not contain broad or destructive operation %q", name, forbidden)
				}
			}
		})
	}
}

func TestNativeKYCAuthorityWindowPostgres(t *testing.T) {
	if os.Getenv("TEST_MIGRATOR_CI_ISOLATED") != "1" {
		t.Skip("requires the networkless PostgreSQL 18.7 authority-window harness")
	}
	adminURL := os.Getenv("TEST_DATABASE_URL")
	parsed, err := url.Parse(adminURL)
	if err != nil || parsed == nil || parsed.User == nil || parsed.User.Username() != "postgres" {
		t.Fatal("authority-window harness must use the disposable postgres socket DSN")
	}
	if _, hasPassword := parsed.User.Password(); hasPassword {
		t.Fatal("authority-window harness must not use a password")
	}
	socketDir := parsed.Query().Get("host")
	if !filepath.IsAbs(socketDir) {
		t.Fatal("authority-window harness must use an absolute Unix-socket directory")
	}
	marker, err := os.Stat(filepath.Join(socketDir, ".espacigo-migration-ci"))
	if err != nil || marker.Mode().Perm() != 0o600 {
		t.Fatalf("authority-window disposable-target marker is missing or unsafe: %v", err)
	}
	probeCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	probe := connectTestDB(t, probeCtx, adminURL)
	var engine int
	var sessionUser string
	if err := probe.QueryRow(probeCtx, `SELECT current_setting('server_version_num')::integer,session_user`).Scan(&engine, &sessionUser); err != nil {
		cancel()
		_ = probe.Close(context.Background())
		t.Fatal(err)
	}
	cancel()
	if err := probe.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if engine != 180007 || sessionUser != "postgres" {
		t.Fatalf("authority-window harness identity/version=%s/%d, want postgres/180007", sessionUser, engine)
	}

	t.Run("v95-wrong-target-no-ddl", func(t *testing.T) {
		withTestDatabase(t, func(ctx context.Context, databaseURL string) {
			conn := connectTestDB(t, ctx, databaseURL)
			defer conn.Close(context.Background())
			target := authorityWindowTargetForTest(t, ctx, conn)
			setupNativeAuthorityFixture(t, ctx, conn, 94, false)
			beforeOwners := authorityObjectOwners(t, ctx, conn)
			beforeACL := publicSchemaACL(t, ctx, conn)
			beforeLedger := authorityLedger(t, ctx, conn)
			err := executeNativeAuthorityScript(t, ctx, conn, "prepare_native_kyc_v95.sql", target.name, target.oid+1)
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), "target") {
				t.Fatalf("wrong database OID must fail at the target guard, got %v", err)
			}
			if got := authorityObjectOwners(t, ctx, conn); got != beforeOwners {
				t.Fatalf("wrong-target preparation changed object owners: before=%s after=%s", beforeOwners, got)
			}
			if got := publicSchemaACL(t, ctx, conn); got != beforeACL || publicSchemaCreate(t, ctx, conn) {
				t.Fatalf("wrong-target preparation changed public ACL/create: before=%s after=%s", beforeACL, got)
			}
			if got := authorityLedger(t, ctx, conn); got != beforeLedger {
				t.Fatalf("wrong-target preparation changed migration ledger: before=%s after=%s", beforeLedger, got)
			}
		})
	})

	t.Run("v95-prepare-finalize-retry", func(t *testing.T) {
		withTestDatabase(t, func(ctx context.Context, databaseURL string) {
			conn := connectTestDB(t, ctx, databaseURL)
			defer conn.Close(context.Background())
			target := authorityWindowTargetForTest(t, ctx, conn)
			setupNativeAuthorityFixture(t, ctx, conn, 94, false)
			beforeACL := publicSchemaACL(t, ctx, conn)
			beforeLedger := authorityLedger(t, ctx, conn)
			beforeRuntimeACL, beforeGrantor := runtimeVerificationACL(t, ctx, conn)
			if err := executeNativeAuthorityScript(t, ctx, conn, "prepare_native_kyc_v95.sql", target.name, target.oid); err != nil {
				t.Fatalf("prepare V95 authority window: %v", err)
			}
			assertAuthorityOwners(t, ctx, conn, "espacigo_schema_owner")
			if !publicSchemaCreate(t, ctx, conn) {
				t.Fatal("V95 preparation did not grant the temporary public CREATE privilege")
			}
			afterRuntimeACL, afterGrantor := runtimeVerificationACL(t, ctx, conn)
			if afterRuntimeACL != beforeRuntimeACL || beforeGrantor != "postgres" || afterGrantor != "espacigo_schema_owner" {
				t.Fatalf("owner transition changed effective runtime ACLs: tuples %s -> %s, grantor %q -> %q", beforeRuntimeACL, afterRuntimeACL, beforeGrantor, afterGrantor)
			}
			if got := authorityLedger(t, ctx, conn); got != beforeLedger {
				t.Fatalf("V95 preparation changed migration ledger: before=%s after=%s", beforeLedger, got)
			}
			preparedOwners := authorityObjectOwners(t, ctx, conn)
			preparedACL := publicSchemaACL(t, ctx, conn)
			wrongTargetErr := executeNativeAuthorityScript(t, ctx, conn, "finalize_native_kyc_v95.sql", target.name, target.oid+1)
			if wrongTargetErr == nil || !strings.Contains(strings.ToLower(wrongTargetErr.Error()), "target") {
				t.Fatalf("wrong-target finalizer must fail at its target guard, got %v", wrongTargetErr)
			}
			if got := authorityObjectOwners(t, ctx, conn); got != preparedOwners {
				t.Fatalf("wrong-target finalizer changed object owners: before=%s after=%s", preparedOwners, got)
			}
			if got := publicSchemaACL(t, ctx, conn); got != preparedACL || !publicSchemaCreate(t, ctx, conn) {
				t.Fatalf("wrong-target finalizer changed the prepared public ACL: before=%s after=%s", preparedACL, got)
			}
			if got := authorityLedger(t, ctx, conn); got != beforeLedger {
				t.Fatalf("wrong-target finalizer changed migration ledger: before=%s after=%s", beforeLedger, got)
			}
			if err := executeNativeAuthorityScript(t, ctx, conn, "finalize_native_kyc_v95.sql", target.name, target.oid); err != nil {
				t.Fatalf("finalize V95 authority window before migration (failure cleanup): %v", err)
			}
			if publicSchemaCreate(t, ctx, conn) || publicSchemaACL(t, ctx, conn) != beforeACL {
				t.Fatal("V95 finalizer did not restore the exact public ACL baseline")
			}
			if got, grantor := runtimeVerificationACL(t, ctx, conn); got != afterRuntimeACL || grantor != afterGrantor {
				t.Fatalf("V95 finalizer changed direct runtime ACLs: before=%s/%q after=%s/%q", afterRuntimeACL, afterGrantor, got, grantor)
			}
			assertAuthorityOwners(t, ctx, conn, "espacigo_schema_owner")
			if got := authorityLedger(t, ctx, conn); got != beforeLedger {
				t.Fatalf("V95 finalization changed migration ledger: before=%s after=%s", beforeLedger, got)
			}
			if err := executeNativeAuthorityScript(t, ctx, conn, "prepare_native_kyc_v95.sql", target.name, target.oid); err != nil {
				t.Fatalf("retry prepared V95 authority window: %v", err)
			}
			if err := executeNativeAuthorityScript(t, ctx, conn, "finalize_native_kyc_v95.sql", target.name, target.oid); err != nil {
				t.Fatalf("retry V95 cleanup finalizer: %v", err)
			}
			if publicSchemaCreate(t, ctx, conn) || publicSchemaACL(t, ctx, conn) != beforeACL {
				t.Fatal("retried V95 finalizer did not close CREATE and restore the ACL baseline")
			}
			if got, grantor := runtimeVerificationACL(t, ctx, conn); got != afterRuntimeACL || grantor != afterGrantor {
				t.Fatalf("retried V95 finalizer changed direct runtime ACLs: before=%s/%q after=%s/%q", afterRuntimeACL, afterGrantor, got, grantor)
			}
			if err := executeNativeAuthorityScript(t, ctx, conn, "prepare_native_kyc_v95.sql", target.name, target.oid); err != nil {
				t.Fatalf("prepare V95 for committed migration boundary: %v", err)
			}
			applyNativeAuthorityMigration(t, ctx, databaseURL, 95)
			v95RuntimeACL, v95Grantor := runtimeVerificationACL(t, ctx, conn)
			if v95Grantor != "espacigo_schema_owner" {
				t.Fatalf("committed V95 runtime ACL grantor=%q, want espacigo_schema_owner", v95Grantor)
			}
			v95Ledger := authorityLedger(t, ctx, conn)
			if err := executeNativeAuthorityScript(t, ctx, conn, "finalize_native_kyc_v95.sql", target.name, target.oid); err != nil {
				t.Fatalf("finalize V95 after committed migration: %v", err)
			}
			if publicSchemaCreate(t, ctx, conn) || publicSchemaACL(t, ctx, conn) != beforeACL {
				t.Fatal("V95 committed-boundary finalizer did not restore the exact public ACL")
			}
			assertAuthorityOwnersAfterV95(t, ctx, conn)
			if got, grantor := runtimeVerificationACL(t, ctx, conn); got != v95RuntimeACL || grantor != v95Grantor {
				t.Fatalf("V95 committed-boundary finalizer changed runtime ACLs: before=%s/%q after=%s/%q", v95RuntimeACL, v95Grantor, got, grantor)
			}
			if got := authorityLedger(t, ctx, conn); got != v95Ledger {
				t.Fatalf("V95 finalizer changed committed ledger: before=%s after=%s", v95Ledger, got)
			}
			var maxVersion int64
			if err := conn.QueryRow(ctx, `SELECT max(version) FROM public.schema_migrations`).Scan(&maxVersion); err != nil || maxVersion != 95 {
				t.Fatalf("V95 committed migration max version=%d, err=%v", maxVersion, err)
			}
		})
	})

	t.Run("v96-wrong-target-no-ddl", func(t *testing.T) {
		withTestDatabase(t, func(ctx context.Context, databaseURL string) {
			conn := connectTestDB(t, ctx, databaseURL)
			defer conn.Close(context.Background())
			target := authorityWindowTargetForTest(t, ctx, conn)
			setupNativeAuthorityFixture(t, ctx, conn, 95, true)
			beforeOwners := authorityObjectOwners(t, ctx, conn)
			beforeACL := publicSchemaACL(t, ctx, conn)
			beforeLedger := authorityLedger(t, ctx, conn)
			beforeRuntimeACL, beforeGrantor := runtimeVerificationACL(t, ctx, conn)
			err := executeNativeAuthorityScript(t, ctx, conn, "prepare_native_kyc_v96.sql", target.name, target.oid+1)
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), "target") {
				t.Fatalf("wrong-target V96 preparation must fail at its target guard, got %v", err)
			}
			if got := authorityObjectOwners(t, ctx, conn); got != beforeOwners {
				t.Fatalf("wrong-target V96 preparation changed owners: before=%s after=%s", beforeOwners, got)
			}
			if got := publicSchemaACL(t, ctx, conn); got != beforeACL || publicSchemaCreate(t, ctx, conn) {
				t.Fatalf("wrong-target V96 preparation changed public ACL: before=%s after=%s", beforeACL, got)
			}
			if got := authorityLedger(t, ctx, conn); got != beforeLedger {
				t.Fatalf("wrong-target V96 preparation changed ledger: before=%s after=%s", beforeLedger, got)
			}
			if got, grantor := runtimeVerificationACL(t, ctx, conn); got != beforeRuntimeACL || grantor != beforeGrantor {
				t.Fatalf("wrong-target V96 preparation changed runtime grants: before=%s/%q after=%s/%q", beforeRuntimeACL, beforeGrantor, got, grantor)
			}
		})
	})

	t.Run("v96-prepare-finalize-retry", func(t *testing.T) {
		withTestDatabase(t, func(ctx context.Context, databaseURL string) {
			conn := connectTestDB(t, ctx, databaseURL)
			defer conn.Close(context.Background())
			target := authorityWindowTargetForTest(t, ctx, conn)
			setupNativeAuthorityFixture(t, ctx, conn, 95, true)
			beforeOwners := authorityObjectOwners(t, ctx, conn)
			beforeACL := publicSchemaACL(t, ctx, conn)
			beforeLedger := authorityLedger(t, ctx, conn)
			beforeRuntimeACL, beforeGrantor := runtimeVerificationACL(t, ctx, conn)
			if beforeGrantor != "espacigo_schema_owner" {
				t.Fatalf("V95 owner baseline grantor=%q, want espacigo_schema_owner", beforeGrantor)
			}
			if err := executeNativeAuthorityScript(t, ctx, conn, "prepare_native_kyc_v96.sql", target.name, target.oid); err != nil {
				t.Fatalf("prepare V96 authority window: %v", err)
			}
			assertAuthorityOwnersAfterV95(t, ctx, conn)
			if !publicSchemaCreate(t, ctx, conn) {
				t.Fatal("V96 preparation did not grant temporary public CREATE")
			}
			preparedACL := publicSchemaACL(t, ctx, conn)
			if preparedACL == beforeACL {
				t.Fatal("V96 preparation did not open the separate temporary CREATE window")
			}
			if got, grantor := runtimeVerificationACL(t, ctx, conn); got != beforeRuntimeACL || grantor != beforeGrantor {
				t.Fatalf("V96 preparation changed runtime grants: before=%s/%q after=%s/%q", beforeRuntimeACL, beforeGrantor, got, grantor)
			}
			if got := authorityLedger(t, ctx, conn); got != beforeLedger {
				t.Fatalf("V96 preparation changed migration ledger: before=%s after=%s", beforeLedger, got)
			}
			wrongTargetErr := executeNativeAuthorityScript(t, ctx, conn, "finalize_native_kyc_v96.sql", target.name, target.oid+1)
			if wrongTargetErr == nil || !strings.Contains(strings.ToLower(wrongTargetErr.Error()), "target") {
				t.Fatalf("wrong-target V96 finalizer must fail at its target guard, got %v", wrongTargetErr)
			}
			if got := authorityObjectOwners(t, ctx, conn); got != beforeOwners {
				t.Fatalf("wrong-target V96 finalizer changed owners: before=%s after=%s", beforeOwners, got)
			}
			if got := publicSchemaACL(t, ctx, conn); got != preparedACL || !publicSchemaCreate(t, ctx, conn) {
				t.Fatalf("wrong-target V96 finalizer changed prepared ACL: before=%s after=%s", preparedACL, got)
			}
			if got := authorityLedger(t, ctx, conn); got != beforeLedger {
				t.Fatalf("wrong-target V96 finalizer changed migration ledger: before=%s after=%s", beforeLedger, got)
			}
			if err := executeNativeAuthorityScript(t, ctx, conn, "finalize_native_kyc_v96.sql", target.name, target.oid); err != nil {
				t.Fatalf("finalize V96 authority window before migration (failure cleanup): %v", err)
			}
			if publicSchemaCreate(t, ctx, conn) || publicSchemaACL(t, ctx, conn) != beforeACL {
				t.Fatal("V96 finalizer did not restore the exact pre-V96 public ACL")
			}
			assertAuthorityOwnersAfterV95(t, ctx, conn)
			if got, grantor := runtimeVerificationACL(t, ctx, conn); got != beforeRuntimeACL || grantor != beforeGrantor {
				t.Fatalf("V96 finalizer changed runtime grants: before=%s/%q after=%s/%q", beforeRuntimeACL, beforeGrantor, got, grantor)
			}
			if got := authorityLedger(t, ctx, conn); got != beforeLedger {
				t.Fatalf("V96 finalization changed migration ledger: before=%s after=%s", beforeLedger, got)
			}
			if err := executeNativeAuthorityScript(t, ctx, conn, "prepare_native_kyc_v96.sql", target.name, target.oid); err != nil {
				t.Fatalf("retry prepared V96 authority window: %v", err)
			}
			if err := executeNativeAuthorityScript(t, ctx, conn, "finalize_native_kyc_v96.sql", target.name, target.oid); err != nil {
				t.Fatalf("retry V96 cleanup finalizer: %v", err)
			}
			if publicSchemaCreate(t, ctx, conn) || publicSchemaACL(t, ctx, conn) != beforeACL {
				t.Fatal("retried V96 finalizer did not close CREATE and restore the exact ACL")
			}
			if err := executeNativeAuthorityScript(t, ctx, conn, "prepare_native_kyc_v96.sql", target.name, target.oid); err != nil {
				t.Fatalf("prepare V96 for committed migration boundary: %v", err)
			}
			applyNativeAuthorityMigration(t, ctx, databaseURL, 96)
			v96RuntimeACL, v96Grantor := runtimeVerificationACL(t, ctx, conn)
			if v96Grantor != "espacigo_schema_owner" {
				t.Fatalf("committed V96 runtime ACL grantor=%q, want espacigo_schema_owner", v96Grantor)
			}
			v96Ledger := authorityLedger(t, ctx, conn)
			if err := executeNativeAuthorityScript(t, ctx, conn, "finalize_native_kyc_v96.sql", target.name, target.oid); err != nil {
				t.Fatalf("finalize V96 after committed migration: %v", err)
			}
			if publicSchemaCreate(t, ctx, conn) || publicSchemaACL(t, ctx, conn) != beforeACL {
				t.Fatal("V96 committed-boundary finalizer did not restore the exact public ACL")
			}
			assertAuthorityOwnersAfterV95(t, ctx, conn)
			if got, grantor := runtimeVerificationACL(t, ctx, conn); got != v96RuntimeACL || grantor != v96Grantor {
				t.Fatalf("V96 committed-boundary finalizer changed runtime ACLs: before=%s/%q after=%s/%q", v96RuntimeACL, v96Grantor, got, grantor)
			}
			if got := authorityLedger(t, ctx, conn); got != v96Ledger {
				t.Fatalf("V96 finalizer changed committed ledger: before=%s after=%s", v96Ledger, got)
			}
			var maxVersion int64
			if err := conn.QueryRow(ctx, `SELECT max(version) FROM public.schema_migrations`).Scan(&maxVersion); err != nil || maxVersion != 96 {
				t.Fatalf("V96 committed migration max version=%d, err=%v", maxVersion, err)
			}
		})
	})
}

type nativeAuthorityTestTarget struct {
	name string
	oid  int64
}

func authorityWindowTargetForTest(t *testing.T, ctx context.Context, conn *pgx.Conn) nativeAuthorityTestTarget {
	t.Helper()
	var target nativeAuthorityTestTarget
	if err := conn.QueryRow(ctx, `SELECT current_database(),oid::bigint FROM pg_catalog.pg_database WHERE datname=current_database()`).Scan(&target.name, &target.oid); err != nil {
		t.Fatal(err)
	}
	return target
}

func setupNativeAuthorityFixture(t *testing.T, ctx context.Context, conn *pgx.Conn, ledgerVersion int64, schemaOwnerTables bool) {
	t.Helper()
	adminURL := os.Getenv("TEST_DATABASE_URL")
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		admin, err := pgx.Connect(cleanupCtx, adminURL)
		if err != nil {
			t.Errorf("connect disposable PostgreSQL for authority-role cleanup: %v", err)
			return
		}
		defer admin.Close(context.Background())
		_, err = admin.Exec(cleanupCtx, `DROP ROLE IF EXISTS espacigo_privacy_worker_login,espacigo_privacy_cleanup_worker,espacigo_document_verification_fn,espacigo_migrator_login,espacigo_schema_owner,espacigo_runtime,espacigo_gcp06_test_access,dms_migrator`)
		if err != nil {
			t.Errorf("drop disposable authority-window roles: %v", err)
		}
	})
	roles := `
CREATE ROLE dms_migrator NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOREPLICATION NOBYPASSRLS;
CREATE ROLE espacigo_gcp06_test_access NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOREPLICATION NOBYPASSRLS;
CREATE ROLE espacigo_runtime NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOREPLICATION NOBYPASSRLS;
CREATE ROLE espacigo_schema_owner NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOREPLICATION NOBYPASSRLS;
CREATE ROLE espacigo_migrator_login LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOREPLICATION NOBYPASSRLS;
CREATE ROLE espacigo_document_verification_fn NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOREPLICATION NOBYPASSRLS;
CREATE ROLE espacigo_privacy_cleanup_worker NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOREPLICATION NOBYPASSRLS;
CREATE ROLE espacigo_privacy_worker_login LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOREPLICATION NOBYPASSRLS;
GRANT espacigo_schema_owner TO espacigo_migrator_login WITH ADMIN FALSE, INHERIT FALSE, SET TRUE;
GRANT espacigo_document_verification_fn TO espacigo_schema_owner WITH ADMIN FALSE, INHERIT FALSE, SET TRUE;
GRANT espacigo_privacy_cleanup_worker TO espacigo_privacy_worker_login WITH ADMIN FALSE, INHERIT FALSE, SET TRUE;
GRANT USAGE ON SCHEMA public TO dms_migrator, espacigo_document_verification_fn, espacigo_gcp06_test_access, espacigo_privacy_cleanup_worker, espacigo_runtime, espacigo_schema_owner;
CREATE TABLE public.schema_migrations (
 version bigint PRIMARY KEY CHECK(version>0), name text NOT NULL UNIQUE,
 checksum text NOT NULL CHECK(checksum ~ '^[0-9a-f]{64}$'),
 runner_checksum text NOT NULL CHECK(runner_checksum ~ '^[0-9a-f]{64}$'),
 applied_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
GRANT SELECT,INSERT ON public.schema_migrations TO espacigo_schema_owner;
`
	if _, err := conn.Exec(ctx, roles); err != nil {
		t.Fatalf("create synthetic authority roles and ledger: %v", err)
	}
	caseColumns := `
 id uuid PRIMARY KEY,
 purpose text NOT NULL,
 subject_id uuid NOT NULL,
 space_id uuid,
 state text NOT NULL,
 revision bigint NOT NULL,
 reviewer_id uuid,
 verified_at timestamptz,
 verified_until timestamptz,
 document_expires_at timestamptz,
 revoked_at timestamptz,
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 superseded_at timestamptz`
	if ledgerVersion >= 95 {
		caseColumns += `,
 identity_declaration_revision_id uuid,
 document_format_declared text,
 capture_consent_version text,
 capture_consented_at timestamptz`
	}
	objects := `CREATE TABLE public.document_verification_case (` + caseColumns + `,
 CONSTRAINT document_verification_case_id_scope_uq UNIQUE(id,purpose,subject_id,space_id));
CREATE UNIQUE INDEX document_verification_one_open_identity_uq ON public.document_verification_case(subject_id,purpose) WHERE purpose='identity' AND state IN ('borrador','enviada','en_revision','requiere_antecedentes');
CREATE INDEX document_verification_identity_current_idx ON public.document_verification_case(subject_id,verified_until DESC) WHERE purpose='identity' AND state='aprobada';
CREATE UNIQUE INDEX document_verification_one_open_space_subject_uq ON public.document_verification_case(space_id,purpose,subject_id) WHERE purpose='space' AND state IN ('borrador','enviada','en_revision','requiere_antecedentes');
CREATE TABLE public.document_verification_event (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 case_id uuid NOT NULL,purpose text NOT NULL,subject_id uuid NOT NULL,file_id uuid,
 action text NOT NULL,revision bigint NOT NULL,occurred_at timestamptz NOT NULL DEFAULT now());
CREATE INDEX document_verification_event_case_idx ON public.document_verification_event(case_id,occurred_at,id);
CREATE INDEX document_verification_event_file_idx ON public.document_verification_event(file_id,occurred_at,id) WHERE file_id IS NOT NULL;
GRANT SELECT ON public.document_verification_case TO espacigo_runtime;
GRANT SELECT,INSERT ON public.document_verification_event TO espacigo_runtime;
GRANT USAGE ON SEQUENCE public.document_verification_event_id_seq TO espacigo_runtime;
GRANT INSERT(id,purpose,subject_id,space_id,document_expires_at) ON public.document_verification_case TO espacigo_runtime;
GRANT UPDATE(state,revision,reviewer_id,verified_at,verified_until,document_expires_at,revoked_at,updated_at,superseded_at) ON public.document_verification_case TO espacigo_runtime;
`
	if ledgerVersion >= 95 {
		objects += `
GRANT INSERT(identity_declaration_revision_id,document_format_declared,capture_consent_version,capture_consented_at) ON public.document_verification_case TO espacigo_runtime;
GRANT UPDATE(identity_declaration_revision_id) ON public.document_verification_case TO espacigo_runtime;
CREATE INDEX document_verification_case_identity_declaration_idx ON public.document_verification_case(identity_declaration_revision_id,subject_id) WHERE identity_declaration_revision_id IS NOT NULL;
`
	}
	if _, err := conn.Exec(ctx, objects); err != nil {
		t.Fatalf("create synthetic verification objects and baseline ACLs: %v", err)
	}
	functions := `
GRANT CREATE ON SCHEMA public TO espacigo_schema_owner;
SET ROLE espacigo_schema_owner;
CREATE TABLE public.identity_declaration_revision (id uuid NOT NULL,account_id uuid NOT NULL,PRIMARY KEY(id,account_id));
CREATE FUNCTION public.purge_expired_identity_declaration_revisions() RETURNS bigint
 LANGUAGE sql SECURITY DEFINER SET search_path=pg_catalog AS $$ SELECT 0::bigint $$;
REVOKE ALL ON FUNCTION public.purge_expired_identity_declaration_revisions() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION public.purge_expired_identity_declaration_revisions() TO espacigo_privacy_cleanup_worker;
RESET ROLE;
REVOKE CREATE ON SCHEMA public FROM espacigo_schema_owner;
`
	if _, err := conn.Exec(ctx, functions); err != nil {
		t.Fatalf("create synthetic schema-owned declaration/purge prerequisite: %v", err)
	}
	if schemaOwnerTables {
		if _, err := conn.Exec(ctx, `ALTER TABLE public.document_verification_case OWNER TO espacigo_schema_owner; ALTER TABLE public.document_verification_event OWNER TO espacigo_schema_owner;`); err != nil {
			t.Fatalf("create synthetic post-V95 owner state: %v", err)
		}
	}
	migrations, err := DiscoverMigrations(filepath.Join("..", "..", "db", "migrations"))
	if err != nil {
		t.Fatalf("discover source migrations for synthetic ledger: %v", err)
	}
	for _, migration := range migrations {
		if migration.Version > ledgerVersion {
			break
		}
		if _, err := conn.Exec(ctx, `INSERT INTO public.schema_migrations(version,name,checksum,runner_checksum) VALUES($1,$2,$3,$4)`, migration.Version, migration.Name, migration.Checksum, strings.Repeat("0", 64)); err != nil {
			t.Fatalf("seed synthetic V%d ledger row: %v", migration.Version, err)
		}
	}
}

func executeNativeAuthorityScript(t *testing.T, ctx context.Context, conn *pgx.Conn, filename, testDatabase string, testOID int64) error {
	t.Helper()
	path := filepath.Join("..", "..", "db", "repair", filename)
	body, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read versioned authority-window script %s: %w", filename, err)
	}
	sql := string(body)
	const nativeDatabase = "'espacigo_dms_stage_20261020'"
	if strings.Count(sql, nativeDatabase) < 1 {
		return fmt.Errorf("%s must contain a native database pin", filename)
	}
	sql = strings.ReplaceAll(sql, nativeDatabase, "'"+testDatabase+"'")
	const nativeOIDGuard = "<>16388"
	if strings.Count(sql, nativeOIDGuard) < 1 {
		return fmt.Errorf("%s must contain a native database OID pin", filename)
	}
	sql = strings.ReplaceAll(sql, nativeOIDGuard, fmt.Sprintf("<>%d", testOID))
	if _, err := conn.Exec(ctx, sql); err != nil {
		_, rollbackErr := conn.Exec(context.Background(), "ROLLBACK")
		if rollbackErr != nil {
			return fmt.Errorf("execute %s: %v (rollback failed: %w)", filename, err, rollbackErr)
		}
		return fmt.Errorf("execute %s: %w", filename, err)
	}
	return nil
}

func applyNativeAuthorityMigration(t *testing.T, ctx context.Context, adminURL string, version int64) {
	t.Helper()
	migrations, err := DiscoverMigrations(filepath.Join("..", "..", "db", "migrations"))
	if err != nil {
		t.Fatalf("discover migration source for authority-window test: %v", err)
	}
	var selected *Migration
	for i := range migrations {
		if migrations[i].Version == version {
			selected = &migrations[i]
			break
		}
	}
	if selected == nil {
		t.Fatalf("V%d migration source is missing", version)
	}

	parsed, err := url.Parse(adminURL)
	if err != nil {
		t.Fatalf("parse disposable admin DSN: %v", err)
	}
	parsed.User = url.User("espacigo_migrator_login")
	conn := connectTestDB(t, ctx, parsed.String())
	defer conn.Close(context.Background())
	var sessionUser string
	if err := conn.QueryRow(ctx, `SELECT session_user`).Scan(&sessionUser); err != nil || sessionUser != "espacigo_migrator_login" {
		t.Fatalf("migration session user=%q, err=%v; want espacigo_migrator_login", sessionUser, err)
	}
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatalf("begin disposable V%d migration transaction: %v", version, err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, `SET LOCAL ROLE espacigo_schema_owner`); err != nil {
		t.Fatalf("set disposable V%d migration identity: %v", version, err)
	}
	if _, err := tx.Exec(ctx, `SET LOCAL search_path TO public`); err != nil {
		t.Fatalf("set disposable V%d migration search_path: %v", version, err)
	}
	var currentUser string
	if err := tx.QueryRow(ctx, `SELECT session_user,current_user`).Scan(&sessionUser, &currentUser); err != nil || sessionUser != "espacigo_migrator_login" || currentUser != "espacigo_schema_owner" {
		t.Fatalf("V%d migration identity=%q/%q err=%v", version, sessionUser, currentUser, err)
	}
	if _, err := tx.Exec(ctx, string(selected.SQL)); err != nil {
		t.Fatalf("execute V%d source migration in disposable target: %v", version, err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO public.schema_migrations(version,name,checksum,runner_checksum) VALUES($1,$2,$3,$4)`, selected.Version, selected.Name, selected.Checksum, strings.Repeat("0", 64)); err != nil {
		t.Fatalf("record synthetic V%d migration boundary: %v", version, err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit disposable V%d migration transaction: %v", version, err)
	}
}

func authorityObjectOwners(t *testing.T, ctx context.Context, conn *pgx.Conn) string {
	t.Helper()
	var snapshot string
	err := conn.QueryRow(ctx, `SELECT COALESCE(jsonb_agg(jsonb_build_array(c.relname,pg_catalog.pg_get_userbyid(c.relowner)) ORDER BY c.relname),'[]'::jsonb)::text
 FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
 WHERE n.nspname='public' AND c.relname = ANY($1::text[])`, []string{
		"document_verification_case", "document_verification_event", "document_verification_event_id_seq",
		"document_verification_case_pkey", "document_verification_case_id_scope_uq",
		"document_verification_one_open_identity_uq", "document_verification_identity_current_idx",
		"document_verification_one_open_space_subject_uq", "document_verification_event_pkey",
		"document_verification_event_case_idx", "document_verification_event_file_idx",
		"document_verification_case_identity_declaration_idx",
	}).Scan(&snapshot)
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func assertAuthorityOwners(t *testing.T, ctx context.Context, conn *pgx.Conn, want string) {
	t.Helper()
	var count int
	var allOwned bool
	err := conn.QueryRow(ctx, `SELECT count(*)::integer,COALESCE(bool_and(pg_catalog.pg_get_userbyid(c.relowner)=$1),false)
 FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
 WHERE n.nspname='public' AND c.relname = ANY($2::text[])`, want, []string{
		"document_verification_case", "document_verification_event", "document_verification_event_id_seq",
		"document_verification_case_pkey", "document_verification_case_id_scope_uq",
		"document_verification_one_open_identity_uq", "document_verification_identity_current_idx",
		"document_verification_one_open_space_subject_uq", "document_verification_event_pkey",
		"document_verification_event_case_idx", "document_verification_event_file_idx",
	}).Scan(&count, &allOwned)
	if err != nil || count != 11 || !allOwned {
		t.Fatalf("authorized table/index/sequence owners count=%d all-owned=%t err=%v", count, allOwned, err)
	}
}

func assertAuthorityOwnersAfterV95(t *testing.T, ctx context.Context, conn *pgx.Conn) {
	t.Helper()
	var count int
	var allOwned bool
	err := conn.QueryRow(ctx, `SELECT count(*)::integer,COALESCE(bool_and(pg_catalog.pg_get_userbyid(c.relowner)='espacigo_schema_owner'),false)
 FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
 WHERE n.nspname='public' AND c.relname = ANY($1::text[])`, []string{
		"document_verification_case", "document_verification_event", "document_verification_event_id_seq",
		"document_verification_case_pkey", "document_verification_case_id_scope_uq",
		"document_verification_one_open_identity_uq", "document_verification_identity_current_idx",
		"document_verification_one_open_space_subject_uq", "document_verification_event_pkey",
		"document_verification_event_case_idx", "document_verification_event_file_idx",
		"document_verification_case_identity_declaration_idx",
	}).Scan(&count, &allOwned)
	if err != nil || count != 12 || !allOwned {
		t.Fatalf("post-V95 authorized table/index/sequence owners count=%d all-owned=%t err=%v", count, allOwned, err)
	}
}

func publicSchemaACL(t *testing.T, ctx context.Context, conn *pgx.Conn) string {
	t.Helper()
	var snapshot string
	err := conn.QueryRow(ctx, `SELECT COALESCE(jsonb_agg(jsonb_build_array(COALESCE(r.rolname,'PUBLIC'),g.rolname,a.privilege_type,a.is_grantable) ORDER BY COALESCE(r.rolname,'PUBLIC'),g.rolname,a.privilege_type),'[]'::jsonb)::text
 FROM pg_catalog.pg_namespace n CROSS JOIN LATERAL pg_catalog.aclexplode(n.nspacl) a
 LEFT JOIN pg_catalog.pg_roles r ON r.oid=a.grantee JOIN pg_catalog.pg_roles g ON g.oid=a.grantor
 WHERE n.nspname='public'`).Scan(&snapshot)
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func publicSchemaCreate(t *testing.T, ctx context.Context, conn *pgx.Conn) bool {
	t.Helper()
	var create bool
	if err := conn.QueryRow(ctx, `SELECT pg_catalog.has_schema_privilege('espacigo_schema_owner','public','CREATE')`).Scan(&create); err != nil {
		t.Fatal(err)
	}
	return create
}

func authorityLedger(t *testing.T, ctx context.Context, conn *pgx.Conn) string {
	t.Helper()
	var snapshot string
	err := conn.QueryRow(ctx, `SELECT jsonb_build_object('count',count(*),'min',min(version),'max',max(version),'tail_name',(SELECT name FROM public.schema_migrations ORDER BY version DESC LIMIT 1),'tail_checksum',(SELECT checksum FROM public.schema_migrations ORDER BY version DESC LIMIT 1))::text FROM public.schema_migrations`).Scan(&snapshot)
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func runtimeVerificationACL(t *testing.T, ctx context.Context, conn *pgx.Conn) (string, string) {
	t.Helper()
	var tuples, grantor string
	err := conn.QueryRow(ctx, `WITH relations AS (
 SELECT c.relname::text AS object_name,''::text AS column_name,x.grantee,x.grantor,x.privilege_type,x.is_grantable
 FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
 CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(c.relacl,pg_catalog.acldefault(CASE WHEN c.relkind='S' THEN 'S'::"char" ELSE 'r'::"char" END,c.relowner))) x
 WHERE n.nspname='public' AND c.relname IN ('document_verification_case','document_verification_event','document_verification_event_id_seq')
 UNION ALL
 SELECT c.relname::text,a.attname::text,x.grantee,x.grantor,x.privilege_type,x.is_grantable
 FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace JOIN pg_catalog.pg_attribute a ON a.attrelid=c.oid
 CROSS JOIN LATERAL pg_catalog.aclexplode(CASE WHEN pg_catalog.cardinality(a.attacl)>0 THEN a.attacl ELSE NULL::aclitem[] END) x
 WHERE n.nspname='public' AND c.relname='document_verification_case' AND a.attnum>0 AND NOT a.attisdropped
 ), runtime_acl AS (
 SELECT object_name,column_name,grantee,grantor,privilege_type,is_grantable FROM relations
 WHERE grantee=(SELECT oid FROM pg_catalog.pg_roles WHERE rolname='espacigo_runtime')
 )
 SELECT COALESCE(jsonb_agg(jsonb_build_array(object_name,column_name,privilege_type,is_grantable) ORDER BY object_name,column_name,privilege_type),'[]'::jsonb)::text,
        COALESCE(string_agg(DISTINCT pg_catalog.pg_get_userbyid(grantor),',' ORDER BY pg_catalog.pg_get_userbyid(grantor)),'')
 FROM runtime_acl`).Scan(&tuples, &grantor)
	if err != nil {
		t.Fatal(err)
	}
	return tuples, grantor
}
