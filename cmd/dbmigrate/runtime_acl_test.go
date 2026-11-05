package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/HernanEspinozaDev/espaciGo/internal/dbbootstrap"
	"github.com/HernanEspinozaDev/espaciGo/internal/migrator"
)

func TestValidateRuntimeACLPlanRequiresCompletedV82Ledger(t *testing.T) {
	valid := migrator.PlanResult{
		Database:      runtimeACLConfirmation,
		ReadOnly:      true,
		HistoryExists: true,
		Applied: []migrator.HistoryEntry{
			{Version: 81, Name: "V000081__document_verification_runtime_scope_helpers.sql"},
			{Version: 82, Name: "V000082__document_verification_retention_precedence.sql"},
		},
		Pending: []migrator.MigrationSummary{},
	}
	if err := validateRuntimeACLPlan(valid); err != nil {
		t.Fatalf("valid applied ledger rejected: %v", err)
	}

	cases := []struct {
		name   string
		mutate func(*migrator.PlanResult)
	}{
		{name: "wrong database", mutate: func(p *migrator.PlanResult) { p.Database = "espacigo_f5_synth_20261101" }},
		{name: "not read only", mutate: func(p *migrator.PlanResult) { p.ReadOnly = false }},
		{name: "no history", mutate: func(p *migrator.PlanResult) { p.HistoryExists = false }},
		{name: "pending migration", mutate: func(p *migrator.PlanResult) { p.Pending = []migrator.MigrationSummary{{Version: 83}} }},
		{name: "missing V82", mutate: func(p *migrator.PlanResult) { p.Applied = p.Applied[:1] }},
		{name: "wrong V82 identity", mutate: func(p *migrator.PlanResult) { p.Applied[1].Name = "V000082__unexpected_migration.sql" }},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			plan := valid
			plan.Applied = append([]migrator.HistoryEntry(nil), valid.Applied...)
			plan.Pending = append([]migrator.MigrationSummary(nil), valid.Pending...)
			test.mutate(&plan)
			if err := validateRuntimeACLPlan(plan); err == nil {
				t.Fatal("unsafe runtime ACL plan accepted")
			}
		})
	}
}

func TestApplyRuntimeACLPhaseRequiresReviewedReadback(t *testing.T) {
	if !runtimeACLCompleteReadbackImplemented {
		t.Fatal("source-derived catalog readback has been exercised against the F5 PostgreSQL rollback preview")
	}
	if !runtimeACLPhaseReviewed {
		t.Fatal("runtime ACL DDL gate must open only after the independent source review passes")
	}
	for _, mode := range []runtimeACLInvocationMode{runtimeACLInvocationApply, runtimeACLInvocationRecover} {
		if err := validateRuntimeACLReviewGate(mode, runtimeACLPhaseReviewed); err != nil {
			t.Fatalf("reviewed runtime ACL mode %v rejected: %v", mode, err)
		}
	}
	if err := validateRuntimeACLReviewGate(runtimeACLInvocationApply, false); err == nil || !strings.Contains(err.Error(), "not independently reviewed") {
		t.Fatalf("unreviewed runtime ACL apply must remain closed before connection setup, got %v", err)
	}
}

func TestRuntimeACLSourceSetUsesOnlyChecksumMatchedAppliedMigrations(t *testing.T) {
	migrations, err := migrator.DiscoverMigrations("../../db/migrations")
	if err != nil {
		t.Fatal(err)
	}
	applied := []migrator.HistoryEntry{
		{Version: migrations[0].Version, Name: migrations[0].Name, Checksum: migrations[0].Checksum},
		{Version: migrations[1].Version, Name: migrations[1].Name, Checksum: migrations[1].Checksum},
	}
	sources, err := runtimeACLSourceSet(migrations, applied)
	if err != nil {
		t.Fatal(err)
	}
	if len(sources) != len(applied) || sources[0].Version != int(applied[0].Version) || sources[1].Version != int(applied[1].Version) {
		t.Fatalf("source set does not match applied ledger: %#v", sources)
	}
	if _, err := dbbootstrap.BuildRuntimeACLContract(sources); err != nil {
		t.Fatalf("build ACL contract from applied migrations: %v", err)
	}
	applied[1].Checksum = "drift"
	if _, err := runtimeACLSourceSet(migrations, applied); err == nil {
		t.Fatal("checksum drift was accepted in the runtime ACL source set")
	}
}

