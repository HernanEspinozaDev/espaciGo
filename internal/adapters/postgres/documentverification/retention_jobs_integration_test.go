package documentverificationpg

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/documentfs"
	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type delayedRetentionDocuments struct {
	retentionDocuments
	delay time.Duration
}

func (d delayedRetentionDocuments) DeleteIdentityDocument(ctx context.Context, subjectID, caseID, fileID string) error {
	timer := time.NewTimer(d.delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return d.retentionDocuments.DeleteIdentityDocument(ctx, subjectID, caseID, fileID)
	}
}

// TestPostgresRetentionDeletesExpiredEncryptedIdentityDocument verifies the
// runtime role's durable claim, encrypted-object deletion, and append-only
// completion audit in one rollback-scoped F5 transaction.
func TestPostgresRetentionDeletesExpiredEncryptedIdentityDocument(t *testing.T) {
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
	t.Cleanup(pool.Close)
	var database, setupRole string
	if err := pool.QueryRow(ctx, `SELECT current_database(),current_user`).Scan(&database, &setupRole); err != nil || database != "espacigo_f5_synth_20261029" || setupRole == "espacigo_runtime" {
		t.Fatal("refusing unexpected database or runtime fixture setup role")
	}

	outer, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		t.Fatal("unable to begin rollback-scoped retention fixture")
	}
	actor, caseID, fileID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	t.Cleanup(func() {
		if err := outer.Rollback(context.Background()); err != nil && err != pgx.ErrTxClosed {
			t.Errorf("rollback retention fixture failed: %v", err)
		}
		var remaining int
		if err := pool.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM public.usuario WHERE id=$1)+(SELECT count(*) FROM public.document_verification_case WHERE id=$2)+(SELECT count(*) FROM public.document_verification_file WHERE id=$3)+(SELECT count(*) FROM public.document_verification_job WHERE file_id=$3)+(SELECT count(*) FROM public.document_verification_event WHERE case_id=$2)`, actor, caseID, fileID).Scan(&remaining); err != nil {
			t.Errorf("verify retention fixture rollback failed: %v", err)
		} else if remaining != 0 {
			t.Errorf("retention fixture rollback left %d rows", remaining)
		}
	})
	if _, err := outer.Exec(ctx, `INSERT INTO public.usuario(id,correo_original,correo_normalizado,hash_clave,estado) VALUES($1,$2,$2,'synthetic','activo')`, actor, actor+"@example.invalid"); err != nil {
		t.Fatal("unable to seed synthetic retention actor")
	}
	if _, err := outer.Exec(ctx, `INSERT INTO public.document_verification_case(id,purpose,subject_id) VALUES($1,'identity',$2)`, caseID, actor); err != nil {
		t.Fatal("unable to seed synthetic identity case")
	}

	var raw bytes.Buffer
	if err := png.Encode(&raw, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal("unable to create synthetic PNG document")
	}
	content := raw.Bytes()
	contentHash := sha256.Sum256(content)
	if _, err := outer.Exec(ctx, `INSERT INTO public.document_verification_file(id,case_id,state,size_bytes,mime_type,pages,content_sha256,scanner_version,signatures_at,inspected_at,retention_at,retention_reason_code) VALUES($1,$2,'limpio',$3,'image/png',0,$4,'synthetic-test',clock_timestamp(),clock_timestamp(),clock_timestamp()-interval '1 second','abandoned_draft')`, fileID, caseID, len(content), contentHash[:]); err != nil {
		t.Fatal("unable to seed synthetic expired document")
	}
	if _, err := outer.Exec(ctx, `INSERT INTO public.document_verification_job(file_id,kind,state,attempts,available_at,lease_until) VALUES($1,'scan','procesando',1,clock_timestamp(),clock_timestamp()+interval '2 minutes'),($1,'retention_delete','pendiente',0,clock_timestamp()-interval '1 second',NULL)`, fileID); err != nil {
		t.Fatal("unable to seed synthetic scan and due retention jobs")
	}
	if _, err := outer.Exec(ctx, `SET LOCAL ROLE espacigo_runtime`); err != nil {
		t.Fatal("unable to assume restricted runtime role")
	}
	var role string
	var superuser, bypassRLS bool
	if err := outer.QueryRow(ctx, `SELECT current_user,r.rolsuper,r.rolbypassrls FROM pg_catalog.pg_roles r WHERE r.rolname=current_user`).Scan(&role, &superuser, &bypassRLS); err != nil || role != "espacigo_runtime" || superuser || bypassRLS {
		t.Fatal("retention integration is not running as the restricted runtime role")
	}

	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal("unable to create synthetic storage key")
	}
	storageRoot := filepath.Join(t.TempDir(), "private-documents")
	store, err := documentfs.New(storageRoot, key)
	if err != nil {
		t.Fatal("unable to create private synthetic document store")
	}
	if err := store.PutIdentityDocument(ctx, actor, caseID, fileID, content); err != nil {
		t.Fatal("unable to seed encrypted synthetic document")
	}

	repo := newWithTransactionBeginner(savepointBeginner{tx: outer})
	processor := NewRetentionProcessor(repo, store, 2*time.Minute, 30*time.Second)
	if processor == nil {
		t.Fatal("unable to create retention processor")
	}
	if err := processor.ProcessOne(ctx); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			t.Fatalf("retention processor failed: SQLSTATE=%s column=%s constraint=%s", pgErr.Code, pgErr.ColumnName, pgErr.ConstraintName)
		}
		t.Fatalf("retention processor failed: %T", err)
	}
	if retained, err := store.ReadIdentityDocument(ctx, actor, caseID, fileID); err != nil || !bytes.Equal(retained, content) {
		t.Fatal("retention worker deleted a document while its scan job was active")
	}
	var retentionState string
	if err := outer.QueryRow(ctx, `SELECT state FROM public.document_verification_job WHERE file_id=$1 AND kind='retention_delete'`, fileID).Scan(&retentionState); err != nil || retentionState != "pendiente" {
		t.Fatal("retention job was claimed while its scan job was active")
	}
	if _, err := outer.Exec(ctx, `UPDATE public.document_verification_job SET state='completado',lease_until=NULL,updated_at=clock_timestamp() WHERE file_id=$1 AND kind='scan' AND state='procesando'`, fileID); err != nil {
		t.Fatal("unable to finish synthetic scan job")
	}
	staleClaim, err := repo.ClaimReadyRetentionJob(ctx, 2*time.Minute)
	if err != nil || staleClaim == nil {
		t.Fatal("unable to claim due retention job after scan completion")
	}
	if _, err := outer.Exec(ctx, `UPDATE public.document_verification_job SET lease_until=clock_timestamp()-interval '1 second',updated_at=clock_timestamp() WHERE id=$1 AND state='procesando'`, staleClaim.JobID); err != nil {
		t.Fatal("unable to expire synthetic retention lease")
	}
	staleDeleteCalled := false
	if err := repo.ProcessClaimedRetentionJob(ctx, *staleClaim, func(context.Context) error {
		staleDeleteCalled = true
		return nil
	}, 30*time.Second); !errors.Is(err, dv.ErrConflict) || staleDeleteCalled {
		t.Fatal("stale retention worker was not fenced before storage deletion")
	}
	if retained, err := store.ReadIdentityDocument(ctx, actor, caseID, fileID); err != nil || !bytes.Equal(retained, content) {
		t.Fatal("stale worker changed the encrypted document")
	}
	if err := NewRetentionProcessor(repo, &retentionStoreFake{err: errors.New("synthetic storage failure")}, 2*time.Minute, 30*time.Second).ProcessOne(ctx); err != nil {
		t.Fatal("failed storage deletion was not persisted as retry")
	}
	var retryState, retryError string
	var retryAttempts int16
	if err := outer.QueryRow(ctx, `SELECT state,attempts,last_error_code FROM public.document_verification_job WHERE file_id=$1 AND kind='retention_delete'`, fileID).Scan(&retryState, &retryAttempts, &retryError); err != nil || retryState != "reintentar" || retryAttempts != 2 || retryError != "retention_store_error" {
		t.Fatal("retention storage failure did not persist a bounded retry state")
	}
	if retained, err := store.ReadIdentityDocument(ctx, actor, caseID, fileID); err != nil || !bytes.Equal(retained, content) {
		t.Fatal("failed storage deletion removed the encrypted document")
	}
	if _, err := outer.Exec(ctx, `UPDATE public.document_verification_job SET available_at=clock_timestamp()-interval '1 second',updated_at=clock_timestamp() WHERE file_id=$1 AND kind='retention_delete' AND state='reintentar'`, fileID); err != nil {
		t.Fatal("unable to release synthetic retry for deterministic test")
	}
	delayedProcessor := NewRetentionProcessor(repo, delayedRetentionDocuments{retentionDocuments: store, delay: 1100 * time.Millisecond}, time.Second, 30*time.Second)
	if err := delayedProcessor.ProcessOne(ctx); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			t.Fatalf("retention processor failed after retry: SQLSTATE=%s column=%s constraint=%s", pgErr.Code, pgErr.ColumnName, pgErr.ConstraintName)
		}
		t.Fatalf("retention processor failed after retry: %T", err)
	}
	if _, err := store.ReadIdentityDocument(ctx, actor, caseID, fileID); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("expired encrypted identity document was not removed")
	}
	var state, action, result, reason string
	var events int
	if err := outer.QueryRow(ctx, `SELECT j.state,e.action,e.result_code,e.reason_code,(SELECT count(*) FROM public.document_verification_event WHERE case_id=$2 AND file_id=$1 AND action='retention_deleted') FROM public.document_verification_job j JOIN public.document_verification_event e ON e.case_id=$2 AND e.file_id=j.file_id AND e.action='retention_deleted' WHERE j.file_id=$1 AND j.kind='retention_delete'`, fileID, caseID).Scan(&state, &action, &result, &reason, &events); err != nil {
		t.Fatal("retention completion or audit event was not persisted")
	}
	if state != "completado" || action != "retention_deleted" || result != "eliminado" || reason != "retencion_vencida" || events != 1 {
		t.Fatalf("retention completion state/audit = %q/%q/%q/%q events=%d", state, action, result, reason, events)
	}
}

func TestPostgresRetentionDeletesExpiredSupersededSpaceDocument(t *testing.T) {
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
	t.Cleanup(pool.Close)
	var database, setupRole string
	if err := pool.QueryRow(ctx, `SELECT current_database(),current_user`).Scan(&database, &setupRole); err != nil || database != "espacigo_f5_synth_20261029" || setupRole == "espacigo_runtime" {
		t.Fatal("refusing unexpected database or runtime fixture setup role")
	}
	outer, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		t.Fatal("unable to begin rollback-scoped space-retention fixture")
	}
	oldOwner, newOwner, spaceID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	caseID, fileID := uuid.NewString(), uuid.NewString()
	t.Cleanup(func() {
		if err := outer.Rollback(context.Background()); err != nil && err != pgx.ErrTxClosed {
			t.Errorf("rollback space-retention fixture failed: %v", err)
		}
		var remaining int
		if err := pool.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM public.usuario WHERE id IN($1,$2))+(SELECT count(*) FROM public.espacio WHERE id=$3)+(SELECT count(*) FROM public.tarifa_espacio WHERE espacio_id=$3)+(SELECT count(*) FROM public.espacio_caracteristicas WHERE espacio_id=$3)+(SELECT count(*) FROM public.document_verification_case WHERE id=$4)+(SELECT count(*) FROM public.document_verification_file WHERE id=$5)+(SELECT count(*) FROM public.document_verification_job WHERE file_id=$5)+(SELECT count(*) FROM public.document_verification_event WHERE case_id=$4)`, oldOwner, newOwner, spaceID, caseID, fileID).Scan(&remaining); err != nil {
			t.Errorf("verify space-retention fixture rollback failed: %v", err)
		} else if remaining != 0 {
			t.Errorf("space-retention fixture rollback left %d rows", remaining)
		}
	})
	for _, owner := range []string{oldOwner, newOwner} {
		if _, err := outer.Exec(ctx, `INSERT INTO public.usuario(id,correo_original,correo_normalizado,hash_clave,estado) VALUES($1,$2,$2,'synthetic','activo')`, owner, owner+"@example.invalid"); err != nil {
			t.Fatal("unable to seed synthetic space-retention owner")
		}
	}
	if _, err := outer.Exec(ctx, `INSERT INTO public.espacio(id,propietario_id,categoria_codigo,titulo,descripcion,superficie_m2,capacidad_maxima,reglas_uso,modalidad_tarifa,precio_base_clp,direccion,zona_horaria,estado) VALUES($1,$2,'sala_multiproposito','F5 retention fixture',repeat('Synthetic space retention description. ',4),30,4,'Synthetic','hora',8000,'Synthetic address','America/Santiago','activa')`, spaceID, oldOwner); err != nil {
		t.Fatal("unable to seed synthetic retention space")
	}
	if _, err := outer.Exec(ctx, `INSERT INTO public.tarifa_espacio(espacio_id,version,modalidad,precio_base_clp) VALUES($1,1,'hora',8000)`, spaceID); err != nil {
		t.Fatal("unable to seed synthetic space tariff")
	}
	if _, err := outer.Exec(ctx, `INSERT INTO public.espacio_caracteristicas(espacio_id,categoria_codigo,perfil_version,valores) VALUES($1,'sala_multiproposito',1,'{}'::jsonb)`, spaceID); err != nil {
		t.Fatal("unable to seed synthetic space attributes")
	}
	if _, err := outer.Exec(ctx, `INSERT INTO public.document_verification_case(id,purpose,subject_id,space_id) VALUES($1,'space',$2,$3)`, caseID, oldOwner, spaceID); err != nil {
		t.Fatal("unable to seed superseded space case")
	}
	hash := [32]byte{9}
	if _, err := outer.Exec(ctx, `INSERT INTO public.document_verification_file(id,case_id,state,size_bytes,mime_type,pages,content_sha256,scanner_version,signatures_at,inspected_at) VALUES($1,$2,'limpio',64,'image/png',1,$3,'synthetic-test',clock_timestamp(),clock_timestamp())`, fileID, caseID, hash[:]); err != nil {
		t.Fatal("unable to seed synthetic clean space document")
	}
	if _, err := outer.Exec(ctx, `UPDATE public.espacio SET propietario_id=$2,actualizado_en=clock_timestamp() WHERE id=$1`, spaceID, newOwner); err != nil {
		t.Fatal("unable to transfer synthetic space ownership")
	}
	if _, err := outer.Exec(ctx, `UPDATE public.document_verification_file SET retention_at=clock_timestamp()-interval '1 second' WHERE id=$1`, fileID); err != nil {
		t.Fatal("unable to advance synthetic retention deadline")
	}
	if _, err := outer.Exec(ctx, `UPDATE public.document_verification_job SET available_at=clock_timestamp()-interval '1 second' WHERE file_id=$1 AND kind='retention_delete'`, fileID); err != nil {
		t.Fatal("unable to advance synthetic retention job")
	}
	if _, err := outer.Exec(ctx, `SET LOCAL ROLE espacigo_runtime`); err != nil {
		t.Fatal("unable to assume restricted runtime role")
	}
	var role string
	var superuser, bypassRLS bool
	if err := outer.QueryRow(ctx, `SELECT current_user,r.rolsuper,r.rolbypassrls FROM pg_catalog.pg_roles r WHERE r.rolname=current_user`).Scan(&role, &superuser, &bypassRLS); err != nil || role != "espacigo_runtime" || superuser || bypassRLS {
		t.Fatal("space-retention integration is not running as the restricted runtime role")
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal("unable to create synthetic storage key")
	}
	store, err := documentfs.New(filepath.Join(t.TempDir(), "private-documents"), key)
	if err != nil {
		t.Fatal("unable to create private synthetic document store")
	}
	content := []byte("synthetic expired space document")
	if err := store.PutSpaceDocument(ctx, oldOwner, spaceID, caseID, fileID, content); err != nil {
		t.Fatal("unable to seed encrypted synthetic space document")
	}
	repo := newWithTransactionBeginner(savepointBeginner{tx: outer})
	processor := NewRetentionProcessor(repo, store, 2*time.Minute, 30*time.Second)
	if err := processor.ProcessOne(ctx); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			t.Fatalf("space-retention processor failed: SQLSTATE=%s column=%s constraint=%s", pgErr.Code, pgErr.ColumnName, pgErr.ConstraintName)
		}
		t.Fatalf("space-retention processor failed: %T", err)
	}
	if _, err := store.ReadSpaceDocument(ctx, oldOwner, spaceID, caseID, fileID); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("expired encrypted superseded space document was not removed")
	}
	var state, action, result, reason string
	var events int
	if err := outer.QueryRow(ctx, `SELECT j.state,e.action,e.result_code,e.reason_code,(SELECT count(*) FROM public.document_verification_event WHERE case_id=$2 AND file_id=$1 AND action='retention_deleted') FROM public.document_verification_job j JOIN public.document_verification_event e ON e.case_id=$2 AND e.file_id=j.file_id AND e.action='retention_deleted' WHERE j.file_id=$1 AND j.kind='retention_delete'`, fileID, caseID).Scan(&state, &action, &result, &reason, &events); err != nil {
		t.Fatal("space-retention completion or audit event was not persisted")
	}
	if state != "completado" || action != "retention_deleted" || result != "eliminado" || reason != "retencion_vencida" || events != 1 {
		t.Fatal("space-retention completion state or audit event is incorrect")
	}
}

