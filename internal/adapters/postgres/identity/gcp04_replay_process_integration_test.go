package postgres_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/cloudstorage"
	identitypg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/identity"
	verificationpg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/verification"
	"github.com/HernanEspinozaDev/espaciGo/internal/database/cloudsql"
	"github.com/HernanEspinozaDev/espaciGo/internal/privacy"
	"github.com/HernanEspinozaDev/espaciGo/internal/verification"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const gcp04ReplayTestModeEnv = "GCP04_REPLAY_PROCESS_MODE"

type gcp04ReplayProcessResult struct {
	PID      int                               `json:"pid"`
	Status   string                            `json:"status,omitempty"`
	Manifest privacy.SuppressionReplayManifest `json:"manifest,omitempty"`
	Results  []privacy.SuppressionReplayResult `json:"results,omitempty"`
}

// TestGCP04ReplayProcessHelper is launched as a separate OS process by the
// gated integration test below. It uses the real PostgreSQL repository and the
// same Cloud Storage adapter as runtime, pointed at the local fake endpoint.
func TestGCP04ReplayProcessHelper(t *testing.T) {
	mode := os.Getenv(gcp04ReplayTestModeEnv)
	if mode == "" {
		t.Skip("subprocess helper")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 55*time.Second)
	defer cancel()
	databaseURL := os.Getenv("GCP04_REPLAY_DATABASE_URL")
	pool, closeConnector, err := openGCP04ReplayPool(ctx, databaseURL)
	if err != nil {
		t.Fatalf("open disposable test database: %v", err)
	}
	defer func() {
		pool.Close()
		closeConnector()
	}()
	repo := identitypg.NewIdentityRepository(pool)
	service, err := privacy.NewService(repo)
	if err != nil {
		t.Fatal(err)
	}
	bucket, replayPrefix := gcp04ReplayStorageConfig()
	registry, err := cloudstorage.NewReplayRegistry(ctx, bucket, replayPrefix)
	if err != nil {
		t.Fatalf("open replay registry emulator client: %v", err)
	}
	defer registry.Close()
	result := gcp04ReplayProcessResult{PID: os.Getpid()}
	switch mode {
	case "suppress":
		gate := bufio.NewReader(os.Stdin)
		line, err := gate.ReadString('\n')
		if err != nil || strings.TrimSpace(line) != "start" {
			t.Fatalf("subprocess start gate: %v", err)
		}
		outcome, err := service.ExecuteSuppression(ctx, os.Getenv("GCP04_REPLAY_ADMIN_ID"), os.Getenv("GCP04_REPLAY_REQUEST_ID"), os.Getenv("GCP04_REPLAY_IDEMPOTENCY_KEY"), os.Getenv("GCP04_REPLAY_IDEMPOTENCY_KEY"), time.Now, noOpPrivacyCleaner{})
		if err != nil || outcome.Status != "completada" {
			t.Fatalf("synthetic suppression status=%q err=%v", outcome.Status, err)
		}
		var replayStore privacy.SuppressionReplayStore = registry
		if barrierID := os.Getenv("GCP04_REPLAY_BARRIER_ID"); barrierID != "" {
			replayStore = &gcp04BarrierReplayStore{
				inner: registry, pool: pool, barrierID: barrierID,
				participant: os.Getenv("GCP04_REPLAY_BARRIER_PARTICIPANT"),
			}
		}
		manifest, err := service.ExportSuppressionReplayManifestWithStore(ctx, replayStore)
		if err != nil {
			t.Fatalf("synchronize external replay registry: %v", err)
		}
		result.Status = outcome.Status
		result.Manifest = manifest
	case "export":
		manifest, err := service.ExportSuppressionReplayManifestWithStore(ctx, registry)
		if err != nil {
			t.Fatalf("export/reconcile registry after restart: %v", err)
		}
		result.Manifest = manifest
	case "restore":
		manifest, err := service.ExportSuppressionReplayManifestWithStore(ctx, registry)
		if err != nil {
			t.Fatalf("export registry from restored database: %v", err)
		}
		results, err := service.ReplaySuppressionManifest(ctx, manifest, os.Getenv("GCP04_REPLAY_RESTORE_ID"), os.Getenv("GCP04_REPLAY_ADMIN_ID"), time.Now().UTC(), noOpPrivacyCleaner{})
		if err != nil {
			t.Fatalf("apply external replay to restored database: %v", err)
		}
		result.Manifest = manifest
		result.Results = results
	default:
		t.Fatalf("unknown subprocess mode %q", mode)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(os.Getenv("GCP04_REPLAY_RESULT_PATH"), encoded, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestGCP04ReplayTwoProcessesPostgres18(t *testing.T) {
	if os.Getenv("RUN_GCP04_REPLAY_INTEGRATION") != "YES" {
		t.Skip("run with scripts/test-gcp04-replay-postgres.sh against its disposable PostgreSQL 18 cluster")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 110*time.Second)
	defer cancel()
	h := newAuthHarness(t)
	h.ctx = ctx
	var serverVersionText string
	if err := h.pool.QueryRow(ctx, `SHOW server_version_num`).Scan(&serverVersionText); err != nil {
		t.Fatalf("read PostgreSQL server version: %v", err)
	}
	serverVersion, err := strconv.Atoi(serverVersionText)
	if err != nil || serverVersion/10000 != 18 {
		t.Fatalf("expected PostgreSQL 18, server version=%q err=%v", serverVersionText, err)
	}
	var migrationCount int
	if err := h.pool.QueryRow(ctx, `SELECT count(*) FROM public.schema_migrations`).Scan(&migrationCount); err != nil || migrationCount != 62 {
		t.Fatalf("current schema migration ledger count=%d err=%v, want V1-V62", migrationCount, err)
	}

	adminID := h.register(t, "gcp04-replay-admin@ejemplo.invalid")
	h.verify(t)
	if _, err := h.pool.Exec(ctx, `INSERT INTO public.rol_usuario(usuario_id,rol) VALUES($1,'administrador')`, adminID); err != nil {
		t.Fatal(err)
	}
	targetA := h.register(t, "gcp04-replay-a@ejemplo.invalid")
	h.verify(t)
	targetB := h.register(t, "gcp04-replay-b@ejemplo.invalid")
	h.verify(t)
	targetFailure := h.register(t, "gcp04-replay-delete-failure@ejemplo.invalid")
	h.verify(t)
	requests := make([]string, 0, 2)
	for _, target := range []string{targetA, targetB, targetFailure} {
		if _, err := h.repo.SaveProfile(ctx, target, "Cuenta sintética replay", nil); err != nil {
			t.Fatal(err)
		}
		request, err := h.repo.CreateRightsRequest(ctx, target, "supresion", "web")
		if err != nil {
			t.Fatal(err)
		}
		if target != targetFailure {
			requests = append(requests, request.ID)
		}
	}
	evidenceID := seedReplayFailureEvidence(t, h, adminID, targetFailure)

	cloudMode := gcp04ReplayCloudMode()
	emulator := &gcp04ReplayGCS{initialWriteBarrier: make(chan struct{}), loseNextWrite: true}
	emulatorURL := ""
	if !cloudMode {
		server := httptest.NewServer(emulator)
		defer server.Close()
		emulatorURL = server.URL
		t.Setenv("STORAGE_EMULATOR_HOST", emulatorURL)
	}

	// Clone the synthetic V1-V62 database before either process mutates it.
	// This also works through the Cloud SQL Go Connector; pg_dump cannot use its
	// in-process dialer, so the snapshot is a PostgreSQL TEMPLATE database.
	snapshotURL, cleanupSnapshot := createGCP04ReplaySnapshot(t, h, ctx)
	defer cleanupSnapshot()
	if _, err := h.pool.Exec(ctx, `CREATE TABLE public.gcp04_replay_barrier (barrier_id text NOT NULL, participant text NOT NULL, PRIMARY KEY (barrier_id, participant))`); err != nil {
		t.Fatalf("create disposable replay synchronization table: %v", err)
	}
	barrierID := fmt.Sprintf("gcp04-%d", time.Now().UnixNano())
	bucket, _ := gcp04ReplayStorageConfig()
	_, blobPrefix := gcp04ReplayBlobConfig()
	if cloudMode {
		blobStore, err := cloudstorage.NewWithPrefix(ctx, bucket, blobPrefix)
		if err != nil {
			t.Fatalf("open real GCS evidence store: %v", err)
		}
		if err := blobStore.Put(ctx, evidenceID, []byte("synthetic-gcp04-replay-evidence")); err != nil {
			_ = blobStore.Close()
			t.Fatalf("seed private synthetic evidence object in GCS: %v", err)
		}
		_ = blobStore.Close()
	}
	databaseURL := gcp04ReplayURLForDatabase(os.Getenv("TEST_DATABASE_URL"), h.pool.Config().ConnConfig.Database)

	processes := []gcp04ReplayProcessConfig{
		{mode: "suppress", databaseURL: databaseURL, actorID: adminID, requestID: requests[0], idempotencyKey: "gcp04-replay-process-a", barrierID: barrierID, barrierParticipant: "a"},
		{mode: "suppress", databaseURL: databaseURL, actorID: adminID, requestID: requests[1], idempotencyKey: "gcp04-replay-process-b", barrierID: barrierID, barrierParticipant: "b"},
	}
	results := runGCP04ReplayProcesses(t, ctx, emulatorURL, processes)
	if results[0].Status != "completada" || results[1].Status != "completada" || results[0].PID == results[1].PID {
		t.Fatalf("concurrent suppression processes did not both complete independently: %+v", results)
	}
	var sourceLedgerRows int
	if err := h.pool.QueryRow(ctx, `SELECT count(*) FROM public.registro_replay_baja_local`).Scan(&sourceLedgerRows); err != nil || sourceLedgerRows != 2 {
		t.Fatalf("shared PostgreSQL V62 ledger rows=%d err=%v, want 2", sourceLedgerRows, err)
	}
	if !cloudMode {
		if generation, conflicts, lost, writes := emulator.counters(); generation < 1 || conflicts < 1 || lost != 1 || writes < 2 {
			t.Fatalf("fake GCS did not exercise CAS conflict and lost response: generation=%d conflicts=%d lost=%d writes=%d", generation, conflicts, lost, writes)
		}
	}

	// A newly started process proves the shared DB ledger and GCS manifest are
	// readable after both writers have exited.
	restartResult := runGCP04ReplayProcesses(t, ctx, emulatorURL, []gcp04ReplayProcessConfig{{mode: "export", databaseURL: databaseURL}})[0]
	assertReplayManifest(t, restartResult.Manifest, 2)

	// A failed object deletion must leave its DB evidence job retryable and the
	// source suppression incomplete. This uses the actual GCS blob adapter.
	blobBucket, blobPrefix := gcp04ReplayBlobConfig()
	blobStore, err := cloudstorage.NewWithPrefix(ctx, blobBucket, blobPrefix)
	if err != nil {
		t.Fatal(err)
	}
	failureService, err := privacy.NewService(h.repo)
	if err != nil {
		t.Fatal(err)
	}
	failureRequest, err := h.repo.ListOwnRightsRequests(ctx, targetFailure)
	if err != nil || len(failureRequest) != 1 {
		t.Fatalf("delete failure suppression request count=%d err=%v", len(failureRequest), err)
	}
	failureOutcome, err := failureService.ExecuteSuppression(ctx, adminID, failureRequest[0].ID, "gcp04-delete-failure", "gcp04-delete-failure", time.Now, blobStore)
	_ = blobStore.Close()
	deleteAttempts := emulator.deleteAttemptsCount()
	if !cloudMode && (err != nil || failureOutcome.Status != "limpieza_pendiente" || failureOutcome.PendingFiles != 1 || failureOutcome.CompletedAt != nil) {
		t.Fatalf("fake remote Delete failure was confirmed instead of remaining pending: result=%+v deleteAttempts=%d err=%v", failureOutcome, deleteAttempts, err)
	}
	if cloudMode && (err != nil || failureOutcome.Status != "completada" || failureOutcome.PendingFiles != 0 || failureOutcome.CompletedAt == nil) {
		t.Fatalf("real Cloud Storage Delete did not complete the synthetic evidence removal: result=%+v err=%v", failureOutcome, err)
	}
	var pending, attempts, completed int
	var lastError string
	var jobExecutionID, jobEvidenceID string
	if err := h.pool.QueryRow(ctx, `SELECT count(*),COALESCE(max(j.intentos),0),count(*) FILTER (WHERE j.completada_en IS NOT NULL),COALESCE(max(j.ultimo_codigo_error),''),COALESCE(max(j.ejecucion_id::text),''),COALESCE(max(j.evidencia_id::text),'') FROM public.baja_archivo_pendiente_local j JOIN public.ejecucion_baja_local x ON x.id=j.ejecucion_id WHERE x.solicitud_id=$1`, failureRequest[0].ID).Scan(&pending, &attempts, &completed, &lastError, &jobExecutionID, &jobEvidenceID); err != nil || pending != 1 || (cloudMode && completed != 1) || (!cloudMode && (attempts != 1 || completed != 0 || lastError != "archivo_sintetico_no_eliminado")) {
		t.Fatalf("failed remote Delete job state pending=%d attempts=%d completed=%d code=%q execution=%s evidence=%s deleteAttempts=%d err=%v", pending, attempts, completed, lastError, jobExecutionID, jobEvidenceID, deleteAttempts, err)
	}
	if !cloudMode && deleteAttempts < 1 {
		t.Fatalf("fake GCS did not receive Delete: attempts=%d", deleteAttempts)
	}

	// Restore the old database clone into another isolated DB. Its V62 ledger
	// is empty; the restarted helper exports the external manifest and applies it.
	restoreURL, cleanupRestore := createGCP04ReplayRestoreDB(t, ctx, snapshotURL)
	defer cleanupRestore()
	var restoredLedgerRows int
	restoredPool, closeRestoredConnector, err := openGCP04ReplayPool(ctx, restoreURL)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { restoredPool.Close(); closeRestoredConnector() }()
	if err := restoredPool.QueryRow(ctx, `SELECT count(*) FROM public.registro_replay_baja_local`).Scan(&restoredLedgerRows); err != nil || restoredLedgerRows != 0 {
		restoredPool.Close()
		t.Fatalf("old snapshot ledger rows=%d err=%v, want 0", restoredLedgerRows, err)
	}
	restoredPool.Close()
	closeRestoredConnector()
	applyResult := runGCP04ReplayProcesses(t, ctx, emulatorURL, []gcp04ReplayProcessConfig{{mode: "restore", databaseURL: restoreURL, actorID: adminID, restoreID: "f2040000-0000-4000-8000-000000000001"}})[0]
	assertReplayManifest(t, applyResult.Manifest, 2)
	if len(applyResult.Results) != 2 || applyResult.Results[0].Status != "reaplicada" || applyResult.Results[1].Status != "reaplicada" {
		t.Fatalf("apply on old restored database did not replay both completed source suppressions: %+v", applyResult.Results)
	}
	restoredPool, closeRestoredConnector, err = openGCP04ReplayPool(ctx, restoreURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { restoredPool.Close(); closeRestoredConnector() })
	var restoredLedgerRowsAfter int
	if err := restoredPool.QueryRow(ctx, `SELECT count(*) FROM public.registro_replay_baja_local`).Scan(&restoredLedgerRowsAfter); err != nil || restoredLedgerRowsAfter != 2 {
		t.Fatalf("restored V62 ledger rows=%d err=%v, want 2 after apply", restoredLedgerRowsAfter, err)
	}
	for _, accountID := range []string{targetA, targetB} {
		var state, hash string
		if err := restoredPool.QueryRow(ctx, `SELECT estado,hash_clave FROM public.usuario WHERE id=$1`, accountID).Scan(&state, &hash); err != nil || state != "desidentificado" || hash != "" {
			t.Fatalf("old snapshot replay did not minimize account %s: state=%q hash_present=%t err=%v", accountID, state, hash != "", err)
		}
	}
}

type gcp04ReplayProcessConfig struct {
	mode, databaseURL, actorID, requestID, idempotencyKey, restoreID string
	barrierID, barrierParticipant                                    string
}

func runGCP04ReplayProcesses(t *testing.T, ctx context.Context, emulatorURL string, configs []gcp04ReplayProcessConfig) []gcp04ReplayProcessResult {
	t.Helper()
	type childProcess struct {
		cmd    *exec.Cmd
		stdin  io.WriteCloser
		output *bytes.Buffer
		result gcp04ReplayProcessResult
		path   string
		err    error
	}
	children := make([]*childProcess, 0, len(configs))
	for _, config := range configs {
		resultPath := filepath.Join(t.TempDir(), "result.json")
		cmdCtx, cancel := context.WithTimeout(ctx, 65*time.Second)
		t.Cleanup(cancel)
		cmd := exec.CommandContext(cmdCtx, os.Args[0], "-test.run=^TestGCP04ReplayProcessHelper$", "-test.timeout=58s")
		overrides := map[string]string{
			gcp04ReplayTestModeEnv:                config.mode,
			"GCP04_REPLAY_DATABASE_URL":           config.databaseURL,
			"GCP04_REPLAY_ADMIN_ID":               config.actorID,
			"GCP04_REPLAY_REQUEST_ID":             config.requestID,
			"GCP04_REPLAY_IDEMPOTENCY_KEY":        config.idempotencyKey,
			"GCP04_REPLAY_RESTORE_ID":             config.restoreID,
			"GCP04_REPLAY_RESULT_PATH":            resultPath,
			"GCP04_REPLAY_BARRIER_ID":             config.barrierID,
			"GCP04_REPLAY_BARRIER_PARTICIPANT":    config.barrierParticipant,
			"GCP04_REPLAY_CLOUD":                  os.Getenv("GCP04_REPLAY_CLOUD"),
			"GCP04_REPLAY_DATABASE_PASSWORD_FILE": os.Getenv("GCP04_REPLAY_DATABASE_PASSWORD_FILE"),
			"GCP04_REPLAY_CLOUD_SQL_INSTANCE":     os.Getenv("GCP04_REPLAY_CLOUD_SQL_INSTANCE"),
			"GCP04_REPLAY_CLOUD_SQL_PRIVATE_IP":   os.Getenv("GCP04_REPLAY_CLOUD_SQL_PRIVATE_IP"),
			"GCP04_REPLAY_BUCKET":                 os.Getenv("GCP04_REPLAY_BUCKET"),
			"GCP04_REPLAY_PREFIX":                 os.Getenv("GCP04_REPLAY_PREFIX"),
			"GCP04_REPLAY_BLOB_PREFIX":            os.Getenv("GCP04_REPLAY_BLOB_PREFIX"),
		}
		if emulatorURL != "" {
			overrides["STORAGE_EMULATOR_HOST"] = emulatorURL
		}
		cmd.Env = gcp04ReplayProcessEnv(os.Environ(), overrides)
		stdin, err := cmd.StdinPipe()
		if err != nil {
			t.Fatal(err)
		}
		cmd.Stdout = &bytes.Buffer{}
		cmd.Stderr = &bytes.Buffer{}
		if err := cmd.Start(); err != nil {
			t.Fatalf("start helper process: %v", err)
		}
		children = append(children, &childProcess{cmd: cmd, stdin: stdin, output: cmd.Stdout.(*bytes.Buffer), path: resultPath})
	}
	for _, child := range children {
		if configsHaveGate(configs) {
			if _, err := io.WriteString(child.stdin, "start\n"); err != nil {
				t.Fatalf("release subprocess start gate: %v", err)
			}
		}
		_ = child.stdin.Close()
	}
	done := make(chan *childProcess, len(children))
	for _, process := range children {
		go func(process *childProcess) {
			process.err = process.cmd.Wait()
			done <- process
		}(process)
	}
	for range children {
		select {
		case child := <-done:
			if child.err != nil {
				t.Fatalf("helper process %s failed: %v; output=%s", configs[0].mode, child.err, child.output.String())
			}
			data, err := os.ReadFile(child.path)
			if err != nil {
				t.Fatalf("read helper result: %v; output=%s", err, child.output.String())
			}
			if err := json.Unmarshal(data, &child.result); err != nil {
				t.Fatalf("decode helper result: %v", err)
			}
		case <-ctx.Done():
			t.Fatalf("helper processes exceeded integration deadline: %v", ctx.Err())
		}
	}
	results := make([]gcp04ReplayProcessResult, len(children))
	for index, child := range children {
		results[index] = child.result
	}
	return results
}

func configsHaveGate(configs []gcp04ReplayProcessConfig) bool {
	for _, config := range configs {
		if config.mode == "suppress" {
			return true
		}
	}
	return false
}

func gcp04ReplayProcessEnv(environment []string, override map[string]string) []string {
	blocked := map[string]bool{
		"DATABASE_URL": true, "TEST_DATABASE_URL": true, "DATABASE_PASSWORD_FILE": true, "GOOGLE_APPLICATION_CREDENTIALS": true,
		"BLOB_STORE_BACKEND": true, "CLOUD_STORAGE_BUCKET": true, "CLOUD_STORAGE_OBJECT_PREFIX": true,
		"LOCAL_PRIVACY_REPLAY_BACKEND": true, "LOCAL_PRIVACY_REPLAY_BUCKET": true, "LOCAL_PRIVACY_REPLAY_PREFIX": true,
		"RUN_GCP04_REPLAY_INTEGRATION": true,
	}
	values := make(map[string]string)
	for _, item := range environment {
		key, value, ok := strings.Cut(item, "=")
		if ok && !blocked[key] && key != "STORAGE_EMULATOR_HOST" && !strings.HasPrefix(key, "GCP04_REPLAY_") {
			values[key] = value
		}
	}
	for key, value := range override {
		values[key] = value
	}
	out := make([]string, 0, len(values))
	for key, value := range values {
		out = append(out, key+"="+value)
	}
	return out
}

func createGCP04ReplayRestoreDB(t *testing.T, ctx context.Context, snapshotURL string) (string, func()) {
	t.Helper()
	adminURL := os.Getenv("TEST_DATABASE_URL")
	parsed, err := url.Parse(adminURL)
	if err != nil || parsed.Path == "" {
		t.Fatalf("invalid disposable TEST_DATABASE_URL: %v", err)
	}
	restoreDB := fmt.Sprintf("gcp04_replay_restore_%d", time.Now().UnixNano())
	snapshot, err := url.Parse(snapshotURL)
	if err != nil || snapshot.Path == "" {
		t.Fatalf("invalid synthetic GCP-04 database snapshot URL: %v", err)
	}
	admin, closeAdmin, err := openGCP04ReplayConnection(ctx, adminURL)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{restoreDB}.Sanitize()+" TEMPLATE "+pgx.Identifier{strings.TrimPrefix(snapshot.Path, "/")}.Sanitize()); err != nil {
		closeAdmin()
		t.Fatal(err)
	}
	closeAdmin()
	parsed.Path = "/" + restoreDB
	restoreURL := parsed.String()
	cleanup := func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		connection, closeConnection, connectErr := openGCP04ReplayConnection(cleanupCtx, adminURL)
		if connectErr != nil {
			t.Errorf("connect to clean restored scratch database: %v", connectErr)
			return
		}
		defer closeConnection()
		if _, dropErr := connection.Exec(cleanupCtx, "DROP DATABASE IF EXISTS "+pgx.Identifier{restoreDB}.Sanitize()); dropErr != nil {
			t.Errorf("drop only GCP-04 restored scratch database %s: %v", restoreDB, dropErr)
		}
	}
	return restoreURL, cleanup
}

func createGCP04ReplaySnapshot(t *testing.T, h *authHarness, ctx context.Context) (string, func()) {
	t.Helper()
	adminURL := os.Getenv("TEST_DATABASE_URL")
	sourceDB := h.pool.Config().ConnConfig.Database
	snapshotDB := fmt.Sprintf("gcp04_replay_snapshot_%d", time.Now().UnixNano())
	parsedAdmin, err := url.Parse(adminURL)
	if err != nil || parsedAdmin.Path == "" {
		t.Fatalf("invalid disposable TEST_DATABASE_URL for snapshot: %v", err)
	}
	// PostgreSQL requires the template source to have no open sessions.
	h.pool.Close()
	admin, closeAdmin, err := openGCP04ReplayConnection(ctx, adminURL)
	if err != nil {
		t.Fatalf("connect to clone synthetic database snapshot: %v", err)
	}
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{snapshotDB}.Sanitize()+" TEMPLATE "+pgx.Identifier{sourceDB}.Sanitize()); err != nil {
		closeAdmin()
		t.Fatalf("clone pre-suppression synthetic database: %v", err)
	}
	closeAdmin()

	sourceURL := gcp04ReplayURLForDatabase(adminURL, sourceDB)
	newPool, closeConnector, err := openGCP04ReplayPool(ctx, sourceURL)
	if err != nil {
		t.Fatalf("reopen synthetic source database after clone: %v", err)
	}
	h.pool = newPool
	h.repo = identitypg.NewIdentityRepository(newPool)
	t.Cleanup(func() { newPool.Close(); closeConnector() })

	snapshotURL := gcp04ReplayURLForDatabase(adminURL, snapshotDB)
	cleanup := func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		connection, closeConnection, connectErr := openGCP04ReplayConnection(cleanupCtx, adminURL)
		if connectErr != nil {
			t.Errorf("connect to drop synthetic GCP-04 snapshot: %v", connectErr)
			return
		}
		defer closeConnection()
		if _, dropErr := connection.Exec(cleanupCtx, "DROP DATABASE IF EXISTS "+pgx.Identifier{snapshotDB}.Sanitize()); dropErr != nil {
			t.Errorf("drop only synthetic GCP-04 snapshot database: %v", dropErr)
		}
	}
	return snapshotURL, cleanup
}

