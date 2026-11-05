package documentverificationpg

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net"
	"os"
	"sync"
	"testing"
	"time"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const f5ScanFenceDatabase = "espacigo_f5_gate_20261101"

// TestPostgresScanResultIsFencedByOwnershipTransfer is opt-in and deliberately
// commits uniquely identified synthetic rows, then removes and verifies them.
func TestPostgresScanResultIsFencedByOwnershipTransfer(t *testing.T) {
	fixtureCleanupSafe := true
	var workerJoinDeadline time.Time
	dsn := os.Getenv("ESPACIGO_DOCUMENT_VERIFICATION_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ESPACIGO_DOCUMENT_VERIFICATION_TEST_DATABASE_URL is unset")
	}
	expectedIP := net.ParseIP(os.Getenv("ESPACIGO_DOCUMENT_VERIFICATION_TEST_EXPECTED_SERVER_IP"))
	if expectedIP == nil || expectedIP.To4() == nil {
		t.Fatal("ESPACIGO_DOCUMENT_VERIFICATION_TEST_EXPECTED_SERVER_IP must independently specify the expected IPv4 server address")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal("invalid integration DSN")
	}
	if cfg.ConnConfig.Host != "127.0.0.1" || cfg.ConnConfig.Port != 15434 || cfg.ConnConfig.Database != f5ScanFenceDatabase {
		t.Fatal("refusing integration target outside exact loopback F5 endpoint")
	}
	appName := "f5-scan-fence-" + uuid.NewString()
	if cfg.ConnConfig.RuntimeParams == nil {
		cfg.ConnConfig.RuntimeParams = make(map[string]string)
	}
	cfg.ConnConfig.RuntimeParams["application_name"] = appName
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal("unable to connect to authorized loopback synthetic target")
	}
	t.Cleanup(func() {
		if !fixtureCleanupSafe {
			t.Error("leaving scan-fence pool open because a controlled goroutine did not stop")
			return
		}
		pool.Close()
	})

	var database, setupRole string
	var serverIP net.IP
	if err := pool.QueryRow(ctx, `SELECT current_database(),current_user,inet_server_addr()`).Scan(&database, &setupRole, &serverIP); err != nil || database != f5ScanFenceDatabase || setupRole == "espacigo_runtime" || !serverIP.Equal(expectedIP) {
		t.Fatalf("refusing unexpected database/server/setup identity (query error: %v)", err)
	}
	if err := requireScanFenceIsolation(ctx, pool, appName); err != nil {
		t.Fatal(err)
	}
	var v80, v81, v82 bool
	if err := pool.QueryRow(ctx, `SELECT
		EXISTS(SELECT 1 FROM public.schema_migrations WHERE version=80),
		EXISTS(SELECT 1 FROM public.schema_migrations WHERE version=81),
		EXISTS(SELECT 1 FROM public.schema_migrations WHERE version=82)`).Scan(&v80, &v81, &v82); err != nil || !v80 || !v81 || !v82 {
		t.Fatalf("V80/V81/V82 migration identity is not present; refusing trigger-path test (V80=%t V81=%t V82=%t err=%v)", v80, v81, v82, err)
	}

	oldOwner, newOwner, firstSpace, secondSpace := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	t.Cleanup(func() {
		if !fixtureCleanupSafe {
			t.Error("skipping actor/space cleanup because a controlled goroutine did not stop")
			return
		}
		cleanupScanFenceActorsAndSpaces(t, pool, oldOwner, newOwner, firstSpace, secondSpace)
	})
	seedScanFenceActorsAndSpaces(t, ctx, pool, oldOwner, newOwner, firstSpace, secondSpace)
	first := newScanFenceFixture(t, ctx, pool, oldOwner, newOwner, firstSpace, "transfer-first", &fixtureCleanupSafe)
	second := newScanFenceFixture(t, ctx, pool, oldOwner, newOwner, secondSpace, "outcome-first", &fixtureCleanupSafe)
	runtimeTx := scanFenceRuntimeBeginner{pool: pool}

	// Pause VerifyScanClaim after its SELECT has completed but before its
	// read-only transaction commits. The transfer can commit in this window;
	// therefore a true poll remains advisory and the result write must fence.
	pollPID := make(chan int32, 1)
	pollBarrier := &scanFenceCommitBarrier{transactions: scanFenceRuntimeBeginner{pool: pool, backendPID: pollPID}, reached: make(chan struct{}), release: make(chan struct{})}
	defer pollBarrier.unblock()
	pollDone := make(chan struct {
		verified bool
		err      error
	}, 1)
	pollWorkerDone := make(chan struct{})
	go func() {
		defer close(pollWorkerDone)
		verified, err := newWithTransactionBeginner(pollBarrier).VerifyScanClaim(ctx, first.claim)
		pollDone <- struct {
			verified bool
			err      error
		}{verified, err}
	}()
	registerScanFenceWorkerJoin(t, pollWorkerDone, &fixtureCleanupSafe, &workerJoinDeadline, "advisory poll")
	select {
	case <-pollBarrier.reached:
	case got := <-pollDone:
		t.Fatalf("VerifyScanClaim ended before its controlled commit barrier: verified=%t err=%v", got.verified, got.err)
	case <-ctx.Done():
		t.Fatal("VerifyScanClaim did not reach the pre-commit barrier")
	}
	// Drain the reported backend PID: BeginTx only reports it after applying
	// the restricted runtime role and establishing the PostgreSQL transaction.
	<-pollPID
	transferPID := make(chan int32, 1)
	transferDone := make(chan error, 1)
	transferWorkerDone := make(chan struct{})
	go func() {
		defer close(transferWorkerDone)
		transferDone <- runScanFenceTransfer(ctx, scanFenceRuntimeBeginner{pool: pool, backendPID: transferPID}, firstSpace, newOwner, nil)
	}()
	registerScanFenceWorkerJoin(t, transferWorkerDone, &fixtureCleanupSafe, &workerJoinDeadline, "transfer-first ownership update")
	select {
	case <-transferPID:
	case err := <-transferDone:
		t.Fatalf("transfer failed while advisory poll was paused: %v", err)
	case <-ctx.Done():
		t.Fatal("transfer did not complete while advisory poll was paused")
	}
	if err := <-transferDone; err != nil {
		t.Fatalf("commit ownership transfer during advisory poll: %v", err)
	}
	pollBarrier.unblock()
	select {
	case got := <-pollDone:
		if got.err != nil || !got.verified {
			t.Fatalf("barriered VerifyScanClaim=(%t,%v), want stale advisory true after transfer", got.verified, got.err)
		}
	case <-ctx.Done():
		t.Fatal("barriered advisory poll did not return")
	}

	transferSnapshot, err := scanFenceSnapshot(ctx, pool, first.caseID, first.fileID)
	if err != nil {
		t.Fatalf("capture persisted transfer state: %v", err)
	}
	outcomeRepo := newWithTransactionBeginner(scanFenceRuntimeBeginner{pool: pool})
	outcomeDone := make(chan error, 1)
	if err := outcomeRepo.RecordScanOutcome(ctx, first.claim, ScanOutcome{State: dv.FileClean, ScannerVersion: "synthetic-fence-test", SignaturesAt: time.Now().UTC().Truncate(time.Microsecond)}); !errors.Is(err, dv.ErrConflict) {
		t.Fatalf("transfer-first scan outcome error=%v, want ErrConflict", err)
	}
	afterOutcome, err := scanFenceSnapshot(ctx, pool, first.caseID, first.fileID)
	if err != nil {
		t.Fatalf("capture transfer-first state after stale outcome: %v", err)
	}
	if transferSnapshot != afterOutcome {
		t.Fatalf("stale outcome changed transfer state\\nbefore=%s\\nafter=%s", transferSnapshot, afterOutcome)
	}
	if err := assertScanFenceTransferState(ctx, pool, first, "pendiente", 0); err != nil {
		t.Fatal(err)
	}
	beforeRetry, err := scanFenceSnapshot(ctx, pool, first.caseID, first.fileID)
	if err != nil {
		t.Fatalf("capture transfer-first state before stale retry: %v", err)
	}
	if err := outcomeRepo.RecordScanRetry(ctx, first.claim, "scanner_unavailable", time.Minute); !errors.Is(err, dv.ErrConflict) {
		t.Fatalf("transfer-first stale retry error=%v, want ErrConflict", err)
	}
	afterRetry, err := scanFenceSnapshot(ctx, pool, first.caseID, first.fileID)
	if err != nil {
		t.Fatalf("capture transfer-first state after stale retry: %v", err)
	}
	if beforeRetry != afterRetry {
		t.Fatalf("stale retry changed persisted state\\nbefore=%s\\nafter=%s", beforeRetry, afterRetry)
	}

	// Reverse the ordering: hold a successful outcome immediately before commit,
	// prove ownership transfer waits on its space lock, then release the outcome.
	second.claim.CaseRevision = scanFenceCaseRevision(t, ctx, pool, second.caseID)
	verified, err := newWithTransactionBeginner(runtimeTx).VerifyScanClaim(ctx, second.claim)
	if err != nil || !verified {
		t.Fatalf("second VerifyScanClaim=(%t,%v), want true before outcome", verified, err)
	}
	outcomePID := make(chan int32, 1)
	barrier := &scanFenceCommitBarrier{transactions: scanFenceRuntimeBeginner{pool: pool, backendPID: outcomePID}, reached: make(chan struct{}), release: make(chan struct{})}
	defer barrier.unblock()
	outcomeRepo = newWithTransactionBeginner(barrier)
	outcomeDone = make(chan error, 1)
	outcomeWorkerDone := make(chan struct{})
	go func() {
		defer close(outcomeWorkerDone)
		outcomeDone <- outcomeRepo.RecordScanOutcome(ctx, second.claim, ScanOutcome{State: dv.FileClean, ScannerVersion: "synthetic-fence-test", SignaturesAt: time.Now().UTC().Truncate(time.Microsecond)})
	}()
	registerScanFenceWorkerJoin(t, outcomeWorkerDone, &fixtureCleanupSafe, &workerJoinDeadline, "outcome-first scan result")
	select {
	case <-barrier.reached:
	case err := <-outcomeDone:
		t.Fatalf("outcome failed before the pre-commit barrier: %v", err)
	case <-ctx.Done():
		t.Fatal("scan outcome did not reach its pre-commit barrier")
	}
	var outcomeBackendPID int32
	select {
	case outcomeBackendPID = <-outcomePID:
	case <-ctx.Done():
		t.Fatal("scan outcome backend PID was not reported")
	}
	transferPID = make(chan int32, 1)
	transferDone = make(chan error, 1)
	transferWorkerDone = make(chan struct{})
	go func() {
		defer close(transferWorkerDone)
		transferDone <- runScanFenceTransfer(ctx, scanFenceRuntimeBeginner{pool: pool, backendPID: transferPID}, secondSpace, newOwner, nil)
	}()
	registerScanFenceWorkerJoin(t, transferWorkerDone, &fixtureCleanupSafe, &workerJoinDeadline, "outcome-first ownership transfer")
	var transferBackendPID int32
	select {
	case transferBackendPID = <-transferPID:
	case err := <-transferDone:
		t.Fatalf("outcome-first transfer ended before its lock wait: %v", err)
	case <-ctx.Done():
		t.Fatal("outcome-first transfer did not start")
	}
	waitForScanFenceBlocker(t, ctx, pool, appName, transferBackendPID, outcomeBackendPID)
	barrier.unblock()
	if err := <-outcomeDone; err != nil {
		t.Fatalf("commit outcome-first scan result: %v", err)
	}
	if err := <-transferDone; err != nil {
		t.Fatalf("commit outcome-first ownership transfer: %v", err)
	}
	if err := assertScanFenceTransferState(ctx, pool, second, string(dv.FileClean), 1); err != nil {
		t.Fatal(err)
	}
}

