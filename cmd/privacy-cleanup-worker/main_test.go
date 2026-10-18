package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigAllowsOnlyTheDedicatedDatabaseRole(t *testing.T) {
	dir := t.TempDir()
	passwordPath := filepath.Join(dir, "password")
	if err := os.WriteFile(passwordPath, []byte("synthetic"), 0o600); err != nil {
		t.Fatal(err)
	}
	values := map[string]string{
		"DATABASE_HOST": "127.0.0.1", "DATABASE_NAME": "espacigo", "DATABASE_USER": "worker_login",
		"DATABASE_PASSWORD_FILE": passwordPath, "BLOB_STORE_BACKEND": "filesystem", "M03_EVIDENCE_DIR": dir,
	}
	getenv := func(key string) string { return values[key] }
	values["DATABASE_SET_ROLE"] = "espacigo_privacy_cleanup_worker"
	if _, err := loadConfig(getenv); err != nil {
		t.Fatalf("dedicated worker role rejected: %v", err)
	}
	values["DATABASE_SET_ROLE"] = "espacigo_runtime"
	if _, err := loadConfig(getenv); err == nil {
		t.Fatal("broad application role was accepted")
	}
}
