package migrator

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

const wizardAdminRollbackOptIn = "ESPACIGO_WIZARD_ADMIN_ROLLBACK"

type wizardRoleSnapshot struct {
	Attributes string
	Comment    *string
	Membership string
	ACL        string
}

func TestWizardAdministrativeV60RollbackNative(t *testing.T) {
	if os.Getenv(wizardAdminRollbackOptIn) != "1" {
		t.Skipf("native PostgreSQL rollback test disabled; set %s=1 to opt in", wizardAdminRollbackOptIn)
	}
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test source path")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(source), "..", ".."))
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldWD); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir repository root: %v", err)
	}

	manifestBytes, err := os.ReadFile(filepath.Join(root, ".local/testing/wizard-improvements/reviewed-full-83-manifest.json"))
	if err != nil {
		t.Fatalf("read independent reviewed migration manifest: %v", err)
	}
	var entries []struct {
		File    string `json:"file"`
		SHA256  string `json:"sha256"`
		Version int64  `json:"version"`
	}
	if err := json.Unmarshal(manifestBytes, &entries); err != nil {
		t.Fatalf("parse reviewed migration manifest: %v", err)
	}
	expected := make([]MigrationSummary, len(entries))
	for i, entry := range entries {
		if entry.Version != int64(i+1) || entry.File == "" || !wizardChecksumPattern.MatchString(entry.SHA256) {
			t.Fatalf("invalid reviewed manifest entry %d", i+1)
		}
		expected[i] = MigrationSummary{Version: entry.Version, Name: entry.File, Checksum: entry.SHA256}
	}
	if len(expected) != 83 {
		t.Fatalf("manifest has %d entries, want 83", len(expected))
	}
	migrationDir := filepath.Join(root, "db/migrations")
	migrations, err := DiscoverMigrations(migrationDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(migrations) != 83 {
		t.Fatalf("discovered %d migrations, want 83", len(migrations))
	}
	for i := range expected {
		if migrations[i].Version != expected[i].Version || migrations[i].Name != expected[i].Name || migrations[i].Checksum != expected[i].Checksum {
			t.Fatalf("source differs from reviewed manifest at V%06d", i+1)
		}
	}
	manifestHash := sha256.Sum256(manifestBytes)
	t.Logf("review manifest verified: 83 migrations, sha256=%s", hex.EncodeToString(manifestHash[:]))

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	admin, err := pgx.Connect(ctx, wizardTrialDatabaseURL)
	if err != nil {
		t.Fatalf("connect authorized native trial: %v", err)
	}
	defer admin.Close(context.Background())
	var db, sessionUser, currentUser string
	var serverVersionNum int
	if err := admin.QueryRow(ctx, `SELECT current_database(),session_user,current_user,current_setting('server_version_num')::int`).Scan(&db, &sessionUser, &currentUser, &serverVersionNum); err != nil {
		t.Fatal(err)
	}
	if db != "espacigo_wizard_20261106" || sessionUser != "postgres" || currentUser != "postgres" || serverVersionNum != 180006 {
		t.Fatalf("native target identity/version mismatch: database=%q session_user=%q current_user=%q version_num=%d", db, sessionUser, currentUser, serverVersionNum)
	}
	if _, err := admin.Exec(ctx, `BEGIN TRANSACTION READ ONLY`); err != nil {
		t.Fatal(err)
	}
	var ledgerExists bool
	if err := admin.QueryRow(ctx, `SELECT to_regclass('public.schema_migrations') IS NOT NULL`).Scan(&ledgerExists); err != nil {
		t.Fatal(err)
	}
	if !ledgerExists {
		t.Fatal("native rollback target has no existing migration ledger")
	}
	history, err := readHistory(ctx, admin)
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyCompleteHistory(migrations[:59], history, 59); err != nil {
		t.Fatalf("target must match exact V1-V59 prefix: %v", err)
	}
	if _, err := admin.Exec(ctx, `COMMIT`); err != nil {
		t.Fatal(err)
	}
	before, err := snapshotWizardRollbackState(ctx, admin)
	if err != nil {
		t.Fatal(err)
	}

	sentinel := errors.New("injected V60 rollback sentinel")
	hookCalled := false
	result, migrationErr := runWizardAdministrativeMigration(ctx, wizardTrialDatabaseURL, migrationDir, 60, expected, func() error { hookCalled = true; return sentinel })
	if !errors.Is(migrationErr, sentinel) || !hookCalled {
		t.Fatalf("migration error=%v, hook_called=%v; want injected pre-ledger error", migrationErr, hookCalled)
	}
	if len(result.Applied) != 0 {
		t.Fatalf("rollback result reports applied migrations: %v", result.Applied)
	}

	check, err := pgx.Connect(ctx, wizardTrialDatabaseURL)
	if err != nil {
		t.Fatalf("independent post-rollback connection: %v", err)
	}
	defer check.Close(context.Background())
	var checkDB, checkUser string
	if err := check.QueryRow(ctx, `SELECT current_database(),current_user`).Scan(&checkDB, &checkUser); err != nil {
		t.Fatal(err)
	}
	if checkDB != db || checkUser != "postgres" {
		t.Fatalf("postcheck connected to unexpected target: database=%q user=%q", checkDB, checkUser)
	}
	postHistory, err := readHistory(ctx, check)
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyCompleteHistory(migrations[:59], postHistory, 59); err != nil {
		t.Fatalf("post-rollback ledger differs from exact V1-V59 prefix: %v", err)
	}
	for i := range postHistory {
		if postHistory[i] != history[i] {
			t.Fatalf("post-rollback ledger entry %d changed", i+1)
		}
	}
	var functionExists, triggerExists bool
	if err := check.QueryRow(ctx, `SELECT to_regprocedure('public.reject_gadm_bootstrap_identity_change()') IS NOT NULL`).Scan(&functionExists); err != nil {
		t.Fatal(err)
	}
	if err := check.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_catalog.pg_trigger t JOIN pg_catalog.pg_class c ON c.oid=t.tgrelid JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relname='usuario' AND t.tgname='usuario_gadm_bootstrap_identity_immutable' AND NOT t.tgisinternal)`).Scan(&triggerExists); err != nil {
		t.Fatal(err)
	}
	if functionExists || triggerExists {
		t.Fatalf("V60 objects survived rollback: function=%v trigger=%v", functionExists, triggerExists)
	}
	after, err := snapshotWizardRollbackState(ctx, check)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatal("GADM bootstrap role/comment/ACL or membership state changed after rollback")
	}
}

func snapshotWizardRollbackState(ctx context.Context, conn *pgx.Conn) (wizardRoleSnapshot, error) {
	var s wizardRoleSnapshot
	const attrs = `SELECT COALESCE(jsonb_agg(jsonb_build_array(rolname,rolcanlogin,rolsuper,rolcreatedb,rolcreaterole,rolinherit,rolreplication,rolbypassrls,rolconnlimit,rolvaliduntil::text) ORDER BY rolname),'[]'::jsonb)::text FROM pg_catalog.pg_roles WHERE rolname='espacigo_gadm_bootstrap'`
	if err := conn.QueryRow(ctx, attrs).Scan(&s.Attributes); err != nil {
		return s, fmt.Errorf("snapshot bootstrap role attributes: %w", err)
	}
	const comment = `SELECT shobj_description(oid,'pg_authid') FROM pg_catalog.pg_roles WHERE rolname='espacigo_gadm_bootstrap'`
	if err := conn.QueryRow(ctx, comment).Scan(&s.Comment); err != nil {
		return s, fmt.Errorf("snapshot bootstrap shared role comment: %w", err)
	}
	const members = `SELECT COALESCE(jsonb_agg(jsonb_build_array(r.rolname,m.rolname,am.admin_option,am.inherit_option,am.set_option) ORDER BY r.rolname,m.rolname),'[]'::jsonb)::text FROM pg_catalog.pg_auth_members am JOIN pg_catalog.pg_roles r ON r.oid=am.roleid JOIN pg_catalog.pg_roles m ON m.oid=am.member WHERE r.rolname='espacigo_gadm_bootstrap' OR m.rolname='espacigo_gadm_bootstrap'`
	if err := conn.QueryRow(ctx, members).Scan(&s.Membership); err != nil {
		return s, fmt.Errorf("snapshot bootstrap memberships: %w", err)
	}
	const acl = `SELECT jsonb_build_object('database',COALESCE((SELECT jsonb_agg(jsonb_build_array(x.grantor::regrole::text,x.grantee::regrole::text,x.privilege_type,x.is_grantable) ORDER BY x.grantor,x.grantee,x.privilege_type) FROM pg_catalog.pg_database d CROSS JOIN LATERAL aclexplode(COALESCE(d.datacl,acldefault('d',d.datdba))) x WHERE d.datname=current_database() AND x.grantee=(SELECT oid FROM pg_roles WHERE rolname='espacigo_gadm_bootstrap')),'[]'::jsonb),'columns',COALESCE((SELECT jsonb_agg(jsonb_build_array(n.nspname,c.relname,a.attname,x.grantor::regrole::text,x.grantee::regrole::text,x.privilege_type,x.is_grantable) ORDER BY n.nspname,c.relname,a.attname,x.grantor,x.privilege_type) FROM pg_catalog.pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace JOIN pg_attribute a ON a.attrelid=c.oid AND a.attnum>0 AND NOT a.attisdropped CROSS JOIN LATERAL aclexplode(CASE WHEN cardinality(a.attacl)>0 THEN a.attacl ELSE NULL::aclitem[] END) x WHERE x.grantee=(SELECT oid FROM pg_roles WHERE rolname='espacigo_gadm_bootstrap')),'[]'::jsonb),'tables',COALESCE((SELECT jsonb_agg(jsonb_build_array(n.nspname,c.relname,x.grantor::regrole::text,x.grantee::regrole::text,x.privilege_type,x.is_grantable) ORDER BY n.nspname,c.relname,x.grantor,x.privilege_type) FROM pg_catalog.pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace CROSS JOIN LATERAL aclexplode(COALESCE(c.relacl,acldefault((CASE c.relkind WHEN 'S' THEN 's' ELSE 'r' END)::"char",c.relowner))) x WHERE c.relkind IN ('r','p','v','m','f','S') AND x.grantee=(SELECT oid FROM pg_roles WHERE rolname='espacigo_gadm_bootstrap')),'[]'::jsonb))::text`
	if err := conn.QueryRow(ctx, acl).Scan(&s.ACL); err != nil {
		return s, fmt.Errorf("snapshot bootstrap database/table/column ACLs: %w", err)
	}
	return s, nil
}
