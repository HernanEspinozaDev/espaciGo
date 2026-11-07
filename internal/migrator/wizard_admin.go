package migrator

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"syscall"

	"github.com/jackc/pgx/v5"
)

const wizardTrialDatabaseURL = "postgres://postgres@127.0.0.1:15436/espacigo_wizard_20261106?sslmode=disable"

var wizardAdminVersions = map[int64]bool{60: true, 64: true, 66: true}
var wizardChecksumPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

// RunWizardAdministrativeMigration executes exactly one reviewed wizard-trial
// migration against its explicitly authorized local PostgreSQL target. The
// expected argument must be the independent, ordered 1..83 review manifest.
func RunWizardAdministrativeMigration(ctx context.Context, databaseURL, migrationDir string, version int64, expected []MigrationSummary) (Result, error) {
	return runWizardAdministrativeMigration(ctx, databaseURL, migrationDir, version, expected, nil)
}

// runWizardAdministrativeMigration keeps fault injection private to package tests;
// the hook runs after SQL and ownership verification, before the ledger insert.
func runWizardAdministrativeMigration(ctx context.Context, databaseURL, migrationDir string, version int64, expected []MigrationSummary, beforeRecord func() error) (Result, error) {
	if !wizardAdminVersions[version] {
		return Result{}, errors.New("administrative migration version is not authorized")
	}
	if databaseURL != wizardTrialDatabaseURL {
		return Result{}, errors.New("administrative migration target is not the authorized wizard trial")
	}
	if err := verifyWizardManifest(expected); err != nil {
		return Result{}, err
	}
	migrations, err := DiscoverMigrations(migrationDir)
	if err != nil {
		return Result{}, err
	}
	if len(migrations) != len(expected) || len(migrations) != 83 {
		return Result{}, errors.New("migration inventory does not match the reviewed 1..83 manifest")
	}
	for i, m := range migrations {
		if m.Version != expected[i].Version || m.Name != expected[i].Name || m.Checksum != expected[i].Checksum {
			return Result{}, fmt.Errorf("migration inventory differs from reviewed manifest at version %d", i+1)
		}
	}
	config, err := pgx.ParseConfig(databaseURL)
	if err != nil {
		return Result{}, errors.New("invalid administrative migration connection configuration")
	}
	if err := guardWizardConfig(config); err != nil {
		return Result{}, err
	}
	conn, err := pgx.ConnectConfig(ctx, config)
	if err != nil {
		return Result{}, errors.New("connect to authorized wizard trial database")
	}
	defer conn.Close(context.Background())
	var database, sessionUser, currentUser string
	var serverVersion int
	if err := conn.QueryRow(ctx, `SELECT current_database(), session_user, current_user, current_setting('server_version_num')::int`).Scan(&database, &sessionUser, &currentUser, &serverVersion); err != nil {
		return Result{}, errors.New("verify administrative migration identity")
	}
	if database != "espacigo_wizard_20261106" || sessionUser != "postgres" || currentUser != "postgres" {
		return Result{}, errors.New("administrative migration identity does not match authorized target")
	}
	if err := guardWizardServerVersion(serverVersion); err != nil {
		return Result{}, err
	}
	unlock, err := AcquireMigrationAdvisoryLock(ctx, conn)
	if err != nil {
		return Result{}, err
	}
	defer func() { _ = unlock(context.Background()) }()
	var historyExists bool
	if err := conn.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relname='schema_migrations' AND c.relkind IN ('r','p'))`).Scan(&historyExists); err != nil {
		return Result{}, errors.New("inspect existing migration ledger")
	}
	if !historyExists {
		return Result{}, errors.New("administrative migration requires the existing migration ledger")
	}
	history, err := readHistory(ctx, conn)
	if err != nil {
		return Result{}, err
	}
	if err := verifyHistory(migrations, history); err != nil {
		return Result{}, err
	}
	if len(history) != int(version-1) {
		return Result{}, fmt.Errorf("ledger prefix is %d; migration V%06d requires exact prefix %d", len(history), version, version-1)
	}
	if err := guardAppliedSchemaObjects(ctx, conn, history); err != nil {
		return Result{}, err
	}
	if err := guardWizardOwners(ctx, conn, version); err != nil {
		return Result{}, err
	}
	migration := migrations[version-1]
	runnerDigest, err := executableChecksum()
	if err != nil {
		return Result{}, fmt.Errorf("checksum migrator executable: %w", err)
	}
	tx, err := conn.Begin(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("begin administrative migration: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(context.Background())
		}
	}()
	before, err := captureWizardOwnerSnapshot(ctx, tx)
	if err != nil {
		return Result{}, err
	}
	schemaOwnerOID, err := verifyWizardSchemaOwner(ctx, tx)
	if err != nil {
		return Result{}, err
	}
	if _, err := tx.Conn().PgConn().Exec(ctx, string(migration.SQL)).ReadAll(); err != nil {
		return Result{}, fmt.Errorf("execute administrative migration V%06d: %w", version, err)
	}
	expectedOwners, err := resolveWizardExpectedOwners(ctx, tx, version)
	if err != nil {
		return Result{}, err
	}
	if _, err := verifyWizardSchemaOwner(ctx, tx); err != nil {
		return Result{}, err
	}
	if err := normalizeWizardOwners(ctx, tx, version); err != nil {
		return Result{}, err
	}
	after, err := captureWizardOwnerSnapshot(ctx, tx)
	if err != nil {
		return Result{}, err
	}
	if err := verifyWizardOwnerSnapshot(before, after, expectedOwners, schemaOwnerOID); err != nil {
		return Result{}, err
	}
	if _, err := verifyWizardSchemaOwner(ctx, tx); err != nil {
		return Result{}, err
	}
	if beforeRecord != nil {
		if err := beforeRecord(); err != nil {
			return Result{}, fmt.Errorf("administrative pre-ledger hook: %w", err)
		}
	}
	if _, err := tx.Exec(ctx, `INSERT INTO public.schema_migrations(version,name,checksum,runner_checksum) VALUES($1,$2,$3,$4)`, migration.Version, migration.Name, migration.Checksum, runnerDigest); err != nil {
		return Result{}, fmt.Errorf("record administrative migration V%06d: %w", version, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Result{}, fmt.Errorf("commit administrative migration V%06d (read back before any retry): %w", version, err)
	}
	committed = true
	return Result{Applied: []int64{version}}, nil
}

func verifyWizardManifest(m []MigrationSummary) error {
	if len(m) != 83 {
		return errors.New("reviewed migration manifest must contain exactly versions 1..83")
	}
	for i, e := range m {
		if e.Version != int64(i+1) || e.Name == "" || !wizardChecksumPattern.MatchString(e.Checksum) {
			return fmt.Errorf("reviewed migration manifest is invalid at position %d", i+1)
		}
	}
	return nil
}

func guardWizardServerVersion(version int) error {
	if version != 180006 {
		return fmt.Errorf("administrative migration requires PostgreSQL 18.6 (server_version_num 180006); got %d", version)
	}
	return nil
}

func guardWizardConfig(c *pgx.ConnConfig) error {
	if c.Host != "127.0.0.1" || c.Port != 15436 || c.Database != "espacigo_wizard_20261106" || c.User != "postgres" || c.TLSConfig != nil {
		return errors.New("administrative migration connection target is not authorized")
	}
	if len(c.RuntimeParams) != 0 {
		return errors.New("administrative migration startup parameters are forbidden")
	}
	for _, key := range []string{"PGOPTIONS", "PGSERVICE", "PGSERVICEFILE", "PGHOST", "PGPORT", "PGDATABASE", "PGUSER", "PGPASSWORD", "PGSSLMODE"} {
		if _, ok := os.LookupEnv(key); ok {
			return fmt.Errorf("administrative migration environment override %s is forbidden", key)
		}
	}
	if err := validateWizardPassfile(); err != nil {
		return err
	}
	return nil
}

func validateWizardPassfile() error {
	wd, err := os.Getwd()
	if err != nil {
		return errors.New("cannot resolve administrative passfile location")
	}
	path := filepath.Join(wd, ".local", "testing", "wizard-improvements", "postgres", "pgpass")
	for _, dir := range []string{filepath.Join(wd, ".local"), filepath.Join(wd, ".local", "testing"), filepath.Join(wd, ".local", "testing", "wizard-improvements"), filepath.Dir(path)} {
		d, statErr := os.Lstat(dir)
		if statErr != nil || !d.IsDir() || d.Mode()&os.ModeSymlink != 0 || d.Mode().Perm() != 0700 {
			return errors.New("administrative passfile directory must be a real 0700 directory")
		}
		if sys, ok := d.Sys().(*syscall.Stat_t); !ok || int(sys.Uid) != os.Getuid() {
			return errors.New("administrative passfile directory must belong to current user")
		}
	}
	configured, ok := os.LookupEnv("PGPASSFILE")
	if !ok || !filepath.IsAbs(configured) || filepath.Clean(configured) != path {
		return errors.New("administrative PGPASSFILE must name the approved passfile")
	}
	st, err := os.Lstat(path)
	if err != nil || !st.Mode().IsRegular() || st.Mode()&os.ModeSymlink != 0 || st.Mode().Perm() != 0600 {
		return errors.New("administrative passfile must be a real 0600 regular file")
	}
	if sys, ok := st.Sys().(*syscall.Stat_t); !ok || int(sys.Uid) != os.Getuid() {
		return errors.New("administrative passfile must belong to current user")
	}
	return nil
}

// Owner normalization is performed transactionally immediately after SQL and
// before the migration history row. The schema owner is deliberately fixed.
const wizardSchemaOwner = "espacigo_schema_owner"

func normalizeWizardOwners(ctx context.Context, tx pgx.Tx, version int64) error {
	objects := wizardOwnerObjects(version)
	for _, obj := range objects {
		if _, err := tx.Exec(ctx, "ALTER "+obj.kind+" "+obj.identity+" OWNER TO "+pgx.Identifier{wizardSchemaOwner}.Sanitize()); err != nil {
			return fmt.Errorf("normalize owner for %s: %w", obj.identity, err)
		}
	}
	return nil
}

type wizardOwnedObject struct{ kind, identity string }

func wizardOwnerObjects(v int64) []wizardOwnedObject {
	switch v {
	case 60:
		return []wizardOwnedObject{{"FUNCTION", "public.reject_gadm_bootstrap_identity_change()"}}
	case 64:
		return []wizardOwnedObject{{"FUNCTION", "public.finalize_pending_suppression_cleanup(uuid,timestamptz)"}}
	case 66:
		return []wizardOwnedObject{{"TABLE", "public.gcp06_test_access_batch_audit"}, {"FUNCTION", "public.gcp06_test_access_target_ids()"}, {"FUNCTION", "public.gcp06_test_access_owner_ready()"}, {"FUNCTION", "public.gcp06_test_access_operation_exists(uuid)"}}
	default:
		return nil
	}
}

func verifyWizardSchemaOwner(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}) (uint32, error) {
	var oid int64
	var safe bool
	err := q.QueryRow(ctx, `SELECT oid::bigint, NOT rolsuper AND NOT rolcreaterole AND NOT rolcreatedb AND NOT rolreplication AND NOT rolbypassrls AND NOT rolcanlogin AND NOT rolinherit FROM pg_catalog.pg_roles WHERE rolname=$1`, wizardSchemaOwner).Scan(&oid, &safe)
	if err != nil || !safe || oid <= 0 || uint64(oid) > uint64(^uint32(0)) {
		return 0, errors.New("schema owner is missing or has unsafe role attributes")
	}
	return uint32(oid), nil
}

func resolveWizardExpectedOwners(ctx context.Context, tx pgx.Tx, version int64) (map[wizardObjectKey]bool, error) {
	expected := make(map[wizardObjectKey]bool)
	for _, obj := range wizardOwnerObjects(version) {
		var oid *int64
		var query string
		catalog := "pg_proc"
		if obj.kind == "TABLE" {
			catalog = "pg_class"
			query = `SELECT to_regclass($1)::oid::bigint`
		} else {
			query = `SELECT to_regprocedure($1)::oid::bigint`
		}
		if err := tx.QueryRow(ctx, query, obj.identity).Scan(&oid); err != nil {
			return nil, fmt.Errorf("resolve expected wizard object %s: %w", obj.identity, err)
		}
		if oid == nil || *oid <= 0 || uint64(*oid) > uint64(^uint32(0)) {
			return nil, fmt.Errorf("expected wizard object is missing: %s", obj.identity)
		}
		expected[wizardObjectKey{catalog: catalog, oid: uint32(*oid)}] = wizardExpectedSecurityDefiner(version, obj)
	}
	return expected, nil
}

func wizardExpectedSecurityDefiner(version int64, obj wizardOwnedObject) bool {
	return obj.kind == "FUNCTION" && version != 60
}

func guardWizardOwners(ctx context.Context, conn *pgx.Conn, version int64) error {
	_, err := verifyWizardSchemaOwner(ctx, conn)
	return err
}
