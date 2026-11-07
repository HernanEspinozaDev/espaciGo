package migrator

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5"
)

// This is a reproducible synthetic bootstrap, not an operational cloud repair.
// V81 deliberately requires administrator-provisioned roles; never relax that
// guard or change an applied migration to make an empty-cluster test pass.
func TestCurrentSchemaMigrationFromEmptyAndV75(t *testing.T) {
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
	if len(all) != 84 {
		t.Fatal("update the reviewed current-schema bootstrap recipe for the new inventory")
	}
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
	for _, baseline := range []int{0, 75} {
		t.Run(map[int]string{0: "empty", 75: "upgrade-v75"}[baseline], func(t *testing.T) {
			withTestDatabase(t, func(ctx context.Context, databaseURL string) {
				admin := connectTestDB(t, ctx, databaseURL)
				defer admin.Close(context.Background())
				mustExec := func(sql string) {
					t.Helper()
					if _, err := admin.Exec(ctx, sql); err != nil {
						t.Fatal(err)
					}
				}
				mustExec(`CREATE ROLE espacigo_runtime NOLOGIN NOINHERIT;
CREATE ROLE espacigo_api_login LOGIN NOINHERIT;
CREATE ROLE espacigo_migrator_login LOGIN NOINHERIT;
GRANT espacigo_runtime TO espacigo_api_login WITH ADMIN FALSE, INHERIT FALSE, SET TRUE;`)
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
				if _, err := Run(ctx, databaseURL, prefix(80)); err != nil {
					t.Fatal(err)
				}
				mustExec(`CREATE ROLE espacigo_schema_owner NOLOGIN NOINHERIT;
CREATE ROLE espacigo_document_verification_fn NOLOGIN NOINHERIT;
CREATE SCHEMA document_verification_private AUTHORIZATION espacigo_document_verification_fn;
GRANT USAGE ON SCHEMA public TO espacigo_document_verification_fn, espacigo_schema_owner;
GRANT SELECT(id,estado), UPDATE(id) ON public.usuario TO espacigo_document_verification_fn;
GRANT SELECT(id,propietario_id), UPDATE(id) ON public.espacio TO espacigo_document_verification_fn;
GRANT espacigo_schema_owner TO espacigo_migrator_login WITH ADMIN FALSE, INHERIT FALSE, SET TRUE;
GRANT espacigo_document_verification_fn TO espacigo_schema_owner WITH ADMIN FALSE, INHERIT FALSE, SET TRUE;
GRANT SELECT,INSERT ON public.schema_migrations TO espacigo_schema_owner;`)
				mustExec("REVOKE ALL ON DATABASE " + pgx.Identifier{admin.Config().Database}.Sanitize() + " FROM PUBLIC")
				mustExec("GRANT CONNECT ON DATABASE " + pgx.Identifier{admin.Config().Database}.Sanitize() + " TO espacigo_migrator_login, espacigo_api_login")
				migrationURL, _ := url.Parse(databaseURL)
				migrationURL.User = url.User("espacigo_migrator_login")
				if _, err := RunWithRole(ctx, migrationURL.String(), prefix(81), "espacigo_schema_owner"); err != nil {
					t.Fatal(err)
				}
				// The existing V82/V83 require a bounded ownership/grant window.
				mustExec(`GRANT CREATE ON SCHEMA public TO espacigo_schema_owner;
ALTER FUNCTION public.document_verification_space_owner_changed() OWNER TO espacigo_schema_owner;
GRANT UPDATE(id) ON public.document_verification_reviewer_grant TO espacigo_schema_owner WITH GRANT OPTION;`)
				if _, err := RunWithRole(ctx, migrationURL.String(), prefix(83), "espacigo_schema_owner"); err != nil {
					t.Fatal(err)
				}
				mustExec(`ALTER FUNCTION public.document_verification_space_owner_changed() OWNER TO postgres;
REVOKE UPDATE(id) ON public.document_verification_reviewer_grant FROM espacigo_schema_owner CASCADE;
GRANT UPDATE(id) ON public.document_verification_reviewer_grant TO espacigo_runtime;
REVOKE CREATE ON SCHEMA public FROM espacigo_schema_owner;`)
				// V84 expands identity verification schema; requires CREATE on public schema and REFERENCES on usuario.
				mustExec(`GRANT CREATE ON SCHEMA public TO espacigo_schema_owner;
GRANT REFERENCES ON public.usuario TO espacigo_schema_owner;`)
				if _, err := RunWithRole(ctx, migrationURL.String(), prefix(84), "espacigo_schema_owner"); err != nil {
					t.Fatal(err)
				}
				mustExec(`REVOKE CREATE ON SCHEMA public FROM espacigo_schema_owner;
REVOKE REFERENCES ON public.usuario FROM espacigo_schema_owner;`)
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
				t.Run("release-noop", func(t *testing.T) {
					conn := connectTestDB(t, ctx, migrationURL.String())
					defer conn.Close(context.Background())
					report, err := runReleaseWithConn(ctx, conn, prefix(84), "espacigo_schema_owner", ReleaseContract{
						SourceSHA: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", InventorySHA256: inventoryFingerprint(all), ExpectedApplied: 84, Database: admin.Config().Database,
					})
					if err != nil || len(report.Applied) != 0 || len(report.After.Applied) != 84 || len(report.After.Pending) != 0 {
						t.Fatalf("restricted V84 release no-op failed: %v", err)
					}
				})
				t.Run("catalog-read", func(t *testing.T) { assertPackagedCatalogRead(t, ctx, migrationURL.String()) })
				t.Run("api-schema-contract", func(t *testing.T) {
					assertRuntimeAPISchemaContract(t, ctx, databaseURL, admin)
				})
				var unsafe bool
				if err := admin.QueryRow(ctx, `SELECT has_schema_privilege('espacigo_schema_owner','public','CREATE') OR has_schema_privilege('espacigo_runtime','public','CREATE') OR (SELECT count(*)<>0 FROM public.usuario)`).Scan(&unsafe); err != nil || unsafe {
					t.Fatalf("unsafe final synthetic state: %v", err)
				}
			})
			// withTestDatabase has dropped only its own DB before roles are removed.
			admin := connectTestDB(t, context.Background(), os.Getenv("TEST_DATABASE_URL"))
			defer admin.Close(context.Background())
			if _, err := admin.Exec(context.Background(), `DROP ROLE espacigo_migrator_login, espacigo_api_login, espacigo_schema_owner, espacigo_document_verification_fn, espacigo_runtime`); err != nil {
				t.Fatal(err)
			}
		})
	}
}
