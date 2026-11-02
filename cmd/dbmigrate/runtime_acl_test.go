package main

import (
	"context"
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
			{Version: 81, Name: "document_verification_runtime_scope_helpers"},
			{Version: 82, Name: "document_verification_retention_precedence"},
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
		{name: "wrong V82 identity", mutate: func(p *migrator.PlanResult) { p.Applied[1].Name = "unexpected_migration" }},
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

func TestApplyRuntimeACLPhaseKeepsReadbackImplementedButClosesDDLGate(t *testing.T) {
	if !runtimeACLCompleteReadbackImplemented {
		t.Fatal("source-derived catalog readback has been exercised against the F5 PostgreSQL rollback preview")
	}
	if runtimeACLPhaseReviewed {
		t.Fatal("runtime ACL DDL gate must remain closed until the source/catalog contract is independently reviewed")
	}
	err := applyRuntimeACLPhase(context.Background(), "", "")
	if err == nil || !strings.Contains(err.Error(), "not independently reviewed") {
		t.Fatalf("runtime ACL phase must stop at the review gate before connection setup, got %v", err)
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
		t.Fatal("checksum drift was accepted in the ACL migration source set")
	}
}
