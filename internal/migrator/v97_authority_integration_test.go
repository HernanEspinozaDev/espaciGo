package migrator

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5"
)

// assertV97AuthorityLifecycle runs at the V96-to-V97 combined-fixture boundary,
// whose target is the marked, isolated Unix-socket migration harness database.
func assertV97AuthorityLifecycle(t *testing.T, ctx context.Context, admin *pgx.Conn, targetURL, migrationURL string, migrations []Migration) {
	t.Helper()
	if len(migrations) != 97 || inventoryFingerprint(migrations) != v97AuthorityInventory {
		t.Fatal("V97 authority integration requires the exact V1..V97 inventory")
	}
	dir := t.TempDir()
	for _, migration := range migrations {
		if err := os.WriteFile(filepath.Join(dir, migration.Name), migration.SQL, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	inspect, err := manageV97AuthorityWithConn(ctx, admin, dir, "inspect", "")
	if err != nil || inspect.Format != "espacigo.v97-authority.v1" || inspect.AppliedCount != 96 || !inspect.AuthorityClosed || !validHex(inspect.BaselineSHA256, 64) {
		t.Fatalf("inspect V96 V97-authority boundary: report=%+v err=%v", inspect, err)
	}
	if err := verifyV97TemporaryPrivileges(ctx, admin, false); err != nil {
		t.Fatalf("V97 privileges must initially be absent: %v", err)
	}

	t.Run("failure_cleanup", func(t *testing.T) {
		prepared, err := manageV97AuthorityWithConn(ctx, admin, dir, "prepare", "")
		if err != nil || prepared.AppliedCount != 96 || prepared.AuthorityClosed || prepared.BaselineSHA256 != inspect.BaselineSHA256 {
			t.Fatalf("prepare V97 failure-cleanup fixture: report=%+v err=%v", prepared, err)
		}
		if err := verifyV97TemporaryPrivileges(ctx, admin, true); err != nil {
			t.Fatal(err)
		}
		if _, err := manageV97AuthorityWithConn(ctx, admin, dir, "finalize", stringsOfZeroHash()); err == nil {
			t.Fatal("wrong-baseline finalization unexpectedly succeeded")
		}
		if err := verifyV97TemporaryPrivileges(ctx, admin, false); err != nil {
			t.Fatalf("failed V97 migration cleanup left temporary privileges: %v", err)
		}
		var count int
		if err := admin.QueryRow(ctx, `SELECT count(*) FROM public.schema_migrations WHERE version=97`).Scan(&count); err != nil || count != 0 {
			t.Fatalf("failure cleanup changed V97 ledger: count=%d err=%v", count, err)
		}
		var absent bool
		if err := admin.QueryRow(ctx, `SELECT to_regclass('public.cotizacion_arriendo_comercial') IS NULL`).Scan(&absent); err != nil || !absent {
			t.Fatalf("failure cleanup left V97 table: absent=%t err=%v", absent, err)
		}
	})
	t.Run("migration_failure_cleanup", func(t *testing.T) {
		prepared, err := manageV97AuthorityWithConn(ctx, admin, dir, "prepare", "")
		if err != nil || prepared.BaselineSHA256 != inspect.BaselineSHA256 {
			t.Fatalf("prepare migration-failure fixture: report=%+v err=%v", prepared, err)
		}
		if _, err := admin.Exec(ctx, v97RevokeCreateSQL); err != nil {
			t.Fatalf("inject a DDL privilege failure: %v", err)
		}
		if _, err := RunWithRole(ctx, migrationURL, dir, "espacigo_schema_owner"); err == nil {
			t.Fatal("V97 migration unexpectedly succeeded without CREATE on public")
		}
		finalized, err := manageV97AuthorityWithConn(ctx, admin, dir, "finalize", inspect.BaselineSHA256)
		if err != nil || !finalized.AuthorityClosed || finalized.AppliedCount != 96 {
			t.Fatalf("finalize after migration failure: report=%+v err=%v", finalized, err)
		}
		if err := verifyV97TemporaryPrivileges(ctx, admin, false); err != nil {
			t.Fatalf("migration failure cleanup left V97 grants: %v", err)
		}
	})

	prepared, err := manageV97AuthorityWithConn(ctx, admin, dir, "prepare", "")
	if err != nil || prepared.AppliedCount != 96 || prepared.AuthorityClosed || prepared.BaselineSHA256 != inspect.BaselineSHA256 {
		t.Fatalf("prepare V97 migration: report=%+v err=%v", prepared, err)
	}
	if err := verifyV97TemporaryPrivileges(ctx, admin, true); err != nil {
		t.Fatal(err)
	}
	result, err := RunWithRole(ctx, migrationURL, dir, "espacigo_schema_owner")
	if err != nil || len(result.Applied) != 1 || result.Applied[0] != 97 {
		t.Fatalf("run original V97 as dedicated migrator: applied=%v err=%v", result.Applied, err)
	}
	t.Run("rejects_public_table_grant", func(t *testing.T) {
		if _, err := admin.Exec(ctx, `GRANT SELECT ON public.cotizacion_arriendo_comercial TO PUBLIC`); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if _, err := admin.Exec(context.Background(), `REVOKE SELECT ON public.cotizacion_arriendo_comercial FROM PUBLIC`); err != nil {
				t.Errorf("remove synthetic PUBLIC grant: %v", err)
			}
		})
		if _, err := manageV97AuthorityWithConn(ctx, admin, dir, "finalize", inspect.BaselineSHA256); err == nil {
			t.Fatal("V97 finalization accepted a PUBLIC table grant")
		}
		if err := verifyV97TemporaryPrivileges(ctx, admin, false); err != nil {
			t.Fatalf("failed V97 table-ACL verification left temporary grants: %v", err)
		}
	})
	t.Run("rejects_public_column_grant", func(t *testing.T) {
		if _, err := admin.Exec(ctx, `GRANT UPDATE (titulo_espacio) ON public.cotizacion_arriendo_comercial TO PUBLIC`); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if _, err := admin.Exec(context.Background(), `REVOKE UPDATE (titulo_espacio) ON public.cotizacion_arriendo_comercial FROM PUBLIC`); err != nil {
				t.Errorf("remove synthetic PUBLIC column grant: %v", err)
			}
		})
		if _, err := manageV97AuthorityWithConn(ctx, admin, dir, "finalize", inspect.BaselineSHA256); err == nil {
			t.Fatal("V97 finalization accepted a PUBLIC column grant")
		}
		if err := verifyV97TemporaryPrivileges(ctx, admin, false); err != nil {
			t.Fatalf("failed V97 column-ACL verification left temporary grants: %v", err)
		}
	})
	t.Run("rejects_other_table_grantee", func(t *testing.T) {
		if _, err := admin.Exec(ctx, `CREATE ROLE v97_authority_acl_probe NOLOGIN`); err != nil {
			t.Fatal(err)
		}
		if _, err := admin.Exec(ctx, `GRANT SELECT ON public.cotizacion_arriendo_comercial TO v97_authority_acl_probe`); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if _, err := admin.Exec(context.Background(), `REVOKE ALL PRIVILEGES ON public.cotizacion_arriendo_comercial FROM v97_authority_acl_probe`); err != nil {
				t.Errorf("remove synthetic role grant: %v", err)
			}
			if _, err := admin.Exec(context.Background(), `DROP ROLE v97_authority_acl_probe`); err != nil {
				t.Errorf("remove synthetic ACL probe role: %v", err)
			}
		})
		if _, err := manageV97AuthorityWithConn(ctx, admin, dir, "finalize", inspect.BaselineSHA256); err == nil {
			t.Fatal("V97 finalization accepted a table grant to another role")
		}
		if err := verifyV97TemporaryPrivileges(ctx, admin, false); err != nil {
			t.Fatalf("failed V97 other-role ACL verification left temporary grants: %v", err)
		}
	})
	t.Run("rejects_runtime_membership_drift", func(t *testing.T) {
		if _, err := admin.Exec(ctx, `CREATE ROLE v97_authority_membership_probe NOLOGIN`); err != nil {
			t.Fatal(err)
		}
		if _, err := admin.Exec(ctx, `GRANT v97_authority_membership_probe TO espacigo_runtime WITH ADMIN FALSE, INHERIT FALSE, SET TRUE`); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if _, err := admin.Exec(context.Background(), `REVOKE v97_authority_membership_probe FROM espacigo_runtime`); err != nil {
				t.Errorf("remove synthetic runtime membership: %v", err)
			}
			if _, err := admin.Exec(context.Background(), `DROP ROLE v97_authority_membership_probe`); err != nil {
				t.Errorf("remove synthetic membership probe role: %v", err)
			}
		})
		if _, err := manageV97AuthorityWithConn(ctx, admin, dir, "finalize", inspect.BaselineSHA256); err == nil {
			t.Fatal("V97 finalization accepted runtime membership drift")
		}
		if err := verifyV97TemporaryPrivileges(ctx, admin, false); err != nil {
			t.Fatalf("failed V97 membership verification left temporary grants: %v", err)
		}
	})
	t.Run("rejects_runtime_grant_option", func(t *testing.T) {
		if err := setV97RuntimeSelectGrantOption(ctx, admin, true); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := setV97RuntimeSelectGrantOption(context.Background(), admin, false); err != nil {
				t.Errorf("remove synthetic runtime grant option: %v", err)
			}
		})
		if _, err := manageV97AuthorityWithConn(ctx, admin, dir, "finalize", inspect.BaselineSHA256); err == nil {
			t.Fatal("V97 finalization accepted a runtime grant option")
		}
		if err := verifyV97TemporaryPrivileges(ctx, admin, false); err != nil {
			t.Fatalf("failed V97 grant-option verification left temporary grants: %v", err)
		}
	})
	final, err := manageV97AuthorityWithConn(ctx, admin, dir, "finalize", inspect.BaselineSHA256)
	if err != nil || final.AppliedCount != 97 || !final.AuthorityClosed || final.BaselineSHA256 != inspect.BaselineSHA256 {
		t.Fatalf("finalize V97 authority: report=%+v err=%v", final, err)
	}
	if err := verifyV97TemporaryPrivileges(ctx, admin, false); err != nil {
		t.Fatalf("V97 temporary grants remain after finalize: %v", err)
	}

	plan, err := Plan(ctx, targetURL, dir, "espacigo_schema_owner")
	if err != nil || !plan.ReadOnly || len(plan.Applied) != 97 || len(plan.Pending) != 0 {
		t.Fatalf("independent V97 ledger postcheck: plan=%+v err=%v", plan, err)
	}
	assertV97CommercialQuoteSchema(t, ctx, admin)
	if err := verifyV97RuntimePrivileges(ctx, admin); err != nil {
		t.Fatal(err)
	}
	var noRows bool
	if err := admin.QueryRow(ctx, `SELECT NOT EXISTS(SELECT 1 FROM public.usuario) AND NOT EXISTS(SELECT 1 FROM public.espacio) AND NOT EXISTS(SELECT 1 FROM public.cotizacion_arriendo_comercial)`).Scan(&noRows); err != nil || !noRows {
		t.Fatalf("V97 authority fixture must contain no business rows: empty=%t err=%v", noRows, err)
	}
}

