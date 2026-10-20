package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/database/cloudsql"
)

const (
	bundleDir       = "/app/baseline"
	databaseName    = "espacigo"
	databaseUser    = "espacigo_migrator_login"
	maxPasswordSize = 4096
)

var digestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

type manifest struct {
	Format                            string `json:"format"`
	ArchiveSHA256                     string `json:"archive_sha256"`
	TOCSHA256                         string `json:"toc_sha256"`
	TOCOriginalSHA256                 string `json:"toc_original_sha256"`
	ACLRestoreSQLSHA256               string `json:"acl_restore_sql_sha256"`
	RoleDefinitionsAndMembershipsDump bool   `json:"role_definitions_and_memberships_in_dump"`
	Migrations                        struct {
		Count                      int            `json:"count"`
		First                      int            `json:"first"`
		Last                       int            `json:"last"`
		SourceTargetChecksumsEqual bool           `json:"source_target_checksums_equal"`
		SourceLedger               []migrationRow `json:"source_ledger"`
	} `json:"migrations"`
}

type migrationRow struct {
	Version  int    `json:"version"`
	Name     string `json:"name"`
	Checksum string `json:"checksum"`
}

type config struct {
	mode, databaseURL, passwordFile string
	expectedManifest, expectedDump  string
	queryTimeout                    time.Duration
}

type report struct {
	Format         string            `json:"format"`
	Phase          string            `json:"phase"`
	Status         string            `json:"status"`
	Project        string            `json:"project"`
	Instance       string            `json:"instance_connection_name"`
	Database       string            `json:"database"`
	DatabaseUser   string            `json:"database_user"`
	Target         map[string]string `json:"target,omitempty"`
	ManifestSHA256 string            `json:"manifest_sha256"`
	DumpSHA256     string            `json:"dump_sha256"`
	Checks         map[string]bool   `json:"checks,omitempty"`
	SQLState       string            `json:"sqlstate,omitempty"`
	CommitOutcome  string            `json:"commit_outcome,omitempty"`
	ErrorCode      string            `json:"error_code,omitempty"`
}

func main() {
	os.Exit(run())
}

