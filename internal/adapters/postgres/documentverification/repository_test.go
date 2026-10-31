package documentverificationpg

import (
	"context"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
	"github.com/HernanEspinozaDev/espaciGo/internal/identityverification"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Opt-in PostgreSQL coverage. It expects V80 to have been applied by the
// separately authorized parent workflow; this test never runs migrations.
func TestPostgresCreateTransitionAndRollback(t *testing.T) {
	dsn := os.Getenv("ESPACIGO_DOCUMENT_VERIFICATION_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ESPACIGO_DOCUMENT_VERIFICATION_TEST_DATABASE_URL is unset")
	}
	ctx := context.Background()
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal("invalid integration DSN")
	}
	if cfg.ConnConfig.Host != "127.0.0.1" || cfg.ConnConfig.Port != 15433 || cfg.ConnConfig.Database != "espacigo_f5_synth_20261029" {
		t.Fatal("refusing integration target outside the authorized F5 synthetic endpoint")
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pool.Close() })
	var db string
	if err := pool.QueryRow(ctx, `SELECT current_database()`).Scan(&db); err != nil {
		t.Fatal(err)
	}
	if db != "espacigo_f5_synth_20261029" {
		t.Fatalf("refusing unexpected integration database %q", db)
	}

	actor, outsider, caseID, fileID, rollbackFileID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	fixtureStarted := false
	t.Cleanup(func() {
		if !fixtureStarted {
			return
		}
		cleanup := context.Background()
		for _, item := range []struct {
			name  string
			query string
			id    string
		}{
			{"files", `DELETE FROM public.document_verification_file WHERE id=$1`, fileID},
			{"rollback file", `DELETE FROM public.document_verification_file WHERE id=$1`, rollbackFileID},
			{"case", `DELETE FROM public.document_verification_case WHERE id=$1`, caseID},
			{"actor", `DELETE FROM public.usuario WHERE id=$1`, actor},
			{"outsider", `DELETE FROM public.usuario WHERE id=$1`, outsider},
		} {
			tag, cleanupErr := pool.Exec(cleanup, item.query, item.id)
			if cleanupErr != nil {
				t.Errorf("cleanup %s failed: %v", item.name, cleanupErr)
				continue
			}
			if tag.RowsAffected() > 1 {
				t.Errorf("cleanup %s affected %d rows, want at most 1", item.name, tag.RowsAffected())
			}
		}
		var remaining int
		if err := pool.QueryRow(cleanup, `SELECT (SELECT count(*) FROM public.document_verification_file WHERE id IN($1,$2))+(SELECT count(*) FROM public.document_verification_job WHERE file_id IN($1,$2))+(SELECT count(*) FROM public.document_verification_case WHERE id=$3)+(SELECT count(*) FROM public.usuario WHERE id=$4)+(SELECT count(*) FROM public.usuario WHERE id=$5)`, fileID, rollbackFileID, caseID, actor, outsider).Scan(&remaining); err != nil {
			t.Errorf("verify fixture cleanup failed: %v", err)
		} else if remaining != 0 {
			t.Errorf("fixture cleanup left %d non-audit rows", remaining)
		}
		var retainedEvents int
		if err := pool.QueryRow(cleanup, `SELECT count(*) FROM public.document_verification_event WHERE case_id=$1`, caseID).Scan(&retainedEvents); err != nil {
			t.Errorf("verify retained immutable audit events failed: %v", err)
		} else {
			t.Logf("retained immutable audit event rows=%d", retainedEvents)
		}
	})
	_, err = pool.Exec(ctx, `INSERT INTO public.usuario(id,correo_original,correo_normalizado,hash_clave,estado) VALUES($1,$2,$2,'synthetic','activo'),($3,$4,$4,'synthetic','activo')`, actor, actor+"@example.invalid", outsider, outsider+"@example.invalid")
	if err != nil {
		t.Fatal(err)
	}
	fixtureStarted = true
	repo := New(pool)
	created := dv.Snapshot{Purpose: dv.PurposeIdentity, ID: caseID, SubjectID: actor, State: dv.CaseDraft, DocumentExpiresAt: time.Now().AddDate(1, 0, 0)}
	created, err = repo.CreateCase(ctx, actor, dv.PurposeIdentity, "", func(a dv.Authorization) (dv.Snapshot, error) { return created, nil })
	if err != nil {
		t.Fatal(err)
	}
	service, err := dv.NewService(repo)
	if err != nil {
		t.Fatal(err)
	}
	file := dv.File{ID: fileID, State: dv.FilePending, Size: 128, MIME: "image/png", Pages: 1, ContentSHA256: [32]byte{1}}
	if err := service.AddFile(ctx, actor, caseID, dv.PurposeIdentity, created.Revision, file); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.LoadOwnCase(ctx, outsider, caseID); !errors.Is(err, dv.ErrNotFound) {
		t.Fatalf("other owner read error=%v, want not found", err)
	}
	loaded, err := repo.LoadOwnCase(ctx, actor, caseID)
	if err != nil || len(loaded.Files) != 1 || loaded.Files[0] != file {
		t.Fatalf("owner rehydration files=%+v err=%v", loaded.Files, err)
	}
	var stored dv.File
	var storedHash []byte
	if err := pool.QueryRow(ctx, `SELECT id::text,state,size_bytes,mime_type,pages,content_sha256 FROM public.document_verification_file WHERE case_id=$1`, caseID).Scan(&stored.ID, &stored.State, &stored.Size, &stored.MIME, &stored.Pages, &storedHash); err != nil {
		t.Fatal(err)
	}
	copy(stored.ContentSHA256[:], storedHash)
	if stored != file {
		t.Fatalf("persisted file metadata = %+v, want %+v", stored, file)
	}
	var scanJobs, caseCreatedEvents, fileAddedEvents int
	if err := pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM public.document_verification_job WHERE file_id=$1 AND kind='scan' AND state='pendiente'),(SELECT count(*) FROM public.document_verification_event WHERE case_id=$2 AND action='case_created' AND actor_id=$3 AND result_code='borrador'),(SELECT count(*) FROM public.document_verification_event WHERE case_id=$2 AND action='file_added' AND file_id=$1 AND actor_id=$3 AND result_code='pendiente')`, fileID, caseID, actor).Scan(&scanJobs, &caseCreatedEvents, &fileAddedEvents); err != nil {
		t.Fatal(err)
	}
	if scanJobs != 1 || caseCreatedEvents != 1 || fileAddedEvents != 1 {
		t.Fatalf("persisted workflow side effects: scan jobs=%d case-created events=%d file-added events=%d", scanJobs, caseCreatedEvents, fileAddedEvents)
	}
	if err := repo.Transition(ctx, actor, dv.PurposeIdentity, caseID, 1, dv.TransitionAudit{Action: "case_transitioned", ResultCode: string(dv.CaseRejected)}, func(x *dv.Snapshot, _ dv.Authorization) error { x.State = dv.CaseRejected; return nil }); !errors.Is(err, dv.ErrConflict) {
		t.Fatalf("stale revision should conflict, got %v", err)
	}
	if err := repo.Transition(ctx, actor, dv.PurposeIdentity, caseID, 2, dv.TransitionAudit{Action: "review_decision", ResultCode: string(dv.CaseRejected), ReasonCode: dv.ReasonDocumentInvalid}, func(x *dv.Snapshot, _ dv.Authorization) error { x.State = dv.CaseRejected; return context.Canceled }); err == nil {
		t.Fatal("callback error unexpectedly committed")
	}
	rollbackFile := dv.File{ID: rollbackFileID, State: dv.FilePending, Size: 128, MIME: "image/png", Pages: 1, ContentSHA256: [32]byte{3}}
	err = repo.Transition(ctx, actor, dv.PurposeIdentity, caseID, 2, dv.TransitionAudit{Action: "invalid_action", ResultCode: string(dv.CaseSubmitted)}, func(x *dv.Snapshot, _ dv.Authorization) error {
		x.Files = append(x.Files, rollbackFile)
		x.State = dv.CaseSubmitted
		return nil
	})
	if !errors.Is(err, dv.ErrConflict) {
		t.Fatalf("late database audit failure error=%v, want conflict", err)
	}
	var state string
	var revision int64
	if err := pool.QueryRow(ctx, `SELECT state,revision FROM public.document_verification_case WHERE id=$1`, caseID).Scan(&state, &revision); err != nil {
		t.Fatal(err)
	}
	var events int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM public.document_verification_event WHERE case_id=$1`, caseID).Scan(&events); err != nil {
		t.Fatal(err)
	}
	var files, jobs int
	if err := pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM public.document_verification_file WHERE case_id=$1),(SELECT count(*) FROM public.document_verification_job WHERE file_id IN($2,$3))`, caseID, fileID, rollbackFileID).Scan(&files, &jobs); err != nil {
		t.Fatal(err)
	}
	if state != string(dv.CaseDraft) || revision != 2 || events != 2 || files != 1 || jobs != 1 {
		t.Fatalf("unexpected atomic result: %s revision %d events %d files %d jobs %d", state, revision, events, files, jobs)
	}
	for name, query := range map[string]string{
		"update":   `UPDATE public.document_verification_event SET result_code='tampered' WHERE case_id=$1`,
		"delete":   `DELETE FROM public.document_verification_event WHERE case_id=$1`,
		"truncate": `TRUNCATE TABLE public.document_verification_event`,
	} {
		t.Run("immutable_audit_"+name, func(t *testing.T) {
			var operationErr error
			if name == "truncate" {
				_, operationErr = pool.Exec(ctx, query)
			} else {
				_, operationErr = pool.Exec(ctx, query, caseID)
			}
			var pgErr *pgconn.PgError
			if !errors.As(operationErr, &pgErr) || pgErr.Code != "23514" {
				t.Fatalf("%s audit mutation error=%v, want SQLSTATE 23514", name, operationErr)
			}
		})
	}
}

type savepointBeginner struct{ tx pgx.Tx }

func (b savepointBeginner) BeginTx(ctx context.Context, _ pgx.TxOptions) (pgx.Tx, error) {
	return b.tx.Begin(ctx)
}

type commitSerializationFailureBeginner struct{ tx pgx.Tx }

func (b commitSerializationFailureBeginner) BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error) {
	return b.tx, nil
}

type commitSerializationFailureTx struct {
	pgx.Tx
	actor, caseID, spaceID string
}

func (tx commitSerializationFailureTx) QueryRow(_ context.Context, sql string, _ ...any) pgx.Row {
	switch {
	case strings.Contains(sql, "document_verification_lock_active_actor"):
		return commitSerializationFailureRow{values: []any{true}}
	case strings.Contains(sql, "SELECT space_id::text FROM public.document_verification_case"):
		return commitSerializationFailureRow{values: []any{nil}}
	case strings.Contains(sql, "WHERE c.id=$1 AND c.subject_id=$2"):
		return commitSerializationFailureRow{values: []any{tx.caseID, "identity", tx.actor, nil, "borrador", int64(1), "", nil, nil, nil, nil, nil}}
	case strings.Contains(sql, "WHERE c.id=$1 AND c.purpose='space'"):
		return commitSerializationFailureRow{values: []any{tx.caseID, "space", tx.actor, tx.spaceID, "borrador", int64(1), "", nil, nil, nil, nil, nil}}
	case strings.Contains(sql, "SELECT id::text,purpose,subject_id::text,space_id::text,state,revision"):
		return commitSerializationFailureRow{values: []any{tx.caseID, "identity", tx.actor, nil, "borrador", int64(1), "", nil, nil, nil, nil, nil}}
	default:
		return commitSerializationFailureRow{err: errors.New("unexpected serialization test query")}
	}
}

func (commitSerializationFailureTx) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return emptyCommitSerializationRows{}, nil
}

func (commitSerializationFailureTx) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (commitSerializationFailureTx) Commit(context.Context) error {
	return &pgconn.PgError{Code: "40001"}
}

func (commitSerializationFailureTx) Rollback(context.Context) error { return nil }

type commitSerializationFailureRow struct {
	values []any
	err    error
}

func (r commitSerializationFailureRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) != len(r.values) {
		return errors.New("unexpected serialization test scan destination count")
	}
	for i, destination := range dest {
		if scanner, ok := destination.(interface{ Scan(any) error }); ok {
			if err := scanner.Scan(r.values[i]); err != nil {
				return err
			}
			continue
		}
		target := reflect.ValueOf(destination)
		if target.Kind() != reflect.Ptr || target.IsNil() {
			return errors.New("unexpected serialization test scan destination")
		}
		target = target.Elem()
		if r.values[i] == nil {
			target.SetZero()
			continue
		}
		value := reflect.ValueOf(r.values[i])
		if target.Kind() == reflect.Ptr {
			valueType := target.Type().Elem()
			if !value.Type().AssignableTo(valueType) && !value.Type().ConvertibleTo(valueType) {
				return errors.New("unexpected serialization test scan value")
			}
			pointer := reflect.New(valueType)
			if value.Type() != valueType {
				value = value.Convert(valueType)
			}
			pointer.Elem().Set(value)
			target.Set(pointer)
			continue
		}
		if !value.Type().AssignableTo(target.Type()) && !value.Type().ConvertibleTo(target.Type()) {
			return errors.New("unexpected serialization test scan value")
		}
		if value.Type() != target.Type() {
			value = value.Convert(target.Type())
		}
		target.Set(value)
	}
	return nil
}

type emptyCommitSerializationRows struct{ pgx.Rows }

func (emptyCommitSerializationRows) Next() bool { return false }
func (emptyCommitSerializationRows) Err() error { return nil }
func (emptyCommitSerializationRows) Close()     {}

func TestOwnerReadersMapCommitSerializationFailuresToConflict(t *testing.T) {
	const actor = "00000000-0000-4000-8000-000000000001"
	const caseID = "00000000-0000-4000-8000-000000000002"
	const spaceID = "00000000-0000-4000-8000-000000000003"
	readers := []struct {
		name string
		read func(*Repository) error
	}{
		{name: "list", read: func(r *Repository) error { _, err := r.ListOwnCases(context.Background(), actor); return err }},
		{name: "load", read: func(r *Repository) error { _, err := r.LoadOwnCase(context.Background(), actor, caseID); return err }},
		{name: "download", read: func(r *Repository) error {
			_, _, err := r.LoadSpaceCaseForDownload(context.Background(), actor, caseID)
			return err
		}},
	}
	for _, reader := range readers {
		t.Run(reader.name, func(t *testing.T) {
			tx := commitSerializationFailureTx{actor: actor, caseID: caseID, spaceID: spaceID}
			repo := newWithTransactionBeginner(commitSerializationFailureBeginner{tx: tx})
			if err := reader.read(repo); !errors.Is(err, dv.ErrConflict) {
				t.Fatalf("commit serialization failure error=%v, want conflict", err)
			}
		})
	}
}

func TestTransitionMapsCommitSerializationFailureToConflict(t *testing.T) {
	const (
		actor  = "00000000-0000-4000-8000-000000000001"
		caseID = "00000000-0000-4000-8000-000000000002"
	)
	tx := commitSerializationFailureTx{actor: actor, caseID: caseID}
	repo := newWithTransactionBeginner(commitSerializationFailureBeginner{tx: tx})
	err := repo.Transition(context.Background(), actor, dv.PurposeIdentity, caseID, 1, dv.TransitionAudit{Action: "test", ResultCode: "test"}, func(*dv.Snapshot, dv.Authorization) error { return nil })
	if !errors.Is(err, dv.ErrConflict) {
		t.Fatalf("commit serialization failure error=%v, want conflict", err)
	}
}

type lostRetentionClaimBeginner struct{ tx pgx.Tx }

func (b lostRetentionClaimBeginner) BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error) {
	return b.tx, nil
}

type lostRetentionClaimTx struct {
	pgx.Tx
	jobID, fileID, caseID, subjectID string
}

func (tx lostRetentionClaimTx) QueryRow(_ context.Context, sql string, _ ...any) pgx.Row {
	switch {
	case strings.Contains(sql, "FROM public.document_verification_job j"):
		return commitSerializationFailureRow{values: []any{tx.jobID, tx.fileID, tx.caseID, tx.subjectID, string(dv.PurposeIdentity), "", int16(1)}}
	case strings.Contains(sql, "SELECT id FROM public.document_verification_case"):
		return commitSerializationFailureRow{values: []any{tx.caseID}}
	case strings.Contains(sql, "SELECT id FROM public.document_verification_file"):
		return commitSerializationFailureRow{values: []any{tx.fileID}}
	case strings.Contains(sql, "SELECT EXISTS"):
		return commitSerializationFailureRow{values: []any{false}}
	case strings.Contains(sql, "UPDATE public.document_verification_job SET state='procesando'"):
		return commitSerializationFailureRow{err: pgx.ErrNoRows}
	default:
		return commitSerializationFailureRow{err: errors.New("unexpected retention claim query")}
	}
}

func (lostRetentionClaimTx) Rollback(context.Context) error { return nil }

func TestRetentionClaimReturnsNoWorkWhenConditionalUpdateLost(t *testing.T) {
	const (
		jobID     = "00000000-0000-4000-8000-000000000011"
		fileID    = "00000000-0000-4000-8000-000000000012"
		caseID    = "00000000-0000-4000-8000-000000000013"
		subjectID = "00000000-0000-4000-8000-000000000014"
	)
	tx := lostRetentionClaimTx{jobID: jobID, fileID: fileID, caseID: caseID, subjectID: subjectID}
	repo := newWithTransactionBeginner(lostRetentionClaimBeginner{tx: tx})
	claim, err := repo.ClaimReadyRetentionJob(context.Background(), time.Minute)
	if err != nil || claim != nil {
		t.Fatalf("lost conditional claim = (%+v, %v), want (nil, nil)", claim, err)
	}
}

type lostScanClaimBeginner struct{ tx pgx.Tx }

func (b lostScanClaimBeginner) BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error) {
	return b.tx, nil
}

type lostScanClaimTx struct {
	pgx.Tx
	jobID, fileID, caseID, subjectID string
}

func (tx lostScanClaimTx) QueryRow(_ context.Context, sql string, _ ...any) pgx.Row {
	switch {
	case strings.Contains(sql, "FROM public.document_verification_job j"):
		return commitSerializationFailureRow{values: []any{tx.jobID, tx.fileID, tx.caseID, tx.subjectID, string(dv.PurposeIdentity), "", int64(128), "image/png", 1, make([]byte, 32), int64(1)}}
	case strings.Contains(sql, "SELECT revision,superseded_at"):
		return commitSerializationFailureRow{values: []any{int64(1), nil}}
	case strings.Contains(sql, "SELECT state FROM public.document_verification_file"):
		return commitSerializationFailureRow{values: []any{"pendiente"}}
	case strings.Contains(sql, "SELECT attempts FROM public.document_verification_job"):
		return commitSerializationFailureRow{values: []any{int16(0)}}
	case strings.Contains(sql, "UPDATE public.document_verification_job SET state='procesando'"):
		return commitSerializationFailureRow{err: pgx.ErrNoRows}
	default:
		return commitSerializationFailureRow{err: errors.New("unexpected scan claim query")}
	}
}

func (lostScanClaimTx) Rollback(context.Context) error { return nil }

func TestScanClaimReturnsNoWorkWhenConditionalUpdateLost(t *testing.T) {
	const (
		jobID     = "00000000-0000-4000-8000-000000000021"
		fileID    = "00000000-0000-4000-8000-000000000022"
		caseID    = "00000000-0000-4000-8000-000000000023"
		subjectID = "00000000-0000-4000-8000-000000000024"
	)
	tx := lostScanClaimTx{jobID: jobID, fileID: fileID, caseID: caseID, subjectID: subjectID}
	repo := newWithTransactionBeginner(lostScanClaimBeginner{tx: tx})
	claim, err := repo.ClaimReadyScanJob(context.Background(), time.Minute)
	if err != nil || claim != nil {
		t.Fatalf("lost conditional scan claim = (%+v, %v), want (nil, nil)", claim, err)
	}
}

type dbClockExpiredScanBeginner struct{ tx pgx.Tx }

func (b dbClockExpiredScanBeginner) BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error) {
	return b.tx, nil
}

type dbClockExpiredScanTx struct {
	pgx.Tx
	leaseUntil       time.Time
	dbNow            time.Time
	jobLocked        bool
	dbClockRead      bool
	applyExecInvoked bool
}

func (tx *dbClockExpiredScanTx) QueryRow(_ context.Context, sql string, _ ...any) pgx.Row {
	switch {
	case strings.TrimSpace(sql) == "SELECT clock_timestamp()":
		tx.dbClockRead = true
		return commitSerializationFailureRow{values: []any{tx.dbNow}}
	case strings.Contains(sql, "SELECT purpose,subject_id::text,COALESCE(space_id::text,''),revision,superseded_at"):
		return commitSerializationFailureRow{values: []any{string(dv.PurposeIdentity), "00000000-0000-4000-8000-000000000031", "", int64(1), nil}}
	case strings.Contains(sql, "SELECT revision,superseded_at"):
		return commitSerializationFailureRow{values: []any{int64(1), nil}}
	case strings.Contains(sql, "SELECT state FROM public.document_verification_file"):
		return commitSerializationFailureRow{values: []any{"pendiente"}}
	case strings.Contains(sql, "FROM public.document_verification_job"):
		tx.jobLocked = strings.Contains(sql, "FOR UPDATE")
		return commitSerializationFailureRow{values: []any{"procesando", tx.leaseUntil}}
	default:
		return commitSerializationFailureRow{err: errors.New("unexpected database-clock lease query")}
	}
}

func (tx *dbClockExpiredScanTx) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	tx.applyExecInvoked = true
	return pgconn.CommandTag{}, errors.New("unexpected write after expired scan lease")
}

func (*dbClockExpiredScanTx) Commit(context.Context) error   { return nil }
func (*dbClockExpiredScanTx) Rollback(context.Context) error { return nil }

func TestFinishScanRejectsLeaseExpiredByDatabaseClock(t *testing.T) {
	claim := ScanClaim{
		JobID:      "00000000-0000-4000-8000-000000000032",
		FileID:     "00000000-0000-4000-8000-000000000033",
		CaseID:     "00000000-0000-4000-8000-000000000034",
		SubjectID:  "00000000-0000-4000-8000-000000000031",
		Purpose:    dv.PurposeIdentity,
		LeaseUntil: time.Now().UTC().Add(time.Hour),
	}
	tx := &dbClockExpiredScanTx{leaseUntil: claim.LeaseUntil, dbNow: claim.LeaseUntil.Add(time.Second)}
	repo := newWithTransactionBeginner(dbClockExpiredScanBeginner{tx: tx})
	applyCalled := false
	err := repo.finishScan(context.Background(), claim, func(context.Context, pgx.Tx, time.Time) error {
		applyCalled = true
		return nil
	})
	if !errors.Is(err, dv.ErrConflict) || !tx.jobLocked || !tx.dbClockRead || applyCalled || tx.applyExecInvoked {
		t.Fatalf("expired database lease result=(%v), job_locked=%t db_clock_read=%t apply_called=%t writes=%t; want conflict before writes", err, tx.jobLocked, tx.dbClockRead, applyCalled, tx.applyExecInvoked)
	}
}

func TestFinishScanUsesDatabaseClockForOutcomeTime(t *testing.T) {
	claim := ScanClaim{
		JobID:      "00000000-0000-4000-8000-000000000042",
		FileID:     "00000000-0000-4000-8000-000000000043",
		CaseID:     "00000000-0000-4000-8000-000000000044",
		SubjectID:  "00000000-0000-4000-8000-000000000041",
		Purpose:    dv.PurposeIdentity,
		LeaseUntil: time.Now().UTC().Add(time.Hour),
	}
	dbNow := time.Date(2001, time.January, 2, 3, 4, 5, 0, time.UTC)
	tx := &dbClockExpiredScanTx{leaseUntil: claim.LeaseUntil, dbNow: dbNow}
	repo := newWithTransactionBeginner(dbClockExpiredScanBeginner{tx: tx})
	applyErr := errors.New("stop after observing persisted outcome time")
	var appliedAt time.Time
	err := repo.finishScan(context.Background(), claim, func(_ context.Context, _ pgx.Tx, now time.Time) error {
		appliedAt = now
		return applyErr
	})
	if !errors.Is(err, applyErr) || !tx.jobLocked || !tx.dbClockRead || !appliedAt.Equal(dbNow) {
		t.Fatalf("finish result=%v job_locked=%t db_clock_read=%t applied_at=%s; want database time %s", err, tx.jobLocked, tx.dbClockRead, appliedAt, dbNow)
	}
}

func TestPostgresPersistedReviewerGrantAndReasonAudit(t *testing.T) {
	dsn := os.Getenv("ESPACIGO_DOCUMENT_VERIFICATION_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ESPACIGO_DOCUMENT_VERIFICATION_TEST_DATABASE_URL is unset")
	}
	ctx := context.Background()
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal("invalid integration DSN")
	}
	if cfg.ConnConfig.Host != "127.0.0.1" || cfg.ConnConfig.Port != 15433 || cfg.ConnConfig.Database != "espacigo_f5_synth_20261029" {
		t.Fatal("refusing integration target outside the authorized F5 synthetic endpoint")
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pool.Close() })
	outer, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		t.Fatal(err)
	}
	ownerID, reviewerID, caseID, fileID, grantID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	t.Cleanup(func() {
		if err := outer.Rollback(context.Background()); err != nil {
			t.Errorf("rollback synthetic grant fixture failed: %v", err)
		}
		var remaining int
		if err := pool.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM public.usuario WHERE id IN($1,$2))+(SELECT count(*) FROM public.document_verification_case WHERE id=$3)+(SELECT count(*) FROM public.document_verification_file WHERE id=$4)+(SELECT count(*) FROM public.document_verification_job WHERE file_id=$4)+(SELECT count(*) FROM public.document_verification_reviewer_grant WHERE id=$5)+(SELECT count(*) FROM public.document_verification_event WHERE case_id=$3)+(SELECT count(*) FROM public.document_verification_reviewer_grant_event WHERE grant_id=$5)`, ownerID, reviewerID, caseID, fileID, grantID).Scan(&remaining); err != nil {
			t.Errorf("verify rollback of synthetic grant fixture failed: %v", err)
		} else if remaining != 0 {
			t.Errorf("transactional cleanup left %d synthetic rows", remaining)
		}
	})
	for _, id := range []string{ownerID, reviewerID} {
		if _, err := outer.Exec(ctx, `INSERT INTO public.usuario(id,correo_original,correo_normalizado,hash_clave,estado) VALUES($1,$2,$2,'synthetic','activo')`, id, id+"@example.invalid"); err != nil {
			t.Fatal(err)
		}
	}
	repo := newWithTransactionBeginner(savepointBeginner{tx: outer})
	service, err := dv.NewService(repo)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	service.SetClock(func() time.Time { return now })
	created := dv.Snapshot{Purpose: dv.PurposeIdentity, ID: caseID, SubjectID: ownerID, State: dv.CaseDraft, DocumentExpiresAt: now.AddDate(1, 0, 0)}
	created, err = repo.CreateCase(ctx, ownerID, dv.PurposeIdentity, "", func(dv.Authorization) (dv.Snapshot, error) { return created, nil })
	if err != nil {
		t.Fatal(err)
	}
	file := dv.File{ID: fileID, State: dv.FilePending, Size: 128, MIME: "image/png", Pages: 1, ContentSHA256: [32]byte{2}}
	if err := service.AddFile(ctx, ownerID, caseID, dv.PurposeIdentity, created.Revision, file); err != nil {
		t.Fatal(err)
	}
	if _, err := outer.Exec(ctx, `UPDATE public.document_verification_file SET state='limpio',scanner_version='synthetic-fixture',signatures_at=$2,inspected_at=$2,updated_at=$2 WHERE id=$1`, fileID, now); err != nil {
		t.Fatal(err)
	}
	if err := service.Submit(ctx, ownerID, caseID, dv.PurposeIdentity, 2); err != nil {
		t.Fatal(err)
	}
	if _, err := outer.Exec(ctx, `INSERT INTO public.document_verification_reviewer_grant(id,reviewer_id,capability,scope_kind,scope_id,granted_by,granted_reason_code,expires_at) VALUES($1,$2,'review_identity','identity_subject',$3,$4,'temporary_coverage',$5)`, grantID, reviewerID, ownerID, ownerID, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := service.ReviewWithReason(ctx, reviewerID, caseID, dv.CapabilityIdentity, dv.DecisionReject, dv.ReasonDocumentInvalid, time.Time{}, 3); err != nil {
		t.Fatalf("review with persisted scoped grant failed: %v", err)
	}
	result, err := repo.LoadOwnCase(ctx, ownerID, caseID)
	if err != nil || result.State != dv.CaseRejected || result.ReviewerID != reviewerID || result.Revision != 4 {
		t.Fatalf("review snapshot=%+v err=%v", result, err)
	}
	var action, resultCode, reasonCode string
	if err := outer.QueryRow(ctx, `SELECT action,result_code,reason_code FROM public.document_verification_event WHERE case_id=$1 AND action='review_decision'`, caseID).Scan(&action, &resultCode, &reasonCode); err != nil {
		t.Fatal(err)
	}
	if action != "review_decision" || resultCode != string(dv.CaseRejected) || reasonCode != string(dv.ReasonDocumentInvalid) {
		t.Fatalf("persisted review audit action=%q result=%q reason=%q", action, resultCode, reasonCode)
	}
	var grantAction, grantReason string
	if err := outer.QueryRow(ctx, `SELECT action,reason_code FROM public.document_verification_reviewer_grant_event WHERE grant_id=$1`, grantID).Scan(&grantAction, &grantReason); err != nil {
		t.Fatal(err)
	}
	if grantAction != "granted" || grantReason != "temporary_coverage" {
		t.Fatalf("persisted grant audit action=%q reason=%q", grantAction, grantReason)
	}
	for name, query := range map[string]string{
		"update":   `UPDATE public.document_verification_reviewer_grant_event SET reason_code='access_review' WHERE grant_id=$1`,
		"delete":   `DELETE FROM public.document_verification_reviewer_grant_event WHERE grant_id=$1`,
		"truncate": `TRUNCATE TABLE public.document_verification_reviewer_grant_event`,
	} {
		t.Run("immutable_reviewer_grant_audit_"+name, func(t *testing.T) {
			nested, err := outer.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if name == "truncate" {
				_, err = nested.Exec(ctx, query)
			} else {
				_, err = nested.Exec(ctx, query, grantID)
			}
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != "23514" {
				_ = nested.Rollback(context.Background())
				t.Fatalf("%s reviewer-grant audit mutation error=%v, want SQLSTATE 23514", name, err)
			}
			if rollbackErr := nested.Rollback(context.Background()); rollbackErr != nil {
				t.Fatalf("rollback after immutable audit rejection failed: %v", rollbackErr)
			}
		})
	}
	var grantEvents int
	if err := outer.QueryRow(ctx, `SELECT count(*) FROM public.document_verification_reviewer_grant_event WHERE grant_id=$1`, grantID).Scan(&grantEvents); err != nil {
		t.Fatal(err)
	}
	if grantEvents != 1 {
		t.Fatalf("immutable grant audit event count=%d, want 1", grantEvents)
	}
}