type scanFenceFixture struct {
	caseID, fileID, jobID string
	claim                 ScanClaim
	attempt               int16
	leaseUntil            time.Time
}

func seedScanFenceActorsAndSpaces(t *testing.T, ctx context.Context, pool *pgxpool.Pool, oldOwner, newOwner, firstSpace, secondSpace string) {
	t.Helper()
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		t.Fatal("begin actor/space fixture")
	}
	defer tx.Rollback(context.Background())
	for _, owner := range []string{oldOwner, newOwner} {
		if _, err = tx.Exec(ctx, `INSERT INTO public.usuario(id,correo_original,correo_normalizado,hash_clave,estado) VALUES($1,$2,$2,'synthetic','activo')`, owner, owner+"@example.invalid"); err != nil {
			t.Fatalf("insert fixture actor: %v", err)
		}
	}
	// V000005 requires descriptions of at least 100 characters; five repeats
	// produce a valid synthetic space fixture.
	for _, space := range []string{firstSpace, secondSpace} {
		if _, err = tx.Exec(ctx, `INSERT INTO public.espacio(id,propietario_id,categoria_codigo,titulo,descripcion,superficie_m2,capacidad_maxima,reglas_uso,modalidad_tarifa,precio_base_clp,direccion,zona_horaria,estado) VALUES($1,$2,'sala_multiproposito','Synthetic scan fence',repeat('Synthetic description. ',5),30,4,'Synthetic','hora',8000,'Synthetic address','America/Santiago','activa')`, space, oldOwner); err != nil {
			t.Fatalf("insert fixture space: %v", err)
		}
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatalf("commit actors/spaces: %v", err)
	}
}