func run() int {
	modeFlag := flag.String("mode", "", "restore")
	flag.Parse()
	cfg := config{
		mode:             *modeFlag,
		databaseURL:      os.Getenv("DATABASE_URL"),
		passwordFile:     os.Getenv("DATABASE_PASSWORD_FILE"),
		expectedManifest: os.Getenv("GCP04_BASELINE_MANIFEST_SHA256"),
		expectedDump:     os.Getenv("GCP04_BASELINE_DUMP_SHA256"),
		queryTimeout:     25 * time.Minute,
	}
	ctx, cancel := context.WithTimeout(context.Background(), cfg.queryTimeout)
	defer cancel()

	manifestBytes, files, err := readAndVerifyBundle(cfg)
	if err != nil {
		writeReport(report{Format: "espacigo.gcp04.baseline-restore.v1", Phase: cfg.mode, Status: "failed", ErrorCode: "ARTIFACT_VALIDATION_FAILED"})
		return 1
	}
	parsedURL, err := validateDatabaseURL(cfg.databaseURL)
	if err != nil {
		writeReport(report{Format: "espacigo.gcp04.baseline-restore.v1", Phase: cfg.mode, Status: "failed", ErrorCode: "TARGET_CONFIGURATION_FAILED"})
		return 1
	}
	password, err := readPassword(cfg.passwordFile)
	if err != nil {
		writeReport(report{Format: "espacigo.gcp04.baseline-restore.v1", Phase: cfg.mode, Status: "failed", ErrorCode: "CREDENTIAL_FILE_FAILED"})
		return 1
	}
	defer clear(password)

	passfile, cleanup, err := makePassfile(password)
	if err != nil {
		writeReport(report{Format: "espacigo.gcp04.baseline-restore.v1", Phase: cfg.mode, Status: "failed", ErrorCode: "CREDENTIAL_STAGING_FAILED"})
		return 1
	}
	defer cleanup()

	manifestValue := new(manifest)
	if err := json.Unmarshal(manifestBytes, manifestValue); err != nil {
		writeReport(report{Format: "espacigo.gcp04.baseline-restore.v1", Phase: cfg.mode, Status: "failed", ErrorCode: "ARTIFACT_VALIDATION_FAILED"})
		return 1
	}

	if err := verifyArchiveTOC(ctx, files); err != nil {
		writeReport(report{Format: "espacigo.gcp04.baseline-restore.v1", Phase: cfg.mode, Status: "failed", ErrorCode: "TOC_VALIDATION_FAILED"})
		return 1
	}

	script, err := buildScript(*manifestValue, cfg.mode, files)
	if err != nil {
		writeReport(report{Format: "espacigo.gcp04.baseline-restore.v1", Phase: cfg.mode, Status: "failed", ErrorCode: "PREFLIGHT_SCRIPT_FAILED"})
		return 1
	}
	if cfg.mode == "restore" {
		if err := generateRestoreSQL(ctx, files, *manifestValue, &script); err != nil {
			writeReport(report{Format: "espacigo.gcp04.baseline-restore.v1", Phase: cfg.mode, Status: "failed", ErrorCode: "RESTORE_SQL_GENERATION_FAILED", CommitOutcome: "not_started"})
			return 1
		}
	}

	tmpDir, err := os.MkdirTemp("/tmp", "gcp04-baseline-")
	if err != nil {
		writeReport(report{Format: "espacigo.gcp04.baseline-restore.v1", Phase: cfg.mode, Status: "failed", ErrorCode: "TEMPORARY_WORKSPACE_FAILED", CommitOutcome: "not_started"})
		return 1
	}
	defer os.RemoveAll(tmpDir)
	scriptPath := filepath.Join(tmpDir, "operation.sql")
	if err := os.WriteFile(scriptPath, script, 0600); err != nil {
		writeReport(report{Format: "espacigo.gcp04.baseline-restore.v1", Phase: cfg.mode, Status: "failed", ErrorCode: "TEMPORARY_WORKSPACE_FAILED", CommitOutcome: "not_started"})
		return 1
	}

	forward, err := startLocalForward(ctx, cfg.databaseURL, password, os.Getenv("CLOUD_SQL_INSTANCE_CONNECTION_NAME"))
	if err != nil {
		writeReport(report{Format: "espacigo.gcp04.baseline-restore.v1", Phase: cfg.mode, Status: "failed", ErrorCode: "CLOUDSQL_FORWARD_INIT_FAILED", CommitOutcome: "not_started"})
		return 1
	}
	defer forward.close()

	cmd := exec.CommandContext(ctx, "psql", "-X", "-q", "-A", "-t", "-v", "ON_ERROR_STOP=1", "-v", "VERBOSITY=verbose", "--dbname="+parsedURL.String(), "--file="+scriptPath)
	cmd.Env = sanitizedPostgresEnvironment(os.Environ(), passfile)
	var stdout bytes.Buffer
	var stderr boundedBuffer
	stderr.limit = 64 << 10
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		outcome := "not_started"
		if cfg.mode == "restore" {
			outcome = "unknown"
		}
		code, state := classifyPSQLError(stderr.String())
		writeReport(report{Format: "espacigo.gcp04.baseline-restore.v1", Phase: cfg.mode, Status: "failed", ErrorCode: code, SQLState: state, CommitOutcome: outcome})
		return 1
	}

	operationReport, err := parseSQLReport(stdout.Bytes())
	if err != nil {
		outcome := "not_started"
		if cfg.mode == "restore" {
			outcome = "unknown"
		}
		writeReport(report{Format: "espacigo.gcp04.baseline-restore.v1", Phase: cfg.mode, Status: "failed", ErrorCode: "DATABASE_REPORT_FAILED", CommitOutcome: outcome})
		return 1
	}
	operationReport.ManifestSHA256 = hexDigest(manifestBytes)
	operationReport.DumpSHA256 = hexDigest(files["baseline.dump"])
	if err := json.NewEncoder(os.Stdout).Encode(operationReport); err != nil {
		return 1
	}
	return 0
}

type bundleFiles map[string][]byte

func readAndVerifyBundle(cfg config) ([]byte, bundleFiles, error) {
	return readAndVerifyBundleAt(cfg, bundleDir)
}

