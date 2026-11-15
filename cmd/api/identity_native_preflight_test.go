package main

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"

	"path/filepath"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/migrator"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type nativeIdentityPreflight struct {
	Database string                  `json:"database"`
	OID      int64                   `json:"oid"`
	Version  string                  `json:"version"`
	Session  string                  `json:"session"`
	Current  string                  `json:"current"`
	ReadOnly bool                    `json:"read_only"`
	Ledger   []migrator.HistoryEntry `json:"ledger"`
}

const nativeIdentityPreflightVersion = identityDeclarationNativeLedgerVersion
const nativeIdentityPreflightAnySupportedVersion = 0

func validateNativeIdentityPreflight(raw []byte, source []migrator.Migration, version int) error {
	var report nativeIdentityPreflight
	if err := json.Unmarshal(raw, &report); err != nil {
		return errors.New("native identity preflight returned invalid JSON")
	}
	if report.Database != "espacigo_dms_stage_20261020" || report.OID != 16388 || report.Version != "180007" || report.Session != "espacigo_migrator_login" || report.Current != "espacigo_schema_owner" || !report.ReadOnly {
		return errors.New("native identity preflight target or read-only identity mismatch")
	}
	if version == nativeIdentityPreflightAnySupportedVersion {
		if len(report.Ledger) < 89 || len(report.Ledger) > nativeIdentityPreflightVersion {
			return errors.New("native identity preflight requires an exact reviewed V89 through V96 ledger")
		}
		version = len(report.Ledger)
	}
	if version < 89 || version > nativeIdentityPreflightVersion || len(source) < version || len(report.Ledger) != version {
		return errors.New("native identity preflight requires an exact reviewed V89 through V96 ledger")
	}
	for i, h := range report.Ledger {
		if h.Version != int64(i+1) || source[i].Version != h.Version || source[i].Name != h.Name || source[i].Checksum != h.Checksum {
			return errors.New("native identity preflight ledger differs from source")
		}
	}
	finalChecksum := "93671b484f3118c697eebc4654f4b32325cce4231582b488f2e1a8d9b7fece9c"
	if version == 90 {
		finalChecksum = "23bbacde2cf992f912926f84945c701422b097598e698b2e8f38158b61622592"
	} else if version == 91 {
		finalChecksum = identityDeclarationNativeV91Checksum
	} else if version == 92 {
		finalChecksum = identityDeclarationNativeV92Checksum
	} else if version == 93 {
		finalChecksum = identityDeclarationNativeV93Checksum
	} else if version == 94 {
		finalChecksum = identityDeclarationNativeV94Checksum
	} else if version == 95 {
		finalChecksum = identityDeclarationNativeV95Checksum
	} else if version == 96 {
		finalChecksum = identityDeclarationNativeV96Checksum
	}
	if report.Ledger[version-1].Checksum != finalChecksum {
		return errors.New("native identity preflight final migration differs from its reviewed checksum")
	}
	return nil
}

func readNativeIdentityPreflight(ctx context.Context, source []migrator.Migration, version int) error {
	const query = `SELECT json_build_object('database',current_database(),'oid',(SELECT oid::bigint FROM pg_catalog.pg_database WHERE datname=current_database()),'version',current_setting('server_version_num'),'session',session_user,'current',current_user,'read_only',current_setting('transaction_read_only')='on','ledger',(SELECT json_agg(x ORDER BY version) FROM (SELECT version,name,checksum FROM public.schema_migrations) x))`
	config, err := parseNativeIdentityConfig("postgresql://espacigo_migrator_login@127.0.0.1:5432/espacigo_dms_stage_20261020", "/home/nandev/.local/share/espacigo-native-secrets/pgpass")
	if err != nil {
		return err
	}
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return errors.New("native identity read-only preflight pool failed")
	}
	defer pool.Close()
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return errors.New("native identity authenticated read-only preflight failed")
	}
	defer tx.Rollback(context.Background())
	if _, err := tx.Exec(ctx, `SET LOCAL ROLE espacigo_schema_owner`); err != nil {
		return errors.New("native identity read-only preflight role failed")
	}
	var raw []byte
	if err := tx.QueryRow(ctx, query).Scan(&raw); err != nil {
		return errors.New("native identity read-only target/ledger query failed")
	}
	return validateNativeIdentityPreflight(raw, source, version)
}

