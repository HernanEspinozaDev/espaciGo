package migrator

import (
	"encoding/json"
	"testing"
)

func TestMigrationAuthorityReportsPreserveTypedWireEvidence(t *testing.T) {
	input, err := json.Marshal(map[string]any{
		"ddl_authority":          validV84DDLAuthority(),
		"identity_ddl_authority": validIdentityTailDDLAuthority(),
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		report any
	}{
		{"plan", &PlanResult{}},
		{"release", &ReleaseResult{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := json.Unmarshal(input, tc.report); err != nil {
				t.Fatal(err)
			}
			encoded, err := json.Marshal(tc.report)
			if err != nil {
				t.Fatal(err)
			}
			var actual map[string]json.RawMessage
			if err := json.Unmarshal(encoded, &actual); err != nil {
				t.Fatal(err)
			}
			var expected map[string]json.RawMessage
			if err := json.Unmarshal(input, &expected); err != nil {
				t.Fatal(err)
			}
			for _, key := range []string{"ddl_authority", "identity_ddl_authority"} {
				if string(actual[key]) != string(expected[key]) {
					t.Fatalf("%s authority evidence was dropped or changed", key)
				}
			}
		})
	}
}

func TestMigrationAuthorityReportsOmitUncollectedEvidence(t *testing.T) {
	for _, report := range []any{PlanResult{}, ReleaseResult{}} {
		encoded, err := json.Marshal(report)
		if err != nil {
			t.Fatal(err)
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(encoded, &fields); err != nil {
			t.Fatal(err)
		}
		for _, key := range []string{"ddl_authority", "identity_ddl_authority"} {
			if _, exists := fields[key]; exists {
				t.Fatalf("uncollected %s should not be serialized", key)
			}
		}
	}
}