func newScanFenceFixture(t *testing.T, ctx context.Context, pool *pgxpool.Pool, oldOwner, newOwner, spaceID, label string, fixtureCleanupSafe *bool) scanFenceFixture {
	t.Helper()
	caseID, fileID := uuid.NewString(), uuid.NewString()
	t.Cleanup(func() {
		if !*fixtureCleanupSafe {
			t.Errorf("skipping fixture cleanup for case %s because a controlled goroutine did not stop", caseID)
			return
		}
		cleanupScanFenceFixture(t, pool, oldOwner, newOwner, spaceID, scanFenceFixture{caseID: caseID, fileID: fileID})
	})
	content := []byte("synthetic ownership fence " + label + " " + uuid.NewString())
	hash := sha256.Sum256(content)
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		t.Fatal("begin fixture")
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(ctx, `INSERT INTO public.document_verification_case(id,purpose,subject_id,space_id) VALUES($1,'space',$2,$3)`, caseID, oldOwner, spaceID); err != nil {
		t.Fatal("insert fixture case")
	}
	if _, err = tx.Exec(ctx, `INSERT INTO public.document_verification_file(id,case_id,size_bytes,mime_type,pages,content_sha256) VALUES($1,$2,$3,'image/png',0,$4)`, fileID, caseID, len(content), hash[:]); err != nil {
		t.Fatal("insert fixture file")
	}
	var jobID string
	var lease time.Time
	if err = tx.QueryRow(ctx, `INSERT INTO public.document_verification_job(file_id,kind,state,attempts,available_at,lease_until) VALUES($1,'scan','procesando',1,clock_timestamp(),clock_timestamp()+interval '5 minutes') RETURNING id::text,lease_until`, fileID).Scan(&jobID, &lease); err != nil {
		t.Fatal("insert fixture scan job")
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal("commit fixture")
	}
	claim := ScanClaim{JobID: jobID, FileID: fileID, CaseID: caseID, SubjectID: oldOwner, Purpose: dv.PurposeSpace, SpaceID: spaceID, Attempt: 1, LeaseUntil: lease, CaseRevision: 1}
	return scanFenceFixture{caseID: caseID, fileID: fileID, jobID: jobID, claim: claim, attempt: 1, leaseUntil: lease}
}

