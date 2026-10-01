package migrator

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestRunnerAppliesFromEmptyAndRepeatIsNoop(t *testing.T) {
	withTestDatabase(t, func(ctx context.Context, databaseURL string) {
		dir := t.TempDir()
		writeMigration(t, dir, "V000001__probe.sql", "CREATE TABLE probe (id integer PRIMARY KEY); INSERT INTO probe VALUES (1);\n")

		first, err := Run(ctx, databaseURL, dir)
		if err != nil {
			t.Fatal(err)
		}
		if len(first.Applied) != 1 || first.Applied[0] != 1 {
			t.Fatalf("first run applied versions %v", first.Applied)
		}
		second, err := Run(ctx, databaseURL, dir)
		if err != nil {
			t.Fatal(err)
		}
		if len(second.Applied) != 0 {
			t.Fatalf("second run reapplied versions %v", second.Applied)
		}

		conn := connectTestDB(t, ctx, databaseURL)
		defer conn.Close(context.Background())
		var rows, historyRows int
		if err := conn.QueryRow(ctx, "SELECT count(*) FROM probe").Scan(&rows); err != nil {
			t.Fatal(err)
		}
		if err := conn.QueryRow(ctx, "SELECT count(*) FROM public.schema_migrations").Scan(&historyRows); err != nil {
			t.Fatal(err)
		}
		if rows != 1 || historyRows != 1 {
			t.Fatalf("repeat changed state: probe rows=%d history rows=%d", rows, historyRows)
		}
	})
}

func TestRunnerStoresExactMigrationAndRunnerChecksums(t *testing.T) {
	withTestDatabase(t, func(ctx context.Context, databaseURL string) {
		const migrationSQL = "CREATE TABLE checksum_probe (id integer PRIMARY KEY);\n"
		dir := t.TempDir()
		writeMigration(t, dir, "V000001__checksum_probe.sql", migrationSQL)
		if _, err := Run(ctx, databaseURL, dir); err != nil {
			t.Fatal(err)
		}

		conn := connectTestDB(t, ctx, databaseURL)
		defer conn.Close(context.Background())
		var gotMigrationChecksum, gotRunnerChecksum string
		var appliedAt time.Time
		if err := conn.QueryRow(ctx, `SELECT checksum, runner_checksum, applied_at
			FROM public.schema_migrations WHERE version = 1`).Scan(
			&gotMigrationChecksum, &gotRunnerChecksum, &appliedAt); err != nil {
			t.Fatal(err)
		}
		if want := sha256Hex([]byte(migrationSQL)); gotMigrationChecksum != want {
			t.Fatalf("stored migration checksum=%s, want %s", gotMigrationChecksum, want)
		}
		wantRunnerChecksum, err := executableChecksum()
		if err != nil {
			t.Fatal(err)
		}
		if gotRunnerChecksum != wantRunnerChecksum {
			t.Fatalf("stored runner checksum=%s, want %s", gotRunnerChecksum, wantRunnerChecksum)
		}
		if appliedAt.IsZero() {
			t.Fatal("applied_at was not recorded")
		}
	})
}

func TestRunnerRejectsChecksumDriftBeforeDDL(t *testing.T) {
	withTestDatabase(t, func(ctx context.Context, databaseURL string) {
		dir := t.TempDir()
		path := writeMigration(t, dir, "V000001__probe.sql", "CREATE TABLE probe (id integer PRIMARY KEY);\n")
		if _, err := Run(ctx, databaseURL, dir); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("CREATE TABLE drift_must_not_run (id integer);\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := Run(ctx, databaseURL, dir); err == nil {
			t.Fatal("expected checksum drift to fail")
		}
		conn := connectTestDB(t, ctx, databaseURL)
		defer conn.Close(context.Background())
		var relation *string
		if err := conn.QueryRow(ctx, "SELECT to_regclass('public.drift_must_not_run')::text").Scan(&relation); err != nil {
			t.Fatal(err)
		}
		if relation != nil {
			t.Fatalf("DDL ran despite checksum drift: %v", *relation)
		}
	})
}

