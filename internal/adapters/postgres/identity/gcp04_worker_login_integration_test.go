package postgres_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	identitypg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/identity"
	"github.com/HernanEspinozaDev/espaciGo/internal/privacy"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type gcp04WorkerLoginCleaner struct {
	failuresRemaining int
	calls             int
}

func (c *gcp04WorkerLoginCleaner) Delete(context.Context, string) error {
	c.calls++
	if c.failuresRemaining > 0 {
		c.failuresRemaining--
		return errors.New("synthetic object delete failure")
	}
	return nil
}

func TestGCP04DedicatedWorkerLoginClaimsRetriesAcknowledgesAndFinalizesPostgres18(t *testing.T) {
	if os.Getenv("TEST_DATABASE_URL") == "" {
		t.Skip("TEST_DATABASE_URL is not set; requires disposable PostgreSQL 18")
	}
	if gcp04ReplayCloudMode() {
		t.Skip("this test creates a disposable LOGIN role and only runs against a local disposable PostgreSQL cluster")
	}

	ctx, adminPool := newIdentityTestPool(t)
	var major int
	if err := adminPool.QueryRow(ctx, `SELECT current_setting('server_version_num')::int / 10000`).Scan(&major); err != nil || major != 18 {
		t.Fatalf("worker-login integration test requires PostgreSQL 18: major=%d err=%v", major, err)
	}

	roleName := fmt.Sprintf("gcp04_worker_login_%d", time.Now().UnixNano())
	var err error
	passwordBytes := make([]byte, 32)
	if _, err := rand.Read(passwordBytes); err != nil {
		t.Fatal("generate ephemeral database login password")
	}
	password := hex.EncodeToString(passwordBytes)
	for i := range passwordBytes {
		passwordBytes[i] = 0
	}
	roleIdentifier := pgx.Identifier{roleName}.Sanitize()
	if _, err := adminPool.Exec(ctx, "CREATE ROLE "+roleIdentifier+" LOGIN NOINHERIT NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS PASSWORD '"+password+"'"); err != nil {
		t.Fatalf("create disposable worker login: %v", err)
	}
	var workerPool *pgxpool.Pool
	t.Cleanup(func() {
		if workerPool != nil {
			workerPool.Close()
		}
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := adminPool.Exec(cleanupCtx, "DROP ROLE IF EXISTS "+roleIdentifier); err != nil {
			t.Errorf("drop disposable worker login: %v", err)
		}
	})
	if _, err := adminPool.Exec(ctx, "GRANT espacigo_privacy_cleanup_worker TO "+roleIdentifier+" WITH INHERIT FALSE, SET TRUE"); err != nil {
		t.Fatalf("grant SET ROLE membership to disposable worker login: %v", err)
	}

	workerConfig := adminPool.Config().Copy()
	workerConfig.ConnConfig.User = roleName
	workerConfig.ConnConfig.Password = password
	workerConfig.MinConns = 1
	workerConfig.MaxConns = 2
	workerConfig.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, `SET ROLE espacigo_privacy_cleanup_worker`)
		return err
	}
	workerPool, err = pgxpool.NewWithConfig(ctx, workerConfig)
	if err != nil {
		t.Fatalf("connect through dedicated LOGIN role: %v", err)
	}

	var sessionUser, currentUser string
	if err := workerPool.QueryRow(ctx, `SELECT session_user, current_user`).Scan(&sessionUser, &currentUser); err != nil || sessionUser != roleName || currentUser != "espacigo_privacy_cleanup_worker" {
		t.Fatalf("worker connection did not authenticate and SET ROLE: session_user=%q current_user=%q err=%v", sessionUser, currentUser, err)
	}

	const accountID = "06400000-0000-4000-8000-000000000201"
	const requestID = "06400000-0000-4000-8000-000000000202"
	const executionID = "06400000-0000-4000-8000-000000000203"
	const evidenceID = "06400000-0000-4000-8000-000000000204"
	firstAt := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	if _, err := adminPool.Exec(ctx, `INSERT INTO public.usuario(id,correo_original,correo_normalizado,hash_clave,estado)
		VALUES($1,'worker-login@example.invalid','worker-login@example.invalid','synthetic-fixture','activo')`, accountID); err != nil {
		t.Fatal(err)
	}
	if _, err := adminPool.Exec(ctx, `INSERT INTO public.solicitud_titular(id,usuario_id,tipo,canal,estado,solicitada_en)
		VALUES($1,$2,'supresion','web','en_revision',$3)`, requestID, accountID, firstAt); err != nil {
		t.Fatal(err)
	}
	if _, err := adminPool.Exec(ctx, `INSERT INTO public.ejecucion_baja_local
		(id,solicitud_id,usuario_id,actor_id,clave_idempotencia,estado,motivo_codigo,iniciada_en,detalle)
		VALUES($1,$2,$3,$3,'gcp04-worker-login','limpieza_pendiente','baja_local_minimizada',$4,'{}'::jsonb)`, executionID, requestID, accountID, firstAt); err != nil {
		t.Fatal(err)
	}
	if _, err := adminPool.Exec(ctx, `INSERT INTO public.baja_archivo_pendiente_local(ejecucion_id,evidencia_id,disponible_en)
		VALUES($1,$2,$3)`, executionID, evidenceID, firstAt); err != nil {
		t.Fatal(err)
	}

	workerRepo := identitypg.NewSuppressionCleanupWorkerRepository(workerPool)
	service, err := privacy.NewService(workerRepo)
	if err != nil {
		t.Fatal(err)
	}
	cleaner := &gcp04WorkerLoginCleaner{failuresRemaining: 1}
	clock := firstAt
	if err := service.RunSuppressionCleanupOnceWithWorker(ctx, "06400000-0000-4000-8000-000000000205", func() time.Time { return clock }, cleaner); err != nil {
		t.Fatalf("worker retry/failure path through SET ROLE: %v", err)
	}
	var attempts int
	var completedAt *time.Time
	if err := adminPool.QueryRow(ctx, `SELECT intentos,completada_en FROM public.baja_archivo_pendiente_local WHERE ejecucion_id=$1 AND evidencia_id=$2`, executionID, evidenceID).Scan(&attempts, &completedAt); err != nil || attempts != 1 || completedAt != nil {
		t.Fatalf("first cleaner failure was not recorded as retryable: attempts=%d completed_at=%v err=%v", attempts, completedAt, err)
	}
	clock = firstAt.Add(11 * time.Second)
	if _, err := adminPool.Exec(ctx, `UPDATE public.baja_archivo_pendiente_local SET disponible_en=$3 WHERE ejecucion_id=$1 AND evidencia_id=$2`, executionID, evidenceID, clock); err != nil {
		t.Fatal(err)
	}
	if err := service.RunSuppressionCleanupOnceWithWorker(ctx, "06400000-0000-4000-8000-000000000206", func() time.Time { return clock }, cleaner); err != nil {
		t.Fatalf("worker claim/ack/finalizer path through SET ROLE: %v", err)
	}
	var executionState, requestState string
	var finalQueueRows int
	if err := adminPool.QueryRow(ctx, `SELECT x.estado,s.estado,
		(SELECT count(*) FROM public.baja_archivo_pendiente_local j WHERE j.ejecucion_id=x.id AND j.evidencia_id=$2 AND j.completada_en IS NOT NULL)
		FROM public.ejecucion_baja_local x JOIN public.solicitud_titular s ON s.id=x.solicitud_id WHERE x.id=$1`, executionID, evidenceID).Scan(&executionState, &requestState, &finalQueueRows); err != nil {
		t.Fatal(err)
	}
	if cleaner.calls != 2 || executionState != "completada" || requestState != "resuelta" || finalQueueRows != 1 {
		t.Fatalf("worker lifecycle mismatch calls=%d execution=%q request=%q completed_queue_rows=%d", cleaner.calls, executionState, requestState, finalQueueRows)
	}
}
