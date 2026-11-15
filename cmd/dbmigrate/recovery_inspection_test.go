package main

import "testing"

func TestRecoveryInspectionRejectsModesBeforeConnection(t *testing.T) {
	s := connectionSettings{mode: "cloudsql", instance: recoveryInspectionInstance, privateIP: true, setRole: "espacigo_schema_owner", passwordFile: "/var/secrets/migration/password", databaseURL: "postgres://espacigo_migrator_login@localhost:5432/espacigo_dms_stage_20261020?sslmode=disable"}
	getenv := func(string) string { return "" }
	for _, tc := range []struct {
		name                                                    string
		inventory, schema, plan, release, apply, recover, owner bool
		settings                                                connectionSettings
	}{
		{name: "offline", inventory: true}, {name: "schema", schema: true}, {name: "plan", plan: true}, {name: "release", release: true}, {name: "apply", apply: true}, {name: "recover", recover: true}, {name: "owner", owner: true}, {name: "wrong target", settings: connectionSettings{mode: "cloudsql", instance: "other:region:db", privateIP: true, databaseURL: s.databaseURL}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			use := s
			if tc.settings.mode != "" {
				use = tc.settings
			}
			if err := validateRecoveryInspectionInvocation(true, tc.inventory, tc.schema, tc.plan, tc.release, tc.apply, tc.recover, tc.owner, use, getenv); err == nil {
				t.Fatal("unsafe inspection invocation accepted")
			}
		})
	}
	if err := validateRecoveryInspectionInvocation(true, false, false, false, false, false, false, false, s, getenv); err != nil {
		t.Fatalf("valid invocation with approved Job secret mount rejected: %v", err)
	}
	for _, wrong := range []string{"/secrets/admin/password", "/tmp/unapproved-password-path"} {
		s.passwordFile = wrong
		if err := validateRecoveryInspectionInvocation(true, false, false, false, false, false, false, false, s, getenv); err == nil {
			t.Fatalf("accepted unapproved password mount %q", wrong)
		}
	}
}
func TestRecoveryInspectionRejectsPostgreSQLEnvironmentOverrides(t *testing.T) {
	settings := connectionSettings{mode: "cloudsql", instance: recoveryInspectionInstance, privateIP: true, setRole: "espacigo_schema_owner", passwordFile: "/var/secrets/migration/password", databaseURL: "postgres://espacigo_migrator_login@localhost:5432/espacigo_dms_stage_20261020?sslmode=disable"}
	if err := validateRecoveryInspectionInvocation(true, false, false, false, false, false, false, false, settings, func(key string) string {
		if key == "PGOPTIONS" {
			return "-c role=postgres"
		}
		return ""
	}); err == nil {
		t.Fatal("accepted a PostgreSQL session-option environment override")
	}
	settings.passwordFile = "/tmp/unapproved-password-path"
	if err := validateRecoveryInspectionInvocation(true, false, false, false, false, false, false, false, settings, func(string) string { return "" }); err == nil {
		t.Fatal("accepted a password path outside the mounted runtime secret")
	}
}

func TestRecoveryInspectionURLRejectsTargetAndOptionsOverrides(t *testing.T) {
	for _, raw := range []string{"postgres://espacigo_migrator_login@localhost:5432/other?sslmode=disable", "postgres://other@localhost:5432/espacigo_dms_stage_20261020?sslmode=disable", "postgres://espacigo_migrator_login@localhost:5432/espacigo_dms_stage_20261020?sslmode=disable&options=-c%20role=postgres"} {
		if err := validateRecoveryInspectionURL(raw); err == nil {
			t.Fatalf("accepted unsafe URL %q", raw)
		}
	}
}
