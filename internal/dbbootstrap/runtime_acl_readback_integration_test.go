package dbbootstrap

import (
	"context"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/HernanEspinozaDev/espaciGo/internal/migrator"
	"github.com/jackc/pgx/v5"
)

const runtimeACLReadbackF5Database = "espacigo_f5_gate_20261101"

// This opt-in gate executes the catalog readback against the authorized local
// F5 target. It is intentionally separate from unit tests and never mutates DB.
func TestVerifyRuntimeACLReadbackF5Gate(t *testing.T) {
	dsn := os.Getenv("ESPACIGO_RUNTIME_ACL_READBACK_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set the explicit F5 ACL readback DSN to run the PostgreSQL gate")
	}
	parsedDSN, err := url.Parse(dsn)
	if err != nil || (parsedDSN.Scheme != "postgres" && parsedDSN.Scheme != "postgresql") ||
		parsedDSN.Host != "127.0.0.1:15434" || parsedDSN.Path != "/"+runtimeACLReadbackF5Database ||
		parsedDSN.User == nil || parsedDSN.User.Username() != "postgres" {
		t.Fatal("F5 ACL readback gate requires a postgres URI for the exact loopback target")
	}
	if _, hasInlinePassword := parsedDSN.User.Password(); hasInlinePassword {
		t.Fatal("F5 ACL readback gate rejects inline passwords; use the private PGPASSFILE")
	}
	config, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatal("F5 ACL readback gate requires a valid explicit target DSN")
	}
	if config.Host != "127.0.0.1" || config.Port != 15434 ||
		config.Database != runtimeACLReadbackF5Database || config.User != "postgres" {
		t.Fatal("F5 ACL readback gate parsed an unexpected target identity")
	}
	passfile := os.Getenv("PGPASSFILE")
	if !filepath.IsAbs(passfile) || filepath.Clean(passfile) == "/dev/null" {
		t.Fatal("F5 ACL readback gate requires an absolute private passfile")
	}
	info, err := os.Lstat(passfile)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
		t.Fatal("F5 ACL readback gate requires a regular passfile with mode 0600")
	}
	if os.Getenv("PGPASSWORD") != "" || os.Getenv("PGSERVICE") != "" || os.Getenv("PGSERVICEFILE") != "" {
		t.Fatal("F5 ACL readback gate rejects alternate credential sources")
	}

	ctx := context.Background()
	conn, err := pgx.ConnectConfig(ctx, config)
	if err != nil {
		t.Fatal("could not connect to the exact F5 ACL readback target")
	}
	defer conn.Close(context.Background())
	var database, sessionUser, currentUser string
	if err := conn.QueryRow(ctx, "SELECT current_database(), session_user, current_user").Scan(&database, &sessionUser, &currentUser); err != nil ||
		database != runtimeACLReadbackF5Database || sessionUser != "postgres" || currentUser != "postgres" {
		t.Fatal("F5 ACL readback database identity mismatch")
	}

	tx, err := conn.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		t.Fatal("could not start the read-only F5 ACL inspection transaction")
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	migrations, err := migrator.DiscoverMigrations(filepath.Join("..", "..", "db", "migrations"))
	if err != nil {
		t.Fatal(err)
	}
	rows, err := tx.Query(ctx, `SELECT version,name,checksum FROM public.schema_migrations ORDER BY version`)
	if err != nil {
		t.Fatal("could not read the F5 applied migration ledger")
	}
	var sources []RuntimeACLSource
	index := 0
	for rows.Next() {
		var version int64
		var name, checksum string
		if err := rows.Scan(&version, &name, &checksum); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		if index >= len(migrations) || migrations[index].Version != version || migrations[index].Name != name || migrations[index].Checksum != checksum {
			rows.Close()
			t.Fatalf("F5 ledger/source mismatch at ordered entry %d", index)
		}
		sources = append(sources, RuntimeACLSource{Version: int(version), SQL: migrations[index].SQL})
		index++
	}
	rowsErr := rows.Err()
	rows.Close()
	if rowsErr != nil {
		t.Fatal("could not finish reading the F5 migration ledger")
	}
	if index == 0 {
		t.Fatal(errors.New("F5 migration ledger is empty"))
	}
	if err := VerifyRuntimeACLReadbackF5(ctx, tx, sources); err != nil {
		t.Fatalf("complete F5 runtime ACL catalog readback failed: %v", err)
	}
}