func TestRunnerRollsBackMigrationAndHistoryTogether(t *testing.T) {
	withTestDatabase(t, func(ctx context.Context, databaseURL string) {
		dir := t.TempDir()
		writeMigration(t, dir, "V000001__failing_probe.sql", "CREATE TABLE rolled_back_probe (id integer); SELECT 1 / 0;\n")
		if _, err := Run(ctx, databaseURL, dir); err == nil {
			t.Fatal("expected the invalid migration to fail")
		}
		conn := connectTestDB(t, ctx, databaseURL)
		defer conn.Close(context.Background())
		var relation *string
		var historyRows int
		if err := conn.QueryRow(ctx, "SELECT to_regclass('public.rolled_back_probe')::text").Scan(&relation); err != nil {
			t.Fatal(err)
		}
		if err := conn.QueryRow(ctx, "SELECT count(*) FROM public.schema_migrations").Scan(&historyRows); err != nil {
			t.Fatal(err)
		}
		if relation != nil || historyRows != 0 {
			t.Fatalf("failed migration was not atomic: relation=%v history=%d", relation, historyRows)
		}
	})
}

func TestRunnerSerializesConcurrentExecutions(t *testing.T) {
	withTestDatabase(t, func(ctx context.Context, databaseURL string) {
		dir := t.TempDir()
		writeMigration(t, dir, "V000001__concurrent_probe.sql", "SELECT pg_sleep(0.3); CREATE TABLE concurrent_probe (id integer PRIMARY KEY);\n")

		var wg sync.WaitGroup
		errs := make(chan error, 2)
		for range 2 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := Run(ctx, databaseURL, dir)
				errs <- err
			}()
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			if err != nil {
				t.Fatal(err)
			}
		}
		conn := connectTestDB(t, ctx, databaseURL)
		defer conn.Close(context.Background())
		var historyRows int
		if err := conn.QueryRow(ctx, "SELECT count(*) FROM public.schema_migrations").Scan(&historyRows); err != nil {
			t.Fatal(err)
		}
		if historyRows != 1 {
			t.Fatalf("concurrent runs applied migration %d times", historyRows)
		}
	})
}

func TestRunnerRejectsVersionGapBeforeDDL(t *testing.T) {
	withTestDatabase(t, func(ctx context.Context, databaseURL string) {
		dir := t.TempDir()
		writeMigration(t, dir, "V000001__first.sql", "CREATE TABLE first_probe (id integer);\n")
		writeMigration(t, dir, "V000003__third.sql", "CREATE TABLE third_probe (id integer);\n")
		if _, err := Run(ctx, databaseURL, dir); err == nil {
			t.Fatal("expected version gap to fail")
		}
		conn := connectTestDB(t, ctx, databaseURL)
		defer conn.Close(context.Background())
		var historyExists bool
		if err := conn.QueryRow(ctx, "SELECT to_regclass('public.schema_migrations') IS NOT NULL").Scan(&historyExists); err != nil {
			t.Fatal(err)
		}
		if historyExists {
			var historyRows int
			if err := conn.QueryRow(ctx, "SELECT count(*) FROM public.schema_migrations").Scan(&historyRows); err != nil {
				t.Fatal(err)
			}
			if historyRows != 0 {
				t.Fatalf("invalid sequence changed history: %d rows", historyRows)
			}
		}
	})
}

func withTestDatabase(t *testing.T, test func(context.Context, string)) {
	t.Helper()
	adminURL := os.Getenv("TEST_DATABASE_URL")
	if adminURL == "" {
		t.Skip("TEST_DATABASE_URL is not set; requires disposable PostgreSQL 18")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	admin, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		t.Fatalf("connect to disposable PostgreSQL: %v", err)
	}
	defer admin.Close(context.Background())
	databaseName := fmt.Sprintf("migrator_test_%d", time.Now().UnixNano())
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{databaseName}.Sanitize()); err != nil {
		t.Fatalf("create disposable test database: %v", err)
	}
	defer func() {
		if _, err := admin.Exec(context.Background(), "DROP DATABASE IF EXISTS "+pgx.Identifier{databaseName}.Sanitize()); err != nil {
			t.Errorf("drop disposable test database: %v", err)
		}
	}()
	databaseURLParsed, err := url.Parse(adminURL)
	if err != nil {
		t.Fatal(err)
	}
	databaseURLParsed.Path = "/" + databaseName
	testURL := databaseURLParsed.String()
	testConn := connectTestDB(t, ctx, testURL)
	if err := testConn.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	test(ctx, testURL)
}

func connectTestDB(t *testing.T, ctx context.Context, databaseURL string) *pgx.Conn {
	t.Helper()
	conn, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	return conn
}

func writeMigration(t *testing.T, dir, name, sql string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(sql), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
