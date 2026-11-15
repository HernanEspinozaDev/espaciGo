package documentverificationpg

import (
	"context"
	"encoding/json"
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

func TestTypedIdentityDecisionPersistsObservedAuditWithTransition(t *testing.T) {
	actor := "11111111-1111-4111-8111-111111111111"
	subject := "22222222-2222-4222-8222-222222222222"
	caseID := "33333333-3333-4333-8333-333333333333"
	declarationID := "44444444-4444-4444-8444-444444444444"
	expiry := time.Date(2031, 4, 6, 3, 0, 0, 0, time.UTC)
	consentedAt := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	tx := &reviewerAuditFakeTx{actor: actor, subject: subject, caseID: caseID, declarationID: declarationID, consentedAt: consentedAt}
	repo := newWithTransactionBeginner(reviewerAuditFakeBeginner{tx: tx})
	checklist := &dv.IdentityReviewChecklist{
		NamesMatch: boolPointer(true), RUTMatches: boolPointer(true), BirthDateAndMajorityMatch: boolPointer(true),
		PortraitMatchesSelfie: boolPointer(true), EvidenceLegible: boolPointer(true), DocumentUnexpired: boolPointer(true),
		IssueDateAndFormatVerified: boolPointer(true),
	}
	audit := dv.TransitionAudit{Action: "review_decision", ResultCode: string(dv.CaseApproved), DocumentFormatObserved: dv.DocumentFormatLegacy, DocumentExpiresAtObserved: expiry, IdentityReviewChecklist: checklist}
	err := repo.Transition(context.Background(), actor, dv.PurposeIdentity, caseID, 7, audit, func(x *dv.Snapshot, _ dv.Authorization) error {
		x.State = dv.CaseApproved
		x.ReviewerID = actor
		x.VerifiedAt = expiry.Add(-time.Hour)
		x.VerifiedUntil = expiry
		x.DocumentFormatObserved = dv.DocumentFormatLegacy
		x.DocumentExpiresAt = expiry
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !tx.committed || len(tx.execs) != 2 || !strings.Contains(tx.execs[0].sql, "UPDATE public.document_verification_case") || !strings.Contains(tx.execs[1].sql, "INSERT INTO public.document_verification_event") {
		t.Fatalf("decision and audit were not issued in order in one transaction: committed=%v execs=%+v", tx.committed, tx.execs)
	}
	update := tx.execs[0]
	if !strings.Contains(update.sql, "document_format_observed") || !strings.Contains(update.sql, "document_expires_at") {
		t.Fatalf("case update omits observed format/expiry: %s", update.sql)
	}
	if len(update.args) != 9 || update.args[7] != dv.DocumentFormatLegacy {
		t.Fatalf("case update arguments omit the observed identity fields: %#v", update.args)
	}
	persistedExpiry, ok := update.args[8].(time.Time)
	if !ok || !persistedExpiry.Equal(expiry) {
		t.Fatalf("case update expiry=%#v, want exact exclusive boundary %s", update.args[8], expiry.Format(time.RFC3339Nano))
	}
	event := tx.execs[1]
	if !strings.Contains(event.sql, "identity_review_observations") || len(event.args) < 11 {
		t.Fatalf("audit insert omits identity observations: sql=%s args=%#v", event.sql, event.args)
	}
	var observations map[string]any
	if err := json.Unmarshal([]byte(event.args[len(event.args)-1].(string)), &observations); err != nil {
		t.Fatal(err)
	}
	check, ok := observations["identity_review_checklist"].(map[string]any)
	if !ok || len(observations) != 3 || observations["document_format_observed"] != string(dv.DocumentFormatLegacy) || observations["document_expires_at_observed"] != expiry.Format(time.RFC3339Nano) || len(check) != 7 || check["names_match"] != true || check["rut_matches"] != true || check["birth_date_and_majority_match"] != true || check["portrait_matches_selfie"] != true || check["evidence_legible"] != true || check["document_unexpired"] != true || check["issue_date_and_format_verified"] != true {
		t.Fatalf("audit observations=%v", observations)
	}
}

func boolPointer(value bool) *bool { return &value }

type reviewerAuditFakeBeginner struct{ tx *reviewerAuditFakeTx }

func (b reviewerAuditFakeBeginner) BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error) {
	return b.tx, nil
}

type reviewerAuditFakeExec struct {
	sql  string
	args []any
}

type reviewerAuditFakeTx struct {
	pgx.Tx
	actor, subject, caseID, declarationID string
	consentedAt                           time.Time
	execs                                 []reviewerAuditFakeExec
	committed                             bool
}

func (tx *reviewerAuditFakeTx) QueryRow(_ context.Context, query string, args ...any) pgx.Row {
	switch {
	case strings.Contains(query, "document_verification_lock_active_actor"):
		return commitSerializationFailureRow{values: []any{true}}
	case strings.Contains(query, "SELECT COALESCE((SELECT revision.id::text=$2 FROM public.identity_declaration_revision revision"):
		if len(args) != 2 || args[0] != tx.subject || args[1] != tx.declarationID {
			return commitSerializationFailureRow{err: errors.New("identity declaration revision query arguments mismatch")}
		}
		return commitSerializationFailureRow{values: []any{true}}
	case strings.Contains(query, "SELECT space_id::text,subject_id::text FROM public.document_verification_case"):
		return commitSerializationFailureRow{values: []any{nil, tx.subject}}
	case strings.Contains(query, "SELECT id::text,purpose,subject_id::text,space_id::text,state,revision"):
		return commitSerializationFailureRow{values: []any{tx.caseID, "identity", tx.subject, nil, "en_revision", int64(7), "", nil, nil, nil, nil, nil, tx.declarationID, "new", "identity-capture-local-v1", tx.consentedAt, ""}}
	default:
		return commitSerializationFailureRow{err: errors.New("unexpected reviewer audit query: " + query)}
	}
}

func (tx *reviewerAuditFakeTx) Query(_ context.Context, query string, _ ...any) (pgx.Rows, error) {
	if strings.Contains(query, "FROM public.document_verification_file") {
		return &reviewerAuditFakeRows{}, nil
	}
	if strings.Contains(query, "FROM public.document_verification_reviewer_grant") {
		return &reviewerAuditFakeRows{rows: [][]any{{tx.actor, "review_identity", tx.subject}}}, nil
	}
	return nil, errors.New("unexpected reviewer audit rows query: " + query)
}

func (tx *reviewerAuditFakeTx) Exec(_ context.Context, query string, args ...any) (pgconn.CommandTag, error) {
	tx.execs = append(tx.execs, reviewerAuditFakeExec{sql: query, args: append([]any(nil), args...)})
	return pgconn.NewCommandTag("UPDATE 1"), nil
}

func (tx *reviewerAuditFakeTx) Commit(context.Context) error { tx.committed = true; return nil }
func (*reviewerAuditFakeTx) Rollback(context.Context) error  { return nil }

type reviewerAuditFakeRows struct {
	pgx.Rows
	rows    [][]any
	current []any
	index   int
}

func (r *reviewerAuditFakeRows) Next() bool {
	if r.index >= len(r.rows) {
		return false
	}
	r.current = r.rows[r.index]
	r.index++
	return true
}
func (r *reviewerAuditFakeRows) Scan(dest ...any) error {
	return commitSerializationFailureRow{values: r.current}.Scan(dest...)
}
func (*reviewerAuditFakeRows) Close()     {}
func (*reviewerAuditFakeRows) Err() error { return nil }

func TestCreateCaseRequiresSavedIdentityDeclaration(t *testing.T) {
	const actor = "11111111-1111-4111-8111-111111111111"
	const declaration = "22222222-2222-4222-8222-222222222222"
	consentedAt := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name  string
		found bool
	}{{name: "missing"}, {name: "available", found: true}} {
		t.Run(tc.name, func(t *testing.T) {
			tx := &createCaseFakeTx{actorActive: true, declarationID: declaration, declarationFound: tc.found}
			repo := newWithTransactionBeginner(createCaseFakeBeginner{tx: tx})
			buildCalled := false
			created, err := repo.CreateCase(context.Background(), actor, dv.PurposeIdentity, "", func(auth dv.Authorization) (dv.Snapshot, error) {
				buildCalled = true
				if tc.found && auth.IdentityDeclarationRevisionID != declaration {
					t.Fatalf("authorization declaration=%q", auth.IdentityDeclarationRevisionID)
				}
				return dv.Snapshot{
					ID: uuid.NewString(), Purpose: dv.PurposeIdentity, SubjectID: actor, State: dv.CaseDraft,
					IdentityDeclarationRevisionID: auth.IdentityDeclarationRevisionID,
					DocumentFormatDeclared:        dv.DocumentFormatUnsure,
					CaptureConsentVersion:         dv.IdentityCaptureConsentVersion,
					CaptureConsentedAt:            consentedAt,
				}, nil
			})
			if !tc.found {
				if !errors.Is(err, dv.ErrDeclarationRequired) {
					t.Fatalf("error=%v, want declaration-required", err)
				}
				if buildCalled || tx.execCount != 0 || tx.committed {
					t.Fatalf("missing declaration reached build/write/commit: build=%v exec=%d commit=%v", buildCalled, tx.execCount, tx.committed)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !buildCalled || tx.execCount != 2 || !tx.committed {
				t.Fatalf("create not completed: build=%v exec=%d commit=%v", buildCalled, tx.execCount, tx.committed)
			}
			var caseInsert *createCaseExecCall
			for i := range tx.execCalls {
				if strings.Contains(tx.execCalls[i].sql, "INSERT INTO public.document_verification_case") {
					caseInsert = &tx.execCalls[i]
					break
				}
			}
			if caseInsert == nil || !strings.Contains(caseInsert.sql, "identity_declaration_revision_id") ||
				!strings.Contains(caseInsert.sql, "capture_consent_version") || !strings.Contains(caseInsert.sql, "capture_consented_at") {
				t.Fatalf("case insert does not persist the declaration and separate consent: %#v", caseInsert)
			}
			if len(caseInsert.args) != 9 || caseInsert.args[0] != created.ID || caseInsert.args[2] != actor ||
				caseInsert.args[5] != declaration || caseInsert.args[6] != dv.DocumentFormatUnsure ||
				caseInsert.args[7] != dv.IdentityCaptureConsentVersion || !caseInsert.args[8].(time.Time).Equal(consentedAt) {
				t.Fatalf("case insert arguments do not bind the authenticated declaration/consent: %#v", caseInsert.args)
			}
			lockPosition, declarationPosition := -1, -1
			for i, call := range tx.calls {
				if strings.Contains(call, "pg_advisory_xact_lock(hashtextextended($1, 0))") {
					lockPosition = i
				}
				if strings.Contains(call, "identity_declaration_revision") {
					declarationPosition = i
				}
			}
			if lockPosition < 0 || declarationPosition < 0 || lockPosition >= declarationPosition {
				t.Fatalf("declaration lock must precede revision read: calls=%v", tx.calls)
			}
		})
	}
}

type createCaseFakeBeginner struct{ tx pgx.Tx }

func (b createCaseFakeBeginner) BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error) {
	return b.tx, nil
}

type createCaseFakeTx struct {
	pgx.Tx
	actorActive      bool
	declarationID    string
	declarationFound bool
	calls            []string
	execCalls        []createCaseExecCall
	execCount        int
	committed        bool
}

type createCaseExecCall struct {
	sql  string
	args []any
}

func (tx *createCaseFakeTx) QueryRow(_ context.Context, sql string, _ ...any) pgx.Row {
	tx.calls = append(tx.calls, "query:"+sql)
	if strings.Contains(sql, "document_verification_lock_active_actor") {
		return createCaseFakeRow{value: tx.actorActive}
	}
	if strings.HasPrefix(sql, "SELECT id::text FROM public.identity_declaration_revision") && tx.declarationFound {
		return createCaseFakeRow{value: tx.declarationID}
	}
	return createCaseFakeRow{err: pgx.ErrNoRows}
}
func (tx *createCaseFakeTx) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	tx.calls = append(tx.calls, "exec:"+sql)
	tx.execCalls = append(tx.execCalls, createCaseExecCall{sql: sql, args: append([]any(nil), args...)})
	if !strings.Contains(sql, "pg_advisory_xact_lock(hashtextextended($1, 0))") {
		tx.execCount++
	}
	return pgconn.NewCommandTag("INSERT 0 1"), nil
}
func (tx *createCaseFakeTx) Commit(context.Context) error { tx.committed = true; return nil }
func (*createCaseFakeTx) Rollback(context.Context) error  { return nil }

type createCaseFakeRow struct {
	value any
	err   error
}

func (r createCaseFakeRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	switch d := dest[0].(type) {
	case *bool:
		*d = r.value.(bool)
	case *string:
		*d = r.value.(string)
	default:
		return errors.New("unexpected fake scan target")
	}
	return nil
}

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
	if cfg.ConnConfig.Host != "127.0.0.1" || cfg.ConnConfig.Port != 15434 || cfg.ConnConfig.Database != "espacigo_f5_gate_20261101" {
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
	if db != "espacigo_f5_gate_20261101" {
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

type scanClaimVerifyOptionsBeginner struct {
	options pgx.TxOptions
	query   string
	args    []any
}

func (b *scanClaimVerifyOptionsBeginner) BeginTx(_ context.Context, options pgx.TxOptions) (pgx.Tx, error) {
	b.options = options
	return scanClaimVerifyTx{beginner: b}, nil
}

type scanClaimVerifyTx struct {
	pgx.Tx
	beginner *scanClaimVerifyOptionsBeginner
}

func (tx scanClaimVerifyTx) QueryRow(_ context.Context, query string, args ...any) pgx.Row {
	tx.beginner.query = query
	tx.beginner.args = append([]any(nil), args...)
	return commitSerializationFailureRow{values: []any{true}}
}

func (scanClaimVerifyTx) Commit(context.Context) error   { return nil }
func (scanClaimVerifyTx) Rollback(context.Context) error { return nil }

func TestVerifyScanClaimDoesNotLockSpaceDuringPolling(t *testing.T) {
	beginner := &scanClaimVerifyOptionsBeginner{}
	repo := &Repository{transactions: beginner}
	claim := ScanClaim{
		JobID: "00000000-0000-4000-8000-000000000001", FileID: "00000000-0000-4000-8000-000000000002",
		CaseID: "00000000-0000-4000-8000-000000000003", SubjectID: "00000000-0000-4000-8000-000000000004",
		Purpose: dv.PurposeSpace, SpaceID: "00000000-0000-4000-8000-000000000005", Attempt: 1,
		CaseRevision: 1, LeaseUntil: time.Now().UTC().Add(time.Minute),
	}
	current, err := repo.VerifyScanClaim(context.Background(), claim)
	if err != nil || !current {
		t.Fatalf("VerifyScanClaim()=(%t,%v), want current claim", current, err)
	}
	if beginner.options.AccessMode != pgx.ReadOnly {
		t.Fatalf("VerifyScanClaim transaction access mode=%v, want read-only polling", beginner.options.AccessMode)
	}
	if strings.Contains(beginner.query, "document_verification_space_owned_by") || strings.Contains(beginner.query, "FOR UPDATE") {
		t.Fatalf("VerifyScanClaim polling query takes a conflicting lock: %s", beginner.query)
	}
}

func TestVerifyScanClaimBindsTheClaimedRetentionDeadline(t *testing.T) {
	beginner := &scanClaimVerifyOptionsBeginner{}
	repo := &Repository{transactions: beginner}
	prior := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)
	claim := ScanClaim{
		JobID: "00000000-0000-4000-8000-000000000001", FileID: "00000000-0000-4000-8000-000000000002",
		CaseID: "00000000-0000-4000-8000-000000000003", SubjectID: "00000000-0000-4000-8000-000000000004",
		Purpose: dv.PurposeSpace, SpaceID: "00000000-0000-4000-8000-000000000005", Attempt: 1,
		CaseRevision: 1, LeaseUntil: time.Now().UTC().Add(time.Minute), PriorRetentionAt: &prior,
	}
	if current, err := repo.VerifyScanClaim(context.Background(), claim); err != nil || !current {
		t.Fatalf("VerifyScanClaim()=(%t,%v), want retained but current claim", current, err)
	}
	if !strings.Contains(beginner.query, "f.retention_at IS NOT DISTINCT FROM $10::timestamptz") {
		t.Fatalf("VerifyScanClaim retention predicate = %q, want null-safe comparison with claimed deadline", beginner.query)
	}
	if len(beginner.args) != 10 {
		t.Fatalf("VerifyScanClaim args count=%d, want 10", len(beginner.args))
	}
	got, ok := beginner.args[9].(*time.Time)
	if !ok || got == nil || !got.Equal(prior) {
		t.Fatalf("VerifyScanClaim retention argument=%#v, want claimed deadline %s", beginner.args[9], prior)
	}
}

func (tx commitSerializationFailureTx) QueryRow(_ context.Context, sql string, _ ...any) pgx.Row {
	switch {
	case strings.Contains(sql, "document_verification_lock_active_actor"):
		return commitSerializationFailureRow{values: []any{true}}
	case strings.Contains(sql, "SELECT space_id::text,subject_id::text FROM public.document_verification_case"):
		return commitSerializationFailureRow{values: []any{nil, tx.actor}}
	case strings.Contains(sql, "WHERE c.id=$1 AND c.subject_id=$2"):
		return commitSerializationFailureRow{values: []any{tx.caseID, "identity", tx.actor, nil, "borrador", int64(1), "", nil, nil, nil, nil, nil, "", "", "", nil, "", ""}}
	case strings.Contains(sql, "WHERE c.id=$1 AND c.purpose='space'"):
		return commitSerializationFailureRow{values: []any{tx.caseID, "space", tx.actor, tx.spaceID, "borrador", int64(1), "", nil, nil, nil, nil, nil, "", "", "", nil, ""}}
	case strings.Contains(sql, "SELECT id::text,purpose,subject_id::text,space_id::text,state,revision"):
		return commitSerializationFailureRow{values: []any{tx.caseID, "identity", tx.actor, nil, "borrador", int64(1), "", nil, nil, nil, nil, nil, "", "", "", nil, ""}}
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
	case strings.Contains(sql, "SELECT retention_at FROM public.document_verification_file"):
		return commitSerializationFailureRow{values: []any{nil}}
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

type scanClaimRetentionBeginner struct{ tx *scanClaimRetentionTx }

func (b scanClaimRetentionBeginner) BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error) {
	return b.tx, nil
}

type scanClaimRetentionTx struct {
	pgx.Tx
	retentionAt  *time.Time
	reason       string
	candidateSQL string
}

func (tx *scanClaimRetentionTx) QueryRow(_ context.Context, sql string, _ ...any) pgx.Row {
	switch {
	case strings.Contains(sql, "FROM public.document_verification_job j"):
		tx.candidateSQL = sql
		return commitSerializationFailureRow{values: []any{
			"00000000-0000-4000-8000-000000000031",
			"00000000-0000-4000-8000-000000000032",
			"00000000-0000-4000-8000-000000000033",
			"00000000-0000-4000-8000-000000000034",
			string(dv.PurposeIdentity), "", int64(128), "image/png", 1,
			make([]byte, 32), int64(1),
		}}
	case strings.Contains(sql, "SELECT revision,superseded_at"):
		return commitSerializationFailureRow{values: []any{int64(1), nil}}
	case strings.Contains(sql, "SELECT retention_at FROM public.document_verification_file"):
		if tx.retentionAt == nil {
			return commitSerializationFailureRow{values: []any{nil}}
		}
		return commitSerializationFailureRow{values: []any{*tx.retentionAt}}
	case strings.Contains(sql, "SELECT attempts FROM public.document_verification_job"):
		return commitSerializationFailureRow{values: []any{int16(0)}}
	case strings.Contains(sql, "UPDATE public.document_verification_job SET state='procesando'"):
		return commitSerializationFailureRow{values: []any{int16(1), time.Now().UTC().Add(time.Minute)}}
	default:
		return commitSerializationFailureRow{err: errors.New("unexpected retention-preserving scan claim query")}
	}
}

func (tx *scanClaimRetentionTx) Exec(_ context.Context, sql string, _ ...any) (pgconn.CommandTag, error) {
	if strings.Contains(sql, "UPDATE public.document_verification_file SET state='pendiente'") {
		if strings.Contains(sql, "retention_at=NULL") {
			tx.retentionAt = nil
		}
		if strings.Contains(sql, "retention_reason_code=NULL") {
			tx.reason = ""
		}
	}
	return pgconn.CommandTag{}, nil
}

func (tx *scanClaimRetentionTx) Commit(context.Context) error   { return nil }
func (tx *scanClaimRetentionTx) Rollback(context.Context) error { return nil }

func TestClaimReadyScanJobKeepsPriorRetentionForOwnershipTransfer(t *testing.T) {
	deadline := time.Now().UTC().Add(6 * time.Hour)
	tx := &scanClaimRetentionTx{retentionAt: &deadline, reason: "inspection_error"}
	repo := newWithTransactionBeginner(scanClaimRetentionBeginner{tx: tx})
	claim, err := repo.ClaimReadyScanJob(context.Background(), time.Minute)
	if err != nil || claim == nil {
		t.Fatalf("ClaimReadyScanJob()=(%+v,%v), want a claim", claim, err)
	}
	if strings.Contains(tx.candidateSQL, "document_verification_space_owned_by") {
		t.Fatal("scan candidate query must not acquire locks for every matching space before the selected space is locked")
	}
	candidateSQL := strings.ToLower(strings.Join(strings.Fields(tx.candidateSQL), " "))
	if !strings.Contains(candidateSQL, "exists (select 1 from public.espacio") || !strings.Contains(candidateSQL, "s.propietario_id=c.subject_id") {
		t.Fatalf("scan candidate query must non-lockingly exclude stale space ownership before selecting the oldest job: %q", tx.candidateSQL)
	}
	if tx.retentionAt == nil || !tx.retentionAt.Equal(deadline) || tx.reason != "inspection_error" {
		t.Fatalf("scan claim discarded transfer-visible retention policy: deadline=%v reason=%q", tx.retentionAt, tx.reason)
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
	subjectID        string
	caseRevision     int64
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
		return commitSerializationFailureRow{values: []any{string(dv.PurposeIdentity), tx.subjectID, "", tx.caseRevision, nil}}
	case strings.Contains(sql, "SELECT revision,superseded_at"):
		return commitSerializationFailureRow{values: []any{int64(1), nil}}
	case strings.Contains(sql, "SELECT state,retention_at FROM public.document_verification_file"):
		return commitSerializationFailureRow{values: []any{"pendiente", nil}}
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
		JobID:        "00000000-0000-4000-8000-000000000032",
		FileID:       "00000000-0000-4000-8000-000000000033",
		CaseID:       "00000000-0000-4000-8000-000000000034",
		SubjectID:    "00000000-0000-4000-8000-000000000031",
		Purpose:      dv.PurposeIdentity,
		Attempt:      1,
		CaseRevision: 1,
		LeaseUntil:   time.Now().UTC().Add(time.Hour),
	}
	tx := &dbClockExpiredScanTx{leaseUntil: claim.LeaseUntil, dbNow: claim.LeaseUntil.Add(time.Second), subjectID: claim.SubjectID, caseRevision: 1}
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

func TestFinishScanRejectsChangedCaseRevisionBeforeApply(t *testing.T) {
	claim := ScanClaim{
		JobID:        "00000000-0000-4000-8000-000000000052",
		FileID:       "00000000-0000-4000-8000-000000000053",
		CaseID:       "00000000-0000-4000-8000-000000000054",
		SubjectID:    "00000000-0000-4000-8000-000000000051",
		Purpose:      dv.PurposeIdentity,
		Attempt:      1,
		CaseRevision: 2,
		LeaseUntil:   time.Now().UTC().Add(time.Hour),
	}
	tx := &dbClockExpiredScanTx{leaseUntil: claim.LeaseUntil, dbNow: claim.LeaseUntil.Add(-time.Second), subjectID: claim.SubjectID, caseRevision: 1}
	repo := newWithTransactionBeginner(dbClockExpiredScanBeginner{tx: tx})
	applyCalled := false
	err := repo.finishScan(context.Background(), claim, func(context.Context, pgx.Tx, time.Time) error {
		applyCalled = true
		return nil
	})
	if !errors.Is(err, dv.ErrConflict) || applyCalled || tx.applyExecInvoked {
		t.Fatalf("changed-revision scan result=(%v), apply_called=%t writes=%t; want conflict before writes", err, applyCalled, tx.applyExecInvoked)
	}
}

func TestFinishScanUsesDatabaseClockForOutcomeTime(t *testing.T) {
	claim := ScanClaim{
		JobID:        "00000000-0000-4000-8000-000000000042",
		FileID:       "00000000-0000-4000-8000-000000000043",
		CaseID:       "00000000-0000-4000-8000-000000000044",
		SubjectID:    "00000000-0000-4000-8000-000000000041",
		Purpose:      dv.PurposeIdentity,
		Attempt:      1,
		CaseRevision: 1,
		LeaseUntil:   time.Now().UTC().Add(time.Hour),
	}
	dbNow := time.Date(2001, time.January, 2, 3, 4, 5, 0, time.UTC)
	tx := &dbClockExpiredScanTx{leaseUntil: claim.LeaseUntil, dbNow: dbNow, subjectID: claim.SubjectID, caseRevision: 1}
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
	if cfg.ConnConfig.Host != "127.0.0.1" || cfg.ConnConfig.Port != 15434 || cfg.ConnConfig.Database != "espacigo_f5_gate_20261101" {
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
	if cfg.ConnConfig.Host != "127.0.0.1" || cfg.ConnConfig.Port != 15434 || cfg.ConnConfig.Database != "espacigo_f5_gate_20261101" {
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
	if cfg.ConnConfig.Host != "127.0.0.1" || cfg.ConnConfig.Port != 15434 || cfg.ConnConfig.Database != "espacigo_f5_gate_20261101" {
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
	scanCase, err := repo.LoadOwnCase(ctx, oldOwner, caseID)
	if err != nil {
		t.Fatalf("reload case before seeding an active scan lease: %v", err)
	}
	var scanJobID string
	var scanAttempt int16
	var scanLeaseUntil time.Time
	if err := outer.QueryRow(ctx, `UPDATE public.document_verification_job SET state='procesando',attempts=attempts+1,lease_until=clock_timestamp()+interval '1 hour',updated_at=clock_timestamp() WHERE file_id=$1 AND kind='scan' AND state='pendiente' RETURNING id::text,attempts,lease_until`, secondFileID).Scan(&scanJobID, &scanAttempt, &scanLeaseUntil); err != nil {
		t.Fatalf("seed the unique transfer scan lease without claiming the shared queue: %v", err)
	}
	if _, err := outer.Exec(ctx, `UPDATE public.document_verification_job SET state='completado',lease_until=NULL WHERE file_id=$1 AND kind='scan' AND state='pendiente'`, fileID); err != nil {
		t.Fatalf("isolate the second file's scan fixture: %v", err)
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
	activeScanClaim := ScanClaim{JobID: scanJobID, FileID: secondFileID, CaseID: caseID, SubjectID: oldOwner, Purpose: dv.PurposeSpace, SpaceID: spaceID, Size: secondFile.Size, MIME: secondFile.MIME, Pages: secondFile.Pages, ContentSHA256: secondFile.ContentSHA256, Attempt: scanAttempt, CaseRevision: int64(scanCase.Revision), LeaseUntil: scanLeaseUntil}
	claimCurrent, err := repo.VerifyScanClaim(ctx, activeScanClaim)
	if err != nil || !claimCurrent {
		t.Fatalf("active scan claim validation=(%t,%v), want current before ownership transfer", claimCurrent, err)
	}
	if _, err := outer.Exec(ctx, `UPDATE public.espacio SET propietario_id=$2,actualizado_en=now() WHERE id=$1`, spaceID, newOwner); err != nil {
		t.Fatalf("synthetic ownership transfer failed: %v", err)
	}
	claimCurrent, err = repo.VerifyScanClaim(ctx, activeScanClaim)
	if err != nil || claimCurrent {
		t.Fatalf("transferred scan claim validation=(%t,%v), want fenced after ownership transfer", claimCurrent, err)
	}
	if err := repo.RecordScanOutcome(ctx, activeScanClaim, ScanOutcome{State: dv.FileClean, ScannerVersion: "synthetic-stale-result", SignaturesAt: time.Now().UTC()}); !errors.Is(err, dv.ErrConflict) {
		t.Fatalf("scan result claimed before transfer was not fenced after ownership changed: %v", err)
	}
	var staleScanEvents int
	if err := outer.QueryRow(ctx, `SELECT count(*) FROM public.document_verification_event WHERE case_id=$1 AND file_id=$2 AND action='scan_result'`, caseID, secondFileID).Scan(&staleScanEvents); err != nil || staleScanEvents != 0 {
		t.Fatalf("stale scan result wrote audit events: count=%d err=%v", staleScanEvents, err)
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
			expectedAttempts = 1
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
