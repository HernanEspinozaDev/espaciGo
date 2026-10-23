package migrator

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/HernanEspinozaDev/espaciGo/internal/database/cloudsql"
	"github.com/HernanEspinozaDev/espaciGo/internal/dbbootstrap"
	"github.com/jackc/pgx/v5"
)

const (
	advisoryLockClass  int32 = 0x45535047 // "ESPG"
	advisoryLockObject int32 = 0x4d494752 // "MIGR"
)

func validDatabaseRoleName(value string) bool {
	if len(value) == 0 || len(value) > 63 {
		return false
	}
	for i, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_' ||
			(i > 0 && r >= '0' && r <= '9') {
			continue
		}
		return false
	}
	return true
}

type Result struct {
	Applied []int64
}

type MigrationSummary struct {
	Version  int64  `json:"version"`
	Name     string `json:"name"`
	Checksum string `json:"checksum"`
}

type PlanResult struct {
	Database      string             `json:"database"`
	ServerVersion string             `json:"server_version"`
	ReadOnly      bool               `json:"read_only"`
	HistoryExists bool               `json:"history_exists"`
	Applied       []HistoryEntry     `json:"applied"`
	Pending       []MigrationSummary `json:"pending"`
}

func Run(ctx context.Context, databaseURL, migrationDir string) (Result, error) {
	return RunWithRole(ctx, databaseURL, migrationDir, "")
}

func RunWithRole(ctx context.Context, databaseURL, migrationDir, setRole string) (Result, error) {
	config, err := pgx.ParseConfig(databaseURL)
	if err != nil {
		return Result{}, errors.New("invalid database connection configuration")
	}
	return runWithConfig(ctx, config, migrationDir, setRole)
}

// RunCloudSQL runs the existing migration sequence through Cloud SQL Go
// Connector. databaseURL supplies PostgreSQL username/database; password is
// supplied separately from the secret file, and the URL network host is a
// validated localhost placeholder ignored by the connector.
func RunCloudSQL(ctx context.Context, databaseURL, password, migrationDir, instance string, privateIP bool) (Result, error) {
	return RunCloudSQLWithRole(ctx, databaseURL, password, migrationDir, instance, privateIP, "")
}

func RunCloudSQLWithRole(ctx context.Context, databaseURL, password, migrationDir, instance string, privateIP bool, setRole string) (Result, error) {
	config, err := cloudsql.NewPGXConfig(databaseURL, password)
	if err != nil {
		return Result{}, err
	}
	closeConnector, err := cloudsql.ConfigurePGX(ctx, config, instance, privateIP)
	if err != nil {
		return Result{}, err
	}
	return runWithConnector(ctx, config, migrationDir, setRole, closeConnector)
}

// Plan reports the applied and pending migration inventory in a read-only
// transaction. It does not create a history table, acquire an advisory lock,
// or execute migration SQL.
func Plan(ctx context.Context, databaseURL, migrationDir, setRole string) (PlanResult, error) {
	config, err := pgx.ParseConfig(databaseURL)
	if err != nil {
		return PlanResult{}, errors.New("invalid database connection configuration")
	}
	return planWithConfig(ctx, config, migrationDir, setRole)
}

// PlanCloudSQL runs Plan using the Cloud SQL connector and a separately
// supplied password read from a protected file by the caller.
func PlanCloudSQL(ctx context.Context, databaseURL, password, migrationDir, instance string, privateIP bool, setRole string) (PlanResult, error) {
	config, err := cloudsql.NewPGXConfig(databaseURL, password)
	if err != nil {
		return PlanResult{}, err
	}
	closeConnector, err := cloudsql.ConfigurePGX(ctx, config, instance, privateIP)
	if err != nil {
		return PlanResult{}, err
	}
	defer closeConnector()
	return planWithConfig(ctx, config, migrationDir, setRole)
}

