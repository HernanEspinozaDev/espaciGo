package main

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/migrator"
	"github.com/jackc/pgx/v5"
)

const nativeV86SourceFingerprint = "d3ca3619312168dddebe1b7a722252d24381efcfd6bb60349a4dbd86007ec807"

func validateNativeV86Mode(enabled bool, confirmation, fingerprint string, flags []string) error {
	if !enabled || confirmation != nativeAdminConfirmation || fingerprint != nativeV86SourceFingerprint {
		return errors.New("native V86 requires its mode, exact target confirmation and pinned reviewed source fingerprint")
	}
	for _, name := range flags {
		switch name {
		case "native-kyc-v86", "confirm-native-kyc-v86", "native-kyc-v86-source-sha256", "dir":
		default:
			return errors.New("native V86 cannot combine with another mode or unrelated option")
		}
	}
	return nil
}

func validateNativeV86Connection(settings connectionSettings, getenv func(string) string) error {
	if settings.mode != "direct" || settings.setRole != "espacigo_schema_owner" || os.Getuid() == 0 {
		return errors.New("native V86 requires a non-root operator, direct mode and the schema-owner role")
	}
	for _, key := range []string{"PGPASSWORD", "PGOPTIONS", "PGSERVICE", "PGSERVICEFILE", "PGHOST", "PGHOSTADDR", "PGPORT", "PGDATABASE", "PGUSER", "PGAPPNAME", "PGTZ", "PGSSLMODE", "PGSSLROOTCERT", "PGSSLCERT", "PGSSLKEY"} {
		if getenv(key) != "" {
			return errors.New("native V86 rejects libpq session and connection overrides")
		}
	}
	if getenv("PGPASSFILE") != nativeMigratorPassfilePath {
		return errors.New("native V86 requires the reviewed private passfile path")
	}
	for _, path := range []string{filepath.Dir(nativeMigratorPassfilePath), nativeMigratorPassfilePath} {
		info, err := os.Lstat(path)
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("native V86 private passfile metadata is invalid")
		}
		st, ok := info.Sys().(*syscall.Stat_t)
		if !ok || st.Uid != uint32(os.Getuid()) {
			return errors.New("native V86 passfile and directory must belong to the operator")
		}
		if path == nativeMigratorPassfilePath {
			if !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
				return errors.New("native V86 passfile must be regular mode 0600")
			}
		} else if !info.IsDir() || info.Mode().Perm() != 0700 {
			return errors.New("native V86 passfile directory must be mode 0700")
		}
	}
	u, err := url.Parse(settings.databaseURL)
	if err != nil || u.Scheme != "postgresql" || u.Host != "127.0.0.1:5432" || u.Path != "/espacigo_dms_stage_20261020" || u.RawPath != "" || u.Fragment != "" || u.User == nil || u.User.Username() != "espacigo_migrator_login" {
		return errors.New("native V86 connection is outside the exact local allowlist")
	}
	if _, present := u.User.Password(); present {
		return errors.New("native V86 forbids embedded passwords")
	}
	if u.RawQuery != "" && u.RawQuery != "sslmode=disable" {
		return errors.New("native V86 forbids URL session overrides")
	}
	return nil
}

func nativeV86IdentityPreflight(ctx context.Context, databaseURL string, requireCreate ...bool) error {
	wantCreate := true
	if len(requireCreate) > 0 {
		wantCreate = requireCreate[0]
	}
	config, err := pgx.ParseConfig(databaseURL)
	if err != nil || config.Host != "127.0.0.1" || config.Port != 5432 || config.Database != nativeAdminConfirmation || config.User != "espacigo_migrator_login" || len(config.RuntimeParams) != 0 {
		return errors.New("native V86 effective connection configuration mismatch")
	}
	conn, err := pgx.ConnectConfig(ctx, config)
	if err != nil {
		return errors.New("native V86 migrator authentication failed")
	}
	defer conn.Close(context.Background())
	tx, err := conn.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return errors.New("native V86 read-only identity preflight failed")
	}
	defer tx.Rollback(context.Background())
	if _, err := tx.Exec(ctx, `SET LOCAL ROLE espacigo_schema_owner`); err != nil {
		return errors.New("native V86 schema-owner role could not be assumed")
	}
	var database, version, session, current string
	var oid int64
	var readonly, createAllowed, rolesSafe bool
	err = tx.QueryRow(ctx, `SELECT current_database(), d.oid::bigint, current_setting('server_version_num'), session_user, current_user,
 current_setting('transaction_read_only')='on', has_schema_privilege(current_user,'public','CREATE'),
 (SELECT count(*)=2 AND bool_and(NOT rolsuper AND NOT rolcreatedb AND NOT rolcreaterole AND NOT rolinherit AND NOT rolreplication AND NOT rolbypassrls AND (rolcanlogin=(rolname='espacigo_migrator_login'))) FROM pg_roles WHERE rolname IN ('espacigo_migrator_login','espacigo_schema_owner'))
 FROM pg_database d WHERE d.datname=current_database()`).Scan(&database, &oid, &version, &session, &current, &readonly, &createAllowed, &rolesSafe)
	// Memberships and complete ACLs are checked by the canonical V86 authority
	// contract in Plan and again inside the locked migration transaction.
	// Do not duplicate a partial edge count here: the source contract also
	// contains schema_owner -> document_verification_fn SET ROLE.
	if err != nil {
		return errors.New("native V86 identity preflight catalog query failed; no DDL executed")
	}
	if database != nativeAdminConfirmation || oid != 16388 || version != "180007" || session != "espacigo_migrator_login" || current != "espacigo_schema_owner" || !readonly || createAllowed != wantCreate || !rolesSafe {
		return errors.New("native V86 target, identity or prerequisite authority mismatch; no DDL executed")
	}
	return nil
}

func runNativeKYCV86(source string, getenv func(string) string) error {
	settings, err := loadConnectionSettings(getenv)
	if err != nil {
		return err
	}
	if err := validateNativeV86Connection(settings, getenv); err != nil {
		return err
	}
	stage, cleanup, err := makeNativeV86Snapshot(source, nativeV86SourceFingerprint)
	if err != nil {
		return err
	}
	defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := nativeV86IdentityPreflight(ctx, settings.databaseURL); err != nil {
		return err
	}
	plan, err := migrator.Plan(ctx, settings.databaseURL, stage, settings.setRole)
	if err != nil {
		return fmt.Errorf("native V86 read-only plan failed: %w", err)
	}
	if err := validateNativeV86Plan(plan); err != nil {
		return err
	}
	if _, err := migrator.RunNativeKYCV86(ctx, settings.databaseURL, stage); err != nil {
		return fmt.Errorf("native V86 apply failed; read ledger before retry: %w", err)
	}
	post, err := migrator.Plan(ctx, settings.databaseURL, stage, settings.setRole)
	if err != nil {
		return fmt.Errorf("native V86 postcheck unavailable; do not replay: %w", err)
	}
	if err := validateNativeV86PostPlan(post); err != nil {
		return err
	}
	fmt.Println("native V86 applied; source-backed ledger V1..V86 verified; later migrations not applied")
	return nil
}