func gcp04ReplayURLForDatabase(baseURL, databaseName string) string {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return ""
	}
	parsed.Path = "/" + databaseName
	return parsed.String()
}

func gcp04ReplayCloudMode() bool {
	return os.Getenv("GCP04_REPLAY_CLOUD") == "YES"
}

func gcp04ReplayStorageConfig() (bucket, prefix string) {
	bucket = strings.TrimSpace(os.Getenv("GCP04_REPLAY_BUCKET"))
	prefix = strings.TrimSpace(os.Getenv("GCP04_REPLAY_PREFIX"))
	if bucket == "" {
		bucket = "private-bucket"
	}
	if prefix == "" {
		prefix = "privacy"
	}
	return bucket, prefix
}

func gcp04ReplayBlobConfig() (bucket, prefix string) {
	bucket = strings.TrimSpace(os.Getenv("GCP04_REPLAY_BUCKET"))
	prefix = strings.TrimSpace(os.Getenv("GCP04_REPLAY_BLOB_PREFIX"))
	if bucket == "" {
		bucket = "private-bucket"
	}
	if prefix == "" {
		prefix = "evidence"
	}
	return bucket, prefix
}

func readGCP04ReplayPassword() ([]byte, error) {
	path := strings.TrimSpace(os.Getenv("GCP04_REPLAY_DATABASE_PASSWORD_FILE"))
	if path == "" {
		return nil, fmt.Errorf("Cloud SQL password file path is required")
	}
	password, err := os.ReadFile(path)
	if err != nil || len(bytes.TrimSpace(password)) == 0 {
		for i := range password {
			password[i] = 0
		}
		return nil, fmt.Errorf("Cloud SQL password file is unavailable")
	}
	trimmed := bytes.TrimRight(password, "\r\n")
	return append([]byte(nil), trimmed...), nil
}

