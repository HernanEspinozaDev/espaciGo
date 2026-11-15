package main

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadRentalContractDeclarationKeyringUsesConfiguredFile(t *testing.T) {
	root := t.TempDir()
	primary := filepath.Join(root, "primary.json")
	fallback := filepath.Join(root, "fallback.json")
	writeKeyring := func(path, id string, keyByte byte) {
		t.Helper()
		key := make([]byte, 32)
		for i := range key {
			key[i] = keyByte
		}
		data := []byte(`{"current":"` + id + `","keys":{"` + id + `":"` + hex.EncodeToString(key) + `"}}`)
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	writeKeyring(primary, "primary", 0x11)
	writeKeyring(fallback, "fallback", 0x22)
	getenv := func(name string) string {
		switch name {
		case "IDENTITY_DECLARATION_KEYRING_FILE":
			return primary
		case "LOCAL_IDENTITY_DECLARATION_KEYRING_FILE":
			return fallback
		default:
			return ""
		}
	}

	keyring, err := loadRentalContractDeclarationKeyring(getenv)
	if err != nil || keyring == nil {
		t.Fatalf("expected configured keyring, got keyring=%v err=%v", keyring, err)
	}
	version, _, err := keyring.Seal([]byte("aad"), []byte("synthetic"))
	if err != nil || version != "primary" {
		t.Fatalf("primary configured file not selected: version=%q err=%v", version, err)
	}
}

func TestLoadRentalContractDeclarationKeyringAllowsMissingConfiguration(t *testing.T) {
	keyring, err := loadRentalContractDeclarationKeyring(func(string) string { return "" })
	if err != nil || keyring != nil {
		t.Fatalf("missing configuration should fail closed without startup error: keyring=%v err=%v", keyring, err)
	}
}

func TestLoadRentalContractDeclarationKeyringRejectsInvalidFileWithoutLeakingPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "invalid.json")
	if err := os.WriteFile(path, []byte("not a keyring"), 0o600); err != nil {
		t.Fatal(err)
	}
	keyring, err := loadRentalContractDeclarationKeyring(func(string) string { return path })
	if err == nil || keyring != nil {
		t.Fatalf("invalid keyring was accepted: keyring=%v err=%v", keyring, err)
	}
	if got := err.Error(); got == "" || got == path || got == "not a keyring" {
		t.Fatalf("invalid keyring error disclosed configuration details: %q", got)
	}
}
