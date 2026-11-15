package migrator

import (
	"fmt"
	"strings"
	"testing"
)

type fixtureRoleReceipt struct {
	name string
	oid  int64
}

func fixtureRoleNames() []string {
	return []string{"espacigo_runtime", "espacigo_api_login", "espacigo_migrator_login", "espacigo_schema_owner", "espacigo_document_verification_fn", "espacigo_privacy_cleanup_worker", "espacigo_privacy_worker_login"}
}

// Missing and unexpected names are unsafe; every receipt and observation must
// have a positive OID and exact name-to-OID identity.
func validateFixtureRoleReceipts(owned, observed []fixtureRoleReceipt) error {
	if len(owned) != len(observed) {
		return fmt.Errorf("role count mismatch: owned=%d observed=%d", len(owned), len(observed))
	}
	expected := make(map[string]int64, len(owned))
	for _, r := range owned {
		if r.name == "" || r.oid <= 0 {
			return fmt.Errorf("invalid owned receipt")
		}
		if _, ok := expected[r.name]; ok {
			return fmt.Errorf("duplicate owned receipt name %q", r.name)
		}
		expected[r.name] = r.oid
	}
	for _, r := range observed {
		oid, ok := expected[r.name]
		if !ok {
			return fmt.Errorf("unexpected observed role %q", r.name)
		}
		if r.oid <= 0 || oid != r.oid {
			return fmt.Errorf("OID mismatch for role %q", r.name)
		}
		delete(expected, r.name)
	}
	if len(expected) != 0 {
		return fmt.Errorf("missing owned roles")
	}
	return nil
}

func TestValidateFixtureRoleReceipts(t *testing.T) {
	owned := []fixtureRoleReceipt{{name: "a", oid: 10}, {name: "b", oid: 20}}
	cases := []struct {
		name     string
		observed []fixtureRoleReceipt
		wantErr  bool
	}{
		{"matching", []fixtureRoleReceipt{{"a", 10}, {"b", 20}}, false},
		{"reordered", []fixtureRoleReceipt{{"b", 20}, {"a", 10}}, false},
		{"missing", []fixtureRoleReceipt{{"a", 10}}, true},
		{"extra", []fixtureRoleReceipt{{"a", 10}, {"b", 20}, {"c", 30}}, true},
		{"replaced", []fixtureRoleReceipt{{"a", 10}, {"b", 21}}, true},
		{"duplicate observed name", []fixtureRoleReceipt{{"a", 10}, {"a", 10}}, true},
		{"zero observed OID", []fixtureRoleReceipt{{"a", 10}, {"b", 0}}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateFixtureRoleReceipts(owned, tc.observed)
			if (err != nil) != tc.wantErr {
				t.Fatalf("validateFixtureRoleReceipts() error=%v, wantErr=%v", err, tc.wantErr)
			}
		})
	}
	for _, tc := range []struct {
		name, message string
		owned         []fixtureRoleReceipt
	}{
		{"zero owned OID", "invalid owned receipt", []fixtureRoleReceipt{{"a", 0}, {"b", 20}}},
		{"negative owned OID", "invalid owned receipt", []fixtureRoleReceipt{{"a", -1}, {"b", 20}}},
		{"empty owned name", "invalid owned receipt", []fixtureRoleReceipt{{"", 10}, {"b", 20}}},
		{"duplicate owned name", "duplicate owned receipt", []fixtureRoleReceipt{{"a", 10}, {"a", 10}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := validateFixtureRoleReceipts(tc.owned, owned)
			if err == nil || !strings.Contains(err.Error(), tc.message) {
				t.Fatalf("invalid owned receipt did not reach expected guard: %v", err)
			}
		})
	}
}
