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

func TestPostgresScheduleScanRetentionDoesNotRecreateCompletedJob(t *testing.T) {
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
		t.Fatal("refusing unexpected database or runtime fixture setup role")
	}
	requireF5RetentionQueueIdle(t, ctx, pool)
	outer, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		t.Fatal("unable to begin rollback-scoped completed-retention fixture")
	}
	ownerID, caseID, completedOnlyFileID, duplicateFileID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	t.Cleanup(func() {
		if err := outer.Rollback(context.Background()); err != nil && err != pgx.ErrTxClosed {
			t.Errorf("rollback completed-retention fixture failed: %v", err)
		}
		var remaining int
		if err := pool.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM public.usuario WHERE id=$1)+(SELECT count(*) FROM public.document_verification_case WHERE id=$2)+(SELECT count(*) FROM public.document_verification_file WHERE id IN($3,$4))+(SELECT count(*) FROM public.document_verification_job WHERE file_id IN($3,$4))+(SELECT count(*) FROM public.document_verification_event WHERE case_id=$2)`, ownerID, caseID, completedOnlyFileID, duplicateFileID).Scan(&remaining); err != nil {
			t.Errorf("verify completed-retention fixture rollback failed: %v", err)
		} else if remaining != 0 {
			t.Errorf("completed-retention fixture rollback left %d rows", remaining)
		}
	})
	lockF5JobQueueForTest(t, ctx, outer, "retention_delete", "")
	if _, err := outer.Exec(ctx, `INSERT INTO public.usuario(id,correo_original,correo_normalizado,hash_clave,estado) VALUES($1,$2,$2,'synthetic','activo')`, ownerID, ownerID+"@example.invalid"); err != nil {
		t.Fatal("unable to seed synthetic retention actor")
	}
	if _, err := outer.Exec(ctx, `INSERT INTO public.document_verification_case(id,purpose,subject_id) VALUES($1,'identity',$2)`, caseID, ownerID); err != nil {
		t.Fatal("unable to seed synthetic identity case")
	}
	completedOnlyHash, duplicateHash := [32]byte{1}, [32]byte{2}
	if _, err := outer.Exec(ctx, `INSERT INTO public.document_verification_file(id,case_id,size_bytes,mime_type,pages,content_sha256) VALUES($1,$3,96,'image/png',1,$4),($2,$3,96,'image/png',1,$5)`, completedOnlyFileID, duplicateFileID, caseID, completedOnlyHash[:], duplicateHash[:]); err != nil {
		t.Fatal("unable to seed synthetic completed-retention file")
	}
	if _, err := outer.Exec(ctx, `INSERT INTO public.document_verification_job(file_id,kind,state,attempts) VALUES($1,'retention_delete','completado',1),($2,'retention_delete','completado',1),($2,'retention_delete','pendiente',0)`, completedOnlyFileID, duplicateFileID); err != nil {
		t.Fatal("unable to seed completed synthetic retention job")
	}
	if _, err := outer.Exec(ctx, `INSERT INTO public.document_verification_event(case_id,purpose,subject_id,file_id,actor_kind,action,from_state,to_state,result_code,reason_code,revision)
		SELECT c.id,c.purpose,c.subject_id,f.file_id,'system','retention_deleted',c.state,c.state,'eliminado','retencion_vencida',c.revision
		FROM public.document_verification_case c
		JOIN (VALUES($2::uuid),($3::uuid)) f(file_id) ON true
		WHERE c.id=$1`, caseID, completedOnlyFileID, duplicateFileID); err != nil {
		t.Fatal("unable to seed successful retention deletion audit events")
	}
	if _, err := outer.Exec(ctx, `SET LOCAL ROLE espacigo_runtime`); err != nil {
		t.Fatal("unable to assume restricted runtime role")
	}
	deadline := time.Now().UTC().Add(6 * time.Hour)
	for _, fileID := range []string{completedOnlyFileID, duplicateFileID} {
		if err := scheduleScanRetention(ctx, outer, fileID, deadline); err != nil {
			t.Fatalf("schedule scan retention against completed job: %v", err)
		}
	}
	for _, fixture := range []struct {
		fileID        string
		wantTotal     int
		wantCompleted int
	}{{completedOnlyFileID, 1, 1}, {duplicateFileID, 2, 2}} {
		var totalJobs, completedJobs, activeJobs int
		if err := outer.QueryRow(ctx, `SELECT count(*),count(*) FILTER (WHERE state='completado'),count(*) FILTER (WHERE state IN ('pendiente','procesando','reintentar')) FROM public.document_verification_job WHERE file_id=$1 AND kind='retention_delete'`, fixture.fileID).Scan(&totalJobs, &completedJobs, &activeJobs); err != nil {
			t.Fatal("unable to inspect completed retention job")
		}
		if totalJobs != fixture.wantTotal || completedJobs != fixture.wantCompleted || activeJobs != 0 {
			t.Fatalf("completed retention history was recreated or revived for %s: total=%d completed=%d active=%d", fixture.fileID, totalJobs, completedJobs, activeJobs)
		}
	}
}