func registerScanFenceWorkerJoin(t *testing.T, done <-chan struct{}, fixtureCleanupSafe *bool, deadline *time.Time, name string) {
	t.Helper()
	t.Cleanup(func() {
		if deadline.IsZero() {
			*deadline = time.Now().Add(5 * time.Second)
		}
		remaining := time.Until(*deadline)
		if remaining <= 0 {
			*fixtureCleanupSafe = false
			t.Errorf("controlled %s goroutine did not finish before the shared cleanup deadline; fixture cleanup is unsafe", name)
			return
		}
		timer := time.NewTimer(remaining)
		defer timer.Stop()
		select {
		case <-done:
		case <-timer.C:
			*fixtureCleanupSafe = false
			t.Errorf("controlled %s goroutine did not finish; fixture cleanup is unsafe", name)
		}
	})
}

func requireScanFenceIsolation(ctx context.Context, pool *pgxpool.Pool, app string) error {
	var scans, retentions, runtimes int
	err := pool.QueryRow(ctx, `SELECT count(*) FILTER(WHERE kind='scan'),count(*) FILTER(WHERE kind='retention_delete') FROM public.document_verification_job WHERE state IN ('pendiente','reintentar','procesando')`).Scan(&scans, &retentions)
	if err != nil || scans != 0 || retentions != 0 {
		return fmt.Errorf("pre-existing active scan/retention work: scan=%d retention=%d err=%v", scans, retentions, err)
	}
	var activeWorkers int
	err = pool.QueryRow(ctx, `SELECT count(*) FROM pg_catalog.pg_stat_activity WHERE datname=current_database() AND pid<>pg_backend_pid() AND state='active' AND lower(query) LIKE '%document_verification_job%' AND (lower(query) LIKE '%scan%' OR lower(query) LIKE '%retention%')`).Scan(&activeWorkers)
	if err != nil || activeWorkers != 0 {
		return fmt.Errorf("pre-existing active scan/retention database sessions: count=%d err=%v", activeWorkers, err)
	}
	err = pool.QueryRow(ctx, `SELECT count(*) FROM pg_catalog.pg_stat_activity WHERE datname=current_database() AND pid<>pg_backend_pid() AND application_name<>$1 AND usename IS NOT NULL AND pg_catalog.pg_has_role(usename,'espacigo_runtime','MEMBER')`, app).Scan(&runtimes)
	if err != nil || runtimes != 0 {
		return fmt.Errorf("runtime-capable sessions exist: count=%d err=%v", runtimes, err)
	}
	return nil
}

