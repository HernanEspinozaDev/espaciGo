package migrator

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/dbbootstrap"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func captureIdentityTailAuthorityPlan(t *testing.T, version int, plan PlanResult) {
	t.Helper()
	dir := os.Getenv("TEST_MIGRATION_AUTHORITY_REPORT_DIR")
	if dir == "" {
		return
	}
	name := "identity-tail-v95-plan.json"
	if version == 96 {
		name = "identity-tail-v96-plan.json"
	} else if version != 95 {
		t.Fatalf("unsupported identity-tail authority capture V%d", version)
	}
	wire, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(filepath.Join(dir, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if errors.Is(err, os.ErrExist) {
		return
	}
	if err != nil {
		t.Fatalf("create isolated identity-tail report: %v", err)
	}
	if _, err := file.Write(wire); err != nil {
		_ = file.Close()
		t.Fatalf("write isolated identity-tail report: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close isolated identity-tail report: %v", err)
	}
}

// This is a reproducible synthetic bootstrap, not an operational cloud repair.
// V81 deliberately requires administrator-provisioned roles; never relax that
// guard or change an applied migration to make an empty-cluster test pass.
func TestCurrentSchemaMigrationKYCThroughV96(t *testing.T) {
	if os.Getenv("TEST_MIGRATOR_CI_ISOLATED") != "1" {
		t.Skip("requires the disposable migration CI harness")
	}
	u, err := url.Parse(os.Getenv("TEST_DATABASE_URL"))
	if err != nil || u.Query().Get("host") == "" {
		t.Fatal("disposable CI requires its private Unix socket")
	}
	if _, err := os.Stat(filepath.Join(u.Query().Get("host"), ".espacigo-migration-ci")); err != nil {
		t.Fatal("disposable CI marker is missing")
	}
	all, err := DiscoverMigrations(filepath.Join("..", "..", "db", "migrations"))
	if err != nil {
		t.Fatal(err)
	}
	// A new migration needs a reviewed fresh/upgrade recipe, not silent omission.
	if len(all) != 97 {
		t.Fatal("update the reviewed current-schema bootstrap recipe for the new inventory")
	}
	// This historical KYC recipe is intentionally bounded before rental V97.
	all = all[:96]
	prefix := func(last int) string {
		t.Helper()
		dir := t.TempDir()
		for _, m := range all[:last] {
			if err := os.WriteFile(filepath.Join(dir, m.Name), m.SQL, 0600); err != nil {
				t.Fatal(err)
			}
		}
		return dir
	}
	for _, baseline := range []int{0, 75, 76} {
		t.Run(map[int]string{0: "empty", 75: "upgrade-v75", 76: "upgrade-v76"}[baseline], func(t *testing.T) {
			// The test database is dropped by withTestDatabase's defer. Register
			// cluster-role cleanup as a defer too: t.Fatal inside its callback
			// invokes Goexit, so statements after the helper would never run.
			defer func() {
				cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				cleanup, err := pgx.Connect(cleanupCtx, os.Getenv("TEST_DATABASE_URL"))
				if err != nil {
					t.Errorf("connect to disposable PostgreSQL for role cleanup: %v", err)
					return
				}
				defer cleanup.Close(context.Background())
				if _, err := cleanup.Exec(cleanupCtx, `DROP ROLE IF EXISTS espacigo_migrator_login, espacigo_api_login, espacigo_schema_owner, espacigo_document_verification_fn, espacigo_runtime, espacigo_privacy_worker_login, espacigo_privacy_cleanup_worker`); err != nil {
					t.Errorf("drop only this test's disposable cluster roles: %v", err)
				}
			}()
			withTestDatabase(t, func(ctx context.Context, databaseURL string) {
				admin := connectTestDB(t, ctx, databaseURL)
				defer admin.Close(context.Background())
				mustExec := func(sql string) {
					t.Helper()
					if _, err := admin.Exec(ctx, sql); err != nil {
						t.Fatal(err)
					}
				}
				execNativeRepair := func(name string) {
					t.Helper()
					source, err := os.ReadFile(filepath.Join("..", "..", "db", "repair", name))
					if err != nil {
						t.Fatalf("read native authority repair %s: %v", name, err)
					}
					var databaseOID uint32
					if err := admin.QueryRow(ctx, `SELECT oid FROM pg_catalog.pg_database WHERE datname=current_database()`).Scan(&databaseOID); err != nil {
						t.Fatal(err)
					}
					sql := string(source)
					if strings.Count(sql, "__NATIVE_TARGET_DATABASE__") != 1 || strings.Count(sql, "__NATIVE_TARGET_OID__") != 1 {
						t.Fatalf("native authority repair %s must contain exactly one target database and OID pin", name)
					}
					databaseLiteral := "'" + strings.ReplaceAll(admin.Config().Database, "'", "''") + "'"
					sql = strings.Replace(sql, "__NATIVE_TARGET_DATABASE__", databaseLiteral, 1)
					sql = strings.Replace(sql, "__NATIVE_TARGET_OID__", strconv.FormatUint(uint64(databaseOID), 10), 1)
					if _, err := admin.PgConn().Exec(ctx, sql).ReadAll(); err != nil {
						_, _ = admin.PgConn().Exec(context.Background(), "ROLLBACK").ReadAll()
						t.Fatalf("execute native authority repair %s: %v", name, err)
					}
				}
				if baseline == 76 {
					// Model the observed native predecessor without touching that target.
					mustExec(`CREATE ROLE espacigo_runtime LOGIN NOINHERIT`)
				} else {
					mustExec(`CREATE ROLE espacigo_runtime NOLOGIN NOINHERIT;
CREATE ROLE espacigo_api_login LOGIN NOINHERIT;
CREATE ROLE espacigo_migrator_login LOGIN NOINHERIT;
GRANT espacigo_runtime TO espacigo_api_login WITH ADMIN FALSE, INHERIT FALSE, SET TRUE;`)
				}
				var before []HistoryEntry
				if baseline != 0 {
					if _, err := Run(ctx, databaseURL, prefix(baseline)); err != nil {
						t.Fatal(err)
					}
					before, err = readHistory(ctx, admin)
					if err != nil {
						t.Fatal(err)
					}
				}
				if baseline == 76 {
					if err := dbbootstrap.GrantRuntimePermissions(ctx, admin); err != nil {
						t.Fatal(err)
					}
					t.Run("v77-duplicate-predecessor-blocks", func(t *testing.T) {
						const definitionQuery = `SELECT pg_catalog.pg_get_functiondef('public.img01b_enforce_gallery_slot()'::regprocedure)`
						var original string
						if err := admin.QueryRow(ctx, definitionQuery).Scan(&original); err != nil {
							t.Fatal(err)
						}
						if strings.Count(original, v77OriginalFragment) != 1 {
							t.Fatal("V76 fixture does not contain exactly one predecessor fragment")
						}
						// The second occurrence is inert but belongs to the stored
						// body returned by pg_get_functiondef, not trailing SQL.
						duplicate := strings.Replace(original, v77OriginalFragment, "-- "+v77OriginalFragment+"\n    "+v77OriginalFragment, 1)
						if _, err := admin.Exec(ctx, duplicate); err != nil {
							t.Fatal(err)
						}
						t.Cleanup(func() {
							if _, err := admin.Exec(context.Background(), original); err != nil {
								t.Errorf("restore own V76 function fixture: %v", err)
							}
						})
						var injected string
						if err := admin.QueryRow(ctx, definitionQuery).Scan(&injected); err != nil {
							t.Fatal(err)
						}
						if strings.Count(injected, v77OriginalFragment) != 2 {
							t.Fatal("duplicate fixture was not persisted in the function body")
						}
						result, applyErr := Run(ctx, databaseURL, prefix(77))
						if applyErr == nil || !strings.Contains(applyErr.Error(), "V77 predecessor-function guard") || !strings.Contains(applyErr.Error(), "found 2") || len(result.Applied) != 0 {
							t.Fatalf("duplicate predecessor was not rejected before V77 DDL: result=%#v err=%v", result, applyErr)
						}
						history, err := readHistory(ctx, admin)
						if err != nil || len(history) != len(before) {
							t.Fatalf("V77 rejection changed ledger length: %v", err)
						}
						for i := range before {
							if history[i] != before[i] {
								t.Fatalf("V77 rejection changed ledger row %d", i)
							}
						}
						var after string
						if err := admin.QueryRow(ctx, definitionQuery).Scan(&after); err != nil || after != injected {
							t.Fatalf("V77 rejection changed stored function: %v", err)
						}
					})
				}
				if _, err := Run(ctx, databaseURL, prefix(80)); err != nil {
					t.Fatal(err)
				}
				if baseline == 76 {
					t.Run("native-role-transition-contract", func(t *testing.T) {
						var beforeOID, afterOID uint32
						var beforeLogin, afterLogin, canRead bool
						if err := admin.QueryRow(ctx, `SELECT oid,rolcanlogin FROM pg_catalog.pg_roles WHERE rolname='espacigo_runtime'`).Scan(&beforeOID, &beforeLogin); err != nil || !beforeLogin {
							t.Fatalf("fixture did not reproduce native LOGIN predecessor: %v", err)
						}
						if err := ApplyNativeV80RoleTransition(ctx, databaseURL, filepath.Join("..", "..", "db", "migrations"), admin.Config().Database, 0); err != nil {
							t.Fatalf("versioned native role transition rejected the V80 fixture: %v", err)
						}
						if err := admin.QueryRow(ctx, `SELECT oid,rolcanlogin,has_table_privilege(oid,'public.usuario','SELECT') FROM pg_catalog.pg_roles WHERE rolname='espacigo_runtime'`).Scan(&afterOID, &afterLogin, &canRead); err != nil || beforeOID != afterOID || afterLogin || !canRead {
							t.Fatalf("role transition replaced identity or lost runtime grants: %v", err)
						}
						apiURL, _ := url.Parse(databaseURL)
						apiURL.User = url.User("espacigo_api_login")
						api := connectTestDB(t, ctx, apiURL.String())
						defer api.Close(context.Background())
						if _, err := api.Exec(ctx, `SET ROLE espacigo_runtime; SELECT id FROM public.usuario LIMIT 0`); err != nil {
							t.Fatalf("separate API login cannot assume preserved runtime: %v", err)
						}
					})
				}
				if baseline != 76 {
					mustExec(`CREATE ROLE espacigo_schema_owner NOLOGIN NOINHERIT;
CREATE ROLE espacigo_document_verification_fn NOLOGIN NOINHERIT;
CREATE SCHEMA document_verification_private AUTHORIZATION espacigo_document_verification_fn;
GRANT USAGE ON SCHEMA public TO espacigo_document_verification_fn, espacigo_schema_owner;
GRANT SELECT(id,estado), UPDATE(id) ON public.usuario TO espacigo_document_verification_fn;
GRANT SELECT(id,propietario_id), UPDATE(id) ON public.espacio TO espacigo_document_verification_fn;
GRANT espacigo_schema_owner TO espacigo_migrator_login WITH ADMIN FALSE, INHERIT FALSE, SET TRUE;
GRANT espacigo_document_verification_fn TO espacigo_schema_owner WITH ADMIN FALSE, INHERIT FALSE, SET TRUE;
GRANT SELECT,INSERT ON public.schema_migrations TO espacigo_schema_owner;`)
				}
				mustExec("REVOKE ALL ON DATABASE " + pgx.Identifier{admin.Config().Database}.Sanitize() + " FROM PUBLIC")
				mustExec("GRANT TEMPORARY ON DATABASE " + pgx.Identifier{admin.Config().Database}.Sanitize() + " TO PUBLIC")
				mustExec("GRANT CONNECT ON DATABASE " + pgx.Identifier{admin.Config().Database}.Sanitize() + " TO espacigo_migrator_login, espacigo_api_login")
				migrationURL, _ := url.Parse(databaseURL)
				migrationURL.User = url.User("espacigo_migrator_login")
				if _, err := RunWithRole(ctx, migrationURL.String(), prefix(81), "espacigo_schema_owner"); err != nil {
					t.Fatal(err)
				}
				// V82/V83 use the versioned, guarded native ownership/grant window.
				execNativeRepair("prepare_native_v82_v83.sql.tmpl")
				if _, err := RunWithRole(ctx, migrationURL.String(), prefix(83), "espacigo_schema_owner"); err != nil {
					t.Fatal(err)
				}
				execNativeRepair("finalize_native_v83.sql.tmpl")
				// V84 expands identity verification schema; requires CREATE on public schema and REFERENCES on usuario.
				// Close the authority window on failure, but do not rerun this V88-bounded
				// finalizer after later migrations have advanced the ledger.
				v84AuthorityWindowOpen := false
				defer func() {
					if v84AuthorityWindowOpen {
						execNativeRepair("finalize_native_v85.sql.tmpl")
					}
				}()
				mustExec(`CREATE SCHEMA tiger;
CREATE SCHEMA topology;
GRANT USAGE ON SCHEMA tiger, topology TO espacigo_schema_owner;`)
				mustExec("ALTER ROLE espacigo_migrator_login IN DATABASE " + pgx.Identifier{admin.Config().Database}.Sanitize() + ` SET search_path TO "$user", public, tiger, topology`)
				withoutV84Authority, err := Plan(ctx, migrationURL.String(), prefix(84), "espacigo_schema_owner")
				if err != nil {
					t.Fatal(err)
				}
				if withoutV84Authority.DDLAuthority == nil {
					t.Fatal("V84 preflight omitted effective DDL authority")
				}
				if got := strings.Join(withoutV84Authority.DDLAuthority.Catalog.Schemas, ","); got != "pg_catalog,public" {
					t.Fatalf("V84 preflight did not constrain its effective search path: got %q", got)
				}
				if withoutV84Authority.DDLAuthority.SessionUser != "espacigo_migrator_login" || withoutV84Authority.DDLAuthority.CurrentUser != "espacigo_schema_owner" || withoutV84Authority.DDLAuthority.CurrentSchema != "public" || !withoutV84Authority.DDLAuthority.PublicUsage || withoutV84Authority.DDLAuthority.PublicCreate || withoutV84Authority.DDLAuthority.UsuarioIDReferences || !withoutV84Authority.DDLAuthority.LedgerSelect || !withoutV84Authority.DDLAuthority.LedgerInsert {
					t.Fatalf("V84 preflight did not report missing CREATE/REFERENCES separately: %#v", withoutV84Authority.DDLAuthority)
				}
				rejectV84WithoutAuthority := func(label string) {
					t.Helper()
					if _, err := RunWithRole(ctx, migrationURL.String(), prefix(84), "espacigo_schema_owner"); err == nil {
						t.Fatalf("V84 runner accepted incomplete authority: %s", label)
					} else if !strings.Contains(err.Error(), "V84 preflight lacks effective schema, REFERENCES or migration-ledger privileges") {
						t.Fatalf("V84 runner rejected for an unrelated reason (%s): %v", label, err)
					}
					// Separate diagnostic, only in this disposable cluster: prove the
					// underlying SQLSTATE without weakening the production guard.
					probe := connectTestDB(t, ctx, migrationURL.String())
					func() {
						defer probe.Close(context.Background())
						tx, err := probe.Begin(ctx)
						if err != nil {
							t.Fatal(err)
						}
						defer tx.Rollback(context.Background())
						if _, err := tx.Exec(ctx, `SET LOCAL ROLE espacigo_schema_owner`); err != nil {
							t.Fatal(err)
						}
						_, ddlErr := tx.Exec(ctx, string(all[83].SQL))
						var pgErr *pgconn.PgError
						object := "schema public"
						if label == "CREATE without REFERENCES" {
							object = "table usuario"
						}
						if !errors.As(ddlErr, &pgErr) || pgErr.Code != "42501" || !strings.Contains(pgErr.Message, object) {
							t.Fatalf("unexpected direct V84 diagnostic (%s): %v", label, ddlErr)
						}
					}()
					history, err := readHistory(ctx, admin)
					if err != nil || len(history) != 83 || history[82].Version != 83 {
						t.Fatalf("V84 rejection changed the migration ledger (%s): %v", label, err)
					}
					var v84ObjectsExist bool
					if err := admin.QueryRow(ctx, `SELECT pg_catalog.to_regclass('public.identity_verification_attempts') IS NOT NULL OR pg_catalog.to_regclass('public.didit_webhook_inbox') IS NOT NULL`).Scan(&v84ObjectsExist); err != nil || v84ObjectsExist {
						t.Fatalf("V84 rejection left schema objects behind (%s): %v", label, err)
					}
				}
				rejectV84WithoutAuthority("neither CREATE nor REFERENCES")
				mustExec(`GRANT CREATE ON SCHEMA public TO espacigo_schema_owner;`)
				createOnly, err := Plan(ctx, migrationURL.String(), prefix(84), "espacigo_schema_owner")
				if err != nil || createOnly.DDLAuthority == nil || !createOnly.DDLAuthority.PublicCreate || createOnly.DDLAuthority.UsuarioIDReferences {
					t.Fatalf("V84 preflight did not isolate missing REFERENCES: %v (%#v)", err, createOnly.DDLAuthority)
				}
				rejectV84WithoutAuthority("CREATE without REFERENCES")
				mustExec(`REVOKE CREATE ON SCHEMA public FROM espacigo_schema_owner;
GRANT REFERENCES(id) ON public.usuario TO espacigo_schema_owner;`)
				referencesOnly, err := Plan(ctx, migrationURL.String(), prefix(84), "espacigo_schema_owner")
				if err != nil || referencesOnly.DDLAuthority == nil || referencesOnly.DDLAuthority.PublicCreate || !referencesOnly.DDLAuthority.UsuarioIDReferences {
					t.Fatalf("V84 preflight did not isolate missing CREATE: %v (%#v)", err, referencesOnly.DDLAuthority)
				}
				rejectV84WithoutAuthority("REFERENCES without CREATE")
				mustExec(`REVOKE REFERENCES(id) ON public.usuario FROM espacigo_schema_owner;`)
				execNativeRepair("prepare_native_v84_v85.sql.tmpl")
				v84AuthorityWindowOpen = true
				withV84Authority, err := Plan(ctx, migrationURL.String(), prefix(84), "espacigo_schema_owner")
				if err != nil {
					t.Fatal(err)
				}
				if withV84Authority.DDLAuthority == nil {
					t.Fatal("V84 preflight omitted effective DDL authority after the bounded grants")
				}
				if err := requireV84DDLAuthority(*withV84Authority.DDLAuthority); err != nil {
					t.Fatalf("V84 preflight rejected the exact minimal grant window: %v (%#v)", err, withV84Authority.DDLAuthority)
				}
				t.Run("authority-wire", func(t *testing.T) {
					payload, err := json.Marshal(withV84Authority.DDLAuthority)
					if err != nil {
						t.Fatal(err)
					}
					cmd := exec.CommandContext(ctx, "python3", "-c", `import json,sys; sys.path.insert(0,sys.argv[1]); from v84_authority import validate_catalog; validate_catalog(json.load(sys.stdin))`, filepath.Join("..", "..", "scripts", "gcp"))
					cmd.Stdin = bytes.NewReader(payload)
					if out, err := cmd.CombinedOutput(); err != nil {
						t.Fatalf("real PostgreSQL authority Go→JSON→Python rejected: %v %s", err, out)
					}
				})
				for _, negative := range []struct{ name, change, undo, reason string }{
					{"extra-outgoing-membership", `GRANT pg_read_all_data TO espacigo_schema_owner WITH ADMIN FALSE, INHERIT FALSE, SET TRUE`, `REVOKE pg_read_all_data FROM espacigo_schema_owner`, "memberships"},
					{"extra-incoming-membership", `GRANT espacigo_schema_owner TO espacigo_api_login WITH ADMIN FALSE, INHERIT FALSE, SET TRUE`, `REVOKE espacigo_schema_owner FROM espacigo_api_login`, "memberships"},
					{"helper-escalation", `ALTER ROLE espacigo_document_verification_fn BYPASSRLS`, `ALTER ROLE espacigo_document_verification_fn NOBYPASSRLS`, "helper"},
					{"unexpected-direct-acl", `GRANT UPDATE ON public.usuario TO espacigo_schema_owner`, `REVOKE UPDATE ON public.usuario FROM espacigo_schema_owner`, "unexpected ACL"},
					{"grant-option", `GRANT REFERENCES(id) ON public.usuario TO espacigo_schema_owner WITH GRANT OPTION`, `REVOKE GRANT OPTION FOR REFERENCES(id) ON public.usuario FROM espacigo_schema_owner`, "unexpected ACL"},
					{"public-data-exposure", `GRANT SELECT ON public.usuario TO PUBLIC`, `REVOKE SELECT ON public.usuario FROM PUBLIC`, "unexpected ACL"},
					{"unsafe-table-default", `ALTER DEFAULT PRIVILEGES FOR ROLE espacigo_schema_owner IN SCHEMA public GRANT SELECT ON TABLES TO PUBLIC`, `ALTER DEFAULT PRIVILEGES FOR ROLE espacigo_schema_owner IN SCHEMA public REVOKE SELECT ON TABLES FROM PUBLIC`, "unexpected ACL"},
				} {
					t.Run("authority-negative/"+negative.name, func(t *testing.T) {
						mustExec(negative.change)
						defer mustExec(negative.undo)
						p, err := Plan(ctx, migrationURL.String(), prefix(84), "espacigo_schema_owner")
						if err != nil || p.DDLAuthority == nil {
							t.Fatalf("negative preflight unavailable: %v", err)
						}
						if err := requireV84DDLAuthority(*p.DDLAuthority); err == nil || !strings.Contains(err.Error(), negative.reason) {
							t.Fatalf("wrong authority rejection: %v", err)
						}
						if _, err := RunWithRole(ctx, migrationURL.String(), prefix(84), "espacigo_schema_owner"); err == nil || !strings.Contains(err.Error(), negative.reason) {
							t.Fatalf("runner did not reject before DDL: %v", err)
						}
						history, err := readHistory(ctx, admin)
						if err != nil || len(history) != 83 {
							t.Fatalf("negative changed ledger: %v", err)
						}
					})
				}
				t.Run("release-v83-v84", func(t *testing.T) {
					conn := connectTestDB(t, ctx, migrationURL.String())
					defer conn.Close(context.Background())
					report, err := runReleaseWithConn(ctx, conn, prefix(84), "espacigo_schema_owner", ReleaseContract{
						SourceSHA: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", InventorySHA256: inventoryFingerprint(all[:84]), ExpectedApplied: 83, Database: admin.Config().Database,
					})
					if err != nil {
						t.Fatal(err)
					}
					if len(report.Applied) != 1 || report.Applied[0] != 84 || len(report.Before.Applied) != 83 || report.Before.DDLAuthority == nil || len(report.After.Applied) != 84 || len(report.After.Pending) != 0 {
						t.Fatalf("restricted V83→V84 release evidence incomplete: %#v", report)
					}
					payload, _ := json.Marshal(report)
					var wire map[string]json.RawMessage
					if err := json.Unmarshal(payload, &wire); err != nil {
						t.Fatal(err)
					}
					beforeAuthority, _ := json.Marshal(report.Before.DDLAuthority)
					if !bytes.Equal(wire["ddl_authority"], beforeAuthority) {
						t.Fatal("release omitted the actual in-transaction V84 authority or differs from reviewed preflight")
					}
				})
				execNativeRepair("finalize_native_v85.sql.tmpl")
				v84AuthorityWindowOpen = false
				var lingeringAuthority bool
				if err := admin.QueryRow(ctx, `SELECT has_schema_privilege('espacigo_schema_owner','public','CREATE') OR has_column_privilege('espacigo_schema_owner','public.usuario','id','REFERENCES')`).Scan(&lingeringAuthority); err != nil || lingeringAuthority {
					t.Fatalf("V84 temporary authority survived revocation: %v", err)
				}
				plan, err := Plan(ctx, migrationURL.String(), prefix(84), "espacigo_schema_owner")
				if err != nil {
					t.Fatal(err)
				}
				if !plan.ReadOnly || len(plan.Applied) != 84 || len(plan.Pending) != 0 {
					t.Fatalf("incomplete postcheck: %#v", plan)
				}
				for i, entry := range before {
					if plan.Applied[i] != entry {
						t.Fatal("upgrade rewrote the existing migration ledger")
					}
				}
				result, err := RunWithRole(ctx, migrationURL.String(), prefix(84), "espacigo_schema_owner")
				if err != nil || len(result.Applied) != 0 {
					t.Fatalf("reapply was not a no-op: %v", err)
				}
				t.Run("release-v84-v85", func(t *testing.T) {
					execNativeRepair("prepare_native_v84_v85.sql.tmpl")
					t.Run("legacy-data-blocks-without-deleting", func(t *testing.T) { assertV85RejectsLegacyData(t, ctx, admin, migrationURL.String(), prefix(85)) })
					conn := connectTestDB(t, ctx, migrationURL.String())
					defer conn.Close(context.Background())
					report, err := runReleaseWithConn(ctx, conn, prefix(85), "espacigo_schema_owner", ReleaseContract{
						SourceSHA: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", InventorySHA256: inventoryFingerprint(all[:85]), ExpectedApplied: 84, Database: admin.Config().Database,
					})
					if err != nil {
						t.Fatal(err)
					}
					if len(report.Applied) != 1 || report.Applied[0] != 85 || len(report.After.Applied) != 85 || len(report.After.Pending) != 0 || report.Before.DDLAuthority == nil || report.DDLAuthority == nil {
						t.Fatal("V85 release authority/ledger incomplete")
					}
					// The native finalizer is intentionally gated for its later V1..V87
					// operational boundary; close only this disposable fixture's V85
					// temporary grants before exercising V86/V88 separately.
					mustExec(`REVOKE CREATE ON SCHEMA public FROM espacigo_schema_owner;
REVOKE REFERENCES(id) ON TABLE public.usuario FROM espacigo_schema_owner`)
					var lingeringV85Authority bool
					if err := admin.QueryRow(ctx, `SELECT has_schema_privilege('espacigo_schema_owner','public','CREATE') OR has_column_privilege('espacigo_schema_owner','public.usuario','id','REFERENCES')`).Scan(&lingeringV85Authority); err != nil || lingeringV85Authority {
						t.Fatalf("V85 temporary authority survived fixture cleanup: %v", err)
					}
				})
				t.Run("release-v85-v86", func(t *testing.T) {
					if _, err := admin.Exec(ctx, `GRANT CREATE ON SCHEMA public TO espacigo_schema_owner`); err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() {
						if _, err := admin.Exec(context.Background(), `REVOKE CREATE ON SCHEMA public FROM espacigo_schema_owner`); err != nil {
							t.Errorf("revoke disposable V86 CREATE fixture: %v", err)
						}
					})
					conn := connectTestDB(t, ctx, migrationURL.String())
					defer conn.Close(context.Background())
					report, err := runReleaseWithConn(ctx, conn, prefix(86), "espacigo_schema_owner", ReleaseContract{
						SourceSHA: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", InventorySHA256: inventoryFingerprint(all[:86]), ExpectedApplied: 85, Database: admin.Config().Database,
					})
					if err != nil {
						t.Fatal(err)
					}
					if len(report.Applied) != 1 || report.Applied[0] != 86 || len(report.Before.Applied) != 85 || len(report.After.Applied) != 86 || len(report.After.Pending) != 0 || report.Before.DDLAuthority == nil || report.DDLAuthority == nil {
						t.Fatalf("V86 release authority/ledger incomplete: %#v", report)
					}
				})
				t.Run("release-v86-v87", func(t *testing.T) {
					if _, err := admin.Exec(ctx, `CREATE ROLE espacigo_privacy_worker_login LOGIN NOINHERIT NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS;
GRANT espacigo_privacy_cleanup_worker TO espacigo_privacy_worker_login WITH ADMIN FALSE, INHERIT FALSE, SET TRUE`); err != nil {
						t.Fatal(err)
					}
					conn := connectTestDB(t, ctx, migrationURL.String())
					defer conn.Close(context.Background())
					report, err := runReleaseWithConn(ctx, conn, prefix(87), "espacigo_schema_owner", ReleaseContract{
						SourceSHA: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", InventorySHA256: inventoryFingerprint(all[:87]), ExpectedApplied: 86, Database: admin.Config().Database,
					})
					if err != nil {
						t.Fatal(err)
					}
					if len(report.Applied) != 1 || report.Applied[0] != 87 || len(report.Before.Applied) != 86 || len(report.After.Applied) != 87 || len(report.After.Pending) != 0 || report.Before.DDLAuthority == nil || report.DDLAuthority == nil {
						t.Fatalf("V87 release authority/ledger incomplete: %#v", report)
					}
					// Verify V87's original ACL behavior before V88 tightens it.
					t.Run("v87-identity-retention-worker", func(t *testing.T) {
						assertV87IdentityDeclarationRetentionAndWorker(t, ctx, databaseURL, admin)
					})
				})
				t.Run("release-v87-v88", func(t *testing.T) {
					if _, err := admin.Exec(ctx, `GRANT CREATE ON SCHEMA public TO espacigo_schema_owner`); err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() {
						if _, err := admin.Exec(context.Background(), `REVOKE CREATE ON SCHEMA public FROM espacigo_schema_owner`); err != nil {
							t.Errorf("revoke disposable V88 CREATE fixture: %v", err)
						}
					})
					conn := connectTestDB(t, ctx, migrationURL.String())
					defer conn.Close(context.Background())
					report, err := runReleaseWithConn(ctx, conn, prefix(88), "espacigo_schema_owner", ReleaseContract{
						SourceSHA: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", InventorySHA256: inventoryFingerprint(all[:88]), ExpectedApplied: 87, Database: admin.Config().Database,
					})
					if err != nil {
						t.Fatal(err)
					}
					if len(report.Applied) != 1 || report.Applied[0] != 88 || len(report.Before.Applied) != 87 || len(report.After.Applied) != 88 || len(report.After.Pending) != 0 {
						t.Fatalf("V88 release authority/ledger incomplete: %#v", report)
					}
				})
				t.Run("release-v88-v89", func(t *testing.T) {
					if _, err := admin.Exec(ctx, `GRANT CREATE ON SCHEMA public TO espacigo_schema_owner; GRANT REFERENCES(id) ON TABLE public.usuario TO espacigo_schema_owner`); err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() {
						if _, err := admin.Exec(context.Background(), `REVOKE CREATE ON SCHEMA public FROM espacigo_schema_owner; REVOKE REFERENCES(id) ON TABLE public.usuario FROM espacigo_schema_owner`); err != nil {
							t.Errorf("revoke disposable V89 authority fixture: %v", err)
						}
					})
					conn := connectTestDB(t, ctx, migrationURL.String())
					defer conn.Close(context.Background())
					report, err := runReleaseWithConn(ctx, conn, prefix(89), "espacigo_schema_owner", ReleaseContract{
						SourceSHA: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", InventorySHA256: inventoryFingerprint(all[:89]), ExpectedApplied: 88, Database: admin.Config().Database,
					})
					if err != nil || len(report.Applied) != 1 || report.Applied[0] != 89 || len(report.After.Applied) != 89 || len(report.After.Pending) != 0 {
						t.Fatalf("V89 release/ledger failed: %v report=%#v", err, report)
					}
				})
				t.Run("release-v89-v90", func(t *testing.T) {
					if _, err := admin.Exec(ctx, `GRANT CREATE ON SCHEMA public TO espacigo_schema_owner`); err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() {
						if _, err := admin.Exec(context.Background(), `REVOKE CREATE ON SCHEMA public FROM espacigo_schema_owner`); err != nil {
							t.Errorf("revoke disposable V90 CREATE fixture: %v", err)
						}
					})
					conn := connectTestDB(t, ctx, migrationURL.String())
					defer conn.Close(context.Background())
					report, err := runReleaseWithConn(ctx, conn, prefix(90), "espacigo_schema_owner", ReleaseContract{
						SourceSHA: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", InventorySHA256: inventoryFingerprint(all[:90]), ExpectedApplied: 89, Database: admin.Config().Database,
					})
					if err != nil || len(report.Applied) != 1 || report.Applied[0] != 90 || len(report.After.Applied) != 90 || len(report.After.Pending) != 0 {
						t.Fatalf("V90 release/ledger failed: %v report=%#v", err, report)
					}
					// Mirror the separately reviewed V90 least-privilege definer ACL
					// repair on this disposable target; the runtime approval path
					// inserts through commit_verified_identity_decision, not directly.
					if _, err := admin.Exec(ctx, `GRANT SELECT (clave_deduplicacion,agregado_tipo,agregado_id,tipo) ON public.outbox_evento_local TO espacigo_schema_owner;
GRANT INSERT (id,agregado_tipo,agregado_id,tipo,clave_deduplicacion,version,creada_en,disponible_en) ON public.outbox_evento_local TO espacigo_schema_owner`); err != nil {
						t.Fatalf("apply synthetic V90 definer ACL fixture: %v", err)
					}
					var outboxDefinerACL bool
					if err := admin.QueryRow(ctx, `SELECT
  (SELECT count(*)=4 FROM unnest(ARRAY['clave_deduplicacion','agregado_tipo','agregado_id','tipo']) AS c(name)
   WHERE has_column_privilege('espacigo_schema_owner','public.outbox_evento_local',c.name,'SELECT'))
  AND (SELECT count(*)=8 FROM unnest(ARRAY['id','agregado_tipo','agregado_id','tipo','clave_deduplicacion','version','creada_en','disponible_en']) AS c(name)
   WHERE has_column_privilege('espacigo_schema_owner','public.outbox_evento_local',c.name,'INSERT'))
  AND NOT has_table_privilege('espacigo_schema_owner','public.outbox_evento_local','INSERT')`).Scan(&outboxDefinerACL); err != nil || !outboxDefinerACL {
						t.Fatalf("synthetic V90 minimal outbox definer ACL readback failed: %v", err)
					}
					var safe bool
					if err := admin.QueryRow(ctx, `SELECT
  (SELECT is_nullable='YES' FROM information_schema.columns WHERE table_schema='public' AND table_name='identity_rut_claim' AND column_name='declaration_id')
  AND (SELECT is_nullable='YES' FROM information_schema.columns WHERE table_schema='public' AND table_name='identity_provider_attempt' AND column_name='declaration_id')
  AND (SELECT p.prosecdef AND p.proconfig @> ARRAY['search_path=pg_catalog'] AND r.rolname='espacigo_schema_owner'
       FROM pg_proc p JOIN pg_roles r ON r.oid=p.proowner WHERE p.oid='public.commit_verified_identity_decision(uuid,uuid,text,text,timestamp with time zone)'::regprocedure)
  AND has_function_privilege('espacigo_runtime','public.commit_verified_identity_decision(uuid,uuid,text,text,timestamp with time zone)','EXECUTE')
  AND NOT EXISTS (SELECT 1 FROM pg_proc p CROSS JOIN LATERAL aclexplode(COALESCE(p.proacl,acldefault('f'::"char",p.proowner))) a
                  WHERE p.oid='public.commit_verified_identity_decision(uuid,uuid,text,text,timestamp with time zone)'::regprocedure
                    AND a.grantee=0 AND a.privilege_type='EXECUTE')`).Scan(&safe); err != nil || !safe {
						t.Fatalf("V90 narrow commit function/nullable contracts failed: %v", err)
					}
				})
				t.Run("release-v90-v91", func(t *testing.T) {
					conn := connectTestDB(t, ctx, migrationURL.String())
					defer conn.Close(context.Background())
					report, err := runReleaseWithConn(ctx, conn, prefix(91), "espacigo_schema_owner", ReleaseContract{
						SourceSHA: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", InventorySHA256: inventoryFingerprint(all[:91]), ExpectedApplied: 90, Database: admin.Config().Database,
					})
					if err != nil || len(report.Applied) != 1 || report.Applied[0] != 91 || len(report.Before.Applied) != 90 || len(report.After.Applied) != 91 || len(report.After.Pending) != 0 {
						t.Fatalf("V91 release/ledger failed: %v report=%#v", err, report)
					}
					var definition string
					if err := admin.QueryRow(ctx, `SELECT pg_catalog.pg_get_constraintdef(oid) FROM pg_catalog.pg_constraint WHERE conrelid='public.identity_provider_attempt'::regclass AND conname='identity_provider_attempt_detached_approved_ck' AND contype='c'`).Scan(&definition); err != nil {
						t.Fatal(err)
					}
					for _, status := range []string{"APPROVED", "REJECTED", "EXPIRED"} {
						if !strings.Contains(definition, status) {
							t.Fatalf("V91 constraint omits %s: %s", status, definition)
						}
					}
					for _, clause := range []string{"decision_key_version IS NULL", "decision_algorithm IS NULL", "encrypted_decision IS NULL"} {
						if !strings.Contains(definition, clause) {
							t.Fatalf("V91 detached constraint omits %q: %s", clause, definition)
						}
					}
				})
				t.Run("release-v91-v92", func(t *testing.T) {
					mustExec("GRANT CREATE ON SCHEMA public TO espacigo_schema_owner")
					t.Cleanup(func() {
						if _, err := admin.Exec(context.Background(), "REVOKE CREATE ON SCHEMA public FROM espacigo_schema_owner"); err != nil {
							t.Errorf("revoke disposable V92 CREATE fixture: %v", err)
						}
					})
					conn := connectTestDB(t, ctx, migrationURL.String())
					defer conn.Close(context.Background())
					report, err := runReleaseWithConn(ctx, conn, prefix(92), "espacigo_schema_owner", ReleaseContract{
						SourceSHA: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", InventorySHA256: inventoryFingerprint(all[:92]), ExpectedApplied: 91, Database: admin.Config().Database,
					})
					if err != nil || len(report.Applied) != 1 || report.Applied[0] != 92 || len(report.Before.Applied) != 91 || len(report.After.Applied) != 92 || len(report.After.Pending) != 0 {
						t.Fatalf("V92 release/ledger failed: %v report=%#v", err, report)
					}
					var definition string
					if err := admin.QueryRow(ctx, `SELECT pg_catalog.pg_get_functiondef('public.purge_expired_identity_declaration_revisions()'::regprocedure)`).Scan(&definition); err != nil || !strings.Contains(definition, "LIMIT 100") || !strings.Contains(definition, "RETURNS bigint") {
						t.Fatalf("V92 function body/signature does not preserve bounded count-100 behavior: %v", err)
					}
					var workerExecute, runtimeExecute, publicExecute bool
					if err := admin.QueryRow(ctx, `SELECT
						pg_catalog.has_function_privilege('espacigo_privacy_cleanup_worker','public.purge_expired_identity_declaration_revisions()','EXECUTE'),
						pg_catalog.has_function_privilege('espacigo_runtime','public.purge_expired_identity_declaration_revisions()','EXECUTE'),
						EXISTS (SELECT 1 FROM pg_catalog.pg_proc p CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(p.proacl, pg_catalog.acldefault('f'::"char",p.proowner))) acl
						        WHERE p.oid='public.purge_expired_identity_declaration_revisions()'::regprocedure AND acl.grantee=0 AND acl.privilege_type='EXECUTE')`).Scan(&workerExecute, &runtimeExecute, &publicExecute); err != nil || !workerExecute || runtimeExecute || publicExecute {
						t.Fatalf("V92 worker-only ACL/readback failed: worker=%v runtime=%v public=%v err=%v", workerExecute, runtimeExecute, publicExecute, err)
					}
					var createStillGranted bool
					if err := admin.QueryRow(ctx, `SELECT has_schema_privilege('espacigo_schema_owner','public','CREATE')`).Scan(&createStillGranted); err != nil || !createStillGranted {
						t.Fatalf("V92 migration did not leave CREATE fixture available for caller revocation: %v", err)
					}
					mustExec("REVOKE CREATE ON SCHEMA public FROM espacigo_schema_owner")
					var createAfterRevoke bool
					if err := admin.QueryRow(ctx, `SELECT has_schema_privilege('espacigo_schema_owner','public','CREATE')`).Scan(&createAfterRevoke); err != nil || createAfterRevoke {
						t.Fatalf("V92 temporary CREATE revocation readback failed: %v", err)
					}
				})
				t.Run("release-v92-v93", func(t *testing.T) {
					mustExec("GRANT CREATE ON SCHEMA public TO espacigo_schema_owner")
					t.Cleanup(func() {
						if _, err := admin.Exec(context.Background(), "REVOKE CREATE ON SCHEMA public FROM espacigo_schema_owner"); err != nil {
							t.Errorf("revoke disposable V93 CREATE fixture: %v", err)
						}
					})
					conn := connectTestDB(t, ctx, migrationURL.String())
					defer conn.Close(context.Background())
					report, err := runReleaseWithConn(ctx, conn, prefix(93), "espacigo_schema_owner", ReleaseContract{
						SourceSHA: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", InventorySHA256: inventoryFingerprint(all[:93]), ExpectedApplied: 92, Database: admin.Config().Database,
					})
					if err != nil || len(report.Applied) != 1 || report.Applied[0] != 93 || len(report.Before.Applied) != 92 || len(report.After.Applied) != 93 || len(report.After.Pending) != 0 {
						t.Fatalf("V93 release/ledger failed: %v report=%#v", err, report)
					}
					var safe bool
					if err := admin.QueryRow(ctx, `SELECT
  (SELECT p.prosecdef AND p.proconfig @> ARRAY['search_path=pg_catalog'] AND r.rolname='espacigo_schema_owner'
   FROM pg_catalog.pg_proc p JOIN pg_catalog.pg_roles r ON r.oid=p.proowner
   WHERE p.oid='public.commit_verified_identity_decision(uuid,uuid,text,text,timestamp with time zone)'::regprocedure)
  AND pg_catalog.has_function_privilege('espacigo_runtime','public.commit_verified_identity_decision(uuid,uuid,text,text,timestamp with time zone)','EXECUTE')
  AND NOT pg_catalog.has_function_privilege('espacigo_privacy_cleanup_worker','public.commit_verified_identity_decision(uuid,uuid,text,text,timestamp with time zone)','EXECUTE')
  AND NOT EXISTS (SELECT 1 FROM pg_catalog.pg_proc p CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(p.proacl,pg_catalog.acldefault('f'::"char",p.proowner))) acl
                  WHERE p.oid='public.commit_verified_identity_decision(uuid,uuid,text,text,timestamp with time zone)'::regprocedure
                    AND acl.privilege_type='EXECUTE' AND acl.grantee NOT IN (p.proowner,'espacigo_runtime'::regrole))
  AND EXISTS (SELECT 1 FROM pg_catalog.pg_proc p CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(p.proacl,pg_catalog.acldefault('f'::"char",p.proowner))) acl
              WHERE p.oid='public.commit_verified_identity_decision(uuid,uuid,text,text,timestamp with time zone)'::regprocedure
                AND acl.grantee='espacigo_runtime'::regrole AND acl.privilege_type='EXECUTE')`).Scan(&safe); err != nil || !safe {
						t.Fatalf("V93 commit function owner/runtime-only ACL readback failed: %v", err)
					}
					var createGranted bool
					if err := admin.QueryRow(ctx, `SELECT pg_catalog.has_schema_privilege('espacigo_schema_owner','public','CREATE')`).Scan(&createGranted); err != nil || !createGranted {
						t.Fatalf("V93 caller CREATE fixture missing before revocation: %v", err)
					}
					mustExec("REVOKE CREATE ON SCHEMA public FROM espacigo_schema_owner")
					var createAfterRevoke bool
					if err := admin.QueryRow(ctx, `SELECT pg_catalog.has_schema_privilege('espacigo_schema_owner','public','CREATE')`).Scan(&createAfterRevoke); err != nil || createAfterRevoke {
						t.Fatalf("V93 temporary CREATE revocation readback failed: %v", err)
					}
				})
				t.Run("release-v93-v94", func(t *testing.T) {
					mustExec(`DO $fixture$
DECLARE owner_name text;
BEGIN
    SELECT pg_catalog.pg_get_userbyid(c.relowner) INTO owner_name
      FROM pg_catalog.pg_class c WHERE c.oid='public.outbox_evento_local'::regclass;
    IF owner_name <> 'postgres' THEN
        RAISE EXCEPTION 'synthetic V94 fixture expects native PostgreSQL outbox owner, got %', owner_name;
    END IF;
END
$fixture$;
GRANT SELECT(id,agregado_tipo,tipo,clave_deduplicacion,entregada_en,retirar_en,cancelada_en,fallo_terminal_en), DELETE ON public.outbox_evento_local TO espacigo_schema_owner;
GRANT UPDATE(id) ON public.outbox_evento_local TO espacigo_schema_owner;
GRANT CREATE ON SCHEMA public TO espacigo_schema_owner;`)
					var authorityReady bool
					if err := admin.QueryRow(ctx, `SELECT
						pg_catalog.has_table_privilege('espacigo_schema_owner','public.outbox_evento_local','DELETE')
						AND pg_catalog.has_column_privilege('espacigo_schema_owner','public.outbox_evento_local','id','UPDATE')
						AND (SELECT count(*)=8 FROM unnest(ARRAY['id','agregado_tipo','tipo','clave_deduplicacion','entregada_en','retirar_en','cancelada_en','fallo_terminal_en']) AS c(name)
						     WHERE pg_catalog.has_column_privilege('espacigo_schema_owner','public.outbox_evento_local',c.name,'SELECT'))`).Scan(&authorityReady); err != nil || !authorityReady {
						t.Fatalf("synthetic V94 explicit definer authority readback failed: %v", err)
					}
					t.Cleanup(func() {
						// The installed definer still needs its table/column privileges.
						// They disappear with this fixture's own database, not here.
						if _, err := admin.Exec(context.Background(), `REVOKE CREATE ON SCHEMA public FROM espacigo_schema_owner`); err != nil {
							t.Errorf("revoke synthetic V94 definer authority: %v", err)
						}
					})
					conn := connectTestDB(t, ctx, migrationURL.String())
					defer conn.Close(context.Background())
					report, err := runReleaseWithConn(ctx, conn, prefix(94), "espacigo_schema_owner", ReleaseContract{
						SourceSHA: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", InventorySHA256: inventoryFingerprint(all[:94]), ExpectedApplied: 93, Database: admin.Config().Database,
					})
					if err != nil || len(report.Applied) != 1 || report.Applied[0] != 94 || len(report.Before.Applied) != 93 || len(report.After.Applied) != 94 || len(report.After.Pending) != 0 {
						t.Fatalf("V94 release/ledger failed: %v report=%#v", err, report)
					}
					t.Run("v94-worker-only-bounded-retirement", func(t *testing.T) {
						var body string
						if err := admin.QueryRow(ctx, `SELECT pg_catalog.pg_get_functiondef('public.purge_delivered_identity_session_outbox()'::regprocedure)`).Scan(&body); err != nil || !strings.Contains(body, "LIMIT 100") || !strings.Contains(body, "SKIP LOCKED") || !strings.Contains(body, "RETURNS bigint") {
							t.Fatalf("V94 bounded function contract missing: %v", err)
						}
						var workerExecute, runtimeExecute, publicExecute, safeDefiner bool
						if err := admin.QueryRow(ctx, `SELECT
							pg_catalog.has_function_privilege('espacigo_privacy_cleanup_worker','public.purge_delivered_identity_session_outbox()','EXECUTE'),
							pg_catalog.has_function_privilege('espacigo_runtime','public.purge_delivered_identity_session_outbox()','EXECUTE'),
							EXISTS (SELECT 1 FROM pg_catalog.pg_proc p CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(p.proacl,pg_catalog.acldefault('f'::"char",p.proowner))) acl WHERE p.oid='public.purge_delivered_identity_session_outbox()'::regprocedure AND acl.grantee=0 AND acl.privilege_type='EXECUTE'),
							(SELECT p.prosecdef AND p.proconfig @> ARRAY['search_path=pg_catalog'] AND r.rolname='espacigo_schema_owner' FROM pg_catalog.pg_proc p JOIN pg_catalog.pg_roles r ON r.oid=p.proowner WHERE p.oid='public.purge_delivered_identity_session_outbox()'::regprocedure)`).Scan(&workerExecute, &runtimeExecute, &publicExecute, &safeDefiner); err != nil || !workerExecute || runtimeExecute || publicExecute || !safeDefiner {
							t.Fatalf("V94 definer/worker-only ACL failed: worker=%v runtime=%v public=%v definer=%v err=%v", workerExecute, runtimeExecute, publicExecute, safeDefiner, err)
						}
					})
				})
				t.Run("release-v94-v95", func(t *testing.T) {
					pendingTail, err := Plan(ctx, migrationURL.String(), prefix(96), "espacigo_schema_owner")
					if err != nil {
						t.Fatalf("read-only V95/V96 authority plan: %v", err)
					}
					if authority := pendingTail.IdentityDDLAuthority; authority == nil ||
						authority.SessionUser != "espacigo_migrator_login" || authority.CurrentUser != "espacigo_schema_owner" ||
						!authority.SessionRole.CanLogin || authority.SessionRole.Superuser || authority.PublicCreate ||
						authority.CaseOwner != "postgres" || authority.DeclarationOwner != "espacigo_schema_owner" ||
						authority.EventOwner != "postgres" || authority.PurgeOwner != "espacigo_schema_owner" {
						t.Fatalf("read-only V95/V96 plan authority does not reflect the fixture's exact blocked state: %#v", authority)
					}
					pendingWire, err := json.Marshal(pendingTail)
					if err != nil {
						t.Fatal(err)
					}
					var pendingReport map[string]json.RawMessage
					if err := json.Unmarshal(pendingWire, &pendingReport); err != nil {
						t.Fatal(err)
					}
					if len(pendingReport["identity_ddl_authority"]) == 0 {
						t.Fatal("read-only plan omitted the V95/V96 identity DDL authority report")
					}
					mustExec(`GRANT CREATE ON SCHEMA public TO espacigo_schema_owner`)
					t.Cleanup(func() {
						if _, err := admin.Exec(context.Background(), `REVOKE CREATE ON SCHEMA public FROM espacigo_schema_owner`); err != nil {
							t.Errorf("revoke synthetic V95 temporary CREATE: %v", err)
						}
					})
					t.Run("foreign-case-owner-blocks-v95", func(t *testing.T) {
						var caseOwner, declarationOwner string
						if err := admin.QueryRow(ctx, `SELECT
							(SELECT pg_catalog.pg_get_userbyid(relowner) FROM pg_catalog.pg_class WHERE oid='public.document_verification_case'::regclass),
							(SELECT pg_catalog.pg_get_userbyid(relowner) FROM pg_catalog.pg_class WHERE oid='public.identity_declaration_revision'::regclass)`).Scan(&caseOwner, &declarationOwner); err != nil {
							t.Fatal(err)
						}
						if caseOwner != "postgres" || declarationOwner != "espacigo_schema_owner" {
							t.Fatalf("synthetic pre-V95 owners=(case %q, declaration %q), want the observed native mismatch (postgres, espacigo_schema_owner)", caseOwner, declarationOwner)
						}
						conn := connectTestDB(t, ctx, migrationURL.String())
						defer conn.Close(context.Background())
						_, err := runReleaseWithConn(ctx, conn, prefix(95), "espacigo_schema_owner", ReleaseContract{
							SourceSHA: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", InventorySHA256: inventoryFingerprint(all[:95]), ExpectedApplied: 94, Database: admin.Config().Database,
						})
						if err == nil || !strings.Contains(err.Error(), "V95 requires schema-owned identity tables and purge function") {
							t.Fatalf("V95 did not reject the foreign-owned case table: %v", err)
						}
						history, err := readHistory(ctx, admin)
						if err != nil || len(history) != 94 || history[93].Version != 94 {
							t.Fatalf("V95 ownership rejection changed the V1..V94 ledger: %v", err)
						}
						var columnExists bool
						if err := admin.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_catalog.pg_attribute WHERE attrelid='public.document_verification_case'::regclass AND attname='identity_declaration_revision_id' AND attnum>0 AND NOT attisdropped)`).Scan(&columnExists); err != nil || columnExists {
							t.Fatalf("V95 ownership rejection left schema changes behind: %v", err)
						}
					})
					// Owner repair below is confined to this disposable PostgreSQL
					// fixture; it does not touch the persistent native target.
					mustExec(`ALTER TABLE public.document_verification_case OWNER TO espacigo_schema_owner`)
					v95Plan, err := Plan(ctx, migrationURL.String(), prefix(95), "espacigo_schema_owner")
					if err != nil {
						t.Fatalf("read-only V95 apply-time authority plan: %v", err)
					}
					captureIdentityTailAuthorityPlan(t, 95, v95Plan)
					conn := connectTestDB(t, ctx, migrationURL.String())
					defer conn.Close(context.Background())
					report, err := runReleaseWithConn(ctx, conn, prefix(95), "espacigo_schema_owner", ReleaseContract{
						SourceSHA: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", InventorySHA256: inventoryFingerprint(all[:95]), ExpectedApplied: 94, Database: admin.Config().Database,
					})
					if err != nil || len(report.Applied) != 1 || report.Applied[0] != 95 || len(report.Before.Applied) != 94 || len(report.After.Applied) != 95 || len(report.After.Pending) != 0 {
						t.Fatalf("V95 release/ledger failed: %v report=%#v", err, report)
					}
					t.Run("v95-identity-case-binding", func(t *testing.T) { assertV95IdentityCaseBindingContract(t, ctx, admin) })
				})
				t.Run("release-v95-v96", func(t *testing.T) {
					mustExec(`GRANT CREATE ON SCHEMA public TO espacigo_schema_owner`)
					t.Cleanup(func() {
						if _, err := admin.Exec(context.Background(), `REVOKE CREATE ON SCHEMA public FROM espacigo_schema_owner`); err != nil {
							t.Errorf("revoke synthetic V96 temporary CREATE: %v", err)
						}
					})
					t.Run("foreign-event-owner-blocks-v96", func(t *testing.T) {
						var caseOwner, eventOwner string
						if err := admin.QueryRow(ctx, `SELECT
							(SELECT pg_catalog.pg_get_userbyid(relowner) FROM pg_catalog.pg_class WHERE oid='public.document_verification_case'::regclass),
							(SELECT pg_catalog.pg_get_userbyid(relowner) FROM pg_catalog.pg_class WHERE oid='public.document_verification_event'::regclass)`).Scan(&caseOwner, &eventOwner); err != nil {
							t.Fatal(err)
						}
						if caseOwner != "espacigo_schema_owner" || eventOwner != "postgres" {
							t.Fatalf("synthetic pre-V96 owners=(case %q, event %q), want (espacigo_schema_owner, postgres)", caseOwner, eventOwner)
						}
						conn := connectTestDB(t, ctx, migrationURL.String())
						defer conn.Close(context.Background())
						_, err := runReleaseWithConn(ctx, conn, prefix(96), "espacigo_schema_owner", ReleaseContract{
							SourceSHA: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", InventorySHA256: inventoryFingerprint(all[:96]), ExpectedApplied: 95, Database: admin.Config().Database,
						})
						if err == nil || !strings.Contains(err.Error(), "V96 requires schema-owned verification tables") {
							t.Fatalf("V96 did not reject the foreign-owned event table: %v", err)
						}
						history, err := readHistory(ctx, admin)
						if err != nil || len(history) != 95 || history[94].Version != 95 {
							t.Fatalf("V96 ownership rejection changed the V1..V95 ledger: %v", err)
						}
						var columnExists bool
						if err := admin.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_catalog.pg_attribute WHERE attrelid='public.document_verification_event'::regclass AND attname='identity_review_observations' AND attnum>0 AND NOT attisdropped)`).Scan(&columnExists); err != nil || columnExists {
							t.Fatalf("V96 ownership rejection left schema changes behind: %v", err)
						}
					})
					// This owner repair is limited to the disposable integration fixture.
					mustExec(`ALTER TABLE public.document_verification_event OWNER TO espacigo_schema_owner`)
					v96Plan, err := Plan(ctx, migrationURL.String(), prefix(96), "espacigo_schema_owner")
					if err != nil {
						t.Fatalf("read-only V96 apply-time authority plan: %v", err)
					}
					captureIdentityTailAuthorityPlan(t, 96, v96Plan)
					conn := connectTestDB(t, ctx, migrationURL.String())
					defer conn.Close(context.Background())
					report, err := runReleaseWithConn(ctx, conn, prefix(96), "espacigo_schema_owner", ReleaseContract{
						SourceSHA: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", InventorySHA256: inventoryFingerprint(all[:96]), ExpectedApplied: 95, Database: admin.Config().Database,
					})
					if err != nil || len(report.Applied) != 1 || report.Applied[0] != 96 || len(report.Before.Applied) != 95 || len(report.After.Applied) != 96 || len(report.After.Pending) != 0 {
						t.Fatalf("V96 release/ledger failed: %v report=%#v", err, report)
					}
					t.Run("v96-identity-review-audit", func(t *testing.T) { assertV96IdentityReviewAuditContract(t, ctx, admin) })
				})
				t.Run("release-noop", func(t *testing.T) {
					conn := connectTestDB(t, ctx, migrationURL.String())
					defer conn.Close(context.Background())
					report, err := runReleaseWithConn(ctx, conn, prefix(96), "espacigo_schema_owner", ReleaseContract{
						SourceSHA: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", InventorySHA256: inventoryFingerprint(all[:96]), ExpectedApplied: 96, Database: admin.Config().Database,
					})
					if err != nil || len(report.Applied) != 0 || len(report.After.Applied) != 96 || len(report.After.Pending) != 0 {
						t.Fatalf("restricted V96 release no-op failed: %v", err)
					}
				})
				t.Run("catalog-read", func(t *testing.T) { assertPackagedCatalogRead(t, ctx, migrationURL.String()) })
				t.Run("api-schema-contract", func(t *testing.T) {
					assertRuntimeAPISchemaContract(t, ctx, databaseURL, admin)
				})
				t.Run("v67-inmuebles-seed", func(t *testing.T) {
					assertV67InmueblesSeed(t, ctx, admin)
				})
				t.Run("v85-identity-schema", func(t *testing.T) {
					assertV85IdentitySchema(t, ctx, databaseURL, admin)
				})
				t.Run("v88-runtime-execute-revoked", func(t *testing.T) {
					var runtimeCanExecute bool
					if err := admin.QueryRow(ctx, `SELECT pg_catalog.has_function_privilege('espacigo_runtime', 'public.purge_expired_identity_declaration_revisions()', 'EXECUTE')`).Scan(&runtimeCanExecute); err != nil {
						t.Fatal(err)
					}
					if runtimeCanExecute {
						t.Fatal("V87 leaves runtime EXECUTE on the identity purge function")
					}
				})

				var unsafe bool
				if err := admin.QueryRow(ctx, `SELECT has_schema_privilege('espacigo_schema_owner','public','CREATE') OR has_schema_privilege('espacigo_runtime','public','CREATE') OR (SELECT count(*)<>0 FROM public.usuario)`).Scan(&unsafe); err != nil || unsafe {
					t.Fatalf("unsafe final synthetic state: %v", err)
				}
			})
		})
	}
}

