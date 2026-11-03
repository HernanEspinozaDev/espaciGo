package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/HernanEspinozaDev/espaciGo/internal/dbbootstrap"
	"github.com/HernanEspinozaDev/espaciGo/internal/migrator"
	"github.com/jackc/pgx/v5"
)

const runtimeACLConfirmation = "espacigo_f5_gate_20261101"

// This non-secret client marker is injected only after the exact F5
// administrative confirmation passes. It is an accidental-bypass tripwire,
// not an authorization boundary against a PostgreSQL superuser.
const f5AdminMigrationApplicationName = "f5-confirmed:espacigo_f5_gate_20261101"

// The source-derived catalog readback is implemented and has run in the gated
// F5 rollback preview. This status does not authorize persistent ACL DDL.
const runtimeACLCompleteReadbackImplemented = true

// Independent review passed for the exact recovery implementation and plan
// guard. The command still enforces target, identity and readback checks.
const runtimeACLPhaseReviewed = true

type runtimeACLRecoveryResult uint8

const (
	runtimeACLRecoveryAlreadyValid runtimeACLRecoveryResult = iota + 1
	runtimeACLRecoveryApplied
)

func recoverRuntimeACLState(verify func() error, replay func() error) (runtimeACLRecoveryResult, error) {
	if verify == nil || replay == nil {
		return 0, errors.New("runtime ACL recovery requires readback and source-derived replay functions")
	}
	if err := verify(); err == nil {
		return runtimeACLRecoveryAlreadyValid, nil
	} else if _, ok := err.(*dbbootstrap.RuntimeACLRepairableDriftError); !ok {
		return 0, fmt.Errorf("runtime ACL pre-replay readback was inconclusive; no ACL DDL was run: %w", err)
	}
	if err := replay(); err != nil {
		return 0, fmt.Errorf("runtime ACL forward recovery replay failed; rerun -recover-runtime-acl to revalidate the target and source checksums: %w", err)
	}
	if err := verify(); err != nil {
		return 0, fmt.Errorf("runtime ACL recovery committed but complete post-commit readback still fails; inspect the exact drift, then rerun -recover-runtime-acl: %w", err)
	}
	return runtimeACLRecoveryApplied, nil
}

type runtimeACLInvocationMode uint8

const (
	runtimeACLInvocationNone runtimeACLInvocationMode = iota
	runtimeACLInvocationApply
	runtimeACLInvocationRecover
)

func validateRuntimeACLInvocation(apply, recover, planOnly bool, confirmACL, confirmAdmin string) (runtimeACLInvocationMode, error) {
	if apply && recover {
		return runtimeACLInvocationNone, errors.New("runtime ACL apply and recovery modes are mutually exclusive")
	}
	if !apply && !recover {
		if confirmACL != "" {
			return runtimeACLInvocationNone, errors.New("-confirm-runtime-acl is valid only with -apply-runtime-acl or -recover-runtime-acl")
		}
		return runtimeACLInvocationNone, nil
	}
	if planOnly || confirmACL != runtimeACLConfirmation || confirmAdmin != "" {
		return runtimeACLInvocationNone, errors.New("runtime ACL phase requires a separate invocation and exact F5 target confirmation")
	}
	if apply {
		return runtimeACLInvocationApply, nil
	}
	return runtimeACLInvocationRecover, nil
}

func validateRuntimeACLReviewGate(mode runtimeACLInvocationMode, reviewed bool) error {
	if mode != runtimeACLInvocationNone && !reviewed {
		return errors.New("runtime ACL DDL phase is not independently reviewed")
	}
	return nil
}

func validateRuntimeACLPlan(plan migrator.PlanResult) error {
	if plan.Database != runtimeACLConfirmation || !plan.ReadOnly || !plan.HistoryExists || len(plan.Pending) != 0 || len(plan.Applied) == 0 {
		return errors.New("runtime ACL phase requires a complete, read-only migration ledger on the exact F5 target")
	}
	if plan.Applied[len(plan.Applied)-1].Version < 82 {
		return errors.New("runtime ACL phase requires V82 or later to be applied")
	}
	for _, applied := range plan.Applied {
		if applied.Version == 82 && applied.Name == "V000082__document_verification_retention_precedence.sql" {
			return nil
		}
	}
	return errors.New("runtime ACL phase requires the reviewed V82 migration in the applied ledger")
}

