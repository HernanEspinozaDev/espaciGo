package documentverificationpg

import (
	"context"
	"os"
	"testing"
	"time"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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
	if cfg.ConnConfig.Host != "127.0.0.1" || cfg.ConnConfig.Port != 15433 || cfg.ConnConfig.Database != "espacigo_f5_synth_20261029" {
		t.Fatal("refusing integration target outside authorized F5 synthetic endpoint")
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	var db, setupRole string
	if err := pool.QueryRow(ctx, `SELECT current_database(),current_user`).Scan(&db, &setupRole); err != nil || db != "espacigo_f5_synth_20261029" {
		t.Fatalf("unexpected integration target %q: %v", db, err)
	}
	if setupRole == "espacigo_runtime" {
		t.Fatal("refusing to seed scan fixtures through the runtime role")
	}
	outer, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		t.Fatal(err)
	}
	ownerID, caseID, fileID, malwareFileID, spaceID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	t.Cleanup(func() {
		if err := outer.Rollback(context.Background()); err != nil && err != pgx.ErrTxClosed {
			t.Errorf("rollback scan fixtures: %v", err)
		}
		var remaining int
		if err := pool.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM public.usuario WHERE id=$1)+(SELECT count(*) FROM public.espacio WHERE id=$2)+(SELECT count(*) FROM public.tarifa_espacio WHERE espacio_id=$2)+(SELECT count(*) FROM public.espacio_caracteristicas WHERE espacio_id=$2)+(SELECT count(*) FROM public.document_verification_case WHERE id=$3)+(SELECT count(*) FROM public.document_verification_file WHERE id IN($4,$5))+(SELECT count(*) FROM public.document_verification_job WHERE file_id IN($4,$5))+(SELECT count(*) FROM public.document_verification_event WHERE case_id=$3)`, ownerID, spaceID, caseID, fileID, malwareFileID).Scan(&remaining); err != nil {
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
	terminalRetryStarted := time.Now().UTC()
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
	if retryFileState != string(dv.FileError) || retryError != "scanner_unavailable" || retryReason != "inspection_error" || retryJobState != "completado" || retryAttempts != 5 || !retryLeaseClear || retryRetention.Before(terminalRetryStarted.Add(7*24*time.Hour)) || retryRetention.After(terminalRetryStarted.Add(7*24*time.Hour+time.Minute)) || terminalRetentionJobState != "pendiente" || !terminalRetentionScheduled.Equal(retryRetention) {
		t.Fatalf("unexpected terminal retry state: file=%q error=%q reason=%q retention=%v job=%q attempts=%d lease_clear=%t retention_job=%q scheduled=%v", retryFileState, retryError, retryReason, retryRetention, retryJobState, retryAttempts, retryLeaseClear, terminalRetentionJobState, terminalRetentionScheduled)
	}
	if _, err := outer.Exec(ctx, `RESET ROLE`); err != nil {
		t.Fatalf("restore fixture role before adding malware outcome fixture: %v", err)
	}
	malwareHash := [32]byte{2}
	if _, err := outer.Exec(ctx, `INSERT INTO public.document_verification_file(id,case_id,size_bytes,mime_type,pages,content_sha256) VALUES($1,$2,96,'image/png',1,$3)`, malwareFileID, caseID, malwareHash[:]); err != nil {
		t.Fatal(err)
	}
	if _, err := outer.Exec(ctx, `INSERT INTO public.document_verification_job(file_id,kind) VALUES($1,'scan')`, malwareFileID); err != nil {
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
	if malwareState != string(dv.FileBlocked) || malwareError != "malware_detected" || malwareReason != "malicious_file" || malwareJobState != "completado" || malwareAttempts != 1 || !malwareLeaseClear || malwareRetention.Before(malwareStarted.Add(24*time.Hour)) || malwareRetention.After(malwareStarted.Add(24*time.Hour+time.Minute)) || action != "scan_result" || fromState != string(dv.CaseDraft) || toState != string(dv.CaseDraft) || resultCode != string(dv.FileBlocked) || eventReason != string(dv.ReasonMalwareDetected) || malwareRetentionJobState != "pendiente" || !malwareRetentionScheduled.Equal(malwareRetention) {
		t.Fatalf("unexpected persisted malware outcome: file=%q error=%q reason=%q retention=%v job=%q attempts=%d lease_clear=%t action=%q from=%q to=%q result=%q event_reason=%q retention_job=%q scheduled=%v", malwareState, malwareError, malwareReason, malwareRetention, malwareJobState, malwareAttempts, malwareLeaseClear, action, fromState, toState, resultCode, eventReason, malwareRetentionJobState, malwareRetentionScheduled)
	}
	if remaining, err := repo.ClaimReadyScanJob(ctx, time.Minute); err != nil || remaining != nil {
		t.Fatalf("terminal scan jobs were reclaimable: claim=%+v err=%v", remaining, err)
	}
}
