package main

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/HernanEspinozaDev/espaciGo/internal/migrator"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const nativeAdminConfirmation = "espacigo_dms_stage_20261020"
const nativeMigratorPassfilePath = "/home/nandev/.local/share/espacigo-native-secrets/pgpass"

func validateNativeAdminRangeInvocation(confirmation string, settings connectionSettings, getenv func(string) string, osUser string) error {
	if confirmation != nativeAdminConfirmation {
		return errors.New("native pre-V81 migration requires exact target confirmation")
	}
	if settings.mode != "direct" || settings.setRole != "" {
		return errors.New("native pre-V81 migration requires direct mode without DATABASE_SET_ROLE")
	}
	if osUser != "postgres" {
		return errors.New("native pre-V81 migration must run as the local postgres OS account")
	}
	tmpDir := getenv("TMPDIR")
	if !filepath.IsAbs(tmpDir) {
		return errors.New("native pre-V81 migration requires an absolute private TMPDIR")
	}
	tmpInfo, err := os.Lstat(tmpDir)
	if err != nil || !tmpInfo.IsDir() || tmpInfo.Mode()&os.ModeSymlink != 0 || tmpInfo.Mode().Perm() != 0700 {
		return errors.New("native pre-V81 migration requires a real TMPDIR with mode 0700")
	}
	for _, key := range []string{"PGPASSWORD", "PGOPTIONS", "PGSERVICE", "PGSERVICEFILE", "PGHOST", "PGPORT", "PGDATABASE", "PGUSER"} {
		if getenv(key) != "" {
			return errors.New("native pre-V81 migration rejects libpq connection overrides")
		}
	}
	parsed, err := url.Parse(settings.databaseURL)
	if err != nil || parsed.User == nil {
		return errors.New("native pre-V81 migration requires the exact local peer connection")
	}
	if _, hasPassword := parsed.User.Password(); hasPassword {
		return errors.New("native pre-V81 migration forbids embedded credentials")
	}
	config, err := pgx.ParseConfigWithOptions(settings.databaseURL, pgx.ParseConfigOptions{
		ParseConfigOptions: pgconn.ParseConfigOptions{
			ConnStringAllowedKeys: []string{"host", "port", "sslmode", "database", "user"},
		},
	})
	if err != nil || config.Host != "/var/run/postgresql" || config.Port != 5432 ||
		config.Database != nativeAdminConfirmation || config.User != "postgres" {
		return errors.New("native pre-V81 migration requires PostgreSQL peer on /var/run/postgresql:5432 and the exact authorized database")
	}
	return nil
}

func validateNativePreV81Plan(plan migrator.PlanResult) error {
	if plan.Database != nativeAdminConfirmation || !plan.ReadOnly || !plan.HistoryExists {
		return errors.New("native pre-V81 gate requires a read-only ledger for the exact target")
	}
	if !strings.HasPrefix(strings.TrimSpace(plan.ServerVersion), "18.6") {
		return errors.New("native pre-V81 gate requires the reviewed PostgreSQL 18.6 server")
	}
	if len(plan.Applied) < 76 || len(plan.Applied) > 80 || len(plan.Pending) != 85-len(plan.Applied) {
		return errors.New("native pre-V81 gate requires an applied prefix from V76 through V80 and source inventory through V85")
	}
	for i, migration := range plan.Applied {
		if migration.Version != int64(i+1) || migration.Name == "" || migration.Checksum == "" {
			return errors.New("native pre-V81 gate requires a contiguous verified applied prefix")
		}
	}
	for i, migration := range plan.Pending {
		if migration.Version != int64(len(plan.Applied)+i+1) || migration.Name == "" || migration.Checksum == "" {
			return errors.New("native pre-V81 gate requires a contiguous pending suffix through V85")
		}
	}
	return nil
}

func validateNativeV80RoleTransitionPlan(plan migrator.PlanResult) error {
	if err := validateNativePreV81Plan(plan); err != nil {
		return err
	}
	if len(plan.Applied) != 80 || len(plan.Pending) != 5 {
		return errors.New("native role transition requires the exact V1..V80 applied prefix and V81..V85 pending suffix")
	}
	return nil
}

