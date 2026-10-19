package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCompileQueryPinsReadOnlyCatalogAllowlist(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("..", "..", "db", "query", "gcp04_w8_catalog_probe.sql"))
	if err != nil {
		t.Fatal(err)
	}
	query, err := compileQuery(source)
	if err != nil {
		t.Fatal(err)
	}
	if query == "" {
		t.Fatal("compiled SQL was empty")
	}
	for _, forbidden := range []string{"pg_settings", "pg_stat_", "pg_authid", "pg_shadow", "INSERT ", "UPDATE ", "DELETE ", "CREATE ", "ALTER ", "DROP "} {
		if containsFold(query, forbidden) {
			t.Fatalf("SQL includes forbidden token %q", forbidden)
		}
	}
	if _, err := compileQuery(append(source, []byte("-- unreviewed mutation\n")...)); err == nil {
		t.Fatal("changed SQL passed the pinned digest")
	}
}

func TestConfigPinsIdentityAndNumericSecretVersion(t *testing.T) {
	c := config{project: projectID, connection: connectionName, ip: privateIP, database: databaseName,
		user: databaseUser, passwordFile: "/var/secrets/password", secretVersion: "7", queryFile: queryPath, timeout: 90 * 1000000000}
	if err := c.validate(); err != nil {
		t.Fatal(err)
	}
	c.secretVersion = "latest"
	if err := c.validate(); err == nil {
		t.Fatal("latest secret alias passed")
	}
	c.secretVersion = "7"
	c.ip = "10.189.0.4"
	if err := c.validate(); err == nil {
		t.Fatal("unreviewed target IP passed")
	}
}

func TestPasswordFileMustBeProtectedRegularFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "password")
	if err := os.WriteFile(path, []byte("synthetic-only"), 0600); err != nil {
		t.Fatal(err)
	}
	if got, err := readPassword(path); err != nil || got != "synthetic-only" {
		t.Fatalf("protected file failed: %v", err)
	}
	if err := os.Chmod(path, 0660); err != nil {
		t.Fatal(err)
	}
	if _, err := readPassword(path); err == nil {
		t.Fatal("group-writable credential file passed")
	}
}

func containsFold(value, part string) bool {
	for i := 0; i+len(part) <= len(value); i++ {
		match := true
		for j := range part {
			left, right := value[i+j], part[j]
			if left >= 'A' && left <= 'Z' {
				left += 'a' - 'A'
			}
			if right >= 'A' && right <= 'Z' {
				right += 'a' - 'A'
			}
			if left != right {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
