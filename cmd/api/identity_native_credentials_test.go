package main

import (
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func validateNativeIdentityPrivateFile(path string) error {
	if !filepath.IsAbs(path) {
		return errors.New("native identity private path must be absolute")
	}
	for _, p := range []string{filepath.Dir(path), path} {
		info, err := os.Lstat(p)
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("native identity private file metadata is invalid")
		}
		st, ok := info.Sys().(*syscall.Stat_t)
		if !ok || st.Uid != uint32(os.Getuid()) {
			return errors.New("native identity private files must belong to the operator")
		}
		if p == path {
			if !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
				return errors.New("native identity private file must be regular mode 0600")
			}
		} else if !info.IsDir() || info.Mode().Perm() != 0700 {
			return errors.New("native identity private directory must be mode 0700")
		}
	}
	return nil
}

func parseNativeIdentityConfig(rawDSN, passfile string) (*pgxpool.Config, error) {
	if validateIdentityDeclarationRuntimeDSN(rawDSN) != nil && validateIdentityDeclarationCleanupDSN(rawDSN) != nil {
		return nil, errors.New("native identity connection outside local allowlist")
	}
	for _, key := range []string{"PGPASSWORD", "PGOPTIONS", "PGSERVICE", "PGSERVICEFILE", "PGHOST", "PGHOSTADDR", "PGPORT", "PGDATABASE", "PGUSER", "PGAPPNAME", "PGTZ"} {
		if os.Getenv(key) != "" {
			return nil, errors.New("native identity forbids connection or credential environment overrides")
		}
	}
	if err := validateNativeIdentityPrivateFile(passfile); err != nil {
		return nil, err
	}
	cfg, err := pgxpool.ParseConfig(rawDSN + "?passfile=" + url.QueryEscape(passfile))
	if err != nil || cfg.ConnConfig.Password == "" {
		return nil, errors.New("native identity explicit passfile did not supply its login credential")
	}
	return cfg, nil
}

func validateNativeIdentityPolicy(raw []byte, accountID string) error {
	var policy struct {
		Target struct {
			Database string `json:"database"`
			Host     string `json:"host"`
			Port     int    `json:"port"`
			OID      int64  `json:"expected_database_oid"`
		} `json:"target"`
		Authorization struct {
			Maximum   int    `json:"maximum_new_accounts"`
			Domain    string `json:"synthetic_email_domain"`
			Writes    bool   `json:"synthetic_kyc_rows_write_update_delete"`
			Inventory []struct {
				ID       string `json:"account_id"`
				Verified bool   `json:"verified"`
			} `json:"account_inventory"`
		} `json:"authorization"`
	}
	if json.Unmarshal(raw, &policy) != nil || policy.Target.Database != "espacigo_dms_stage_20261020" || policy.Target.Host != "127.0.0.1" || policy.Target.Port != 5432 || policy.Target.OID != 16388 || policy.Authorization.Maximum != 2 || policy.Authorization.Domain != "example.invalid" || !policy.Authorization.Writes || len(policy.Authorization.Inventory) > 2 {
		return errors.New("native identity fixture policy target or authorization mismatch")
	}
	seen := make(map[string]bool)
	authorized := false
	for _, entry := range policy.Authorization.Inventory {
		if !canonicalNativeUUID(entry.ID) || seen[entry.ID] {
			return errors.New("native identity fixture inventory IDs must be canonical and unique")
		}
		seen[entry.ID] = true
		if entry.ID == accountID && entry.Verified {
			authorized = true
		}
	}
	if authorized {
		return nil
	}
	return errors.New("native identity test account is not an authorized verified fixture")
}

func TestNativeIdentityExplicitPassfilesStayRoleScoped(t *testing.T) {
	for _, key := range []string{"PGPASSWORD", "PGOPTIONS", "PGSERVICE", "PGSERVICEFILE", "PGHOST", "PGHOSTADDR", "PGPORT", "PGDATABASE", "PGUSER", "PGAPPNAME", "PGTZ"} {
		t.Setenv(key, "")
	}
	path := filepath.Join(t.TempDir(), "pgpass")
	if err := os.Chmod(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("127.0.0.1:5432:espacigo_dms_stage_20261020:espacigo_api_login:synthetic-api\n127.0.0.1:5432:espacigo_dms_stage_20261020:espacigo_migrator_login:synthetic-migrator\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, role := range []string{"espacigo_api_login", "espacigo_migrator_login"} {
		cfg, err := parseNativeIdentityConfig("postgresql://"+role+"@127.0.0.1:5432/espacigo_dms_stage_20261020", path)
		if err != nil {
			t.Fatal(err)
		}
		expected := "synthetic-api"
		if role == "espacigo_migrator_login" {
			expected = "synthetic-migrator"
		}
		if cfg.ConnConfig.Password != expected {
			t.Fatal("explicit role-scoped passfile was not used")
		}
	}
	if _, err := parseNativeIdentityConfig("postgresql://espacigo_api_login@127.0.0.1:5432/espacigo_dms_stage_20261020", "relative"); err == nil {
		t.Fatal("relative passfile accepted")
	}
	if err := os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := parseNativeIdentityConfig("postgresql://espacigo_api_login@127.0.0.1:5432/espacigo_dms_stage_20261020", path); err == nil {
		t.Fatal("non-private passfile accepted")
	}
}

func TestNativeIdentityPolicyRequiresAuthorizedVerifiedFixture(t *testing.T) {
	account := "11111111-1111-4111-8111-111111111111"
	policy := map[string]any{"target": map[string]any{"database": "espacigo_dms_stage_20261020", "host": "127.0.0.1", "port": 5432, "expected_database_oid": 16388}, "authorization": map[string]any{"maximum_new_accounts": 2, "synthetic_email_domain": "example.invalid", "synthetic_kyc_rows_write_update_delete": true, "account_inventory": []map[string]any{{"account_id": account, "verified": true}}}}
	raw, err := json.Marshal(policy)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateNativeIdentityPolicy(raw, account); err != nil {
		t.Fatal(err)
	}
	if validateNativeIdentityPolicy(raw, "22222222-2222-4222-8222-222222222222") == nil {
		t.Fatal("unlisted account accepted")
	}
	auth := policy["authorization"].(map[string]any)
	auth["account_inventory"] = []map[string]any{{"account_id": account, "verified": false}}
	raw, _ = json.Marshal(policy)
	if validateNativeIdentityPolicy(raw, account) == nil {
		t.Fatal("unverified fixture accepted")
	}
}
