package migrator

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	identitydeclarationpg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/identitydeclaration"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// assertV87IdentityDeclarationRetentionAndWorker exercises the V86 purge and
// V87 dedicated-worker boundary only in the disposable migration CI database.
func assertV87IdentityDeclarationRetentionAndWorker(t *testing.T, ctx context.Context, databaseURL string, admin *pgx.Conn) {
	t.Helper()
	accountID := uuid.NewString()
	referencedID, futureID, attemptID, idempotencyID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	defer func() {
		if _, err := admin.Exec(context.Background(), `DELETE FROM public.identity_provider_attempt WHERE account_id=$1`, accountID); err != nil {
			t.Errorf("remove synthetic retention attempt: %v", err)
		}
		if _, err := admin.Exec(context.Background(), `DELETE FROM public.identity_declaration_revision WHERE account_id=$1`, accountID); err != nil {
			t.Errorf("remove synthetic retention declarations: %v", err)
		}
		if _, err := admin.Exec(context.Background(), `DELETE FROM public.usuario WHERE id=$1`, accountID); err != nil {
			t.Errorf("remove synthetic retention account: %v", err)
		}
		var declarations, attempts, users int
		if err := admin.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM public.identity_declaration_revision WHERE account_id=$1),(SELECT count(*) FROM public.identity_provider_attempt WHERE account_id=$1),(SELECT count(*) FROM public.usuario WHERE id=$1)`, accountID).Scan(&declarations, &attempts, &users); err != nil || declarations != 0 || attempts != 0 || users != 0 {
			t.Errorf("synthetic retention cleanup left declarations=%d attempts=%d users=%d: %v", declarations, attempts, users, err)
		}
	}()
	if _, err := admin.Exec(ctx, `INSERT INTO public.usuario(id,correo_original,correo_normalizado,hash_clave,estado) VALUES ($1,$2,$2,'synthetic-only','activo')`, accountID, accountID+"@example.invalid"); err != nil {
		t.Fatal(err)
	}
	poolConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	poolConfig.MaxConns = 2
	poolConfig.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, `SET ROLE espacigo_runtime`)
		return err
	}
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	repository := identitydeclarationpg.New(pool)
	if deleted, err := repository.PurgeExpiredRevisions(ctx); err != nil || deleted != 0 {
		t.Fatalf("empty V86 purge returned deleted=%d err=%v", deleted, err)
	}
	workerConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	workerConfig.MaxConns = 1
	workerConfig.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		if _, err := conn.Exec(ctx, `SET ROLE espacigo_privacy_worker_login`); err != nil {
			return err
		}
		_, err := conn.Exec(ctx, `SET ROLE espacigo_privacy_cleanup_worker`)
		return err
	}
	workerPool, err := pgxpool.NewWithConfig(ctx, workerConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer workerPool.Close()
	var workerDeleted int64
	if err := workerPool.QueryRow(ctx, `SELECT public.purge_expired_identity_declaration_revisions()`).Scan(&workerDeleted); err != nil || workerDeleted != 0 {
		t.Fatalf("dedicated cleanup worker cannot invoke bounded purge: deleted=%d err=%v", workerDeleted, err)
	}

	for revision := int64(1); revision <= 101; revision++ {
		if _, err := admin.Exec(ctx, `INSERT INTO public.identity_declaration_revision(account_id,revision,key_version,ciphertext,schema_version,consent_version,consent_given_at,retention_at,created_at) VALUES($1,$2,'synthetic-key',decode(repeat('ab',29),'hex'),'1','test-v1',now()-interval '48 hours',now()-interval '24 hours',now()-interval '48 hours')`, accountID, revision); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := admin.Exec(ctx, `INSERT INTO public.identity_declaration_revision(id,account_id,revision,key_version,ciphertext,schema_version,consent_version,consent_given_at,retention_at,created_at) VALUES($1,$2,102,'synthetic-key',decode(repeat('ab',29),'hex'),'1','test-v1',now()-interval '48 hours',now()-interval '24 hours',now()-interval '48 hours'),($3,$2,103,'synthetic-key',decode(repeat('ab',29),'hex'),'1','test-v1',now()-interval '24 hours',now()+interval '24 hours',now()-interval '24 hours')`, referencedID, accountID, futureID); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `INSERT INTO public.identity_provider_attempt(id,account_id,declaration_id,provider,environment,application_id,workflow_id,workflow_version,idempotency_key,status,retention_at,created_at,updated_at) VALUES($1,$2,$3,'didit','SANDBOX','synthetic-app','synthetic-workflow','1',$4,'FAILED',now()-interval '24 hours',now()-interval '48 hours',now()-interval '48 hours')`, attemptID, accountID, referencedID, idempotencyID); err != nil {
		t.Fatal(err)
	}

	if deleted, err := repository.PurgeExpiredRevisions(ctx); err != nil || deleted != 100 {
		t.Fatalf("first bounded purge returned deleted=%d err=%v; want 100", deleted, err)
	}
	var expiredRemaining int
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM public.identity_declaration_revision WHERE account_id=$1 AND retention_at<=clock_timestamp()`, accountID).Scan(&expiredRemaining); err != nil || expiredRemaining != 2 {
		t.Fatalf("first bounded purge left %d expired rows; want one batch remainder and one FK reference: %v", expiredRemaining, err)
	}
	if deleted, err := repository.PurgeExpiredRevisions(ctx); err != nil || deleted != 1 {
		t.Fatalf("second bounded purge returned deleted=%d err=%v; want 1", deleted, err)
	}
	if deleted, err := repository.PurgeExpiredRevisions(ctx); err != nil || deleted != 0 {
		t.Fatalf("purge removed an expired declaration still referenced by an attempt: deleted=%d err=%v", deleted, err)
	}
	var retainedReferenced, retainedFuture bool
	if err := admin.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.identity_declaration_revision WHERE id=$1),EXISTS(SELECT 1 FROM public.identity_declaration_revision WHERE id=$2)`, referencedID, futureID).Scan(&retainedReferenced, &retainedFuture); err != nil || !retainedReferenced || !retainedFuture {
		t.Fatalf("purge did not retain referenced/future declarations: referenced=%t future=%t err=%v", retainedReferenced, retainedFuture, err)
	}

	var securityDefiner, safeSearchPath, runtimeCanExecute, workerCanExecute, publicCanExecute, runtimeCanDelete, workerCanDelete bool
	var owner string
	var argumentCount int
	if err := admin.QueryRow(ctx, `SELECT p.prosecdef,pg_catalog.pg_get_userbyid(p.proowner),p.pronargs,p.proconfig @> ARRAY['search_path=pg_catalog'],has_function_privilege('espacigo_runtime',p.oid,'EXECUTE'),has_function_privilege('espacigo_privacy_cleanup_worker',p.oid,'EXECUTE'),EXISTS(SELECT 1 FROM pg_catalog.aclexplode(COALESCE(p.proacl,pg_catalog.acldefault('f',p.proowner))) acl WHERE acl.grantee=0 AND acl.privilege_type='EXECUTE'),has_table_privilege('espacigo_runtime','public.identity_declaration_revision','DELETE'),has_table_privilege('espacigo_privacy_cleanup_worker','public.identity_declaration_revision','DELETE') FROM pg_catalog.pg_proc p WHERE p.oid='public.purge_expired_identity_declaration_revisions()'::regprocedure`).Scan(&securityDefiner, &owner, &argumentCount, &safeSearchPath, &runtimeCanExecute, &workerCanExecute, &publicCanExecute, &runtimeCanDelete, &workerCanDelete); err != nil || !securityDefiner || owner != "espacigo_schema_owner" || argumentCount != 0 || !safeSearchPath || !runtimeCanExecute || !workerCanExecute || publicCanExecute || runtimeCanDelete || workerCanDelete {
		t.Fatalf("unsafe purge function contract: definer=%t owner=%q args=%d safe_path=%t runtime_exec=%t worker_exec=%t public_exec=%t runtime_delete=%t worker_delete=%t err=%v", securityDefiner, owner, argumentCount, safeSearchPath, runtimeCanExecute, workerCanExecute, publicCanExecute, runtimeCanDelete, workerCanDelete, err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM public.identity_declaration_revision WHERE id=$1`, futureID); err == nil {
		t.Fatal("runtime role directly deleted an identity declaration")
	} else {
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
			t.Fatalf("runtime direct delete failed for unexpected reason: %v", err)
		}
	}
	if _, err := workerPool.Exec(ctx, `DELETE FROM public.identity_declaration_revision WHERE id=$1`, futureID); err == nil {
		t.Fatal("dedicated cleanup role directly deleted an identity declaration")
	} else {
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
			t.Fatalf("cleanup-worker direct delete failed for unexpected reason: %v", err)
		}
	}

	for revision := int64(104); revision <= 113; revision++ {
		if _, err := admin.Exec(ctx, `INSERT INTO public.identity_declaration_revision(account_id,revision,key_version,ciphertext,schema_version,consent_version,consent_given_at,retention_at,created_at) VALUES($1,$2,'synthetic-key',decode(repeat('ab',29),'hex'),'1','test-v1',now()-interval '48 hours',now()-interval '24 hours',now()-interval '48 hours')`, accountID, revision); err != nil {
			t.Fatal(err)
		}
	}
	type purgeResult struct {
		deleted int64
		err     error
	}
	start := make(chan struct{})
	results := make(chan purgeResult, 2)
	var callers sync.WaitGroup
	for i := 0; i < 2; i++ {
		callers.Add(1)
		go func() {
			defer callers.Done()
			<-start
			deleted, err := repository.PurgeExpiredRevisions(ctx)
			results <- purgeResult{deleted: deleted, err: err}
		}()
	}
	close(start)
	callers.Wait()
	close(results)
	var concurrentDeleted int64
	for result := range results {
		if result.err != nil {
			t.Fatalf("concurrent purge failed: %v", result.err)
		}
		concurrentDeleted += result.deleted
	}
	if concurrentDeleted != 10 {
		t.Fatalf("concurrent purge deleted %d rows; want exactly 10", concurrentDeleted)
	}
	raceRevisionID, raceAttemptID, raceIdempotencyID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	if _, err := admin.Exec(ctx, `INSERT INTO public.identity_declaration_revision(id,account_id,revision,key_version,ciphertext,schema_version,consent_version,consent_given_at,retention_at,created_at) VALUES($1,$2,114,'synthetic-key',decode(repeat('ab',29),'hex'),'1','test-v1',now()-interval '48 hours',now()-interval '24 hours',now()-interval '48 hours')`, raceRevisionID, accountID); err != nil {
		t.Fatal(err)
	}
	raceCtx, cancelRace := context.WithTimeout(ctx, 2*time.Second)
	defer cancelRace()
	tx, err := pool.Begin(raceCtx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	if _, err := tx.Exec(raceCtx, `INSERT INTO public.identity_provider_attempt(id,account_id,declaration_id,provider,environment,application_id,workflow_id,workflow_version,idempotency_key,status,retention_at,created_at,updated_at) VALUES($1,$2,$3,'didit','SANDBOX','synthetic-app','synthetic-workflow','1',$4,'FAILED',now()-interval '24 hours',now()-interval '48 hours',now()-interval '48 hours')`, raceAttemptID, accountID, raceRevisionID, raceIdempotencyID); err != nil {
		t.Fatalf("insert concurrent FK reference as runtime: %v", err)
	}
	if deleted, err := repository.PurgeExpiredRevisions(raceCtx); err != nil || deleted != 0 {
		t.Fatalf("purge raced past uncommitted provider FK insert: deleted=%d err=%v", deleted, err)
	}
	if err := tx.Commit(raceCtx); err != nil {
		t.Fatalf("commit concurrent provider FK insert: %v", err)
	}
	var racedReferenceRetained bool
	if err := admin.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.identity_declaration_revision WHERE id=$1)`, raceRevisionID).Scan(&racedReferenceRetained); err != nil || !racedReferenceRetained {
		t.Fatalf("provider FK race lost the referenced declaration: retained=%t err=%v", racedReferenceRetained, err)
	}
}