func TestNativeIdentityExternalReadOnlyPreflight(t *testing.T) {
	if os.Getenv("IDENTITY_DECLARATION_NATIVE_PREFLIGHT_ONLY") != "1" {
		t.Skip("native external preflight is explicitly opt-in")
	}
	source, err := migrator.DiscoverMigrations(filepath.Join("..", "..", "db", "migrations"))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := readNativeIdentityPreflight(ctx, source, nativeIdentityPreflightAnySupportedVersion); err != nil {
		t.Fatal(err)
	}
}

func TestNativeKYCFixtureReadOnlyPreflight(t *testing.T) {
	if os.Getenv("IDENTITY_DECLARATION_NATIVE_PREFLIGHT_ONLY") != "1" {
		t.Skip("native KYC fixture read-only preflight is explicitly opt-in")
	}
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal("native KYC repository root could not be resolved")
	}
	privateDir := filepath.Join(root, ".local", "testing", "kyc-native")
	policyPath := filepath.Join(root, ".local", "testing", "test-account-policy.json")
	if err := validateNativeIdentityPrivateFile(policyPath); err != nil {
		t.Fatal("native KYC private policy metadata is invalid")
	}
	policy, err := os.ReadFile(policyPath)
	if err != nil {
		t.Fatal("native KYC private policy could not be read")
	}
	var fixtures [2]nativeFixtureReceipt
	for i, name := range []string{"fixture.json", "fixture-2.json"} {
		path := filepath.Join(privateDir, name)
		if err := validateNativeIdentityPrivateFile(path); err != nil {
			t.Fatal("authorized native KYC fixture receipt metadata is invalid")
		}
		data, err := os.ReadFile(path)
		if err != nil || json.Unmarshal(data, &fixtures[i]) != nil || !fixtures[i].Verified || !canonicalNativeUUID(fixtures[i].AccountID) {
			t.Fatal("authorized native KYC fixture receipt is invalid")
		}
	}
	accountID, otherAccountID := fixtures[0].AccountID, fixtures[1].AccountID
	if err := validateNativeIdentityAccountPair(policy, accountID, otherAccountID); err != nil {
		t.Fatal("native KYC fixture pair is not explicitly authorized")
	}
	profile, err := runtimeProfile(os.Getenv)
	if err != nil || profile != runtimeProfileKYCWorkerFree {
		t.Fatal("native KYC fixture preflight requires the dedicated worker-free profile")
	}
	cfg, err := loadRuntimeConfig(os.Getenv, profile)
	if err != nil {
		t.Fatal("native KYC fixture runtime configuration is invalid")
	}
	t.Cleanup(cfg.close)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	source, err := migrator.DiscoverMigrations(filepath.Join(root, "db", "migrations"))
	if err != nil || len(source) < identityDeclarationNativeLedgerVersion {
		t.Fatal("native KYC migration source inventory is incomplete")
	}
	if err := readNativeIdentityPreflight(ctx, source, identityDeclarationNativeLedgerVersion); err != nil {
		t.Fatal("native KYC destination or ledger preflight failed")
	}
	pool, closeConnector, err := newDatabasePool(ctx, cfg)
	if err != nil {
		t.Fatal("native KYC read-only application pool could not be initialized")
	}
	t.Cleanup(closeConnector)
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Fatal("native KYC read-only application pool is unavailable")
	}
	var database, session, current, address string
	var port int
	if err := pool.QueryRow(ctx, `SELECT current_database(),session_user,current_user,inet_server_addr()::text,inet_server_port()`).Scan(&database, &session, &current, &address, &port); err != nil {
		t.Fatal("native KYC read-only target identity query failed")
	}
	ip, _, ipErr := net.ParseCIDR(address)
	if database != "espacigo_dms_stage_20261020" || session != "espacigo_api_login" || current != "espacigo_runtime" || ipErr != nil || ip == nil || !ip.IsLoopback() || port != 5432 {
		t.Fatalf("native KYC read-only application target identity mismatch (database=%q session=%q role=%q address=%q port=%d)", database, session, current, address, port)
	}
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		t.Fatal("native KYC fixture preflight could not begin a read-only transaction")
	}
	defer tx.Rollback(context.Background())
	for _, fixtureID := range []string{accountID, otherAccountID} {
		var active bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM public.usuario WHERE id=$1::uuid AND estado='activo')`, fixtureID).Scan(&active); err != nil || !active {
			t.Fatal("authorized native KYC fixture is not an active account")
		}
		var existingRows int64
		if err := tx.QueryRow(ctx, `SELECT
 (SELECT count(*) FROM public.identity_declaration_revision WHERE account_id=$1::uuid)+
 (SELECT count(*) FROM public.identity_provider_attempt WHERE account_id=$1::uuid)+
 (SELECT count(*) FROM public.identity_provider_event e JOIN public.identity_provider_attempt a ON a.id=e.attempt_id WHERE a.account_id=$1::uuid)+
 (SELECT count(*) FROM public.identity_rut_claim WHERE account_id=$1::uuid)+
	(SELECT count(*) FROM public.evento_auditoria_local WHERE actor_id=$1::uuid AND recurso_tipo='identity_declaration')+
	(SELECT count(*) FROM public.outbox_evento_local o WHERE o.agregado_tipo='didit_session' AND o.tipo='didit.session_remote_deletion' AND EXISTS (SELECT 1 FROM public.identity_provider_attempt a WHERE a.account_id=$1::uuid AND (o.clave_deduplicacion='didit_session_deletion:' || a.provider_session_id)))`, fixtureID).Scan(&existingRows); err != nil || existingRows != 0 {
			t.Fatal("authorized native KYC fixture already has artifacts; preserve and reconcile read-only")
		}
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal("native KYC fixture preflight transaction could not commit")
	}
}

func TestNativeIdentityCleanupAuthorityReadOnlyPreflight(t *testing.T) {
	if os.Getenv("IDENTITY_DECLARATION_NATIVE_PREFLIGHT_ONLY") != "1" {
		t.Skip("native cleanup authority read-only preflight is explicitly opt-in")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	source, err := migrator.DiscoverMigrations(filepath.Join("..", "..", "db", "migrations"))
	if err != nil || len(source) < identityDeclarationNativeLedgerVersion {
		t.Fatal("native cleanup authority migration source inventory is incomplete")
	}
	if err := readNativeIdentityPreflight(ctx, source, identityDeclarationNativeLedgerVersion); err != nil {
		t.Fatal("native cleanup authority target/ledger preflight failed")
	}
	cfg, err := parseNativeIdentityConfig(
		"postgresql://espacigo_migrator_login@127.0.0.1:5432/espacigo_dms_stage_20261020",
		"/home/nandev/.local/share/espacigo-native-secrets/pgpass",
	)
	if err != nil {
		t.Fatal("native cleanup authority read-only credentials failed validation")
	}
	cfg.MaxConns = 1
	cfg.AfterConnect = func(connectCtx context.Context, conn *pgx.Conn) error {
		if _, err := conn.Exec(connectCtx, `SET ROLE espacigo_schema_owner`); err != nil {
			return errors.New("native cleanup authority read-only role could not be selected")
		}
		var session, current string
		if err := conn.QueryRow(connectCtx, `SELECT session_user,current_user`).Scan(&session, &current); err != nil || session != "espacigo_migrator_login" || current != "espacigo_schema_owner" {
			return errors.New("native cleanup authority read-only identity mismatch")
		}
		return nil
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal("native cleanup authority read-only pool could not be initialized")
	}
	defer pool.Close()
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		t.Fatal("native cleanup authority read-only transaction could not start")
	}
	defer tx.Rollback(context.Background())
	var database, version, session, current string
	if err := tx.QueryRow(ctx, `SELECT current_database(),current_setting('server_version_num'),session_user,current_user`).Scan(&database, &version, &session, &current); err != nil || database != "espacigo_dms_stage_20261020" || version != "180007" || session != "espacigo_migrator_login" || current != "espacigo_schema_owner" {
		t.Fatal("native cleanup authority read-only target identity mismatch")
	}
	report, err := readNativeIdentityCleanupAuthority(ctx, tx)
	if err != nil {
		t.Fatal("native cleanup authority read-only catalog check failed")
	}
	if issues := nativeIdentityCleanupAuthorityIssues(report); len(issues) != 0 {
		t.Logf("read-only cleanup authority status=BLOCKED; missing or unsafe privileges: %v", issues)
	} else {
		t.Log("read-only cleanup authority status=AVAILABLE; this does not authorize functional writes")
	}
}

func TestNativeIdentityPreflightRejectsUntrustedTargetAndLedger(t *testing.T) {
	source, err := migrator.DiscoverMigrations(filepath.Join("..", "..", "db", "migrations"))
	if err != nil {
		t.Fatal(err)
	}
	for _, version := range []int{89, 90, 91, 92, 93, 94, 95, 96} {
		history := make([]migrator.HistoryEntry, version)
		for i := range history {
			history[i] = migrator.HistoryEntry{Version: source[i].Version, Name: source[i].Name, Checksum: source[i].Checksum}
		}
		valid := map[string]any{"database": "espacigo_dms_stage_20261020", "oid": 16388, "version": "180007", "session": "espacigo_migrator_login", "current": "espacigo_schema_owner", "read_only": true, "ledger": history}
		encode := func(v map[string]any) []byte {
			t.Helper()
			b, e := json.Marshal(v)
			if e != nil {
				t.Fatal(e)
			}
			return b
		}
		if err := validateNativeIdentityPreflight(encode(valid), source, version); err != nil {
			t.Fatalf("valid V%d preflight rejected: %v", version, err)
		}
		if err := validateNativeIdentityPreflight(encode(valid), source, 0); err != nil {
			t.Fatalf("supported-current-tail preflight rejected exact V%d ledger: %v", version, err)
		}
		if version == 90 {
			valid["ledger"] = history[:89]
			if validateNativeIdentityPreflight(encode(valid), source, version) == nil {
				t.Fatal("V89 preflight accepted for a V90 target")
			}
			valid["ledger"] = history
		}
		for _, tc := range []struct {
			name, key string
			value     any
		}{
			{"wrong_database", "database", "postgres"}, {"wrong_oid", "oid", 42}, {"wrong_engine", "version", "170000"}, {"previous_engine", "version", "180006"},
			{"wrong_session", "session", "other"}, {"wrong_role", "current", "other"}, {"not_read_only", "read_only", false},
			{"missing_ledger", "ledger", nil}, {"incomplete_ledger", "ledger", history[:version-1]},
		} {
			t.Run(tc.name, func(t *testing.T) {
				copy := map[string]any{}
				for k, v := range valid {
					copy[k] = v
				}
				copy[tc.key] = tc.value
				if validateNativeIdentityPreflight(encode(copy), source, version) == nil {
					t.Fatal("unsafe preflight accepted")
				}
			})
		}
		for _, raw := range []string{"", "{}", "null", "{", string(encode(valid)) + " {}"} {
			if validateNativeIdentityPreflight([]byte(raw), source, version) == nil {
				t.Fatal("malformed preflight accepted")
			}
		}
	}
}
