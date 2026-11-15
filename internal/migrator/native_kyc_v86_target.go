package migrator

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

const nativeV86TargetSQL = `SELECT current_database(),d.oid::bigint,current_setting('server_version_num'),session_user::text,current_user::text,current_setting('transaction_read_only')='on',pg_catalog.has_schema_privilege(current_user,'public','CREATE') FROM pg_catalog.pg_database d WHERE d.datname=current_database()`

// NativeKYCMigrationRequiresCreate reports whether the native KYC runner
// requires effective CREATE on public for the pinned migration version.
func NativeKYCMigrationRequiresCreate(version int64) bool {
	return version == 86 || version >= 88 && version <= 90 || version == 92 || version >= 93 && version <= 96
}

func guardNativeV86ExecutionTarget(ctx context.Context, q migrationRowQueryer, requireCreate bool) error {
	var database, version, session, current string
	var oid int64
	var readOnly, canCreate bool
	if err := q.QueryRow(ctx, nativeV86TargetSQL).Scan(&database, &oid, &version, &session, &current, &readOnly, &canCreate); err != nil {
		return errors.New("native V86 execution target readback failed")
	}
	if database != "espacigo_dms_stage_20261020" || oid != 16388 || version != "180007" || session != "espacigo_migrator_login" || current != "espacigo_schema_owner" {
		return errors.New("native V86 execution target or identity mismatch")
	}
	if readOnly {
		return errors.New("native V86 execution transaction is read-only")
	}
	if requireCreate && !canCreate {
		return errors.New("native V86 execution role lacks required public CREATE privilege")
	}
	return nil
}

func RunNativeKYCTail87(ctx context.Context, databaseURL, migrationDir string) (Result, error) {
	config, err := pgx.ParseConfig(databaseURL)
	if err != nil {
		return Result{}, errors.New("invalid native V87 connection configuration")
	}
	return runWithConfigOptions(ctx, config, migrationDir, "espacigo_schema_owner", &migrationRange{minApplied: 86, maxVersion: 87}, true)
}
func RunNativeKYCTail88(ctx context.Context, databaseURL, migrationDir string) (Result, error) {
	config, err := pgx.ParseConfig(databaseURL)
	if err != nil {
		return Result{}, errors.New("invalid native V88 connection configuration")
	}
	return runWithConfigOptions(ctx, config, migrationDir, "espacigo_schema_owner", &migrationRange{minApplied: 87, maxVersion: 88}, true)
}

func RunNativeKYCTail89(ctx context.Context, databaseURL, migrationDir string) (Result, error) {
	config, err := pgx.ParseConfig(databaseURL)
	if err != nil {
		return Result{}, errors.New("invalid native V89 connection configuration")
	}
	return runWithConfigOptions(ctx, config, migrationDir, "espacigo_schema_owner", &migrationRange{minApplied: 88, maxVersion: 89}, true)
}

func RunNativeKYCTail90(ctx context.Context, databaseURL, migrationDir string) (Result, error) {
	config, err := pgx.ParseConfig(databaseURL)
	if err != nil {
		return Result{}, errors.New("invalid native V90 connection configuration")
	}
	return runWithConfigOptions(ctx, config, migrationDir, "espacigo_schema_owner", &migrationRange{minApplied: 89, maxVersion: 90}, true)
}

func RunNativeKYCTail91(ctx context.Context, databaseURL, migrationDir string) (Result, error) {
	config, err := pgx.ParseConfig(databaseURL)
	if err != nil {
		return Result{}, errors.New("invalid native V91 connection configuration")
	}
	return runWithConfigOptions(ctx, config, migrationDir, "espacigo_schema_owner", &migrationRange{minApplied: 90, maxVersion: 91}, true)
}

// RunNativeKYCV86 rechecks the native target on the locked execution connection
// and in the V86 transaction, in addition to the standard ledger/authority gates.
func RunNativeKYCV86(ctx context.Context, databaseURL, migrationDir string) (Result, error) {
	config, err := pgx.ParseConfig(databaseURL)
	if err != nil {
		return Result{}, errors.New("invalid native V86 connection configuration")
	}
	return runWithConfigOptions(ctx, config, migrationDir, "espacigo_schema_owner", &migrationRange{minApplied: 85, maxVersion: 86}, true)
}

func RunNativeKYCTail92(ctx context.Context, databaseURL, migrationDir string) (Result, error) {
	config, err := pgx.ParseConfig(databaseURL)
	if err != nil {
		return Result{}, errors.New("invalid native V92 connection configuration")
	}
	return runWithConfigOptions(ctx, config, migrationDir, "espacigo_schema_owner", &migrationRange{minApplied: 91, maxVersion: 92}, true)
}

func RunNativeKYCTail93(ctx context.Context, databaseURL, migrationDir string) (Result, error) {
	config, err := pgx.ParseConfig(databaseURL)
	if err != nil {
		return Result{}, errors.New("invalid native V93 connection configuration")
	}
	return runWithConfigOptions(ctx, config, migrationDir, "espacigo_schema_owner", &migrationRange{minApplied: 92, maxVersion: 93}, true)
}

func RunNativeKYCTail94(ctx context.Context, databaseURL, migrationDir string) (Result, error) {
	config, err := pgx.ParseConfig(databaseURL)
	if err != nil {
		return Result{}, errors.New("invalid native V94 connection configuration")
	}
	return runWithConfigOptions(ctx, config, migrationDir, "espacigo_schema_owner", &migrationRange{minApplied: 93, maxVersion: 94}, true)
}

func RunNativeKYCTail95(ctx context.Context, databaseURL, migrationDir string) (Result, error) {
	config, err := pgx.ParseConfig(databaseURL)
	if err != nil {
		return Result{}, errors.New("invalid native V95 connection configuration")
	}
	return runWithConfigOptions(ctx, config, migrationDir, "espacigo_schema_owner", &migrationRange{minApplied: 94, maxVersion: 95}, true)
}

func RunNativeKYCTail96(ctx context.Context, databaseURL, migrationDir string) (Result, error) {
	config, err := pgx.ParseConfig(databaseURL)
	if err != nil {
		return Result{}, errors.New("invalid native V96 connection configuration")
	}
	return runWithConfigOptions(ctx, config, migrationDir, "espacigo_schema_owner", &migrationRange{minApplied: 95, maxVersion: 96}, true)
}