// This preview builds the exact V1-V80 runtime role/ACL state inside one
// transaction, runs the real PostgreSQL catalog readback, and always rolls it
// back. It lets the verifier be exercised before any persistent repair or DDL.
func TestRuntimeACLReadbackF5RollbackPreview(t *testing.T) {
	dsn := os.Getenv("ESPACIGO_RUNTIME_ACL_READBACK_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set the explicit F5 ACL readback DSN to run the PostgreSQL rollback preview")
	}
	parsedDSN, err := url.Parse(dsn)
	if err != nil || (parsedDSN.Scheme != "postgres" && parsedDSN.Scheme != "postgresql") ||
		parsedDSN.Host != "127.0.0.1:15434" || parsedDSN.Path != "/"+runtimeACLReadbackF5Database ||
		parsedDSN.User == nil || parsedDSN.User.Username() != "postgres" {
		t.Fatal("F5 ACL preview requires a postgres URI for the exact loopback target")
	}
	if _, hasInlinePassword := parsedDSN.User.Password(); hasInlinePassword {
		t.Fatal("F5 ACL preview rejects inline passwords; use the private PGPASSFILE")
	}
	config, err := pgx.ParseConfig(dsn)
	if err != nil || config.Host != "127.0.0.1" || config.Port != 15434 ||
		config.Database != runtimeACLReadbackF5Database || config.User != "postgres" {
		t.Fatal("F5 ACL preview parsed an unexpected target identity")
	}
	passfile := os.Getenv("PGPASSFILE")
	info, err := os.Lstat(passfile)
	if err != nil || !filepath.IsAbs(passfile) || filepath.Clean(passfile) == "/dev/null" ||
		!info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
		t.Fatal("F5 ACL preview requires an absolute regular mode-0600 private passfile")
	}
	if os.Getenv("PGPASSWORD") != "" || os.Getenv("PGSERVICE") != "" || os.Getenv("PGSERVICEFILE") != "" {
		t.Fatal("F5 ACL preview rejects alternate credential sources")
	}

	ctx := context.Background()
	conn, err := pgx.ConnectConfig(ctx, config)
	if err != nil {
		t.Fatal("could not connect to the exact F5 ACL preview target")
	}
	defer conn.Close(context.Background())
	var database, sessionUser, currentUser string
	if err := conn.QueryRow(ctx, "SELECT current_database(), session_user, current_user").Scan(&database, &sessionUser, &currentUser); err != nil ||
		database != runtimeACLReadbackF5Database || sessionUser != "postgres" || currentUser != "postgres" {
		t.Fatal("F5 ACL preview database identity mismatch")
	}

	migrations, err := migrator.DiscoverMigrations(filepath.Join("..", "..", "db", "migrations"))
	if err != nil {
		t.Fatal(err)
	}
	rows, err := conn.Query(ctx, `SELECT version,name,checksum FROM public.schema_migrations ORDER BY version`)
	if err != nil {
		t.Fatal("could not read the F5 applied migration ledger")
	}
	var sources []RuntimeACLSource
	index := 0
	for rows.Next() {
		var version int64
		var name, checksum string
		if err := rows.Scan(&version, &name, &checksum); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		if index >= len(migrations) || migrations[index].Version != version || migrations[index].Name != name || migrations[index].Checksum != checksum {
			rows.Close()
			t.Fatalf("F5 ledger/source mismatch at ordered entry %d", index)
		}
		sources = append(sources, RuntimeACLSource{Version: int(version), SQL: migrations[index].SQL})
		index++
	}
	rowsErr := rows.Err()
	rows.Close()
	if rowsErr != nil || index != 80 {
		t.Fatal("could not verify the complete V1-V80 F5 migration ledger")
	}

	tx, err := conn.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		t.Fatal("could not start the rollback-only F5 ACL preview transaction")
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	roleDDL := []string{
		`CREATE ROLE espacigo_runtime NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT`,
		`CREATE ROLE espacigo_api_login LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT`,
		`CREATE ROLE espacigo_schema_owner NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT`,
		`CREATE ROLE espacigo_migrator_login LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT`,
		`CREATE ROLE espacigo_document_verification_fn NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT`,
		`GRANT espacigo_runtime TO espacigo_api_login WITH ADMIN FALSE, INHERIT FALSE, SET TRUE`,
		`GRANT espacigo_schema_owner TO espacigo_migrator_login WITH ADMIN FALSE, INHERIT FALSE, SET TRUE`,
		`GRANT espacigo_document_verification_fn TO espacigo_schema_owner WITH ADMIN FALSE, INHERIT FALSE, SET TRUE`,
		`GRANT CONNECT ON DATABASE espacigo_f5_gate_20261101 TO espacigo_runtime`,
	}
	for i, statement := range roleDDL {
		if _, err := tx.Exec(ctx, statement); err != nil {
			t.Fatalf("F5 ACL preview role/bootstrap statement %d failed: %v", i+1, err)
		}
	}
	if err := ApplyRuntimeACLFromMigrationsF5(ctx, tx, sources); err != nil {
		t.Fatalf("apply source-derived V1-V80 ACLs in preview transaction: %v", err)
	}
	if err := GrantRuntimePermissions(ctx, tx); err != nil {
		t.Fatalf("apply shared runtime ACL allowlist in preview transaction: %v", err)
	}
	if err := VerifyRuntimeACLReadbackF5(ctx, tx, sources); err != nil {
		t.Fatalf("complete F5 runtime ACL catalog readback failed in rollback preview: %v", err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatalf("could not roll back F5 ACL preview: %v", err)
	}
	if err := verifyRuntimeACLF5PublicBaseline(ctx, conn); err != nil {
		t.Fatalf("rollback preview changed the persistent F5 PUBLIC/default baseline: %v", err)
	}
}
