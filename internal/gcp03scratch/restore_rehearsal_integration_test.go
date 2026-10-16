package gcp03scratch

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/migrator"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type restoreFixtureManifest struct {
	Format     string `json:"format"`
	FixtureSQL struct {
		File   string `json:"file"`
		Bytes  int    `json:"bytes"`
		SHA256 string `json:"sha256"`
	} `json:"fixture_sql"`
	Migrations struct {
		Count           int    `json:"count"`
		InventorySHA256 string `json:"inventory_sha256"`
	} `json:"migrations"`
}

type restoreEvidence struct {
	Format                    string          `json:"format"`
	GeneratedAtUTC            string          `json:"generated_at_utc"`
	Source                    string          `json:"source"`
	SourceDatabase            string          `json:"source_database"`
	RestoredDatabase          string          `json:"restored_database"`
	PostgreSQL                string          `json:"postgresql"`
	PostGISFullVersion        string          `json:"postgis_full_version"`
	Migrations                int             `json:"migrations"`
	MigrationInventory        string          `json:"migration_inventory_sha256"`
	FixtureSHA256             string          `json:"fixture_sha256"`
	DumpBytes                 int64           `json:"dump_bytes"`
	DumpSHA256                string          `json:"dump_sha256"`
	TOCSHA256                 string          `json:"toc_sha256"`
	ConcurrentDeadlockRetries int             `json:"concurrent_deadlock_retries"`
	RestoredCounts            map[string]int  `json:"restored_counts"`
	Checks                    map[string]bool `json:"checks"`
	Notes                     []string        `json:"limits"`
}

