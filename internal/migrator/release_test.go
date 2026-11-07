package migrator

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

func releaseFixture(t *testing.T, ctx context.Context, databaseURL string) (*pgx.Conn, string, ReleaseContract) {
	t.Helper()
	dir := t.TempDir()
	writeMigration(t, dir, "V000001__release_probe.sql", "CREATE TABLE public.release_probe(id integer PRIMARY KEY);\n")
	if _, err := Run(ctx, databaseURL, dir); err != nil {
		t.Fatal(err)
	}
	writeMigration(t, dir, "V000002__release_delta.sql", "ALTER TABLE public.release_probe ADD COLUMN title text;\n")
	migrations, err := DiscoverMigrations(dir)
	if err != nil {
		t.Fatal(err)
	}
	conn := connectTestDB(t, ctx, databaseURL)
	var database string
	if err := conn.QueryRow(ctx, "SELECT current_database()").Scan(&database); err != nil {
		t.Fatal(err)
	}
	return conn, dir, ReleaseContract{strings.Repeat("a", 40), inventoryFingerprint(migrations), 1, database}
}

func TestRunnerReleasePinsInventoryAndReviewedPrefix(t *testing.T) {
	withTestDatabase(t, func(ctx context.Context, databaseURL string) {
		conn, dir, contract := releaseFixture(t, ctx, databaseURL)
		defer conn.Close(context.Background())
		for _, change := range []func(*ReleaseContract){
			func(c *ReleaseContract) { c.InventorySHA256 = strings.Repeat("0", 64) },
			func(c *ReleaseContract) { c.ExpectedApplied = 2 },
			func(c *ReleaseContract) { c.Database = "another_database" },
		} {
			invalid := contract
			change(&invalid)
			if _, err := runReleaseWithConn(ctx, conn, dir, "", invalid); err == nil {
				t.Fatal("unreviewed release accepted")
			}
		}
		result, err := runReleaseWithConn(ctx, conn, dir, "", contract)
		if err != nil || len(result.Applied) != 1 || result.Applied[0] != 2 || len(result.Before.Pending) != 1 || len(result.After.Pending) != 0 || !result.After.ReadOnly {
			t.Fatalf("release result=%+v error=%v", result, err)
		}
		// A retry with the old reviewed prefix must not silently replay/accept it.
		if _, err := runReleaseWithConn(ctx, conn, dir, "", contract); err == nil {
			t.Fatal("obsolete prefix accepted after successful application")
		}
		contract.ExpectedApplied = 2
		result, err = runReleaseWithConn(ctx, conn, dir, "", contract)
		if err != nil || len(result.Applied) != 0 {
			t.Fatalf("reviewed no-op: %v", err)
		}
		var held bool
		if err := conn.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_locks WHERE locktype='advisory' AND pid=pg_backend_pid())").Scan(&held); err != nil || held {
			t.Fatalf("nested release lock leaked: %v", err)
		}
	})
}

func TestRunnerReleaseFailureDoesNotProduceSuccessOrReplayCommittedPrefix(t *testing.T) {
	withTestDatabase(t, func(ctx context.Context, databaseURL string) {
		conn, dir, contract := releaseFixture(t, ctx, databaseURL)
		defer conn.Close(context.Background())
		writeMigration(t, dir, "V000003__release_failure.sql", "CREATE TABLE public.release_rolled_back(id integer); SELECT 1/0;\n")
		migrations, err := DiscoverMigrations(dir)
		if err != nil {
			t.Fatal(err)
		}
		contract.InventorySHA256 = inventoryFingerprint(migrations)
		result, err := runReleaseWithConn(ctx, conn, dir, "", contract)
		if err == nil || result.Format != "" {
			t.Fatal("failed release returned success")
		}
		var count int
		var exists bool
		if err := conn.QueryRow(ctx, "SELECT (SELECT count(*) FROM public.schema_migrations),to_regclass('public.release_rolled_back') IS NOT NULL").Scan(&count, &exists); err != nil || count != 2 || exists {
			t.Fatalf("partial commit/rollback: count=%d exists=%v err=%v", count, exists, err)
		}
		if _, err := runReleaseWithConn(ctx, conn, dir, "", contract); err == nil || !strings.Contains(err.Error(), "prefix changed") {
			t.Fatalf("blind retry was not blocked: %v", err)
		}
	})
}

func TestRunnerReleaseSerializesCompetingReviewedPrefixes(t *testing.T) {
	withTestDatabase(t, func(ctx context.Context, databaseURL string) {
		first, dir, contract := releaseFixture(t, ctx, databaseURL)
		defer first.Close(context.Background())
		writeMigration(t, dir, "V000002__release_delta.sql", "SELECT pg_sleep(0.25); ALTER TABLE public.release_probe ADD COLUMN title text;\n")
		migrations, err := DiscoverMigrations(dir)
		if err != nil {
			t.Fatal(err)
		}
		contract.InventorySHA256 = inventoryFingerprint(migrations)
		second := connectTestDB(t, ctx, databaseURL)
		defer second.Close(context.Background())
		// Hold a third session's lock so both candidates contend at the same gate.
		gate := connectTestDB(t, ctx, databaseURL)
		defer gate.Close(context.Background())
		unlock, err := AcquireMigrationAdvisoryLock(ctx, gate)
		if err != nil {
			t.Fatal(err)
		}
		results := make(chan error, 2)
		ready := make(chan struct{}, 2)
		for _, conn := range []*pgx.Conn{first, second} {
			go func(c *pgx.Conn) {
				ready <- struct{}{}
				_, err := runReleaseWithConn(ctx, c, dir, "", contract)
				results <- err
			}(conn)
		}
		<-ready
		<-ready
		if err := unlock(ctx); err != nil {
			t.Fatal(err)
		}
		a, b := <-results, <-results
		if a == nil && b == nil || a != nil && b != nil {
			t.Fatalf("want one release and one obsolete prefix, got %v / %v", a, b)
		}
		failure := a
		if failure == nil {
			failure = b
		}
		if !strings.Contains(failure.Error(), "prefix changed") {
			t.Fatalf("competing release failed for another reason: %v", failure)
		}
	})
}
