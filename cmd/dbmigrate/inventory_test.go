package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOfflineInventoryHashesExactBytesWithoutConnectionSettings(t *testing.T) {
	t.Setenv("DATABASE_URL", "invalid-and-must-not-be-used")
	t.Setenv("DATABASE_PASSWORD_FILE", "/missing-and-must-not-be-read")
	dir := t.TempDir()
	const sql = "SELECT 1;\n"
	if err := os.WriteFile(filepath.Join(dir, "V000001__probe.sql"), []byte(sql), 0600); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := printOfflineInventory(&output, dir, false); err != nil {
		t.Fatal(err)
	}
	var report offlineInventory
	if err := json.Unmarshal(output.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(sql))
	want := hex.EncodeToString(sum[:])
	if report.Format != "espacigo.migrations.inventory.v1" || len(report.Migrations) != 1 || report.Migrations[0].Checksum != want || report.Migrations[0].Version != 1 {
		t.Fatalf("unexpected inventory: %#v", report)
	}
	aggregate := sha256.Sum256([]byte(want + "  V000001__probe.sql\n"))
	if report.InventorySHA256 != hex.EncodeToString(aggregate[:]) {
		t.Fatal("inventory fingerprint does not describe the exact ordered SQL bytes")
	}
}

func TestOfflineInventoryFailsClosed(t *testing.T) {
	for _, fixture := range []struct {
		name     string
		file     string
		conflict bool
	}{
		{"empty", "", false},
		{"gap", "V000002__gap.sql", false},
		{"bad-name", "unsafe.sql", false},
		{"conflicting-mode", "V000001__probe.sql", true},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			dir := t.TempDir()
			if fixture.file != "" {
				if err := os.WriteFile(filepath.Join(dir, fixture.file), []byte("SELECT 1;"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			var output bytes.Buffer
			if err := printOfflineInventory(&output, dir, fixture.conflict); err == nil || strings.TrimSpace(output.String()) != "" {
				t.Fatal("invalid inventory produced a successful report")
			}
		})
	}
}