func openGCP04ReplayConnection(ctx context.Context, databaseURL string) (*pgx.Conn, func(), error) {
	if !gcp04ReplayCloudMode() {
		conn, err := pgx.Connect(ctx, databaseURL)
		if err != nil {
			return nil, nil, err
		}
		return conn, func() { _ = conn.Close(context.Background()) }, nil
	}
	password, err := readGCP04ReplayPassword()
	if err != nil {
		return nil, nil, err
	}
	conn, closeConnector, err := cloudsql.Connect(ctx, databaseURL, string(password), os.Getenv("GCP04_REPLAY_CLOUD_SQL_INSTANCE"), os.Getenv("GCP04_REPLAY_CLOUD_SQL_PRIVATE_IP") == "true")
	for i := range password {
		password[i] = 0
	}
	if err != nil {
		return nil, nil, err
	}
	return conn, func() {
		_ = conn.Close(context.Background())
		closeConnector()
	}, nil
}

func openGCP04ReplayPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, func(), error) {
	if !gcp04ReplayCloudMode() {
		pool, err := pgxpool.New(ctx, databaseURL)
		if err != nil {
			return nil, nil, err
		}
		return pool, func() {}, nil
	}
	password, err := readGCP04ReplayPassword()
	if err != nil {
		return nil, nil, err
	}
	config, err := cloudsql.NewPGXConfig(databaseURL, string(password))
	for i := range password {
		password[i] = 0
	}
	if err != nil {
		return nil, nil, err
	}
	poolConfig, err := identitypg.GCP04ReplayPoolConfig(databaseURL)
	if err != nil {
		return nil, nil, err
	}
	poolConfig.ConnConfig = config
	closeConnector, err := cloudsql.ConfigurePGX(ctx, poolConfig.ConnConfig, os.Getenv("GCP04_REPLAY_CLOUD_SQL_INSTANCE"), os.Getenv("GCP04_REPLAY_CLOUD_SQL_PRIVATE_IP") == "true")
	if err != nil {
		return nil, nil, err
	}
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		closeConnector()
		return nil, nil, err
	}
	return pool, closeConnector, nil
}