func setV97RuntimeSelectGrantOption(ctx context.Context, conn *pgx.Conn, enabled bool) error {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, `SET LOCAL ROLE espacigo_schema_owner`); err != nil {
		return err
	}
	statement := `REVOKE GRANT OPTION FOR SELECT ON public.cotizacion_arriendo_comercial FROM espacigo_runtime`
	if enabled {
		statement = `GRANT SELECT ON public.cotizacion_arriendo_comercial TO espacigo_runtime WITH GRANT OPTION`
	}
	if _, err := tx.Exec(ctx, statement); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func assertV97CommercialQuoteSchema(t *testing.T, ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}) {
	t.Helper()
	var tableOwner string
	if err := q.QueryRow(ctx, `SELECT pg_catalog.pg_get_userbyid(c.relowner) FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relname='cotizacion_arriendo_comercial'`).Scan(&tableOwner); err != nil || tableOwner != "espacigo_schema_owner" {
		t.Fatalf("V97 quote table owner=%q err=%v", tableOwner, err)
	}
	var foreignKeys, checks, expiryIndex bool
	const inspect = `SELECT
 (SELECT count(*)=3 AND bool_and(confdeltype='c') FROM pg_catalog.pg_constraint WHERE conrelid='public.cotizacion_arriendo_comercial'::regclass AND contype='f'),
 (SELECT count(*)=7 AND bool_and(convalidated) FROM pg_catalog.pg_constraint WHERE conrelid='public.cotizacion_arriendo_comercial'::regclass AND contype='c'),
 EXISTS(SELECT 1 FROM pg_catalog.pg_index i JOIN pg_catalog.pg_class c ON c.oid=i.indexrelid JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relname='cotizacion_arriendo_comercial_expira_idx' AND i.indisvalid)`
	if err := q.QueryRow(ctx, inspect).Scan(&foreignKeys, &checks, &expiryIndex); err != nil || !foreignKeys || !checks || !expiryIndex {
		t.Fatalf("V97 quote schema contract: foreign_keys=%t checks=%t expiry_index=%t err=%v", foreignKeys, checks, expiryIndex, err)
	}
}

func stringsOfZeroHash() string {
	return "0000000000000000000000000000000000000000000000000000000000000000"
}
