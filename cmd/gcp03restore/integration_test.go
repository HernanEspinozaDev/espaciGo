package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/gcp03restore"
	"github.com/HernanEspinozaDev/espaciGo/internal/migrator"
	"github.com/jackc/pgx/v5"
)

func TestGCP03PhasesPostgres18Disposable(t *testing.T) {
	adminURL := os.Getenv("GCP03_EXECUTOR_TEST_DATABASE_URL")
	fixturePath := os.Getenv("GCP03_EXECUTOR_FIXTURE_PATH")
	if adminURL == "" || fixturePath == "" {
		t.Skip("run scripts/test-gcp03-restore-executor-postgres.sh for the disposable PostgreSQL 18/PostGIS phase integration")
	}
	parsed, err := url.Parse(adminURL)
	if err != nil || parsed.Scheme != "postgres" || parsed.User == nil || parsed.User.Username() != "localcontrol" || parsed.Hostname() != "localhost" || parsed.Path != "/postgres" {
		t.Fatal("integration accepts only the local Unix-socket harness URL")
	}
	if _, hasPassword := parsed.User.Password(); hasPassword || parsed.Port() != "" {
		t.Fatal("integration refuses password-bearing or TCP harness URLs")
	}
	socket := parsed.Query().Get("host")
	if !filepath.IsAbs(socket) || filepath.Base(socket) != "socket" || !strings.HasPrefix(filepath.Base(filepath.Dir(socket)), "espacigo-gcp03-restore-executor.") {
		t.Fatal("integration requires the network-isolated temporary socket directory")
	}
	if !filepath.IsAbs(fixturePath) || os.IsPathSeparator(filepath.Separator) && !strings.Contains(fixturePath, string(filepath.Separator)+".local"+string(filepath.Separator)+"testing"+string(filepath.Separator)) {
		t.Fatal("integration fixture must be an absolute local .local/testing path")
	}
	_, sourceFile, _, _ := runtime.Caller(0)
	root := filepath.Clean(filepath.Join(filepath.Dir(sourceFile), "..", ".."))
	migrationDirectory = filepath.Join(root, "db", "migrations")
	t.Cleanup(func() { migrationDirectory = "/app/db/migrations" })

	ctx, cancel := context.WithTimeout(context.Background(), 9*time.Minute)
	defer cancel()
	admin, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		t.Fatalf("connect to isolated PostgreSQL harness: %v", err)
	}
	defer admin.Close(context.Background())
	var major int
	if err := admin.QueryRow(ctx, "SELECT current_setting('server_version_num')::int / 10000").Scan(&major); err != nil || major != 18 {
		t.Fatalf("expected PostgreSQL 18, got major=%d", major)
	}
	for _, binary := range []string{"pg_dump", "pg_restore"} {
		if _, err := exec.LookPath(binary); err != nil {
			t.Fatalf("%s required for the isolated restore integration", binary)
		}
	}

	runID := "local01"
	sourceName := "g03_source_" + runID
	targetName := "espacigo_scratch"
	owner, verifier := ownerRole(runID), verifyRole(runID)
	restoreLogin, verifyLogin := iamSQLUsername("restore", runID), iamSQLUsername("verify", runID)
	extraRole := "g03_retire_extra_" + runID
	controlRole := "g03_control_" + runID
	auditLogin := "g03_audit_" + runID
	fixture, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatal("read generated synthetic fixture")
	}
	targetExists := false
	for _, dbName := range []string{sourceName, targetName} {
		createDatabase := "CREATE DATABASE " + pgx.Identifier{dbName}.Sanitize()
		if dbName == targetName {
			createDatabase += " TEMPLATE template_postgis OWNER postgres"
		}
		if _, err := admin.Exec(ctx, createDatabase); err != nil {
			t.Fatalf("create disposable database %s", dbName)
		}
		if dbName == targetName {
			targetExists = true
		}
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cleanupCancel()
		cleanupAdmin, connectErr := pgx.Connect(cleanupCtx, localDatabaseURL(parsed, "postgres", controlRole))
		if connectErr == nil {
			defer cleanupAdmin.Close(context.Background())
			for _, dbName := range []string{targetName, sourceName} {
				_, _ = cleanupAdmin.Exec(cleanupCtx, "DROP DATABASE IF EXISTS "+pgx.Identifier{dbName}.Sanitize()+" WITH (FORCE)")
			}
			for _, role := range []string{extraRole, restoreLogin, verifyLogin, auditLogin, owner, verifier, controlRole} {
				_, _ = cleanupAdmin.Exec(cleanupCtx, "DROP ROLE IF EXISTS "+pgx.Identifier{role}.Sanitize())
			}
		}
	})
	if !targetExists {
		t.Fatal("target database setup failed")
	}

	sourceURL := localDatabaseURL(parsed, sourceName, "localcontrol")
	targetAdminURL := localDatabaseURL(parsed, targetName, "postgres")
	adminPasswordBytes := make([]byte, 32)
	if _, err := rand.Read(adminPasswordBytes); err != nil {
		t.Fatal("generate ephemeral admin password")
	}
	adminPassword := base64.RawURLEncoding.EncodeToString(adminPasswordBytes)
	if _, err := admin.Exec(ctx, "CREATE ROLE "+pgx.Identifier{controlRole}.Sanitize()+" LOGIN SUPERUSER PASSWORD '"+adminPassword+"'"); err != nil {
		t.Fatal("create isolated harness control role")
	}
	controlURL := localDatabaseURL(parsed, targetName, controlRole)
	if err := bootstrap(ctx, targetAdminURL, adminPassword, runID); err != nil {
		t.Fatalf("bootstrap phase failed: %v", err)
	}
	// PostgreSQL 18 gives a CREATEROLE actor ADMIN-only memberships in new
	// roles. Remove those harness-created memberships through the separate local
	// control role before the exact V60 role-membership migration is exercised.
	control, err := pgx.Connect(ctx, localDatabaseURL(parsed, targetName, "localcontrol"))
	if err != nil {
		t.Fatal("connect as isolated harness control role for creator-membership cleanup")
	}
	for _, role := range []string{"espacigo_runtime", "espacigo_gadm_bootstrap", "espacigo_scratch_migrator", owner, verifier} {
		if _, err := control.Exec(ctx, "REVOKE "+pgx.Identifier{role}.Sanitize()+" FROM postgres"); err != nil {
			_ = control.Close(context.Background())
			t.Fatalf("remove PostgreSQL-created admin membership for fixed group role %s", role)
		}
	}
	_ = control.Close(context.Background())
	if err := bootstrap(ctx, targetAdminURL, adminPassword, runID); err == nil {
		t.Fatal("bootstrap retry accepted pre-existing run roles")
	}
	source, err := pgx.Connect(ctx, sourceURL)
	if err != nil {
		t.Fatal("connect to disposable fixture source")
	}
	for _, ext := range []string{"btree_gist", "postgis", "unaccent"} {
		if _, err := source.Exec(ctx, "CREATE EXTENSION "+pgx.Identifier{ext}.Sanitize()); err != nil {
			_ = source.Close(context.Background())
			t.Fatalf("install source extension %s", ext)
		}
	}
	if _, err := migrator.Run(ctx, sourceURL, migrationDirectory); err != nil {
		_ = source.Close(context.Background())
		t.Fatalf("apply exact V1-V61 to disposable synthetic source: %v", err)
	}
	if _, err := source.Exec(ctx, string(fixture)); err != nil {
		_ = source.Close(context.Background())
		t.Fatal("seed hand-authored synthetic fixture")
	}
	if err := source.Close(ctx); err != nil {
		t.Fatal("close disposable fixture source")
	}

	dumpPath := filepath.Join(t.TempDir(), "synthetic.dump")
	dumpCmd := exec.CommandContext(ctx, "pg_dump", "--format=custom", "--no-owner", "--no-acl", "--file", dumpPath, "--dbname", sourceURL)
	dumpCmd.Stdout, dumpCmd.Stderr = nil, nil
	if err := dumpCmd.Run(); err != nil {
		t.Fatal("create custom-format synthetic fixture dump")
	}
	dump, err := os.ReadFile(dumpPath)
	if err != nil {
		t.Fatal("read generated synthetic dump")
	}
	rawTOC, err := exec.CommandContext(ctx, "pg_restore", "--list", dumpPath).Output()
	if err != nil {
		t.Fatal("read dump TOC")
	}
	allowLines := make([]string, 0, 800)
	for _, line := range strings.Split(string(rawTOC), "\n") {
		if strings.HasPrefix(line, ";") || strings.TrimSpace(line) == "" || strings.Contains(line, " EXTENSION - ") || strings.Contains(line, " COMMENT - EXTENSION ") || strings.Contains(line, " TABLE DATA public spatial_ref_sys ") {
			continue
		}
		allowLines = append(allowLines, line)
	}
	allowlist := []byte(strings.Join(allowLines, "\n") + "\n")
	if err := validateArchiveTOC(ctx, dump, rawTOC, allowlist); err != nil {
		t.Fatalf("read-only generated TOC diagnostic rejected the exact local archive: %v", err)
	}
	migrationSHA, err := migrationInventory(migrationDirectory)
	if err != nil {
		t.Fatal("calculate local V1-V61 inventory")
	}
	dumpHash, tocHash, allowHash := sha256.Sum256(dump), sha256.Sum256(rawTOC), sha256.Sum256(allowlist)
	manifest := gcp03restore.Manifest{
		Format: gcp03restore.ManifestFormat, RunID: runID, Database: targetName,
		DumpObject: "runs/" + runID + "/synthetic-v1-v61.dump", DumpBytes: int64(len(dump)), DumpSHA256: hex.EncodeToString(dumpHash[:]),
		TOCObject: "runs/" + runID + "/dump.toc", TOCSHA256: hex.EncodeToString(tocHash[:]),
		AllowlistObject: "runs/" + runID + "/allowlist.toc", AllowlistSHA256: hex.EncodeToString(allowHash[:]),
		MigrationCount: migrationCount, MigrationSHA256: migrationSHA,
	}

	if _, err := admin.Exec(ctx, "CREATE ROLE "+pgx.Identifier{restoreLogin}.Sanitize()+" LOGIN INHERIT NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS"); err != nil {
		t.Fatal("create local restore login simulation")
	}
	if _, err := admin.Exec(ctx, "CREATE ROLE "+pgx.Identifier{verifyLogin}.Sanitize()+" LOGIN INHERIT NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS"); err != nil {
		t.Fatal("create local verifier login simulation")
	}
	if _, err := admin.Exec(ctx, "CREATE ROLE "+pgx.Identifier{auditLogin}.Sanitize()+" LOGIN INHERIT NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS"); err != nil {
		t.Fatal("create local restricted audit login simulation")
	}
	if _, err := admin.Exec(ctx, "GRANT "+pgx.Identifier{owner}.Sanitize()+" TO "+pgx.Identifier{restoreLogin}.Sanitize()); err != nil {
		t.Fatal("grant local restore role simulation")
	}
	if _, err := admin.Exec(ctx, "GRANT "+pgx.Identifier{verifier}.Sanitize()+" TO "+pgx.Identifier{verifyLogin}.Sanitize()); err != nil {
		t.Fatal("grant local verifier role simulation")
	}
	restoreURL := localDatabaseURL(parsed, targetName, restoreLogin)
	verifyURL := localDatabaseURL(parsed, targetName, verifyLogin)
	baseline, err := inspectReadOnlyBaseline(ctx, verifyURL, runID)
	if err != nil || !baseline["transaction_read_only"] || !baseline["target_empty_except_extensions"] {
		t.Fatalf("diagnostic did not report a read-only empty target before restore: baseline=%v err=%v", baseline, err)
	}
	if err := checkRestorePrerequisites(ctx, restoreURL, runID); err != nil {
		t.Fatalf("restore role preflight failed: %v", err)
	}
	badManifest := manifest
	badManifest.DumpSHA256 = strings.Repeat("0", 64)
	if _, err := restoreSynthetic(ctx, restoreURL, runID, badManifest, dump, rawTOC, allowlist); err == nil {
		t.Fatal("restore accepted a corrupted dump hash")
	}
	if _, err := restoreSynthetic(ctx, restoreURL, runID, manifest, dump, rawTOC, allowlist); err != nil {
		var processErr subprocessFailure
		if errors.As(err, &processErr) {
			t.Fatalf("restore phase failed: code=%s sqlstate=%s exit=%d", processErr.code, processErr.sqlState, processErr.exitCode)
		}
		t.Fatalf("restore phase failed: %v", err)
	}
	baseline, err = inspectReadOnlyBaseline(ctx, verifyURL, runID)
	if err != nil || !baseline["transaction_read_only"] || baseline["target_empty_except_extensions"] || !baseline["application_relations_present"] || !baseline["migration_history_present"] {
		t.Fatalf("diagnostic did not safely detect restored schema metadata: baseline=%v err=%v", baseline, err)
	}
	if _, err := restoreSynthetic(ctx, restoreURL, runID, manifest, dump, rawTOC, allowlist); err == nil {
		t.Fatal("restore retry accepted a populated target")
	}
	if _, err := verifyRestored(ctx, verifyURL, runID, migrationSHA, false); err != nil {
		t.Fatalf("separate verifier phase failed: %v", err)
	}
	if _, err := verifyRestored(ctx, verifyURL, runID, strings.Repeat("0", 64), false); err == nil {
		t.Fatal("verification accepted a corrupted migration inventory pin")
	}

	// The restricted audit login proves the fixed grants independently. This
	// local image keeps postgres as superuser, so retirement is not a Cloud SQL
	// privilege simulation.
	control, err = pgx.Connect(ctx, controlURL)
	if err != nil {
		t.Fatal("connect as isolated control role for restricted audit setup")
	}
	var postgresIsSuper, ownsOccupancy bool
	if err := control.QueryRow(ctx, `SELECT r.rolsuper,pg_catalog.pg_get_userbyid(c.relowner)='postgres'
 FROM pg_catalog.pg_roles r,pg_catalog.pg_class c WHERE r.rolname='postgres' AND c.oid='public.ocupacion'::regclass`).Scan(&postgresIsSuper, &ownsOccupancy); err != nil || postgresIsSuper || ownsOccupancy {
		t.Fatal("retirement harness postgres must be NOSUPERUSER and not own the audited table")
	}
	var auditIsSuper bool
	if _, err := control.Exec(ctx, "GRANT CONNECT ON DATABASE espacigo_scratch TO "+pgx.Identifier{auditLogin}.Sanitize()); err != nil {
		t.Fatal("grant restricted local audit login database connection")
	}
	if err := control.QueryRow(ctx, `SELECT rolsuper FROM pg_catalog.pg_roles WHERE rolname=$1`, auditLogin).Scan(&auditIsSuper); err != nil || auditIsSuper {
		t.Fatal("restricted audit simulation unexpectedly has superuser capability")
	}
	auditConn, err := pgx.Connect(ctx, localDatabaseURL(parsed, targetName, auditLogin))
	if err != nil {
		t.Fatal("connect using restricted local audit login")
	}
	if err := verifyRetirementAuditACL(ctx, auditConn); err != nil {
		_ = auditConn.Close(context.Background())
		t.Fatalf("restricted local audit privilege verification failed: %v", err)
	}
	_ = auditConn.Close(context.Background())
	ownerConn, err := pgx.Connect(ctx, restoreURL)
	if err != nil {
		t.Fatal("connect as synthetic restore login for grant-control checks")
	}
	if _, err := ownerConn.Exec(ctx, "SET ROLE "+pgx.Identifier{owner}.Sanitize()); err != nil {
		_ = ownerConn.Close(context.Background())
		t.Fatal("select restricted restore owner")
	}
	if _, err := ownerConn.Exec(ctx, "REVOKE SELECT ON TABLE public.schema_migrations FROM postgres"); err != nil {
		_ = ownerConn.Close(context.Background())
		t.Fatal("remove one audit grant to exercise retire preflight")
	}
	_ = ownerConn.Close(context.Background())
	if err := retire(ctx, targetAdminURL, adminPassword, runID); err == nil {
		t.Fatal("retire proceeded without its explicit migration audit grant")
	}
	ownerConn, err = pgx.Connect(ctx, restoreURL)
	if err != nil {
		t.Fatal("reconnect as synthetic restore login")
	}
	if _, err := ownerConn.Exec(ctx, "SET ROLE "+pgx.Identifier{owner}.Sanitize()); err != nil {
		_ = ownerConn.Close(context.Background())
		t.Fatal("reselect restricted restore owner")
	}
	if _, err := ownerConn.Exec(ctx, "GRANT SELECT ON TABLE public.schema_migrations TO postgres"); err != nil {
		_ = ownerConn.Close(context.Background())
		t.Fatal("restore fixed migration audit grant")
	}
	_ = ownerConn.Close(context.Background())

	if _, err := control.Exec(ctx, "CREATE ROLE "+pgx.Identifier{extraRole}.Sanitize()+" NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS"); err != nil {
		t.Fatal("create retirement rollback sentinel role")
	}
	if _, err := control.Exec(ctx, "GRANT "+pgx.Identifier{extraRole}.Sanitize()+" TO "+pgx.Identifier{restoreLogin}.Sanitize()); err != nil {
		t.Fatal("grant retirement rollback sentinel membership")
	}
	if err := retire(ctx, targetAdminURL, adminPassword, runID); err == nil {
		t.Fatal("retirement committed despite the unexpected extra membership")
	}
	var restoreMembership, extraMembership bool
	if err := control.QueryRow(ctx, `SELECT pg_catalog.pg_has_role($1,$2,'MEMBER'),pg_catalog.pg_has_role($1,$3,'MEMBER')`, restoreLogin, owner, extraRole).Scan(&restoreMembership, &extraMembership); err != nil || !restoreMembership || !extraMembership {
		t.Fatal("failed retirement did not preserve expected and sentinel memberships")
	}
	var baselineRows, allOccupancy int
	if err := control.QueryRow(ctx, `SELECT count(*), (SELECT count(*) FROM public.ocupacion WHERE id IN ('eeeeeeee-eeee-4eee-8eee-eeeeeeeeee01','eeeeeeee-eeee-4eee-8eee-eeeeeeeeee02')) FROM public.ocupacion`).Scan(&allOccupancy, &baselineRows); err != nil || allOccupancy != 2 || baselineRows != 2 {
		t.Fatal("failed retirement changed the baseline fixture rows")
	}
	if _, err := control.Exec(ctx, "REVOKE "+pgx.Identifier{extraRole}.Sanitize()+" FROM "+pgx.Identifier{restoreLogin}.Sanitize()); err != nil {
		t.Fatal("remove retirement rollback sentinel membership")
	}
	ownerConn, err = pgx.Connect(ctx, restoreURL)
	if err != nil {
		t.Fatal("connect to the synthetic owner for the rollback sentinel row")
	}
	if _, err := ownerConn.Exec(ctx, "SET ROLE "+pgx.Identifier{owner}.Sanitize()); err != nil {
		_ = ownerConn.Close(context.Background())
		t.Fatal("select synthetic owner for the rollback sentinel row")
	}
	if _, err := ownerConn.Exec(ctx, "UPDATE public.ocupacion SET id='eeeeeeee-eeee-4eee-8eee-eeeeeeeeee03' WHERE id='eeeeeeee-eeee-4eee-8eee-eeeeeeeeee01'"); err != nil {
		_ = ownerConn.Close(context.Background())
		t.Fatal("prepare a fixed retirement row to exercise delete rollback")
	}
	_ = ownerConn.Close(context.Background())
	if err := retire(ctx, targetAdminURL, adminPassword, runID); err == nil {
		t.Fatal("retirement committed after deleting a fixed baseline row")
	}
	var rollbackRow, rollbackCount int
	if err := control.QueryRow(ctx, `SELECT count(*) FILTER (WHERE id='eeeeeeee-eeee-4eee-8eee-eeeeeeeeee03'),count(*) FROM public.ocupacion`).Scan(&rollbackRow, &rollbackCount); err != nil || rollbackRow != 1 || rollbackCount != 2 {
		t.Fatal("retirement did not roll back its fixed calendar-row deletion")
	}
	ownerConn, err = pgx.Connect(ctx, restoreURL)
	if err != nil {
		t.Fatal("reconnect to restore the baseline fixture row")
	}
	if _, err := ownerConn.Exec(ctx, "SET ROLE "+pgx.Identifier{owner}.Sanitize()); err != nil {
		_ = ownerConn.Close(context.Background())
		t.Fatal("reselect synthetic owner to restore baseline fixture row")
	}
	if _, err := ownerConn.Exec(ctx, "UPDATE public.ocupacion SET id='eeeeeeee-eeee-4eee-8eee-eeeeeeeeee01' WHERE id='eeeeeeee-eeee-4eee-8eee-eeeeeeeeee03'"); err != nil {
		_ = ownerConn.Close(context.Background())
		t.Fatal("restore the baseline fixture row after rollback check")
	}
	_ = ownerConn.Close(context.Background())
	if err := retire(ctx, targetAdminURL, adminPassword, runID); err != nil {
		t.Fatalf("atomic retire phase failed in the disposable local database: %v", err)
	}
	for _, login := range []string{restoreLogin, verifyLogin} {
		groupRole := owner
		if login == verifyLogin {
			groupRole = verifier
		}
		var canLogin, member bool
		var membershipCount int
		if err := control.QueryRow(ctx, `SELECT u.rolcanlogin,pg_catalog.pg_has_role(u.rolname,$2,'MEMBER'),(SELECT count(*) FROM pg_catalog.pg_auth_members m WHERE m.member=u.oid) FROM pg_catalog.pg_roles u WHERE u.rolname=$1`, login, groupRole).Scan(&canLogin, &member, &membershipCount); err != nil || !canLogin || !member || membershipCount != 1 {
			t.Fatalf("retirement must leave the IAM SQL user login-capable with only its exact database role until Terraform deletes the account: %s", login)
		}
	}
	if err := retire(ctx, targetAdminURL, adminPassword, runID); err == nil {
		t.Fatal("retirement accepted a second attempt after synthetic rows were removed")
	}
	_ = control.Close(context.Background())
}

