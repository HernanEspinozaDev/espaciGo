package main

import (
	"context"
	"errors"

	"github.com/HernanEspinozaDev/espaciGo/internal/dbbootstrap"
	"github.com/HernanEspinozaDev/espaciGo/internal/migrator"
	"github.com/jackc/pgx/v5"
)

const runtimeACLConfirmation = "espacigo_f5_gate_20261101"

// Keep this false until the readback is generated from the complete reviewed
// allowlist and covers direct and effective ACLs, including PUBLIC grants.
const runtimeACLCompleteReadbackImplemented = false

func validateRuntimeACLPlan(plan migrator.PlanResult) error {
	if plan.Database != runtimeACLConfirmation || !plan.ReadOnly || !plan.HistoryExists || len(plan.Pending) != 0 || len(plan.Applied) == 0 {
		return errors.New("runtime ACL phase requires a complete, read-only migration ledger on the exact F5 target")
	}
	if plan.Applied[len(plan.Applied)-1].Version < 82 {
		return errors.New("runtime ACL phase requires V82 or later to be applied")
	}
	for _, applied := range plan.Applied {
		if applied.Version == 82 && applied.Name == "document_verification_retention_precedence" {
			return nil
		}
	}
	return errors.New("runtime ACL phase requires the reviewed V82 migration in the applied ledger")
}

func applyRuntimeACLPhase(ctx context.Context, databaseURL, migrationDir string) error {
	if !runtimeACLCompleteReadbackImplemented {
		return errors.New("runtime ACL phase is disabled until complete runtime ACL readback is implemented and reviewed")
	}
	config, err := pgx.ParseConfig(databaseURL)
	if err != nil {
		return errors.New("invalid runtime ACL connection configuration")
	}
	conn, err := pgx.ConnectConfig(ctx, config)
	if err != nil {
		return errors.New("could not connect for the explicit runtime ACL phase")
	}
	defer conn.Close(context.Background())

	var database, sessionUser, currentUser string
	if err := conn.QueryRow(ctx, "SELECT current_database(), session_user, current_user").Scan(&database, &sessionUser, &currentUser); err != nil ||
		database != runtimeACLConfirmation || sessionUser != "postgres" || currentUser != "postgres" {
		return errors.New("connected database or administrator identity does not match the approved runtime ACL target")
	}
	releaseMigrationLock, err := migrator.AcquireMigrationAdvisoryLock(ctx, conn)
	if err != nil {
		return errors.New("could not acquire migration lock for the runtime ACL phase")
	}
	defer func() { _ = releaseMigrationLock(context.Background()) }()

	plan, err := migrator.Plan(ctx, databaseURL, migrationDir, "")
	if err != nil {
		return errors.New("read-only migration preflight failed while holding the migration lock")
	}
	if err := validateRuntimeACLPlan(plan); err != nil {
		return err
	}

	tx, err := conn.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return errors.New("could not begin the runtime ACL transaction")
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err := dbbootstrap.GrantRuntimePermissions(ctx, tx); err != nil {
		return errors.New("runtime ACL allowlist application failed")
	}
	if err := dbbootstrap.VerifyRuntimePermissionCoreReadback(ctx, tx); err != nil {
		return errors.New("runtime ACL core readback failed before commit")
	}
	if err := tx.Commit(ctx); err != nil {
		return errors.New("runtime ACL transaction commit failed")
	}
	if err := dbbootstrap.VerifyRuntimePermissionCoreReadback(ctx, conn); err != nil {
		return errors.New("runtime ACL transaction committed, but post-commit core readback failed")
	}
	return nil
}