type gcp04BarrierReplayStore struct {
	inner       privacy.SuppressionReplayStore
	pool        *pgxpool.Pool
	barrierID   string
	participant string
}

func (s *gcp04BarrierReplayStore) Load(ctx context.Context) (privacy.SuppressionReplaySnapshot, error) {
	return s.inner.Load(ctx)
}

func (s *gcp04BarrierReplayStore) CompareAndSwap(ctx context.Context, revision string, manifest privacy.SuppressionReplayManifest) (string, error) {
	if _, err := s.pool.Exec(ctx, `INSERT INTO public.gcp04_replay_barrier(barrier_id,participant) VALUES($1,$2) ON CONFLICT DO NOTHING`, s.barrierID, s.participant); err != nil {
		return "", err
	}
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		var participants int
		if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM public.gcp04_replay_barrier WHERE barrier_id=$1`, s.barrierID).Scan(&participants); err != nil {
			return "", err
		}
		if participants >= 2 {
			break
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-ticker.C:
		}
	}
	return s.inner.CompareAndSwap(ctx, revision, manifest)
}

func seedReplayFailureEvidence(t *testing.T, h *authHarness, adminID, targetID string) string {
	t.Helper()
	caseID := "f2040000-0000-4000-8000-000000000010"
	fixtureID := "f2040000-0000-4000-8000-000000000011"
	evidenceID := "f2040000-0000-4000-8000-000000000012"
	repository := verificationpg.New(h.pool)
	if _, err := repository.Create(h.ctx, verification.Case{ID: caseID, OwnerID: targetID, Type: "kyc", State: "en_revision", Provider: "local-fixture-v1", EvidenceRef: "fixture:" + fixtureID, Idempotency: "gcp04-replay-evidence-case", CreatedAt: h.now.Add(-time.Hour)}); err != nil {
		t.Fatalf("create synthetic verification case: %v", err)
	}
	if _, err := repository.Review(h.ctx, caseID, adminID, true, "", func() time.Time { return h.now }); err != nil {
		t.Fatalf("approve synthetic verification case: %v", err)
	}
	if _, err := h.pool.Exec(h.ctx, `INSERT INTO public.verificacion_evidencia_sintetica(id,verificacion_id,codigo_fixture,mime_type,tamano_bytes,sha256,creada_en,archivo_id) VALUES($1,$2,'synthetic-png-v1','image/png',16,repeat('a',64),$3,$1)`, evidenceID, caseID, h.now); err != nil {
		t.Fatalf("create synthetic evidence metadata: %v", err)
	}
	var rows int
	if err := h.pool.QueryRow(h.ctx, `SELECT count(*) FROM public.verificacion_evidencia_sintetica e JOIN public.verificacion v ON v.id=e.verificacion_id WHERE v.usuario_id=$1`, targetID).Scan(&rows); err != nil || rows != 1 {
		t.Fatalf("synthetic suppression evidence rows=%d err=%v", rows, err)
	}
	return evidenceID
}

func assertReplayManifest(t *testing.T, manifest privacy.SuppressionReplayManifest, count int) {
	t.Helper()
	if manifest.Version != 1 || len(manifest.Entries) != count {
		t.Fatalf("shared replay manifest version=%d entries=%d, want 1/%d", manifest.Version, len(manifest.Entries), count)
	}
}

func redactReplayCommandOutput(output []byte) string {
	text := string(output)
	for _, secretKey := range []string{"password=", "sslpassword="} {
		if index := strings.Index(strings.ToLower(text), secretKey); index >= 0 {
			text = text[:index] + "[redacted]"
		}
	}
	return text
}

type gcp04ReplayGCS struct {
	mu                  sync.Mutex
	data                []byte
	generation          int64
	writeRequests       int
	initialWriteBarrier chan struct{}
	loseNextWrite       bool
	lostResponses       int
	preconditionErrors  int
	deleteAttempts      int
}

func (s *gcp04ReplayGCS) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/upload/storage/v1/b/private-bucket/o"):
		s.serveWrite(w, r)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/private-bucket/privacy/privacy-replay-v1.json"):
		s.serveReplayRead(w, r)
	case r.Method == http.MethodDelete:
		s.mu.Lock()
		s.deleteAttempts++
		s.mu.Unlock()
		http.Error(w, "synthetic remote object delete failure", http.StatusServiceUnavailable)
	default:
		http.NotFound(w, r)
	}
}

func (s *gcp04ReplayGCS) serveReplayRead(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	data := append([]byte(nil), s.data...)
	generation := s.generation
	s.mu.Unlock()
	if generation == 0 {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.Header().Set("X-Goog-Generation", strconv.FormatInt(generation, 10))
	w.Header().Set("X-Goog-Metageneration", "1")
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (s *gcp04ReplayGCS) serveWrite(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("name") != "privacy/privacy-replay-v1.json" {
		http.Error(w, "unexpected replay object name", http.StatusBadRequest)
		return
	}
	_, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil {
		http.Error(w, "invalid upload content type", http.StatusBadRequest)
		return
	}
	parts := multipart.NewReader(r.Body, params["boundary"])
	metadata, err := parts.NextPart()
	if err != nil {
		http.Error(w, "missing object metadata", http.StatusBadRequest)
		return
	}
	_, _ = io.Copy(io.Discard, metadata)
	content, err := parts.NextPart()
	if err != nil {
		http.Error(w, "missing object data", http.StatusBadRequest)
		return
	}
	data, err := io.ReadAll(content)
	if err != nil {
		http.Error(w, "unable to read object data", http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	s.writeRequests++
	writeRequest := s.writeRequests
	barrier := s.initialWriteBarrier
	s.mu.Unlock()
	if writeRequest == 1 {
		select {
		case <-barrier:
		case <-r.Context().Done():
			return
		}
	} else if writeRequest == 2 {
		close(barrier)
	}
	condition := r.URL.Query().Get("ifGenerationMatch")
	s.mu.Lock()
	if condition == "0" && s.generation != 0 || condition != "0" && condition != strconv.FormatInt(s.generation, 10) {
		s.preconditionErrors++
		s.mu.Unlock()
		http.Error(w, "generation precondition failed", http.StatusPreconditionFailed)
		return
	}
	s.data = append([]byte(nil), data...)
	s.generation++
	generation := s.generation
	loseResponse := s.loseNextWrite
	if loseResponse {
		s.loseNextWrite = false
		s.lostResponses++
	}
	s.mu.Unlock()
	if loseResponse {
		http.Error(w, "synthetic response lost after durable commit", http.StatusServiceUnavailable)
		return
	}
	writeReplayObjectMetadata(w, generation, data)
}

func writeReplayObjectMetadata(w http.ResponseWriter, generation int64, data []byte) {
	checksum := make([]byte, 4)
	binary.BigEndian.PutUint32(checksum, crc32.Checksum(data, crc32.MakeTable(crc32.Castagnoli)))
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"kind": "storage#object", "id": "private-bucket/privacy/privacy-replay-v1.json/" + strconv.FormatInt(generation, 10),
		"bucket": "private-bucket", "name": "privacy/privacy-replay-v1.json", "generation": strconv.FormatInt(generation, 10),
		"metageneration": "1", "size": strconv.Itoa(len(data)), "contentType": "application/json",
		"crc32c": base64.StdEncoding.EncodeToString(checksum), "timeCreated": time.Now().UTC().Format(time.RFC3339Nano), "updated": time.Now().UTC().Format(time.RFC3339Nano),
	})
}

func (s *gcp04ReplayGCS) counters() (generation int64, conflicts, lost, writes int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.generation, s.preconditionErrors, s.lostResponses, s.writeRequests
}

func (s *gcp04ReplayGCS) deleteAttemptsCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.deleteAttempts
}
