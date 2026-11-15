package migrator

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

func assertV85RejectsLegacyData(t *testing.T, ctx context.Context, admin *pgx.Conn, migrationURL, dir string) {
	t.Helper()
	_, err := admin.Exec(ctx, `INSERT INTO public.usuario(id,correo_original,correo_normalizado,hash_clave,estado) VALUES ('85000000-0000-4000-8000-000000000090','v85-legacy@example.invalid','v85-legacy@example.invalid','synthetic-not-used','activo');
 INSERT INTO public.identity_verification_attempts(id,account_id,declared_first_name,declared_last_name,declared_date_of_birth,declared_rut_normalized,consent_given_at,consent_version) VALUES ('85000000-0000-4000-8000-000000000091','85000000-0000-4000-8000-000000000090','Synthetic','Fixture','1990-01-01','11222333-9',now(),'synthetic-v1')`)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := admin.Exec(context.Background(), `DELETE FROM public.identity_verification_attempts WHERE id='85000000-0000-4000-8000-000000000091'; DELETE FROM public.usuario WHERE id='85000000-0000-4000-8000-000000000090'`); err != nil {
			t.Errorf("cleanup own V85 legacy fixture: %v", err)
		}
	}()
	if _, err := RunWithRole(ctx, migrationURL, dir, "espacigo_schema_owner"); err == nil || !strings.Contains(strings.ToLower(err.Error()), "legacy") {
		t.Fatalf("V85 legacy guard missing or unrelated failure: %v", err)
	}
	var version int
	var intact, noNewTables bool
	if err := admin.QueryRow(ctx, `SELECT (SELECT max(version) FROM public.schema_migrations),EXISTS(SELECT 1 FROM public.identity_verification_attempts WHERE id='85000000-0000-4000-8000-000000000091'),to_regclass('public.identity_declaration_revision') IS NULL AND to_regclass('public.identity_provider_attempt') IS NULL AND to_regclass('public.identity_provider_event') IS NULL`).Scan(&version, &intact, &noNewTables); err != nil || version != 84 || !intact || !noNewTables {
		t.Fatalf("V85 rejection mutated legacy/schema/ledger: %d %t %t %v", version, intact, noNewTables, err)
	}
}
