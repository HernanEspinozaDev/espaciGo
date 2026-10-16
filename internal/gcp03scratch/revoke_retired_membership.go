package gcp03scratch

import (
	"context"
	"errors"

	"github.com/HernanEspinozaDev/espaciGo/internal/dbbootstrap"
	"github.com/HernanEspinozaDev/espaciGo/internal/migrator"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// RetiredMembershipRevocationReport contains fixed catalog checks only. ErrorCode
// is a PostgreSQL SQLSTATE or a fixed local stage code; it never carries server text.
type RetiredMembershipRevocationReport struct {
	PrecheckPassed           bool                       `json:"precheck_passed"`
	ExpectedMembership       bool                       `json:"expected_membership_pass"`
	MutationAttempted        bool                       `json:"mutation_attempted"`
	RevokeStatementSucceeded bool                       `json:"revoke_statement_succeeded"`
	Revoked                  bool                       `json:"revoked"`
	Postcheck                *RetiredVerificationReport `json:"postcheck,omitempty"`
	DiagnosticsCollected     bool                       `json:"diagnostics_collected"`
	DiagnosticsErrorCode     string                     `json:"diagnostics_error_code,omitempty"`
	Passed                   bool                       `json:"passed"`
	ErrorCode                string                     `json:"error_code,omitempty"`
}

func (r RetiredMembershipRevocationReport) IntegrityPassed() bool { return r.Passed }

func runRevokeRetiredMembership(ctx context.Context, config Config, password, migrationDir string) error {
	report, err := RevokeRetiredMembershipPhase(ctx, config, password, migrationDir)
	if err != nil {
		return err
	}
	if !report.Passed {
		return errors.New("GCP-03 fixed retired membership revocation failed its checks")
	}
	return nil
}

// RevokeRetiredMembershipPhase performs one fixed role-membership REVOKE in a
// transaction and commits only after the complete read-only retired contract passes.
func RevokeRetiredMembershipPhase(ctx context.Context, config Config, password, migrationDir string) (RetiredMembershipRevocationReport, error) {
	var report RetiredMembershipRevocationReport
	if config.Phase != PhaseRevokeRetiredMembership || config.Instance != InstanceConnection || config.DatabaseURL != DatabaseOwnerURL || config.Login != DatabaseOwnerLogin || config.PasswordFile != DatabaseOwnerPasswordFile || !config.PrivateIP || migrationDir == "" {
		return report, errors.New("GCP-03 retired membership revocation target is invalid")
	}
	err := withScratchConnection(ctx, config, password, func(ctx context.Context, conn *pgx.Conn) error {
		report = revokeRetiredMembershipOnConnection(ctx, conn, migrationDir)
		return nil
	})
	return report, err
}

// revokeRetiredMembershipOnConnection is shared with the disposable PostgreSQL
// fixture; production target validation stays in RevokeRetiredMembershipPhase.
func revokeRetiredMembershipOnConnection(ctx context.Context, conn *pgx.Conn, migrationDir string) (report RetiredMembershipRevocationReport) {
	tx, err := conn.Begin(ctx)
	if err != nil {
		report.ErrorCode = "begin_failed"
		return
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(context.Background())
		}
	}()
	fail := func(code string) RetiredMembershipRevocationReport {
		report.ErrorCode = code
		_ = collectRevocationDiagnostics(ctx, conn, migrationDir, &report)
		return report
	}
	var exactSession bool
	if err := tx.QueryRow(ctx, `SELECT current_database() = 'espacigo_scratch' AND current_user = 'postgres'`).Scan(&exactSession); err != nil || !exactSession {
		return fail("target_precheck_failed")
	}
	if err := migrator.VerifyAllApplied(ctx, conn, migrationDir, MigrationCount); err != nil {
		return fail("history_precheck_failed")
	}
	// Before removing the inherited cloudsqlsuperuser membership, the migrator
	// is expected to retain TEMP/CREATE. Verify the active ACL/runtime contract
	// here; the strict retired ACL is enforced by the post-revoke observer.
	if err := dbbootstrap.VerifyScratchBootstrap(ctx, conn); err != nil {
		return fail("bootstrap_roles_precheck_failed")
	}
	if err := dbbootstrap.VerifyScratchDatabaseACL(ctx, conn); err != nil {
		return fail("active_database_acl_precheck_failed")
	}
	if err := dbbootstrap.VerifyScratchRuntimePermissions(ctx, conn); err != nil {
		return fail("runtime_precheck_failed")
	}
	preReport, err := ObserveRetiredScratch(ctx, conn, migrationDir)
	if err != nil || !preReport.MigrationHistory || !preReport.OwnerHistoryACL || preReport.HistoryOwner != MigratorLogin ||
		preReport.DatabaseOwner != "cloudsqlsuperuser" || preReport.PublicSchemaOwner != "pg_database_owner" ||
		!allRuntimeChecksPassed(preReport.Observation) {
		return fail("history_acl_precheck_failed")
	}
	var safeAttrs bool
	if err := tx.QueryRow(ctx, `SELECT NOT rolcanlogin AND NOT rolsuper AND NOT rolcreatedb AND NOT rolcreaterole AND NOT rolreplication AND NOT rolbypassrls AND NOT rolinherit FROM pg_catalog.pg_roles WHERE rolname = 'espacigo_scratch_migrator'`).Scan(&safeAttrs); err != nil || !safeAttrs {
		return fail("role_attributes_precheck_failed")
	}
	var membershipCount int
	var exactMembership bool
	if err := tx.QueryRow(ctx, `SELECT count(*), coalesce(bool_and(granted.rolname = 'cloudsqlsuperuser' AND member.rolname = 'espacigo_scratch_migrator' AND NOT m.admin_option AND m.inherit_option AND m.set_option), false)
		FROM pg_catalog.pg_auth_members m JOIN pg_catalog.pg_roles granted ON granted.oid=m.roleid JOIN pg_catalog.pg_roles member ON member.oid=m.member
		WHERE granted.rolname = 'espacigo_scratch_migrator' OR member.rolname = 'espacigo_scratch_migrator'`).Scan(&membershipCount, &exactMembership); err != nil || membershipCount != 1 || !exactMembership {
		return fail("membership_precheck_failed")
	}
	report.ExpectedMembership, report.PrecheckPassed, report.MutationAttempted = true, true, true
	if _, err := tx.Exec(ctx, `REVOKE cloudsqlsuperuser FROM espacigo_scratch_migrator`); err != nil {
		return fail(safeSQLState(err, "revoke_denied"))
	}
	report.RevokeStatementSucceeded = true
	post, err := ObserveRetiredScratch(ctx, conn, migrationDir)
	if err != nil {
		return fail("postcheck_collection_failed")
	}
	report.Postcheck = &post
	report.DiagnosticsCollected = true
	if !post.Passed {
		return fail("postcheck_failed")
	}
	if err := tx.Commit(ctx); err != nil {
		return fail("commit_failed")
	}
	committed, report.Passed, report.Revoked = true, true, true
	return report
}

