package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/HernanEspinozaDev/espaciGo/internal/migrator"
)

func TestStageSpacesOrder2Snapshot(t *testing.T) {
	source := filepath.Join("..", "..", "db", "migrations")
	original, err := migrator.DiscoverMigrations(source)
	if err != nil {
		t.Fatal(err)
	}
	root, cleanup, err := stageSpacesOrder2Snapshot(source)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	info, err := os.Stat(root)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0700 {
		t.Fatalf("directory permissions = %o, want 700", info.Mode().Perm())
	}
	staged, err := migrator.DiscoverMigrations(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(staged) != 61 {
		t.Fatalf("staged count = %d, want 61", len(staged))
	}
	for i, m := range staged {
		if m.Name != original[i].Name || m.Checksum != original[i].Checksum || string(m.SQL) != string(original[i].SQL) {
			t.Fatalf("staged migration %d differs from original", i+1)
		}
		info, err := os.Stat(m.Path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0600 {
			t.Fatalf("file permissions = %o, want 600", info.Mode().Perm())
		}
	}
	cleanup()
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatalf("snapshot still exists after cleanup: %v", err)
	}
}

func TestStageSpacesOrder2SnapshotRejectsInventoryDrift(t *testing.T) {
	fixture := t.TempDir()
	source, err := filepath.Abs(filepath.Join("..", "..", "db", "migrations"))
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(source)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(source, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(fixture, entry.Name()), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	first := filepath.Join(fixture, entries[0].Name())
	data, err := os.ReadFile(first)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(first, append(data, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
	root, cleanup, err := stageSpacesOrder2Snapshot(fixture)
	if err == nil {
		cleanup()
		t.Fatal("inventory drift was accepted")
	}
	if root != "" {
		t.Fatalf("snapshot returned on drift: %q", root)
	}
}
