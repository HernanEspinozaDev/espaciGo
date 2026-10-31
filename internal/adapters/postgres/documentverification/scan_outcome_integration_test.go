package documentverificationpg

import (
	"context"
	"crypto/sha256"
	"os"
	"testing"
	"time"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Gated PostgreSQL regression test for the persisted scan-result audit event.
// The fixture is rolled back; this test never applies migrations.
func TestPostgresScanOutcomeAuditsCaseStateAndFileResult(t *testing.T) {
	dsn := os.Getenv("ESPACIGO_DOCUMENT_VERIFICATION_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ESPACIGO_DOCUMENT_VERIFICATION_TEST_DATABASE_URL is unset")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal("invalid integration DSN")
	}
	if cfg.ConnConfig.Host != "127.0.0.1" || cfg.ConnConfig.Port != 15433 || cfg.ConnConfig.Database != "espacigo_f5_synth_20261029" {
		t.Fatal("refusing integration target outside the authorized F5 synthetic endpoint")
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal("unable to connect to the authorized F5 synthetic target")
	}
	t.Cleanup(func() { pool.Close() })
	var database string
	if err := pool.QueryRow(ctx, `SELECT current_database()`).Scan(&database); err != nil || database != "espacigo_f5_synth_20261029" {
		t.Fatal("refusing unexpected integration database")
	}

	outer, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		t.Fatal("unable to begin rollback-scoped fixture")
	}
	actor, caseID, fileID, infectedFileID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	t.Cleanup(func() {
		if err := outer.Rollback(context.Background()); err != nil && err != pgx.ErrTxClosed {
			t.Errorf("rollback scan fixture failed: %v", err)
		}
		var remaining int
		if err := pool.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM public.usuario WHERE id=$1)+(SELECT count(*) FROM public.document_verification_case WHERE id=$2)+(SELECT count(*) FROM public.document_verification_file WHERE id IN($3,$4))+(SELECT count(*) FROM public.document_verification_job WHERE file_id IN($3,$4))+(SELECT count(*) FROM public.document_verification_event WHERE case_id=$2)`, actor, caseID, fileID, infectedFileID).Scan(&remaining); err != nil {
			t.Errorf("verify scan fixture rollback failed: %v", err)
		} else if remaining != 0 {
			t.Errorf("scan fixture rollback left %d rows", remaining)
		}
	})
	if _, err := outer.Exec(ctx, `INSERT INTO public.usuario(id,correo_original,correo_normalizado,hash_clave,estado) VALUES($1,$2,$2,'synthetic','activo')`, actor, actor+"@example.invalid"); err != nil {
		t.Fatal("unable to seed synthetic scan actor")
	}
	if _, err := outer.Exec(ctx, `INSERT INTO public.document_verification_case(id,purpose,subject_id) VALUES($1,'identity',$2)`, caseID, actor); err != nil {
		t.Fatal("unable to seed synthetic identity case")
	}
	content := []byte("synthetic")
	hash := sha256.Sum256(content)
	if _, err := outer.Exec(ctx, `INSERT INTO public.document_verification_file(id,case_id,size_bytes,mime_type,pages,content_sha256) VALUES($1,$2,$3,'image/png',0,$4)`, fileID, caseID, len(content), hash[:]); err != nil {
		t.Fatal("unable to seed synthetic document metadata")
	}
	var jobID string
	var leaseUntil time.Time
	if err := outer.QueryRow(ctx, `INSERT INTO public.document_verification_job(file_id,kind,state,attempts,available_at,lease_until) VALUES($1,'scan','procesando',1,clock_timestamp(),clock_timestamp()+interval '2 minutes') RETURNING id::text,lease_until`, fileID).Scan(&jobID, &leaseUntil); err != nil {
		t.Fatal("unable to seed synthetic scan lease")
	}
	if _, err := outer.Exec(ctx, `INSERT INTO public.document_verification_file(id,case_id,size_bytes,mime_type,pages,content_sha256) VALUES($1,$2,$3,'image/png',0,$4)`, infectedFileID, caseID, len(content), hash[:]); err != nil {
		t.Fatal("unable to seed synthetic infected document metadata")
	}
	var infectedJobID string
	var infectedLeaseUntil time.Time
	if err := outer.QueryRow(ctx, `INSERT INTO public.document_verification_job(file_id,kind,state,attempts,available_at,lease_until) VALUES($1,'scan','procesando',1,clock_timestamp(),clock_timestamp()+interval '2 minutes') RETURNING id::text,lease_until`, infectedFileID).Scan(&infectedJobID, &infectedLeaseUntil); err != nil {
		t.Fatal("unable to seed synthetic infected scan lease")
	}
	var infectedRetentionAvailable, infectedRetentionLease time.Time
	if err := outer.QueryRow(ctx, `INSERT INTO public.document_verification_job(file_id,kind,state,attempts,available_at,lease_until,last_error_code) VALUES($1,'retention_delete','procesando',3,clock_timestamp()-interval '1 second',clock_timestamp()+interval '2 minutes','retention_store_error') RETURNING available_at,lease_until`, infectedFileID).Scan(&infectedRetentionAvailable, &infectedRetentionLease); err != nil {
		t.Fatal("unable to seed active synthetic retention lease")
	}
	if _, err := outer.Exec(ctx, `SET LOCAL ROLE espacigo_runtime`); err != nil {
		t.Fatal("unable to assume restricted runtime role")
	}

	repo := newWithTransactionBeginner(savepointBeginner{tx: outer})
	claim := ScanClaim{JobID: jobID, FileID: fileID, CaseID: caseID, LeaseUntil: leaseUntil, Attempt: 1}
	cleanSignaturesAt := time.Now().UTC().Truncate(time.Microsecond)
	outcome := ScanOutcome{State: dv.FileClean, ScannerVersion: "ClamAV synthetic", SignaturesAt: cleanSignaturesAt}
	if err := repo.RecordScanOutcome(ctx, claim, outcome); err != nil {
		t.Fatalf("persist clean scan outcome under runtime role: %v", err)
	}
	var action, fromState, toState, resultCode string
	if err := outer.QueryRow(ctx, `SELECT action,from_state,to_state,result_code FROM public.document_verification_event WHERE case_id=$1 AND file_id=$2 AND action='scan_result'`, caseID, fileID).Scan(&action, &fromState, &toState, &resultCode); err != nil {
		t.Fatal("scan-result audit event was not persisted")
	}
	if action != "scan_result" || fromState != string(dv.CaseDraft) || toState != string(dv.CaseDraft) || resultCode != string(dv.FileClean) {
		t.Fatalf("unexpected scan audit event: action=%q from=%q to=%q result=%q", action, fromState, toState, resultCode)
	}
	var fileState, lastError, scannerVersion, retentionReason string
	var storedSignaturesAt time.Time
	var inspected, hasRetention bool
	if err := outer.QueryRow(ctx, `SELECT state,COALESCE(last_error_code,''),scanner_version,signatures_at,inspected_at IS NOT NULL,retention_at IS NOT NULL,COALESCE(retention_reason_code,'') FROM public.document_verification_file WHERE id=$1`, fileID).Scan(&fileState, &lastError, &scannerVersion, &storedSignaturesAt, &inspected, &hasRetention, &retentionReason); err != nil {
		t.Fatal("clean scan file metadata was not persisted")
	}
	if fileState != string(dv.FileClean) || lastError != "" || scannerVersion != outcome.ScannerVersion || !storedSignaturesAt.Equal(cleanSignaturesAt) || !inspected || hasRetention || retentionReason != "" {
		t.Fatalf("unexpected clean scan file metadata: state=%q error=%q scanner=%q inspected=%t retention=%t reason=%q", fileState, lastError, scannerVersion, inspected, hasRetention, retentionReason)
	}
	var jobState, jobError string
	var leaseCleared bool
	if err := outer.QueryRow(ctx, `SELECT state,lease_until IS NULL,COALESCE(last_error_code,'') FROM public.document_verification_job WHERE id=$1`, jobID).Scan(&jobState, &leaseCleared, &jobError); err != nil {
		t.Fatal("clean scan job state was not persisted")
	}
	if jobState != "completado" || !leaseCleared || jobError != "" {
		t.Fatalf("unexpected clean scan job state: state=%q lease_cleared=%t error=%q", jobState, leaseCleared, jobError)
	}

	infectedOutcome := ScanOutcome{State: dv.FileBlocked, ErrorCode: "malware_detected", ReasonCode: dv.ReasonMalwareDetected, ScannerVersion: "ClamAV synthetic", SignaturesAt: time.Now().UTC().Truncate(time.Microsecond)}
	infectedClaim := ScanClaim{JobID: infectedJobID, FileID: infectedFileID, CaseID: caseID, LeaseUntil: infectedLeaseUntil, Attempt: 1}
	if err := repo.RecordScanOutcome(ctx, infectedClaim, infectedOutcome); err != nil {
		t.Fatalf("persist infected scan outcome under runtime role: %v", err)
	}
	var infectedState, infectedError, infectedScanner, infectedRetentionReason string
	var infectedSignaturesAt time.Time
	var infectedInspected, retainedForMalwareWindow bool
	if err := outer.QueryRow(ctx, `SELECT state,COALESCE(last_error_code,''),scanner_version,signatures_at,inspected_at IS NOT NULL,retention_at>=clock_timestamp()+interval '23 hours' AND retention_at<=clock_timestamp()+interval '25 hours',COALESCE(retention_reason_code,'') FROM public.document_verification_file WHERE id=$1`, infectedFileID).Scan(&infectedState, &infectedError, &infectedScanner, &infectedSignaturesAt, &infectedInspected, &retainedForMalwareWindow, &infectedRetentionReason); err != nil {
		t.Fatal("infected scan file metadata was not persisted")
	}
	if infectedState != string(dv.FileBlocked) || infectedError != "malware_detected" || infectedScanner != infectedOutcome.ScannerVersion || !infectedSignaturesAt.Equal(infectedOutcome.SignaturesAt) || !infectedInspected || !retainedForMalwareWindow || infectedRetentionReason != "malicious_file" {
		t.Fatalf("unexpected infected scan file metadata: state=%q error=%q scanner=%q inspected=%t retention_window=%t reason=%q", infectedState, infectedError, infectedScanner, infectedInspected, retainedForMalwareWindow, infectedRetentionReason)
	}
	var retentionState, retentionError string
	var retentionAttempts int16
	var retentionAvailable time.Time
	var retentionLeaseUntil *time.Time
	if err := outer.QueryRow(ctx, `SELECT state,attempts,available_at,lease_until,COALESCE(last_error_code,'') FROM public.document_verification_job WHERE file_id=$1 AND kind='retention_delete'`, infectedFileID).Scan(&retentionState, &retentionAttempts, &retentionAvailable, &retentionLeaseUntil, &retentionError); err != nil {
		t.Fatal("active retention lease state was not preserved")
	}
	if retentionState != "procesando" || retentionAttempts != 3 || !retentionAvailable.Equal(infectedRetentionAvailable) || retentionLeaseUntil == nil || !retentionLeaseUntil.Equal(infectedRetentionLease) || retentionError != "retention_store_error" {
		t.Fatalf("scan scheduling overwrote active retention work: state=%q attempts=%d available=%v lease=%v error=%q", retentionState, retentionAttempts, retentionAvailable, retentionLeaseUntil, retentionError)
	}
	if err := outer.QueryRow(ctx, `SELECT state,lease_until IS NULL,COALESCE(last_error_code,'') FROM public.document_verification_job WHERE id=$1`, infectedJobID).Scan(&jobState, &leaseCleared, &jobError); err != nil {
		t.Fatal("infected scan job state was not persisted")
	}
	if jobState != "completado" || !leaseCleared || jobError != "malware_detected" {
		t.Fatalf("unexpected infected scan job state: state=%q lease_cleared=%t error=%q", jobState, leaseCleared, jobError)
	}
	var infectedAction, infectedFromState, infectedToState, infectedResult, infectedReason string
	if err := outer.QueryRow(ctx, `SELECT action,from_state,to_state,result_code,COALESCE(reason_code,'') FROM public.document_verification_event WHERE case_id=$1 AND file_id=$2 AND action='scan_result'`, caseID, infectedFileID).Scan(&infectedAction, &infectedFromState, &infectedToState, &infectedResult, &infectedReason); err != nil {
		t.Fatal("infected scan-result audit event was not persisted")
	}
	if infectedAction != "scan_result" || infectedFromState != string(dv.CaseDraft) || infectedToState != string(dv.CaseDraft) || infectedResult != string(dv.FileBlocked) || infectedReason != string(dv.ReasonMalwareDetected) {
		t.Fatalf("unexpected infected scan audit event: action=%q from=%q to=%q result=%q reason=%q", infectedAction, infectedFromState, infectedToState, infectedResult, infectedReason)
	}
}
