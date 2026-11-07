package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	"github.com/HernanEspinozaDev/espaciGo/internal/migrator"
	"github.com/jackc/pgx/v5"
)

const (
	wizardAdminConfirmation = "espacigo_wizard_20261106"
	wizardAdminManifest     = ".local/testing/wizard-improvements/reviewed-full-83-manifest.json"
	wizardAdminPassfile     = ".local/testing/wizard-improvements/postgres/pgpass"
)

func repoRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for dir := cwd; ; dir = filepath.Dir(dir) {
		if info, err := os.Stat(filepath.Join(dir, "db", "migrations")); err == nil && info.IsDir() {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("could not locate repository root")
		}
	}
}

func validateWizardAdminInvocation(version int64, confirmation string, planOnly, applyACL, recoverACL bool, confirmACL, confirmF5 string, cloudOwner bool, confirmCloud string) error {
	if version == 0 {
		if confirmation != "" {
			return errors.New("-confirm-wizard-admin requires -wizard-admin-version")
		}
		return nil
	}
	if version != 60 && version != 64 && version != 66 {
		return errors.New("-wizard-admin-version must be 60, 64, or 66")
	}
	if confirmation != wizardAdminConfirmation {
		return errors.New("wizard administrative migration requires exact ensayo confirmation")
	}
	if planOnly || applyACL || recoverACL || confirmACL != "" || confirmF5 != "" || cloudOwner || confirmCloud != "" {
		return errors.New("wizard administrative migration requires a separate invocation")
	}
	return nil
}

const wizardAdminDatabaseURL = "postgres://postgres@127.0.0.1:15436/espacigo_wizard_20261106?sslmode=disable"

func validateWizardAdminIdentity(databaseURL, mode, setRole string, getenv func(string) string) error {
	if mode != "direct" || setRole != "" {
		return errors.New("wizard administrative migration requires direct mode without DATABASE_SET_ROLE")
	}
	for _, key := range []string{"PGPASSWORD", "PGOPTIONS", "PGSERVICE", "PGSERVICEFILE", "PGHOST"} {
		if getenv(key) != "" {
			return errors.New("wizard administrative migration forbids libpq credential, host, service and session-option overrides")
		}
	}
	root, err := repoRoot()
	if err != nil {
		return errors.New("wizard administrative migration requires the repository private passfile")
	}
	abs, err := filepath.Abs(filepath.Join(root, filepath.FromSlash(wizardAdminPassfile)))
	if err != nil {
		return errors.New("wizard administrative migration requires the fixed private PGPASSFILE")
	}
	if getenv("PGPASSFILE") != abs {
		return errors.New("wizard administrative migration requires the fixed private PGPASSFILE")
	}
	if err := verifyPrivatePath(abs); err != nil {
		return err
	}
	if databaseURL != wizardAdminDatabaseURL {
		return errors.New("wizard administrative migration requires the exact local administrator target")
	}
	u, err := url.Parse(databaseURL)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || u.User == nil || u.User.Username() != "postgres" {
		return errors.New("wizard administrative migration requires the exact local administrator target")
	}
	if _, ok := u.User.Password(); ok {
		return errors.New("wizard administrative migration requires credentials from PGPASSFILE")
	}
	for key, values := range u.Query() {
		if (key != "sslmode" && key != "application_name") || len(values) != 1 {
			return errors.New("wizard administrative migration requires reviewed URL parameters only")
		}
	}
	cfg, err := pgx.ParseConfig(databaseURL)
	if err != nil || cfg.Host != "127.0.0.1" || cfg.Port != 15436 || cfg.Database != wizardAdminConfirmation || cfg.User != "postgres" || u.Query().Get("sslmode") != "disable" {
		return errors.New("wizard administrative migration requires 127.0.0.1:15436/espacigo_wizard_20261106 with sslmode=disable")
	}
	return nil
}