func readAndVerifyBundleAt(cfg config, directory string) ([]byte, bundleFiles, error) {
	if cfg.mode != "restore" || !digestPattern.MatchString(cfg.expectedManifest) || !digestPattern.MatchString(cfg.expectedDump) {
		return nil, nil, errors.New("invalid fixed operation inputs")
	}
	read := func(name string) ([]byte, error) {
		path := filepath.Join(directory, name)
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0022 != 0 {
			return nil, errors.New("bundle file is not a protected regular file")
		}
		return os.ReadFile(path)
	}
	manifestBytes, err := read("baseline.manifest.json")
	if err != nil || hexDigest(manifestBytes) != cfg.expectedManifest {
		return nil, nil, errors.New("manifest digest does not match its reviewed pin")
	}
	var value manifest
	if err := json.Unmarshal(manifestBytes, &value); err != nil {
		return nil, nil, errors.New("manifest is invalid")
	}
	if value.Format != "espacigo-gcp04-w4-baseline-package-v2" || value.RoleDefinitionsAndMembershipsDump || value.Migrations.Count != 64 || value.Migrations.First != 1 || value.Migrations.Last != 64 || !value.Migrations.SourceTargetChecksumsEqual || len(value.Migrations.SourceLedger) != 64 {
		return nil, nil, errors.New("manifest does not describe the reviewed canonical baseline")
	}
	files := bundleFiles{}
	for _, name := range []string{"baseline.dump", "baseline.toc.original", "baseline.toc.restore", "baseline.acl.restore.sql"} {
		files[name], err = read(name)
		if err != nil {
			return nil, nil, err
		}
	}
	if hexDigest(files["baseline.dump"]) != cfg.expectedDump || hexDigest(files["baseline.dump"]) != value.ArchiveSHA256 || hexDigest(files["baseline.toc.restore"]) != value.TOCSHA256 || hexDigest(files["baseline.toc.original"]) != value.TOCOriginalSHA256 || hexDigest(files["baseline.acl.restore.sql"]) != value.ACLRestoreSQLSHA256 {
		return nil, nil, errors.New("bundle payload hashes do not match the manifest")
	}
	return manifestBytes, files, nil
}

func validateDatabaseURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "postgres" || u.User == nil || u.User.Username() != databaseUser || u.Hostname() != "localhost" || u.Port() != "5432" || u.Path != "/"+databaseName || u.RawQuery != "sslmode=disable" {
		return nil, errors.New("database URL does not identify the fixed local proxy target")
	}
	if _, present := u.User.Password(); present {
		return nil, errors.New("inline database passwords are forbidden")
	}
	return u, nil
}

func readPassword(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0022 != 0 || info.Size() == 0 || info.Size() > maxPasswordSize {
		return nil, errors.New("mounted credential is not a protected regular file")
	}
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 || len(data) > maxPasswordSize {
		return nil, errors.New("mounted credential could not be read")
	}
	password := strings.TrimSuffix(strings.TrimSuffix(string(data), "\n"), "\r")
	if password == "" || strings.ContainsAny(password, "\x00\r\n") {
		return nil, errors.New("mounted credential has an invalid shape")
	}
	return []byte(password), nil
}

func makePassfile(password []byte) (string, func(), error) {
	dir, err := os.MkdirTemp("/tmp", "gcp04-pass-")
	if err != nil {
		return "", func() {}, err
	}
	if err := os.Chmod(dir, 0700); err != nil {
		_ = os.RemoveAll(dir)
		return "", func() {}, err
	}
	path := filepath.Join(dir, "pgpass")
	value := "localhost:5432:espacigo:espacigo_migrator_login:" + escapePGPass(string(password)) + "\n"
	if err := os.WriteFile(path, []byte(value), 0600); err != nil {
		_ = os.RemoveAll(dir)
		return "", func() {}, err
	}
	return path, func() { _ = os.RemoveAll(dir) }, nil
}

func escapePGPass(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	return strings.ReplaceAll(value, ":", `\:`)
}

func sanitizedPostgresEnvironment(source []string, passfile string) []string {
	blocked := map[string]bool{
		"PGHOST": true, "PGPORT": true, "PGDATABASE": true, "PGUSER": true, "PGPASSWORD": true,
		"PGPASSFILE": true, "PGSERVICE": true, "PGSERVICEFILE": true, "PGOPTIONS": true, "PGSSLMODE": true,
	}
	out := make([]string, 0, len(source)+1)
	for _, entry := range source {
		key, _, ok := strings.Cut(entry, "=")
		if !ok || !blocked[key] {
			out = append(out, entry)
		}
	}
	return append(out, "PGPASSFILE="+passfile)
}

type localForward struct {
	listener       net.Listener
	ctx            context.Context
	cancel         context.CancelFunc
	closeConnector func()
	dial           func(context.Context, string, string) (net.Conn, error)
	workers        chan struct{}
}

