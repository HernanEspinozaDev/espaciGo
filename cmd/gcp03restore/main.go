package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	"cloud.google.com/go/storage"
	"github.com/HernanEspinozaDev/espaciGo/internal/dbbootstrap"
	"github.com/HernanEspinozaDev/espaciGo/internal/gcp03restore"
	"github.com/HernanEspinozaDev/espaciGo/internal/migrator"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const maxArtifactBytes = 64 << 20

var migrationDirectory = "/app/db/migrations"

const migrationCount = 61

type config struct {
	project, region, instance, runID, bucket, manifestSHA, migrationSHA, confirmTarget string
	phase, databaseURL, adminPasswordFile                                              string
}

var runIDPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{5,11}$`)
var manifestHashPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
var adminPasswordPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)

func main() {
	os.Exit(execute())
}

func execute() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	evidence, runErr := runWithProxyShutdown(ctx, runContext, func(ctx context.Context) error {
		return shutdownProxy(ctx, http.DefaultClient)
	})
	stop()
	if runErr != nil {
		writeFailureEvidence(os.Stderr, os.Getenv, runErr)
		return 1
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetEscapeHTML(true)
	if err := encoder.Encode(evidence); err != nil {
		writeFailureEvidence(os.Stderr, os.Getenv, errors.New("evidence encoding failed"))
		return 1
	}
	return 0
}

func runWithProxyShutdown(ctx context.Context, runner func(context.Context) (evidence, error), shutdown func(context.Context) error) (evidence, error) {
	result, runErr := runner(ctx)
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	shutdownErr := shutdown(shutdownCtx)
	if runErr != nil {
		return result, runFailure{cause: runErr, proxyShutdownConfirmed: shutdownErr == nil}
	}
	if shutdownErr != nil {
		return evidence{}, runFailure{cause: errors.New("proxy shutdown could not be confirmed"), proxyShutdownConfirmed: false}
	}
	return result, nil
}

type httpDoer interface {
	Do(*http.Request) (*http.Response, error)
}

func shutdownProxy(ctx context.Context, client httpDoer) error {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://127.0.0.1:9091/quitquitquit", nil)
		if err != nil {
			return errors.New("fixed proxy shutdown request could not be constructed")
		}
		response, err := client.Do(req)
		if err == nil {
			_ = response.Body.Close()
			if response.StatusCode >= 200 && response.StatusCode < 300 {
				return nil
			}
			lastErr = errors.New("proxy shutdown endpoint returned a non-success status")
		} else {
			lastErr = errors.New("proxy shutdown endpoint was unavailable")
		}
		if attempt < 2 {
			select {
			case <-ctx.Done():
				return errors.New("proxy shutdown could not be confirmed before timeout")
			case <-time.After(100 * time.Millisecond):
			}
		}
	}
	return lastErr
}

type failureEvidence struct {
	Format                 string `json:"format"`
	RunID                  string `json:"run_id,omitempty"`
	Phase                  string `json:"phase"`
	Stage                  string `json:"stage"`
	ErrorCode              string `json:"error_code"`
	SQLState               string `json:"sqlstate,omitempty"`
	SubprocessExitCode     int    `json:"subprocess_exit_code,omitempty"`
	CommitOutcomeUncertain bool   `json:"commit_outcome_uncertain"`
	ProxyShutdownConfirmed bool   `json:"proxy_shutdown_confirmed"`
	ManifestSHA256         string `json:"manifest_sha256,omitempty"`
	FailureSHA256          string `json:"failure_sha256"`
}

type subprocessFailure struct {
	code     string
	exitCode int
	sqlState string
}

func (e subprocessFailure) Error() string { return e.code }

type runFailure struct {
	cause                  error
	proxyShutdownConfirmed bool
}

func (e runFailure) Error() string { return e.cause.Error() }
func (e runFailure) Unwrap() error { return e.cause }

func writeFailureEvidence(out io.Writer, getenv func(string) string, err error) {
	phase := getenv("GCP03_PHASE")
	if !validPhase(phase) {
		phase = "preflight"
	}
	record := failureEvidence{Format: "espacigo-gcp03-managed-synthetic-restore-failure-v1", Phase: phase, Stage: "phase_execute", ErrorCode: "PHASE_FAILED"}
	if runID := getenv("GCP03_RUN_ID"); runIDPattern.MatchString(runID) {
		record.RunID = runID
	}
	if hash := getenv("GCP03_MANIFEST_SHA256"); manifestHashPattern.MatchString(hash) {
		record.ManifestSHA256 = hash
	}
	message := ""
	if err != nil {
		message = err.Error()
	}
	var phaseFailure runFailure
	if errors.As(err, &phaseFailure) {
		record.ProxyShutdownConfirmed = phaseFailure.proxyShutdownConfirmed
	}
	switch {
	case strings.Contains(message, "uncertain"):
		record.Stage, record.ErrorCode, record.CommitOutcomeUncertain = "commit", "COMMIT_OUTCOME_UNCERTAIN", true
	case strings.Contains(message, "preflight") || strings.Contains(message, "refused") || strings.Contains(message, "already") || strings.Contains(message, "empty"):
		record.Stage, record.ErrorCode = "preflight", "PRECONDITION_FAILED"
	case strings.Contains(message, "manifest") || strings.Contains(message, "artifact") || strings.Contains(message, "TOC") || strings.Contains(message, "dump"):
		record.Stage, record.ErrorCode = "artifact_validation", "ARTIFACT_VALIDATION_FAILED"
	case strings.Contains(message, "proxy shutdown"):
		record.Phase, record.Stage, record.ErrorCode = phase, "proxy_shutdown", "PROXY_SHUTDOWN_UNCONFIRMED"
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		record.Stage = "database_operation"
		record.SQLState = pgErr.Code
		record.ErrorCode = "DATABASE_SQLSTATE"
	}
	var processErr subprocessFailure
	if errors.As(err, &processErr) {
		record.Stage = "pg_restore"
		record.ErrorCode = processErr.code
		record.SubprocessExitCode = processErr.exitCode
		record.SQLState = processErr.sqlState
	}
	withoutHash := record
	withoutHash.FailureSHA256 = ""
	canonical, _ := json.Marshal(withoutHash)
	sum := sha256.Sum256(canonical)
	record.FailureSHA256 = hex.EncodeToString(sum[:])
	enc := json.NewEncoder(out)
	_ = enc.Encode(record)
}

func validPhase(phase string) bool {
	switch phase {
	case "bootstrap", "restore", "verify", "retire":
		return true
	default:
		return false
	}
}

func run() (evidence, error) {
	return runContext(context.Background())
}

func runContext(parent context.Context) (evidence, error) {
	config, err := loadConfig(os.Getenv)
	if err != nil {
		return evidence{}, err
	}
	ctx, cancel := context.WithTimeout(parent, 12*time.Minute)
	defer cancel()
	return runConfigured(ctx, config)
}

func loadConfig(getenv func(string) string) (config, error) {
	c := config{
		project: getenv("GCP03_PROJECT"), region: getenv("GCP03_REGION"), instance: getenv("GCP03_INSTANCE"),
		runID: getenv("GCP03_RUN_ID"), bucket: getenv("GCP03_BUCKET"), manifestSHA: getenv("GCP03_MANIFEST_SHA256"),
		migrationSHA:  getenv("GCP03_MIGRATION_SHA256"),
		confirmTarget: getenv("GCP03_CONFIRM_TARGET"), databaseURL: getenv("DATABASE_URL"),
	}
	c.phase = getenv("GCP03_PHASE")
	if c.project != "espacigo-testprod" || c.region != "southamerica-west1" ||
		!runIDPattern.MatchString(c.runID) || c.instance != "espacigo-gcp03-restore-"+c.runID+"-pg" ||
		c.confirmTarget != c.project+":"+c.region+":"+c.instance+"/espacigo_scratch" {
		return config{}, errors.New("GCP-03 phase target is not the exact reviewed run")
	}
	u, err := url.Parse(c.databaseURL)
	hasInlinePassword := false
	if u != nil && u.User != nil {
		_, hasInlinePassword = u.User.Password()
	}
	if err != nil || u.Scheme != "postgres" || u.User == nil || u.User.Username() == "" || hasInlinePassword ||
		u.Host != "127.0.0.1:5432" || u.Path != "/espacigo_scratch" || u.Query().Get("sslmode") != "disable" || len(u.Query()) != 1 {
		return config{}, errors.New("GCP-03 URL must target the local Cloud SQL Auth Proxy and fixed database")
	}
	switch c.phase {
	case "bootstrap", "retire":
		if u.User.Username() != "postgres" || c.bucket != "" || c.manifestSHA != "" || getenv("GCP03_ADMIN_PASSWORD_FILE") != "/secrets/postgres/password" {
			return config{}, errors.New("GCP-03 administrative phase inputs are invalid")
		}
		c.adminPasswordFile = "/secrets/postgres/password"
	case "restore":
		if u.User.Username() != iamSQLUsername("restore", c.runID) || c.bucket != "espacigo-gcp03-restore-"+c.runID || !manifestHashPattern.MatchString(c.manifestSHA) {
			return config{}, errors.New("GCP-03 restore URL or artifact binding is invalid")
		}
	case "verify":
		if u.User.Username() != iamSQLUsername("verify", c.runID) || c.bucket != "" || !manifestHashPattern.MatchString(c.manifestSHA) || !manifestHashPattern.MatchString(c.migrationSHA) {
			return config{}, errors.New("GCP-03 verification identity or manifest binding is invalid")
		}
	default:
		return config{}, errors.New("GCP03_PHASE must be bootstrap, restore, verify, or retire")
	}
	return c, nil
}

type evidence struct {
	Format                   string          `json:"format"`
	RunID                    string          `json:"run_id"`
	Phase                    string          `json:"phase"`
	Target                   string          `json:"target"`
	ManifestSHA256           string          `json:"manifest_sha256"`
	DumpBytes                int64           `json:"dump_bytes"`
	DumpSHA256               string          `json:"dump_sha256"`
	TOCSHA256                string          `json:"toc_sha256"`
	AllowlistSHA256          string          `json:"allowlist_sha256"`
	MigrationCount           int             `json:"migration_count"`
	MigrationInventorySHA256 string          `json:"migration_inventory_sha256"`
	RestoredCounts           map[string]int  `json:"restored_counts"`
	Checks                   map[string]bool `json:"checks"`
}

type verification struct {
	counts map[string]int
	checks map[string]bool
}

func phaseEvidence(c config, phase string, checks map[string]bool) evidence {
	return evidence{Format: "espacigo-gcp03-managed-synthetic-restore-evidence-v1", RunID: c.runID,
		Phase: phase, Target: c.confirmTarget, Checks: checks}
}

func readAdminPassword(path string) (string, error) {
	if path != "/secrets/postgres/password" {
		return "", errors.New("administrative secret path is not the fixed mount")
	}
	data, err := os.ReadFile(path)
	if err != nil || !adminPasswordPattern.MatchString(string(data)) {
		return "", errors.New("administrative secret format is invalid")
	}
	return string(data), nil
}

func connectAdmin(ctx context.Context, databaseURL, password string) (*pgx.Conn, error) {
	parsed, err := pgx.ParseConfig(databaseURL)
	if err != nil {
		return nil, errors.New("administrative database URL is invalid")
	}
	parsed.Password = password
	conn, err := pgx.ConnectConfig(ctx, parsed)
	if err != nil {
		return nil, errors.New("administrative database connection failed")
	}
	var db, user string
	if err := conn.QueryRow(ctx, "SELECT current_database(),current_user").Scan(&db, &user); err != nil || db != "espacigo_scratch" || user != "postgres" {
		_ = conn.Close(context.Background())
		return nil, errors.New("administrative connection did not reach the fixed Cloud SQL database owner")
	}
	return conn, nil
}

func bootstrap(ctx context.Context, databaseURL, password, runID string) error {
	conn, err := connectAdmin(ctx, databaseURL, password)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())
	var migrations bool
	var nonExtensionRelations int
	if err := conn.QueryRow(ctx, "SELECT to_regclass('public.schema_migrations') IS NOT NULL").Scan(&migrations); err != nil || migrations {
		return errors.New("bootstrap target already has migration history")
	}
	if err := conn.QueryRow(ctx, `SELECT count(*) FROM pg_catalog.pg_class c
 JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
 WHERE n.nspname='public' AND c.relkind IN ('r','p','v','m','S','f')
 AND NOT EXISTS(SELECT 1 FROM pg_catalog.pg_depend d WHERE d.classid='pg_catalog.pg_class'::regclass AND d.objid=c.oid AND d.deptype='e')`).Scan(&nonExtensionRelations); err != nil || nonExtensionRelations != 0 {
		return errors.New("bootstrap target is not empty except for preinstalled extensions")
	}
	owner, verifier := ownerRole(runID), verifyRole(runID)
	roles := []string{owner, verifier, "espacigo_runtime", "espacigo_gadm_bootstrap", "espacigo_scratch_migrator"}
	var existing int
	if err := conn.QueryRow(ctx, "SELECT count(*) FROM pg_catalog.pg_roles WHERE rolname=ANY($1::text[])", roles).Scan(&existing); err != nil || existing != 0 {
		return errors.New("bootstrap refuses pre-existing run or application roles")
	}
	tx, err := conn.Begin(ctx)
	if err != nil {
		return errors.New("could not start one-shot bootstrap transaction")
	}
	defer tx.Rollback(context.Background())
	for _, ext := range []string{"btree_gist", "postgis", "unaccent"} {
		if _, err := tx.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS "+pgx.Identifier{ext}.Sanitize()); err != nil {
			return errors.New("required Cloud SQL extension could not be installed")
		}
	}
	for _, role := range []string{owner, "espacigo_runtime", "espacigo_gadm_bootstrap", "espacigo_scratch_migrator"} {
		if _, err := tx.Exec(ctx, "CREATE ROLE "+pgx.Identifier{role}.Sanitize()+" NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT"); err != nil {
			return errors.New("fixed restricted owner/application role creation failed")
		}
	}
	if _, err := tx.Exec(ctx, "CREATE ROLE "+pgx.Identifier{verifier}.Sanitize()+" NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS INHERIT"); err != nil {
		return errors.New("fixed verification role creation failed")
	}
	for _, statement := range []string{
		"REVOKE TEMPORARY ON DATABASE espacigo_scratch FROM PUBLIC",
		"REVOKE CREATE ON SCHEMA public FROM PUBLIC",
		"GRANT TEMPORARY ON DATABASE espacigo_scratch TO espacigo_scratch_migrator",
		"GRANT CREATE ON SCHEMA public TO espacigo_scratch_migrator",
		"GRANT CONNECT ON DATABASE espacigo_scratch TO " + pgx.Identifier{owner}.Sanitize(),
		"GRANT CONNECT ON DATABASE espacigo_scratch TO " + pgx.Identifier{verifier}.Sanitize(),
		"GRANT CONNECT ON DATABASE espacigo_scratch TO espacigo_runtime",
		"GRANT USAGE, CREATE ON SCHEMA public TO " + pgx.Identifier{owner}.Sanitize() + " WITH GRANT OPTION",
	} {
		if _, err := tx.Exec(ctx, statement); err != nil {
			return errors.New("fixed bootstrap ACL configuration failed")
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return errors.New("bootstrap transaction outcome is uncertain; do not retry")
	}
	if err := dbbootstrap.VerifyScratchBootstrap(ctx, conn); err != nil {
		return fmt.Errorf("bootstrap extension and role postcheck failed: %w", err)
	}
	if err := dbbootstrap.VerifyScratchDatabaseACL(ctx, conn); err != nil {
		return fmt.Errorf("bootstrap database and schema ACL postcheck failed: %w", err)
	}
	if _, err := dbbootstrap.ScratchExtensionVersions(ctx, conn); err != nil {
		return errors.New("bootstrap extension inventory postcheck failed")
	}
	return nil
}

func retire(ctx context.Context, databaseURL, password, runID string) error {
	conn, err := connectAdmin(ctx, databaseURL, password)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())
	if _, err := migrationInventory(migrationDirectory); err != nil {
		return errors.New("retirement migration inventory preflight failed")
	}
	if err := migrator.VerifyAllApplied(ctx, conn, migrationDirectory, migrationCount); err != nil {
		return errors.New("retirement refused because V1-V61 history no longer verifies")
	}
	if err := dbbootstrap.VerifyScratchRuntimePermissions(ctx, conn); err != nil {
		return errors.New("retirement refused because runtime ACL checks no longer pass")
	}
	if err := verifyRetirementAuditACL(ctx, conn); err != nil {
		return errors.New("retirement refused because the fixed audit grant contract does not pass")
	}
	var users, spaces, occupancy int
	if err := conn.QueryRow(ctx, `SELECT (SELECT count(*) FROM public.usuario),(SELECT count(*) FROM public.espacio),(SELECT count(*) FROM public.ocupacion)`).Scan(&users, &spaces, &occupancy); err != nil || users != 2 || spaces != 1 || occupancy != 2 {
		return errors.New("retirement refused because synthetic restore counts changed")
	}
	restoreLogin, verifyLogin := iamSQLUsername("restore", runID), iamSQLUsername("verify", runID)
	for _, pair := range [][2]string{{restoreLogin, ownerRole(runID)}, {verifyLogin, verifyRole(runID)}} {
		var canLogin, superuser, member bool
		if err := conn.QueryRow(ctx, `SELECT u.rolcanlogin,u.rolsuper,pg_catalog.pg_has_role(u.rolname,g.rolname,'MEMBER')
 FROM pg_catalog.pg_roles u JOIN pg_catalog.pg_roles g ON g.rolname=$2 WHERE u.rolname=$1`, pair[0], pair[1]).Scan(&canLogin, &superuser, &member); err != nil || !canLogin || superuser || !member {
			return errors.New("retirement preflight expected both IAM users to remain login-capable with their exact group role")
		}
	}
	tx, err := conn.Begin(ctx)
	if err != nil {
		return errors.New("retirement could not begin its atomic cleanup transaction")
	}
	defer tx.Rollback(context.Background())
	if _, err := tx.Exec(ctx, `DELETE FROM public.ocupacion WHERE id IN
 ('eeeeeeee-eeee-4eee-8eee-eeeeeeeeee03','eeeeeeee-eeee-4eee-8eee-eeeeeeeeee04','eeeeeeee-eeee-4eee-8eee-eeeeeeeeee05')`); err != nil {
		return errors.New("fixed synthetic calendar verification rows could not be removed")
	}
	for _, statement := range []string{"REVOKE TEMPORARY ON DATABASE espacigo_scratch FROM espacigo_scratch_migrator", "REVOKE CREATE ON SCHEMA public FROM espacigo_scratch_migrator"} {
		if _, err := tx.Exec(ctx, statement); err != nil {
			return errors.New("fixed synthetic migrator ACL retirement failed")
		}
	}
	if err := dbbootstrap.VerifyScratchRetiredRuntimePermissions(ctx, tx); err != nil {
		return errors.New("retired runtime ACL postcheck failed")
	}
	if err := dbbootstrap.VerifyScratchMigratorRetired(ctx, tx); err != nil {
		return errors.New("retired synthetic migrator role postcheck failed")
	}
	if err := verifyRetirementAuditACL(ctx, tx); err != nil {
		return errors.New("retirement changed the explicit audit privilege set")
	}
	var remainingUsers, remainingSpaces, remainingOccupancy, baselineOccupancy int
	if err := tx.QueryRow(ctx, `SELECT (SELECT count(*) FROM public.usuario),
 (SELECT count(*) FROM public.espacio),(SELECT count(*) FROM public.ocupacion),
 (SELECT count(*) FROM public.ocupacion WHERE id IN ('eeeeeeee-eeee-4eee-8eee-eeeeeeeeee01','eeeeeeee-eeee-4eee-8eee-eeeeeeeeee02'))`).Scan(&remainingUsers, &remainingSpaces, &remainingOccupancy, &baselineOccupancy); err != nil || remainingUsers != 2 || remainingSpaces != 1 || remainingOccupancy != 2 || baselineOccupancy != 2 {
		return errors.New("retirement changed the fixed synthetic baseline rows")
	}
	for _, login := range []string{restoreLogin, verifyLogin} {
		groupRole := ownerRole(runID)
		if login == verifyLogin {
			groupRole = verifyRole(runID)
		}
		var stillLogin, expectedMembership bool
		var membershipCount int
		if err := tx.QueryRow(ctx, `SELECT u.rolcanlogin,
 EXISTS(SELECT 1 FROM pg_catalog.pg_auth_members m JOIN pg_catalog.pg_roles granted ON granted.oid=m.roleid WHERE m.member=u.oid AND granted.rolname=$2),
 (SELECT count(*) FROM pg_catalog.pg_auth_members m WHERE m.member=u.oid)
 FROM pg_catalog.pg_roles u WHERE u.rolname=$1`, login, groupRole).Scan(&stillLogin, &expectedMembership, &membershipCount); err != nil || !stillLogin || !expectedMembership || membershipCount != 1 {
			return errors.New("retirement did not preserve login capability and the exact IAM SQL role binding")
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return errors.New("retirement commit outcome is uncertain; do not retry")
	}
	return nil
}

func grantRetirementAuditAccess(ctx context.Context, conn *pgx.Conn) error {
	for _, statement := range []string{
		"GRANT SELECT ON TABLE public.schema_migrations, public.usuario, public.espacio, public.ocupacion TO postgres",
		"GRANT DELETE ON TABLE public.ocupacion TO postgres",
	} {
		if _, err := conn.Exec(ctx, statement); err != nil {
			return errors.New("fixed retirement audit grant could not be applied")
		}
	}
	return verifyRetirementAuditACL(ctx, conn)
}

func verifyRetirementAuditACL(ctx context.Context, db dbbootstrap.ScratchPermissionDB) error {
	var hasSelect, hasDelete, exactDirectACL, noPublicReadWrite bool
	var directCount int
	err := db.QueryRow(ctx, `WITH actual AS (
 SELECT c.relname, acl.privilege_type, acl.is_grantable
 FROM pg_catalog.pg_class c
 JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
 CROSS JOIN LATERAL pg_catalog.aclexplode(c.relacl) acl
 JOIN pg_catalog.pg_roles grantee ON grantee.oid=acl.grantee
 WHERE n.nspname='public' AND c.relname IN ('schema_migrations','usuario','espacio','ocupacion')
   AND c.relkind IN ('r','p','v','m') AND grantee.rolname='postgres'
), expected(relname,privilege_type) AS (
 VALUES ('schema_migrations','SELECT'),('usuario','SELECT'),('espacio','SELECT'),('ocupacion','SELECT'),('ocupacion','DELETE')
), public_acl AS (
 SELECT 1 FROM pg_catalog.pg_class c
 JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
 CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(c.relacl,pg_catalog.acldefault('r',c.relowner))) acl
 WHERE n.nspname='public' AND c.relname IN ('schema_migrations','usuario','espacio','ocupacion')
   AND acl.grantee=0 AND acl.privilege_type IN ('SELECT','DELETE')
)
SELECT has_table_privilege('postgres','public.schema_migrations','SELECT'),
 has_table_privilege('postgres','public.usuario','SELECT'),
 (SELECT count(*)=5 AND NOT EXISTS(
   SELECT relname,privilege_type FROM actual EXCEPT SELECT relname,privilege_type FROM expected
 ) AND NOT EXISTS(
   SELECT relname,privilege_type FROM expected EXCEPT SELECT relname,privilege_type FROM actual
 ) AND NOT EXISTS(SELECT 1 FROM actual WHERE is_grantable)
 FROM actual),
 NOT EXISTS(SELECT 1 FROM public_acl)
 AND has_table_privilege('postgres','public.espacio','SELECT')
 AND has_table_privilege('postgres','public.ocupacion','SELECT')
 AND has_table_privilege('postgres','public.ocupacion','DELETE'),
 (SELECT count(*) FROM actual)`).Scan(&hasSelect, &hasDelete, &exactDirectACL, &noPublicReadWrite, &directCount)
	if err != nil {
		return errors.New("retirement audit ACL query failed")
	}
	if !hasSelect {
		return errors.New("retirement audit migration-history SELECT grant is missing")
	}
	if !hasDelete {
		return errors.New("retirement audit user SELECT grant is missing")
	}
	if !exactDirectACL {
		return fmt.Errorf("retirement audit direct grant set is not exact; explicit privilege rows=%d", directCount)
	}
	if !noPublicReadWrite {
		return errors.New("retirement audit fixture grants or PUBLIC ACL differ from the fixed contract")
	}
	var runtimeSchemaHistory bool
	if err := db.QueryRow(ctx, `SELECT has_table_privilege('espacigo_runtime','public.schema_migrations','SELECT')`).Scan(&runtimeSchemaHistory); err != nil || runtimeSchemaHistory {
		return errors.New("retirement audit grant leaked migration history to runtime")
	}
	return nil
}

func ownerRole(runID string) string {
	return "g03_restore_owner_" + strings.ReplaceAll(runID, "-", "_")
}
func verifyRole(runID string) string {
	return "g03_restore_verify_" + strings.ReplaceAll(runID, "-", "_")
}

func iamSQLUsername(kind, runID string) string {
	return "g03-" + kind + "-" + runID + "@espacigo-testprod.iam"
}

func runConfigured(ctx context.Context, c config) (evidence, error) {
	if err := waitProxy(ctx, "127.0.0.1:5432"); err != nil {
		return evidence{}, errors.New("Cloud SQL Auth Proxy did not become ready")
	}
	if c.phase == "bootstrap" || c.phase == "retire" {
		password, err := readAdminPassword(c.adminPasswordFile)
		if err != nil {
			return evidence{}, err
		}
		if c.phase == "bootstrap" {
			if err := bootstrap(ctx, c.databaseURL, password, c.runID); err != nil {
				return evidence{}, err
			}
			return phaseEvidence(c, "bootstrap", map[string]bool{"roles_created": true, "extensions_ready": true, "target_empty": true}), nil
		}
		if err := retire(ctx, c.databaseURL, password, c.runID); err != nil {
			return evidence{}, err
		}
		return phaseEvidence(c, "retire", map[string]bool{"synthetic_rows_removed": true, "iam_users_remain_login_capable": true, "iam_role_bindings_preserved_for_reviewed_terraform_cleanup": true, "runtime_acl_retired": true}), nil
	}
	if c.phase == "verify" {
		verified, err := verifyRestored(ctx, c.databaseURL, c.runID, c.migrationSHA, false)
		if err != nil {
			return evidence{}, err
		}
		return evidence{Format: "espacigo-gcp03-managed-synthetic-restore-evidence-v1", RunID: c.runID, Phase: "verify", Target: c.confirmTarget, ManifestSHA256: c.manifestSHA, MigrationCount: migrationCount, MigrationInventorySHA256: c.migrationSHA, RestoredCounts: verified.counts, Checks: verified.checks}, nil
	}

	client, err := storage.NewClient(ctx)
	if err != nil {
		return evidence{}, errors.New("could not initialize artifact reader")
	}
	defer client.Close()
	manifestObject := "runs/" + c.runID + "/manifest.json"
	manifestBytes, err := download(ctx, client, c.bucket, manifestObject, 1<<20)
	if err != nil {
		return evidence{}, errors.New("could not read run manifest")
	}
	manifest, err := gcp03restore.DecodeManifest(manifestBytes, c.manifestSHA)
	if err != nil || manifest.RunID != c.runID {
		return evidence{}, errors.New("run manifest validation failed")
	}
	dump, err := download(ctx, client, c.bucket, manifest.DumpObject, maxArtifactBytes)
	if err != nil || gcp03restore.VerifyDump(dump, manifest) != nil {
		return evidence{}, errors.New("synthetic dump validation failed")
	}
	rawTOC, err := download(ctx, client, c.bucket, manifest.TOCObject, 8<<20)
	if err != nil {
		return evidence{}, errors.New("could not read dump TOC")
	}
	allowlist, err := download(ctx, client, c.bucket, manifest.AllowlistObject, 8<<20)
	if err != nil || gcp03restore.VerifyTOC(rawTOC, allowlist, manifest) != nil {
		return evidence{}, errors.New("restore allowlist validation failed")
	}

	verified, err := restoreSynthetic(ctx, c.databaseURL, c.runID, manifest, dump, rawTOC, allowlist)
	if err != nil {
		return evidence{}, err
	}
	return evidence{
		Format: "espacigo-gcp03-managed-synthetic-restore-evidence-v1", RunID: c.runID, Phase: "restore",
		Target: c.confirmTarget, ManifestSHA256: c.manifestSHA, DumpBytes: manifest.DumpBytes,
		DumpSHA256: manifest.DumpSHA256, TOCSHA256: manifest.TOCSHA256, AllowlistSHA256: manifest.AllowlistSHA256,
		MigrationCount: manifest.MigrationCount, MigrationInventorySHA256: manifest.MigrationSHA256,
		RestoredCounts: verified.counts, Checks: verified.checks,
	}, nil
}

func restoreSynthetic(ctx context.Context, databaseURL, runID string, manifest gcp03restore.Manifest, dump, rawTOC, allowlist []byte) (verification, error) {
	var empty verification
	if err := gcp03restore.VerifyDump(dump, manifest); err != nil || gcp03restore.VerifyTOC(rawTOC, allowlist, manifest) != nil {
		return empty, errors.New("synthetic artifact hashes or TOC allowlist are invalid")
	}
	if err := checkRestorePrerequisites(ctx, databaseURL, runID); err != nil {
		return empty, err
	}
	tmp, err := os.MkdirTemp("", "gcp03-restore-")
	if err != nil {
		return empty, errors.New("could not allocate temporary restore workspace")
	}
	defer os.RemoveAll(tmp)
	dumpPath, listPath := filepath.Join(tmp, "restore.dump"), filepath.Join(tmp, "allowlist.toc")
	if err := writePrivate(dumpPath, dump); err != nil {
		return empty, errors.New("could not stage verified dump")
	}
	if err := writePrivate(listPath, allowlist); err != nil {
		return empty, errors.New("could not stage verified allowlist")
	}
	actualTOC, err := exec.CommandContext(ctx, "pg_restore", "--list", dumpPath).Output()
	if err != nil || gcp03restore.VerifyTOC(actualTOC, allowlist, manifest) != nil || !bytes.Equal(actualTOC, rawTOC) {
		return empty, errors.New("dump contents do not match the reviewed TOC")
	}
	owner := ownerRole(runID)
	cmd := exec.CommandContext(ctx, "pg_restore", "--exit-on-error", "--single-transaction", "--no-owner", "--no-acl", "--role="+owner, "--use-list="+listPath, "--dbname="+databaseURL, dumpPath)
	var restoreStderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = io.Discard, &restoreStderr
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return empty, subprocessFailure{code: classifyPGRestoreFailure(restoreStderr.String()), exitCode: exitErr.ExitCode(), sqlState: postgresSQLState(restoreStderr.String())}
		}
		return empty, subprocessFailure{code: "PG_RESTORE_START_FAILED", exitCode: -1}
	}
	conn, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		return empty, errors.New("could not connect to apply fixed synthetic ACL checks")
	}
	defer conn.Close(context.Background())
	if _, err := conn.Exec(ctx, "SET ROLE "+pgx.Identifier{owner}.Sanitize()); err != nil {
		return empty, errors.New("could not select restore owner for ACL checks")
	}
	if err := grantRetirementAuditAccess(ctx, conn); err != nil {
		return empty, fmt.Errorf("fixed retirement audit ACL setup failed: %w", err)
	}
	if err := dbbootstrap.GrantRuntimePermissions(ctx, conn); err != nil {
		return empty, errors.New("runtime ACL restoration failed")
	}
	if _, err := conn.Exec(ctx, "GRANT USAGE ON SCHEMA public TO "+pgx.Identifier{verifyRole(runID)}.Sanitize()); err != nil {
		return empty, errors.New("verification role schema grant failed")
	}
	if _, err := conn.Exec(ctx, "GRANT SELECT ON ALL TABLES IN SCHEMA public TO "+pgx.Identifier{verifyRole(runID)}.Sanitize()); err != nil {
		return empty, errors.New("verification role read grants failed")
	}
	if _, err := conn.Exec(ctx, "GRANT INSERT, DELETE ON TABLE public.ocupacion TO "+pgx.Identifier{verifyRole(runID)}.Sanitize()); err != nil {
		return empty, errors.New("verification role calendar test grants failed")
	}
	if err := dbbootstrap.VerifyScratchRuntimePermissions(ctx, conn); err != nil {
		return empty, errors.New("restored runtime ACL contract failed")
	}
	if err := verifyRetirementAuditACL(ctx, conn); err != nil {
		return empty, errors.New("restored retirement audit ACL contract failed")
	}
	return verifyRestored(ctx, databaseURL, runID, manifest.MigrationSHA256, true)
}

var sqlStatePattern = regexp.MustCompile(`SQL state: ([0-9A-Z]{5})`)

func postgresSQLState(stderr string) string {
	match := sqlStatePattern.FindStringSubmatch(stderr)
	if len(match) != 2 {
		return ""
	}
	return match[1]
}

func classifyPGRestoreFailure(stderr string) string {
	lower := strings.ToLower(stderr)
	switch {
	case strings.Contains(lower, "permission denied for schema"):
		return "PG_RESTORE_SCHEMA_PERMISSION_DENIED"
	case strings.Contains(lower, "permission denied for table") || strings.Contains(lower, "permission denied for relation"):
		return "PG_RESTORE_RELATION_PERMISSION_DENIED"
	case strings.Contains(lower, "permission denied for function"):
		return "PG_RESTORE_FUNCTION_PERMISSION_DENIED"
	case strings.Contains(lower, "permission denied for sequence"):
		return "PG_RESTORE_SEQUENCE_PERMISSION_DENIED"
	case strings.Contains(lower, "permission denied for language"):
		return "PG_RESTORE_LANGUAGE_PERMISSION_DENIED"
	case strings.Contains(lower, "permission denied"):
		return "PG_RESTORE_PERMISSION_DENIED"
	case strings.Contains(lower, "already exists"):
		return "PG_RESTORE_OBJECT_CONFLICT"
	case strings.Contains(lower, "does not exist"):
		return "PG_RESTORE_DEPENDENCY_MISSING"
	case strings.Contains(lower, "must be owner"):
		return "PG_RESTORE_OWNERSHIP_FAILED"
	default:
		return "PG_RESTORE_SQL_FAILED"
	}
}

func checkRestorePrerequisites(ctx context.Context, databaseURL, runID string) error {
	conn, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		return errors.New("could not connect to the fixed restore target")
	}
	defer conn.Close(context.Background())
	owner := ownerRole(runID)
	verifier := verifyRole(runID)
	var db, login string
	var member, roleSafe, verifierMember, verifierSafe bool
	err = conn.QueryRow(ctx, `SELECT current_database(),session_user,
 pg_catalog.pg_has_role(session_user,$1,'MEMBER'),
 NOT r.rolcanlogin AND NOT r.rolsuper AND NOT r.rolcreatedb AND NOT r.rolcreaterole
 AND NOT r.rolreplication AND NOT r.rolbypassrls AND NOT r.rolinherit
	,pg_catalog.pg_has_role($2,$3,'MEMBER'),
 NOT v.rolcanlogin AND NOT v.rolsuper AND NOT v.rolcreatedb AND NOT v.rolcreaterole
 AND NOT v.rolreplication AND NOT v.rolbypassrls AND v.rolinherit
	FROM pg_catalog.pg_roles r,pg_catalog.pg_roles v WHERE r.rolname=$1 AND v.rolname=$3`, owner, iamSQLUsername("verify", runID), verifier).Scan(&db, &login, &member, &roleSafe, &verifierMember, &verifierSafe)
	if err != nil || db != "espacigo_scratch" || login != iamSQLUsername("restore", runID) || !member || !roleSafe || !verifierMember || !verifierSafe {
		return errors.New("target logins, owner/verifier roles, or memberships failed fixed preflight")
	}
	var existingMigrations bool
	var userRelations int
	if err := conn.QueryRow(ctx, `SELECT to_regclass('public.schema_migrations') IS NOT NULL`).Scan(&existingMigrations); err != nil || existingMigrations {
		return errors.New("restore target already contains migration history")
	}
	if err := conn.QueryRow(ctx, `SELECT count(*) FROM pg_catalog.pg_class c
 JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
 WHERE n.nspname='public' AND c.relkind IN ('r','p','v','m','S','f')
 AND NOT EXISTS (SELECT 1 FROM pg_catalog.pg_depend d WHERE d.classid='pg_catalog.pg_class'::regclass AND d.objid=c.oid AND d.deptype='e')`).Scan(&userRelations); err != nil || userRelations != 0 {
		return errors.New("restore target is not empty except for preinstalled extensions")
	}
	var extensions int
	if err := conn.QueryRow(ctx, `SELECT count(*) FROM pg_catalog.pg_extension WHERE extname IN ('btree_gist','postgis','unaccent')`).Scan(&extensions); err != nil || extensions != 3 {
		return errors.New("required Cloud SQL extensions are not preinstalled")
	}
	return nil
}

func verifyRestored(ctx context.Context, databaseURL, runID, expectedMigrationInventory string, selectOwner bool) (verification, error) {
	result := verification{counts: map[string]int{}, checks: map[string]bool{}}
	inventory, err := migrationInventory(migrationDirectory)
	if err != nil || inventory != expectedMigrationInventory {
		return result, errors.New("executor migration inventory differs from the reviewed manifest")
	}
	conn, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		return result, errors.New("could not connect for post-restore verification")
	}
	defer conn.Close(context.Background())
	owner := "g03_restore_owner_" + strings.ReplaceAll(runID, "-", "_")
	if selectOwner {
		if _, err := conn.Exec(ctx, "SET ROLE "+pgx.Identifier{owner}.Sanitize()); err != nil {
			return result, errors.New("restore owner role could not be selected for verification")
		}
	}
	if err := dbbootstrap.VerifyScratchRuntimePermissions(ctx, conn); err != nil {
		return result, errors.New("restored runtime permission contract failed")
	}
	if err := migrator.VerifyAllApplied(ctx, conn, migrationDirectory, migrationCount); err != nil {
		return result, errors.New("restored V1-V61 checksum verification failed")
	}
	var users, spaces, occupancy, adjacency int
	if err := conn.QueryRow(ctx, `SELECT (SELECT count(*) FROM public.usuario),
 (SELECT count(*) FROM public.espacio),(SELECT count(*) FROM public.ocupacion),
 (SELECT count(*) FROM public.ocupacion WHERE espacio_id='dddddddd-dddd-4ddd-8ddd-dddddddddddd' AND activo AND tipo='bloqueo_manual')`).Scan(&users, &spaces, &occupancy, &adjacency); err != nil || users != 2 || spaces != 1 || occupancy != 2 || adjacency != 2 {
		return result, errors.New("restored synthetic record counts or adjacent calendar slots differ")
	}
	result.counts = map[string]int{"usuario": users, "espacio": spaces, "ocupacion": occupancy}
	result.checks["migration_checksums_v1_v61"] = true
	result.checks["adjacent_calendar_slots"] = true
	var postgis, validSpatial, dst23 bool
	if err := conn.QueryRow(ctx, `SELECT postgis_full_version() <> '',
 public.ST_IsValid(public.ST_SetSRID(public.ST_MakePoint(-70.6,-33.4),4326)),
 extract(epoch FROM ((timestamp '2026-09-07 00:00' AT TIME ZONE 'America/Santiago')-(timestamp '2026-09-06 00:00' AT TIME ZONE 'America/Santiago')))=23*3600`).Scan(&postgis, &validSpatial, &dst23); err != nil || !postgis || !validSpatial || !dst23 {
		return result, errors.New("PostGIS, spatial function, or Santiago DST verification failed")
	}
	result.checks["postgis_spatial_function"] = true
	result.checks["santiago_dst_23_hour_day"] = true
	var tableOwner, functionOwner string
	var securityDefiner, safeSearchPath, triggerPresent bool
	if err := conn.QueryRow(ctx, `SELECT pg_catalog.pg_get_userbyid(c.relowner),
 (SELECT pg_catalog.pg_get_userbyid(p.proowner) FROM pg_catalog.pg_proc p
  WHERE p.oid='public.purge_expired_local_communication(timestamptz,integer)'::regprocedure),
 (SELECT p.prosecdef FROM pg_catalog.pg_proc p WHERE p.oid='public.purge_expired_local_communication(timestamptz,integer)'::regprocedure),
 (SELECT replace(coalesce(array_to_string(p.proconfig,','),''),' ','') LIKE '%search_path=pg_catalog,public%' FROM pg_catalog.pg_proc p WHERE p.oid='public.purge_expired_local_communication(timestamptz,integer)'::regprocedure),
 EXISTS(SELECT 1 FROM pg_catalog.pg_trigger WHERE tgname='usuario_gadm_bootstrap_identity_immutable' AND NOT tgisinternal)
 FROM pg_catalog.pg_class c WHERE c.oid='public.ocupacion'::regclass`).Scan(&tableOwner, &functionOwner, &securityDefiner, &safeSearchPath, &triggerPresent); err != nil || tableOwner != owner || functionOwner != owner || !securityDefiner || !safeSearchPath || !triggerPresent {
		return result, errors.New("restored object ownership, SECURITY DEFINER policy, or V60 trigger verification failed")
	}
	result.checks["object_ownership"] = true
	result.checks["security_definer_search_path"] = true
	result.checks["v60_trigger"] = true
	_, err = conn.Exec(ctx, `INSERT INTO public.ocupacion(id,espacio_id,reserva_id,intervalo,tipo,activo,motivo)
 VALUES('eeeeeeee-eeee-4eee-8eee-eeeeeeeeee03','dddddddd-dddd-4ddd-8ddd-dddddddddddd',NULL,
 tstzrange('2026-09-06 04:30:00+00','2026-09-06 05:30:00+00','[)'),'bloqueo_manual',true,'overlap verification')`)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23P01" {
		return result, errors.New("restored overlap constraint did not reject conflicting occupancy")
	}
	verificationRole := ""
	if selectOwner {
		verificationRole = owner
	}
	if err := verifyConcurrentOverlap(ctx, databaseURL, verificationRole); err != nil {
		return result, err
	}
	result.checks["overlap_rejected"] = true
	result.checks["concurrent_overlap_one_winner"] = true
	return result, nil
}

func verifyConcurrentOverlap(ctx context.Context, databaseURL, role string) (result error) {
	ids := []string{"eeeeeeee-eeee-4eee-8eee-eeeeeeeeee04", "eeeeeeee-eeee-4eee-8eee-eeeeeeeeee05"}
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		cleanup, err := pgx.Connect(cleanupCtx, databaseURL)
		if err != nil {
			if result == nil {
				result = errors.New("could not clean up fixed synthetic concurrency rows")
			}
			return
		}
		defer cleanup.Close(context.Background())
		if role != "" {
			_, err = cleanup.Exec(cleanupCtx, "SET ROLE "+pgx.Identifier{role}.Sanitize())
		}
		if err == nil {
			_, err = cleanup.Exec(cleanupCtx, `DELETE FROM public.ocupacion
 WHERE id IN ('eeeeeeee-eeee-4eee-8eee-eeeeeeeeee04','eeeeeeee-eeee-4eee-8eee-eeeeeeeeee05')`)
		}
		if err != nil && result == nil {
			result = errors.New("could not remove fixed synthetic concurrency rows")
		}
	}()
	gate := make(chan struct{})
	errs := make([]error, 2)
	var wg sync.WaitGroup
	for i := range ids {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			conn, err := pgx.Connect(ctx, databaseURL)
			if err != nil {
				errs[i] = errors.New("could not open concurrent verification connection")
				return
			}
			defer conn.Close(context.Background())
			if role != "" {
				if _, err := conn.Exec(ctx, "SET ROLE "+pgx.Identifier{role}.Sanitize()); err != nil {
					errs[i] = errors.New("could not select restore owner for concurrency check")
					return
				}
			}
			<-gate
			for attempt := 0; attempt < 4; attempt++ {
				_, err = conn.Exec(ctx, `INSERT INTO public.ocupacion(id,espacio_id,reserva_id,intervalo,tipo,activo,motivo)
 VALUES($1,'dddddddd-dddd-4ddd-8ddd-dddddddddddd',NULL,tstzrange('2026-10-01 12:00:00+00','2026-10-01 13:00:00+00','[)'), 'bloqueo_manual',true,'concurrent restore verification')`, ids[i])
				var pgErr *pgconn.PgError
				if errors.As(err, &pgErr) && pgErr.Code == "40P01" && attempt < 3 {
					continue
				}
				errs[i] = err
				return
			}
			errs[i] = errors.New("deadlock retry bound was exhausted")
		}(i)
	}
	close(gate)
	wg.Wait()
	wins, conflicts := 0, 0
	for _, err := range errs {
		if err == nil {
			wins++
			continue
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23P01" {
			conflicts++
			continue
		}
		return errors.New("concurrent calendar check returned an unexpected result")
	}
	if wins != 1 || conflicts != 1 {
		return errors.New("concurrent calendar check did not yield one winner and one overlap rejection")
	}
	return nil
}

func migrationInventory(dir string) (string, error) {
	files, err := filepath.Glob(filepath.Join(dir, "V*.sql"))
	if err != nil || len(files) != migrationCount {
		return "", errors.New("migration directory does not contain V1-V61")
	}
	h := sha256.New()
	for i, path := range files {
		if !strings.HasPrefix(filepath.Base(path), fmt.Sprintf("V%06d__", i+1)) {
			return "", errors.New("migration order is invalid")
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return "", errors.New("migration bytes could not be read")
		}
		sum := sha256.Sum256(data)
		_, _ = fmt.Fprintf(h, "%s %x\n", filepath.Base(path), sum)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func download(ctx context.Context, client *storage.Client, bucket, object string, max int64) ([]byte, error) {
	r, err := client.Bucket(bucket).Object(object).NewReader(ctx)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	data, err := io.ReadAll(io.LimitReader(r, max+1))
	if err != nil || int64(len(data)) > max {
		return nil, errors.New("artifact exceeds size limit or cannot be read")
	}
	return data, nil
}

func writePrivate(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	_, writeErr := f.Write(data)
	closeErr := f.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}

func waitProxy(ctx context.Context, address string) error {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		conn, err := net.DialTimeout("tcp", address, time.Second)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
