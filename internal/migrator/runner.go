package migrator

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/HernanEspinozaDev/espaciGo/internal/database/cloudsql"
	"github.com/jackc/pgx/v5"
)

const (
	advisoryLockClass  int32 = 0x45535047 // "ESPG"
	advisoryLockObject int32 = 0x4d494752 // "MIGR"
)

type Result struct {
	Applied []int64
}

func Run(ctx context.Context, databaseURL, migrationDir string) (Result, error) {
	config, err := pgx.ParseConfig(databaseURL)
	if err != nil {
		return Result{}, errors.New("invalid database connection configuration")
	}
	return runWithConfig(ctx, config, migrationDir)
}

// RunCloudSQL runs the existing migration sequence through Cloud SQL Go
// Connector. databaseURL still supplies PostgreSQL credentials/database; its
// network host is ignored by the connector.
func RunCloudSQL(ctx context.Context, databaseURL, migrationDir, instance string, privateIP bool) (Result, error) {
	config, err := pgx.ParseConfig(databaseURL)
	if err != nil {
		return Result{}, errors.New("invalid database connection configuration")
	}
	if config.TLSConfig != nil {
		return Result{}, errors.New("Cloud SQL connector mode requires sslmode=disable; connector TLS is managed separately")
	}
	closeConnector, err := cloudsql.ConfigurePGX(ctx, config, instance, privateIP)
	if err != nil {
		return Result{}, err
	}
	return runWithConnector(ctx, config, migrationDir, closeConnector)
}

func runWithConnector(ctx context.Context, config *pgx.ConnConfig, migrationDir string, closeConnector func()) (Result, error) {
	defer closeConnector()
	return runWithConfig(ctx, config, migrationDir)
}

func runWithConfig(ctx context.Context, config *pgx.ConnConfig, migrationDir string) (Result, error) {
	migrations, err := DiscoverMigrations(migrationDir)
	if err != nil {
		return Result{}, err
	}
	runnerDigest, err := executableChecksum()
	if err != nil {
		return Result{}, fmt.Errorf("checksum migrator executable: %w", err)
	}
	conn, err := pgx.ConnectConfig(ctx, config)
	if err != nil {
		return Result{}, errors.New("connect to migration database")
	}
	defer conn.Close(context.Background())

	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1, $2)", advisoryLockClass, advisoryLockObject); err != nil {
		return Result{}, fmt.Errorf("acquire migration advisory lock: %w", err)
	}
	defer func() {
		_, _ = conn.Exec(context.Background(), "SELECT pg_advisory_unlock($1, $2)", advisoryLockClass, advisoryLockObject)
	}()

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

func readHistory(ctx context.Context, conn *pgx.Conn) ([]HistoryEntry, error) {
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
