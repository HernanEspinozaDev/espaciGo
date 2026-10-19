package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
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
	if containsFold(query, "BEGIN READ ONLY") || containsFold(query, "COMMIT") || containsFold(query, "BEGIN") {
		t.Fatal("transaction wrapper was returned to pgx as part of the prepared query")
	}
	if query[len(query)-1] == ';' || containsFold(query, ";") {
		t.Fatal("compiled query must be one semicolon-free SELECT statement")
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

func TestLocalPostgres18GoRunnerPath(t *testing.T) {
	dsn := os.Getenv("GCP04_W8_TEST_DSN")
	if dsn == "" {
		t.Skip("requires the pinned isolated PostgreSQL 18 W8 fixture")
	}
	queryFile := os.Getenv("GCP04_W8_TEST_QUERY_FILE")
	if queryFile == "" {
		t.Fatal("GCP04_W8_TEST_QUERY_FILE is required with the local fixture DSN")
	}
	source, err := os.ReadFile(queryFile)
	if err != nil {
		t.Fatal(err)
	}
	query, err := compileQuery(source)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("local dedicated login connection failed: %v", err)
	}
	defer conn.Close(context.Background())
	var database, login string
	if err := conn.QueryRow(ctx, "SELECT current_database(), current_user").Scan(&database, &login); err != nil {
		t.Fatal(err)
	}
	if database != databaseName || login != databaseUser {
		t.Fatalf("fixture connected to unexpected identity %q/%q", database, login)
	}
	tx, err := conn.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := queryReport(ctx, tx, query)
	if err != nil {
		_ = tx.Rollback(context.Background())
		t.Fatalf("single-statement pgx catalog query failed: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Target struct {
			TransactionReadOnly bool `json:"transaction_read_only"`
		} `json:"target"`
		Roles []struct {
			Name string `json:"name"`
		} `json:"roles"`
		Memberships []struct {
			GrantedRole string `json:"granted_role"`
			MemberRole  string `json:"member_role"`
		} `json:"memberships"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if !decoded.Target.TransactionReadOnly {
		t.Fatal("catalog query did not observe a read-only transaction")
	}
	foundRole, foundMembership := false, false
	for _, role := range decoded.Roles {
		foundRole = foundRole || role.Name == "pg_read_all_settings"
	}
	for _, membership := range decoded.Memberships {
		foundMembership = foundMembership || membership.GrantedRole == "pg_read_all_settings" && membership.MemberRole == databaseUser
	}
	if !foundRole || !foundMembership {
		t.Fatalf("dedicated login/explicit candidate membership absent in local report: role=%v membership=%v", foundRole, foundMembership)
	}
}

func TestConfigPinsIdentityAndNumericSecretVersion(t *testing.T) {
	c := config{project: projectID, connection: connectionName, ip: privateIP, database: databaseName,
		user: databaseUser, passwordFile: "/var/secrets/password", secretVersion: "7", queryFile: queryPath, timeout: 90 * time.Second}
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