func runtimeACLSourceSet(migrations []migrator.Migration, applied []migrator.HistoryEntry) ([]dbbootstrap.RuntimeACLSource, error) {
	byVersion := make(map[int64]migrator.Migration, len(migrations))
	for _, migration := range migrations {
		byVersion[migration.Version] = migration
	}
	sources := make([]dbbootstrap.RuntimeACLSource, 0, len(applied))
	previousVersion := int64(0)
	for _, entry := range applied {
		if entry.Version <= previousVersion {
			return nil, errors.New("runtime ACL source ledger is not strictly ordered")
		}
		previousVersion = entry.Version
		migration, ok := byVersion[entry.Version]
		if !ok || migration.Name != entry.Name || migration.Checksum != entry.Checksum {
			return nil, fmt.Errorf("runtime ACL source differs from the applied ledger at V%06d", entry.Version)
		}
		sources = append(sources, dbbootstrap.RuntimeACLSource{Version: int(migration.Version), SQL: migration.SQL})
	}
	if len(sources) == 0 {
		return nil, errors.New("runtime ACL source set is empty")
	}
	return sources, nil
}

func applyRuntimeACLPhase(ctx context.Context, databaseURL, migrationDir string) error {
	_, err := runRuntimeACLPhase(ctx, databaseURL, migrationDir, false)
	return err
}

func recoverRuntimeACLPhase(ctx context.Context, databaseURL, migrationDir string) (runtimeACLRecoveryResult, error) {
	return runRuntimeACLPhase(ctx, databaseURL, migrationDir, true)
}

func runRuntimeACLPhase(ctx context.Context, databaseURL, migrationDir string, recover bool) (runtimeACLRecoveryResult, error) {
	mode := runtimeACLInvocationApply
	if recover {
		mode = runtimeACLInvocationRecover
	}
	if err := validateRuntimeACLReviewGate(mode, runtimeACLPhaseReviewed); err != nil {
		return 0, err
	}
	if !runtimeACLCompleteReadbackImplemented {
		return 0, errors.New("runtime ACL phase is disabled until source-derived runtime ACL readback is implemented and exercised")
	}
	config, err := pgx.ParseConfig(databaseURL)
	if err != nil {
		return 0, errors.New("invalid runtime ACL connection configuration")
	}
	conn, err := pgx.ConnectConfig(ctx, config)
	if err != nil {
		return 0, errors.New("could not connect for the explicit runtime ACL phase")
	}
	defer conn.Close(context.Background())

	var database, sessionUser, currentUser string
	if err := conn.QueryRow(ctx, "SELECT current_database(), session_user, current_user").Scan(&database, &sessionUser, &currentUser); err != nil ||
		database != runtimeACLConfirmation || sessionUser != "postgres" || currentUser != "postgres" {
		return 0, errors.New("connected database or administrator identity does not match the approved runtime ACL target")
	}
	releaseMigrationLock, err := migrator.AcquireMigrationAdvisoryLock(ctx, conn)
	if err != nil {
		return 0, errors.New("could not acquire migration lock for the runtime ACL phase")
	}
	defer func() { _ = releaseMigrationLock(context.Background()) }()

	plan, err := migrator.Plan(ctx, databaseURL, migrationDir, "")
	if err != nil {
		return 0, errors.New("read-only migration preflight failed while holding the migration lock")
	}
	if err := validateRuntimeACLPlan(plan); err != nil {
		return 0, err
	}
	migrations, err := migrator.DiscoverMigrations(migrationDir)
	if err != nil {
		return 0, errors.New("could not discover migrations for the source-derived runtime ACL readback")
	}
	sources, err := runtimeACLSourceSet(migrations, plan.Applied)
	if err != nil {
		return 0, err
	}

	verify := func() error { return dbbootstrap.VerifyRuntimeACLReadbackF5(ctx, conn, sources) }
	replay := func() error {
		tx, err := conn.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return errors.New("could not begin the runtime ACL transaction")
		}
		defer func() { _ = tx.Rollback(context.Background()) }()
		if err := dbbootstrap.ApplyRuntimeACLFromMigrationsF5(ctx, tx, sources); err != nil {
			return fmt.Errorf("apply source-derived runtime ACL migration effects: %w", err)
		}
		if err := dbbootstrap.GrantRuntimePermissions(ctx, tx); err != nil {
			return errors.New("runtime ACL allowlist application failed")
		}
		if err := dbbootstrap.VerifyRuntimeACLReadbackF5(ctx, tx, sources); err != nil {
			return fmt.Errorf("complete runtime ACL readback failed before commit: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("runtime ACL transaction commit outcome may be ambiguous; run -recover-runtime-acl to read back and reconcile the exact target: %w", err)
		}
		return nil
	}

	if recover {
		return recoverRuntimeACLState(verify, replay)
	}
	if err := replay(); err != nil {
		return 0, err
	}
	if err := verify(); err != nil {
		return 0, fmt.Errorf("runtime ACL transaction committed, but complete post-commit readback failed; run -recover-runtime-acl to read back and reconcile the exact target: %w", err)
	}
	return runtimeACLRecoveryApplied, nil
}
