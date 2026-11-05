package documentverificationpg

import (
	"context"
	"net"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresRecordScanRetryPreservesEarlierRetentionDeadline(t *testing.T) {
	dsn := os.Getenv("ESPACIGO_DOCUMENT_VERIFICATION_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ESPACIGO_DOCUMENT_VERIFICATION_TEST_DATABASE_URL is unset")
	}
	expectedAddress := os.Getenv("ESPACIGO_DOCUMENT_VERIFICATION_EXPECTED_SERVER_ADDRESS")
	if expectedAddress == "" || net.ParseIP(expectedAddress) == nil {
		t.Fatal("ESPACIGO_DOCUMENT_VERIFICATION_EXPECTED_SERVER_ADDRESS must be a separately supplied IP address")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal("invalid integration DSN")
	}
	cc := cfg.ConnConfig
	if cc.Host != "127.0.0.1" || cc.Port != 15434 || cc.Database != "espacigo_f5_gate_20261101" || len(cc.Fallbacks) != 0 {
		t.Fatal("refusing integration target outside the exact loopback F5 endpoint or with fallback hosts")
	}
	if cc.Config.TLSConfig != nil {
		t.Fatal("F5 integration DSN must explicitly disable TLS")
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal("cannot connect to authorized F5 synthetic endpoint")
	}
	t.Cleanup(pool.Close)
	var database string
	var serverAddress net.IP
	if err := pool.QueryRow(ctx, `SELECT current_database(),inet_server_addr()`).Scan(&database, &serverAddress); err != nil || database != "espacigo_f5_gate_20261101" || serverAddress == nil || serverAddress.String() != expectedAddress {
		t.Fatal("refusing unexpected PostgreSQL database or server address")
	}
	var setupRole string
	if err := pool.QueryRow(ctx, `SELECT current_user`).Scan(&setupRole); err != nil || setupRole == "espacigo_runtime" {
		t.Fatal("refusing unexpected database or runtime fixture setup role")
	}
	requireF5RetentionQueueIdle(t, ctx, pool)
	outer, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		t.Fatal("cannot begin rollback-scoped scan retry fixture")
	}
	ownerID, caseID, fileID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	t.Cleanup(func() {
		if err := outer.Rollback(context.Background()); err != nil && err != pgx.ErrTxClosed {
			t.Errorf("rollback scan retry fixture: %v", err)
		}
		var remaining int
		if err := pool.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM public.usuario WHERE id=$1)+(SELECT count(*) FROM public.document_verification_case WHERE id=$2)+(SELECT count(*) FROM public.document_verification_file WHERE id=$3)+(SELECT count(*) FROM public.document_verification_job WHERE file_id=$3)`, ownerID, caseID, fileID).Scan(&remaining); err != nil {
			t.Errorf("verify scan retry fixture rollback: %v", err)
		} else if remaining != 0 {
			t.Errorf("scan retry fixture rollback left %d rows", remaining)
		}
	})
	lockF5JobQueueForTest(t, ctx, outer, "scan", "")
	if _, err := outer.Exec(ctx, `INSERT INTO public.usuario(id,correo_original,correo_normalizado,hash_clave,estado) VALUES($1,$2,$2,'synthetic','activo')`, ownerID, ownerID+"@example.invalid"); err != nil {
		t.Fatal("cannot seed synthetic scan owner")
	}
	if _, err := outer.Exec(ctx, `INSERT INTO public.document_verification_case(id,purpose,subject_id) VALUES($1,'identity',$2)`, caseID, ownerID); err != nil {
		t.Fatal("cannot seed synthetic identity case")
	}
	priorDeadline := time.Now().UTC().Add(2 * time.Hour).Truncate(time.Microsecond)
	hash := [32]byte{1}
	if _, err := outer.Exec(ctx, `INSERT INTO public.document_verification_file(id,case_id,state,size_bytes,mime_type,pages,content_sha256,retention_at,retention_reason_code,last_error_code)
		VALUES($1,$2,'bloqueado',96,'image/png',1,$3,$4,'abandoned_draft','inspection_rejected')`, fileID, caseID, hash[:], priorDeadline); err != nil {
		t.Fatal("cannot seed synthetic blocked identity file")
	}
	if _, err := outer.Exec(ctx, `INSERT INTO public.document_verification_job(file_id,kind,state,attempts,available_at)
		VALUES($1,'scan','pendiente',0,clock_timestamp()-interval '1 minute')`, fileID); err != nil {
		t.Fatal("cannot seed synthetic ready scan job")
	}
	if _, err := outer.Exec(ctx, `SET LOCAL ROLE espacigo_runtime`); err != nil {
		t.Fatal("cannot assume restricted runtime role")
	}
	var effectiveRole string
	if err := outer.QueryRow(ctx, `SELECT current_user`).Scan(&effectiveRole); err != nil || effectiveRole != "espacigo_runtime" {
		t.Fatal("scan retry repository must run as espacigo_runtime")
	}
	repo := newWithTransactionBeginner(savepointBeginner{tx: outer})
	claim, err := repo.ClaimReadyScanJob(ctx, time.Minute)
	if err != nil || claim == nil || claim.FileID != fileID || claim.Attempt != 1 {
		t.Fatalf("ClaimReadyScanJob()=%#v, err=%v; want the unique synthetic fixture", claim, err)
	}
	if err := repo.RecordScanRetry(ctx, *claim, "scanner_unavailable", 0); err != nil {
		t.Fatalf("RecordScanRetry() failed against PostgreSQL: %v", err)
	}

	var fileState, retentionReason, fileError, scanJobState, retentionJobState string
	var gotDeadline, scheduledDeadline time.Time
	if err := outer.QueryRow(ctx, `SELECT state,retention_at,retention_reason_code,last_error_code FROM public.document_verification_file WHERE id=$1`, fileID).Scan(&fileState, &gotDeadline, &retentionReason, &fileError); err != nil {
		t.Fatal("cannot inspect PostgreSQL scan retry file state")
	}
	if err := outer.QueryRow(ctx, `SELECT state FROM public.document_verification_job WHERE file_id=$1 AND kind='scan'`, fileID).Scan(&scanJobState); err != nil {
		t.Fatal("cannot inspect PostgreSQL scan retry job state")
	}
	if err := outer.QueryRow(ctx, `SELECT state,available_at FROM public.document_verification_job WHERE file_id=$1 AND kind='retention_delete'`, fileID).Scan(&retentionJobState, &scheduledDeadline); err != nil {
		t.Fatal("cannot inspect PostgreSQL retention schedule")
	}
	if fileState != "bloqueado" || !gotDeadline.Equal(priorDeadline) || retentionReason != "abandoned_draft" || fileError != "scanner_unavailable" || scanJobState != "reintentar" || retentionJobState != "pendiente" || !scheduledDeadline.Equal(priorDeadline) {
		t.Fatalf("PostgreSQL retry changed prior retention policy: file=%q deadline=%s reason=%q error=%q scan_job=%q retention_job=%q scheduled=%s; want preserved deadline %s/reason abandoned_draft", fileState, gotDeadline, retentionReason, fileError, scanJobState, retentionJobState, scheduledDeadline, priorDeadline)
	}
}
