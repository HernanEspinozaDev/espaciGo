package main

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/HernanEspinozaDev/espaciGo/internal/migrator"
	"github.com/jackc/pgx/v5"
)

const cloudOwnerConfirmation = "espacigo_dms_stage_20261020"

// Canonical version|name|checksum lines for the 83 entries in
// planning/gcp/espacios_cloudsql_v83_manifest_20261105.json, reviewed source
// 9edd61c405bc01f7f5ac5c46e616a6d3063bdf5f. This is not learned from the target.
const cloudOwnerInventorySHA256 = "937a15b83f3114d846aa4bf99768dd5a68da19967d21de435cafa36f7c751a2a"

func validateCloudOwnerInvocation(enabled, planOnly, applyACL, recoverACL bool, confirmation string, settings connectionSettings, getenv func(string) string) error {
	if !enabled {
		return nil
	}
	if planOnly || applyACL || recoverACL {
		return errors.New("cloud owner migration cannot combine with plan-only or runtime ACL modes")
	}
	if confirmation != cloudOwnerConfirmation {
		return errors.New("cloud owner migration requires exact target confirmation")
	}
	if settings.mode != "direct" || settings.setRole != "" {
		return errors.New("cloud owner migration requires direct mode without DATABASE_SET_ROLE")
	}
	for _, key := range []string{"PGPASSWORD", "PGOPTIONS", "PGSERVICE", "PGSERVICEFILE"} {
		if getenv(key) != "" {
			return errors.New("cloud owner migration forbids credential, service and session-option overrides")
		}
	}
	pass := getenv("PGPASSFILE")
	if !filepath.IsAbs(pass) || filepath.Clean(pass) == "/dev/null" {
		return errors.New("cloud owner migration requires absolute private PGPASSFILE")
	}
	di, e := os.Lstat(filepath.Dir(pass))
	if e != nil || !di.IsDir() || di.Mode().Perm() != 0700 {
		return errors.New("PGPASSFILE parent must be a real directory with mode 0700")
	}
	fi, e := os.Lstat(pass)
	if e != nil || !fi.Mode().IsRegular() || fi.Mode().Perm() != 0600 {
		return errors.New("PGPASSFILE must be a regular file with mode 0600")
	}
	u, e := url.Parse(settings.databaseURL)
	if e != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") {
		return errors.New("cloud owner migration requires PostgreSQL URL")
	}
	if u.User == nil || u.User.Username() != "postgres" {
		return errors.New("cloud owner migration requires postgres login")
	}
	if _, ok := u.User.Password(); ok {
		return errors.New("password must come only from PGPASSFILE")
	}
	for key, values := range u.Query() {
		if (key != "sslmode" && key != "application_name") || len(values) != 1 {
			return errors.New("cloud owner migration requires the reviewed URL parameters only")
		}
	}
	cfg, e := pgx.ParseConfig(settings.databaseURL)
	if e != nil || cfg.Host != "127.0.0.1" || cfg.Port != 15432 || cfg.User != "postgres" || cfg.Database != "espacigo_dms_stage_20261020" || cfg.TLSConfig == nil {
		return errors.New("cloud owner migration requires exact loopback staging target with sslmode=require")
	}
	if u.Query().Get("sslmode") != "require" {
		return errors.New("cloud owner migration requires sslmode=require")
	}
	return nil
}

// makeCloudOwnerSnapshot validates the complete original history before copying only V1..V80.
func makeCloudOwnerSnapshot(source string) (string, func(), error) {
	all, err := migrator.DiscoverMigrations(source)
	if err != nil {
		return "", nil, err
	}
	if len(all) != 83 {
		return "", nil, errors.New("source migration inventory differs from the reviewed V1..V83 manifest")
	}
	var inventory strings.Builder
	for i, m := range all {
		if m.Version != int64(i+1) {
			return "", nil, errors.New("source migration sequence is not contiguous")
		}
		fmt.Fprintf(&inventory, "%d|%s|%s\n", m.Version, m.Name, m.Checksum)
	}
	if fmt.Sprintf("%x", sha256.Sum256([]byte(inventory.String()))) != cloudOwnerInventorySHA256 {
		return "", nil, errors.New("source migration names or checksums differ from the reviewed manifest")
	}
	root, err := os.MkdirTemp("", "dbmigrate-cloud-owner-v80-")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() { _ = os.RemoveAll(root) }
	for _, m := range all {
		if m.Version > 80 {
			break
		}
		if err = os.WriteFile(filepath.Join(root, filepath.Base(m.Path)), m.SQL, 0600); err != nil {
			cleanup()
			return "", nil, err
		}
	}
	copied, err := migrator.DiscoverMigrations(root)
	if err != nil {
		cleanup()
		return "", nil, err
	}
	if len(copied) != 80 {
		cleanup()
		return "", nil, errors.New("staged migration snapshot does not contain V1..V80")
	}
	for i := range copied {
		if copied[i].Checksum != all[i].Checksum || copied[i].Name != all[i].Name {
			cleanup()
			return "", nil, errors.New("staged migration snapshot differs from original history")
		}
	}
	return root, cleanup, nil
}

func validateCloudOwnerPending(plan migrator.PlanResult) error {
	if plan.Database != cloudOwnerConfirmation || !plan.ReadOnly || !plan.HistoryExists {
		return errors.New("exact read-only target plan with existing ledger required")
	}
	if len(plan.Applied) < 75 || len(plan.Applied) > 80 || len(plan.Applied)+len(plan.Pending) != 80 {
		return errors.New("complete V1..V80 inventory with committed V75..V80 baseline required")
	}
	for i, applied := range plan.Applied {
		if applied.Version != int64(i+1) {
			return errors.New("applied ledger is not contiguous")
		}
	}
	for i, pending := range plan.Pending {
		if pending.Version != int64(len(plan.Applied)+i+1) || pending.Version < 76 || pending.Version > 80 {
			return errors.New("pending migration sequence must complete the exact V76..V80 range")
		}
	}
	return nil
}