func TestRecoverRuntimeACLStateSkipsReplayWhenReadbackPasses(t *testing.T) {
	verifyCalls, replayCalls := 0, 0
	result, err := recoverRuntimeACLState(
		func() error { verifyCalls++; return nil },
		func() error { replayCalls++; return nil },
	)
	if err != nil {
		t.Fatalf("already-valid ACL state rejected: %v", err)
	}
	if result != runtimeACLRecoveryAlreadyValid || verifyCalls != 1 || replayCalls != 0 {
		t.Fatalf("recovery must verify once and avoid DDL when already valid: result=%v verify=%d replay=%d", result, verifyCalls, replayCalls)
	}
}

func TestRecoverRuntimeACLStateDoesNotReplayAfterUnavailableReadback(t *testing.T) {
	readbackErr := errors.New("catalog query failed")
	replayCalls := 0
	_, err := recoverRuntimeACLState(
		func() error { return readbackErr },
		func() error { replayCalls++; return nil },
	)
	if err == nil || !strings.Contains(err.Error(), "inconclusive") {
		t.Fatalf("unavailable readback must fail closed as inconclusive, got %v", err)
	}
	if replayCalls != 0 {
		t.Fatalf("unavailable readback must not trigger ACL DDL, replay calls=%d", replayCalls)
	}
}

func TestRecoverRuntimeACLStateDoesNotReplayWhenReadbackAlsoHasUnrelatedFailure(t *testing.T) {
	readbackErr := errors.Join(&dbbootstrap.RuntimeACLRepairableDriftError{Detail: "grant mismatch"}, errors.New("effective privilege query failed"))
	replayCalls := 0
	_, err := recoverRuntimeACLState(
		func() error { return readbackErr },
		func() error { replayCalls++; return nil },
	)
	if err == nil || !strings.Contains(err.Error(), "inconclusive") {
		t.Fatalf("repairable drift plus an unrelated readback failure must fail closed, got %v", err)
	}
	if replayCalls != 0 {
		t.Fatalf("mixed readback failure must not trigger ACL DDL, replay calls=%d", replayCalls)
	}
}

func TestRecoverRuntimeACLStateReplaysAndVerifiesAfterMismatch(t *testing.T) {
	verified, replayCalls, verifyCalls := false, 0, 0
	result, err := recoverRuntimeACLState(
		func() error {
			verifyCalls++
			if !verified {
				return &dbbootstrap.RuntimeACLRepairableDriftError{Detail: "required direct ACL grant is missing"}
			}
			return nil
		},
		func() error { replayCalls++; verified = true; return nil },
	)
	if err != nil {
		t.Fatalf("source-derived ACL recovery failed: %v", err)
	}
	if result != runtimeACLRecoveryApplied || verifyCalls != 2 || replayCalls != 1 {
		t.Fatalf("recovery must replay once and verify after commit: result=%v verify=%d replay=%d", result, verifyCalls, replayCalls)
	}
}

func TestRecoverRuntimeACLStateReportsCommittedButUnverified(t *testing.T) {
	verifyCalls, replayCalls := 0, 0
	_, err := recoverRuntimeACLState(
		func() error {
			verifyCalls++
			return &dbbootstrap.RuntimeACLRepairableDriftError{Detail: "required direct ACL grant is missing"}
		},
		func() error { replayCalls++; return nil },
	)
	if err == nil || !strings.Contains(err.Error(), "committed but") || !strings.Contains(err.Error(), "-recover-runtime-acl") {
		t.Fatalf("post-commit readback failure must identify committed state and recovery command, got %v", err)
	}
	if verifyCalls != 2 || replayCalls != 1 {
		t.Fatalf("post-commit readback must follow exactly one replay: verify=%d replay=%d", verifyCalls, replayCalls)
	}
}

