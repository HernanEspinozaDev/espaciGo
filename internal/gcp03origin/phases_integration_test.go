package gcp03origin

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// TestOriginBootstrapAndObserverPostgres18 exercises the complete bootstrap
// transaction and observer report against the isolated fixture launched by
// scripts/test-gcp03-origin-postgres.sh. The local postgres superuser is only
// fixture control; it does not model Cloud SQL's cloudsqlsuperuser role.
func TestOriginBootstrapAndObserverPostgres18(t *testing.T) {
	connectionURL := os.Getenv("GCP03_ORIGIN_TEST_DATABASE_URL")
	if connectionURL == "" {
		t.Skip("use scripts/test-gcp03-origin-postgres.sh for the isolated PostgreSQL 18/PostGIS integration")
	}
	parsed, err := url.Parse(connectionURL)
	if err != nil || parsed.Scheme != "postgres" || parsed.Hostname() != "localhost" || parsed.User == nil || parsed.User.Username() != AdminLogin || parsed.Path != "/"+DatabaseName {
		t.Fatal("integration URL must use the disposable localhost postgres login and exact scratch database")
	}
	query, err := url.ParseQuery(parsed.RawQuery)
	socketDir := query.Get("host")
	if err != nil || len(query) != 2 || query.Get("sslmode") != "disable" || !filepath.IsAbs(socketDir) || !strings.HasPrefix(socketDir, "/tmp/espacigo-gcp03-origin.") {
		t.Fatal("integration URL must use the isolated Unix socket and sslmode=disable")
	}
	if _, err := os.Stat(filepath.Join(socketDir, ".gcp03-origin-isolated-test")); err != nil {
		t.Fatal("integration socket is missing its disposable fixture marker")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	admin, err := pgx.Connect(ctx, connectionURL)
	if err != nil {
		t.Fatalf("connect to isolated PostgreSQL endpoint: %v", err)
	}
	defer admin.Close(context.Background())
	var major int
	if err := admin.QueryRow(ctx, `SELECT current_setting('server_version_num')::int / 10000`).Scan(&major); err != nil || major != 18 {
		t.Fatalf("integration requires PostgreSQL 18, got %d (%v)", major, err)
	}
	var db, login string
	if err := admin.QueryRow(ctx, `SELECT current_database(),current_user`).Scan(&db, &login); err != nil || db != DatabaseName || login != AdminLogin {
		t.Fatalf("integration target mismatch: database=%q login=%q err=%v", db, login, err)
	}
	var initialExtensions int
	var hasPlpgsql bool
	if err := admin.QueryRow(ctx, `SELECT count(*), bool_and(extname='plpgsql' AND extversion='1.0') FROM pg_catalog.pg_extension`).Scan(&initialExtensions, &hasPlpgsql); err != nil || initialExtensions != 1 || !hasPlpgsql {
		t.Fatalf("fixture must begin with only plpgsql 1.0 (count=%d, valid=%t, err=%v)", initialExtensions, hasPlpgsql, err)
	}
	commitConn, err := pgx.Connect(ctx, connectionURL)
	if err != nil {
		t.Fatal("open isolated connection for commit failure injection", err)
	}
	commitReport, commitErr := bootstrapOnConnectionWithCommit(ctx, commitConn, func(context.Context, pgx.Tx) error {
		return errors.New("injected commit acknowledgment failure")
	})
	if commitErr == nil || commitReport.Passed || commitReport.TransactionOutcome != "outcome_uncertain" {
		t.Fatalf("commit failure must not produce successful evidence: report=%+v err=%v", commitReport, commitErr)
	}
	if err := commitConn.Close(context.Background()); err != nil {
		t.Fatal("close transaction after injected uncertain commit", err)
	}
	assertExtensionsAndRoles(t, ctx, admin, 1, nil)

	// An unexpected baseline is rejected before any bootstrap DDL is committed.
	if _, err := admin.Exec(ctx, `CREATE TABLE public.gcp03_unexpected_baseline (id integer)`); err != nil {
		t.Fatal("create baseline rejection marker")
	}
	baselineReport, baselineErr := bootstrapOnConnection(ctx, admin)
	if baselineErr == nil || !strings.Contains(baselineErr.Error(), "pristine database") || baselineReport.TransactionOutcome != "rolled_back" || baselineReport.Passed {
		t.Fatalf("unexpected baseline was not rejected and rolled back: report=%+v err=%v", baselineReport, baselineErr)
	}
	assertExtensionsAndRoles(t, ctx, admin, 1, nil)
	if _, err := admin.Exec(ctx, `DROP TABLE public.gcp03_unexpected_baseline`); err != nil {
		t.Fatal("remove baseline rejection marker")
	}

	// Cause a failure after extension DDL and two role creations. The complete
	// transaction must roll back those changes while preserving the marker role.
	observerRole := pgx.Identifier{"espacigo_gcp03_observer"}.Sanitize()
	if _, err := admin.Exec(ctx, `CREATE ROLE `+observerRole+` NOLOGIN NOSUPERUSER`); err != nil {
		t.Fatal("create rollback marker role")
	}
	ddlReport, ddlErr := bootstrapOnConnection(ctx, admin)
	if ddlErr == nil || ddlReport.TransactionOutcome != "rolled_back" || ddlReport.Passed {
		t.Fatalf("forced in-transaction failure did not roll back: report=%+v err=%v", ddlReport, ddlErr)
	}
	assertExtensionsAndRoles(t, ctx, admin, 1, []string{"espacigo_gcp03_observer"})
	if _, err := admin.Exec(ctx, `DROP ROLE `+observerRole); err != nil {
		t.Fatal("remove rollback marker role")
	}

	// Omitting the PUBLIC TEMP ACL correction leaves the original default
	// privilege in effect. The same report used by production must reject that
	// state before commit, and rolling back must remove every extension/role.
	incompleteStatements := make([]string, 0, len(bootstrapStatements)-1)
	for _, statement := range bootstrapStatements {
		if statement == `REVOKE TEMPORARY ON DATABASE espacigo_scratch FROM PUBLIC` {
			continue
		}
		incompleteStatements = append(incompleteStatements, statement)
	}
	postconditionReport, postconditionErr := bootstrapOnConnectionWithStatements(ctx, admin, incompleteStatements)
	if postconditionErr == nil || postconditionReport.Passed || postconditionReport.ObserverNoWrite || postconditionReport.TransactionOutcome != "rolled_back" {
		t.Fatalf("PUBLIC TEMP postcondition was not rejected and rolled back: report=%+v err=%v", postconditionReport, postconditionErr)
	}
	assertExtensionsAndRoles(t, ctx, admin, 1, nil)

	report, err := bootstrapOnConnection(ctx, admin)
	if err != nil {
		t.Fatalf("bootstrap pristine PostgreSQL 18 fixture: %v (report=%+v)", err, report)
	}
	if !report.Passed || report.TransactionOutcome != "committed" || !report.ObserverConnect || !report.ObserverPublicUsage || !report.ObserverNoWrite || report.ApplicationSchemas != 0 || report.ApplicationRelations != 0 || report.ApplicationFunctions != 0 {
		t.Fatalf("bootstrap report did not describe the committed baseline: %+v", report)
	}
	var publicTemp, observerTemp bool
	if err := admin.QueryRow(ctx, `SELECT
		EXISTS (SELECT 1 FROM pg_catalog.pg_database d CROSS JOIN LATERAL aclexplode(COALESCE(d.datacl, acldefault('d',d.datdba))) a WHERE d.datname=current_database() AND a.grantee=0 AND a.privilege_type='TEMPORARY'),
		has_database_privilege('espacigo_gcp03_observer',current_database(),'TEMP')`).Scan(&publicTemp, &observerTemp); err != nil {
		t.Fatal("inspect effective database TEMP privileges", err)
	}
	if publicTemp || observerTemp {
		t.Fatalf("PUBLIC effective TEMP must be revoked, got public=%t observer=%t", publicTemp, observerTemp)
	}

	// The observer is a separate local login granted only the no-login group
	// role. This exercises the fixed read-only query/report path without claiming
	// to emulate Cloud SQL IAM authentication.
	observerLogin := pgx.Identifier{ObserverLogin}.Sanitize()
	if _, err := admin.Exec(ctx, `CREATE ROLE `+observerLogin+` LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS INHERIT`); err != nil {
		t.Fatal("create local observer login")
	}
	if _, err := admin.Exec(ctx, `GRANT espacigo_gcp03_observer TO `+observerLogin); err != nil {
		t.Fatal("grant observer group role to local login")
	}
	observerURL := *parsed
	observerURL.User = url.User(ObserverLogin)
	observer, err := pgx.Connect(ctx, observerURL.String())
	if err != nil {
		t.Fatalf("connect with local observer login: %v", err)
	}
	defer observer.Close(context.Background())
	observed, err := observeOnConnection(ctx, observer)
	if err != nil {
		t.Fatalf("run read-only observer report: %v", err)
	}
	if !observed.Passed || observed.TransactionOutcome != "committed" || !observed.ObserverMembership || !observed.ObserverPermissions || observed.ApplicationSchemas != 0 || observed.ApplicationRelations != 0 || observed.ApplicationFunctions != 0 || observed.Relations != observed.ExtensionOwnedRelations || observed.ExtensionOwnedSchemas == 0 || observed.ExtensionOwnedRelations == 0 || observed.ExtensionOwnedFunctions == 0 {
		t.Fatalf("observer report did not pass its fixed baseline: %+v", observed)
	}
	failedCommitReport, failedCommitErr := observeOnConnectionWithCommit(ctx, observer, func(context.Context, pgx.Tx) error {
		return errors.New("injected observer commit acknowledgment failure")
	})
	if failedCommitErr == nil || failedCommitReport.Passed || failedCommitReport.TransactionOutcome != "outcome_uncertain" {
		t.Fatalf("observer commit failure must not produce successful evidence: report=%+v err=%v", failedCommitReport, failedCommitErr)
	}
	ownerTx, err := admin.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		t.Fatal("begin PG18 read-only mutation check", err)
	}
	_, mutationErr := ownerTx.Exec(ctx, `CREATE TABLE public.g03_readonly_probe(id integer)`)
	var pgErr *pgconn.PgError
	if !errors.As(mutationErr, &pgErr) || pgErr.Code != "25006" {
		_ = ownerTx.Rollback(ctx)
		t.Fatalf("read-only transaction must reject DDL with SQLSTATE 25006, got %v", mutationErr)
	}
	_ = ownerTx.Rollback(ctx)
	ownerReport, ownerErr := ownerACLDiagnosticOnConnection(ctx, admin, func(ctx context.Context, tx pgx.Tx) error { return tx.Commit(ctx) })
	if ownerErr != nil || !ownerReport.ReportComplete || ownerReport.TransactionOutcome != "committed" || !ownerReport.ReadOnly || ownerReport.ServerVersionMajor != 18 {
		t.Fatalf("read-only owner/ACL catalog diagnostic should report an incomplete import without requiring normalization: report=%+v err=%v", ownerReport, ownerErr)
	}
	if ownerReport.ExpectedMigration61Present || ownerReport.MigrationHistoryCount != 0 || ownerReport.ValidSessions != 0 || ownerReport.UsableActionTokens != 0 || ownerReport.PendingNotices != 0 || ownerReport.IdentityEventLeases != 0 {
		t.Fatalf("empty scratch fixture must produce catalog/count-only diagnostics: %+v", ownerReport)
	}

}

func assertExtensionsAndRoles(t *testing.T, ctx context.Context, conn *pgx.Conn, wantExtensions int, wantRoles []string) {
	t.Helper()
	var extensionCount int
	if err := conn.QueryRow(ctx, `SELECT count(*) FROM pg_catalog.pg_extension`).Scan(&extensionCount); err != nil {
		t.Fatal("count extensions after rejected bootstrap", err)
	}
	if extensionCount != wantExtensions {
		t.Fatalf("extension count after rejected bootstrap = %d, want %d", extensionCount, wantExtensions)
	}
	rows, err := conn.Query(ctx, `SELECT rolname FROM pg_catalog.pg_roles WHERE rolname IN ('espacigo_runtime','espacigo_gadm_bootstrap','espacigo_gcp03_observer') ORDER BY rolname`)
	if err != nil {
		t.Fatal("inspect bootstrap roles after rejected bootstrap", err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var role string
		if err := rows.Scan(&role); err != nil {
			t.Fatal(err)
		}
		got = append(got, role)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(got) != fmt.Sprint(wantRoles) {
		t.Fatalf("roles after rejected bootstrap = %v, want %v", got, wantRoles)
	}
}