func startLocalForward(ctx context.Context, databaseURL string, password []byte, instance string) (*localForward, error) {
	if instance != "espacigo-testprod:southamerica-west1:espacigo-testprod-pg" || !cloudsql.ValidInstanceConnectionName(instance) {
		return nil, errors.New("Cloud SQL instance does not match the fixed restore target")
	}
	config, err := cloudsql.NewPGXConfig(databaseURL, string(password))
	if err != nil {
		return nil, errors.New("Cloud SQL connector configuration failed")
	}
	closeConnector, err := cloudsql.ConfigurePGX(ctx, config, instance, true)
	if err != nil {
		return nil, errors.New("Cloud SQL connector initialization failed")
	}
	listener, err := net.Listen("tcp", "localhost:5432")
	if err != nil {
		closeConnector()
		return nil, errors.New("local PostgreSQL forward listener failed")
	}
	forwardCtx, cancel := context.WithCancel(ctx)
	forward := &localForward{listener: listener, ctx: forwardCtx, cancel: cancel, closeConnector: closeConnector, dial: config.DialFunc, workers: make(chan struct{}, 4)}
	go forward.acceptLoop()
	return forward, nil
}

func (f *localForward) acceptLoop() {
	for {
		local, err := f.listener.Accept()
		if err != nil {
			return
		}
		select {
		case f.workers <- struct{}{}:
			go func() {
				defer func() { <-f.workers }()
				f.forwardConnection(local)
			}()
		default:
			_ = local.Close()
		}
	}
}

func (f *localForward) forwardConnection(local net.Conn) {
	defer local.Close()
	remote, err := f.dial(f.ctx, "tcp", "localhost:5432")
	if err != nil {
		return
	}
	defer remote.Close()
	done := make(chan struct{}, 2)
	go func() { _, _ = io.Copy(remote, local); done <- struct{}{} }()
	go func() { _, _ = io.Copy(local, remote); done <- struct{}{} }()
	<-done
	_ = local.Close()
	_ = remote.Close()
	<-done
}

func (f *localForward) close() {
	f.cancel()
	_ = f.listener.Close()
	f.closeConnector()
}

func verifyArchiveTOC(ctx context.Context, files bundleFiles) error {
	tmpDir, err := os.MkdirTemp("/tmp", "gcp04-toc-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)
	dumpPath := filepath.Join(tmpDir, "baseline.dump")
	if err := os.WriteFile(dumpPath, files["baseline.dump"], 0600); err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, "pg_restore", "--list", dumpPath)
	cmd.Env = sanitizedPostgresEnvironment(os.Environ(), "")
	actual, err := cmd.Output()
	if err != nil || !sameTOCEntries(actual, files["baseline.toc.original"]) {
		return errors.New("archive TOC entries differ from the reviewed bundle")
	}
	return nil
}

func sameTOCEntries(left, right []byte) bool {
	filter := func(value []byte) []string {
		lines := strings.Split(strings.ReplaceAll(string(value), "\r\n", "\n"), "\n")
		out := make([]string, 0, len(lines))
		for _, line := range lines {
			if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), ";") {
				continue
			}
			out = append(out, line)
		}
		return out
	}
	a, b := filter(left), filter(right)
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func buildScript(m manifest, mode string, files bundleFiles) ([]byte, error) {
	var b strings.Builder
	if mode != "restore" {
		return nil, errors.New("unsupported operation mode")
	}
	lockedGuard, err := targetGuardSQL(m)
	if err != nil {
		return nil, err
	}
	b.WriteString("BEGIN;\nSET LOCAL lock_timeout='10s';\nSET LOCAL statement_timeout='1500s';\n")
	b.WriteString(lockedGuard)
	b.WriteString("-- Restore SQL is inserted by pg_restore below.\n")
	// The complete restore body is inserted by generateRestoreSQL after this guard.
	_ = files
	return []byte(b.String()), nil
}