func TestRecoverRuntimeACLStateRetriesAmbiguousCommitByReadback(t *testing.T) {
	committed, replayCalls := false, 0
	verify := func() error {
		if committed {
			return nil
		}
		return &dbbootstrap.RuntimeACLRepairableDriftError{Detail: "required direct ACL grant is missing"}
	}
	_, firstErr := recoverRuntimeACLState(verify, func() error {
		replayCalls++
		committed = true
		return errors.New("runtime ACL transaction commit outcome may be ambiguous: connection lost")
	})
	if firstErr == nil || !strings.Contains(firstErr.Error(), "outcome may be ambiguous") || !strings.Contains(firstErr.Error(), "-recover-runtime-acl") {
		t.Fatalf("ambiguous commit must direct operator to readback-first recovery, got %v", firstErr)
	}
	result, secondErr := recoverRuntimeACLState(verify, func() error {
		replayCalls++
		return nil
	})
	if secondErr != nil || result != runtimeACLRecoveryAlreadyValid || replayCalls != 1 {
		t.Fatalf("retry must discover committed ACLs and skip a second replay: result=%v err=%v replay=%d", result, secondErr, replayCalls)
	}
}

func TestValidateRuntimeACLInvocationRequiresSeparateExactConfirmation(t *testing.T) {
	cases := []struct {
		name                     string
		apply, recover, planOnly bool
		confirmACL, confirmAdmin string
		wantMode                 runtimeACLInvocationMode
		wantError                bool
	}{
		{name: "normal migration mode", wantMode: runtimeACLInvocationNone},
		{name: "apply exact target", apply: true, confirmACL: runtimeACLConfirmation, wantMode: runtimeACLInvocationApply},
		{name: "recover exact target", recover: true, confirmACL: runtimeACLConfirmation, wantMode: runtimeACLInvocationRecover},
		{name: "missing confirmation", apply: true, wantError: true},
		{name: "both runtime modes", apply: true, recover: true, confirmACL: runtimeACLConfirmation, wantError: true},
		{name: "plan only", recover: true, planOnly: true, confirmACL: runtimeACLConfirmation, wantError: true},
		{name: "confirmation without mode", confirmACL: runtimeACLConfirmation, wantError: true},
		{name: "admin migration confirmation collision", recover: true, confirmACL: runtimeACLConfirmation, confirmAdmin: runtimeACLConfirmation, wantError: true},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			mode, err := validateRuntimeACLInvocation(test.apply, test.recover, test.planOnly, test.confirmACL, test.confirmAdmin)
			if (err != nil) != test.wantError {
				t.Fatalf("unexpected validation error: %v", err)
			}
			if err == nil && mode != test.wantMode {
				t.Fatalf("mode=%v want=%v", mode, test.wantMode)
			}
		})
	}
}

func TestValidateRuntimeACLReviewGateRejectsUnreviewedMutations(t *testing.T) {
	if err := validateRuntimeACLReviewGate(runtimeACLInvocationNone, false); err != nil {
		t.Fatalf("non-runtime migration command must not require ACL review: %v", err)
	}
	for _, mode := range []runtimeACLInvocationMode{runtimeACLInvocationApply, runtimeACLInvocationRecover} {
		if err := validateRuntimeACLReviewGate(mode, false); err == nil || !strings.Contains(err.Error(), "not independently reviewed") {
			t.Fatalf("unreviewed runtime ACL mode %v must fail before configuration or connection, got %v", mode, err)
		}
		if err := validateRuntimeACLReviewGate(mode, true); err != nil {
			t.Fatalf("reviewed runtime ACL mode %v rejected: %v", mode, err)
		}
	}
}
