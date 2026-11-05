package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStandaloneCatalogKeyRequiresPrivateRegularStableFile(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "cursor-key")
	if err := os.WriteFile(p, []byte(strings.Repeat("ab", 32)), 0600); err != nil {
		t.Fatal(err)
	}
	key, err := localPublicCatalogKey(p)
	if err != nil || len(key) != 32 {
		t.Fatal("private stable key rejected")
	}
	link := filepath.Join(root, "link")
	if err = os.Symlink(p, link); err != nil {
		t.Fatal(err)
	}
	if _, err = localPublicCatalogKey(link); err == nil {
		t.Fatal("symlink key accepted")
	}
	if err = os.Chmod(p, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err = localPublicCatalogKey(p); err == nil {
		t.Fatal("public key file accepted")
	}
	if err = os.Chmod(p, 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(p, []byte("invalid"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = localPublicCatalogKey(p); err == nil {
		t.Fatal("invalid key accepted")
	}
	if _, err = localPublicCatalogKey("relative"); err == nil {
		t.Fatal("relative key accepted")
	}
}

func TestPublicCatalogStandaloneRequiresExactWorkerFreeTarget(t *testing.T) {
	base := map[string]string{"LOCAL_PUBLIC_CATALOG_TRIAL": "1", "LOCAL_AUTH_PROTOTYPE": "1", "HTTP_ADDR": "127.0.0.1:18082", "DATABASE_HOST": "127.0.0.1", "DATABASE_PORT": "15434", "DATABASE_NAME": "espacigo_f5_gate_20261101", "DATABASE_USER": "espacigo_api_login", "DATABASE_SET_ROLE": "espacigo_runtime"}
	check := func(env map[string]string, profile runtimeProfileName, want bool) {
		t.Helper()
		got, err := localPublicCatalogEnabled(func(k string) string { return env[k] }, profile)
		if got != want || (want && err != nil) {
			t.Fatalf("catalog scope mismatch: enabled=%t error=%t", got, err != nil)
		}
	}
	check(base, runtimeProfileWorkerFree, true)
	for _, profile := range []runtimeProfileName{runtimeProfileStandard, runtimeProfileDocumentScanOnly} {
		check(base, profile, false)
	}
	for key, value := range map[string]string{"LOCAL_PUBLIC_CATALOG_TRIAL": "true", "LOCAL_AUTH_PROTOTYPE": "", "HTTP_ADDR": "0.0.0.0:18082", "DATABASE_HOST": "172.28.0.2", "DATABASE_PORT": "5432", "DATABASE_NAME": "espacigo_local", "DATABASE_USER": "postgres", "DATABASE_SET_ROLE": "postgres", "LOCAL_BOOKING_TRIAL": "1", "APP_PROFILE": "cloud-test"} {
		env := map[string]string{}
		for k, v := range base {
			env[k] = v
		}
		env[key] = value
		check(env, runtimeProfileWorkerFree, false)
	}
	got, err := localPublicCatalogEnabled(func(string) string { return "" }, runtimeProfileWorkerFree)
	if got || err != nil {
		t.Fatal("standalone catalog enabled without explicit flag")
	}
}