func localDatabaseURL(adminURL *url.URL, databaseName, username string) string {
	copyURL := *adminURL
	copyURL.Path = "/" + databaseName
	copyURL.User = url.User(username)
	return copyURL.String()
}

func TestFailureEvidenceIsHashedAndSanitized(t *testing.T) {
	env := map[string]string{"GCP03_PHASE": "restore", "GCP03_RUN_ID": "local01", "GCP03_MANIFEST_SHA256": strings.Repeat("a", 64)}
	getenv := func(key string) string { return env[key] }
	var output strings.Builder
	writeFailureEvidence(&output, getenv, errors.New("password=do-not-print token=secret row@example.test"))
	if strings.Contains(output.String(), "do-not-print") || strings.Contains(output.String(), "secret") || strings.Contains(output.String(), "row@example.test") {
		t.Fatal("sanitized failure output leaked a private error value")
	}
	var record failureEvidence
	if err := json.Unmarshal([]byte(output.String()), &record); err != nil {
		t.Fatal("failure evidence is not valid JSON")
	}
	hash := record.FailureSHA256
	record.FailureSHA256 = ""
	canonical, _ := json.Marshal(record)
	sum := sha256.Sum256(canonical)
	if hash != hex.EncodeToString(sum[:]) || record.Phase != "restore" || record.Stage != "phase_execute" || record.RunID != "local01" {
		t.Fatal("failure evidence hash or phase binding is invalid")
	}
}

