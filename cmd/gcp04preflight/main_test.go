package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestConfigRequiresPinnedIdentityBudgetAndSingleConcurrency(t *testing.T) {
	cfg := config{
		ProjectID: expectedProject, ConnectionName: expectedConnectionName, ExpectedPrivateIP: expectedPrivateIP,
		Database: expectedDatabase, DatabaseUser: expectedDatabaseUser, PasswordFile: "/mounted/password",
		SecretVersion: "12", RequiredConnections: 24, Timeout: 2 * time.Minute, Concurrency: 1,
		QueryFile: "/target_preflight.sql",
	}
	if err := cfg.validate(); err != nil {
		t.Fatalf("valid target rejected: %v", err)
	}
	cfg.ConnectionName = "other-project:southamerica-west1:espacigo-testprod-pg"
	if err := cfg.validate(); err == nil {
		t.Fatal("connection name from another project passed")
	}
	cfg.ConnectionName = expectedConnectionName
	cfg.Concurrency = 2
	if err := cfg.validate(); err == nil {
		t.Fatal("concurrency above one passed")
	}
	cfg.Concurrency = 1
	cfg.SecretVersion = "latest"
	if err := cfg.validate(); err == nil {
		t.Fatal("non-numeric secret version passed")
	}
}

func TestCompileQueryPinsAndTranslatesPsqlWrapper(t *testing.T) {
	queryPath := filepath.Join("..", "..", "db", "query", "gcp04_w4_target_preflight.sql")
	source, err := os.ReadFile(queryPath)
	if err != nil {
		t.Fatal(err)
	}
	query, digest, err := compileQuery(source)
	if err != nil {
		t.Fatal(err)
	}
	if digest != expectedQuerySHA256 {
		t.Fatalf("query digest = %s", digest)
	}
	if got := countSubstrings(query, "$1"); got != 3 {
		t.Fatalf("query parameter count = %d, want 3", got)
	}
	if got := countSubstrings(query, "\\"); got != 0 {
		t.Fatalf("query retained %d psql meta commands", got)
	}
	if _, _, err := compileQuery(append(source, []byte("-- unreviewed change\n")...)); err == nil {
		t.Fatal("changed query passed its pinned digest")
	}
}

func TestPasswordFileRequiresProtectedRegularFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "password")
	if err := os.WriteFile(path, []byte("synthetic-test-password\n"), 0600); err != nil {
		t.Fatal(err)
	}
	password, err := readPasswordFile(path)
	if err != nil || password != "synthetic-test-password" {
		t.Fatalf("protected password file rejected: %v", err)
	}
	if err := os.Chmod(path, 0664); err != nil {
		t.Fatal(err)
	}
	if _, err := readPasswordFile(path); err == nil {
		t.Fatal("group/world-writable password file passed")
	}
}

func TestLocalObserverReadOnlyIntegration(t *testing.T) {
	dsn := os.Getenv("GCP04_W7_TEST_DSN")
	if dsn == "" {
		t.Skip("requires the dedicated local PostgreSQL observer fixture")
	}
	queryPath := os.Getenv("GCP04_W7_TEST_QUERY_FILE")
	if queryPath == "" {
		t.Fatal("GCP04_W7_TEST_QUERY_FILE is required with the local DSN")
	}
	required := 1
	if value := os.Getenv("GCP04_W7_TEST_REQUIRED_CONNECTIONS"); value != "" {
		if _, err := fmt.Sscanf(value, "%d", &required); err != nil || required < 1 {
			t.Fatal("local integration budget must be a positive integer")
		}
	}
	source, err := os.ReadFile(queryPath)
	if err != nil {
		t.Fatal(err)
	}
	query, _, err := compileQuery(source)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("local observer connection failed: %v", err)
	}
	defer conn.Close(context.Background())
	var databaseName, databaseUser string
	if err := conn.QueryRow(ctx, "SELECT current_database(), current_user").Scan(&databaseName, &databaseUser); err != nil {
		t.Fatal(err)
	}
	if databaseName != expectedDatabase || databaseUser != "espacigo_observer_login" {
		t.Fatalf("local fixture connected as %q/%q instead of the dedicated read-only observer", databaseName, databaseUser)
	}
	report, gateFailed, err := runForLocalIntegration(ctx, conn, query, required, databaseName, databaseUser)
	if err != nil {
		t.Fatalf("local observer preflight failed: %v", err)
	}
	if gateFailed {
		t.Fatalf("local fixture rejected the selected test budget: %s", report)
	}
	var decoded map[string]any
	if err := json.Unmarshal(report, &decoded); err != nil || decoded["database"] == nil {
		t.Fatalf("local report is not a database inventory: %v", err)
	}
}

func countSubstrings(value, substring string) int {
	count := 0
	for len(value) > 0 {
		index := strings.Index(value, substring)
		if index < 0 {
			break
		}
		count++
		value = value[index+len(substring):]
	}
	return count
}
