package gcp03scratch

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func historicalMigrationDirectory(t *testing.T) string {
	t.Helper()
	root := filepath.Clean(filepath.Join(filepath.Dir(sourceFile(t)), "..", ".."))
	source := filepath.Join(root, "db", "migrations")
	destination := t.TempDir()
	for version := 1; version <= MigrationCount; version++ {
		pattern := filepath.Join(source, fmt.Sprintf("V%06d__*.sql", version))
		matches, err := filepath.Glob(pattern)
		if err != nil || len(matches) != 1 {
			t.Fatalf("historical migration V%d count=%d err=%v", version, len(matches), err)
		}
		info, err := os.Lstat(matches[0])
		if err != nil || !info.Mode().IsRegular() {
			t.Fatalf("historical migration V%d is not a regular file: %v", version, err)
		}
		data, err := os.ReadFile(matches[0])
		if err != nil {
			t.Fatalf("read historical migration V%d: %v", version, err)
		}
		if err := os.WriteFile(filepath.Join(destination, filepath.Base(matches[0])), data, 0o600); err != nil {
			t.Fatalf("copy historical migration V%d: %v", version, err)
		}
	}
	return destination
}
