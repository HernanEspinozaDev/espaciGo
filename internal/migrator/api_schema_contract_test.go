package migrator

import (
	"context"
	"errors"
	"net/url"
	"testing"

	documentpg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/documentverification"
	identitypg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/identity"
	catalogpg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/publiccatalog"
	"github.com/HernanEspinozaDev/espaciGo/internal/dbbootstrap"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
	"github.com/HernanEspinozaDev/espaciGo/internal/identityverification"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Called only within the isolated, empty-account full-schema harness. Real
// repositories exercise old and new SQL projections under the actual runtime
// role; this is not an HTTP E2E or a claim about every previous API image.
func assertRuntimeAPISchemaContract(t *testing.T, ctx context.Context, adminURL string, admin *pgx.Conn) {
	t.Helper()
	if err := dbbootstrap.GrantRuntimePermissions(ctx, admin); err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(adminURL)
	if err != nil {
		t.Fatal(err)
	}
	u.User = url.User("espacigo_api_login")
	cfg, err := pgxpool.ParseConfig(u.String())
	if err != nil {
		t.Fatal(err)
	}
	cfg.MaxConns = 1
	cfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, "SET ROLE espacigo_runtime")
		return err
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	const absent = "10000000-0000-4000-8000-000000000001"
	if _, err := identitypg.NewIdentityRepository(pool).AccountByID(ctx, absent); !errors.Is(err, identity.ErrNotFound) {
		t.Fatalf("legacy identity projection is incompatible with current schema/runtime: %v", err)
	}
	categories, err := catalogpg.New(pool).Categories(ctx)
	if err != nil || len(categories) < 8 {
		t.Fatalf("public catalogue projection is incompatible with current schema/runtime: %v", err)
	}
	if _, err := documentpg.New(pool).CurrentForAccount(ctx, absent); !errors.Is(err, identityverification.ErrNotFound) {
		t.Fatalf("current document projection is incompatible with schema/runtime: %v", err)
	}
	const lockQuery = `SELECT reviewer_id::text,capability,scope_id::text FROM public.document_verification_reviewer_grant WHERE reviewer_id=$1 AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at>clock_timestamp()) ORDER BY capability,scope_id FOR SHARE`
	if _, err := pool.Exec(ctx, lockQuery, absent); err != nil {
		t.Fatalf("V83 runtime row-lock contract is missing: %v", err)
	}
	if _, err := admin.Exec(ctx, "REVOKE UPDATE(id) ON public.document_verification_reviewer_grant FROM espacigo_runtime"); err != nil {
		t.Fatal(err)
	}
	_, rejected := pool.Exec(ctx, lockQuery, absent)
	// Restore only the fixture privilege even when the negative assertion fails.
	if _, err := admin.Exec(ctx, "GRANT UPDATE(id) ON public.document_verification_reviewer_grant TO espacigo_runtime"); err != nil {
		t.Fatal(err)
	}
	var permissionError *pgconn.PgError
	if !errors.As(rejected, &permissionError) || permissionError.Code != "42501" {
		t.Fatalf("schema/API contract did not catch the missing column-level lock privilege: %v", rejected)
	}
	var tableUpdate, ledgerSelect, createSchema bool
	if err := pool.QueryRow(ctx, `SELECT has_table_privilege(current_user,'public.document_verification_reviewer_grant','UPDATE'),has_table_privilege(current_user,'public.schema_migrations','SELECT'),has_schema_privilege(current_user,'public','CREATE')`).Scan(&tableUpdate, &ledgerSelect, &createSchema); err != nil || tableUpdate || ledgerSelect || createSchema {
		t.Fatalf("runtime gained migration or generic reviewer privileges: %v", err)
	}
}
