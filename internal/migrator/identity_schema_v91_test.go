package migrator

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestIdentityV91PredecessorMatchesObservedPostgres18Definition(t *testing.T) {
	// READ ONLY catalog observation from the authorized PostgreSQL 18.6 target.
	const observed = "CHECK (((declaration_id IS NOT NULL) OR (((status)::text = 'APPROVED'::text) AND (decision_key_version IS NULL) AND (decision_algorithm IS NULL) AND (encrypted_decision IS NULL))))"
	data, err := os.ReadFile(filepath.Join("..", "..", "db", "migrations", "V000091__identity_detached_terminal_withdrawal.sql"))
	if err != nil {
		t.Fatal(err)
	}
	match := regexp.MustCompile(`IF v_definition <> '((?:[^']|'')*)' THEN`).FindStringSubmatch(string(data))
	if len(match) != 2 {
		t.Fatal("missing exact predecessor guard")
	}
	want := strings.ReplaceAll(match[1], "''", "'")
	got := observed
	if got != want {
		t.Fatalf("V91 rejects the actual V90 PostgreSQL constraint: got %q expected %q", got, want)
	}
}

func TestIdentityV91DetachedTerminalMigrationContract(t *testing.T) {
	migrationPath := filepath.Join("..", "..", "db", "migrations", "V000091__identity_detached_terminal_withdrawal.sql")
	migration, err := os.ReadFile(migrationPath)
	if err != nil {
		t.Fatal(err)
	}
	sql := string(migration)
	for _, required := range []string{
		"session_user <> 'espacigo_migrator_login'",
		"current_user <> 'espacigo_schema_owner'",
		"current_schema() <> 'public'",
		"owner_role.rolname = 'espacigo_schema_owner'",
		"NOT r.rolcanlogin AND NOT r.rolsuper AND NOT r.rolinherit",
		"NOT r.rolcreaterole AND NOT r.rolcreatedb",
		"NOT r.rolreplication AND NOT r.rolbypassrls",
		"pg_get_constraintdef(c.oid)",
		"identity_provider_attempt_detached_approved_ck",
		"status IN ('APPROVED', 'REJECTED', 'EXPIRED')",
		"decision_key_version IS NULL",
		"decision_algorithm IS NULL",
		"encrypted_decision IS NULL",
	} {
		if !strings.Contains(sql, required) {
			t.Errorf("V91 migration is missing required contract %q", required)
		}
	}
	if strings.Contains(sql, "SECURITY DEFINER") || strings.Contains(sql, "GRANT ") || strings.Contains(sql, "REVOKE ") {
		t.Fatal("V91 must not add definer code or change privileges")
	}
	if strings.Contains(sql, "'FAILED'") || strings.Contains(sql, "PENDING_USER_INPUT") || strings.Contains(sql, "PROCESSING") || strings.Contains(sql, "IN_REVIEW") || strings.Contains(sql, "'CREATED'") {
		t.Fatal("V91 must not permit detached pending/failed states")
	}

	// Pin the application path that reaches the terminal status update after
	// V90 has detached declaration_id; PostgreSQL behavior is covered separately.
	repositoryPath := filepath.Join("..", "adapters", "postgres", "identitydeclaration", "provider_attempt.go")
	repository, err := os.ReadFile(repositoryPath)
	if err != nil {
		t.Fatal(err)
	}
	source := string(repository)
	methodStart := strings.Index(source, "func (r *Repository) UpdateAttemptStatusAndCompleteEvent(")
	if methodStart < 0 {
		t.Fatal("terminal attempt status repository method not found")
	}
	method := source[methodStart:]
	if !strings.Contains(method, "SET status = $2") || !strings.Contains(method, "($2 IN ('EXPIRED', 'REJECTED')") {
		t.Fatal("repository source no longer exposes the detached REJECTED/EXPIRED update path")
	}
}