type scanFenceCommitBarrier struct {
	transactions     transactionBeginner
	reached, release chan struct{}
	releaseOnce      sync.Once
}

func (b *scanFenceCommitBarrier) unblock() {
	b.releaseOnce.Do(func() { close(b.release) })
}

func (b *scanFenceCommitBarrier) BeginTx(ctx context.Context, opts pgx.TxOptions) (pgx.Tx, error) {
	tx, err := b.transactions.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	return &scanFenceTx{Tx: tx, barrier: b}, nil
}

type scanFenceTx struct {
	pgx.Tx
	barrier *scanFenceCommitBarrier
}

func (t *scanFenceTx) Commit(ctx context.Context) error {
	select {
	case <-t.barrier.reached:
	default:
		close(t.barrier.reached)
	}
	select {
	case <-t.barrier.release:
	case <-ctx.Done():
		return ctx.Err()
	}
	return t.Tx.Commit(ctx)
}

type scanFenceRuntimeBeginner struct {
	pool       *pgxpool.Pool
	backendPID chan<- int32
}

func (b scanFenceRuntimeBeginner) BeginTx(ctx context.Context, opts pgx.TxOptions) (pgx.Tx, error) {
	tx, err := b.pool.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `SET LOCAL ROLE espacigo_runtime`); err != nil {
		rollbackErr := tx.Rollback(context.Background())
		if rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) {
			err = errors.Join(err, rollbackErr)
		}
		return nil, err
	}
	var role string
	var superuser, bypassRLS bool
	if err := tx.QueryRow(ctx, `SELECT current_user,r.rolsuper,r.rolbypassrls FROM pg_catalog.pg_roles r WHERE r.rolname=current_user`).Scan(&role, &superuser, &bypassRLS); err != nil || role != "espacigo_runtime" || superuser || bypassRLS {
		rollbackErr := tx.Rollback(context.Background())
		if rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) {
			err = errors.Join(err, rollbackErr)
		}
		if err == nil {
			err = errors.New("scan-fence transaction is not using the restricted runtime role")
		}
		return nil, err
	}
	if b.backendPID != nil {
		var pid int32
		if err := tx.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pid); err != nil {
			rollbackErr := tx.Rollback(context.Background())
			if rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) {
				err = errors.Join(err, rollbackErr)
			}
			return nil, err
		}
		select {
		case b.backendPID <- pid:
		case <-ctx.Done():
			rollbackErr := tx.Rollback(context.Background())
			if rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) {
				return nil, errors.Join(ctx.Err(), rollbackErr)
			}
			return nil, ctx.Err()
		}
	}
	return tx, nil
}

func runScanFenceTransfer(ctx context.Context, beginner scanFenceRuntimeBeginner, spaceID, owner string, afterUpdate func(pgx.Tx) error) error {
	tx, err := beginner.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin ownership transfer: %w", err)
	}
	defer tx.Rollback(context.Background())
	if _, err := tx.Exec(ctx, `UPDATE public.espacio SET propietario_id=$2,actualizado_en=clock_timestamp() WHERE id=$1`, spaceID, owner); err != nil {
		return fmt.Errorf("transfer owner: %w", err)
	}
	if afterUpdate != nil {
		if err := afterUpdate(tx); err != nil {
			return fmt.Errorf("post-update transfer barrier: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit owner transfer: %w", err)
	}
	return nil
}

