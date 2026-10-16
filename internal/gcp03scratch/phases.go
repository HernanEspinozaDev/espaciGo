package gcp03scratch

import (
	"context"
	"errors"
	"fmt"

	scratchsql "github.com/HernanEspinozaDev/espaciGo/db/bootstrap/gcp03scratch"
	"github.com/HernanEspinozaDev/espaciGo/internal/database/cloudsql"
	"github.com/HernanEspinozaDev/espaciGo/internal/dbbootstrap"
	"github.com/HernanEspinozaDev/espaciGo/internal/migrator"
	"github.com/jackc/pgx/v5"
)

type PhaseActions struct {
	Bootstrap func(context.Context) error
	Migrate   func(context.Context) error
	Finalize  func(context.Context) error
	Retire    func(context.Context) error
}

// ExecutePhase dispatches exactly one named phase. Cloud Run Jobs use separate
// immutable phase values, so a failed phase cannot fall through into another.
func ExecutePhase(ctx context.Context, phase Phase, actions PhaseActions) error {
	var action func(context.Context) error
	switch phase {
	case PhaseBootstrap:
		action = actions.Bootstrap
	case PhaseMigrate:
		action = actions.Migrate
	case PhaseFinalize:
		action = actions.Finalize
	case PhaseRetire:
		action = actions.Retire
	default:
		return errors.New("unsupported GCP-03 scratch phase")
	}
	if action == nil {
		return errors.New("GCP-03 scratch phase handler is unavailable")
	}
	return action(ctx)
}

func RunPhase(ctx context.Context, config Config, password, migrationDir string) error {
	if config.Instance != InstanceConnection || config.DatabaseURL != DatabaseURL || !config.PrivateIP {
		return errors.New("GCP-03 target configuration is invalid")
	}
	if !scratchPasswordPattern.MatchString(password) {
		return errors.New("GCP-03 scratch password has an invalid format")
	}
	actions := PhaseActions{
		Bootstrap: func(ctx context.Context) error { return runBootstrap(ctx, config, password) },
		Migrate:   func(ctx context.Context) error { return runMigrations(ctx, config, password, migrationDir) },
		Finalize:  func(ctx context.Context) error { return runFinalize(ctx, config, password, migrationDir) },
		Retire:    func(ctx context.Context) error { return runRetire(ctx, config, password, migrationDir) },
	}
	return ExecutePhase(ctx, config.Phase, actions)
}

func ReadScratchExtensionVersions(ctx context.Context, config Config, password string) (map[string]string, error) {
	var versions map[string]string
	err := withScratchConnection(ctx, config, password, func(ctx context.Context, conn *pgx.Conn) error {
		var err error
		versions, err = dbbootstrap.ScratchExtensionVersions(ctx, conn)
		return err
	})
	return versions, err
}

func withScratchConnection(ctx context.Context, config Config, password string, operation func(context.Context, *pgx.Conn) error) error {
	conn, closeConnector, err := cloudsql.Connect(ctx, config.DatabaseURL, password, config.Instance, config.PrivateIP)
	if err != nil {
		return err
	}
	defer closeConnector()
	defer conn.Close(context.Background())
	var database, login string
	if err := conn.QueryRow(ctx, "SELECT current_database(), current_user").Scan(&database, &login); err != nil {
		return errors.New("could not verify the GCP-03 database session")
	}
	if database != DatabaseName || login != MigratorLogin {
		return errors.New("connected session does not match the reviewed GCP-03 scratch target")
	}
	return operation(ctx, conn)
}

func runBootstrap(ctx context.Context, config Config, password string) error {
	return withScratchConnection(ctx, config, password, func(ctx context.Context, conn *pgx.Conn) error {
		if _, err := conn.PgConn().Exec(ctx, scratchsql.PreMigrate).ReadAll(); err != nil {
			return fmt.Errorf("GCP-03 pre-migration bootstrap SQL failed: %w", err)
		}
		return dbbootstrap.VerifyScratchBootstrap(ctx, conn)
	})
}

func runMigrations(ctx context.Context, config Config, password, migrationDir string) error {
	if err := withScratchConnection(ctx, config, password, dbbootstrap.VerifyScratchBootstrap); err != nil {
		return fmt.Errorf("GCP-03 migration preflight failed: %w", err)
	}
	result, err := migrator.RunCloudSQL(ctx, config.DatabaseURL, password, migrationDir, config.Instance, config.PrivateIP)
	if err != nil {
		return fmt.Errorf("GCP-03 V1-V61 migration phase failed: %w", err)
	}
	if len(result.Applied) > MigrationCount {
		return errors.New("GCP-03 migration runner reported an unexpected result")
	}
	return nil
}

func runFinalize(ctx context.Context, config Config, password, migrationDir string) error {
	return withScratchConnection(ctx, config, password, func(ctx context.Context, conn *pgx.Conn) error {
		if err := migrator.VerifyAllApplied(ctx, conn, migrationDir, MigrationCount); err != nil {
			return fmt.Errorf("GCP-03 migration history verification failed: %w", err)
		}
		if err := dbbootstrap.GrantRuntimePermissions(ctx, conn); err != nil {
			return fmt.Errorf("GCP-03 runtime grants failed: %w", err)
		}
		if err := dbbootstrap.VerifyScratchRuntimePermissions(ctx, conn); err != nil {
			return err
		}
		return nil
	})
}

func runRetire(ctx context.Context, config Config, password, migrationDir string) error {
	return withScratchConnection(ctx, config, password, func(ctx context.Context, conn *pgx.Conn) error {
		if err := migrator.VerifyAllApplied(ctx, conn, migrationDir, MigrationCount); err != nil {
			return fmt.Errorf("GCP-03 retirement migration history verification failed: %w", err)
		}
		if err := dbbootstrap.VerifyScratchRuntimePermissions(ctx, conn); err != nil {
			return err
		}
		if _, err := conn.PgConn().Exec(ctx, scratchsql.RetireMigrator).ReadAll(); err != nil {
			return fmt.Errorf("GCP-03 temporary login retirement failed: %w", err)
		}
		return dbbootstrap.VerifyScratchMigratorRetired(ctx, conn)
	})
}
