package documentverificationpg

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresV81OwnershipCheckFailsClosedAfterConcurrentTransfer(t *testing.T) {
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
		t.Fatal("cannot connect to authorized F5 synthetic endpoint")
	}
	t.Cleanup(pool.Close)
	var database, setupRole string
	if err := pool.QueryRow(ctx, `SELECT current_database(),current_user`).Scan(&database, &setupRole); err != nil || database != "espacigo_f5_gate_20261101" || setupRole == "espacigo_runtime" {
		t.Fatal("refusing unexpected database or runtime fixture setup role")
	}
	oldOwner, newOwner, spaceID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	setupTx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		t.Fatal("cannot begin V81 concurrency fixture setup")
	}
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if rollbackErr := setupTx.Rollback(cleanupCtx); rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) {
			t.Errorf("rollback V81 synthetic fixture setup: %v", rollbackErr)
			return
		}
		cleanupTx, err := pool.Begin(cleanupCtx)
		if err != nil {
			t.Errorf("begin V81 concurrency fixture cleanup: %v", err)
			return
		}
		for _, query := range []struct {
			name string
			sql  string
			args []any
		}{
			{"attributes", `DELETE FROM public.espacio_caracteristicas WHERE espacio_id=$1`, []any{spaceID}},
			{"tariff", `DELETE FROM public.tarifa_espacio WHERE espacio_id=$1`, []any{spaceID}},
			{"space", `DELETE FROM public.espacio WHERE id=$1`, []any{spaceID}},
			{"owners", `DELETE FROM public.usuario WHERE id IN($1,$2)`, []any{oldOwner, newOwner}},
		} {
			if _, err := cleanupTx.Exec(cleanupCtx, query.sql, query.args...); err != nil {
				t.Errorf("delete V81 synthetic %s: %v", query.name, err)
				if rollbackErr := cleanupTx.Rollback(cleanupCtx); rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) {
					t.Errorf("rollback V81 synthetic fixture cleanup: %v", rollbackErr)
				}
				return
			}
		}
		if err := cleanupTx.Commit(cleanupCtx); err != nil {
			t.Errorf("commit V81 synthetic fixture cleanup: %v", err)
			return
		}
		var remaining int
		if err := pool.QueryRow(cleanupCtx, `SELECT (SELECT count(*) FROM public.usuario WHERE id IN($1,$2))+(SELECT count(*) FROM public.espacio WHERE id=$3)+(SELECT count(*) FROM public.tarifa_espacio WHERE espacio_id=$3)+(SELECT count(*) FROM public.espacio_caracteristicas WHERE espacio_id=$3)`, oldOwner, newOwner, spaceID).Scan(&remaining); err != nil {
			t.Errorf("verify V81 synthetic fixture cleanup: %v", err)
		} else if remaining != 0 {
			t.Errorf("V81 concurrency fixture cleanup left %d rows", remaining)
		}
	})
	for _, owner := range []string{oldOwner, newOwner} {
		if _, err := setupTx.Exec(ctx, `INSERT INTO public.usuario(id,correo_original,correo_normalizado,hash_clave,estado) VALUES($1,$2,$2,'synthetic','activo')`, owner, owner+"@example.invalid"); err != nil {
			t.Fatal("cannot seed V81 synthetic owner")
		}
	}
	if _, err := setupTx.Exec(ctx, `INSERT INTO public.espacio(id,propietario_id,categoria_codigo,titulo,descripcion,superficie_m2,capacidad_maxima,reglas_uso,modalidad_tarifa,precio_base_clp,direccion,zona_horaria,estado) VALUES($1,$2,'sala_multiproposito','V81 lock fixture',repeat('Synthetic V81 lock description. ',4),30,4,'Synthetic','hora',8000,'Synthetic address','America/Santiago','activa')`, spaceID, oldOwner); err != nil {
		t.Fatal("cannot seed V81 synthetic space")
	}
	if _, err := setupTx.Exec(ctx, `INSERT INTO public.tarifa_espacio(espacio_id,version,modalidad,precio_base_clp) VALUES($1,1,'hora',8000)`, spaceID); err != nil {
		t.Fatal("cannot seed V81 synthetic tariff")
	}
	if _, err := setupTx.Exec(ctx, `INSERT INTO public.espacio_caracteristicas(espacio_id,categoria_codigo,perfil_version,valores) VALUES($1,'sala_multiproposito',1,'{}'::jsonb)`, spaceID); err != nil {
		t.Fatal("cannot seed V81 synthetic attributes")
	}
	if err := setupTx.Commit(ctx); err != nil {
		t.Fatal("cannot commit V81 synthetic fixture")
	}

	readerTx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		t.Fatal("cannot begin repeatable-read ownership check")
	}
	registerOwnershipRaceTxCleanup(t, readerTx, "repeatable-read ownership check")
	mappedReaderTx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		_ = rollbackV81RaceTx(readerTx)
		t.Fatal("cannot begin repeatable-read mapped ownership check")
	}
	registerOwnershipRaceTxCleanup(t, mappedReaderTx, "mapped repeatable-read ownership check")
	if _, err := readerTx.Exec(ctx, `SET LOCAL ROLE espacigo_runtime`); err != nil {
		_ = rollbackV81RaceTx(readerTx)
		_ = rollbackV81RaceTx(mappedReaderTx)
		t.Fatal("cannot assume restricted runtime role")
	}
	if _, err := mappedReaderTx.Exec(ctx, `SET LOCAL ROLE espacigo_runtime`); err != nil {
		_ = rollbackV81RaceTx(readerTx)
		_ = rollbackV81RaceTx(mappedReaderTx)
		t.Fatal("cannot assume restricted runtime role for mapped check")
	}
	var active bool
	if err := readerTx.QueryRow(ctx, `SELECT document_verification_private.document_verification_lock_active_actor($1)`, oldOwner).Scan(&active); err != nil || !active {
		_ = rollbackV81RaceTx(readerTx)
		_ = rollbackV81RaceTx(mappedReaderTx)
		t.Fatal("unable to establish runtime snapshot for ownership race")
	}
	var snapshot int
	if err := mappedReaderTx.QueryRow(ctx, `SELECT 1`).Scan(&snapshot); err != nil || snapshot != 1 {
		_ = rollbackV81RaceTx(readerTx)
		_ = rollbackV81RaceTx(mappedReaderTx)
		t.Fatal("unable to establish mapped runtime ownership snapshot")
	}
	writerTx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		_ = rollbackV81RaceTx(readerTx)
		_ = rollbackV81RaceTx(mappedReaderTx)
		t.Fatal("cannot begin competing ownership transfer")
	}
	registerOwnershipRaceTxCleanup(t, writerTx, "competing ownership transfer")
	if _, err := writerTx.Exec(ctx, `UPDATE public.espacio SET propietario_id=$2,actualizado_en=clock_timestamp() WHERE id=$1`, spaceID, newOwner); err != nil {
		_ = rollbackV81RaceTx(writerTx)
		_ = rollbackV81RaceTx(readerTx)
		_ = rollbackV81RaceTx(mappedReaderTx)
		t.Fatal("competing synthetic ownership transfer failed")
	}
	if err := writerTx.Commit(ctx); err != nil {
		_ = rollbackV81RaceTx(readerTx)
		_ = rollbackV81RaceTx(mappedReaderTx)
		t.Fatal("competing synthetic ownership transfer did not commit")
	}
	var staleOwned bool
	err = readerTx.QueryRow(ctx, `SELECT document_verification_private.document_verification_space_owned_by($1,$2)`, spaceID, oldOwner).Scan(&staleOwned)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "40001" {
		_ = rollbackV81RaceTx(readerTx)
		t.Fatalf("repeatable-read ownership check did not fail closed after transfer: owned=%t err=%v", staleOwned, err)
	}
	if err := rollbackV81RaceTx(readerTx); err != nil && err != pgx.ErrTxClosed {
		t.Fatal("rollback stale ownership snapshot failed")
	}
	mappedOwned, mappedErr := lockSpace(ctx, mappedReaderTx, spaceID, oldOwner)
	if !errors.Is(mappedErr, dv.ErrConflict) || mappedOwned {
		_ = rollbackV81RaceTx(mappedReaderTx)
		t.Fatalf("repository ownership check did not map stale snapshot to conflict: owned=%t err=%v", mappedOwned, mappedErr)
	}
	if err := rollbackV81RaceTx(mappedReaderTx); err != nil && err != pgx.ErrTxClosed {
		t.Fatal("rollback mapped stale ownership snapshot failed")
	}

	checkTx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		t.Fatal("cannot begin fresh ownership check")
	}
	registerOwnershipRaceTxCleanup(t, checkTx, "fresh ownership check")
	if _, err := checkTx.Exec(ctx, `SET LOCAL ROLE espacigo_runtime`); err != nil {
		t.Fatal("cannot assume restricted runtime role for fresh ownership check")
	}
	var oldOwned, newOwned, rowExists bool
	if err := checkTx.QueryRow(ctx, `SELECT document_verification_private.document_verification_lock_space_owner($1,$2)`, spaceID, oldOwner).Scan(&oldOwned); err != nil {
		t.Fatal("old-owner check failed after transfer")
	}
	if err := checkTx.QueryRow(ctx, `SELECT document_verification_private.document_verification_space_owned_by($1,$2)`, spaceID, newOwner).Scan(&newOwned); err != nil {
		t.Fatal("new-owner check failed after transfer")
	}
	if err := checkTx.QueryRow(ctx, `SELECT document_verification_private.document_verification_lock_space_row($1)`, spaceID).Scan(&rowExists); err != nil {
		t.Fatal("space-row lock failed after transfer")
	}
	if oldOwned || !newOwned || !rowExists {
		t.Fatal("fresh runtime ownership check returned stale or incomplete state")
	}
}