func verifyPrivatePath(path string) error {
	root, err := repoRoot()
	if err != nil {
		return errors.New("wizard administrative migration requires a private repository path")
	}
	privateRoot := filepath.Join(root, ".local")
	dir := filepath.Dir(path)
	rel, err := filepath.Rel(privateRoot, dir)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return errors.New("wizard administrative migration private path must be under .local")
	}
	for current := dir; ; current = filepath.Dir(current) {
		info, e := os.Lstat(current)
		if e != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0700 {
			return errors.New("wizard administrative migration requires private real parent directories with mode 0700")
		}
		if current == privateRoot {
			break
		}
		parent := filepath.Dir(current)
		if parent == current {
			return errors.New("wizard administrative migration private path is invalid")
		}
	}
	file, err := os.Lstat(path)
	if err != nil || !file.Mode().IsRegular() || file.Mode()&os.ModeSymlink != 0 || file.Mode().Perm() != 0600 {
		return errors.New("wizard administrative migration requires a regular mode-0600 private file")
	}
	stat, ok := file.Sys().(*syscall.Stat_t)
	if !ok || int(stat.Uid) != os.Getuid() {
		return errors.New("wizard administrative migration private file must belong to the current user")
	}
	return nil
}

func reviewedWizardAdminInventory(source string) ([]migrator.MigrationSummary, error) {
	root, err := repoRoot()
	if err != nil {
		return nil, errors.New("could not locate reviewed wizard migration manifest")
	}
	absSource, err := filepath.Abs(source)
	if err != nil {
		return nil, errors.New("wizard migration source must remain inside the repository")
	}
	realRoot, rootErr := filepath.EvalSymlinks(root)
	realSource, sourceErr := filepath.EvalSymlinks(absSource)
	if rootErr != nil || sourceErr != nil {
		return nil, errors.New("wizard migration source must be a real repository directory")
	}
	rel, relErr := filepath.Rel(realRoot, realSource)
	if relErr != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil, errors.New("wizard migration source must remain inside the repository")
	}
	info, infoErr := os.Stat(realSource)
	if infoErr != nil || !info.IsDir() {
		return nil, errors.New("wizard migration source must be a directory")
	}
	absManifest := filepath.Join(root, filepath.FromSlash(wizardAdminManifest))
	if err := verifyPrivatePath(absManifest); err != nil {
		return nil, err
	}
	b, err := os.ReadFile(absManifest)
	if err != nil {
		return nil, errors.New("could not read reviewed wizard migration manifest")
	}
	var records []struct {
		File    string `json:"file"`
		SHA256  string `json:"sha256"`
		Version int64  `json:"version"`
	}
	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&records); err != nil {
		return nil, errors.New("reviewed wizard manifest is invalid")
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return nil, errors.New("reviewed wizard manifest has trailing data")
	}
	if len(records) != 83 {
		return nil, errors.New("reviewed wizard manifest must contain exactly 83 migrations")
	}
	migrations, err := migrator.DiscoverMigrations(source)
	if err != nil {
		return nil, errors.New("could not discover complete wizard migration source")
	}
	if len(migrations) != len(records) {
		return nil, errors.New("wizard migration source differs from reviewed 83-entry manifest")
	}
	out := make([]migrator.MigrationSummary, len(records))
	seen := map[int64]bool{}
	for i, row := range records {
		if row.Version < 1 || seen[row.Version] || row.Version != int64(i+1) || filepath.Base(row.File) != row.File || len(row.SHA256) != 64 {
			return nil, errors.New("reviewed wizard manifest versions, filenames, or hashes are invalid")
		}
		seen[row.Version] = true
		hash, e := hex.DecodeString(row.SHA256)
		if e != nil || len(hash) != sha256.Size {
			return nil, errors.New("reviewed wizard manifest contains invalid checksum")
		}
		m := migrations[i]
		if row.File != filepath.Base(m.Path) || !strings.EqualFold(row.SHA256, m.Checksum) || row.Version != m.Version {
			return nil, fmt.Errorf("wizard migration source differs from reviewed manifest at V%06d", row.Version)
		}
		out[i] = migrator.MigrationSummary{Version: row.Version, Name: row.File, Checksum: strings.ToLower(row.SHA256)}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Version < out[j].Version })
	return out, nil
}