func TestPostgresBookingIdentityVerifierUsesPersistedManualCases(t *testing.T) {
	dsn := os.Getenv("ESPACIGO_DOCUMENT_VERIFICATION_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ESPACIGO_DOCUMENT_VERIFICATION_TEST_DATABASE_URL is unset")
	}
	ctx := context.Background()
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal("invalid integration DSN")
	}
	if cfg.ConnConfig.Host != "127.0.0.1" || cfg.ConnConfig.Port != 15433 || cfg.ConnConfig.Database != "espacigo_f5_synth_20261029" {
		t.Fatal("refusing integration target outside the authorized F5 synthetic endpoint")
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pool.Close() })
	outer, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()}
	caseIDs := []string{uuid.NewString(), uuid.NewString()}
	fileIDs := []string{uuid.NewString(), uuid.NewString()}
	grantIDs := []string{uuid.NewString(), uuid.NewString()}
	t.Cleanup(func() {
		if err := outer.Rollback(context.Background()); err != nil {
			t.Errorf("rollback booking-identity fixtures failed: %v", err)
		}
		var remaining int
		if err := pool.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM public.usuario WHERE id=ANY($1::uuid[]))+(SELECT count(*) FROM public.document_verification_case WHERE id=ANY($2::uuid[]))+(SELECT count(*) FROM public.document_verification_file WHERE id=ANY($3::uuid[]))+(SELECT count(*) FROM public.document_verification_job WHERE file_id=ANY($3::uuid[]))+(SELECT count(*) FROM public.document_verification_reviewer_grant WHERE id=ANY($4::uuid[]))+(SELECT count(*) FROM public.document_verification_event WHERE case_id=ANY($2::uuid[]))+(SELECT count(*) FROM public.document_verification_reviewer_grant_event WHERE grant_id=ANY($4::uuid[]))`, ids, caseIDs, fileIDs, grantIDs).Scan(&remaining); err != nil {
			t.Errorf("verify booking-identity rollback failed: %v", err)
		} else if remaining != 0 {
			t.Errorf("booking-identity rollback left %d synthetic rows", remaining)
		}
	})
	for _, id := range ids {
		if _, err := outer.Exec(ctx, `INSERT INTO public.usuario(id,correo_original,correo_normalizado,hash_clave,estado) VALUES($1,$2,$2,'synthetic','activo')`, id, id+"@example.invalid"); err != nil {
			t.Fatal(err)
		}
	}
	repo := newWithTransactionBeginner(savepointBeginner{tx: outer})
	documents, err := dv.NewService(repo)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	documents.SetClock(func() time.Time { return now })
	for i := 0; i < 2; i++ {
		ownerID, reviewerID := ids[i], ids[i+2]
		created, err := documents.Start(ctx, ownerID, dv.CreateInput{Purpose: dv.PurposeIdentity, DocumentExpiresAt: now.AddDate(1, 0, 0)})
		if err != nil {
			t.Fatal(err)
		}
		caseIDs[i] = created.ID
		file := dv.File{ID: fileIDs[i], State: dv.FilePending, Size: 128, MIME: "image/png", Pages: 1, ContentSHA256: [32]byte{byte(i + 1)}}
		if err := documents.AddFile(ctx, ownerID, created.ID, dv.PurposeIdentity, created.Revision, file); err != nil {
			t.Fatal(err)
		}
		if _, err := outer.Exec(ctx, `UPDATE public.document_verification_file SET state='limpio',scanner_version='synthetic-fixture',signatures_at=$2,inspected_at=$2,updated_at=$2 WHERE id=$1`, file.ID, now); err != nil {
			t.Fatal(err)
		}
		if err := documents.Submit(ctx, ownerID, created.ID, dv.PurposeIdentity, 2); err != nil {
			t.Fatal(err)
		}
		grantIDs[i] = uuid.NewString()
		if _, err := outer.Exec(ctx, `INSERT INTO public.document_verification_reviewer_grant(id,reviewer_id,capability,scope_kind,scope_id,granted_by,granted_reason_code,expires_at) VALUES($1,$2,'review_identity','identity_subject',$3,$4,'role_assignment',$5)`, grantIDs[i], reviewerID, ownerID, ownerID, now.Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
		if err := documents.ReviewWithReason(ctx, reviewerID, created.ID, dv.CapabilityIdentity, dv.DecisionApprove, "", time.Time{}, 3); err != nil {
			t.Fatalf("manual approval %d failed: %v", i, err)
		}
	}
	verifier, err := identityverification.NewService(repo)
	if err != nil {
		t.Fatal(err)
	}
	if err := verifier.RequireForBooking(ctx, ids[0], ids[1], now); err != nil {
		t.Fatalf("booking rejected two current manual identity cases: %v", err)
	}
	if err := documents.Revoke(ctx, ids[2], caseIDs[0], dv.PurposeIdentity, dv.ReasonOwnerRequest, 4); err != nil {
		t.Fatalf("manual identity revocation failed: %v", err)
	}
	if err := verifier.RequireForBooking(ctx, ids[0], ids[1], now); !errors.Is(err, identityverification.ErrNotVerified) {
		t.Fatalf("booking after persisted revocation error=%v, want ErrNotVerified", err)
	}
}

func TestPostgresSpaceCaseOwnershipTransferRequiresNewCase(t *testing.T) {
	dsn := os.Getenv("ESPACIGO_DOCUMENT_VERIFICATION_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ESPACIGO_DOCUMENT_VERIFICATION_TEST_DATABASE_URL is unset")
	}
	ctx := context.Background()
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal("invalid integration DSN")
	}
	if cfg.ConnConfig.Host != "127.0.0.1" || cfg.ConnConfig.Port != 15433 || cfg.ConnConfig.Database != "espacigo_f5_synth_20261029" {
		t.Fatal("refusing integration target outside the authorized F5 synthetic endpoint")
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pool.Close() })
	outer, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		t.Fatal(err)
	}
	oldOwner, newOwner, spaceID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	caseID, newCaseID, fileID, secondFileID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	t.Cleanup(func() {
		if err := outer.Rollback(context.Background()); err != nil {
			t.Errorf("rollback ownership-transfer fixtures failed: %v", err)
		}
		var remaining int
		if err := pool.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM public.usuario WHERE id IN($1,$2))+(SELECT count(*) FROM public.espacio WHERE id=$3)+(SELECT count(*) FROM public.tarifa_espacio WHERE espacio_id=$3)+(SELECT count(*) FROM public.espacio_caracteristicas WHERE espacio_id=$3)+(SELECT count(*) FROM public.document_verification_case WHERE id IN($4,$5))+(SELECT count(*) FROM public.document_verification_file WHERE id IN($6,$7))+(SELECT count(*) FROM public.document_verification_job WHERE file_id IN($6,$7))+(SELECT count(*) FROM public.document_verification_event WHERE case_id IN($4,$5))`, oldOwner, newOwner, spaceID, caseID, newCaseID, fileID, secondFileID).Scan(&remaining); err != nil {
			t.Errorf("verify ownership-transfer fixture cleanup failed: %v", err)
		} else if remaining != 0 {
			t.Errorf("ownership-transfer rollback left %d synthetic rows", remaining)
		}
	})
	for _, owner := range []string{oldOwner, newOwner} {
		if _, err := outer.Exec(ctx, `INSERT INTO public.usuario(id,correo_original,correo_normalizado,hash_clave,estado) VALUES($1,$2,$2,'synthetic','activo')`, owner, owner+"@example.invalid"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := outer.Exec(ctx, `INSERT INTO public.espacio(id,propietario_id,categoria_codigo,titulo,descripcion,superficie_m2,capacidad_maxima,reglas_uso,modalidad_tarifa,precio_base_clp,direccion,zona_horaria,estado) VALUES($1,$2,'sala_multiproposito','F5 transfer fixture',repeat('Synthetic space transfer description. ',4),30,4,'Synthetic','hora',8000,'Synthetic address','America/Santiago','activa')`, spaceID, oldOwner); err != nil {
		t.Fatal(err)
	}
	if _, err := outer.Exec(ctx, `INSERT INTO public.tarifa_espacio(espacio_id,version,modalidad,precio_base_clp) VALUES($1,1,'hora',8000)`, spaceID); err != nil {
		t.Fatal(err)
	}
	if _, err := outer.Exec(ctx, `INSERT INTO public.espacio_caracteristicas(espacio_id,categoria_codigo,perfil_version,valores) VALUES($1,'sala_multiproposito',1,'{}'::jsonb)`, spaceID); err != nil {
		t.Fatal(err)
	}
	repo := newWithTransactionBeginner(savepointBeginner{tx: outer})
	service, err := dv.NewService(repo)
	if err != nil {
		t.Fatal(err)
	}
	created, err := service.Start(ctx, oldOwner, dv.CreateInput{Purpose: dv.PurposeSpace, SpaceID: spaceID})
	if err != nil {
		t.Fatal(err)
	}
	caseID = created.ID
	fileID = uuid.NewString()
	file := dv.File{ID: fileID, State: dv.FilePending, Size: 128, MIME: "image/png", Pages: 1, ContentSHA256: [32]byte{4}}
	if err := service.AddFile(ctx, oldOwner, caseID, dv.PurposeSpace, created.Revision, file); err != nil {
		t.Fatal(err)
	}
	current, err := repo.LoadOwnCase(ctx, oldOwner, caseID)
	if err != nil {
		t.Fatalf("reload case before adding a second file: %v", err)
	}
	secondFile := file
	secondFile.ID = secondFileID
	secondFile.ContentSHA256 = [32]byte{5}
	if err := service.AddFile(ctx, oldOwner, caseID, dv.PurposeSpace, current.Revision, secondFile); err != nil {
		t.Fatalf("add second transfer fixture file: %v", err)
	}
	if _, err := outer.Exec(ctx, `UPDATE public.document_verification_job SET state='procesando',attempts=2,lease_until=now()+interval '1 hour' WHERE file_id=$1 AND kind='scan' AND state='pendiente'`, secondFileID); err != nil {
		t.Fatalf("seed active scan lease: %v", err)
	}
	preexistingRetentionDeadline := time.Now().UTC().Truncate(time.Microsecond).Add(24 * time.Hour)
	if _, err := outer.Exec(ctx, `UPDATE public.document_verification_file SET retention_at=$2,retention_reason_code='abandoned_draft' WHERE id=$1`, fileID, preexistingRetentionDeadline); err != nil {
		t.Fatalf("seed existing retention deadline: %v", err)
	}
	if _, err := outer.Exec(ctx, `INSERT INTO public.document_verification_job(file_id,kind,state,attempts,available_at,lease_until,last_error_code) VALUES($1,'retention_delete','procesando',3,now()+interval '1 day',now()+interval '1 hour','retention_store_error')`, fileID); err != nil {
		t.Fatalf("seed existing retention job: %v", err)
	}
	if _, err := outer.Exec(ctx, `SET LOCAL ROLE espacigo_runtime`); err != nil {
		t.Fatalf("cannot enter the restricted runtime role for ownership transfer: %v", err)
	}
	var runtimeRole string
	var superuser, bypassRLS bool
	if err := outer.QueryRow(ctx, `SELECT current_user,r.rolsuper,r.rolbypassrls FROM pg_catalog.pg_roles r WHERE r.rolname=current_user`).Scan(&runtimeRole, &superuser, &bypassRLS); err != nil {
		t.Fatal(err)
	}
	if runtimeRole != "espacigo_runtime" || superuser || bypassRLS {
		t.Fatalf("ownership transfer is not running as the restricted runtime role: role=%q superuser=%t bypassRLS=%t", runtimeRole, superuser, bypassRLS)
	}
	if _, err := outer.Exec(ctx, `UPDATE public.espacio SET propietario_id=$2,actualizado_en=now() WHERE id=$1`, spaceID, newOwner); err != nil {
		t.Fatalf("synthetic ownership transfer failed: %v", err)
	}
	var oldState string
	var oldRevision int64
	var supersededAt time.Time
	if err := outer.QueryRow(ctx, `SELECT state,revision,superseded_at FROM public.document_verification_case WHERE id=$1`, caseID).Scan(&oldState, &oldRevision, &supersededAt); err != nil {
		t.Fatalf("read transferred case terminal marker: %v", err)
	}
	if oldState != string(dv.CaseDraft) || oldRevision != 4 || supersededAt.IsZero() {
		t.Fatalf("transferred case state=%q revision=%d superseded_at=%v", oldState, oldRevision, supersededAt)
	}
	for _, actor := range []string{oldOwner, newOwner} {
		if _, err := repo.LoadOwnCase(ctx, actor, caseID); !errors.Is(err, dv.ErrNotFound) {
			t.Fatalf("actor %s loaded pre-transfer case: %v", actor, err)
		}
		cases, err := repo.ListOwnCases(ctx, actor)
		if err != nil {
			t.Fatal(err)
		}
		for _, item := range cases {
			if item.ID == caseID {
				t.Fatalf("actor %s listed pre-transfer case", actor)
			}
		}
		if err := repo.Transition(ctx, actor, dv.PurposeSpace, caseID, 3, dv.TransitionAudit{Action: "case_transitioned", ResultCode: string(dv.CaseSubmitted)}, func(*dv.Snapshot, dv.Authorization) error { return nil }); !errors.Is(err, dv.ErrConflict) {
			t.Fatalf("actor %s transition on pre-transfer case error=%v, want conflict", actor, err)
		}
		if _, _, err := repo.LoadSpaceCaseForDownload(ctx, actor, caseID); !errors.Is(err, dv.ErrNotFound) {
			t.Fatalf("actor %s loaded superseded space case for download: %v", actor, err)
		}
	}
	newCase, err := service.Start(ctx, newOwner, dv.CreateInput{Purpose: dv.PurposeSpace, SpaceID: spaceID})
	if err != nil {
		t.Fatalf("new owner could not start required replacement case: %v", err)
	}
	newCaseID = newCase.ID
	if newCase.SubjectID != newOwner || newCase.ID == caseID || newCase.State != dv.CaseDraft {
		t.Fatalf("replacement case=%+v, want a new draft owned by the new owner", newCase)
	}
	for _, transferredFileID := range []string{fileID, secondFileID} {
		var retentionAt time.Time
		var retentionReason string
		var supersededAt time.Time
		if err := outer.QueryRow(ctx, `SELECT f.retention_at,f.retention_reason_code,c.superseded_at FROM public.document_verification_file f JOIN public.document_verification_case c ON c.id=f.case_id WHERE f.id=$1`, transferredFileID).Scan(&retentionAt, &retentionReason, &supersededAt); err != nil {
			t.Fatal(err)
		}
		var expectedDeadline time.Time
		expectedReason := "rejected_or_replaced"
		if transferredFileID == fileID {
			expectedDeadline = preexistingRetentionDeadline
			expectedReason = "abandoned_draft"
		} else {
			expectedDeadline = supersededAt.Add(30 * 24 * time.Hour)
		}
		if retentionReason != expectedReason || !retentionAt.Equal(expectedDeadline) {
			t.Fatalf("transferred file %s retention=%v reason=%q; want deadline=%v reason=%q", transferredFileID, retentionAt, retentionReason, expectedDeadline, expectedReason)
		}
		var completedScans, pendingRetention int
		var scheduledRetentionAt time.Time
		if err := outer.QueryRow(ctx, `SELECT (SELECT count(*) FROM public.document_verification_job WHERE file_id=$1 AND kind='scan' AND state='completado'),(SELECT count(*) FROM public.document_verification_job WHERE file_id=$1 AND kind='retention_delete' AND state='pendiente'),(SELECT max(available_at) FROM public.document_verification_job WHERE file_id=$1 AND kind='retention_delete' AND state='pendiente')`, transferredFileID).Scan(&completedScans, &pendingRetention, &scheduledRetentionAt); err != nil {
			t.Fatal(err)
		}
		if completedScans != 1 || pendingRetention != 1 {
			t.Fatalf("transfer jobs for %s: completed scan=%d pending retention=%d", transferredFileID, completedScans, pendingRetention)
		}
		var scanState string
		var scanAttempts int16
		var scanLeaseClear bool
		if err := outer.QueryRow(ctx, `SELECT state,attempts,lease_until IS NULL FROM public.document_verification_job WHERE file_id=$1 AND kind='scan'`, transferredFileID).Scan(&scanState, &scanAttempts, &scanLeaseClear); err != nil {
			t.Fatal(err)
		}
		expectedAttempts := int16(0)
		if transferredFileID == secondFileID {
			expectedAttempts = 2
		}
		if scanState != "completado" || scanAttempts != expectedAttempts || !scanLeaseClear {
			t.Fatalf("scan job for %s was not cancelled/fenced: state=%q attempts=%d lease_clear=%t", transferredFileID, scanState, scanAttempts, scanLeaseClear)
		}
		if !scheduledRetentionAt.Equal(retentionAt) {
			t.Fatalf("retention job for %s scheduled at %v, file deadline is %v", transferredFileID, scheduledRetentionAt, retentionAt)
		}
		var retentionState, retentionError string
		var retentionAttempts int16
		var retentionLeaseIsClear bool
		if err := outer.QueryRow(ctx, `SELECT state,attempts,lease_until IS NULL,COALESCE(last_error_code,'') FROM public.document_verification_job WHERE file_id=$1 AND kind='retention_delete'`, transferredFileID).Scan(&retentionState, &retentionAttempts, &retentionLeaseIsClear, &retentionError); err != nil {
			t.Fatal(err)
		}
		if retentionState != "pendiente" || retentionAttempts != 0 || !retentionLeaseIsClear || retentionError != "" {
			t.Fatalf("retention job for %s was not rescheduled cleanly: state=%q attempts=%d lease_clear=%t error=%q", transferredFileID, retentionState, retentionAttempts, retentionLeaseIsClear, retentionError)
		}
	}
	var supersessionEvents int
	if err := outer.QueryRow(ctx, `SELECT count(*) FROM public.document_verification_event WHERE case_id=$1 AND actor_kind='system' AND action='case_superseded' AND from_state='borrador' AND to_state='borrador' AND result_code='borrador' AND reason_code='cambio_titularidad' AND revision=4`, caseID).Scan(&supersessionEvents); err != nil {
		t.Fatal(err)
	}
	if supersessionEvents != 1 {
		t.Fatalf("ownership supersession audit event count=%d, want 1", supersessionEvents)
	}
}

func TestRepositoryInterfaces(t *testing.T) {
	var _ dv.Repository = (*Repository)(nil)
	var _ dv.OwnCaseReader = (*Repository)(nil)
	var _ dv.SpaceDownloadRepository = (*Repository)(nil)
}

func TestNullableTimeScanMapsNullToZero(t *testing.T) {
	got := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	if err := (&nullableT{v: &got}).Scan(nil); err != nil {
		t.Fatal(err)
	}
	if !got.IsZero() {
		t.Fatalf("nullable timestamp after NULL scan=%v, want zero time", got)
	}
}