func registerOwnershipRaceTxCleanup(t *testing.T, tx pgx.Tx, name string) {
	t.Helper()
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := tx.Rollback(cleanupCtx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			t.Errorf("rollback %s: %v", name, err)
		}
	})
}

func rollbackV81RaceTx(tx pgx.Tx) error {
	cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return tx.Rollback(cleanupCtx)
}

func TestPostgresV81NewOwnerCaseDoesNotDeadlockWithOwnershipTransfer(t *testing.T) {
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
		t.Fatal("cannot begin ownership deadlock fixture setup")
	}
	setupCommitted := false
	fixtureCleanupSafe := true
	t.Cleanup(func() {
		if !fixtureCleanupSafe {
			t.Error("skipping fixture cleanup because a CreateCase goroutine did not stop")
			return
		}
		if !setupCommitted {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if rollbackErr := setupTx.Rollback(cleanupCtx); rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) {
				t.Errorf("rollback ownership deadlock fixture setup: %v", rollbackErr)
			}
		}
		cleanupV81DeadlockFixture(t, pool, oldOwner, newOwner, spaceID, caseID)
	})
	for _, owner := range []string{oldOwner, newOwner} {
		if _, err := setupTx.Exec(ctx, `INSERT INTO public.usuario(id,correo_original,correo_normalizado,hash_clave,estado) VALUES($1,$2,$2,'synthetic','activo')`, owner, owner+"@example.invalid"); err != nil {
			t.Fatal("cannot seed ownership deadlock fixture owner")
		}
	}
	if _, err := setupTx.Exec(ctx, `INSERT INTO public.espacio(id,propietario_id,categoria_codigo,titulo,descripcion,superficie_m2,capacidad_maxima,reglas_uso,modalidad_tarifa,precio_base_clp,direccion,zona_horaria,estado) VALUES($1,$2,'sala_multiproposito','V81 deadlock fixture',repeat('Synthetic V81 description. ',4),30,4,'Synthetic','hora',8000,'Synthetic address','America/Santiago','activa')`, spaceID, oldOwner); err != nil {
		t.Fatal("cannot seed ownership deadlock fixture space")
	}
	if _, err := setupTx.Exec(ctx, `INSERT INTO public.tarifa_espacio(espacio_id,version,modalidad,precio_base_clp) VALUES($1,1,'hora',8000)`, spaceID); err != nil {
		t.Fatal("cannot seed ownership deadlock fixture tariff")
	}
	if _, err := setupTx.Exec(ctx, `INSERT INTO public.espacio_caracteristicas(espacio_id,categoria_codigo,perfil_version,valores) VALUES($1,'sala_multiproposito',1,'{}'::jsonb)`, spaceID); err != nil {
		t.Fatal("cannot seed ownership deadlock fixture attributes")
	}
	if err := setupTx.Commit(ctx); err != nil {
		t.Fatal("cannot commit ownership deadlock fixture")
	}
	setupCommitted = true

	// Ownership changes have no separate application mutation; this UPDATE is the
	// production trigger path, with the target space locked before its FK check.
	transferTx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		t.Fatal("cannot begin ownership transfer transaction")
	}
	registerOwnershipRaceTxCleanup(t, transferTx, "ownership transfer transaction")
	if _, err := transferTx.Exec(ctx, `SET LOCAL ROLE espacigo_runtime`); err != nil {
		t.Fatal("cannot assume runtime role for ownership transfer")
	}
	var lockedSpace string
	if err := transferTx.QueryRow(ctx, `SELECT id::text FROM public.espacio WHERE id=$1 FOR UPDATE`, spaceID).Scan(&lockedSpace); err != nil {
		t.Fatal("cannot establish transfer-first space lock")
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
			t.Error("CreateCase goroutine did not finish before fixture cleanup")
		}
	})
	select {
	case <-actorLocked:
	case <-ctx.Done():
		t.Fatal("CreateCase did not acquire its actor lock before the transfer race")
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
			t.Error("ownership transfer goroutine did not finish before fixture cleanup")
		}
	})
	select {
	case err := <-transferResult:
		if err != nil {
			t.Fatalf("ownership transfer deadlocked or failed while creating a new-owner case: %v", err)
		}
	case <-ctx.Done():
		t.Fatal("ownership transfer did not complete within the deadlock test deadline")
	}
	var createErr error
	select {
	case createErr = <-createResult:
		if createErr != nil && !errors.Is(createErr, dv.ErrConflict) {
			t.Fatalf("new-owner case failed with an unexpected result: %v", createErr)
		}
	case <-ctx.Done():
		t.Fatal("new-owner case did not finish after the ownership transfer")
	}
	var totalCases, validNewOwnerCases int
	if err := pool.QueryRow(ctx, `SELECT count(*),count(*) FILTER (WHERE purpose='space' AND subject_id=$2 AND space_id=$3 AND superseded_at IS NULL) FROM public.document_verification_case WHERE id=$1`, caseID, newOwner, spaceID).Scan(&totalCases, &validNewOwnerCases); err != nil {
		t.Fatal("cannot verify serialized new-owner case result")
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
		t.Fatalf("ownership transfer did not commit as expected: owner=%q err=%v", currentOwner, err)
	}
}