func assertV67InmueblesSeed(t *testing.T, ctx context.Context, db *pgx.Conn) {
	t.Helper()
	var code, name string
	var order int
	if err := db.QueryRow(ctx, `SELECT codigo,nombre,orden FROM public.categoria_espacio WHERE codigo='inmuebles'`).Scan(&code, &name, &order); err != nil {
		t.Fatalf("V67 inmuebles category is missing: %v", err)
	}
	if code != "inmuebles" || name != "Inmuebles" || order != 9 {
		t.Fatalf("V67 category seed = (%q,%q,%d), want (inmuebles,Inmuebles,9)", code, name, order)
	}
	type attribute struct {
		Code            string   `json:"code"`
		Order           int      `json:"order"`
		Type            string   `json:"type"`
		Required        bool     `json:"required"`
		Unit            string   `json:"unit"`
		FilterCandidate bool     `json:"filter_candidate"`
		Options         []string `json:"options"`
		Minimum         *float64 `json:"minimum"`
		Maximum         *float64 `json:"maximum"`
		Step            *float64 `json:"step"`
		Description     string   `json:"description"`
	}
	var profile struct {
		CategoryCode  string      `json:"category_code"`
		SchemaVersion int         `json:"schema_version"`
		Attributes    []attribute `json:"attributes"`
	}
	var raw []byte
	if err := db.QueryRow(ctx, `SELECT perfil FROM public.categoria_perfil_atributos WHERE categoria_codigo='inmuebles' AND version=1`).Scan(&raw); err != nil {
		t.Fatalf("V67 inmuebles profile v1 is missing: %v", err)
	}
	if err := json.Unmarshal(raw, &profile); err != nil {
		t.Fatalf("decode V67 inmuebles profile: %v", err)
	}
	wantCodes := []string{
		"tipo_inmueble", "tipo_de_espacio", "superficie_construida_m2", "superficie_terreno_m2",
		"dormitorios", "camas", "banos_completos", "medios_banos", "privacidad_bano", "cocina",
		"amoblado", "estacionamientos", "bodegas", "piso", "pisos_edificio", "ascensor", "balcon",
		"terraza", "patio_o_jardin", "piscina", "wifi", "calefaccion", "aire_acondicionado",
		"agua_caliente", "lavadora", "secadora", "refrigerador", "servicios_incluidos", "mascotas",
		"visitas", "fumadores", "fiestas_o_reuniones", "duracion_minima_unidades", "duracion_maxima_unidades",
	}
	if profile.CategoryCode != "inmuebles" || profile.SchemaVersion != 1 || len(profile.Attributes) != len(wantCodes) {
		t.Fatalf("V67 profile header/attribute count = (%q,%d,%d), want (inmuebles,1,%d)", profile.CategoryCode, profile.SchemaVersion, len(profile.Attributes), len(wantCodes))
	}
	byCode := make(map[string]attribute, len(profile.Attributes))
	for i, a := range profile.Attributes {
		if a.Code != wantCodes[i] || a.Order != i+1 {
			t.Fatalf("V67 attribute[%d] = (%q,order %d), want (%q,order %d)", i, a.Code, a.Order, wantCodes[i], i+1)
		}
		byCode[a.Code] = a
	}
	if a := byCode["tipo_inmueble"]; a.Type != "enum" || !a.Required || !a.FilterCandidate || len(a.Options) != 8 {
		t.Fatalf("V67 required property type definition is incomplete: %+v", a)
	}
	if a := byCode["tipo_de_espacio"]; a.Type != "enum" || !a.Required || !a.FilterCandidate || len(a.Options) != 4 {
		t.Fatalf("V67 rental-scope definition is incomplete: %+v", a)
	}
	if a := byCode["superficie_construida_m2"]; a.Type != "number" || a.Unit != "m²" || a.Minimum == nil || *a.Minimum != 0.01 || a.Maximum == nil || *a.Maximum != 99999999.99 || a.Step == nil || *a.Step != 0.01 || !a.FilterCandidate {
		t.Fatalf("V67 constructed-area bounds are incomplete: %+v", a)
	}
	for _, code := range []string{"duracion_minima_unidades", "duracion_maxima_unidades"} {
		a := byCode[code]
		if a.Type != "integer" || a.Minimum == nil || *a.Minimum != 1 || a.Description != "Se expresa en la unidad de la tarifa base: horas, días o meses." {
			t.Fatalf("V67 stay-duration definition %s is incomplete: %+v", code, a)
		}
	}
}