func TestSyntheticRestoreRehearsalPostgres18(t *testing.T) {
	adminURL := os.Getenv("TEST_DATABASE_URL")
	fixturePath := os.Getenv("GCP03_RESTORE_FIXTURE_PATH")
	evidenceDir := os.Getenv("GCP03_RESTORE_EVIDENCE_DIR")
	if adminURL == "" || fixturePath == "" || evidenceDir == "" {
		t.Skip("use scripts/test-gcp03-restore-rehearsal-postgres.sh for the isolated PG18/PostGIS restore rehearsal")
	}
	adminURLParsed, err := url.Parse(adminURL)
	if err != nil || adminURLParsed.Scheme != "postgres" || adminURLParsed.User == nil || adminURLParsed.User.Username() != "postgres" {
		t.Fatal("restore rehearsal only accepts the isolated local harness URL")
	}
	if _, hasPassword := adminURLParsed.User.Password(); hasPassword || adminURLParsed.Hostname() != "localhost" || adminURLParsed.Port() != "" || adminURLParsed.Path != "/postgres" {
		t.Fatal("restore rehearsal refuses password-bearing, TCP, or non-harness database URLs")
	}
	socket := adminURLParsed.Query().Get("host")
	if !filepath.IsAbs(socket) || !strings.HasPrefix(filepath.Base(filepath.Dir(socket)), "espacigo-m04-test.") || filepath.Base(socket) != "socket" {
		t.Fatal("restore rehearsal requires the network-isolated ephemeral M04 test socket")
	}
	if !filepath.IsAbs(evidenceDir) || !filepath.IsAbs(fixturePath) {
		t.Fatal("fixture and evidence paths must be absolute local paths")
	}
	root, err := repositoryRoot(t)
	if err != nil {
		t.Fatal(err)
	}
	localTesting, err := filepath.Rel(root, evidenceDir)
	if err != nil || localTesting == ".." || strings.HasPrefix(localTesting, ".."+string(filepath.Separator)) || !strings.HasPrefix(localTesting, filepath.Join(".local", "testing")+string(filepath.Separator)) {
		t.Fatal("restore evidence must stay under repository .local/testing")
	}
	fixtureRel, err := filepath.Rel(root, fixturePath)
	if err != nil || strings.HasPrefix(fixtureRel, "..") {
		t.Fatal("fixture must be generated inside this repository worktree")
	}
	if err := os.MkdirAll(evidenceDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(evidenceDir, 0o700); err != nil {
		t.Fatal(err)
	}

	fixtureSQL, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatal(err)
	}
	manifestBytes, err := os.ReadFile(filepath.Join(evidenceDir, "fixture-manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixtureManifest restoreFixtureManifest
	if err := json.Unmarshal(manifestBytes, &fixtureManifest); err != nil {
		t.Fatal(err)
	}
	fixtureHash := sha256.Sum256(fixtureSQL)
	fixtureHashText := hex.EncodeToString(fixtureHash[:])
	if fixtureManifest.Format != "espacigo-gcp03-restore-fixture-v1" || fixtureManifest.FixtureSQL.SHA256 != fixtureHashText || fixtureManifest.FixtureSQL.Bytes != len(fixtureSQL) || fixtureManifest.Migrations.Count != MigrationCount {
		t.Fatal("generated fixture manifest does not match fixture bytes or V1-V61 inventory")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	admin, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		t.Fatalf("connect to ephemeral PostgreSQL harness: %v", err)
	}
	defer admin.Close(context.Background())
	var serverVersion int
	if err := admin.QueryRow(ctx, `SELECT current_setting('server_version_num')::int`).Scan(&serverVersion); err != nil || serverVersion/10000 != 18 {
		t.Fatalf("expected PostgreSQL 18, server_version_num=%d err=%v", serverVersion, err)
	}
	if _, err := exec.LookPath("pg_dump"); err != nil {
		t.Fatal("pg_dump is required for the local dump/restore rehearsal")
	}
	if _, err := exec.LookPath("pg_restore"); err != nil {
		t.Fatal("pg_restore is required for the local dump/restore rehearsal")
	}

	runID := fmt.Sprintf("%d", time.Now().UnixNano())
	sourceDB := "g03_restore_src_" + runID
	targetDB := "g03_restore_dst_" + runID
	ownerRole := "g03_restore_owner_" + runID
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{sourceDB}.Sanitize()); err != nil {
		t.Fatal("create disposable synthetic source database: ", err)
	}
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{targetDB}.Sanitize()); err != nil {
		_, _ = admin.Exec(context.Background(), "DROP DATABASE "+pgx.Identifier{sourceDB}.Sanitize()+" WITH (FORCE)")
		t.Fatal("create disposable synthetic restore target: ", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cleanupCancel()
		cleanupAdmin, connectErr := pgx.Connect(cleanupCtx, adminURL)
		if connectErr != nil {
			t.Errorf("reconnect ephemeral PostgreSQL for cleanup: %v", connectErr)
			return
		}
		defer cleanupAdmin.Close(context.Background())
		for _, db := range []string{targetDB, sourceDB} {
			if _, e := cleanupAdmin.Exec(cleanupCtx, "DROP DATABASE IF EXISTS "+pgx.Identifier{db}.Sanitize()+" WITH (FORCE)"); e != nil {
				t.Errorf("drop disposable synthetic database: %v", e)
			}
		}
		if _, e := cleanupAdmin.Exec(cleanupCtx, "DROP ROLE IF EXISTS "+pgx.Identifier{ownerRole}.Sanitize()); e != nil {
			t.Errorf("drop disposable synthetic owner role: %v", e)
		}
	})

	sourceURL := databaseURL(adminURLParsed, sourceDB)
	targetURL := databaseURL(adminURLParsed, targetDB)
	migrationDir := filepath.Join(root, "db", "migrations")
	result, err := migrator.Run(ctx, sourceURL, migrationDir)
	if err != nil || len(result.Applied) != MigrationCount || result.Applied[MigrationCount-1] != MigrationCount {
		t.Fatalf("apply exact V1-V61 to disposable synthetic source: applied=%d err=%v", len(result.Applied), err)
	}
	source, err := pgx.Connect(ctx, sourceURL)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := source.Exec(ctx, string(fixtureSQL)); err != nil {
		_ = source.Close(context.Background())
		t.Fatalf("seed deterministic synthetic restore fixture: %v", err)
	}
	if _, err := admin.Exec(ctx, "CREATE ROLE "+pgx.Identifier{ownerRole}.Sanitize()+" NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT"); err != nil {
		_ = source.Close(context.Background())
		t.Fatal("create unique NOLOGIN owner role only in disposable local cluster: ", err)
	}
	if _, err := source.Exec(ctx, "ALTER TABLE public.ocupacion OWNER TO "+pgx.Identifier{ownerRole}.Sanitize()); err != nil {
		_ = source.Close(context.Background())
		t.Fatal("set synthetic source table owner: ", err)
	}
	if _, err := source.Exec(ctx, "ALTER FUNCTION public.purge_expired_local_communication(timestamptz,integer) OWNER TO "+pgx.Identifier{ownerRole}.Sanitize()); err != nil {
		_ = source.Close(context.Background())
		t.Fatal("set synthetic SECURITY DEFINER function owner: ", err)
	}
	if err := source.Close(ctx); err != nil {
		t.Fatal(err)
	}

	dumpPath := filepath.Join(evidenceDir, "synthetic-v1-v61.dump")
	dumpFile, err := os.OpenFile(dumpPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := dumpFile.Close(); err != nil {
		t.Fatal(err)
	}
	dump := exec.CommandContext(ctx, "pg_dump", "--format=custom", "--no-owner", "--no-acl", "--file", dumpPath, "--dbname", sourceURL)
	if out, err := dump.CombinedOutput(); err != nil {
		t.Fatalf("pg_dump synthetic source: %v: %s", err, strings.TrimSpace(string(out)))
	}
	if err := os.Chmod(dumpPath, 0o600); err != nil {
		t.Fatal(err)
	}
	dumpBytes, err := os.ReadFile(dumpPath)
	if err != nil {
		t.Fatal(err)
	}
	dumpHash := sha256.Sum256(dumpBytes)
	toc, err := exec.CommandContext(ctx, "pg_restore", "--list", dumpPath).Output()
	if err != nil {
		t.Fatal("read synthetic dump TOC: ", err)
	}
	tocHash := sha256.Sum256(toc)
	restore := exec.CommandContext(ctx, "pg_restore", "--exit-on-error", "--no-owner", "--no-acl", "--dbname", targetURL, dumpPath)
	if out, err := restore.CombinedOutput(); err != nil {
		t.Fatalf("pg_restore synthetic dump: %v: %s", err, strings.TrimSpace(string(out)))
	}

	target, err := pgx.Connect(ctx, targetURL)
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close(context.Background())
	if _, err := target.Exec(ctx, "ALTER TABLE public.ocupacion OWNER TO "+pgx.Identifier{ownerRole}.Sanitize()); err != nil {
		t.Fatal("reapply explicit scratch ownership after --no-owner restore: ", err)
	}
	if _, err := target.Exec(ctx, "ALTER FUNCTION public.purge_expired_local_communication(timestamptz,integer) OWNER TO "+pgx.Identifier{ownerRole}.Sanitize()); err != nil {
		t.Fatal("reapply explicit SECURITY DEFINER owner after --no-owner restore: ", err)
	}
	var ownerLogin, ownerSuper, ownerCreateDB, ownerCreateRole, ownerReplication, ownerBypassRLS, ownerInherit bool
	if err := target.QueryRow(ctx, `SELECT rolcanlogin,rolsuper,rolcreatedb,rolcreaterole,rolreplication,rolbypassrls,rolinherit FROM pg_roles WHERE rolname=$1`, ownerRole).
		Scan(&ownerLogin, &ownerSuper, &ownerCreateDB, &ownerCreateRole, &ownerReplication, &ownerBypassRLS, &ownerInherit); err != nil {
		t.Fatal("read synthetic owner role attributes: ", err)
	}
	if ownerLogin || ownerSuper || ownerCreateDB || ownerCreateRole || ownerReplication || ownerBypassRLS || ownerInherit {
		t.Fatal("synthetic owner role has unsafe or login privileges")
	}
	var migrationCount int
	if err := target.QueryRow(ctx, `SELECT count(*) FROM public.schema_migrations`).Scan(&migrationCount); err != nil || migrationCount != MigrationCount {
		t.Fatalf("restored migration history=%d err=%v, want V1-V61", migrationCount, err)
	}
	var migrationChecksumsEqual bool
	if err := target.QueryRow(ctx, `SELECT (SELECT string_agg(version::text||':'||checksum,E'\n' ORDER BY version) FROM public.schema_migrations) = $1`, migrationHistory(ctx, t, sourceURL)).Scan(&migrationChecksumsEqual); err != nil || !migrationChecksumsEqual {
		t.Fatalf("restored migration checksum inventory equal=%v err=%v", migrationChecksumsEqual, err)
	}
	var users, spaces, occupancies, adjacency int
	if err := target.QueryRow(ctx, `SELECT (SELECT count(*) FROM public.usuario),(SELECT count(*) FROM public.espacio),(SELECT count(*) FROM public.ocupacion),
		(SELECT count(*) FROM public.ocupacion WHERE espacio_id='dddddddd-dddd-4ddd-8ddd-dddddddddddd' AND activo AND tipo='bloqueo_manual')`).Scan(&users, &spaces, &occupancies, &adjacency); err != nil {
		t.Fatal(err)
	}
	if users != 2 || spaces != 1 || occupancies != 2 || adjacency != 2 {
		t.Fatalf("restored fixture counts users/spaces/occupancies/adjacent=%d/%d/%d/%d", users, spaces, occupancies, adjacency)
	}
	var postgis, dstHours, spatialValid bool
	var postgisVersion string
	if err := target.QueryRow(ctx, `SELECT postgis_full_version(),postgis_full_version() <> '',
		ST_IsValid(ST_SetSRID(ST_MakePoint(-70.6,-33.4),4326)),
		extract(epoch FROM ((timestamp '2026-09-07 00:00' AT TIME ZONE 'America/Santiago')-(timestamp '2026-09-06 00:00' AT TIME ZONE 'America/Santiago')))=23*3600`).Scan(&postgisVersion, &postgis, &spatialValid, &dstHours); err != nil {
		t.Fatal("verify PostGIS and Santiago DST after restore: ", err)
	}
	if !postgis || !spatialValid || !dstHours {
		t.Fatalf("PostGIS/DST checks failed: postgis=%t spatial=%t 23-hour-day=%t", postgis, spatialValid, dstHours)
	}
	var owner, functionOwner string
	var securityDefiner, safeSearchPath, triggerPresent bool
	if err := target.QueryRow(ctx, `SELECT pg_get_userbyid(c.relowner) FROM pg_class c WHERE c.oid='public.ocupacion'::regclass`).Scan(&owner); err != nil {
		t.Fatal(err)
	}
	if err := target.QueryRow(ctx, `SELECT pg_get_userbyid(p.proowner),p.prosecdef,
		replace(coalesce(array_to_string(p.proconfig,','),''),' ','') LIKE '%search_path=pg_catalog,public%'
		FROM pg_proc p WHERE p.oid='public.purge_expired_local_communication(timestamptz,integer)'::regprocedure`).Scan(&functionOwner, &securityDefiner, &safeSearchPath); err != nil {
		t.Fatal(err)
	}
	if err := target.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_trigger WHERE tgname='usuario_gadm_bootstrap_identity_immutable' AND NOT tgisinternal)`).Scan(&triggerPresent); err != nil {
		t.Fatal(err)
	}
	if owner != ownerRole || functionOwner != ownerRole || !securityDefiner || !safeSearchPath || !triggerPresent {
		t.Fatalf("ownership/SECURITY DEFINER/trigger mismatch owner=%s function_owner=%s security_definer=%t safe_path=%t trigger=%t", owner, functionOwner, securityDefiner, safeSearchPath, triggerPresent)
	}
	if err := assertOverlapRejected(ctx, target); err != nil {
		t.Fatal(err)
	}
	deadlockRetries, err := assertConcurrentOverlap(ctx, targetURL)
	if err != nil {
		t.Fatal(err)
	}

	evidence := restoreEvidence{
		Format: "espacigo-gcp03-synthetic-restore-evidence-v1", GeneratedAtUTC: time.Now().UTC().Format(time.RFC3339),
		Source:         "locally generated synthetic data in network-isolated PostgreSQL 18/PostGIS container; no persistent source or dataset accessed",
		SourceDatabase: sourceDB, RestoredDatabase: targetDB, PostgreSQL: fmt.Sprintf("%d.%d", serverVersion/10000, serverVersion%10000),
		PostGISFullVersion: postgisVersion, Migrations: migrationCount, MigrationInventory: fixtureManifest.Migrations.InventorySHA256,
		FixtureSHA256: fixtureHashText, DumpBytes: int64(len(dumpBytes)), DumpSHA256: hex.EncodeToString(dumpHash[:]), TOCSHA256: hex.EncodeToString(tocHash[:]),
		ConcurrentDeadlockRetries: deadlockRetries,
		RestoredCounts:            map[string]int{"usuario": users, "espacio": spaces, "ocupacion": occupancies},
		Checks:                    map[string]bool{"migration_checksums": migrationChecksumsEqual, "adjacent_ranges": true, "overlap_rejected": true, "concurrent_overlap_one_winner": true, "santiago_dst_23_hour_day": dstHours, "postgis_spatial_function": spatialValid, "object_ownership_reapplied": owner == ownerRole && functionOwner == ownerRole, "owner_role_minimal_no_login": !ownerLogin && !ownerSuper && !ownerCreateDB && !ownerCreateRole && !ownerReplication && !ownerBypassRLS && !ownerInherit, "security_definer_search_path": securityDefiner && safeSearchPath, "v60_trigger_present": triggerPresent},
		Notes:                     []string{"This local synthetic rehearsal tests the dump/restore mechanics only; it is not a restore of a source snapshot.", "Local PostgreSQL role behavior does not prove Cloud SQL managed-role behavior.", "The restore uses --no-owner/--no-acl then reapplies explicit ownership; cloud resources, SQL source, secrets, and application deployment were not used."},
	}
	evidenceBytes, err := json.MarshalIndent(evidence, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	evidenceBytes = append(evidenceBytes, '\n')
	evidencePath := filepath.Join(evidenceDir, "restore-evidence.json")
	if err := os.WriteFile(evidencePath, evidenceBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(evidencePath, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Logf("synthetic restore passed; fixture sha256=%s dump sha256=%s bytes=%d", fixtureHashText, hex.EncodeToString(dumpHash[:]), len(dumpBytes))
}

func repositoryRoot(t *testing.T) (string, error) {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("resolve integration source path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..")), nil
}

func databaseURL(base *url.URL, db string) string {
	u := *base
	u.Path = "/" + db
	return u.String()
}

func migrationHistory(ctx context.Context, t *testing.T, dbURL string) string {
	t.Helper()
	conn, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())
	var history string
	if err := conn.QueryRow(ctx, `SELECT string_agg(version::text||':'||checksum,E'\n' ORDER BY version) FROM public.schema_migrations`).Scan(&history); err != nil {
		t.Fatal(err)
	}
	return history
}

func assertOverlapRejected(ctx context.Context, conn *pgx.Conn) error {
	_, err := conn.Exec(ctx, `INSERT INTO public.ocupacion(id,espacio_id,reserva_id,intervalo,tipo,activo,motivo)
		VALUES('eeeeeeee-eeee-4eee-8eee-eeeeeeeeee03','dddddddd-dddd-4ddd-8ddd-dddddddddddd',NULL,
		tstzrange('2026-09-06 04:30:00+00','2026-09-06 05:30:00+00','[)'),'bloqueo_manual',true,'overlap must reject')`)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23P01" {
		return fmt.Errorf("restored exclusion constraint returned %v; want SQLSTATE 23P01", err)
	}
	return nil
}

func assertConcurrentOverlap(ctx context.Context, dbURL string) (int, error) {
	const startID = "eeeeeeee-eeee-4eee-8eee-eeeeeeeeee04"
	const endID = "eeeeeeee-eeee-4eee-8eee-eeeeeeeeee05"
	start := time.Date(2026, time.October, 1, 12, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	gate := make(chan struct{})
	errs := make([]error, 2)
	retries := make([]int, 2)
	ids := []string{startID, endID}
	var wg sync.WaitGroup
	for i := range ids {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			conn, err := pgx.Connect(ctx, dbURL)
			if err != nil {
				errs[i] = err
				return
			}
			defer conn.Close(context.Background())
			<-gate
			retries[i], errs[i] = insertWithDeadlockRetry(ctx, func(attemptCtx context.Context) error {
				return insertConcurrentOccupancy(attemptCtx, conn, ids[i], start, end)
			})
		}(i)
	}
	close(gate)
	wg.Wait()
	wins, conflicts := 0, 0
	for _, err := range errs {
		if err == nil {
			wins++
			continue
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23P01" {
			conflicts++
			continue
		}
		return 0, fmt.Errorf("unexpected concurrent insert error: %w", err)
	}
	if wins != 1 || conflicts != 1 {
		return 0, fmt.Errorf("concurrent overlapping inserts successes/conflicts=%d/%d, want 1/1", wins, conflicts)
	}
	checkConn, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		return 0, fmt.Errorf("connect to verify committed concurrent rows: %w", err)
	}
	defer checkConn.Close(context.Background())
	var persisted int
	if err := checkConn.QueryRow(ctx, `SELECT count(*) FROM public.ocupacion WHERE id IN ($1,$2) AND activo`, startID, endID).Scan(&persisted); err != nil {
		return 0, fmt.Errorf("count persisted concurrent overlap rows: %w", err)
	}
	if persisted != 1 {
		return 0, fmt.Errorf("persisted concurrent overlap rows=%d, want exactly one committed row", persisted)
	}
	if retries[0]+retries[1] > 2*(concurrentDeadlockMaxAttempts-1) {
		return 0, fmt.Errorf("deadlock retry count exceeded bound: %v", retries)
	}
	return retries[0] + retries[1], nil
}

const concurrentDeadlockMaxAttempts = 4

// insertWithDeadlockRetry retries only a PostgreSQL deadlock victim. The
// callback must roll back a failed transaction before returning; callers reuse
// the exact same statement and payload on each bounded retry.
func insertWithDeadlockRetry(ctx context.Context, insert func(context.Context) error) (int, error) {
	retries := 0
	for attempt := 1; attempt <= concurrentDeadlockMaxAttempts; attempt++ {
		err := insert(ctx)
		if err == nil {
			return retries, nil
		}
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "40P01" {
			return retries, err
		}
		if attempt == concurrentDeadlockMaxAttempts {
			return retries, fmt.Errorf("deadlock retry limit reached after %d attempts: %w", attempt, err)
		}
		retries++
		timer := time.NewTimer(time.Duration(retries) * 5 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return retries, ctx.Err()
		case <-timer.C:
		}
	}
	return retries, errors.New("unreachable deadlock retry state")
}

func insertConcurrentOccupancy(ctx context.Context, conn *pgx.Conn, id string, start, end time.Time) error {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO public.ocupacion(id,espacio_id,reserva_id,intervalo,tipo,activo,motivo)
		VALUES($1,'dddddddd-dddd-4ddd-8ddd-dddddddddddd',NULL,tstzrange($2,$3,'[)'),'bloqueo_manual',true,'concurrency fixture')`, id, start, end)
	if err != nil {
		rollbackErr := tx.Rollback(context.Background())
		if rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) {
			return fmt.Errorf("insert failed (%v) and rollback failed: %v", err, rollbackErr)
		}
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		rollbackErr := tx.Rollback(context.Background())
		if rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) {
			return fmt.Errorf("commit failed (%v) and rollback failed: %v", err, rollbackErr)
		}
		return err
	}
	return nil
}
