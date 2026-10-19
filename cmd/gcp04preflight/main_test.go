package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
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

func TestValidateRemoteAddressPinsObservedPrivateIP(t *testing.T) {
	got, err := validateRemoteAddress(&net.TCPAddr{IP: net.ParseIP(expectedPrivateIP), Port: 5432}, expectedPrivateIP)
	if err != nil || got != expectedPrivateIP {
		t.Fatalf("pinned connector address rejected: ip=%q err=%v", got, err)
	}
	if _, err := validateRemoteAddress(&net.TCPAddr{IP: net.ParseIP("10.189.0.4"), Port: 5432}, expectedPrivateIP); err == nil {
		t.Fatal("connector address mismatch passed")
	} else {
		var mismatch *remoteAddressMismatchError
		if !errors.As(err, &mismatch) || mismatch.Expected != expectedPrivateIP || mismatch.Observed != "10.189.0.4" {
			t.Fatalf("connector mismatch evidence is incomplete: %#v", err)
		}
	}
	if _, err := validateRemoteAddress(&net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 5432}, expectedPrivateIP); err == nil {
		t.Fatal("loopback connector address passed")
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
	localConfig := config{Database: databaseName, DatabaseUser: databaseUser, RequiredConnections: required}
	validReport, err := validateReport(report, localConfig)
	if err != nil || !validReport.ObserverContractPass || !validReport.CapacityPass {
		t.Fatalf("legitimate local observer report was not accepted: validation=%+v err=%v", validReport, err)
	}
	var mutated map[string]any
	if err := json.Unmarshal(report, &mutated); err != nil {
		t.Fatal(err)
	}
	delete(mutated, "destination")
	incomplete, _ := json.Marshal(mutated)
	if _, err := validateReport(incomplete, localConfig); err == nil {
		t.Fatal("inventory without destination details passed schema validation")
	}
	if err := json.Unmarshal(report, &mutated); err != nil {
		t.Fatal(err)
	}
	delete(mutated, "owner_effective_privileges")
	incomplete, _ = json.Marshal(mutated)
	if _, err := validateReport(incomplete, localConfig); err == nil {
		t.Fatal("inventory without effective ACL details passed schema validation")
	}
	if err := json.Unmarshal(report, &mutated); err != nil {
		t.Fatal(err)
	}
	roles := mutated["roles"].([]any)
	for _, item := range roles {
		role := item.(map[string]any)
		if role["name"] == databaseUser {
			role["createdb"] = true
		}
	}
	overprivileged, _ := json.Marshal(mutated)
	validation, err := validateReport(overprivileged, localConfig)
	if err != nil || validation.ObserverContractPass {
		t.Fatalf("overprivileged observer passed: validation=%+v err=%v", validation, err)
	}
	if err := json.Unmarshal(report, &mutated); err != nil {
		t.Fatal(err)
	}
	memberships := mutated["memberships"].([]any)
	memberships = append(memberships, map[string]any{"app_login": databaseUser, "granted_role": "cloudsqlsuperuser", "member_role": databaseUser, "admin_option": false, "inherit_option": true, "set_option": false, "edge_context": "transitive-login-membership"})
	mutated["memberships"] = memberships
	overprivilegedMembership, _ := json.Marshal(mutated)
	validation, err = validateReport(overprivilegedMembership, localConfig)
	if err != nil || validation.ObserverContractPass {
		t.Fatalf("observer with unexpected administrative membership passed: validation=%+v err=%v", validation, err)
	}
	if err := json.Unmarshal(report, &mutated); err != nil {
		t.Fatal(err)
	}
	capacity := mutated["capacity"].(map[string]any)
	capacity["free_slots_lower_bound"] = capacity["free_slots_lower_bound"].(float64) + 1
	inconsistentCapacity, _ := json.Marshal(mutated)
	if _, err := validateReport(inconsistentCapacity, localConfig); err == nil {
		t.Fatal("internally inconsistent connection capacity passed")
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
