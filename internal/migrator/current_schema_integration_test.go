package migrator

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"testing"
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
	if len(all) != 97 {
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
				migrationURL, before := establishRentalCombinedFixture(t, ctx, databaseURL, admin, all, baseline)
				t.Run("v97-upgrade", func(t *testing.T) {
					assertV97AuthorityLifecycle(t, ctx, admin, databaseURL, migrationURL, all)
				})
				plan, err := Plan(ctx, migrationURL, prefix(97), "espacigo_schema_owner")
				if err != nil {
					t.Fatal(err)
				}
				if !plan.ReadOnly || len(plan.Applied) != 97 || len(plan.Pending) != 0 {
					t.Fatalf("incomplete postcheck: %#v", plan)
				}
				for i, entry := range before {
					if plan.Applied[i] != entry {
						t.Fatal("upgrade rewrote the existing migration ledger")
					}
				}
				result, err := RunWithRole(ctx, migrationURL, prefix(97), "espacigo_schema_owner")
				if err != nil || len(result.Applied) != 0 {
					t.Fatalf("reapply was not a no-op: %v", err)
				}
				t.Run("release-noop", func(t *testing.T) {
					conn := connectTestDB(t, ctx, migrationURL)
					defer conn.Close(context.Background())
					report, err := runReleaseWithConn(ctx, conn, prefix(97), "espacigo_schema_owner", ReleaseContract{
						SourceSHA: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", InventorySHA256: inventoryFingerprint(all), ExpectedApplied: 97, Database: admin.Config().Database,
					})
					if err != nil || len(report.Applied) != 0 || len(report.After.Applied) != 97 || len(report.After.Pending) != 0 {
						t.Fatalf("restricted V97 release no-op failed: %v", err)
					}
				})
				t.Run("catalog-read", func(t *testing.T) { assertPackagedCatalogRead(t, ctx, migrationURL) })
				t.Run("api-schema-contract", func(t *testing.T) {
					assertRuntimeAPISchemaContract(t, ctx, databaseURL, admin)
				})
				var unsafe bool
				if err := admin.QueryRow(ctx, `SELECT has_schema_privilege('espacigo_schema_owner','public','CREATE') OR has_schema_privilege('espacigo_runtime','public','CREATE') OR (SELECT count(*)<>0 FROM public.usuario)`).Scan(&unsafe); err != nil || unsafe {
					t.Fatalf("unsafe final synthetic state: %v", err)
				}
				t.Run("commercial-quote-suppression", func(t *testing.T) {
					assertCommercialQuoteSuppression(t, ctx, databaseURL, admin)
				})
				t.Run("commercial-quote-persistence", func(t *testing.T) {
					assertCommercialQuotePersistenceWithExistingRuntimeACL(t, ctx, databaseURL, admin)
				})
				var legacyReadable bool
				if err := admin.QueryRow(ctx, `SELECT has_table_privilege('espacigo_runtime','public.identity_verification_attempts','SELECT')`).Scan(&legacyReadable); err != nil || legacyReadable {
					t.Fatalf("current-schema behaviors reopened legacy identity access: readable=%t err=%v", legacyReadable, err)
				}
			})
		})
	}
}