type transferSpaceOwnerAfterRetentionClaim struct {
	*Repository
	tx                pgx.Tx
	spaceID, newOwner string
}

func (r *transferSpaceOwnerAfterRetentionClaim) ClaimReadyRetentionJob(ctx context.Context, lease time.Duration) (*RetentionClaim, error) {
	claim, err := r.Repository.ClaimReadyRetentionJob(ctx, lease)
	if err != nil || claim == nil {
		return claim, err
	}
	if _, err := r.tx.Exec(ctx, `RESET ROLE`); err != nil {
		return nil, err
	}
	if _, err := r.tx.Exec(ctx, `UPDATE public.espacio SET propietario_id=$2,actualizado_en=clock_timestamp() WHERE id=$1`, r.spaceID, r.newOwner); err != nil {
		return nil, err
	}
	if _, err := r.tx.Exec(ctx, `SET LOCAL ROLE espacigo_runtime`); err != nil {
		return nil, err
	}
	return claim, nil
}

func TestPostgresRetentionDoesNotDeleteAfterOwnershipTransferResetsClaim(t *testing.T) {
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
	t.Cleanup(pool.Close)
	var database, setupRole string
	if err := pool.QueryRow(ctx, `SELECT current_database(),current_user`).Scan(&database, &setupRole); err != nil || database != "espacigo_f5_synth_20261029" || setupRole == "espacigo_runtime" {
		t.Fatal("refusing unexpected database or runtime fixture setup role")
	}
	outer, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		t.Fatal("unable to begin rollback-scoped retention race fixture")
	}
	oldOwner, newOwner, spaceID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	caseID, fileID := uuid.NewString(), uuid.NewString()
	t.Cleanup(func() {
		if err := outer.Rollback(context.Background()); err != nil && err != pgx.ErrTxClosed {
			t.Errorf("rollback retention race fixture failed: %v", err)
		}
		var remaining int
		if err := pool.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM public.usuario WHERE id IN($1,$2))+(SELECT count(*) FROM public.espacio WHERE id=$3)+(SELECT count(*) FROM public.tarifa_espacio WHERE espacio_id=$3)+(SELECT count(*) FROM public.espacio_caracteristicas WHERE espacio_id=$3)+(SELECT count(*) FROM public.document_verification_case WHERE id=$4)+(SELECT count(*) FROM public.document_verification_file WHERE id=$5)+(SELECT count(*) FROM public.document_verification_job WHERE file_id=$5)+(SELECT count(*) FROM public.document_verification_event WHERE case_id=$4)`, oldOwner, newOwner, spaceID, caseID, fileID).Scan(&remaining); err != nil {
			t.Errorf("verify retention race rollback failed: %v", err)
		} else if remaining != 0 {
			t.Errorf("retention race fixture rollback left %d rows", remaining)
		}
	})
	for _, owner := range []string{oldOwner, newOwner} {
		if _, err := outer.Exec(ctx, `INSERT INTO public.usuario(id,correo_original,correo_normalizado,hash_clave,estado) VALUES($1,$2,$2,'synthetic','activo')`, owner, owner+"@example.invalid"); err != nil {
			t.Fatal("unable to seed synthetic retention race owner")
		}
	}
	if _, err := outer.Exec(ctx, `INSERT INTO public.espacio(id,propietario_id,categoria_codigo,titulo,descripcion,superficie_m2,capacidad_maxima,reglas_uso,modalidad_tarifa,precio_base_clp,direccion,zona_horaria,estado) VALUES($1,$2,'sala_multiproposito','F5 retention race fixture',repeat('Synthetic space retention race description. ',4),30,4,'Synthetic','hora',8000,'Synthetic address','America/Santiago','activa')`, spaceID, oldOwner); err != nil {
		t.Fatal("unable to seed synthetic retention race space")
	}
	if _, err := outer.Exec(ctx, `INSERT INTO public.tarifa_espacio(espacio_id,version,modalidad,precio_base_clp) VALUES($1,1,'hora',8000)`, spaceID); err != nil {
		t.Fatal("unable to seed synthetic retention race tariff")
	}
	if _, err := outer.Exec(ctx, `INSERT INTO public.espacio_caracteristicas(espacio_id,categoria_codigo,perfil_version,valores) VALUES($1,'sala_multiproposito',1,'{}'::jsonb)`, spaceID); err != nil {
		t.Fatal("unable to seed synthetic retention race attributes")
	}
	if _, err := outer.Exec(ctx, `INSERT INTO public.document_verification_case(id,purpose,subject_id,space_id) VALUES($1,'space',$2,$3)`, caseID, oldOwner, spaceID); err != nil {
		t.Fatal("unable to seed synthetic retention race case")
	}
	var raw bytes.Buffer
	if err := png.Encode(&raw, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal("unable to create synthetic retention race PNG")
	}
	content := raw.Bytes()
	hash := sha256.Sum256(content)
	originalDeadline := time.Now().UTC().Truncate(time.Microsecond).Add(-time.Second)
	if _, err := outer.Exec(ctx, `INSERT INTO public.document_verification_file(id,case_id,state,size_bytes,mime_type,pages,content_sha256,scanner_version,signatures_at,inspected_at,retention_at,retention_reason_code) VALUES($1,$2,'limpio',$3,'image/png',0,$4,'synthetic-test',clock_timestamp(),clock_timestamp(),$5,'abandoned_draft')`, fileID, caseID, len(content), hash[:], originalDeadline); err != nil {
		t.Fatal("unable to seed due synthetic retention race file")
	}
	if _, err := outer.Exec(ctx, `INSERT INTO public.document_verification_job(file_id,kind,state,attempts,available_at,lease_until) VALUES($1,'retention_delete','pendiente',0,clock_timestamp()-interval '1 second',NULL)`, fileID); err != nil {
		t.Fatal("unable to seed due synthetic retention race job")
	}
	if _, err := outer.Exec(ctx, `SET LOCAL ROLE espacigo_runtime`); err != nil {
		t.Fatal("unable to assume restricted runtime role")
	}
	var role string
	var superuser, bypassRLS bool
	if err := outer.QueryRow(ctx, `SELECT current_user,r.rolsuper,r.rolbypassrls FROM pg_catalog.pg_roles r WHERE r.rolname=current_user`).Scan(&role, &superuser, &bypassRLS); err != nil || role != "espacigo_runtime" || superuser || bypassRLS {
		t.Fatal("retention race integration is not running as the restricted runtime role")
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal("unable to create synthetic storage key")
	}
	store, err := documentfs.New(filepath.Join(t.TempDir(), "private-documents"), key)
	if err != nil {
		t.Fatal("unable to create private synthetic document store")
	}
	if err := store.PutSpaceDocument(ctx, oldOwner, spaceID, caseID, fileID, content); err != nil {
		t.Fatal("unable to seed encrypted synthetic retention race document")
	}
	repo := newWithTransactionBeginner(savepointBeginner{tx: outer})
	transfer := &transferSpaceOwnerAfterRetentionClaim{Repository: repo, tx: outer, spaceID: spaceID, newOwner: newOwner}
	processor := NewRetentionProcessor(transfer, store, 2*time.Minute, 30*time.Second)
	processErr := processor.ProcessOne(ctx)
	if _, err := store.ReadSpaceDocument(ctx, oldOwner, spaceID, caseID, fileID); err != nil {
		t.Fatalf("retention worker deleted the document after ownership transfer reset its lease (process error %v): %v", processErr, err)
	}
	if !errors.Is(processErr, dv.ErrConflict) {
		t.Fatalf("stale retention claim should be fenced after ownership transfer, got %v", processErr)
	}
	var state string
	var leaseUntil *time.Time
	var retentionDeadline time.Time
	var retentionReason string
	if err := outer.QueryRow(ctx, `SELECT j.state,j.lease_until,f.retention_at,f.retention_reason_code FROM public.document_verification_job j JOIN public.document_verification_file f ON f.id=j.file_id WHERE j.file_id=$1 AND j.kind='retention_delete'`, fileID).Scan(&state, &leaseUntil, &retentionDeadline, &retentionReason); err != nil {
		t.Fatal("unable to read retention state after ownership transfer")
	}
	if state != "pendiente" || leaseUntil != nil || !retentionDeadline.Equal(originalDeadline) || retentionReason != "abandoned_draft" {
		t.Fatalf("ownership transfer overwrote the earlier retention state: job=%q lease=%v deadline=%v reason=%q", state, leaseUntil, retentionDeadline, retentionReason)
	}
}

type retentionClaimBarrier struct {
	mu      sync.Mutex
	arrived int
	ready   chan struct{}
}

func newRetentionClaimBarrier() *retentionClaimBarrier {
	return &retentionClaimBarrier{ready: make(chan struct{})}
}

func (b *retentionClaimBarrier) wait(ctx context.Context) error {
	b.mu.Lock()
	b.arrived++
	if b.arrived == 2 {
		close(b.ready)
	}
	ready := b.ready
	b.mu.Unlock()
	select {
	case <-ready:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type retentionClaimBarrierRow struct {
	pgx.Row
	ctx     context.Context
	barrier *retentionClaimBarrier
}

func (r retentionClaimBarrierRow) Scan(dest ...any) error {
	if err := r.Row.Scan(dest...); err != nil {
		return err
	}
	return r.barrier.wait(r.ctx)
}

type retentionClaimBarrierTx struct {
	pgx.Tx
	barrier *retentionClaimBarrier
	first   bool
}

func (tx *retentionClaimBarrierTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	row := tx.Tx.QueryRow(ctx, sql, args...)
	if tx.first {
		return row
	}
	tx.first = true
	return retentionClaimBarrierRow{Row: row, ctx: ctx, barrier: tx.barrier}
}

type retentionClaimBarrierBeginner struct {
	pool    *pgxpool.Pool
	barrier *retentionClaimBarrier
}

func (b retentionClaimBarrierBeginner) BeginTx(ctx context.Context, options pgx.TxOptions) (pgx.Tx, error) {
	tx, err := b.pool.BeginTx(ctx, options)
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
			err = errors.New("retention claim transaction is not using the restricted runtime role")
		}
		return nil, err
	}
	return &retentionClaimBarrierTx{Tx: tx, barrier: b.barrier}, nil
}

func TestPostgresRetentionConcurrentClaimersReturnOneLeaseOwner(t *testing.T) {
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
	t.Cleanup(pool.Close)
	var database, setupRole string
	if err := pool.QueryRow(ctx, `SELECT current_database(),current_user`).Scan(&database, &setupRole); err != nil || database != "espacigo_f5_synth_20261029" || setupRole == "espacigo_runtime" {
		t.Fatal("refusing unexpected database or runtime fixture setup role")
	}
	actor, caseID, fileID, jobID, decoyFileID, decoyJobID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	setupTx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		t.Fatal("unable to begin concurrent retention fixture setup")
	}
	t.Cleanup(func() {
		if err := setupTx.Rollback(context.Background()); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			t.Errorf("rollback concurrent retention fixture setup: %v", err)
		}
		cleanupTx, err := pool.Begin(context.Background())
		if err != nil {
			t.Errorf("begin concurrent retention fixture cleanup: %v", err)
			return
		}
		for _, query := range []struct {
			name string
			sql  string
			id   string
		}{
			{"job", `DELETE FROM public.document_verification_job WHERE id=$1`, jobID},
			{"decoy job", `DELETE FROM public.document_verification_job WHERE id=$1`, decoyJobID},
			{"file", `DELETE FROM public.document_verification_file WHERE id=$1`, fileID},
			{"decoy file", `DELETE FROM public.document_verification_file WHERE id=$1`, decoyFileID},
			{"case", `DELETE FROM public.document_verification_case WHERE id=$1`, caseID},
			{"actor", `DELETE FROM public.usuario WHERE id=$1`, actor},
		} {
			if _, err := cleanupTx.Exec(context.Background(), query.sql, query.id); err != nil {
				t.Errorf("delete concurrent retention fixture %s: %v", query.name, err)
				if rollbackErr := cleanupTx.Rollback(context.Background()); rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) {
					t.Errorf("rollback concurrent retention fixture cleanup: %v", rollbackErr)
				}
				return
			}
		}
		if err := cleanupTx.Commit(context.Background()); err != nil {
			t.Errorf("commit concurrent retention fixture cleanup: %v", err)
			return
		}
		var remaining int
		if err := pool.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM public.usuario WHERE id=$1)+(SELECT count(*) FROM public.document_verification_case WHERE id=$2)+(SELECT count(*) FROM public.document_verification_file WHERE id=ANY($3::uuid[]))+(SELECT count(*) FROM public.document_verification_job WHERE id=ANY($4::uuid[]))`, actor, caseID, []string{fileID, decoyFileID}, []string{jobID, decoyJobID}).Scan(&remaining); err != nil {
			t.Errorf("verify concurrent retention fixture cleanup: %v", err)
		} else if remaining != 0 {
			t.Errorf("concurrent retention fixture cleanup left %d rows", remaining)
		}
	})
	if _, err := setupTx.Exec(ctx, `INSERT INTO public.usuario(id,correo_original,correo_normalizado,hash_clave,estado) VALUES($1,$2,$2,'synthetic','activo')`, actor, actor+"@example.invalid"); err != nil {
		t.Fatal("unable to seed concurrent retention actor")
	}
	if _, err := setupTx.Exec(ctx, `INSERT INTO public.document_verification_case(id,purpose,subject_id) VALUES($1,'identity',$2)`, caseID, actor); err != nil {
		t.Fatal("unable to seed concurrent retention case")
	}
	content := []byte("synthetic concurrent retention file")
	hash := sha256.Sum256(content)
	if _, err := setupTx.Exec(ctx, `INSERT INTO public.document_verification_file(id,case_id,state,size_bytes,mime_type,pages,content_sha256,scanner_version,signatures_at,inspected_at,retention_at,retention_reason_code) VALUES($1,$2,'limpio',$3,'image/png',0,$4,'synthetic-test',clock_timestamp(),clock_timestamp(),clock_timestamp()-interval '1 second','abandoned_draft')`, fileID, caseID, len(content), hash[:]); err != nil {
		t.Fatal("unable to seed concurrent retention file")
	}
	if _, err := setupTx.Exec(ctx, `INSERT INTO public.document_verification_file(id,case_id,state,size_bytes,mime_type,pages,content_sha256,scanner_version,signatures_at,inspected_at,retention_at,retention_reason_code) VALUES($1,$2,'limpio',$3,'image/png',0,$4,'synthetic-test',clock_timestamp(),clock_timestamp(),clock_timestamp()-interval '1 second','abandoned_draft')`, decoyFileID, caseID, len(content), hash[:]); err != nil {
		t.Fatal("unable to seed concurrent retention decoy file")
	}
	if _, err := setupTx.Exec(ctx, `INSERT INTO public.document_verification_job(id,file_id,kind,state,attempts,available_at,lease_until) VALUES($1,$2,'retention_delete','pendiente',0,clock_timestamp()+interval '1 hour',NULL)`, decoyJobID, decoyFileID); err != nil {
		t.Fatal("unable to seed concurrent retention decoy job")
	}
	if _, err := setupTx.Exec(ctx, `INSERT INTO public.document_verification_job(id,file_id,kind,state,attempts,available_at,lease_until) VALUES($1,$2,'retention_delete','pendiente',0,clock_timestamp()+interval '1 hour',NULL)`, jobID, fileID); err != nil {
		t.Fatal("unable to seed concurrent retention job")
	}
	if err := setupTx.Commit(ctx); err != nil {
		t.Fatal("unable to commit concurrent retention fixture")
	}

	barrier := newRetentionClaimBarrier()
	repo := newWithTransactionBeginner(retentionClaimBarrierBeginner{pool: pool, barrier: barrier})
	var eligibleBefore time.Time
	if err := pool.QueryRow(ctx, `SELECT clock_timestamp()+interval '2 hours'`).Scan(&eligibleBefore); err != nil {
		t.Fatal("cannot read server clock for the isolated claim fixture")
	}
	type result struct {
		claim *RetentionClaim
		err   error
	}
	results := make(chan result, 2)
	for i := 0; i < 2; i++ {
		go func() {
			claim, err := repo.claimReadyRetentionJob(ctx, time.Minute, jobID, eligibleBefore)
			results <- result{claim: claim, err: err}
		}()
	}
	var winner *RetentionClaim
	for i := 0; i < 2; i++ {
		result := <-results
		if result.err != nil {
			t.Fatalf("concurrent retention claim failed: %v", result.err)
		}
		if result.claim != nil {
			if winner != nil {
				t.Fatalf("both claimers received leases for job %s", jobID)
			}
			winner = result.claim
		}
	}
	if winner == nil || winner.JobID != jobID || winner.Attempt != 1 {
		t.Fatalf("concurrent claim winner=%+v, want one lease for job %s at attempt 1", winner, jobID)
	}
	var state string
	var attempts int16
	var leaseUntil *time.Time
	if err := pool.QueryRow(ctx, `SELECT state,attempts,lease_until FROM public.document_verification_job WHERE id=$1`, jobID).Scan(&state, &attempts, &leaseUntil); err != nil || state != "procesando" || attempts != 1 || leaseUntil == nil {
		t.Fatalf("persisted claim state=%q attempts=%d lease=%v err=%v", state, attempts, leaseUntil, err)
	}
	var decoyState string
	var decoyAttempts int16
	var decoyLease *time.Time
	if err := pool.QueryRow(ctx, `SELECT state,attempts,lease_until FROM public.document_verification_job WHERE id=$1`, decoyJobID).Scan(&decoyState, &decoyAttempts, &decoyLease); err != nil || decoyState != "pendiente" || decoyAttempts != 0 || decoyLease != nil {
		t.Fatalf("isolated claim changed decoy job: state=%q attempts=%d lease=%v err=%v", decoyState, decoyAttempts, decoyLease, err)
	}
}
