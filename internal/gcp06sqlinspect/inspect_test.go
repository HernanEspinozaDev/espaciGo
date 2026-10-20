package gcp06sqlinspect

import (
	"strings"
	"testing"
)

func TestValidateTargetPinsCandidateAndOneRunLogin(t *testing.T) {
	good := "gcp06_inspect_012345abcdef"
	if err := ValidateTarget(RunID, ConnectionName, DatabaseName, good); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, run, connection, database, login string
	}{
		{"wrong run", "other", ConnectionName, DatabaseName, good},
		{"wrong instance", RunID, "other", DatabaseName, good},
		{"wrong database", RunID, ConnectionName, "espacigo", good},
		{"wrong login prefix", RunID, ConnectionName, DatabaseName, "gcp06_admin_012345abcdef"},
		{"short login", RunID, ConnectionName, DatabaseName, "gcp06_inspect_123"},
		{"uppercase login", RunID, ConnectionName, DatabaseName, "gcp06_inspect_012345ABCDEF"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateTarget(tc.run, tc.connection, tc.database, tc.login); err == nil {
				t.Fatal("non-candidate target was accepted")
			}
		})
	}
}

func TestInspectorSQLIsFixedReadOnlyCatalogAllowlist(t *testing.T) {
	statements := AllowedStatements()
	if len(statements) != 6 {
		t.Fatalf("expected six fixed catalog statements; got %d", len(statements))
	}
	for i, statement := range statements {
		upper := strings.ToUpper(strings.TrimSpace(statement))
		if !strings.HasPrefix(upper, "SELECT") {
			t.Fatalf("statement %d is not a SELECT", i)
		}
		for _, forbidden := range []string{" INSERT ", " UPDATE ", " DELETE ", " TRUNCATE ", " ALTER ", " DROP ", " CREATE ", " COPY ", " CALL ", " DO "} {
			if strings.Contains(" "+upper+" ", forbidden) {
				t.Fatalf("statement %d includes a forbidden SQL verb", i)
			}
		}
	}
	joined := strings.Join(statements, "\n")
	for _, allowed := range []string{"pg_catalog.pg_roles", "pg_catalog.pg_auth_members", "pg_catalog.pg_extension", "pg_catalog.pg_class", "to_regclass('public.schema_migrations')", "has_table_privilege"} {
		if !strings.Contains(joined, allowed) {
			t.Fatalf("allowlist is missing catalog check %q", allowed)
		}
	}
	for _, forbidden := range []string{"FROM public.usuario", "FROM public.schema_migrations", "password", "correo", "token"} {
		if strings.Contains(strings.ToLower(joined), strings.ToLower(forbidden)) {
			t.Fatalf("allowlist includes application rows or sensitive columns: %q", forbidden)
		}
	}
}
