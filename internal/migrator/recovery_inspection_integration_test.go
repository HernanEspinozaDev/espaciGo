package migrator

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestRecoveryInspectionPostgresReadonlySnapshot(t *testing.T) {
	if os.Getenv("TEST_RECOVERY_INSPECTION_ISOLATED") != "1" {
		t.Skip("requires private disposable recovery harness")
	}
	raw := os.Getenv("TEST_DATABASE_URL")
	u, err := url.Parse(raw)
	if err != nil || u.Query().Get("host") == "" {
		t.Fatal("private socket required")
	}
	if _, err := os.Stat(filepath.Join(u.Query().Get("host"), ".espacigo-recovery-ci")); err != nil {
		t.Fatal("private recovery marker required")
	}
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, raw)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = admin.Close(ctx) })
	const db = "espacigo_dms_stage_20261020"
	if _, err = admin.Exec(ctx, `CREATE DATABASE `+pgx.Identifier{db}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, e := admin.Exec(ctx, `DROP DATABASE `+pgx.Identifier{db}.Sanitize()+` WITH (FORCE)`); e != nil {
			t.Errorf("drop own disposable database: %v", e)
		}
	})
	u.Path = "/" + db
	setup, err := pgx.Connect(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer setup.Close(ctx)
	all, err := DiscoverMigrations(filepath.Join("..", "..", "db", "migrations"))
	if err != nil {
		t.Fatal(err)
	}
	prefix := t.TempDir()
	for _, m := range all[:2] {
		if err := os.WriteFile(filepath.Join(prefix, m.Name), m.SQL, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := Run(ctx, u.String(), prefix); err != nil {
		t.Fatal(err)
	}
	if _, err := setup.Exec(ctx, `CREATE ROLE espacigo_schema_owner NOLOGIN NOINHERIT;
 CREATE ROLE espacigo_migrator_login LOGIN NOINHERIT;
 CREATE ROLE espacigo_runtime NOLOGIN NOINHERIT;
 CREATE ROLE espacigo_document_verification_fn NOLOGIN NOINHERIT;
 GRANT espacigo_schema_owner TO espacigo_migrator_login WITH ADMIN FALSE, INHERIT FALSE, SET TRUE;
 GRANT espacigo_document_verification_fn TO espacigo_schema_owner WITH ADMIN FALSE, INHERIT FALSE, SET TRUE;
 GRANT USAGE ON SCHEMA public TO espacigo_schema_owner,espacigo_migrator_login;
 GRANT SELECT ON public.schema_migrations TO espacigo_schema_owner;
 GRANT SELECT(id) ON public.usuario TO espacigo_runtime;
 ALTER DEFAULT PRIVILEGES FOR ROLE postgres IN SCHEMA public GRANT SELECT ON TABLES TO espacigo_runtime;
 REVOKE CREATE ON SCHEMA public FROM PUBLIC,espacigo_schema_owner;`); err != nil {
		t.Fatal(err)
	}
	snapshot := func() string {
		t.Helper()
		var s string
		err := setup.QueryRow(ctx, `SELECT jsonb_build_object('ledger',(SELECT jsonb_agg(to_jsonb(x) ORDER BY version) FROM public.schema_migrations x),'rels',(SELECT jsonb_agg(jsonb_build_array(c.relname,c.relowner,c.relacl) ORDER BY c.relname) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relkind IN('r','S')),'columns',(SELECT jsonb_agg(jsonb_build_array(c.relname,a.attname,a.attacl) ORDER BY c.relname,a.attnum) FROM pg_attribute a JOIN pg_class c ON c.oid=a.attrelid JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND a.attnum>0 AND NOT a.attisdropped),'roles',(SELECT jsonb_agg(jsonb_build_array(rolname,rolcanlogin,rolsuper,rolcreaterole,rolcreatedb,rolinherit) ORDER BY rolname) FROM pg_roles WHERE rolname LIKE 'espacigo_%'),'members',(SELECT jsonb_agg(to_jsonb(m) ORDER BY roleid,member) FROM pg_auth_members m),'defaults',(SELECT jsonb_agg(to_jsonb(d) ORDER BY oid) FROM pg_default_acl d))::text`).Scan(&s)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	before := snapshot()
	login := *u
	login.User = url.User("espacigo_migrator_login")
	report, err := InspectRecovery(ctx, login.String(), prefix, "espacigo_schema_owner")
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("RECOVERY_REPORT_JSON=%s", encoded)
	if !report.ReadOnly || report.TransactionIsolation != "repeatable read" || report.Database != db || report.SessionUser != "espacigo_migrator_login" || report.CurrentUser != "espacigo_schema_owner" || len(report.Applied) != 2 || len(report.Pending) != 0 {
		t.Fatalf("wrong recovery report: %#v", report)
	}
	var effective map[string]bool
	if err := json.Unmarshal(report.Authority.Effective, &effective); err != nil {
		t.Fatal(err)
	}
	if effective["schema_public_create"] || effective["ledger_insert"] || !effective["ledger_select"] || !effective["schema_owner_public_usage"] || !effective["session_public_usage"] {
		t.Fatalf("missing privileges must be reported, not granted: %s", report.Authority.Effective)
	}
	var acls []map[string]any
	if err := json.Unmarshal(report.Authority.ACL, &acls); err != nil {
		t.Fatal(err)
	}
	column, defaults, public := false, false, false
	for _, a := range acls {
		if a["class"] == "column" && a["object"] == "usuario" && a["column"] == "id" && a["grantee"] == "espacigo_runtime" && a["owner"] == "postgres" {
			column = true
		}
		if a["class"] == "default" && a["grantee"] == "espacigo_runtime" && a["owner"] == "postgres" {
			defaults = true
		}
		if a["class"] == "schema" && a["grantee"] == "PUBLIC" {
			public = true
		}
	}
	if !column || !defaults || !public {
		t.Fatalf("bounded ACL metadata missing column/default/PUBLIC: %s", report.Authority.ACL)
	}
	if !strings.Contains(string(report.Authority.Memberships), "espacigo_document_verification_fn") || len(report.Authority.Roles) != 4 {
		t.Fatal("role/member facts absent")
	}
	if got := snapshot(); got != before {
		t.Fatal("inspection mutated ledger/catalog")
	}
	// Missing ledger is observed without initialization. Existing application
	// objects remain intact and subsequent inspection has an empty applied set.
	if _, err := setup.Exec(ctx, `DROP TABLE public.schema_migrations`); err != nil {
		t.Fatal(err)
	}
	missingBefore := snapshotWithoutLedger(t, ctx, setup)
	missing, err := InspectRecovery(ctx, login.String(), prefix, "espacigo_schema_owner")
	if err != nil {
		t.Fatal(err)
	}
	if len(missing.Applied) != 0 || len(missing.Pending) != 2 {
		t.Fatalf("missing ledger report %#v", missing)
	}
	if got := snapshotWithoutLedger(t, ctx, setup); got != missingBefore {
		t.Fatal("missing-ledger inspection initialized or modified schema")
	}
}

func snapshotWithoutLedger(t *testing.T, ctx context.Context, c *pgx.Conn) string {
	t.Helper()
	var s string
	if err := c.QueryRow(ctx, `SELECT COALESCE(to_regclass('public.schema_migrations')::text,'absent')||':'||(SELECT count(*)::text FROM pg_tables WHERE schemaname='public')`).Scan(&s); err != nil {
		t.Fatal(fmt.Errorf("snapshot missing ledger: %w", err))
	}
	return s
}