func allRuntimeChecksPassed(observation dbbootstrap.ScratchObservation) bool {
	if len(observation.RuntimeChecks) != 16 {
		return false
	}
	for _, check := range observation.RuntimeChecks {
		if !check.Passed {
			return false
		}
	}
	return true
}

func collectRevocationDiagnostics(ctx context.Context, conn *pgx.Conn, migrationDir string, report *RetiredMembershipRevocationReport) error {
	if _, err := conn.Exec(ctx, "ROLLBACK"); err != nil {
		report.DiagnosticsErrorCode = "rollback_failed"
		return nil
	}
	if _, err := conn.Exec(ctx, "BEGIN READ ONLY"); err != nil {
		report.DiagnosticsErrorCode = "diagnostic_transaction_failed"
		return nil
	}
	defer func() { _, _ = conn.Exec(context.Background(), "ROLLBACK") }()
	post, err := ObserveRetiredScratch(ctx, conn, migrationDir)
	if err != nil {
		report.DiagnosticsErrorCode = "diagnostic_collection_failed"
		return nil
	}
	report.Postcheck, report.DiagnosticsCollected = &post, true
	return nil
}

func safeSQLState(err error, fallback string) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && len(pgErr.Code) == 5 {
		return pgErr.Code
	}
	return fallback
}
