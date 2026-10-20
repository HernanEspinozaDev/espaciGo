package gcp06sqlinspect

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
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

func TestCatalogPermissionQueriesExecuteOnDisposablePostgres18(t *testing.T) {
	dsn := os.Getenv("GCP06_INSPECTOR_SQL_TEST_DSN")
	if dsn == "" {
		if os.Getenv("GCP06_INSPECTOR_SQL_TEST_REQUIRED") == "1" {
			t.Fatal("GCP06_INSPECTOR_SQL_TEST_DSN is required for this PostgreSQL 18 integration run")
		}
		t.Skip("set GCP06_INSPECTOR_SQL_TEST_DSN only for an isolated PostgreSQL 18 test database")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal("could not connect to disposable PostgreSQL test database")
	}
	defer conn.Close(context.Background())
	tx, err := conn.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		t.Fatal("could not start read-only test transaction")
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	var version int
	var readOnly bool
	if err := tx.QueryRow(ctx, `SELECT current_setting('server_version_num')::integer, current_setting('transaction_read_only')::boolean`).Scan(&version, &readOnly); err != nil || version < 180000 || !readOnly {
		t.Fatal("SQL contract requires a read-only PostgreSQL 18 test transaction")
	}

	roles := append(append([]string(nil), catalogRoles...), "gcp06_inspect_012345abcdef")
	var access DatabaseSchemaAccess
	if err := tx.QueryRow(ctx, queryDatabaseSchemaAccess).Scan(
		&access.DatabaseOwner, &access.PublicSchemaOwner, &access.APILoginExists, &access.RuntimeRoleExists,
		&access.APILoginCanConnect, &access.APILoginCanCreateDatabase, &access.APILoginCanUseTemporary,
		&access.RuntimeRoleCanConnect, &access.RuntimeRoleCanCreateDatabase, &access.RuntimeRoleCanUseTemporary,
		&access.APILoginPublicUsage, &access.APILoginPublicCreate, &access.RuntimeRolePublicUsage, &access.RuntimeRolePublicCreate,
	); err != nil {
		t.Fatal("database and schema access inventory query did not execute")
	}

	rows, err := tx.Query(ctx, queryRelationAccess)
	if err != nil {
		t.Fatalf("relation access inventory query did not execute: %v", err)
	}
	relationCount := 0
	foundProbeTable := false
	foundProbeSequence := false
	for rows.Next() {
		var access RelationAccess
		if err := rows.Scan(&access.Identity, &access.Kind, &access.Owner, &access.ObjectClass,
			&access.APILoginExists, &access.RuntimeRoleExists,
			&access.APILoginSelect, &access.APILoginInsert, &access.APILoginUpdate, &access.APILoginDelete,
			&access.APILoginTruncate, &access.APILoginReferences, &access.APILoginTrigger, &access.APILoginMaintain, &access.APILoginUsage,
			&access.RuntimeRoleSelect, &access.RuntimeRoleInsert, &access.RuntimeRoleUpdate, &access.RuntimeRoleDelete,
			&access.RuntimeRoleTruncate, &access.RuntimeRoleReferences, &access.RuntimeRoleTrigger, &access.RuntimeRoleMaintain, &access.RuntimeRoleUsage); err != nil {
			rows.Close()
			t.Fatal("relation access inventory result shape is invalid")
		}
		if access.Identity == "public.inspector_probe" {
			foundProbeTable = true
			if !access.RuntimeRoleSelect || !access.RuntimeRoleInsert || !access.RuntimeRoleTruncate || !access.RuntimeRoleReferences || !access.RuntimeRoleTrigger || !access.RuntimeRoleMaintain {
				t.Fatal("runtime relation inventory missed granted table privileges")
			}
			if access.APILoginSelect || access.APILoginTruncate || access.APILoginReferences || access.APILoginTrigger || access.APILoginMaintain {
				t.Fatal("API login relation inventory reported an ungranted table privilege")
			}
		}
		if access.Identity == "public.inspector_probe_id_seq" {
			foundProbeSequence = true
			if !access.APILoginUsage || access.RuntimeRoleUsage {
				t.Fatal("sequence usage inventory did not distinguish the granted and ungranted roles")
			}
		}
		relationCount++
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		t.Fatal("relation access inventory returned an error")
	}
	rows.Close()
	if relationCount == 0 {
		t.Fatal("relation access inventory returned no fixture relations")
	}
	if !foundProbeTable || !foundProbeSequence {
		t.Fatal("relation access inventory did not return both positive and negative privilege fixtures")
	}

	rows, err = tx.Query(ctx, queryColumnAccess)
	if err != nil {
		t.Fatalf("column access inventory query did not execute: %v", err)
	}
	columnCount := 0
	foundGrantedColumn := false
	foundDeniedColumn := false
	for rows.Next() {
		var access ColumnAccess
		if err := rows.Scan(&access.Identity, &access.APILoginExists,
			&access.APILoginSelect, &access.APILoginInsert, &access.APILoginUpdate, &access.APILoginReferences,
			&access.RuntimeRoleExists, &access.RuntimeRoleSelect, &access.RuntimeRoleInsert, &access.RuntimeRoleUpdate, &access.RuntimeRoleReferences); err != nil {
			rows.Close()
			t.Fatal("column access inventory result shape is invalid")
		}
		if access.Identity == "public.inspector_probe.visible" {
			foundGrantedColumn = true
			if !access.APILoginSelect || !access.APILoginInsert || !access.APILoginUpdate || !access.APILoginReferences {
				t.Fatal("column inventory missed privileges granted to the API login")
			}
		}
		if access.Identity == "public.inspector_probe.protected" {
			foundDeniedColumn = true
			if access.APILoginSelect || access.APILoginInsert || access.APILoginUpdate || access.APILoginReferences {
				t.Fatal("column inventory reported privileges not granted to the API login")
			}
		}
		columnCount++
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		t.Fatal("column access inventory returned an error")
	}
	rows.Close()
	if columnCount == 0 || !foundGrantedColumn || !foundDeniedColumn {
		t.Fatal("column access inventory did not return positive and negative fixture columns")
	}

	rows, err = tx.Query(ctx, queryRoutineAccess)
	if err != nil {
		t.Fatalf("routine access inventory query did not execute: %v", err)
	}
	routineCount := 0
	foundProbeRoutine := false
	for rows.Next() {
		var access RoutineAccess
		if err := rows.Scan(&access.Identity, &access.Owner, &access.SecurityDefiner, &access.APILoginExists, &access.APILoginExecute, &access.RuntimeRoleExists, &access.RuntimeRoleExecute); err != nil {
			rows.Close()
			t.Fatal("routine access inventory result shape is invalid")
		}
		if strings.Contains(access.Identity, "inspector_probe_fn") {
			foundProbeRoutine = true
			if access.APILoginExecute || !access.RuntimeRoleExecute {
				t.Fatal("routine inventory did not distinguish granted and ungranted EXECUTE")
			}
		}
		routineCount++
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		t.Fatal("routine access inventory returned an error")
	}
	rows.Close()
	if routineCount == 0 || !foundProbeRoutine {
		t.Fatal("routine access inventory did not return its positive/negative EXECUTE fixture")
	}

	rows, err = tx.Query(ctx, queryDefaultACL, roles)
	if err != nil {
		t.Fatalf("default ACL inventory query did not execute: %v", err)
	}
	defaultACLCount := 0
	foundDefaultACL := false
	for rows.Next() {
		var acl DefaultACL
		if err := rows.Scan(&acl.OwnerClass, &acl.Schema, &acl.ObjectType, &acl.Grantee, &acl.Privilege, &acl.Grantable, &acl.Count); err != nil {
			rows.Close()
			t.Fatal("default ACL inventory result shape is invalid")
		}
		if acl.OwnerClass == "postgres" && acl.Grantee == "espacigo_runtime" && acl.Privilege == "SELECT" && acl.Count > 0 {
			foundDefaultACL = true
		}
		defaultACLCount++
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		t.Fatal("default ACL inventory returned an error")
	}
	rows.Close()
	if defaultACLCount == 0 || !foundDefaultACL {
		t.Fatal("default ACL inventory did not return its positive fixture grant")
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal("catalog preflight test transaction did not commit")
	}
}

