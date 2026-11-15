package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/HernanEspinozaDev/espaciGo/internal/migrator"
)

func TestValidateNativeAdminRangeInvocationPinsTargetAndIdentity(t *testing.T) {
	baseURL := "postgresql://postgres@/espacigo_dms_stage_20261020?host=%2Fvar%2Frun%2Fpostgresql&port=5432&sslmode=disable"
	base := connectionSettings{databaseURL: baseURL, mode: "direct"}
	tmpDir := t.TempDir()
	if err := os.Chmod(tmpDir, 0700); err != nil {
		t.Fatal(err)
	}
	cleanEnv := map[string]string{}
	env := func(values map[string]string) func(string) string {
		return func(key string) string {
			if value, ok := values[key]; ok {
				return value
			}
			if key == "TMPDIR" {
				return tmpDir
			}
			return ""
		}
	}

	tests := []struct {
		name      string
		confirm   string
		settings  connectionSettings
		variables map[string]string
		osUser    string
		wantErr   bool
	}{
		{name: "exact native target and postgres OS identity", confirm: "espacigo_dms_stage_20261020", settings: base, variables: cleanEnv, osUser: "postgres"},
		{name: "missing confirmation", settings: base, variables: cleanEnv, osUser: "postgres", wantErr: true},
		{name: "wrong confirmation", confirm: "espacigo_local", settings: base, variables: cleanEnv, osUser: "postgres", wantErr: true},
		{name: "wrong database", confirm: "espacigo_dms_stage_20261020", settings: connectionSettings{databaseURL: "postgresql://postgres@/postgres?host=%2Fvar%2Frun%2Fpostgresql&port=5432&sslmode=disable", mode: "direct"}, variables: cleanEnv, osUser: "postgres", wantErr: true},
		{name: "TCP instead of peer socket", confirm: "espacigo_dms_stage_20261020", settings: connectionSettings{databaseURL: "postgresql://postgres@127.0.0.1:5432/espacigo_dms_stage_20261020?sslmode=disable", mode: "direct"}, variables: cleanEnv, osUser: "postgres", wantErr: true},
		{name: "runtime role is not administrator", confirm: "espacigo_dms_stage_20261020", settings: connectionSettings{databaseURL: "postgresql://espacigo_runtime@/espacigo_dms_stage_20261020?host=%2Fvar%2Frun%2Fpostgresql&port=5432&sslmode=disable", mode: "direct"}, variables: cleanEnv, osUser: "postgres", wantErr: true},
		{name: "set role forbidden", confirm: "espacigo_dms_stage_20261020", settings: connectionSettings{databaseURL: baseURL, mode: "direct", setRole: "espacigo_schema_owner"}, variables: cleanEnv, osUser: "postgres", wantErr: true},
		{name: "cloudsql mode forbidden", confirm: "espacigo_dms_stage_20261020", settings: connectionSettings{databaseURL: baseURL, mode: "cloudsql"}, variables: cleanEnv, osUser: "postgres", wantErr: true},
		{name: "non-postgres OS identity forbidden", confirm: "espacigo_dms_stage_20261020", settings: base, variables: cleanEnv, osUser: "nandev", wantErr: true},
		{name: "postgres password override forbidden", confirm: "espacigo_dms_stage_20261020", settings: base, variables: map[string]string{"PGPASSWORD": "present"}, osUser: "postgres", wantErr: true},
		{name: "postgres session options forbidden", confirm: "espacigo_dms_stage_20261020", settings: base, variables: map[string]string{"PGOPTIONS": "present"}, osUser: "postgres", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateNativeAdminRangeInvocation(tc.confirm, tc.settings, env(tc.variables), tc.osUser)
			if (err != nil) != tc.wantErr {
				t.Fatalf("validateNativeAdminRangeInvocation() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestValidateNativePreV81PlanAcceptsOnlyVerifiedPrefix(t *testing.T) {
	valid := func(appliedCount int) migrator.PlanResult {
		plan := migrator.PlanResult{
			Database:      nativeAdminConfirmation,
			ServerVersion: "18.6 (Ubuntu local test)",
			ReadOnly:      true,
			HistoryExists: true,
		}
		for version := int64(1); version <= int64(appliedCount); version++ {
			plan.Applied = append(plan.Applied, migrator.HistoryEntry{Version: version, Name: "migration", Checksum: "checksum"})
		}
		for version := int64(appliedCount + 1); version <= 85; version++ {
			plan.Pending = append(plan.Pending, migrator.MigrationSummary{Version: version, Name: "migration", Checksum: "checksum"})
		}
		return plan
	}
	for _, count := range []int{76, 77, 80} {
		if err := validateNativePreV81Plan(valid(count)); err != nil {
			t.Errorf("prefix through V%d should pass: %v", count, err)
		}
	}
	if err := validateNativeV80RoleTransitionPlan(valid(80)); err != nil {
		t.Errorf("exact V80 prefix should pass role-transition gate: %v", err)
	}
	if err := validateNativeV80RoleTransitionPlan(valid(76)); err == nil {
		t.Error("role-transition gate accepted an incomplete V76 prefix")
	}
	if err := validateNativeV81Plan(valid(80)); err != nil {
		t.Errorf("V81 preflight should accept exact V80 prefix: %v", err)
	}
	if err := validateNativeV81PostPlan(valid(81)); err != nil {
		t.Errorf("V81 postcheck should accept exact V81 prefix: %v", err)
	}
	if err := validateNativeV81PostPlan(valid(80)); err == nil {
		t.Error("V81 postcheck accepted a ledger that still has V81 pending")
	}
	tests := []struct {
		name   string
		mutate func(*migrator.PlanResult)
	}{
		{name: "wrong database", mutate: func(p *migrator.PlanResult) { p.Database = "espacigo_local" }},
		{name: "not read only", mutate: func(p *migrator.PlanResult) { p.ReadOnly = false }},
		{name: "wrong server version", mutate: func(p *migrator.PlanResult) { p.ServerVersion = "17.9" }},
		{name: "skipped applied version", mutate: func(p *migrator.PlanResult) { p.Applied[10].Version = 999 }},
		{name: "missing pending version", mutate: func(p *migrator.PlanResult) { p.Pending = p.Pending[1:] }},
		{name: "too-new prefix", mutate: func(p *migrator.PlanResult) { *p = valid(81) }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			plan := valid(76)
			tc.mutate(&plan)
			if err := validateNativePreV81Plan(plan); err == nil {
				t.Fatal("invalid native pre-V81 plan was accepted")
			}
		})
	}
}

func TestMakeNativePreV81SnapshotContainsExactlyReviewedPrefix(t *testing.T) {
	privateTemp := t.TempDir()
	if err := os.Chmod(privateTemp, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", privateTemp)
	source := filepath.Join("..", "..", "db", "migrations")
	all, err := migrator.DiscoverMigrations(source)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, cleanup, err := makeNativePreV81Snapshot(source)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	got, err := migrator.DiscoverMigrations(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) < 85 || len(got) != 80 {
		t.Fatalf("source/snapshot counts = %d/%d, want at least 85/80", len(all), len(got))
	}
	for i := range got {
		if got[i].Version != int64(i+1) || got[i].Name != all[i].Name || got[i].Checksum != all[i].Checksum {
			t.Fatalf("snapshot entry %d does not match source prefix", i+1)
		}
	}
	snapshot81, cleanup81, err := makeNativeV81Snapshot(source)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup81)
	got81, err := migrator.DiscoverMigrations(snapshot81)
	if err != nil || len(got81) != 81 {
		t.Fatalf("V81 snapshot has %d entries: %v", len(got81), err)
	}
	for i := range got81 {
		if got81[i].Version != int64(i+1) || got81[i].Name != all[i].Name || got81[i].Checksum != all[i].Checksum {
			t.Fatalf("V81 snapshot entry %d does not match source prefix", i+1)
		}
	}
}

func TestValidateNativeMigratorV81InvocationPinsPrivatePassfileAndTarget(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	passfile := filepath.Join(root, "pgpass")
	if err := os.WriteFile(passfile, []byte("127.0.0.1:5432:espacigo_dms_stage_20261020:espacigo_migrator_login:private\n"), 0600); err != nil {
		t.Fatal(err)
	}
	base := connectionSettings{
		databaseURL: "postgresql://espacigo_migrator_login@127.0.0.1:5432/espacigo_dms_stage_20261020?sslmode=disable",
		mode:        "direct",
		setRole:     "espacigo_schema_owner",
	}
	env := func(overrides map[string]string) func(string) string {
		return func(key string) string {
			if value, ok := overrides[key]; ok {
				return value
			}
			if key == "PGPASSFILE" {
				return passfile
			}
			if key == "TMPDIR" {
				return root
			}
			return ""
		}
	}
	if err := validateNativeMigratorV81Invocation(nativeAdminConfirmation, base, env(nil), "nandev", passfile); err != nil {
		t.Fatalf("exact native migrator configuration rejected: %v", err)
	}
	tests := []struct {
		name      string
		confirm   string
		settings  connectionSettings
		variables map[string]string
		osUser    string
		passfile  string
	}{
		{name: "wrong confirmation", confirm: "espacigo_local", settings: base, osUser: "nandev", passfile: passfile},
		{name: "wrong database", confirm: nativeAdminConfirmation, settings: connectionSettings{databaseURL: "postgresql://espacigo_migrator_login@127.0.0.1:5432/postgres?sslmode=disable", mode: "direct", setRole: "espacigo_schema_owner"}, osUser: "nandev", passfile: passfile},
		{name: "password embedded in URL", confirm: nativeAdminConfirmation, settings: connectionSettings{databaseURL: "postgresql://espacigo_migrator_login:bad@127.0.0.1:5432/espacigo_dms_stage_20261020?sslmode=disable", mode: "direct", setRole: "espacigo_schema_owner"}, osUser: "nandev", passfile: passfile},
		{name: "wrong set role", confirm: nativeAdminConfirmation, settings: connectionSettings{databaseURL: base.databaseURL, mode: "direct", setRole: "espacigo_runtime"}, osUser: "nandev", passfile: passfile},
		{name: "administrator process forbidden", confirm: nativeAdminConfirmation, settings: base, osUser: "postgres", passfile: passfile},
		{name: "credential override forbidden", confirm: nativeAdminConfirmation, settings: base, variables: map[string]string{"PGPASSWORD": "present"}, osUser: "nandev", passfile: passfile},
		{name: "wrong passfile", confirm: nativeAdminConfirmation, settings: base, osUser: "nandev", passfile: filepath.Join(root, "other")},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateNativeMigratorV81Invocation(tc.confirm, tc.settings, env(tc.variables), tc.osUser, tc.passfile); err == nil {
				t.Fatal("invalid native migrator configuration was accepted")
			}
		})
	}
}