func TestCurrentSchemaMigrationV19WeeklyHours(t *testing.T) {
	if os.Getenv("TEST_MIGRATOR_CI_ISOLATED") != "1" {
		t.Skip("requires the disposable migration CI harness")
	}
	u, err := url.Parse(os.Getenv("TEST_DATABASE_URL"))
	if err != nil || u.Query().Get("host") == "" {
		t.Fatal("disposable CI requires its private Unix socket")
	}
	if _, err := os.Stat(filepath.Join(u.Query().Get("host"), ".espacigo-migration-ci")); err != nil {
		t.Fatal("disposable CI marker is missing")
	}
	all, err := DiscoverMigrations(filepath.Join("..", "..", "db", "migrations"))
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 96 {
		t.Fatal("update the reviewed migration inventory before running the V19-prefix test")
	}
	v19Dir := t.TempDir()
	for _, migration := range all[:19] {
		if err := os.WriteFile(filepath.Join(v19Dir, migration.Name), migration.SQL, 0600); err != nil {
			t.Fatal(err)
		}
	}
	withTestDatabase(t, func(ctx context.Context, databaseURL string) {
		if _, err := Run(ctx, databaseURL, v19Dir); err != nil {
			t.Fatalf("apply disposable migration prefix V1-V19: %v", err)
		}
		db := connectTestDB(t, ctx, databaseURL)
		defer db.Close(context.Background())
		mustExec := func(t *testing.T, sql string, args ...any) {
			t.Helper()
			if _, err := db.Exec(ctx, sql, args...); err != nil {
				t.Fatalf("execute V19 fixture SQL: %v", err)
			}
		}
		assertPgError := func(t *testing.T, name, sql, code, constraint string, args ...any) {
			t.Helper()
			_, err := db.Exec(ctx, sql, args...)
			if err == nil {
				t.Fatalf("%s unexpectedly succeeded", name)
			}
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != code || (constraint != "" && pgErr.ConstraintName != constraint) {
				t.Fatalf("%s error = %v, want SQLSTATE %s constraint %q", name, err, code, constraint)
			}
		}
		ownerID := "00000000-0000-0000-0000-000000000019"
		renterID := "00000000-0000-0000-0000-000000000020"
		mustExec(t, `INSERT INTO public.usuario(id,correo_original,correo_normalizado,hash_clave)
VALUES($1,'host@example.test','host@example.test','synthetic-test-hash'),
       ($2,'renter@example.test','renter@example.test','synthetic-test-hash')`, ownerID, renterID)
		makeSpace := func(t *testing.T, id, title string) {
			t.Helper()
			mustExec(t, `INSERT INTO public.espacio(id,propietario_id,categoria_codigo,titulo,descripcion,
superficie_m2,capacidad_maxima,reglas_uso,modalidad_tarifa,precio_base_clp,direccion,zona_horaria)
VALUES($1,$2,'oficina',$3,$4,20,2,'uso sintético','hora',10000,'dirección sintética','UTC')`,
				id, ownerID, title, strings.Repeat("x", 110))
			mustExec(t, `INSERT INTO public.tarifa_espacio(espacio_id,version,modalidad,precio_base_clp)
SELECT id,1,modalidad_tarifa,precio_base_clp FROM public.espacio WHERE id=$1`, id)
		}
		makeFixture := func(t *testing.T, spaceID string) {
			t.Helper()
			mustExec(t, `INSERT INTO public.reserva_ensayo_local_fixture(espacio_id,anfitrion_id,arrendatario_id,habilitada)
VALUES($1,$2,$3,true)`, spaceID, ownerID, renterID)
		}
		makeSchedule := func(t *testing.T, spaceID string) {
			t.Helper()
			mustExec(t, `INSERT INTO public.espacio_horario_semanal(espacio_id,activo) VALUES($1,false)`, spaceID)
		}
		withoutFixture := "00000000-0000-0000-0000-000000000191"
		validFixture := "00000000-0000-0000-0000-000000000192"
		nonHourlyFixture := "00000000-0000-0000-0000-000000000193"
		makeSpace(t, withoutFixture, "V19 sin fixture")
		makeSchedule(t, withoutFixture)
		t.Run("fixture-required", func(t *testing.T) {
			assertPgError(t, "activating schedule without fixture",
				`UPDATE public.espacio_horario_semanal SET activo=true WHERE espacio_id=$1`,
				"23514", "horario_semanal_fixture_hora_ck", withoutFixture)
		})
		makeSpace(t, validFixture, "V19 fixture horario")
		makeFixture(t, validFixture)
		makeSchedule(t, validFixture)
		mustExec(t, `INSERT INTO public.espacio_horario_semanal_tramo(espacio_id,dia_iso,orden,apertura_minuto,cierre_minuto)
VALUES($1,1,1,540,720)`, validFixture)
		t.Run("interval-day-check", func(t *testing.T) {
			assertPgError(t, "out-of-range ISO day",
				`INSERT INTO public.espacio_horario_semanal_tramo(espacio_id,dia_iso,orden,apertura_minuto,cierre_minuto) VALUES($1,8,2,780,840)`,
				"23514", "", validFixture)
		})
		t.Run("interval-positive-duration-check", func(t *testing.T) {
			assertPgError(t, "non-positive interval",
				`INSERT INTO public.espacio_horario_semanal_tramo(espacio_id,dia_iso,orden,apertura_minuto,cierre_minuto) VALUES($1,2,1,780,780)`,
				"23514", "", validFixture)
		})
		t.Run("interval-overlap-exclusion", func(t *testing.T) {
			assertPgError(t, "overlapping intervals",
				`INSERT INTO public.espacio_horario_semanal_tramo(espacio_id,dia_iso,orden,apertura_minuto,cierre_minuto) VALUES($1,1,2,660,780)`,
				"23P01", "", validFixture)
		})
		t.Run("disabled-fixture-rejected", func(t *testing.T) {
			mustExec(t, `UPDATE public.reserva_ensayo_local_fixture SET habilitada=false WHERE espacio_id=$1`, validFixture)
			assertPgError(t, "activating schedule with disabled fixture",
				`UPDATE public.espacio_horario_semanal SET activo=true WHERE espacio_id=$1`,
				"23514", "horario_semanal_fixture_hora_ck", validFixture)
			mustExec(t, `UPDATE public.reserva_ensayo_local_fixture SET habilitada=true WHERE espacio_id=$1`, validFixture)
		})
		makeSpace(t, nonHourlyFixture, "V19 tarifa no horaria")
		makeFixture(t, nonHourlyFixture)
		mustExec(t, `INSERT INTO public.tarifa_espacio(espacio_id,version,modalidad,precio_base_clp) VALUES($1,2,'dia',15000)`, nonHourlyFixture)
		makeSchedule(t, nonHourlyFixture)
		t.Run("latest-tariff-must-be-hourly", func(t *testing.T) {
			assertPgError(t, "activating schedule with latest daily tariff",
				`UPDATE public.espacio_horario_semanal SET activo=true WHERE espacio_id=$1`,
				"23514", "horario_semanal_fixture_hora_ck", nonHourlyFixture)
		})
		t.Run("valid-fixture-enables-hourly-schedule", func(t *testing.T) {
			mustExec(t, `UPDATE public.espacio_horario_semanal SET activo=true WHERE espacio_id=$1`, validFixture)
			var active bool
			if err := db.QueryRow(ctx, `SELECT activo FROM public.espacio_horario_semanal WHERE espacio_id=$1`, validFixture).Scan(&active); err != nil || !active {
				t.Fatalf("valid hourly fixture did not activate schedule: %v", err)
			}
		})
		t.Run("zone-change-blocked", func(t *testing.T) {
			assertPgError(t, "changing zone with active weekly schedule",
				`UPDATE public.espacio SET zona_horaria='America/Santiago' WHERE id=$1`,
				"23514", "zona_horaria_horario_semanal_activo", validFixture)
		})
		t.Run("space-mode-change-blocked", func(t *testing.T) {
			assertPgError(t, "changing space rate mode with active weekly schedule",
				`UPDATE public.espacio SET modalidad_tarifa='dia' WHERE id=$1`,
				"23514", "modalidad_horario_semanal_activo_ck", validFixture)
		})
		t.Run("tariff-mode-change-blocked", func(t *testing.T) {
			assertPgError(t, "inserting daily tariff with active weekly schedule",
				`INSERT INTO public.tarifa_espacio(espacio_id,version,modalidad,precio_base_clp) VALUES($1,2,'dia',15000)`,
				"23514", "tarifa_horario_semanal_activo_ck", validFixture)
		})
	})
}