func TestFailureEvidenceRetainsOnlyAllowlistedStageAndSQLState(t *testing.T) {
	env := map[string]string{"GCP03_PHASE": "restore", "GCP03_RUN_ID": "local01", "GCP03_MANIFEST_SHA256": strings.Repeat("b", 64)}
	var output strings.Builder
	failure := runFailure{cause: subprocessFailure{code: "PG_RESTORE_PERMISSION_DENIED", exitCode: 1, sqlState: "42501"}, proxyShutdownConfirmed: true}
	writeFailureEvidence(&output, func(key string) string { return env[key] }, failure)
	var record failureEvidence
	if err := json.Unmarshal([]byte(output.String()), &record); err != nil {
		t.Fatal("failure evidence is not valid JSON")
	}
	if strings.Contains(output.String(), "password") || strings.Contains(output.String(), "stderr") || record.Stage != "pg_restore" || record.ErrorCode != "PG_RESTORE_PERMISSION_DENIED" || record.SQLState != "42501" || record.SubprocessExitCode != 1 || !record.ProxyShutdownConfirmed {
		t.Fatal("failure evidence lost safe diagnostics or exposed raw subprocess detail")
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) { return f(request) }

type testHTTPClient struct{ transport http.RoundTripper }

func (c testHTTPClient) Do(request *http.Request) (*http.Response, error) {
	return c.transport.RoundTrip(request)
}

func TestProxyShutdownUsesOnlyFixedLoopbackEndpoint(t *testing.T) {
	called := false
	client := testHTTPClient{transport: roundTripperFunc(func(request *http.Request) (*http.Response, error) {
		called = true
		if request.Method != http.MethodGet || request.URL.String() != "http://127.0.0.1:9091/quitquitquit" {
			t.Fatalf("unexpected proxy shutdown endpoint %s %s", request.Method, request.URL.String())
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
	})}
	if err := shutdownProxy(context.Background(), client); err != nil || !called {
		t.Fatalf("fixed proxy shutdown was not confirmed: called=%t err=%v", called, err)
	}
}

func TestProxyShutdownRunsAfterSuccessAndError(t *testing.T) {
	for _, wantRunErr := range []bool{false, true} {
		calls := 0
		runner := func(context.Context) (evidence, error) {
			if wantRunErr {
				return evidence{}, errors.New("phase failure")
			}
			return evidence{Phase: "verify"}, nil
		}
		got, err := runWithProxyShutdown(context.Background(), runner, func(context.Context) error { calls++; return nil })
		if calls != 1 || (err != nil) != wantRunErr || (!wantRunErr && got.Phase != "verify") {
			t.Fatalf("shutdown lifecycle mismatch: calls=%d err=%v result=%+v", calls, err, got)
		}
	}
}
