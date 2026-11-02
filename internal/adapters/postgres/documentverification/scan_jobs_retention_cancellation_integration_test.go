package documentverificationpg

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresScheduleScanRetentionReschedulesCancelledCompletedJob(t *testing.T) {
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
		t.Fatal("refusing integration target outside the authorized F5 synthetic endpoint")
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal("unable to connect to the authorized F5 synthetic target")
	}
	t.Cleanup(pool.Close)
	var database, setupRole string
	if err := pool.QueryRow(ctx, `SELECT current_database(),current_user`).Scan(&database, &setupRole); err != nil || database != "espacigo_f5_gate_20261101" || setupRole == "espacigo_runtime" {
		t.Fatalf("refusing unexpected database or runtime fixture setup role: database=%q role=%q err=%v", database, setupRole, err)
	}
	requireF5RetentionQueueIdle(t, ctx, pool)
	outer, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		t.Fatal("unable to begin rollback-scoped cancelled-retention fixture")
	}
	ownerID, caseID, fileID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	t.Cleanup(func() {
		if err := outer.Rollback(context.Background()); err != nil && err != pgx.ErrTxClosed {
			t.Errorf("rollback cancelled-retention fixture failed: %v", err)
		}
		var remaining int
		if err := pool.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM public.usuario WHERE id=$1)+(SELECT count(*) FROM public.document_verification_case WHERE id=$2)+(SELECT count(*) FROM public.document_verification_file WHERE id=$3)+(SELECT count(*) FROM public.document_verification_job WHERE file_id=$3)+(SELECT count(*) FROM public.document_verification_event WHERE case_id=$2)`, ownerID, caseID, fileID).Scan(&remaining); err != nil {
			t.Errorf("verify cancelled-retention fixture rollback failed: %v", err)
		} else if remaining != 0 {
			t.Errorf("cancelled-retention fixture rollback left %d rows", remaining)
		}
	})
	lockF5JobQueueForTest(t, ctx, outer, "retention_delete", "")
	if _, err := outer.Exec(ctx, `INSERT INTO public.usuario(id,correo_original,correo_normalizado,hash_clave,estado) VALUES($1,$2,$2,'synthetic','activo')`, ownerID, ownerID+"@example.invalid"); err != nil {
		t.Fatal("unable to seed synthetic retention actor")
	}
	if _, err := outer.Exec(ctx, `INSERT INTO public.document_verification_case(id,purpose,subject_id) VALUES($1,'identity',$2)`, caseID, ownerID); err != nil {
		t.Fatal("unable to seed synthetic identity case")
	}
	hash := [32]byte{3}
	if _, err := outer.Exec(ctx, `INSERT INTO public.document_verification_file(id,case_id,size_bytes,mime_type,pages,content_sha256) VALUES($1,$2,96,'image/png',1,$3)`, fileID, caseID, hash[:]); err != nil {
		t.Fatal("unable to seed synthetic retention file")
	}
	if _, err := outer.Exec(ctx, `INSERT INTO public.document_verification_job(file_id,kind,state,attempts) VALUES($1,'retention_delete','pendiente',1)`, fileID); err != nil {
		t.Fatal("unable to seed active retention job")
	}
	if _, err := outer.Exec(ctx, `SET LOCAL ROLE espacigo_runtime`); err != nil {
		t.Fatal("unable to assume restricted runtime role")
	}
	if err := cancelScanRetention(ctx, outer, fileID); err != nil {
		t.Fatalf("cancel retention before deletion: %v", err)
	}
	var cancelledState string
	var deletionEvents int
	if err := outer.QueryRow(ctx, `SELECT j.state,(SELECT count(*) FROM public.document_verification_event e WHERE e.file_id=j.file_id AND e.action='retention_deleted') FROM public.document_verification_job j WHERE j.file_id=$1 AND j.kind='retention_delete'`, fileID).Scan(&cancelledState, &deletionEvents); err != nil {
		t.Fatal("unable to inspect cancelled retention history")
	}
	if cancelledState != "completado" || deletionEvents != 0 {
		t.Fatalf("cancellation was mistaken for deletion: state=%q retention_deleted_events=%d", cancelledState, deletionEvents)
	}
	deadline := time.Now().UTC().Add(6 * time.Hour).Truncate(time.Microsecond)
	if err := scheduleScanRetention(ctx, outer, fileID, deadline); err != nil {
		t.Fatalf("schedule retention after cancelled completion: %v", err)
	}
	var total, completed, pending, active int
	var scheduled *time.Time
	if err := outer.QueryRow(ctx, `SELECT count(*),count(*) FILTER (WHERE state='completado'),count(*) FILTER (WHERE state='pendiente'),count(*) FILTER (WHERE state IN ('pendiente','procesando','reintentar')),max(available_at) FILTER (WHERE state='pendiente') FROM public.document_verification_job WHERE file_id=$1 AND kind='retention_delete'`, fileID).Scan(&total, &completed, &pending, &active, &scheduled); err != nil {
		t.Fatal("unable to inspect retention scheduling after cancellation")
	}
	if total != 2 || completed != 1 || pending != 1 || active != 1 || scheduled == nil || !scheduled.Equal(deadline) {
		t.Fatalf("cancelled completion suppressed retention: total=%d completed=%d pending=%d active=%d scheduled=%v want=%v", total, completed, pending, active, scheduled, deadline)
	}
}
