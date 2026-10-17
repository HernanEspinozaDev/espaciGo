package gcp03origin

import (
	"context"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// TestOriginMembershipPostgres18 executes the production transaction helper as
// a non-superuser postgres login with Cloud SQL's LOGIN/CREATEROLE/CREATEDB
// attributes. The disposable fixture is prepared by the shell test runner.
func TestOriginMembershipPostgres18(t *testing.T) {
	connectionURL := os.Getenv("GCP03_ORIGIN_MEMBERSHIP_TEST_DATABASE_URL")
	if connectionURL == "" {
		t.Skip("use scripts/test-gcp03-origin-postgres.sh for the isolated membership PostgreSQL 18 integration")
	}
	postgresURL, err := url.Parse(connectionURL)
	if err != nil || postgresURL.Scheme != "postgres" || postgresURL.Hostname() != "localhost" || postgresURL.User == nil || postgresURL.User.Username() != AdminLogin || postgresURL.Path != "/"+DatabaseName {
		t.Fatal("membership integration URL must use the disposable localhost postgres login and exact scratch database")
	}
	query, err := url.ParseQuery(postgresURL.RawQuery)
	socketDir := query.Get("host")
	if err != nil || len(query) != 2 || query.Get("sslmode") != "disable" || !filepath.IsAbs(socketDir) || !strings.HasPrefix(socketDir, "/tmp/espacigo-gcp03-origin.") {
		t.Fatal("membership integration URL must use the isolated Unix socket and sslmode=disable")
	}
	if _, err := os.Stat(filepath.Join(socketDir, ".gcp03-origin-membership-isolated-test")); err != nil {
		t.Fatal("membership integration socket is missing its disposable fixture marker")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	phaseConn, err := pgx.Connect(ctx, connectionURL)
	if err != nil {
		t.Fatalf("connect as Cloud SQL-compatible postgres login: %v", err)
	}
	defer phaseConn.Close(context.Background())
	var database, login string
	var superuser, createRole, createDB bool
	var major int
	if err := phaseConn.QueryRow(ctx, `SELECT current_database(),current_user,current_setting('server_version_num')::int/10000,r.rolsuper,r.rolcreaterole,r.rolcreatedb FROM pg_catalog.pg_roles r WHERE r.rolname=current_user`).Scan(&database, &login, &major, &superuser, &createRole, &createDB); err != nil {
		t.Fatal("read fixed membership fixture identity")
	}
	if database != DatabaseName || login != AdminLogin || major != 18 || superuser || !createRole || !createDB {
		t.Fatalf("membership phase fixture does not match Cloud SQL postgres profile: database=%q login=%q major=%d super=%t createrole=%t createdb=%t", database, login, major, superuser, createRole, createDB)
	}

	controlURL := *postgresURL
	controlURL.User = url.User("fixture_control")
	control, err := pgx.Connect(ctx, controlURL.String())
	if err != nil {
		t.Fatalf("connect isolated fixture control: %v", err)
	}
	defer control.Close(context.Background())
	var roleCountBefore, objectCountBefore int64
	if err := control.QueryRow(ctx, `SELECT (SELECT count(*) FROM pg_catalog.pg_roles), (SELECT count(*) FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname NOT LIKE 'pg_%' AND n.nspname <> 'information_schema')`).Scan(&roleCountBefore, &objectCountBefore); err != nil {
		t.Fatal("record membership fixture inventory")
	}

	commit := func(ctx context.Context, tx pgx.Tx) error { return tx.Commit(ctx) }

	// Wrong identity is rejected before the one mutating statement.
	otherURL := *postgresURL
	otherURL.User = url.User("membership_observer")
	other, err := pgx.Connect(ctx, otherURL.String())
	if err != nil {
		t.Fatalf("connect unexpected-identity fixture login: %v", err)
	}
	wrongIdentityReport, wrongIdentityErr := membershipOnConnection(ctx, other, commit)
	_ = other.Close(context.Background())
	if wrongIdentityErr == nil || wrongIdentityReport.Login != "membership_observer" || wrongIdentityReport.TransactionOutcome != "rolled_back" {
		t.Fatalf("membership phase did not reject unexpected identity: report=%+v err=%v", wrongIdentityReport, wrongIdentityErr)
	}

	// Unsafe role attributes are rejected before the grant, with no side effect.
	if _, err := control.Exec(ctx, `ALTER ROLE espacigo_runtime LOGIN`); err != nil {
		t.Fatal("prepare unsafe runtime role")
	}
	unsafeRoleReport, unsafeRoleErr := membershipOnConnection(ctx, phaseConn, commit)
	if unsafeRoleErr == nil || unsafeRoleReport.TransactionOutcome != "rolled_back" {
		t.Fatalf("membership phase did not reject unsafe runtime role: report=%+v err=%v", unsafeRoleReport, unsafeRoleErr)
	}
	if _, err := control.Exec(ctx, `ALTER ROLE espacigo_runtime NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT`); err != nil {
		t.Fatal("restore restricted runtime role")
	}

	// Remove grant authority so the phase's exact GRANT fails and rolls back.
	if _, err := control.Exec(ctx, `REVOKE ADMIN OPTION FOR espacigo_runtime FROM postgres`); err != nil {
		t.Fatal("remove fixture grant authority")
	}
	grantFailureReport, grantFailureErr := membershipOnConnection(ctx, phaseConn, commit)
	if grantFailureErr == nil || grantFailureReport.TransactionOutcome != "rolled_back" {
		t.Fatalf("membership phase did not roll back GRANT failure: report=%+v err=%v", grantFailureReport, grantFailureErr)
	}
	assertRuntimeMembershipOptions(t, ctx, control, "fixture_control", false, false, true)

	// A deliberately false postcheck verifies rollback after the GRANT ran.
	if _, err := control.Exec(ctx, `GRANT espacigo_runtime TO postgres WITH ADMIN TRUE, INHERIT FALSE, SET TRUE`); err != nil {
		t.Fatal("restore fixture grant authority")
	}
	postcheckReport, postcheckErr := membershipOnConnectionWithPostcheck(ctx, phaseConn, `SELECT false,false,false,false`, commit)
	if postcheckErr == nil || postcheckReport.Passed || postcheckReport.TransactionOutcome != "rolled_back" {
		t.Fatalf("membership phase did not roll back failed postcheck: report=%+v err=%v", postcheckReport, postcheckErr)
	}
	assertRuntimeMembershipOptions(t, ctx, control, "fixture_control", true, false, true)

	// A lost commit acknowledgment must be reported as uncertain; closing the
	// isolated connection rolls the fixture transaction back for verification.
	uncertainConn, err := pgx.Connect(ctx, connectionURL)
	if err != nil {
		t.Fatal("connect membership commit-uncertainty fixture")
	}
	uncertainReport, uncertainErr := membershipOnConnection(ctx, uncertainConn, func(context.Context, pgx.Tx) error {
		return errors.New("injected commit acknowledgment failure")
	})
	if uncertainErr == nil || uncertainReport.Passed || uncertainReport.TransactionOutcome != "outcome_uncertain" {
		t.Fatalf("membership commit uncertainty was not reported safely: report=%+v err=%v", uncertainReport, uncertainErr)
	}
	_ = uncertainConn.Close(context.Background())
	assertRuntimeMembershipOptions(t, ctx, control, "fixture_control", true, false, true)

	// Exercise the actual production transaction helper and exact final flags.
	successReport, successErr := membershipOnConnection(ctx, phaseConn, commit)
	if successErr != nil || !successReport.Passed || successReport.TransactionOutcome != "committed" || successReport.AdminOption || successReport.InheritOption || !successReport.SetOption {
		t.Fatalf("membership phase did not commit minimal membership: report=%+v err=%v", successReport, successErr)
	}
	assertRuntimeMembershipOptions(t, ctx, control, AdminLogin, false, false, true)
	assertRuntimeMembershipOptions(t, ctx, control, "fixture_control", true, false, true)
	var roleCountAfter, objectCountAfter int64
	if err := control.QueryRow(ctx, `SELECT (SELECT count(*) FROM pg_catalog.pg_roles), (SELECT count(*) FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname NOT LIKE 'pg_%' AND n.nspname <> 'information_schema')`).Scan(&roleCountAfter, &objectCountAfter); err != nil {
		t.Fatal("read membership fixture inventory after phase")
	}
	if roleCountAfter != roleCountBefore || objectCountAfter != objectCountBefore {
		t.Fatalf("membership phase changed role/object inventory: roles %d->%d objects %d->%d", roleCountBefore, roleCountAfter, objectCountBefore, objectCountAfter)
	}
}

func assertRuntimeMembershipOptions(t *testing.T, ctx context.Context, conn *pgx.Conn, grantor string, admin, inherit, set bool) {
	t.Helper()
	var gotAdmin, gotInherit, gotSet bool
	if err := conn.QueryRow(ctx, `SELECT m.admin_option,m.inherit_option,m.set_option FROM pg_catalog.pg_auth_members m WHERE m.roleid='espacigo_runtime'::regrole AND m.member='postgres'::regrole AND m.grantor=$1::regrole`, grantor).Scan(&gotAdmin, &gotInherit, &gotSet); err != nil {
		t.Fatalf("read runtime membership options: %v", err)
	}
	if gotAdmin != admin || gotInherit != inherit || gotSet != set {
		t.Fatalf("runtime membership options = (%t,%t,%t), want (%t,%t,%t)", gotAdmin, gotInherit, gotSet, admin, inherit, set)
	}
}
