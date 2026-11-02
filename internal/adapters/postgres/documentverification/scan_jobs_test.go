package documentverificationpg

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Gated integration coverage. Requires the parent's authorized F5 target with V81 helpers;
// this suite never applies migrations or creates persistent accounts.
func TestPostgresDurableScanJobs(t *testing.T) {
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
	if cfg.ConnConfig.Host != "127.0.0.1" || cfg.ConnConfig.Port != 15434 || cfg.ConnConfig.Database != "espacigo_f5_gate_20261101" {
		t.Fatal("refusing integration target outside authorized F5 synthetic endpoint")
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	var db, setupRole string
	if err := pool.QueryRow(ctx, `SELECT current_database(),current_user`).Scan(&db, &setupRole); err != nil || db != "espacigo_f5_gate_20261101" {
		t.Fatalf("unexpected integration target %q: %v", db, err)
	}
	if setupRole == "espacigo_runtime" {
		t.Fatal("refusing to seed scan fixtures through the runtime role")
	}
	var preexistingScanJobs, runtimeCapableSessions int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM public.document_verification_job WHERE kind='scan' AND state IN ('pendiente','reintentar','procesando')`).Scan(&preexistingScanJobs); err != nil || preexistingScanJobs != 0 {
		t.Fatalf("refusing shared scan-queue integration with preexisting eligible/active work: jobs=%d err=%v", preexistingScanJobs, err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM pg_catalog.pg_stat_activity WHERE datname=current_database() AND pid<>pg_backend_pid() AND usename IS NOT NULL AND pg_catalog.pg_has_role(usename,'espacigo_runtime','MEMBER')`).Scan(&runtimeCapableSessions); err != nil || runtimeCapableSessions != 0 {
		t.Fatalf("refusing shared scan-queue integration while runtime-capable sessions are connected: sessions=%d err=%v", runtimeCapableSessions, err)
	}
	outer, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		t.Fatal(err)
	}
	lockF5JobQueueForTest(t, ctx, outer, "scan", "")
	ownerID, caseID, fileID, malwareFileID, spaceID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	staleSubjectID, staleCaseID, staleFileID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	t.Cleanup(func() {
		if err := outer.Rollback(context.Background()); err != nil && err != pgx.ErrTxClosed {
			t.Errorf("rollback scan fixtures: %v", err)
		}
		var remaining int
		if err := pool.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM public.usuario WHERE id IN($1,$6))+(SELECT count(*) FROM public.espacio WHERE id=$2)+(SELECT count(*) FROM public.tarifa_espacio WHERE espacio_id=$2)+(SELECT count(*) FROM public.espacio_caracteristicas WHERE espacio_id=$2)+(SELECT count(*) FROM public.document_verification_case WHERE id IN($3,$7))+(SELECT count(*) FROM public.document_verification_file WHERE id IN($4,$5,$8))+(SELECT count(*) FROM public.document_verification_job WHERE file_id IN($4,$5,$8))+(SELECT count(*) FROM public.document_verification_event WHERE case_id IN($3,$7))`, ownerID, spaceID, caseID, fileID, malwareFileID, staleSubjectID, staleCaseID, staleFileID).Scan(&remaining); err != nil {
			t.Errorf("verify scan fixture rollback: %v", err)
		} else if remaining != 0 {
			t.Errorf("scan fixture rollback left %d rows", remaining)
		}
	})
	if _, err := outer.Exec(ctx, `INSERT INTO public.usuario(id,correo_original,correo_normalizado,hash_clave,estado) VALUES($1,$2,$2,'synthetic','activo')`, ownerID, ownerID+"@example.invalid"); err != nil {
		t.Fatal(err)
	}
	if _, err := outer.Exec(ctx, `INSERT INTO public.espacio(id,propietario_id,categoria_codigo,titulo,descripcion,superficie_m2,capacidad_maxima,reglas_uso,modalidad_tarifa,precio_base_clp,direccion,zona_horaria,estado) VALUES($1,$2,'sala_multiproposito','F5 scan fixture',repeat('Synthetic scan description. ',4),30,4,'Synthetic','hora',8000,'Synthetic address','America/Santiago','activa')`, spaceID, ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err := outer.Exec(ctx, `INSERT INTO public.tarifa_espacio(espacio_id,version,modalidad,precio_base_clp) VALUES($1,1,'hora',8000)`, spaceID); err != nil {
		t.Fatal(err)
	}
	if _, err := outer.Exec(ctx, `INSERT INTO public.espacio_caracteristicas(espacio_id,categoria_codigo,perfil_version,valores) VALUES($1,'sala_multiproposito',1,'{}'::jsonb)`, spaceID); err != nil {
		t.Fatal(err)
	}
	if _, err := outer.Exec(ctx, `INSERT INTO public.document_verification_case(id,purpose,subject_id,space_id) VALUES($1,'space',$2,$3)`, caseID, ownerID, spaceID); err != nil {
		t.Fatal(err)
	}
	hash := [32]byte{1}
	if _, err := outer.Exec(ctx, `INSERT INTO public.document_verification_file(id,case_id,size_bytes,mime_type,pages,content_sha256) VALUES($1,$2,128,'image/png',1,$3)`, fileID, caseID, hash[:]); err != nil {
		t.Fatal(err)
	}
	if _, err := outer.Exec(ctx, `INSERT INTO public.document_verification_job(file_id,kind) VALUES($1,'scan')`, fileID); err != nil {
		t.Fatal(err)
	}
	if _, err := outer.Exec(ctx, `INSERT INTO public.usuario(id,correo_original,correo_normalizado,hash_clave,estado) VALUES($1,$2,$2,'synthetic','activo')`, staleSubjectID, staleSubjectID+"@example.invalid"); err != nil {
		t.Fatal(err)
	}
	if _, err := outer.Exec(ctx, `INSERT INTO public.document_verification_case(id,purpose,subject_id,space_id) VALUES($1,'space',$2,$3)`, staleCaseID, staleSubjectID, spaceID); err != nil {
		t.Fatal(err)
	}
	staleHash := [32]byte{3}
	if _, err := outer.Exec(ctx, `INSERT INTO public.document_verification_file(id,case_id,size_bytes,mime_type,pages,content_sha256) VALUES($1,$2,128,'image/png',1,$3)`, staleFileID, staleCaseID, staleHash[:]); err != nil {
		t.Fatal(err)
	}
	if _, err := outer.Exec(ctx, `INSERT INTO public.document_verification_job(file_id,kind,available_at) VALUES($1,'scan',clock_timestamp()-interval '1 minute')`, staleFileID); err != nil {
		t.Fatal(err)
	}
	var queueJobs, fixtureJobs int
	if err := outer.QueryRow(ctx, `SELECT count(*),count(*) FILTER (WHERE file_id=$1) FROM public.document_verification_job WHERE kind='scan' AND state IN ('pendiente','reintentar','procesando')`, fileID).Scan(&queueJobs, &fixtureJobs); err != nil || queueJobs != 2 || fixtureJobs != 1 {
		t.Fatalf("scan queue lost fixture isolation before claim: total=%d fixture=%d err=%v", queueJobs, fixtureJobs, err)
	}
	if _, err := outer.Exec(ctx, `SET LOCAL ROLE espacigo_runtime`); err != nil {
		t.Fatal("unable to assume restricted runtime role")
	}
	var currentRole string
	var superuser, bypassRLS bool
	if err := outer.QueryRow(ctx, `SELECT current_user,r.rolsuper,r.rolbypassrls FROM pg_catalog.pg_roles r WHERE r.rolname=current_user`).Scan(&currentRole, &superuser, &bypassRLS); err != nil {
		t.Fatal(err)
	}
	if currentRole != "espacigo_runtime" || superuser || bypassRLS {
		t.Fatalf("scan repository role=%q superuser=%t bypassRLS=%t", currentRole, superuser, bypassRLS)
	}
	repo := newWithTransactionBeginner(savepointBeginner{tx: outer})
	claim, err := repo.ClaimReadyScanJob(ctx, time.Minute)
	if err != nil {
		t.Fatalf("claim scan job under runtime role: %v", err)
	}
	if claim == nil || claim.FileID != fileID || claim.CaseID != caseID || claim.ContentSHA256 != hash || claim.Attempt != 1 || !claim.LeaseUntil.After(time.Now()) {
		t.Fatalf("unexpected scan claim: %+v", claim)
	}
	if err := repo.RecordScanRetry(ctx, *claim, "scanner_unavailable", 0); err != nil {
		t.Fatalf("persist retry under runtime role: %v", err)
	}
	var firstRetryFileState, firstRetryJobState, firstRetryReason, firstRetentionJobState string
	var firstRetryAttempts int16
	var firstRetryRetention, firstRetentionScheduled time.Time
	if err := outer.QueryRow(ctx, `SELECT f.state,j.state,j.attempts,COALESCE(f.retention_reason_code,''),f.retention_at,COALESCE(r.state,''),COALESCE(r.available_at,'epoch'::timestamptz) FROM public.document_verification_file f JOIN public.document_verification_job j ON j.file_id=f.id AND j.kind='scan' LEFT JOIN public.document_verification_job r ON r.file_id=f.id AND r.kind='retention_delete' AND r.state IN ('pendiente','procesando','reintentar') WHERE f.id=$1`, fileID).Scan(&firstRetryFileState, &firstRetryJobState, &firstRetryAttempts, &firstRetryReason, &firstRetryRetention, &firstRetentionJobState, &firstRetentionScheduled); err != nil {
		t.Fatal(err)
	}
	if firstRetryFileState != string(dv.FileBlocked) || firstRetryJobState != "reintentar" || firstRetryAttempts != 1 || firstRetryReason != "inspection_error" || firstRetryRetention.Before(time.Now().UTC().Add(7*24*time.Hour-time.Minute)) || firstRetryRetention.After(time.Now().UTC().Add(7*24*time.Hour+time.Minute)) || firstRetentionJobState != "pendiente" || !firstRetentionScheduled.Equal(firstRetryRetention) {
		t.Fatalf("retry retention was not persisted consistently: file=%q scan_job=%q attempts=%d reason=%q retention=%v retention_job=%q scheduled=%v", firstRetryFileState, firstRetryJobState, firstRetryAttempts, firstRetryReason, firstRetryRetention, firstRetentionJobState, firstRetentionScheduled)
	}
	retryClaim, err := repo.ClaimReadyScanJob(ctx, time.Minute)
	if err != nil || retryClaim == nil || retryClaim.FileID != fileID || retryClaim.Attempt != 2 || !retryClaim.LeaseUntil.After(time.Now()) {
		t.Fatalf("blocked retry was not reclaimable: claim=%+v err=%v", retryClaim, err)
	}
	if current, verifyErr := repo.VerifyScanClaim(ctx, *retryClaim); verifyErr != nil || !current {
		t.Fatalf("retained deadline incorrectly fenced active retry scan: current=%t err=%v", current, verifyErr)
	}
	var retryClaimRetention *time.Time
	var retryClaimReason string
	if err := outer.QueryRow(ctx, `SELECT retention_at,COALESCE(retention_reason_code,'') FROM public.document_verification_file WHERE id=$1`, fileID).Scan(&nullablePT{v: &retryClaimRetention}, &retryClaimReason); err != nil {
		t.Fatal("unable to inspect retained deadline during active retry scan")
	}
	if retryClaimRetention == nil || !retryClaimRetention.Equal(firstRetryRetention) || retryClaimReason != "inspection_error" {
		t.Fatalf("active retry scan discarded transfer-visible retention: deadline=%v reason=%q want_deadline=%v reason=inspection_error", retryClaimRetention, retryClaimReason, firstRetryRetention)
	}
	var activeRetentionJobs int
	if err := outer.QueryRow(ctx, `SELECT count(*) FROM public.document_verification_job WHERE file_id=$1 AND kind='retention_delete' AND state IN ('pendiente','procesando','reintentar')`, fileID).Scan(&activeRetentionJobs); err != nil || activeRetentionJobs != 0 {
		t.Fatalf("retention deletion remained active after a scan retry was reclaimed: jobs=%d err=%v", activeRetentionJobs, err)
	}
	if _, err := outer.Exec(ctx, `RESET ROLE`); err != nil {
		t.Fatalf("restore fixture role before expiring lease: %v", err)
	}
	if _, err := outer.Exec(ctx, `UPDATE public.document_verification_job SET lease_until=clock_timestamp()-interval '1 second' WHERE id=$1 AND state='procesando'`, retryClaim.JobID); err != nil {
		t.Fatalf("expire synthetic scan lease: %v", err)
	}
	if _, err := outer.Exec(ctx, `SET LOCAL ROLE espacigo_runtime`); err != nil {
		t.Fatalf("re-enter the restricted runtime role: %v", err)
	}
	reclaimed, err := repo.ClaimReadyScanJob(ctx, time.Minute)
	if err != nil || reclaimed == nil || reclaimed.FileID != fileID || reclaimed.Attempt != 3 || !reclaimed.LeaseUntil.After(time.Now()) {
		t.Fatalf("expired processing lease was not reclaimed: claim=%+v err=%v", reclaimed, err)
	}
	if err := repo.RecordScanOutcome(ctx, *retryClaim, ScanOutcome{State: dv.FileClean, ScannerVersion: "stale", SignaturesAt: time.Now().UTC()}); err != dv.ErrConflict {
		t.Fatalf("stale lease completion error=%v, want conflict", err)
	}
	if err := repo.RecordScanRetry(ctx, *reclaimed, "scanner_unavailable", 0); err != nil {
		t.Fatalf("persist third-attempt retry under runtime role: %v", err)
	}
	claim4, err := repo.ClaimReadyScanJob(ctx, time.Minute)
	if err != nil || claim4 == nil || claim4.FileID != fileID || claim4.Attempt != 4 {
		t.Fatalf("fourth scan attempt was not claimed: claim=%+v err=%v", claim4, err)
	}
	if err := repo.RecordScanRetry(ctx, *claim4, "scanner_unavailable", 0); err != nil {
		t.Fatalf("persist fourth-attempt retry under runtime role: %v", err)
	}
	claim5, err := repo.ClaimReadyScanJob(ctx, time.Minute)
	if err != nil || claim5 == nil || claim5.FileID != fileID || claim5.Attempt != 5 {
		t.Fatalf("fifth scan attempt was not claimed: claim=%+v err=%v", claim5, err)
	}
	if _, err := outer.Exec(ctx, `RESET ROLE`); err != nil {
		t.Fatalf("restore fixture role before expiring fifth-attempt lease: %v", err)
	}
	if _, err := outer.Exec(ctx, `UPDATE public.document_verification_job SET lease_until=clock_timestamp()-interval '1 second' WHERE id=$1 AND state='procesando' AND attempts=5`, claim5.JobID); err != nil {
		t.Fatalf("expire fifth-attempt scan lease: %v", err)
	}
	if _, err := outer.Exec(ctx, `SET LOCAL ROLE espacigo_runtime`); err != nil {
		t.Fatalf("re-enter the restricted runtime role after fifth-attempt expiry: %v", err)
	}
	reclaimedFifth, err := repo.ClaimReadyScanJob(ctx, time.Minute)
	if err != nil || reclaimedFifth == nil || reclaimedFifth.FileID != fileID || reclaimedFifth.Attempt != 5 {
		t.Fatalf("expired fifth-attempt lease was not recovered within the attempt cap: claim=%+v err=%v", reclaimedFifth, err)
	}
	if err := repo.RecordScanOutcome(ctx, *claim5, ScanOutcome{State: dv.FileClean, ScannerVersion: "stale-fifth-attempt", SignaturesAt: time.Now().UTC()}); err != dv.ErrConflict {
		t.Fatalf("stale fifth-attempt completion error=%v, want conflict", err)
	}
	if err := repo.RecordScanRetry(ctx, *reclaimedFifth, "scanner_unavailable", 0); err != nil {
		t.Fatalf("persist terminal retry under runtime role: %v", err)
	}
	var retryFileState, retryError, retryReason, retryJobState string
	var terminalRetentionJobState string
	var retryRetention time.Time
	var terminalRetentionScheduled time.Time
	var retryAttempts int16
	var retryLeaseClear bool
	if err := outer.QueryRow(ctx, `SELECT f.state,COALESCE(f.last_error_code,''),COALESCE(f.retention_reason_code,''),f.retention_at,j.state,j.attempts,j.lease_until IS NULL,COALESCE(r.state,''),COALESCE(r.available_at,'epoch'::timestamptz) FROM public.document_verification_file f JOIN public.document_verification_job j ON j.file_id=f.id AND j.kind='scan' LEFT JOIN public.document_verification_job r ON r.file_id=f.id AND r.kind='retention_delete' AND r.state IN ('pendiente','procesando','reintentar') WHERE f.id=$1`, fileID).Scan(&retryFileState, &retryError, &retryReason, &retryRetention, &retryJobState, &retryAttempts, &retryLeaseClear, &terminalRetentionJobState, &terminalRetentionScheduled); err != nil {
		t.Fatal("terminal retry state was not persisted")
	}
	if retryFileState != string(dv.FileError) || retryError != "scanner_unavailable" || retryReason != "inspection_error" || retryJobState != "completado" || retryAttempts != 5 || !retryLeaseClear || !retryRetention.Equal(firstRetryRetention) || terminalRetentionJobState != "pendiente" || !terminalRetentionScheduled.Equal(retryRetention) {
		t.Fatalf("unexpected terminal retry state: file=%q error=%q reason=%q retention=%v job=%q attempts=%d lease_clear=%t retention_job=%q scheduled=%v", retryFileState, retryError, retryReason, retryRetention, retryJobState, retryAttempts, retryLeaseClear, terminalRetentionJobState, terminalRetentionScheduled)
	}
	if _, err := outer.Exec(ctx, `RESET ROLE`); err != nil {
		t.Fatalf("restore fixture role before adding malware outcome fixture: %v", err)
	}
	malwareHash := [32]byte{2}
	malwareEarlierRetention := time.Now().UTC().Add(6 * time.Hour).Truncate(time.Microsecond)
	malwareInspectedAt := malwareEarlierRetention.Add(-24 * time.Hour)
	if _, err := outer.Exec(ctx, `INSERT INTO public.document_verification_file(id,case_id,state,size_bytes,mime_type,pages,content_sha256,inspected_at,last_error_code,retention_at,retention_reason_code) VALUES($1,$2,'bloqueado',96,'image/png',1,$3,$4,'malware_detected',$5,'abandoned_draft')`, malwareFileID, caseID, malwareHash[:], malwareInspectedAt, malwareEarlierRetention); err != nil {
		t.Fatal(err)
	}
	if _, err := outer.Exec(ctx, `INSERT INTO public.document_verification_job(file_id,kind) VALUES($1,'scan')`, malwareFileID); err != nil {
		t.Fatal(err)
	}
	if _, err := outer.Exec(ctx, `INSERT INTO public.document_verification_job(file_id,kind,state,available_at) VALUES($1,'retention_delete','pendiente',$2)`, malwareFileID, malwareEarlierRetention); err != nil {
		t.Fatal(err)
	}
	if _, err := outer.Exec(ctx, `SET LOCAL ROLE espacigo_runtime`); err != nil {
		t.Fatalf("re-enter the restricted runtime role for malware outcome: %v", err)
	}
	malwareClaim, err := repo.ClaimReadyScanJob(ctx, time.Minute)
	if err != nil || malwareClaim == nil || malwareClaim.FileID != malwareFileID || malwareClaim.Attempt != 1 {
		t.Fatalf("malware scan job was not claimed after exhausted retry: claim=%+v err=%v", malwareClaim, err)
	}
	malwareStarted := time.Now().UTC()
	if err := repo.RecordScanOutcome(ctx, *malwareClaim, ScanOutcome{State: dv.FileBlocked, ErrorCode: "malware_detected", ReasonCode: dv.ReasonMalwareDetected, ScannerVersion: "synthetic-clamav", SignaturesAt: malwareStarted}); err != nil {
		t.Fatalf("persist terminal malware outcome under runtime role: %v", err)
	}
	var malwareState, malwareError, malwareReason, malwareJobState, action, fromState, toState, resultCode, eventReason string
	var malwareRetentionJobState string
	var malwareRetention time.Time
	var malwareRetentionScheduled time.Time
	var malwareAttempts int16
	var malwareLeaseClear bool
	if err := outer.QueryRow(ctx, `SELECT f.state,COALESCE(f.last_error_code,''),COALESCE(f.retention_reason_code,''),f.retention_at,j.state,j.attempts,j.lease_until IS NULL,e.action,e.from_state,e.to_state,e.result_code,COALESCE(e.reason_code,''),COALESCE(r.state,''),COALESCE(r.available_at,'epoch'::timestamptz) FROM public.document_verification_file f JOIN public.document_verification_job j ON j.file_id=f.id AND j.kind='scan' JOIN public.document_verification_event e ON e.file_id=f.id AND e.case_id=f.case_id AND e.action='scan_result' LEFT JOIN public.document_verification_job r ON r.file_id=f.id AND r.kind='retention_delete' AND r.state IN ('pendiente','procesando','reintentar') WHERE f.id=$1`, malwareFileID).Scan(&malwareState, &malwareError, &malwareReason, &malwareRetention, &malwareJobState, &malwareAttempts, &malwareLeaseClear, &action, &fromState, &toState, &resultCode, &eventReason, &malwareRetentionJobState, &malwareRetentionScheduled); err != nil {
		t.Fatal("malware state, job completion, or audit event was not persisted")
	}
	if malwareState != string(dv.FileBlocked) || malwareError != "malware_detected" || malwareReason != "abandoned_draft" || malwareJobState != "completado" || malwareAttempts != 1 || !malwareLeaseClear || !malwareRetention.Equal(malwareEarlierRetention) || !malwareEarlierRetention.Before(malwareStarted.Add(24*time.Hour)) || action != "scan_result" || fromState != string(dv.CaseDraft) || toState != string(dv.CaseDraft) || resultCode != string(dv.FileBlocked) || eventReason != string(dv.ReasonMalwareDetected) || malwareRetentionJobState != "pendiente" || !malwareRetentionScheduled.Equal(malwareEarlierRetention) {
		t.Fatalf("unexpected persisted malware outcome: file=%q error=%q reason=%q retention=%v job=%q attempts=%d lease_clear=%t action=%q from=%q to=%q result=%q event_reason=%q retention_job=%q scheduled=%v", malwareState, malwareError, malwareReason, malwareRetention, malwareJobState, malwareAttempts, malwareLeaseClear, action, fromState, toState, resultCode, eventReason, malwareRetentionJobState, malwareRetentionScheduled)
	}
	if remaining, err := repo.ClaimReadyScanJob(ctx, time.Minute); err != nil || remaining != nil {
		t.Fatalf("terminal scan jobs were reclaimable: claim=%+v err=%v", remaining, err)
	}
}

type scanRetryRetentionBeginner struct{ tx *scanRetryRetentionTx }

func (b scanRetryRetentionBeginner) BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error) {
	return b.tx, nil
}

type scanRetryRetentionTx struct {
	pgx.Tx
	claim             ScanClaim
	dbNow             time.Time
	priorRetentionAt  *time.Time
	priorReason       string
	priorError        string
	inspectedAt       *time.Time
	persistedDeadline time.Time
	persistedReason   string
	persistedError    string
	scanEventReason   string
	scheduledDeadline time.Time
}

func (tx *scanRetryRetentionTx) QueryRow(_ context.Context, query string, _ ...any) pgx.Row {
	switch {
	case strings.Contains(query, "SELECT purpose,subject_id::text,COALESCE(space_id::text,''),revision,superseded_at"):
		return commitSerializationFailureRow{values: []any{string(dv.PurposeIdentity), tx.claim.SubjectID, "", tx.claim.CaseRevision, nil}}
	case strings.Contains(query, "SELECT state FROM public.document_verification_file"):
		return commitSerializationFailureRow{values: []any{"pendiente"}}
	case strings.Contains(query, "SELECT state,retention_at FROM public.document_verification_file"):
		var deadlineValue any
		if tx.priorRetentionAt != nil {
			deadlineValue = *tx.priorRetentionAt
		}
		return commitSerializationFailureRow{values: []any{"pendiente", deadlineValue}}
	case strings.Contains(query, "SELECT state,lease_until FROM public.document_verification_job"):
		return commitSerializationFailureRow{values: []any{"procesando", tx.claim.LeaseUntil}}
	case strings.TrimSpace(query) == "SELECT clock_timestamp()":
		return commitSerializationFailureRow{values: []any{tx.dbNow}}
	case strings.Contains(query, "SELECT purpose,subject_id::text,state,revision FROM public.document_verification_case"):
		return commitSerializationFailureRow{values: []any{string(dv.PurposeIdentity), tx.claim.SubjectID, string(dv.CaseDraft), tx.claim.CaseRevision + 1}}
	case strings.Contains(query, "SELECT retention_at,COALESCE(retention_reason_code,'') FROM public.document_verification_file"):
		var deadlineValue any
		if tx.priorRetentionAt != nil {
			deadlineValue = *tx.priorRetentionAt
		}
		return commitSerializationFailureRow{values: []any{deadlineValue, tx.priorReason}}
	case strings.Contains(query, "SELECT retention_at,COALESCE(retention_reason_code"):
		var deadlineValue, inspectedValue any
		if tx.priorRetentionAt != nil {
			deadlineValue = *tx.priorRetentionAt
		}
		if tx.inspectedAt != nil {
			inspectedValue = *tx.inspectedAt
		}
		return commitSerializationFailureRow{values: []any{deadlineValue, tx.priorReason, tx.priorError, inspectedValue}}
	case strings.Contains(query, "SELECT retention_at FROM public.document_verification_file"):
		if tx.priorRetentionAt == nil {
			return commitSerializationFailureRow{values: []any{nil}}
		}
		return commitSerializationFailureRow{values: []any{*tx.priorRetentionAt}}
	case strings.Contains(query, "SELECT EXISTS(") && strings.Contains(query, "done.kind='retention_delete'"):
		return commitSerializationFailureRow{values: []any{false}}
	default:
		return commitSerializationFailureRow{err: errors.New("unexpected scan retry retention query")}
	}
}

func (tx *scanRetryRetentionTx) Exec(_ context.Context, query string, args ...any) (pgconn.CommandTag, error) {
	if strings.Contains(query, "UPDATE public.document_verification_file SET state=") {
		if strings.Contains(query, "retention_reason_code=NULLIF($8,'')") {
			if len(args) < 8 {
				return pgconn.CommandTag{}, errors.New("missing scan outcome retention arguments")
			}
			deadline, ok := args[6].(time.Time)
			if !ok {
				return pgconn.CommandTag{}, errors.New("invalid scan outcome deadline argument")
			}
			reason, ok := args[7].(string)
			if !ok {
				return pgconn.CommandTag{}, errors.New("invalid scan outcome retention reason")
			}
			tx.persistedDeadline = deadline
			if errorCode, ok := args[2].(string); ok {
				tx.persistedError = errorCode
			}
			tx.persistedReason = reason
			return pgconn.NewCommandTag("UPDATE 1"), nil
		}
		if len(args) < 3 {
			return pgconn.CommandTag{}, errors.New("missing scan retry deadline argument")
		}
		deadline, ok := args[2].(time.Time)
		if !ok {
			return pgconn.CommandTag{}, errors.New("invalid scan retry deadline argument")
		}
		tx.persistedDeadline = deadline
		if strings.Contains(query, "retention_reason_code=$4") {
			reason, ok := args[3].(string)
			if !ok {
				return pgconn.CommandTag{}, errors.New("invalid scan retry retention reason")
			}
			tx.persistedReason = reason
		} else if strings.Contains(query, "retention_reason_code='malicious_file'") {
			tx.persistedReason = "malicious_file"
		} else {
			tx.persistedReason = "inspection_error"
		}
	}
	if strings.Contains(query, "INSERT INTO public.document_verification_event") && strings.Contains(query, "'scan_result'") {
		if len(args) >= 7 {
			switch reason := args[6].(type) {
			case string:
				tx.scanEventReason = reason
			case dv.ReasonCode:
				tx.scanEventReason = string(reason)
			}
		}
	}
	if strings.Contains(query, "UPDATE public.document_verification_job SET state='pendiente'") {
		if len(args) < 2 {
			return pgconn.CommandTag{}, errors.New("missing scheduled retention deadline argument")
		}
		deadline, ok := args[1].(time.Time)
		if !ok {
			return pgconn.CommandTag{}, errors.New("invalid scheduled retention deadline argument")
		}
		tx.scheduledDeadline = deadline
	}
	return pgconn.CommandTag{}, nil
}

func (*scanRetryRetentionTx) Commit(context.Context) error   { return nil }
func (*scanRetryRetentionTx) Rollback(context.Context) error { return nil }

func TestRecordScanRetryPreservesRetentionPrecedence(t *testing.T) {
	now := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)
	priorDeadline := now.Add(6 * time.Hour)
	laterDeadline := now.Add(48 * time.Hour)
	inspected := now.Add(-12 * time.Hour)
	equalDeadline := inspected.Add(24 * time.Hour)
	for _, tt := range []struct {
		name       string
		attempt    int16
		deadline   *time.Time
		reason     string
		lastError  string
		inspected  *time.Time
		wantDate   time.Time
		wantReason string
	}{
		{name: "earlier deadline keeps its reason", attempt: 1, deadline: &priorDeadline, reason: "abandoned_draft", lastError: "scanner_unavailable", wantDate: priorDeadline, wantReason: "abandoned_draft"},
		{name: "earlier deadline preserves null reason", attempt: 1, deadline: &priorDeadline, lastError: "scanner_unavailable", wantDate: priorDeadline, wantReason: ""},
		{name: "malware is capped from inspection", attempt: 1, deadline: &laterDeadline, reason: "malicious_file", lastError: "malware_detected", inspected: &inspected, wantDate: inspected.Add(24 * time.Hour), wantReason: "malicious_file"},
		{name: "earlier malware deadline wins", attempt: 1, deadline: &priorDeadline, reason: "malicious_file", lastError: "malware_detected", inspected: &inspected, wantDate: priorDeadline, wantReason: "malicious_file"},
		{name: "earlier malware deadline keeps its other reason", attempt: 1, deadline: &priorDeadline, reason: "abandoned_draft", lastError: "malware_detected", inspected: &inspected, wantDate: priorDeadline, wantReason: "abandoned_draft"},
		{name: "earlier malware deadline preserves null reason", attempt: 1, deadline: &priorDeadline, lastError: "malware_detected", inspected: &inspected, wantDate: priorDeadline, wantReason: ""},
		{name: "equal malware deadline uses malware reason", attempt: 1, deadline: &equalDeadline, reason: "abandoned_draft", lastError: "malware_detected", inspected: &inspected, wantDate: equalDeadline, wantReason: "malicious_file"},
		{name: "malware without inspection is immediately due", attempt: 1, reason: "malicious_file", lastError: "malware_detected", wantDate: now, wantReason: "malicious_file"},
		{name: "new inspection error uses retry policy", attempt: 1, lastError: "scanner_unavailable", wantDate: now.Add(7 * 24 * time.Hour), wantReason: "inspection_error"},
		{name: "terminal inspection error uses retry policy", attempt: 5, lastError: "scanner_unavailable", wantDate: now.Add(7 * 24 * time.Hour), wantReason: "inspection_error"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			claim := ScanClaim{
				JobID: "00000000-0000-4000-8000-000000000101", FileID: "00000000-0000-4000-8000-000000000102",
				CaseID: "00000000-0000-4000-8000-000000000103", SubjectID: "00000000-0000-4000-8000-000000000104",
				Purpose: dv.PurposeIdentity, Attempt: tt.attempt, CaseRevision: 1, LeaseUntil: now.Add(time.Hour), PriorRetentionAt: tt.deadline,
			}
			tx := &scanRetryRetentionTx{claim: claim, dbNow: now, priorRetentionAt: tt.deadline, priorReason: tt.reason, priorError: tt.lastError, inspectedAt: tt.inspected}
			repo := newWithTransactionBeginner(scanRetryRetentionBeginner{tx: tx})
			if err := repo.RecordScanRetry(context.Background(), claim, "scanner_unavailable", 0); err != nil {
				t.Fatalf("RecordScanRetry() error = %v", err)
			}
			if !tx.persistedDeadline.Equal(tt.wantDate) || !tx.scheduledDeadline.Equal(tt.wantDate) || tx.persistedReason != tt.wantReason {
				t.Fatalf("RecordScanRetry() persisted deadline/reason=%s/%q scheduled=%s, want %s/%q", tx.persistedDeadline, tx.persistedReason, tx.scheduledDeadline, tt.wantDate, tt.wantReason)
			}
		})
	}
}

func TestRecordScanOutcomeFileErrorPreservesEarlierMalwareRetention(t *testing.T) {
	now := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)
	inspectedAt := now.Add(-20 * time.Hour)
	priorDeadline := now.Add(6 * time.Hour)
	wantDeadline := inspectedAt.Add(24 * time.Hour)
	claim := ScanClaim{
		JobID: "00000000-0000-4000-8000-000000000101", FileID: "00000000-0000-4000-8000-000000000102",
		CaseID: "00000000-0000-4000-8000-000000000103", SubjectID: "00000000-0000-4000-8000-000000000104",
		Purpose: dv.PurposeIdentity, Attempt: 2, CaseRevision: 1, LeaseUntil: now.Add(time.Hour), PriorRetentionAt: &priorDeadline,
	}
	tx := &scanRetryRetentionTx{
		claim: claim, dbNow: now, priorRetentionAt: &priorDeadline,
		priorReason: "malicious_file", priorError: "malware_detected", inspectedAt: &inspectedAt,
	}
	repo := newWithTransactionBeginner(scanRetryRetentionBeginner{tx: tx})
	outcome := ScanOutcome{State: dv.FileError, ErrorCode: "inspection_rejected"}
	if err := repo.RecordScanOutcome(context.Background(), claim, outcome); err != nil {
		t.Fatalf("RecordScanOutcome() error = %v", err)
	}
	if !tx.persistedDeadline.Equal(wantDeadline) || tx.persistedReason != "malicious_file" || !tx.scheduledDeadline.Equal(wantDeadline) {
		t.Fatalf("RecordScanOutcome(FileError) retention = %s/%q scheduled=%s, want %s/%q", tx.persistedDeadline, tx.persistedReason, tx.scheduledDeadline, wantDeadline, "malicious_file")
	}
}

func TestRecordScanOutcomeBlockedPreservesEarlierDeadlineReason(t *testing.T) {
	now := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)
	priorDeadline := now.Add(6 * time.Hour)
	claim := ScanClaim{
		JobID: "00000000-0000-4000-8000-000000000101", FileID: "00000000-0000-4000-8000-000000000102",
		CaseID: "00000000-0000-4000-8000-000000000103", SubjectID: "00000000-0000-4000-8000-000000000104",
		Purpose: dv.PurposeIdentity, Attempt: 1, CaseRevision: 1, LeaseUntil: now.Add(time.Hour), PriorRetentionAt: &priorDeadline,
	}
	tx := &scanRetryRetentionTx{
		claim: claim, dbNow: now, priorRetentionAt: &priorDeadline,
		priorReason: "abandoned_draft",
	}
	repo := newWithTransactionBeginner(scanRetryRetentionBeginner{tx: tx})
	outcome := ScanOutcome{
		State: dv.FileBlocked, ScannerVersion: "synthetic-clamav/1", SignaturesAt: now.Add(-time.Minute),
		ErrorCode: "malware_detected", ReasonCode: dv.ReasonMalwareDetected,
	}
	if err := repo.RecordScanOutcome(context.Background(), claim, outcome); err != nil {
		t.Fatalf("RecordScanOutcome() error = %v", err)
	}
	if !tx.persistedDeadline.Equal(priorDeadline) || tx.persistedReason != "abandoned_draft" {
		t.Fatalf("malware outcome persisted deadline/reason=%s/%q, want %s/%q", tx.persistedDeadline, tx.persistedReason, priorDeadline, "abandoned_draft")
	}
	if tx.persistedError != "malware_detected" || tx.scanEventReason != string(dv.ReasonMalwareDetected) {
		t.Fatalf("malware outcome lost classification: last_error_code=%q scan event reason=%q", tx.persistedError, tx.scanEventReason)
	}
}

func TestRecordScanOutcomeRejectsStaleRetentionSnapshot(t *testing.T) {
	now := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)
	claimDeadline := now.Add(time.Hour)
	currentDeadline := now.Add(2 * time.Hour)
	claim := ScanClaim{
		JobID: "00000000-0000-4000-8000-000000000101", FileID: "00000000-0000-4000-8000-000000000102",
		CaseID: "00000000-0000-4000-8000-000000000103", SubjectID: "00000000-0000-4000-8000-000000000104",
		Purpose: dv.PurposeIdentity, Attempt: 2, CaseRevision: 1, LeaseUntil: now.Add(time.Hour), PriorRetentionAt: &claimDeadline,
	}
	tx := &scanRetryRetentionTx{
		claim: claim, dbNow: now, priorRetentionAt: &currentDeadline,
		priorReason: "malicious_file", priorError: "malware_detected",
	}
	repo := newWithTransactionBeginner(scanRetryRetentionBeginner{tx: tx})
	err := repo.RecordScanOutcome(context.Background(), claim, ScanOutcome{State: dv.FileError, ErrorCode: "inspection_rejected"})
	if err != dv.ErrConflict {
		t.Fatalf("RecordScanOutcome() error = %v, want conflict for stale retention snapshot", err)
	}
	if !tx.persistedDeadline.IsZero() || !tx.scheduledDeadline.IsZero() {
		t.Fatalf("stale claim mutated retention: persisted=%v scheduled=%v", tx.persistedDeadline, tx.scheduledDeadline)
	}
}