type v81ActorLockSignalBeginner struct {
	pool        *pgxpool.Pool
	actorLocked chan int32
	once        sync.Once
}

func (b *v81ActorLockSignalBeginner) BeginTx(ctx context.Context, options pgx.TxOptions) (pgx.Tx, error) {
	tx, err := b.pool.BeginTx(ctx, options)
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `SET LOCAL ROLE espacigo_runtime`); err != nil {
		if rollbackErr := tx.Rollback(ctx); rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) {
			err = errors.Join(err, rollbackErr)
		}
		return nil, err
	}
	var backendPID int32
	if err := tx.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&backendPID); err != nil {
		if rollbackErr := rollbackV81RaceTx(tx); rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) {
			err = errors.Join(err, rollbackErr)
		}
		return nil, err
	}
	return &v81ActorLockSignalTx{Tx: tx, actorLocked: b.actorLocked, backendPID: backendPID, once: &b.once}, nil
}

type v81ActorLockSignalTx struct {
	pgx.Tx
	actorLocked chan int32
	backendPID  int32
	once        *sync.Once
}

func (tx *v81ActorLockSignalTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	row := tx.Tx.QueryRow(ctx, sql, args...)
	if strings.Contains(sql, "document_verification_lock_active_actor") {
		return v81ActorLockSignalRow{Row: row, actorLocked: tx.actorLocked, backendPID: tx.backendPID, once: tx.once}
	}
	return row
}

