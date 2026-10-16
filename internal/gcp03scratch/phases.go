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
	Bootstrap               func(context.Context) error
	DatabaseACL             func(context.Context) error
	Migrate                 func(context.Context) error
	Finalize                func(context.Context) error
	Retire                  func(context.Context) error
	GrantOwnerHistory       func(context.Context) error
	VerifyRetired           func(context.Context) error
	RevokeRetiredMembership func(context.Context) error
}

// ExecutePhase dispatches exactly one named phase. Cloud Run Jobs use separate
// immutable phase values, so a failed phase cannot fall through into another.
func ExecutePhase(ctx context.Context, phase Phase, actions PhaseActions) error {
	var action func(context.Context) error
	switch phase {
	case PhaseBootstrap:
		action = actions.Bootstrap
	case PhaseDatabaseACL:
		action = actions.DatabaseACL
	case PhaseMigrate:
		action = actions.Migrate
	case PhaseFinalize:
		action = actions.Finalize
	case PhaseRetire:
		action = actions.Retire
	case PhaseGrantOwnerHistory:
		action = actions.GrantOwnerHistory
	case PhaseVerifyRetired:
		action = actions.VerifyRetired
	case PhaseRevokeRetiredMembership:
		action = actions.RevokeRetiredMembership
	default:
		return errors.New("unsupported GCP-03 scratch phase")
	}
	if action == nil {
		return errors.New("GCP-03 scratch phase handler is unavailable")
	}
	return action(ctx)
}

func RunPhase(ctx context.Context, config Config, password, migrationDir string) error {
	expectedLogin, expectedURL, expectedPasswordFile := MigratorLogin, DatabaseURL, PasswordFile
	if config.Phase == PhaseDatabaseACL || config.Phase == PhaseRetire || config.Phase == PhaseVerifyRetired || config.Phase == PhaseRevokeRetiredMembership {
		expectedLogin, expectedURL, expectedPasswordFile = DatabaseOwnerLogin, DatabaseOwnerURL, DatabaseOwnerPasswordFile
	}
	if config.Instance != InstanceConnection || config.DatabaseURL != expectedURL || config.Login != expectedLogin || config.PasswordFile != expectedPasswordFile || !config.PrivateIP {
		return errors.New("GCP-03 target configuration is invalid")
	}
	if !scratchPasswordPattern.MatchString(password) {
		return errors.New("GCP-03 scratch password has an invalid format")
	}
	actions := PhaseActions{
		Bootstrap:         func(ctx context.Context) error { return runBootstrap(ctx, config, password) },
		DatabaseACL:       func(ctx context.Context) error { return runDatabaseACL(ctx, config, password) },
		Migrate:           func(ctx context.Context) error { return runMigrations(ctx, config, password, migrationDir) },
		Finalize:          func(ctx context.Context) error { return runFinalize(ctx, config, password, migrationDir) },
		Retire:            func(ctx context.Context) error { return runRetire(ctx, config, password, migrationDir) },
		GrantOwnerHistory: func(ctx context.Context) error { return runGrantOwnerHistory(ctx, config, password, migrationDir) },
		VerifyRetired:     func(ctx context.Context) error { return runVerifyRetired(ctx, config, password, migrationDir) },
		RevokeRetiredMembership: func(ctx context.Context) error {
			return runRevokeRetiredMembership(ctx, config, password, migrationDir)
		},
	}
	return ExecutePhase(ctx, config.Phase, actions)
}

func runVerifyRetired(ctx context.Context, config Config, password, migrationDir string) error {
	report, err := ObserveRetiredPhase(ctx, config, password, migrationDir)
	if err != nil {
		return err
	}
	if !report.Passed {
		return errors.New("GCP-03 fixed read-only retirement verification found failed checks")
	}
	return nil
}

func ObserveRetiredPhase(ctx context.Context, config Config, password, migrationDir string) (RetiredVerificationReport, error) {
	if config.Phase != PhaseVerifyRetired || config.Instance != InstanceConnection || config.DatabaseURL != DatabaseOwnerURL ||
		config.Login != DatabaseOwnerLogin || config.PasswordFile != DatabaseOwnerPasswordFile || !config.PrivateIP {
		return RetiredVerificationReport{}, errors.New("GCP-03 read-only retirement observer target configuration is invalid")
	}
	var report RetiredVerificationReport
	err := withScratchConnection(ctx, config, password, func(ctx context.Context, conn *pgx.Conn) error {
		if _, err := conn.Exec(ctx, "BEGIN READ ONLY"); err != nil {
			return errors.New("GCP-03 could not begin a read-only retirement verification")
		}
		defer conn.Exec(context.Background(), "ROLLBACK")
		var err error
		report, err = ObserveRetiredScratch(ctx, conn, migrationDir)
		return err
	})
	return report, err
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
	if database != DatabaseName || login != config.Login {
		return errors.New("connected session does not match the reviewed GCP-03 scratch target")
	}
	return operation(ctx, conn)
}

func runBootstrap(ctx context.Context, config Config, password string) error {
	return withScratchConnection(ctx, config, password, func(ctx context.Context, conn *pgx.Conn) error {
		return bootstrapOnConnection(ctx, conn)
	})
}