func planWithConfig(ctx context.Context, config *pgx.ConnConfig, migrationDir, setRole string) (PlanResult, error) {
	migrations, err := DiscoverMigrations(migrationDir)
	if err != nil {
		return PlanResult{}, err
	}
	conn, err := pgx.ConnectConfig(ctx, config)
	if err != nil {
		return PlanResult{}, errors.New("connect to migration database")
	}
	defer conn.Close(context.Background())
	if setRole != "" {
		if !validDatabaseRoleName(setRole) {
			return PlanResult{}, errors.New("configured database role name is invalid")
		}
	}
	tx, err := conn.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return PlanResult{}, errors.New("begin read-only migration plan")
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if setRole != "" {
		if _, err := tx.Exec(ctx, "SET ROLE "+pgx.Identifier{setRole}.Sanitize()); err != nil {
			return PlanResult{}, errors.New("could not assume configured database role")
		}
	}
	var report PlanResult
	if err := tx.QueryRow(ctx, `SELECT current_database(), current_setting('server_version'), current_setting('transaction_read_only') = 'on'`).Scan(&report.Database, &report.ServerVersion, &report.ReadOnly); err != nil {
		return PlanResult{}, errors.New("read migration plan target identity")
	}
	if !report.ReadOnly {
		return PlanResult{}, errors.New("migration plan transaction is not read-only")
	}
	if err := guardHistoryInitialization(ctx, tx); err != nil {
		return PlanResult{}, err
	}
	var historyKind string
	if err := tx.QueryRow(ctx, `SELECT COALESCE((SELECT c.relkind::text FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relname='schema_migrations'), '')`).Scan(&historyKind); err != nil {
		return PlanResult{}, errors.New("inspect migration history relation")
	}
	report.HistoryExists = historyKind == "r" || historyKind == "p"
	var history []HistoryEntry
	if report.HistoryExists {
		history, err = readHistory(ctx, tx)
		if err != nil {
			return PlanResult{}, err
		}
		if err := verifyHistory(migrations, history); err != nil {
			return PlanResult{}, err
		}
		if err := guardAppliedSchemaObjects(ctx, tx, history); err != nil {
			return PlanResult{}, err
		}
	}
	report.Applied = history
	for _, migration := range migrations[len(history):] {
		report.Pending = append(report.Pending, MigrationSummary{Version: migration.Version, Name: migration.Name, Checksum: migration.Checksum})
	}
	if report.Applied == nil {
		report.Applied = []HistoryEntry{}
	}
	if report.Pending == nil {
		report.Pending = []MigrationSummary{}
	}
	return report, nil
}

func runWithConnector(ctx context.Context, config *pgx.ConnConfig, migrationDir, setRole string, closeConnector func()) (Result, error) {
	defer closeConnector()
	return runWithConfig(ctx, config, migrationDir, setRole)
}

func runWithConfig(ctx context.Context, config *pgx.ConnConfig, migrationDir, setRole string) (Result, error) {
	conn, err := pgx.ConnectConfig(ctx, config)
	if err != nil {
		return Result{}, errors.New("connect to migration database")
	}
	defer conn.Close(context.Background())
	if setRole != "" {
		if !validDatabaseRoleName(setRole) {
			return Result{}, errors.New("configured database role name is invalid")
		}
		if _, err := conn.Exec(ctx, "SET ROLE "+pgx.Identifier{setRole}.Sanitize()); err != nil {
			return Result{}, errors.New("could not assume configured database role")
		}
	}
	result, err := runWithConn(ctx, conn, migrationDir)
	if err != nil {
		return result, err
	}
	if setRole == "espacigo_schema_owner" {
		if err := dbbootstrap.GrantRuntimePermissions(ctx, conn); err != nil {
			return result, fmt.Errorf("apply restricted runtime grants: %w", err)
		}
	}
	return result, nil
}

