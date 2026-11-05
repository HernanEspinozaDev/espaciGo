package documentverificationpg

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresV81OwnershipRaceWaitsOnSpaceBeforeForeignKeyCheck(t *testing.T) {
	dsn := os.Getenv("ESPACIGO_DOCUMENT_VERIFICATION_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ESPACIGO_DOCUMENT_VERIFICATION_TEST_DATABASE_URL is unset")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal("invalid integration DSN")
	}
	if cfg.ConnConfig.Host != "127.0.0.1" || cfg.ConnConfig.Port != 15434 || cfg.ConnConfig.Database != "espacigo_f5_gate_20261101" {
		t.Fatal("refusing integration target outside the authorized F5 synthetic endpoint")
	}
	appName := "v81-lock-order-" + uuid.NewString()
	if cfg.ConnConfig.RuntimeParams == nil {
		cfg.ConnConfig.RuntimeParams = make(map[string]string)
	}
	cfg.ConnConfig.RuntimeParams["application_name"] = appName
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal("cannot connect to authorized F5 synthetic endpoint")
	}
	t.Cleanup(pool.Close)
	var database, setupRole string
	if err := pool.QueryRow(ctx, `SELECT current_database(),current_user`).Scan(&database, &setupRole); err != nil || database != "espacigo_f5_gate_20261101" || setupRole == "espacigo_runtime" {
		t.Fatal("refusing unexpected database or runtime fixture setup role")
	}

	oldOwner, newOwner, spaceID, caseID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	setupTx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		t.Fatal("cannot begin controlled V81 fixture setup")
	}
	setupCommitted := false
	fixtureCleanupSafe := true
	t.Cleanup(func() {
		if !fixtureCleanupSafe {
			t.Error("skipping fixture cleanup because a controlled goroutine did not stop")
			return
		}
		if !setupCommitted {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := setupTx.Rollback(cleanupCtx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
				t.Errorf("rollback controlled V81 fixture setup: %v", err)
			}
		}
		cleanupV81DeadlockFixture(t, pool, oldOwner, newOwner, spaceID, caseID)
	})
	for _, owner := range []string{oldOwner, newOwner} {
		if _, err := setupTx.Exec(ctx, `INSERT INTO public.usuario(id,correo_original,correo_normalizado,hash_clave,estado) VALUES($1,$2,$2,'synthetic','activo')`, owner, owner+"@example.invalid"); err != nil {
			t.Fatal("cannot seed controlled V81 actor")
		}
	}
	if _, err := setupTx.Exec(ctx, `INSERT INTO public.espacio(id,propietario_id,categoria_codigo,titulo,descripcion,superficie_m2,capacidad_maxima,reglas_uso,modalidad_tarifa,precio_base_clp,direccion,zona_horaria,estado) VALUES($1,$2,'sala_multiproposito','V81 controlled lock fixture',repeat('Synthetic V81 description. ',4),30,4,'Synthetic','hora',8000,'Synthetic address','America/Santiago','activa')`, spaceID, oldOwner); err != nil {
		t.Fatal("cannot seed controlled V81 space")
	}
	if _, err := setupTx.Exec(ctx, `INSERT INTO public.tarifa_espacio(espacio_id,version,modalidad,precio_base_clp) VALUES($1,1,'hora',8000)`, spaceID); err != nil {
		t.Fatal("cannot seed controlled V81 tariff")
	}
	if _, err := setupTx.Exec(ctx, `INSERT INTO public.espacio_caracteristicas(espacio_id,categoria_codigo,perfil_version,valores) VALUES($1,'sala_multiproposito',1,'{}'::jsonb)`, spaceID); err != nil {
		t.Fatal("cannot seed controlled V81 attributes")
	}
	if err := setupTx.Commit(ctx); err != nil {
		t.Fatal("cannot commit controlled V81 fixture")
	}
	setupCommitted = true

	transferTx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		t.Fatal("cannot begin transfer-first transaction")
	}
	registerOwnershipRaceTxCleanup(t, transferTx, "controlled ownership transfer")
	if _, err := transferTx.Exec(ctx, `SET LOCAL ROLE espacigo_runtime`); err != nil {
		t.Fatal("cannot assume runtime role for transfer-first transaction")
	}
	var transferBackendPID int32
	if err := transferTx.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&transferBackendPID); err != nil {
		t.Fatal("cannot identify transfer-first backend")
	}
	var lockedSpace string
	if err := transferTx.QueryRow(ctx, `SELECT id::text FROM public.espacio WHERE id=$1 FOR UPDATE`, spaceID).Scan(&lockedSpace); err != nil {
		t.Fatal("cannot acquire transfer-first space lock")
	}

	actorLocked := make(chan int32, 1)
	repository := newWithTransactionBeginner(&v81ActorLockSignalBeginner{pool: pool, actorLocked: actorLocked})
	createResult := make(chan error, 1)
	createDone := make(chan struct{})
	go func() {
		defer close(createDone)
		_, err := repository.CreateCase(ctx, newOwner, dv.PurposeSpace, spaceID, func(dv.Authorization) (dv.Snapshot, error) {
			return dv.Snapshot{ID: caseID, Purpose: dv.PurposeSpace, SubjectID: newOwner, SpaceID: spaceID, State: dv.CaseDraft}, nil
		})
		createResult <- err
	}()
	t.Cleanup(func() {
		select {
		case <-createDone:
		case <-time.After(5 * time.Second):
			fixtureCleanupSafe = false
			t.Error("controlled CreateCase goroutine did not finish before cleanup")
		}
	})
	var createBackendPID int32
	select {
	case createBackendPID = <-actorLocked:
	case <-ctx.Done():
		t.Fatal("CreateCase did not acquire the actor lock")
	}

	spaceWaitDeadline := time.NewTimer(3 * time.Second)
	defer spaceWaitDeadline.Stop()
	spaceWaitPoll := time.NewTicker(10 * time.Millisecond)
	defer spaceWaitPoll.Stop()
	for {
		var waitingOnSpace bool
		if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_catalog.pg_stat_activity WHERE pid=$1 AND application_name=$2 AND state='active' AND wait_event_type='Lock' AND query LIKE '%document_verification_lock_space_owner%' AND $3=ANY(pg_catalog.pg_blocking_pids(pid)))`, createBackendPID, appName, transferBackendPID).Scan(&waitingOnSpace); err != nil {
			t.Fatalf("cannot observe controlled CreateCase lock wait: %v", err)
		}
		if waitingOnSpace {
			break
		}
		select {
		case <-spaceWaitPoll.C:
		case <-spaceWaitDeadline.C:
			t.Fatal("CreateCase did not block on the transfer-locked space row")
		case <-ctx.Done():
			t.Fatal("deadline expired while observing the space-lock wait")
		}
	}

	transferResult := make(chan error, 1)
	transferDone := make(chan struct{})
	go func() {
		defer close(transferDone)
		if _, err := transferTx.Exec(ctx, `UPDATE public.espacio SET propietario_id=$2,actualizado_en=clock_timestamp() WHERE id=$1`, spaceID, newOwner); err != nil {
			transferResult <- err
			return
		}
		transferResult <- transferTx.Commit(ctx)
	}()
	t.Cleanup(func() {
		select {
		case <-transferDone:
		case <-time.After(5 * time.Second):
			fixtureCleanupSafe = false
			t.Error("controlled transfer goroutine did not finish before cleanup")
		}
	})
	select {
	case err := <-transferResult:
		if err != nil {
			t.Fatalf("FK ownership transfer blocked or deadlocked against CreateCase: %v", err)
		}
	case <-ctx.Done():
		t.Fatal("transfer did not complete within the controlled race deadline")
	}
	var createErr error
	select {
	case createErr = <-createResult:
		if createErr != nil && !errors.Is(createErr, dv.ErrConflict) {
			t.Fatalf("CreateCase returned an unexpected result after the transfer: %v", createErr)
		}
	case <-ctx.Done():
		t.Fatal("CreateCase did not finish after transfer committed")
	}
	var totalCases, validNewOwnerCases int
	if err := pool.QueryRow(ctx, `SELECT count(*),count(*) FILTER (WHERE purpose='space' AND subject_id=$2 AND space_id=$3 AND superseded_at IS NULL) FROM public.document_verification_case WHERE id=$1`, caseID, newOwner, spaceID).Scan(&totalCases, &validNewOwnerCases); err != nil {
		t.Fatal("cannot verify controlled V81 case outcome")
	}
	if createErr == nil {
		if totalCases != 1 || validNewOwnerCases != 1 {
			t.Fatalf("successful new-owner case was not persisted exactly once: total=%d valid=%d", totalCases, validNewOwnerCases)
		}
	} else if totalCases != 0 {
		t.Fatalf("conflicted new-owner case left persisted rows: total=%d", totalCases)
	}
	var currentOwner string
	if err := pool.QueryRow(ctx, `SELECT propietario_id::text FROM public.espacio WHERE id=$1`, spaceID).Scan(&currentOwner); err != nil || currentOwner != newOwner {
		t.Fatalf("transfer did not commit as expected: owner=%q err=%v", currentOwner, err)
	}
}