func targetGuardSQL(m manifest) (string, error) {
	if len(m.Migrations.SourceLedger) != 64 {
		return "", errors.New("migration ledger inventory is incomplete")
	}
	values, err := migrationValues(m.Migrations.SourceLedger[:59])
	if err != nil {
		return "", err
	}
	guard := `DO $gcp04_guard$
DECLARE owner_oid oid; target_oid oid; gadm_oid oid; count_value bigint;
BEGIN
  IF current_database()<>'espacigo' OR session_user<>'espacigo_migrator_login' OR current_user<>'espacigo_migrator_login' THEN RAISE EXCEPTION 'target_identity'; END IF;
  SELECT oid INTO owner_oid FROM pg_catalog.pg_roles WHERE rolname='espacigo_schema_owner' AND NOT rolcanlogin AND NOT rolsuper AND NOT rolcreatedb AND NOT rolcreaterole AND NOT rolreplication AND NOT rolbypassrls AND NOT rolinherit;
  SELECT oid INTO target_oid FROM pg_catalog.pg_roles WHERE rolname=session_user AND rolcanlogin AND NOT rolsuper AND NOT rolcreatedb AND NOT rolcreaterole AND NOT rolreplication AND NOT rolbypassrls;
  IF owner_oid IS NULL OR target_oid IS NULL OR NOT pg_catalog.pg_has_role(session_user,'espacigo_schema_owner','SET') THEN RAISE EXCEPTION 'target_role_contract'; END IF;
  IF (SELECT count(*) FROM pg_catalog.pg_auth_members WHERE member=target_oid OR roleid=target_oid)<>1 OR NOT EXISTS (SELECT 1 FROM pg_catalog.pg_auth_members WHERE member=target_oid AND roleid=owner_oid AND NOT admin_option AND inherit_option AND set_option) THEN RAISE EXCEPTION 'migrator_membership_contract'; END IF;
  SELECT oid INTO gadm_oid FROM pg_catalog.pg_roles WHERE rolname='espacigo_gadm_bootstrap' AND NOT rolcanlogin AND NOT rolsuper AND NOT rolcreatedb AND NOT rolcreaterole AND NOT rolreplication AND NOT rolbypassrls AND NOT rolinherit AND rolconnlimit=-1;
  IF gadm_oid IS NULL OR shobj_description(gadm_oid,'pg_authid') IS DISTINCT FROM 'NOLOGIN GADM local bootstrap group; temporary isolated login membership only.' OR EXISTS (SELECT 1 FROM pg_catalog.pg_auth_members WHERE roleid=gadm_oid OR member=gadm_oid) OR EXISTS (SELECT 1 FROM pg_catalog.pg_db_role_setting WHERE setrole=gadm_oid) THEN RAISE EXCEPTION 'gadm_role_contract'; END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_catalog.pg_namespace WHERE nspname='public' AND nspowner='pg_database_owner'::regrole) THEN RAISE EXCEPTION 'public_schema_owner'; END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relname='schema_migrations' AND c.relkind='r' AND c.relowner=owner_oid) OR NOT EXISTS (SELECT 1 FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relname='usuario' AND c.relkind='r' AND c.relowner=owner_oid) THEN RAISE EXCEPTION 'baseline_tables_missing_or_wrong_owner'; END IF;
END
$gcp04_guard$;
`
	guard += "SET ROLE espacigo_schema_owner;\nLOCK TABLE public.schema_migrations IN ACCESS EXCLUSIVE MODE;\nLOCK TABLE public.usuario IN ACCESS EXCLUSIVE MODE;\n"
	guard += fmt.Sprintf(`DO $gcp04_data_guard$
DECLARE count_value bigint;
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relname='schema_migrations' AND c.relkind='r' AND c.relowner='espacigo_schema_owner'::regrole) OR NOT EXISTS (SELECT 1 FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relname='usuario' AND c.relkind='r' AND c.relowner='espacigo_schema_owner'::regrole) THEN RAISE EXCEPTION 'baseline_tables_missing_or_wrong_owner'; END IF;
  SELECT count(*) INTO count_value FROM public.schema_migrations;
  IF count_value<>59 OR (SELECT min(version) FROM public.schema_migrations)<>1 OR (SELECT max(version) FROM public.schema_migrations)<>59 THEN RAISE EXCEPTION 'migration_history_not_v1_v59'; END IF;
  IF EXISTS (SELECT 1 FROM (VALUES %s) AS expected(version,name,checksum) FULL JOIN public.schema_migrations actual USING(version) WHERE expected.version IS NULL OR actual.version IS NULL OR expected.name IS DISTINCT FROM actual.name OR expected.checksum IS DISTINCT FROM actual.checksum OR actual.version>59) THEN RAISE EXCEPTION 'migration_history_checksum_drift'; END IF;
  IF (SELECT count(*) FROM public.usuario)<>0 THEN RAISE EXCEPTION 'usuario_rows_present'; END IF;
END
$gcp04_data_guard$;
`, values)
	return guard, nil
}

