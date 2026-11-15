package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestNativeFixtureInventoryLimitPrecedesReceiptWrite(t *testing.T) {
	policy := map[string]any{"authorization": map[string]any{"account_inventory": []any{
		map[string]any{"account_id": "11111111-1111-4111-8111-111111111111", "verified": true},
		map[string]any{"account_id": "22222222-2222-4222-8222-222222222222", "verified": true},
	}}}
	var receipt nativeFixtureReceipt
	writes := 0
	err := persistNativeFixtureIdentity("receipt", "policy", &receipt, policy, "33333333-3333-4333-8333-333333333333", false, func(string, any) error { writes++; return nil })
	if err == nil || writes != 0 || receipt.AccountID != "" {
		t.Fatalf("inventory rejection must precede receipt mutation: error=%v writes=%d receipt_changed=%v", err, writes, receipt.AccountID != "")
	}
}

func TestNativeFixtureRecoversReceiptAfterPolicyWriteFailure(t *testing.T) {
	const id = "11111111-1111-4111-8111-111111111111"
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	receiptPath, policyPath := filepath.Join(dir, "receipt.json"), filepath.Join(dir, "policy.json")
	policy := map[string]any{"preserved": "unchanged", "authorization": map[string]any{"account_inventory": []any{}}}
	if err := writeNativeFixtureJSON(policyPath, policy); err != nil {
		t.Fatal(err)
	}
	receipt := nativeFixtureReceipt{RegistrationAttempted: true}
	err := persistNativeFixtureIdentity(receiptPath, policyPath, &receipt, policy, id, false, func(path string, value any) error {
		if path == policyPath {
			return errors.New("injected policy write failure")
		}
		return writeNativeFixtureJSON(path, value)
	})
	if err == nil {
		t.Fatal("injected policy failure was ignored")
	}
	// Reload as a new invocation would; do not reuse the already-mutated map.
	raw, err := os.ReadFile(receiptPath)
	if err != nil {
		t.Fatal(err)
	}
	var recovered nativeFixtureReceipt
	if json.Unmarshal(raw, &recovered) != nil || recovered.AccountID != id || recovered.Verified {
		t.Fatal("pending identity was not durably journaled")
	}
	raw, err = os.ReadFile(policyPath)
	if err != nil {
		t.Fatal(err)
	}
	var reloaded map[string]any
	if json.Unmarshal(raw, &reloaded) != nil {
		t.Fatal("old policy could not be reloaded")
	}
	if len(reloaded["authorization"].(map[string]any)["account_inventory"].([]any)) != 0 {
		t.Fatal("failed policy write changed the disk policy")
	}
	if err := validateNativeFixtureResume(recovered, id); err != nil {
		t.Fatal("own journaled account cannot be resumed")
	}
	if err := persistNativeFixtureIdentity(receiptPath, policyPath, &recovered, reloaded, id, false, writeNativeFixtureJSON); err != nil {
		t.Fatal(err)
	}
	if err := persistNativeFixtureIdentity(receiptPath, policyPath, &recovered, reloaded, id, true, writeNativeFixtureJSON); err != nil {
		t.Fatal(err)
	}
	raw, err = os.ReadFile(policyPath)
	if err != nil {
		t.Fatal(err)
	}
	if json.Unmarshal(raw, &reloaded) != nil {
		t.Fatal("repaired policy could not be reloaded")
	}
	entries := reloaded["authorization"].(map[string]any)["account_inventory"].([]any)
	if len(entries) != 1 || entries[0].(map[string]any)["account_id"] != id || entries[0].(map[string]any)["verified"] != true || reloaded["preserved"] != "unchanged" {
		t.Fatal("recovery duplicated or lost fixture authorization")
	}
}
