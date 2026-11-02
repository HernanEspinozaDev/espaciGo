package main

import (
	"context"
	"strings"
	"testing"

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

func TestApplyRuntimeACLPhaseFailsClosedUntilCompleteReadbackExists(t *testing.T) {
	err := applyRuntimeACLPhase(context.Background(), "", "")
	if err == nil || !strings.Contains(err.Error(), "complete runtime ACL readback") {
		t.Fatalf("runtime ACL phase should fail before parsing or using a connection until complete readback exists; got %v", err)
	}
}
