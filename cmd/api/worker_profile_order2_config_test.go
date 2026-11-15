package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSpacesOrder2RuntimeConfigUsesPrivatePassfile(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	root, err := os.MkdirTemp(home, ".spaces-order2-passfile-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(root); err != nil {
			t.Errorf("remove temporary passfile fixture: %v", err)
		}
	})
	dir := filepath.Join(root, "private")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	passfile := filepath.Join(dir, "pgpass")
	if err := os.WriteFile(passfile, []byte("127.0.0.1:15440:espacigo_spaces_order2_20261113:espacigo_api_login:synthetic-only\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validatePrivatePGPassfile(passfile); err != nil {
		t.Fatalf("validate private passfile: %v", err)
	}
	values := map[string]string{
		"HTTP_ADDR": "127.0.0.1:18088", "DATABASE_CONNECTION_MODE": "direct",
		"DATABASE_HOST": "127.0.0.1", "DATABASE_PORT": "15440",
		"DATABASE_NAME": "espacigo_spaces_order2_20261113", "DATABASE_USER": "espacigo_api_login",
		"DATABASE_SET_ROLE": "espacigo_runtime", "PGPASSFILE": passfile,
	}
	cfg, err := loadRuntimeConfig(func(key string) string { return values[key] }, runtimeProfileSpacesOrder2)
	if err != nil {
		t.Fatalf("load order-2 runtime config: %v", err)
	}
	t.Cleanup(cfg.close)
	if cfg.httpAddr != "127.0.0.1:18088" || cfg.dbHost != "127.0.0.1" || cfg.dbPort != 15440 || cfg.dbName != "espacigo_spaces_order2_20261113" || cfg.dbUser != "espacigo_api_login" || cfg.dbSetRole != "espacigo_runtime" {
		t.Fatal("order-2 runtime config did not preserve the approved local target")
	}
}