func bootstrapOnConnection(ctx context.Context, conn *pgx.Conn) error {
	if _, err := conn.PgConn().Exec(ctx, scratchsql.PreMigrate).ReadAll(); err != nil {
		return fmt.Errorf("GCP-03 pre-migration bootstrap SQL failed: %w", err)
	}
	return dbbootstrap.VerifyScratchBootstrapRolesCreated(ctx, conn)
}

func runDatabaseACL(ctx context.Context, config Config, password string) error {
	return withScratchConnection(ctx, config, password, func(ctx context.Context, conn *pgx.Conn) error {
		return databaseACLOnConnection(ctx, conn)
	})
}

func databaseACLOnConnection(ctx context.Context, conn *pgx.Conn) error {
	if _, err := conn.PgConn().Exec(ctx, scratchsql.DatabaseACL).ReadAll(); err != nil {
		return fmt.Errorf("GCP-03 database TEMP ACL SQL failed: %w", err)
	}
	return dbbootstrap.VerifyScratchDatabaseACL(ctx, conn)
}

func runMigrations(ctx context.Context, config Config, password, migrationDir string) error {
	if err := withScratchConnection(ctx, config, password, dbbootstrap.VerifyScratchBootstrap); err != nil {
		return fmt.Errorf("GCP-03 migration preflight failed: %w", err)
	}
	if err := withScratchConnection(ctx, config, password, dbbootstrap.VerifyScratchDatabaseACL); err != nil {
		return fmt.Errorf("GCP-03 migration database ACL preflight failed: %w", err)
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
		return finalizeOnConnection(ctx, conn, migrationDir)
	})
}

func finalizeOnConnection(ctx context.Context, conn *pgx.Conn, migrationDir string) error {
	if err := migrator.VerifyAllApplied(ctx, conn, migrationDir, MigrationCount); err != nil {
		return fmt.Errorf("GCP-03 migration history verification failed: %w", err)
	}
	if err := dbbootstrap.GrantRuntimePermissions(ctx, conn); err != nil {
		return fmt.Errorf("GCP-03 runtime grants failed: %w", err)
	}
	return dbbootstrap.VerifyScratchRuntimePermissions(ctx, conn)
}

func runRetire(ctx context.Context, config Config, password, migrationDir string) error {
	return withScratchConnection(ctx, config, password, func(ctx context.Context, conn *pgx.Conn) error {
		return retireOnConnection(ctx, conn, migrationDir)
	})
}

func runGrantOwnerHistory(ctx context.Context, config Config, password, migrationDir string) error {
	return withScratchConnection(ctx, config, password, func(ctx context.Context, conn *pgx.Conn) error {
		return grantOwnerHistoryOnConnection(ctx, conn, migrationDir)
	})
}

func grantOwnerHistoryOnConnection(ctx context.Context, conn *pgx.Conn, migrationDir string) error {
	if err := migrator.VerifyAllApplied(ctx, conn, migrationDir, MigrationCount); err != nil {
		return fmt.Errorf("GCP-03 owner-history grant migration verification failed: %w", err)
	}
	if err := dbbootstrap.VerifyScratchBootstrap(ctx, conn); err != nil {
		return fmt.Errorf("GCP-03 owner-history grant role verification failed: %w", err)
	}
	if err := dbbootstrap.VerifyScratchRuntimePermissions(ctx, conn); err != nil {
		return fmt.Errorf("GCP-03 owner-history grant runtime ACL verification failed: %w", err)
	}
	var exactTarget bool
	if err := conn.QueryRow(ctx, `SELECT current_database() = 'espacigo_scratch'
		AND current_user = 'espacigo_scratch_migrator'
		AND pg_catalog.pg_get_userbyid(c.relowner) = 'espacigo_scratch_migrator'
		FROM pg_catalog.pg_class c
		JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'public' AND c.relname = 'schema_migrations' AND c.relkind = 'r'`).Scan(&exactTarget); err != nil || !exactTarget {
		return errors.New("GCP-03 owner-history grant target or owner is unexpected")
	}
	if _, err := conn.PgConn().Exec(ctx, scratchsql.GrantOwnerHistory).ReadAll(); err != nil {
		return fmt.Errorf("GCP-03 fixed owner-history grant failed: %w", err)
	}
	var valid bool
	err := conn.QueryRow(ctx, `SELECT has_table_privilege('postgres', c.oid, 'SELECT')
		AND NOT has_table_privilege('espacigo_runtime', c.oid, 'SELECT')
		AND NOT EXISTS (
			SELECT 1 FROM pg_catalog.aclexplode(coalesce(c.relacl, pg_catalog.acldefault('r', c.relowner))) acl
			WHERE acl.grantee = 0 AND acl.privilege_type = 'SELECT'
		)
		AND NOT EXISTS (
			SELECT 1 FROM pg_catalog.aclexplode(coalesce(c.relacl, pg_catalog.acldefault('r', c.relowner))) acl
			JOIN pg_catalog.pg_roles r ON r.oid = acl.grantee
			WHERE r.rolname = 'postgres' AND acl.privilege_type = 'SELECT' AND acl.is_grantable
		)
		FROM pg_catalog.pg_class c
		JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'public' AND c.relname = 'schema_migrations' AND c.relkind = 'r'`).Scan(&valid)
	if err != nil || !valid {
		return errors.New("GCP-03 owner-history grant did not produce the exact restricted privilege")
	}
	return nil
}

func retireOnConnection(ctx context.Context, conn *pgx.Conn, migrationDir string) error {
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
}