func validateNativeMigratorV81Invocation(confirmation string, settings connectionSettings, getenv func(string) string, osUser, expectedPassfile string) error {
	if confirmation != nativeAdminConfirmation {
		return errors.New("native V81 migration requires exact target confirmation")
	}
	if settings.mode != "direct" || settings.setRole != "espacigo_schema_owner" {
		return errors.New("native V81 migration requires direct mode and DATABASE_SET_ROLE=espacigo_schema_owner")
	}
	if osUser == "" || osUser == "root" || osUser == "postgres" {
		return errors.New("native V81 migration must run as the private passfile owner, not an administrator")
	}
	tmpDir := getenv("TMPDIR")
	if !filepath.IsAbs(tmpDir) {
		return errors.New("native V81 migration requires an absolute private TMPDIR")
	}
	tmpInfo, err := os.Lstat(tmpDir)
	if err != nil || !tmpInfo.IsDir() || tmpInfo.Mode()&os.ModeSymlink != 0 || tmpInfo.Mode().Perm() != 0700 {
		return errors.New("native V81 migration requires a real TMPDIR with mode 0700")
	}
	for _, key := range []string{"PGPASSWORD", "PGOPTIONS", "PGSERVICE", "PGSERVICEFILE", "PGHOST", "PGPORT", "PGDATABASE", "PGUSER"} {
		if getenv(key) != "" {
			return errors.New("native V81 migration rejects libpq credential, service, and connection overrides")
		}
	}
	passfile := getenv("PGPASSFILE")
	if !filepath.IsAbs(expectedPassfile) || passfile != expectedPassfile {
		return errors.New("native V81 migration requires the reviewed private PGPASSFILE")
	}
	dirInfo, err := os.Lstat(filepath.Dir(passfile))
	if err != nil || !dirInfo.IsDir() || dirInfo.Mode()&os.ModeSymlink != 0 || dirInfo.Mode().Perm() != 0700 {
		return errors.New("native V81 PGPASSFILE directory must be a real private directory with mode 0700")
	}
	fileInfo, err := os.Lstat(passfile)
	if err != nil || !fileInfo.Mode().IsRegular() || fileInfo.Mode()&os.ModeSymlink != 0 || fileInfo.Mode().Perm() != 0600 {
		return errors.New("native V81 PGPASSFILE must be a regular private file with mode 0600")
	}
	passfileHandle, err := os.Open(passfile)
	if err != nil {
		return errors.New("native V81 PGPASSFILE must be readable by the invoking account")
	}
	if err := passfileHandle.Close(); err != nil {
		return errors.New("native V81 PGPASSFILE could not be closed after access validation")
	}
	parsed, err := url.Parse(settings.databaseURL)
	if err != nil || parsed.User == nil {
		return errors.New("native V81 migration requires the exact loopback TCP target")
	}
	if _, hasPassword := parsed.User.Password(); hasPassword {
		return errors.New("native V81 migration forbids embedded credentials")
	}
	for key, values := range parsed.Query() {
		if key != "sslmode" || len(values) != 1 || values[0] != "disable" {
			return errors.New("native V81 migration requires the reviewed loopback URL parameters only")
		}
	}
	config, err := pgx.ParseConfigWithOptions(settings.databaseURL, pgx.ParseConfigOptions{
		ParseConfigOptions: pgconn.ParseConfigOptions{
			ConnStringAllowedKeys: []string{"host", "port", "sslmode", "database", "user"},
		},
	})
	if err != nil || config.Host != "127.0.0.1" || config.Port != 5432 || config.Database != nativeAdminConfirmation || config.User != "espacigo_migrator_login" {
		return errors.New("native V81 migration requires the exact loopback PostgreSQL target and dedicated migrator login")
	}
	return nil
}

func validateNativeV81Plan(plan migrator.PlanResult) error {
	return validateNativeV80RoleTransitionPlan(plan)
}

func validateNativeV81PostPlan(plan migrator.PlanResult) error {
	if plan.Database != nativeAdminConfirmation || !plan.ReadOnly || !plan.HistoryExists || !strings.HasPrefix(strings.TrimSpace(plan.ServerVersion), "18.6") {
		return errors.New("native V81 postcheck requires a read-only ledger on the exact PostgreSQL 18.6 target")
	}
	if len(plan.Applied) != 81 || len(plan.Pending) != 4 || plan.Applied[80].Version != 81 {
		return errors.New("native V81 postcheck requires applied V1..V81 and pending V82..V85")
	}
	return nil
}

func makeNativePreV81Snapshot(source string) (string, func(), error) {
	return makeNativePrefixSnapshot(source, 80, "native-pre-v81-v80-")
}

func makeNativeV81Snapshot(source string) (string, func(), error) {
	return makeNativePrefixSnapshot(source, 81, "native-migrator-v81-")
}

func makeNativePrefixSnapshot(source string, prefixCount int, tempPattern string) (string, func(), error) {
	all, err := migrator.DiscoverMigrations(source)
	if err != nil {
		return "", nil, err
	}
	if len(all) < 85 || prefixCount < 1 || prefixCount > 85 {
		return "", nil, errors.New("native pre-V81 source must contain the reviewed V1..V85 inventory")
	}
	for i, migration := range all[:85] {
		if migration.Version != int64(i+1) {
			return "", nil, errors.New("native pre-V81 source inventory is not contiguous")
		}
	}
	root, err := os.MkdirTemp("", tempPattern)
	if err != nil {
		return "", nil, err
	}
	cleanup := func() { _ = os.RemoveAll(root) }
	for _, migration := range all[:prefixCount] {
		if err := os.WriteFile(filepath.Join(root, filepath.Base(migration.Path)), migration.SQL, 0600); err != nil {
			cleanup()
			return "", nil, err
		}
	}
	copied, err := migrator.DiscoverMigrations(root)
	if err != nil {
		cleanup()
		return "", nil, err
	}
	if len(copied) != prefixCount {
		cleanup()
		return "", nil, fmt.Errorf("native staged inventory has %d entries, want %d", len(copied), prefixCount)
	}
	for i := range copied {
		if copied[i].Name != all[i].Name || copied[i].Checksum != all[i].Checksum {
			cleanup()
			return "", nil, errors.New("native pre-V81 staged prefix differs from the reviewed source inventory")
		}
	}
	return root, cleanup, nil
}
