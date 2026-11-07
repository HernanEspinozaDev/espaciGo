package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/HernanEspinozaDev/espaciGo/internal/migrator"
)

// wizardAdminTestFixture builds an isolated, synthetic private configuration.
func wizardAdminTestFixture(t *testing.T, sourceRoot string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "db", "migrations"), 0700); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(sourceRoot, "db", "migrations"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if filepath.Ext(name) != ".sql" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(sourceRoot, "db", "migrations", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "db", "migrations", name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	migrations, err := migrator.DiscoverMigrations(filepath.Join(root, "db", "migrations"))
	if err != nil {
		t.Fatal(err)
	}
	manifest := make([]struct {
		File    string `json:"file"`
		SHA256  string `json:"sha256"`
		Version int64  `json:"version"`
	}, len(migrations))
	for i, migration := range migrations {
		manifest[i].File = filepath.Base(migration.Path)
		manifest[i].SHA256 = migration.Checksum
		manifest[i].Version = migration.Version
	}
	local := filepath.Join(root, ".local", "testing", "wizard-improvements")
	if err := os.MkdirAll(filepath.Join(local, "postgres"), 0700); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(local, "reviewed-full-83-manifest.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(local, "postgres", "pgpass"), []byte("# synthetic fixture; no credentials\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	return root
}