func waitForScanFenceBlocker(t *testing.T, ctx context.Context, pool *pgxpool.Pool, appName string, waiterPID, blockerPID int32) {
	t.Helper()
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var blocked bool
		err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_catalog.pg_stat_activity a WHERE a.pid=$1 AND a.application_name=$2 AND a.state='active' AND a.wait_event_type='Lock' AND $3=ANY(pg_catalog.pg_blocking_pids(a.pid)))`, waiterPID, appName, blockerPID).Scan(&blocked)
		if err != nil {
			t.Fatalf("inspect PostgreSQL lock wait: %v", err)
		}
		if blocked {
			return
		}
		select {
		case <-ticker.C:
		case <-deadline.C:
			t.Fatalf("backend %d was not observed waiting on space fence held by backend %d", waiterPID, blockerPID)
		case <-ctx.Done():
			t.Fatalf("wait for PostgreSQL space-fence lock wait: %v", ctx.Err())
		}
	}
}

type scanFenceQueryRower interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func scanFenceSnapshot(ctx context.Context, query scanFenceQueryRower, caseID, fileID string) (string, error) {
	var snapshot string
	err := query.QueryRow(ctx, `SELECT jsonb_build_object(
		'case',to_jsonb(c),'file',to_jsonb(f),
		'jobs',(SELECT COALESCE(jsonb_agg(to_jsonb(j) ORDER BY j.id),'[]'::jsonb) FROM public.document_verification_job j WHERE j.file_id=f.id),
		'events',(SELECT COALESCE(jsonb_agg(to_jsonb(e) ORDER BY e.id),'[]'::jsonb) FROM public.document_verification_event e WHERE e.case_id=c.id)
	)::text FROM public.document_verification_case c JOIN public.document_verification_file f ON f.case_id=c.id WHERE c.id=$1 AND f.id=$2`, caseID, fileID).Scan(&snapshot)
	return snapshot, err
}

func assertScanFenceTransferState(ctx context.Context, pool *pgxpool.Pool, f scanFenceFixture, wantFileState string, wantScanResults int) error {
	var fileState, lastError, reason, scanJobState, retentionJobState string
	var retentionDeadline *time.Time
	var retentionAvailable time.Time
	var superseded bool
	var scanJobs, retentionJobs, events, supersessionEvents, scanResults int
	err := pool.QueryRow(ctx, `SELECT f.state,COALESCE(f.last_error_code,''),COALESCE(f.retention_reason_code,''),f.retention_at,s.state,r.state,r.available_at,c.superseded_at IS NOT NULL,
		(SELECT count(*) FROM public.document_verification_job j WHERE j.file_id=f.id AND j.kind='scan'),
		(SELECT count(*) FROM public.document_verification_job j WHERE j.file_id=f.id AND j.kind='retention_delete'),
		(SELECT count(*) FROM public.document_verification_event e WHERE e.case_id=c.id),
		(SELECT count(*) FROM public.document_verification_event e WHERE e.case_id=c.id AND e.action='case_superseded'),
		(SELECT count(*) FROM public.document_verification_event e WHERE e.case_id=c.id AND e.action='scan_result' AND e.result_code=$2)
		FROM public.document_verification_file f
		JOIN public.document_verification_case c ON c.id=f.case_id
		JOIN public.document_verification_job s ON s.file_id=f.id AND s.kind='scan'
		JOIN public.document_verification_job r ON r.file_id=f.id AND r.kind='retention_delete'
		WHERE f.id=$1`, f.fileID, string(dv.FileClean)).Scan(&fileState, &lastError, &reason, &nullablePT{v: &retentionDeadline}, &scanJobState, &retentionJobState, &retentionAvailable, &superseded, &scanJobs, &retentionJobs, &events, &supersessionEvents, &scanResults)
	if err != nil {
		return err
	}
	if retentionDeadline == nil {
		return errors.New("ownership transfer did not persist a retention deadline")
	}
	now := time.Now().UTC()
	if fileState != wantFileState || lastError != "" || reason != "rejected_or_replaced" || scanJobState != "completado" || retentionJobState != "pendiente" || !superseded || scanJobs != 1 || retentionJobs != 1 || events != wantScanResults+1 || supersessionEvents != 1 || scanResults != wantScanResults || retentionDeadline.Before(now.Add(29*24*time.Hour)) || retentionDeadline.After(now.Add(31*24*time.Hour)) || !retentionDeadline.Equal(retentionAvailable) {
		return fmt.Errorf("ownership-transfer state mismatch: file=%s last_error=%q reason=%q retention_at=%s retention_available=%s scan_job=%s retention_job=%s superseded=%t scan_jobs=%d retention_jobs=%d events=%d supersessions=%d scan_results=%d", fileState, lastError, reason, retentionDeadline, retentionAvailable, scanJobState, retentionJobState, superseded, scanJobs, retentionJobs, events, supersessionEvents, scanResults)
	}
	return nil
}
func scanFenceCaseRevision(t *testing.T, ctx context.Context, p *pgxpool.Pool, id string) int64 {
	t.Helper()
	var revision int64
	if err := p.QueryRow(ctx, `SELECT revision FROM public.document_verification_case WHERE id=$1`, id).Scan(&revision); err != nil {
		t.Fatal(err)
	}
	return revision
}
func cleanupScanFenceFixture(t *testing.T, pool *pgxpool.Pool, ownerA, ownerB, spaceID string, f scanFenceFixture) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// Keep ownership on the fixture's original subject so FK/case cleanup is deterministic.
	if _, err := pool.Exec(ctx, `UPDATE public.espacio SET propietario_id=$2 WHERE id=$1`, spaceID, ownerA); err != nil {
		t.Errorf("restore synthetic space owner before cleanup %s: %v", spaceID, err)
	}
	for _, stmt := range []struct {
		sql  string
		args []any
	}{
		{`DELETE FROM public.document_verification_job WHERE file_id=$1`, []any{f.fileID}},
		{`DELETE FROM public.document_verification_file WHERE id=$1`, []any{f.fileID}},
		{`DELETE FROM public.document_verification_case WHERE id=$1`, []any{f.caseID}},
	} {
		if _, err := pool.Exec(ctx, stmt.sql, stmt.args...); err != nil {
			t.Errorf("cleanup fixture rows %s: %v", f.caseID, err)
			return
		}
	}
	// The schema makes audit events append-only (V79); retain only these uniquely
	// keyed synthetic audit records while deleting every mutable fixture row.
	var remaining int
	if err := pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM public.document_verification_case WHERE id=$1)+(SELECT count(*) FROM public.document_verification_file WHERE id=$2)+(SELECT count(*) FROM public.document_verification_job WHERE file_id=$2)`, f.caseID, f.fileID).Scan(&remaining); err != nil || remaining != 0 {
		t.Errorf("fixture cleanup read-back case=%s mutable rows remaining=%d err=%v", f.caseID, remaining, err)
	}
	var auditRows int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM public.document_verification_event WHERE case_id=$1`, f.caseID).Scan(&auditRows); err != nil {
		t.Errorf("read back append-only synthetic audit rows for case=%s: %v", f.caseID, err)
	} else {
		t.Logf("retained %d append-only synthetic audit rows for case %s", auditRows, f.caseID)
	}
	_ = ownerB
}

func cleanupScanFenceActorsAndSpaces(t *testing.T, pool *pgxpool.Pool, oldOwner, newOwner, firstSpace, secondSpace string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, space := range []string{firstSpace, secondSpace} {
		if _, err := pool.Exec(ctx, `DELETE FROM public.espacio WHERE id=$1`, space); err != nil {
			t.Errorf("cleanup fixture space %s: %v", space, err)
		}
	}
	for _, owner := range []string{oldOwner, newOwner} {
		if _, err := pool.Exec(ctx, `DELETE FROM public.usuario WHERE id=$1`, owner); err != nil {
			t.Errorf("cleanup fixture actor %s: %v", owner, err)
		}
	}
	var remaining int
	if err := pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM public.espacio WHERE id IN($1,$2))+(SELECT count(*) FROM public.usuario WHERE id IN($3,$4))`, firstSpace, secondSpace, oldOwner, newOwner).Scan(&remaining); err != nil || remaining != 0 {
		t.Errorf("fixture actor/space cleanup read-back remaining=%d err=%v", remaining, err)
	}
}

var _ pgx.Tx = (*scanFenceTx)(nil)