func runWithConn(ctx context.Context, conn *pgx.Conn, migrationDir string) (Result, error) {
	migrations, err := DiscoverMigrations(migrationDir)
	if err != nil {
		return Result{}, err
	}
	runnerDigest, err := executableChecksum()
	if err != nil {
		return Result{}, fmt.Errorf("checksum migrator executable: %w", err)
	}

	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1, $2)", advisoryLockClass, advisoryLockObject); err != nil {
		return Result{}, fmt.Errorf("acquire migration advisory lock: %w", err)
	}
	defer func() {
		_, _ = conn.Exec(context.Background(), "SELECT pg_advisory_unlock($1, $2)", advisoryLockClass, advisoryLockObject)
	}()

	if err := guardHistoryInitialization(ctx, conn); err != nil {
		return Result{}, err
	}
	const createHistory = `CREATE TABLE IF NOT EXISTS public.schema_migrations (
		version bigint PRIMARY KEY CHECK (version > 0),
		name text NOT NULL UNIQUE,
		checksum text NOT NULL CHECK (checksum ~ '^[0-9a-f]{64}$'),
		runner_checksum text NOT NULL CHECK (runner_checksum ~ '^[0-9a-f]{64}$'),
		applied_at timestamptz NOT NULL DEFAULT clock_timestamp()
	)`
	if _, err := conn.Exec(ctx, createHistory); err != nil {
		return Result{}, fmt.Errorf("initialize migration history: %w", err)
	}
	history, err := readHistory(ctx, conn)
	if err != nil {
		return Result{}, err
	}
	if err := verifyHistory(migrations, history); err != nil {
		return Result{}, err
	}
	if err := guardAppliedSchemaObjects(ctx, conn, history); err != nil {
		return Result{}, err
	}

	result := Result{}
	for _, migration := range migrations[len(history):] {
		tx, err := conn.Begin(ctx)
		if err != nil {
			return result, fmt.Errorf("begin migration V%06d: %w", migration.Version, err)
		}
		if _, err := tx.Conn().PgConn().Exec(ctx, string(migration.SQL)).ReadAll(); err != nil {
			_ = tx.Rollback(context.Background())
			return result, fmt.Errorf("execute migration V%06d (%s): %w", migration.Version, migration.Name, err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.schema_migrations
			(version, name, checksum, runner_checksum)
			VALUES ($1, $2, $3, $4)`,
			migration.Version, migration.Name, migration.Checksum, runnerDigest); err != nil {
			_ = tx.Rollback(context.Background())
			return result, fmt.Errorf("record migration V%06d: %w", migration.Version, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return result, fmt.Errorf("commit migration V%06d: %w", migration.Version, err)
		}
		result.Applied = append(result.Applied, migration.Version)
	}
	return result, nil
}

// guardHistoryInitialization prevents an existing application schema from
// being mistaken for an empty database when its migration ledger is absent.
// Replaying V1 in that state could collide with, or overwrite, existing data.
type queryRower interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func guardHistoryInitialization(ctx context.Context, conn queryRower) error {
	var historyKind string
	var hasApplicationObjects bool
	err := conn.QueryRow(ctx, `
		SELECT COALESCE((
			SELECT c.relkind::text
			FROM pg_catalog.pg_class c
			JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
			WHERE n.nspname = 'public' AND c.relname = 'schema_migrations'
		), ''), EXISTS (
			SELECT 1
			FROM pg_catalog.pg_class c
			JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
			WHERE n.nspname = 'public'
			  AND c.relname <> 'schema_migrations'
			  AND c.relkind IN ('r','p','v','m','S','f')
			  AND NOT EXISTS (
				SELECT 1 FROM pg_catalog.pg_depend d
				WHERE d.classid = 'pg_catalog.pg_class'::regclass
				  AND d.objid = c.oid AND d.deptype = 'e'
			  )
		)`,
	).Scan(&historyKind, &hasApplicationObjects)
	if err != nil {
		return fmt.Errorf("inspect migration history baseline: %w", err)
	}
	if historyKind != "" && historyKind != "r" {
		return errors.New("migration history name exists but is not a regular table; refusing migration")
	}
	if historyKind == "" && hasApplicationObjects {
		return errors.New("application objects exist without public.schema_migrations; refusing to initialize or replay migration history")
	}
	return nil
}

// guardAppliedSchemaObjects detects known cases where an applied migration's
// ledger row remains but its critical objects are missing. Such drift requires
// a reviewed repair; the runner must not continue to later migrations.
func guardAppliedSchemaObjects(ctx context.Context, conn queryRower, history []HistoryEntry) error {
	if len(history) == 0 {
		return nil
	}
	maxVersion := history[len(history)-1].Version
	checks := []struct {
		version int64
		query   string
		label   string
	}{
		{64, `SELECT EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname='espacigo_privacy_cleanup_worker')`, "V000064 cleanup worker role"},
		{68, `SELECT to_regclass('public.space_media') IS NOT NULL AND to_regclass('public.media_asset') IS NOT NULL AND to_regclass('public.media_variant') IS NOT NULL`, "V000068 media relations"},
		{73, `SELECT to_regprocedure('public.claim_private_media_cleanup(integer,timestamp with time zone)') IS NOT NULL AND to_regprocedure('public.complete_private_media_cleanup(uuid,bigint,timestamp with time zone)') IS NOT NULL AND to_regprocedure('public.fail_private_media_cleanup(uuid,bigint,timestamp with time zone)') IS NOT NULL AND to_regprocedure('public.finalize_pending_suppression_cleanup(uuid,timestamp with time zone)') IS NOT NULL`, "V000073 cleanup routines"},
		{74, `SELECT to_regclass('public.rol_usuario_evento') IS NOT NULL`, "V000074 role event relation"},
	}
	for _, check := range checks {
		if maxVersion < check.version {
			continue
		}
		var present bool
		if err := conn.QueryRow(ctx, check.query).Scan(&present); err != nil {
			return fmt.Errorf("inspect applied migration prerequisites: %w", err)
		}
		if !present {
			return fmt.Errorf("migration history says %s is applied but a required object is absent; refusing further migrations", check.label)
		}
	}
	return nil
}

// VerifyAllApplied checks that every migration file in migrationDir has one
// matching, contiguous history row and that there are no unapplied files.
func VerifyAllApplied(ctx context.Context, conn migrationHistoryQueryer, migrationDir string, expectedCount int) error {
	if conn == nil || expectedCount < 1 {
		return errors.New("migration verification configuration is invalid")
	}
	migrations, err := DiscoverMigrations(migrationDir)
	if err != nil {
		return err
	}
	history, err := readHistory(ctx, conn)
	if err != nil {
		return err
	}
	return verifyCompleteHistory(migrations, history, expectedCount)
}

func verifyCompleteHistory(migrations []Migration, history []HistoryEntry, expectedCount int) error {
	if expectedCount < 1 || len(migrations) != expectedCount {
		return fmt.Errorf("migration inventory count is %d, want %d", len(migrations), expectedCount)
	}
	if err := verifyHistory(migrations, history); err != nil {
		return err
	}
	if len(history) != expectedCount {
		return fmt.Errorf("migration history count is %d, want %d", len(history), expectedCount)
	}
	return nil
}

type migrationHistoryQueryer interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func readHistory(ctx context.Context, conn migrationHistoryQueryer) ([]HistoryEntry, error) {
	rows, err := conn.Query(ctx, `SELECT version, name, checksum
		FROM public.schema_migrations ORDER BY version`)
	if err != nil {
		return nil, fmt.Errorf("read migration history: %w", err)
	}
	defer rows.Close()
	var history []HistoryEntry
	for rows.Next() {
		var entry HistoryEntry
		if err := rows.Scan(&entry.Version, &entry.Name, &entry.Checksum); err != nil {
			return nil, fmt.Errorf("decode migration history: %w", err)
		}
		history = append(history, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read migration history: %w", err)
	}
	return history, nil
}

func executableChecksum() (string, error) {
	path, err := os.Executable()
	if err != nil {
		return "", err
	}
	binary, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return sha256Hex(binary), nil
}