func generateRestoreSQL(ctx context.Context, files bundleFiles, m manifest, script *[]byte) error {
	tmpDir, err := os.MkdirTemp("/tmp", "gcp04-restore-sql-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)
	paths := map[string]string{}
	for _, name := range []string{"baseline.dump", "baseline.toc.restore"} {
		path := filepath.Join(tmpDir, name)
		if err := os.WriteFile(path, files[name], 0600); err != nil {
			return err
		}
		paths[name] = path
	}
	cmd := exec.CommandContext(ctx, "pg_restore", "--exit-on-error", "--clean", "--if-exists", "--no-owner", "--no-acl", "--use-list="+paths["baseline.toc.restore"], "--file=-", paths["baseline.dump"])
	cmd.Env = sanitizedPostgresEnvironment(os.Environ(), "")
	restoreBody, err := cmd.Output()
	if err != nil {
		return errors.New("pg_restore could not generate the fixed restore script")
	}
	var combined bytes.Buffer
	combined.Write(*script)
	combined.Write(restoreBody)
	combined.WriteString("\n")
	combined.Write(files["baseline.acl.restore.sql"])
	combined.WriteString("\nDO $gcp04_postcheck$ BEGIN\n")
	combined.WriteString("IF (SELECT count(*) FROM public.schema_migrations)<>64 OR (SELECT min(version) FROM public.schema_migrations)<>1 OR (SELECT max(version) FROM public.schema_migrations)<>64 THEN RAISE EXCEPTION 'restored_migration_history_invalid'; END IF;\n")
	allRows, err := migrationValues(m.Migrations.SourceLedger)
	if err != nil {
		return err
	}
	combined.WriteString("IF EXISTS (SELECT 1 FROM (VALUES " + allRows + ") AS expected(version,name,checksum) FULL JOIN public.schema_migrations actual USING(version) WHERE expected.version IS NULL OR actual.version IS NULL OR expected.name IS DISTINCT FROM actual.name OR expected.checksum IS DISTINCT FROM actual.checksum) THEN RAISE EXCEPTION 'restored_migration_checksum_invalid'; END IF;\n")
	combined.WriteString("IF EXISTS (SELECT 1 FROM pg_catalog.pg_database d CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(d.datacl,pg_catalog.acldefault('d',d.datdba))) a WHERE d.datname='espacigo' AND a.grantee=0 AND a.privilege_type IN ('CREATE','TEMPORARY')) THEN RAISE EXCEPTION 'restored_public_database_acl_invalid'; END IF;\n")
	combined.WriteString("IF EXISTS (SELECT 1 FROM pg_catalog.pg_namespace n CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(n.nspacl,pg_catalog.acldefault('n',n.nspowner))) a WHERE n.nspname='public' AND a.grantee=0 AND a.privilege_type='CREATE') OR NOT EXISTS (SELECT 1 FROM pg_catalog.pg_namespace n CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(n.nspacl,pg_catalog.acldefault('n',n.nspowner))) a WHERE n.nspname='public' AND a.grantee=0 AND a.privilege_type='USAGE') THEN RAISE EXCEPTION 'restored_public_schema_acl_invalid'; END IF;\n")
	combined.WriteString("IF (SELECT count(*) FROM public.usuario)<>0 THEN RAISE EXCEPTION 'restored_usuario_rows_present'; END IF;\n")
	combined.WriteString("IF NOT EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname='espacigo_gadm_bootstrap' AND NOT rolcanlogin AND NOT rolsuper AND NOT rolcreatedb AND NOT rolcreaterole AND NOT rolreplication AND NOT rolbypassrls AND NOT rolinherit AND rolconnlimit=-1 AND shobj_description(oid,'pg_authid')='NOLOGIN GADM local bootstrap group; temporary isolated login membership only.') OR EXISTS (SELECT 1 FROM pg_catalog.pg_auth_members m JOIN pg_catalog.pg_roles r ON r.oid=m.roleid JOIN pg_catalog.pg_roles u ON u.oid=m.member WHERE r.rolname='espacigo_gadm_bootstrap' OR u.rolname='espacigo_gadm_bootstrap') THEN RAISE EXCEPTION 'gadm_role_changed'; END IF;\n")
	combined.WriteString("END $gcp04_postcheck$;\nCOMMIT;\n")
	combined.WriteString("SELECT json_build_object('format','espacigo.gcp04.baseline-restore.v1','phase','restore','status','passed','target',json_build_object('database','espacigo','database_user','espacigo_migrator_login'),'migration_history','V1-V64_verified','gadm_role','preserved')::text;\n")
	*script = combined.Bytes()
	return nil
}

func parseSQLReport(output []byte) (report, error) {
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) == 0 || lines[len(lines)-1] == "" {
		return report{}, errors.New("database operation did not emit a report")
	}
	var value report
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &value); err != nil || value.Status != "passed" {
		return report{}, errors.New("database report is invalid")
	}
	return value, nil
}