type v81ActorLockSignalRow struct {
	pgx.Row
	actorLocked chan int32
	backendPID  int32
	once        *sync.Once
}

func (row v81ActorLockSignalRow) Scan(dest ...any) error {
	if err := row.Row.Scan(dest...); err != nil {
		return err
	}
	row.once.Do(func() { row.actorLocked <- row.backendPID })
	return nil
}

func cleanupV81DeadlockFixture(t *testing.T, pool *pgxpool.Pool, oldOwner, newOwner, spaceID, caseID string) {
	t.Helper()
	cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cleanupTx, err := pool.Begin(cleanupCtx)
	if err != nil {
		t.Errorf("begin ownership deadlock fixture cleanup: %v", err)
		return
	}
	failed := false
	for _, statement := range []struct {
		name string
		sql  string
		args []any
	}{
		{"events", `DELETE FROM public.document_verification_event WHERE case_id=$1`, []any{caseID}},
		{"case", `DELETE FROM public.document_verification_case WHERE id=$1`, []any{caseID}},
		{"attributes", `DELETE FROM public.espacio_caracteristicas WHERE espacio_id=$1`, []any{spaceID}},
		{"tariff", `DELETE FROM public.tarifa_espacio WHERE espacio_id=$1`, []any{spaceID}},
		{"space", `DELETE FROM public.espacio WHERE id=$1`, []any{spaceID}},
		{"owners", `DELETE FROM public.usuario WHERE id IN($1,$2)`, []any{oldOwner, newOwner}},
	} {
		if _, err := cleanupTx.Exec(cleanupCtx, statement.sql, statement.args...); err != nil {
			t.Errorf("delete ownership deadlock fixture %s: %v", statement.name, err)
			failed = true
			break
		}
	}
	if failed {
		if err := cleanupTx.Rollback(cleanupCtx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			t.Errorf("rollback ownership deadlock fixture cleanup: %v", err)
		}
	} else if err := cleanupTx.Commit(cleanupCtx); err != nil {
		t.Errorf("commit ownership deadlock fixture cleanup: %v", err)
	}
	var remaining int
	if err := pool.QueryRow(cleanupCtx, `SELECT (SELECT count(*) FROM public.usuario WHERE id IN($1,$2))+(SELECT count(*) FROM public.espacio WHERE id=$3)+(SELECT count(*) FROM public.tarifa_espacio WHERE espacio_id=$3)+(SELECT count(*) FROM public.espacio_caracteristicas WHERE espacio_id=$3)+(SELECT count(*) FROM public.document_verification_case WHERE id=$4)+(SELECT count(*) FROM public.document_verification_event WHERE case_id=$4)`, oldOwner, newOwner, spaceID, caseID).Scan(&remaining); err != nil {
		t.Errorf("verify ownership deadlock fixture cleanup: %v", err)
	} else if remaining != 0 {
		t.Errorf("ownership deadlock fixture cleanup left %d rows", remaining)
	}
}
