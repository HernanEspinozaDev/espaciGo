package migrator

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

	"github.com/HernanEspinozaDev/espaciGo/internal/dbbootstrap"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const rentalCombinedFixtureMarker = ".espacigo-migration-ci"

// establishRentalCombinedFixture applies the exact discovered V1..V96 SQL to
// the marked disposable test database. admin is used only for synthetic role,
// ownership, and narrowly scoped setup-authority transitions; all ledger writes
// are performed by Run/RunWithRole. The caller owns the database lifecycle.
func establishRentalCombinedFixture(t *testing.T, ctx context.Context, targetURL string, admin *pgx.Conn, migrations []Migration, baseline int) (string, []HistoryEntry) {
	t.Helper()
	if baseline != 0 && baseline != 75 {
		t.Fatal("combined fixture baseline must be empty or exact V75")
	}
	if os.Getenv("TEST_MIGRATOR_CI_ISOLATED") != "1" {
		t.Fatal("combined fixture requires TEST_MIGRATOR_CI_ISOLATED=1")
	}
	u, err := url.Parse(targetURL)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || !filepath.IsAbs(u.Query().Get("host")) || !strings.HasPrefix(u.Path, "/migrator_test_") {
		t.Fatal("combined fixture requires a private Unix-socket target")
	}
	selected, err := url.Parse(os.Getenv("TEST_DATABASE_URL"))
	if err != nil || selected.Query().Get("host") != u.Query().Get("host") {
		t.Fatal("combined fixture socket differs from the selected disposable harness")
	}
	if _, err := os.Stat(filepath.Join(u.Query().Get("host"), rentalCombinedFixtureMarker)); err != nil {
		t.Fatal("combined fixture disposable harness marker is missing")
	}
	if admin == nil || admin.Config() == nil || admin.Config().Database != strings.TrimPrefix(u.Path, "/") || admin.Config().Host != u.Query().Get("host") || admin.Config().Port != 5432 || admin.Config().User != "postgres" {
		t.Fatal("combined fixture administrator connection does not match selected database")
	}
	var database, sessionUser, currentUser string
	if err := admin.QueryRow(ctx, `SELECT current_database(),session_user,current_user`).Scan(&database, &sessionUser, &currentUser); err != nil || database != admin.Config().Database || sessionUser != "postgres" || currentUser != "postgres" {
		t.Fatalf("combined fixture effective target identity mismatch: %v", err)
	}
	if len(migrations) != 97 {
		t.Fatalf("combined fixture requires the complete exact V1..V97 inventory, got %d", len(migrations))
	}
	for i, m := range migrations {
		if m.Version != int64(i+1) {
			t.Fatalf("combined fixture migration inventory is not contiguous at index %d", i)
		}
	}
	packaged, err := DiscoverMigrations(filepath.Join("..", "..", "db", "migrations"))
	if err != nil || len(packaged) != len(migrations) {
		t.Fatalf("read combined fixture packaged source: %v", err)
	}
	for i, m := range migrations {
		if m.Name != packaged[i].Name || m.Checksum != packaged[i].Checksum || sha256Hex(m.SQL) != m.Checksum {
			t.Fatalf("combined fixture source drift at V%d", m.Version)
		}
	}
	var existing int
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM pg_catalog.pg_roles WHERE rolname = ANY($1::text[])`, []string{
		"espacigo_runtime", "espacigo_api_login", "espacigo_migrator_login", "espacigo_schema_owner",
		"espacigo_document_verification_fn", "espacigo_privacy_cleanup_worker", "espacigo_privacy_worker_login",
	}).Scan(&existing); err != nil {
		t.Fatalf("check disposable fixture role-name collisions: %v", err)
	}
	if existing != 0 {
		t.Fatalf("refusing to reuse %d pre-existing combined-fixture task roles", existing)
	}

	// Capture the base database target now: cleanup runs after withTestDatabase
	// drops the task database, and roles are cluster-scoped.
	cleanupURL := selected.String()
	ownedRoles := []fixtureRoleReceipt(nil)
	t.Cleanup(func() {
		if len(ownedRoles) == 0 {
			return
		}
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		conn, err := pgx.Connect(cleanupCtx, cleanupURL)
		if err != nil {
			t.Errorf("connect for owned fixture role cleanup: %v", err)
			return
		}
		defer conn.Close(context.Background())
		var observed []fixtureRoleReceipt
		rows, e := conn.Query(cleanupCtx, `SELECT rolname, oid::bigint FROM pg_catalog.pg_roles WHERE rolname = ANY($1::text[])`, fixtureRoleNames())
		if e != nil {
			t.Errorf("inspect owned combined-fixture roles: %v", e)
			return
		}
		for rows.Next() {
			var r fixtureRoleReceipt
			if e := rows.Scan(&r.name, &r.oid); e != nil {
				rows.Close()
				t.Errorf("read owned role receipt: %v", e)
				return
			}
			observed = append(observed, r)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			t.Errorf("read owned role inventory: %v", e)
			return
		}
		if e := validateFixtureRoleReceipts(ownedRoles, observed); e != nil {
			t.Errorf("refusing unsafe combined-fixture role cleanup: %v", e)
			return
		}
		tx, e := conn.Begin(cleanupCtx)
		if e != nil {
			t.Errorf("begin owned role cleanup: %v", e)
			return
		}
		for _, r := range ownedRoles {
			if _, e = tx.Exec(cleanupCtx, "DROP ROLE "+pgx.Identifier{r.name}.Sanitize()); e != nil {
				_ = tx.Rollback(cleanupCtx)
				t.Errorf("drop owned combined-fixture role %s: %v", r.name, e)
				return
			}
		}
		if e = tx.Commit(cleanupCtx); e != nil {
			t.Errorf("commit owned role cleanup: %v", e)
			return
		}
		var remaining int
		if e = conn.QueryRow(cleanupCtx, `SELECT count(*) FROM pg_catalog.pg_roles WHERE rolname = ANY($1::text[])`, fixtureRoleNames()).Scan(&remaining); e != nil || remaining != 0 {
			t.Errorf("owned fixture role cleanup readback: remaining=%d err=%v", remaining, e)
		}
	})

	prefix := func(last int) string {
		t.Helper()
		dir := t.TempDir()
		for _, m := range migrations[:last] {
			if e := os.WriteFile(filepath.Join(dir, m.Name), m.SQL, 0o600); e != nil {
				t.Fatal(e)
			}
		}
		return dir
	}
	tx, err := admin.Begin(ctx)
	if err != nil {
		t.Fatalf("begin atomic fixture role creation: %v", err)
	}
	if _, err := tx.Exec(ctx, `CREATE ROLE espacigo_runtime NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT;
CREATE ROLE espacigo_api_login LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT;
CREATE ROLE espacigo_migrator_login LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT;
CREATE ROLE espacigo_schema_owner NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT;
CREATE ROLE espacigo_document_verification_fn NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT;
CREATE ROLE espacigo_privacy_cleanup_worker NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT;
CREATE ROLE espacigo_privacy_worker_login LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT;

GRANT espacigo_runtime TO espacigo_api_login WITH ADMIN FALSE, INHERIT FALSE, SET TRUE;`); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatalf("create initial isolated fixture roles: %v", err)
	}
	for _, name := range fixtureRoleNames() {
		var receipt fixtureRoleReceipt
		if err := tx.QueryRow(ctx, `SELECT rolname, oid::bigint FROM pg_catalog.pg_roles WHERE rolname=$1`, name).Scan(&receipt.name, &receipt.oid); err != nil {
			_ = tx.Rollback(ctx)
			t.Fatalf("capture created role OID: %v", err)
		}
		ownedRoles = append(ownedRoles, receipt)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit atomic fixture role creation (cleanup retains captured receipts): %v", err)
	}
	var before []HistoryEntry
	if baseline == 75 {
		if _, err := Run(ctx, targetURL, prefix(75)); err != nil {
			t.Fatalf("establish exact V75 upgrade baseline: %v", err)
		}
		before, err = readHistory(ctx, admin)
		if err != nil || len(before) != baseline {
			t.Fatalf("capture V75 upgrade baseline: count=%d err=%v", len(before), err)
		}
	}
	prefixResult, err := Run(ctx, targetURL, prefix(80))
	if err != nil {
		t.Fatalf("apply exact V1..V80 source SQL: %v", err)
	}
	if len(prefixResult.Applied) != 80-baseline {
		t.Fatalf("V80 continuation applied unexpected prefix: baseline=%d applied=%v", baseline, prefixResult.Applied)
	}
	for i, version := range prefixResult.Applied {
		if version != int64(baseline+i+1) {
			t.Fatalf("V80 continuation replayed or skipped a version: baseline=%d applied=%v", baseline, prefixResult.Applied)
		}
	}
	if _, err := admin.Exec(ctx, `CREATE SCHEMA document_verification_private AUTHORIZATION espacigo_document_verification_fn;
GRANT USAGE ON SCHEMA public TO espacigo_document_verification_fn, espacigo_schema_owner;
GRANT SELECT(id,estado), UPDATE(id) ON public.usuario TO espacigo_document_verification_fn;
GRANT SELECT(id,propietario_id), UPDATE(id) ON public.espacio TO espacigo_document_verification_fn;
GRANT espacigo_schema_owner TO espacigo_migrator_login WITH ADMIN FALSE, INHERIT FALSE, SET TRUE;
GRANT espacigo_document_verification_fn TO espacigo_schema_owner WITH ADMIN FALSE, INHERIT FALSE, SET TRUE;
GRANT SELECT, INSERT ON public.schema_migrations TO espacigo_schema_owner;
GRANT CONNECT ON DATABASE `+pgx.Identifier{admin.Config().Database}.Sanitize()+` TO espacigo_migrator_login, espacigo_api_login`); err != nil {
		t.Fatalf("create initial isolated fixture roles: %v", err)
	}
	migrationURL := *u
	migrationURL.User = url.User("espacigo_migrator_login")
	wire := func(version int) {
		t.Helper()
		if !t.Run("authority-wire-v"+strconv.Itoa(version), func(t *testing.T) {
			assertPostgresAuthorityPythonWire(t, ctx, migrationURL.String(), admin.Config().Database, migrations, version)
		}) {
			t.Fatal("authority wire boundary failed; refusing subsequent fixture SQL")
		}
	}

	if _, err := RunWithRole(ctx, migrationURL.String(), prefix(81), "espacigo_schema_owner"); err != nil {
		t.Fatalf("apply exact V81 SQL: %v", err)
	}
	// Synthetic counterpart to the historical V82/V83 authority window. No
	// native repair template or native target authority helper is used.
	if _, err := admin.Exec(ctx, `GRANT CREATE ON SCHEMA public TO espacigo_schema_owner;
ALTER FUNCTION public.document_verification_space_owner_changed() OWNER TO espacigo_schema_owner;
GRANT UPDATE(id) ON public.document_verification_reviewer_grant TO espacigo_schema_owner WITH GRANT OPTION`); err != nil {
		t.Fatal(err)
	}
	if _, err := RunWithRole(ctx, migrationURL.String(), prefix(83), "espacigo_schema_owner"); err != nil {
		t.Fatalf("apply exact V82..V83 SQL: %v", err)
	}
	// Install the maintained baseline before identity tightening. Replaying this
	// bootstrap after V85 would conceal revocation/compatibility regressions.
	if err := dbbootstrap.GrantRuntimePermissions(ctx, admin); err != nil {
		t.Fatalf("prepare source-defined pre-identity runtime ACL: %v", err)
	}
	if _, err := admin.Exec(ctx, `ALTER FUNCTION public.document_verification_space_owner_changed() OWNER TO postgres;
REVOKE UPDATE(id) ON public.document_verification_reviewer_grant FROM espacigo_schema_owner CASCADE;
GRANT UPDATE(id) ON public.document_verification_reviewer_grant TO espacigo_runtime;
REVOKE CREATE ON SCHEMA public FROM espacigo_schema_owner;
GRANT CREATE ON SCHEMA public TO espacigo_schema_owner;
GRANT REFERENCES(id) ON TABLE public.usuario TO espacigo_schema_owner`); err != nil {
		t.Fatal(err)
	}
	wire(84)
	if _, err := RunWithRole(ctx, migrationURL.String(), prefix(84), "espacigo_schema_owner"); err != nil {
		t.Fatalf("apply exact V84 SQL: %v", err)
	}
	if _, err := admin.Exec(ctx, `REVOKE CREATE ON SCHEMA public FROM espacigo_schema_owner; REVOKE REFERENCES(id) ON TABLE public.usuario FROM espacigo_schema_owner`); err != nil {
		t.Fatal(err)
	}
	var create, references bool
	if err := admin.QueryRow(ctx, `SELECT has_schema_privilege('espacigo_schema_owner','public','CREATE'),has_column_privilege('espacigo_schema_owner','public.usuario','id','REFERENCES')`).Scan(&create, &references); err != nil || create || references {
		t.Fatalf("V84 synthetic authority did not close: create=%t references=%t err=%v", create, references, err)
	}
	if _, err := admin.Exec(ctx, `GRANT CREATE ON SCHEMA public TO espacigo_schema_owner; GRANT REFERENCES(id) ON TABLE public.usuario TO espacigo_schema_owner`); err != nil {
		t.Fatal(err)
	}
	wire(85)
	if _, err := RunWithRole(ctx, migrationURL.String(), prefix(85), "espacigo_schema_owner"); err != nil {
		t.Fatalf("apply exact V85 identity SQL: %v", err)
	}
	if _, err := admin.Exec(ctx, `REVOKE CREATE ON SCHEMA public FROM espacigo_schema_owner;
REVOKE REFERENCES(id) ON TABLE public.usuario FROM espacigo_schema_owner`); err != nil {
		t.Fatal(err)
	}

	// Exact source prefixes are applied separately; each CREATE window is
	// closed immediately after the version that requires it.
	apply := func(version int, setup, teardown string) {
		t.Helper()
		if setup != "" {
			if _, e := admin.Exec(ctx, setup); e != nil {
				t.Fatalf("prepare V%d synthetic authority: %v", version, e)
			}
		}
		switch version {
		case 86, 87, 88, 95, 96:
			wire(version)
		}
		if _, e := RunWithRole(ctx, migrationURL.String(), prefix(version), "espacigo_schema_owner"); e != nil {
			t.Fatalf("apply exact V%d SQL: %v", version, e)
		}
		if teardown != "" {
			if _, e := admin.Exec(ctx, teardown); e != nil {
				t.Fatalf("close V%d synthetic authority: %v", version, e)
			}
		}
	}
	apply(86, `GRANT CREATE ON SCHEMA public TO espacigo_schema_owner`, `REVOKE CREATE ON SCHEMA public FROM espacigo_schema_owner`)
	if _, err := admin.Exec(ctx, `GRANT espacigo_privacy_cleanup_worker TO espacigo_privacy_worker_login WITH ADMIN FALSE, INHERIT FALSE, SET TRUE`); err != nil {
		t.Fatal(err)
	}
	apply(87, "", "")
	apply(88, `GRANT CREATE ON SCHEMA public TO espacigo_schema_owner`, `REVOKE CREATE ON SCHEMA public FROM espacigo_schema_owner`)
	apply(89, `GRANT CREATE ON SCHEMA public TO espacigo_schema_owner; GRANT REFERENCES(id) ON public.usuario TO espacigo_schema_owner`, `REVOKE CREATE ON SCHEMA public FROM espacigo_schema_owner; REVOKE REFERENCES(id) ON public.usuario FROM espacigo_schema_owner`)
	apply(90, `GRANT CREATE ON SCHEMA public TO espacigo_schema_owner`, `REVOKE CREATE ON SCHEMA public FROM espacigo_schema_owner`)
	apply(91, "", "")
	apply(92, `GRANT CREATE ON SCHEMA public TO espacigo_schema_owner`, `REVOKE CREATE ON SCHEMA public FROM espacigo_schema_owner`)
	apply(93, `GRANT CREATE ON SCHEMA public TO espacigo_schema_owner`, `REVOKE CREATE ON SCHEMA public FROM espacigo_schema_owner`)
	// V94 requires a disposable synthetic outbox ownership/privilege model.
	apply(94, `ALTER TABLE public.outbox_evento_local OWNER TO postgres;
GRANT SELECT(id,agregado_tipo,tipo,clave_deduplicacion,entregada_en,retirar_en,cancelada_en,fallo_terminal_en), DELETE ON public.outbox_evento_local TO espacigo_schema_owner;
GRANT UPDATE(id) ON public.outbox_evento_local TO espacigo_schema_owner;
GRANT CREATE ON SCHEMA public TO espacigo_schema_owner`, `REVOKE CREATE ON SCHEMA public FROM espacigo_schema_owner`)
	apply(95, `GRANT CREATE ON SCHEMA public TO espacigo_schema_owner; ALTER TABLE public.document_verification_case OWNER TO espacigo_schema_owner`, `REVOKE CREATE ON SCHEMA public FROM espacigo_schema_owner`)
	apply(96, `GRANT CREATE ON SCHEMA public TO espacigo_schema_owner; ALTER TABLE public.document_verification_event OWNER TO espacigo_schema_owner`, `REVOKE CREATE ON SCHEMA public FROM espacigo_schema_owner`)
	if _, err := RunWithRole(ctx, migrationURL.String(), prefix(96), "espacigo_schema_owner"); err != nil {
		t.Fatalf("V1..V96 exact-source postcheck: %v", err)
	}
	return migrationURL.String(), before
}

func TestRentalCombinedFixtureOptIn(t *testing.T) {
	if os.Getenv("TEST_MIGRATOR_CI_ISOLATED") != "1" {
		t.Skip("requires the disposable migration CI harness; opt in explicitly")
	}
	u, err := url.Parse(os.Getenv("TEST_DATABASE_URL"))
	if err != nil || u.Query().Get("host") == "" {
		t.Fatal("disposable CI requires its private Unix socket")
	}
	if _, err := os.Stat(filepath.Join(u.Query().Get("host"), rentalCombinedFixtureMarker)); err != nil {
		t.Fatal("disposable CI marker is missing")
	}
	all, err := DiscoverMigrations(filepath.Join("..", "..", "db", "migrations"))
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 97 {
		t.Fatalf("expected complete V1..V97 inventory, got %d", len(all))
	}
	withTestDatabase(t, func(ctx context.Context, databaseURL string) {
		admin := connectTestDB(t, ctx, databaseURL)
		defer admin.Close(context.Background())
		migrationURL, _ := establishRentalCombinedFixture(t, ctx, databaseURL, admin, all, 0)
		plan, err := Plan(ctx, migrationURL, prefixMigrationDirForCombined(t, all, 97), "espacigo_schema_owner")
		if err != nil || !plan.ReadOnly || len(plan.Applied) != 96 || len(plan.Pending) != 1 || plan.Pending[0].Version != 97 {
			t.Fatalf("combined V1..V96 pending-V97 postcheck: plan=%+v err=%v", plan, err)
		}
		t.Run("v97-without-authority", func(t *testing.T) {
			result, err := RunWithRole(ctx, migrationURL, prefixMigrationDirForCombined(t, all, 97), "espacigo_schema_owner")
			var pgErr *pgconn.PgError
			if err == nil || !errors.As(err, &pgErr) || pgErr.Code != "42501" || len(result.Applied) != 0 {
				t.Fatalf("V97 must reject missing DDL authority without applying: result=%+v err=%v", result, err)
			}
			var count int
			var absent bool
			if err := admin.QueryRow(ctx, `SELECT count(*),to_regclass('public.cotizacion_arriendo_comercial') IS NULL FROM public.schema_migrations`).Scan(&count, &absent); err != nil || count != 96 || !absent {
				t.Fatalf("V97 rejection changed schema or ledger: count=%d absent=%t err=%v", count, absent, err)
			}
		})
		t.Run("v97-upgrade", func(t *testing.T) {
			assertV97AuthorityLifecycle(t, ctx, admin, databaseURL, migrationURL, all)
		})
		var create, userRef, spaceRef, legacyReadable bool
		if err := admin.QueryRow(ctx, `SELECT has_schema_privilege('espacigo_schema_owner','public','CREATE'),has_column_privilege('espacigo_schema_owner','public.usuario','id','REFERENCES'),has_column_privilege('espacigo_schema_owner','public.espacio','id','REFERENCES'),has_table_privilege('espacigo_runtime','public.identity_verification_attempts','SELECT')`).Scan(&create, &userRef, &spaceRef, &legacyReadable); err != nil || create || userRef || spaceRef || legacyReadable {
			t.Fatalf("V97 temporary authority or identity legacy access left open: create=%t userRef=%t spaceRef=%t legacy=%t err=%v", create, userRef, spaceRef, legacyReadable, err)
		}
		post, err := Plan(ctx, migrationURL, prefixMigrationDirForCombined(t, all, 97), "espacigo_schema_owner")
		if err != nil || len(post.Applied) != 97 || len(post.Pending) != 0 {
			t.Fatalf("V97 ledger postcheck failed: %+v err=%v", post, err)
		}
		t.Run("commercial-quote-persistence", func(t *testing.T) {
			assertCommercialQuotePersistenceWithExistingRuntimeACL(t, ctx, databaseURL, admin)
		})
		t.Run("contract-identity-reader", func(t *testing.T) {
			assertContractIdentityReaderPostgres(t, ctx, databaseURL, admin)
			if err := admin.QueryRow(ctx, `SELECT has_table_privilege('espacigo_runtime','public.identity_verification_attempts','SELECT')`).Scan(&legacyReadable); err != nil || legacyReadable {
				t.Fatalf("identity reader fixture reopened legacy identity access: readable=%t err=%v", legacyReadable, err)
			}
		})
		if err := admin.QueryRow(ctx, `SELECT has_table_privilege('espacigo_runtime','public.identity_verification_attempts','SELECT')`).Scan(&legacyReadable); err != nil || legacyReadable {
			t.Fatalf("commercial fixture reopened legacy identity access: readable=%t err=%v", legacyReadable, err)
		}
	})
}

func prefixMigrationDirForCombined(t *testing.T, migrations []Migration, last int) string {
	t.Helper()
	dir := t.TempDir()
	for _, m := range migrations[:last] {
		if err := os.WriteFile(filepath.Join(dir, m.Name), m.SQL, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}
