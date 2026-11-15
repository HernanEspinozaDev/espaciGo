package main

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func validateNativeIdentityAccountPair(policy []byte, primary, secondary string) error {
	if !canonicalNativeUUID(primary) || !canonicalNativeUUID(secondary) || primary == secondary {
		return errors.New("native identity isolation requires two distinct canonical fixture IDs")
	}
	if err := validateNativeIdentityPolicy(policy, primary); err != nil {
		return err
	}
	return validateNativeIdentityPolicy(policy, secondary)
}

func TestNativeIdentityAccountPairRejectsUnauthorizedSecondFixture(t *testing.T) {
	const primary = "11111111-1111-4111-8111-111111111111"
	const secondary = "22222222-2222-4222-8222-222222222222"
	makePolicy := func(verified bool) []byte {
		p := map[string]any{"target": map[string]any{"database": "espacigo_dms_stage_20261020", "host": "127.0.0.1", "port": 5432, "expected_database_oid": 16388}, "authorization": map[string]any{"maximum_new_accounts": 2, "synthetic_email_domain": "example.invalid", "synthetic_kyc_rows_write_update_delete": true, "account_inventory": []map[string]any{{"account_id": primary, "verified": true}, {"account_id": secondary, "verified": verified}}}}
		raw, err := json.Marshal(p)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	valid := makePolicy(true)
	var duplicatePolicy map[string]any
	if err := json.Unmarshal(valid, &duplicatePolicy); err != nil {
		t.Fatal(err)
	}
	duplicatePolicy["authorization"].(map[string]any)["account_inventory"] = []map[string]any{{"account_id": primary, "verified": true}, {"account_id": primary, "verified": true}}
	duplicateRaw, err := json.Marshal(duplicatePolicy)
	if err != nil {
		t.Fatal(err)
	}
	if validateNativeIdentityAccountPair(duplicateRaw, primary, secondary) == nil {
		t.Fatal("duplicate inventory authorized an absent second account")
	}
	if err := validateNativeIdentityAccountPair(valid, primary, secondary); err != nil {
		t.Fatal("valid pair rejected")
	}
	for _, tc := range []struct {
		name, other string
		policy      []byte
	}{
		{"missing", "", valid}, {"same", primary, valid}, {"malformed", "not-a-uuid", valid},
		{"unlisted", "33333333-3333-4333-8333-333333333333", valid}, {"unverified", secondary, makePolicy(false)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if validateNativeIdentityAccountPair(tc.policy, primary, tc.other) == nil {
				t.Fatal("unauthorized second fixture accepted")
			}
		})
	}
}

func TestNativeIdentityPolicyRejectsDuplicateInventory(t *testing.T) {
	const primary = "11111111-1111-4111-8111-111111111111"
	p := map[string]any{"target": map[string]any{"database": "espacigo_dms_stage_20261020", "host": "127.0.0.1", "port": 5432, "expected_database_oid": 16388}, "authorization": map[string]any{"maximum_new_accounts": 2, "synthetic_email_domain": "example.invalid", "synthetic_kyc_rows_write_update_delete": true, "account_inventory": []map[string]any{{"account_id": primary, "verified": true}, {"account_id": primary, "verified": true}}}}
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	if validateNativeIdentityPolicy(raw, primary) == nil {
		t.Fatal("single-account policy validator accepted duplicated inventory")
	}
}

func TestNativeFixtureInvalidSlotRejectedBeforePrivatePolicy(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "cmd", "api")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestNativeIdentityPrepareAuthorizedFixture$", "-test.count=1")
	cmd.Dir = dir
	cmd.Env = []string{"IDENTITY_DECLARATION_NATIVE_PREPARE_FIXTURE=1", "IDENTITY_DECLARATION_NATIVE_FIXTURE_SLOT=../blocked"}
	out, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(out), "native fixture slot must be") {
		t.Fatal("invalid slot was not rejected before private policy access")
	}
}