func migrationValues(rows []migrationRow) (string, error) {
	if len(rows) != 59 && len(rows) != 64 {
		return "", errors.New("canonical migration ledger is incomplete")
	}
	values := make([]string, 0, len(rows))
	for i, entry := range rows {
		if entry.Version != i+1 || !regexp.MustCompile(`^V[0-9]{6}__[a-z0-9_]+\.sql$`).MatchString(entry.Name) || !digestPattern.MatchString(entry.Checksum) {
			return "", errors.New("canonical migration ledger entry is invalid")
		}
		values = append(values, fmt.Sprintf("(%d,'%s','%s')", entry.Version, entry.Name, entry.Checksum))
	}
	return strings.Join(values, ","), nil
}

func writeReport(value report) {
	_ = json.NewEncoder(os.Stdout).Encode(value)
}

type boundedBuffer struct {
	buffer bytes.Buffer
	limit  int
}

func (b *boundedBuffer) Write(value []byte) (int, error) {
	remaining := b.limit - b.buffer.Len()
	if remaining > 0 {
		if len(value) > remaining {
			_, _ = b.buffer.Write(value[:remaining])
		} else {
			_, _ = b.buffer.Write(value)
		}
	}
	return len(value), nil
}

func (b *boundedBuffer) String() string { return b.buffer.String() }

var sqlStatePattern = regexp.MustCompile(`(?im)(?:ERROR:\s+|SQLSTATE\s*[:=]?\s*)([0-9A-Z]{5})(?:\s|:)`)

func classifyPSQLError(stderr string) (string, string) {
	guardCodes := map[string]string{
		"target_identity":                        "TARGET_IDENTITY_MISMATCH",
		"target_role_contract":                   "TARGET_ROLE_CONTRACT_FAILED",
		"migrator_membership_contract":           "MIGRATOR_MEMBERSHIP_CONTRACT_FAILED",
		"gadm_role_contract":                     "GADM_ROLE_CONTRACT_FAILED",
		"public_schema_owner":                    "PUBLIC_SCHEMA_OWNER_MISMATCH",
		"baseline_tables_missing_or_wrong_owner": "BASELINE_TABLE_CONTRACT_FAILED",
		"migration_history_not_v1_v59":           "MIGRATION_HISTORY_NOT_V1_V59",
		"migration_history_checksum_drift":       "MIGRATION_HISTORY_CHECKSUM_DRIFT",
		"usuario_rows_present":                   "USUARIO_ROWS_PRESENT",
		"restored_migration_history_invalid":     "RESTORE_MIGRATION_HISTORY_INVALID",
		"restored_migration_checksum_invalid":    "RESTORE_MIGRATION_CHECKSUM_INVALID",
		"restored_public_database_acl_invalid":   "RESTORE_PUBLIC_DATABASE_ACL_INVALID",
		"restored_public_schema_acl_invalid":     "RESTORE_PUBLIC_SCHEMA_ACL_INVALID",
		"restored_usuario_rows_present":          "RESTORE_USUARIO_ROWS_PRESENT",
		"gadm_role_changed":                      "RESTORE_GADM_ROLE_CHANGED",
	}
	code := "DATABASE_OPERATION_FAILED"
	for marker, candidate := range guardCodes {
		if strings.Contains(stderr, marker) {
			code = candidate
			break
		}
	}
	state := ""
	if match := sqlStatePattern.FindStringSubmatch(stderr); len(match) == 2 {
		state = match[1]
	}
	return code, state
}

func hexDigest(value []byte) string {
	digest := sha256.Sum256(value)
	return hex.EncodeToString(digest[:])
}