func TestInspectorSQLIsFixedReadOnlyCatalogAllowlist(t *testing.T) {
	statements := AllowedStatements()
	if len(statements) != 11 {
		t.Fatalf("expected eleven fixed catalog statements; got %d", len(statements))
	}
	for i, statement := range statements {
		upper := strings.ToUpper(strings.TrimSpace(statement))
		if !strings.HasPrefix(upper, "SELECT") && !strings.HasPrefix(upper, "WITH") {
			t.Fatalf("statement %d is not a read-only SELECT", i)
		}
		for _, forbidden := range []string{" INSERT ", " UPDATE ", " DELETE ", " TRUNCATE ", " ALTER ", " DROP ", " CREATE ", " COPY ", " CALL ", " DO "} {
			if strings.Contains(" "+upper+" ", forbidden) {
				t.Fatalf("statement %d includes a forbidden SQL verb", i)
			}
		}
	}
	joined := strings.Join(statements, "\n")
	for _, allowed := range []string{
		"pg_catalog.pg_roles", "pg_catalog.pg_auth_members", "pg_catalog.pg_extension", "pg_catalog.pg_class",
		"pg_catalog.pg_database", "pg_catalog.pg_default_acl", "to_regclass('public.schema_migrations')",
		"has_table_privilege", "has_sequence_privilege", "has_function_privilege", "has_column_privilege",
		"'TRUNCATE'", "'REFERENCES'", "'TRIGGER'", "'MAINTAIN'", "'USAGE'",
		"espacigo_api_login", "espacigo_runtime",
	} {
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
