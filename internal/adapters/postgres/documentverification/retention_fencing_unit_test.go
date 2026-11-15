package documentverificationpg

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type retentionFencingUnitBeginner struct{ tx *retentionFencingUnitTx }

type retentionFencingUnitScope struct {
	purpose   dv.Purpose
	subjectID string
	spaceID   string
}

func (b retentionFencingUnitBeginner) BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error) {
	return b.tx, nil
}

type retentionFencingUnitTx struct {
	pgx.Tx
	claim          RetentionClaim
	caseScope      *retentionFencingUnitScope
	leaseMatches   bool
	leaseLive      bool
	activeScan     bool
	committed      bool
	rollbacks      int
	queryCount     int
	execs          []string
	execArgs       [][]any
	trace          []string
	unexpectedSQL  string
	unexpectedExec string
}

func (tx *retentionFencingUnitTx) QueryRow(_ context.Context, query string, _ ...any) pgx.Row {
	tx.queryCount++
	purpose, subjectID, spaceID := tx.claim.Purpose, tx.claim.SubjectID, tx.claim.SpaceID
	if tx.caseScope != nil && strings.Contains(query, "SELECT purpose,subject_id::text,COALESCE(space_id::text,'')") && strings.Contains(query, "FOR UPDATE") {
		purpose, subjectID, spaceID = tx.caseScope.purpose, tx.caseScope.subjectID, tx.caseScope.spaceID
	}
	switch {
	case strings.Contains(query, "document_verification_lock_space_row"):
		return commitSerializationFailureRow{values: []any{true}}
	case strings.Contains(query, "SELECT purpose,subject_id::text,COALESCE(space_id::text,'')") && strings.Contains(query, "FOR UPDATE"):
		return commitSerializationFailureRow{values: []any{string(purpose), subjectID, spaceID, "vencida", int64(9)}}
	case strings.Contains(query, "SELECT purpose,subject_id::text,COALESCE(space_id::text,'')"):
		return commitSerializationFailureRow{values: []any{string(purpose), subjectID, spaceID}}
	case strings.Contains(query, "SELECT id::text FROM public.document_verification_file"):
		return commitSerializationFailureRow{values: []any{tx.claim.FileID}}
	case strings.Contains(query, "SELECT state,COALESCE(lease_until=$2,false),COALESCE(lease_until>clock_timestamp(),false)"):
		return commitSerializationFailureRow{values: []any{"procesando", tx.leaseMatches, tx.leaseLive}}
	case strings.Contains(query, "SELECT EXISTS(SELECT 1 FROM public.document_verification_job WHERE file_id=$1 AND kind='scan'"):
		return commitSerializationFailureRow{values: []any{tx.activeScan}}
	default:
		tx.unexpectedSQL = query
		return commitSerializationFailureRow{err: errors.New("unexpected retention fencing query")}
	}
}

func (tx *retentionFencingUnitTx) Commit(context.Context) error {
	tx.committed = true
	tx.trace = append(tx.trace, "commit")
	return nil
}

func (tx *retentionFencingUnitTx) Rollback(context.Context) error {
	tx.rollbacks++
	return nil
}

func (tx *retentionFencingUnitTx) Exec(_ context.Context, query string, args ...any) (pgconn.CommandTag, error) {
	switch {
	case strings.Contains(query, "SET state='reintentar'"):
		tx.execs = append(tx.execs, "retry")
		tx.trace = append(tx.trace, "retry")
		tx.execArgs = append(tx.execArgs, append([]any(nil), args...))
		return pgconn.NewCommandTag("UPDATE 1"), nil
	case strings.Contains(query, "UPDATE public.document_verification_job SET state='completado'"):
		tx.execs = append(tx.execs, "complete")
		tx.trace = append(tx.trace, "complete")
		return pgconn.NewCommandTag("UPDATE 1"), nil
	case strings.Contains(query, "INSERT INTO public.document_verification_event"):
		tx.execs = append(tx.execs, "audit")
		tx.trace = append(tx.trace, "audit")
		return pgconn.NewCommandTag("INSERT 0 1"), nil
	default:
		tx.unexpectedExec = query
		return pgconn.CommandTag{}, errors.New("unexpected retention fencing Exec")
	}
}

func TestProcessClaimedRetentionJobFencesMismatchedOrExpiredLeaseBeforeDeletion(t *testing.T) {
	for _, tc := range []struct {
		name         string
		leaseMatches bool
		leaseLive    bool
	}{
		{name: "lease replaced", leaseMatches: false, leaseLive: true},
		{name: "lease expired", leaseMatches: true, leaseLive: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			claim := RetentionClaim{
				JobID: "00000000-0000-4000-8000-000000000011", FileID: "00000000-0000-4000-8000-000000000012",
				CaseID: "00000000-0000-4000-8000-000000000013", SubjectID: "00000000-0000-4000-8000-000000000014",
				Purpose: dv.PurposeIdentity, Attempt: 2, LeaseUntil: time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC),
			}
			tx := &retentionFencingUnitTx{claim: claim, leaseMatches: tc.leaseMatches, leaseLive: tc.leaseLive}
			repo := newWithTransactionBeginner(retentionFencingUnitBeginner{tx: tx})
			deleteCalled := false
			err := repo.ProcessClaimedRetentionJob(context.Background(), claim, func(context.Context) error {
				deleteCalled = true
				return nil
			}, time.Minute)
			if !errors.Is(err, dv.ErrConflict) {
				t.Fatalf("ProcessClaimedRetentionJob() error=%v, want conflict", err)
			}
			if deleteCalled {
				t.Fatal("storage deletion callback ran without the exact live lease")
			}
			if tx.committed || tx.rollbacks != 1 {
				t.Fatalf("transaction committed=%t rollbacks=%d, want no commit and one rollback", tx.committed, tx.rollbacks)
			}
			if tx.unexpectedSQL != "" || tx.unexpectedExec != "" || tx.queryCount != 4 || len(tx.execs) != 0 {
				t.Fatalf("query count=%d unexpected query=%q unexpected exec=%q execs=%v", tx.queryCount, tx.unexpectedSQL, tx.unexpectedExec, tx.execs)
			}
		})
	}
}

func TestProcessClaimedRetentionJobDoesNotDeleteWhileScanIsActive(t *testing.T) {
	claim := RetentionClaim{
		JobID: "00000000-0000-4000-8000-000000000041", FileID: "00000000-0000-4000-8000-000000000042",
		CaseID: "00000000-0000-4000-8000-000000000043", SubjectID: "00000000-0000-4000-8000-000000000044",
		Purpose: dv.PurposeIdentity, Attempt: 1, LeaseUntil: time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC),
	}
	tx := &retentionFencingUnitTx{claim: claim, leaseMatches: true, leaseLive: true, activeScan: true}
	repo := newWithTransactionBeginner(retentionFencingUnitBeginner{tx: tx})
	deleteCalled := false
	err := repo.ProcessClaimedRetentionJob(context.Background(), claim, func(context.Context) error {
		deleteCalled = true
		return nil
	}, time.Minute)
	if !errors.Is(err, dv.ErrConflict) || deleteCalled {
		t.Fatalf("ProcessClaimedRetentionJob() error=%v deleteCalled=%t, want conflict before deletion", err, deleteCalled)
	}
	if tx.committed || tx.rollbacks != 1 || tx.unexpectedSQL != "" || tx.unexpectedExec != "" || tx.queryCount != 5 || len(tx.execs) != 0 {
		t.Fatalf("committed=%t rollbacks=%d queryCount=%d unexpected query=%q exec=%q execs=%v", tx.committed, tx.rollbacks, tx.queryCount, tx.unexpectedSQL, tx.unexpectedExec, tx.execs)
	}
}

func TestProcessClaimedRetentionJobRejectsChangedCaseScopeBeforeDeletion(t *testing.T) {
	const (
		jobID      = "00000000-0000-4000-8000-000000000051"
		fileID     = "00000000-0000-4000-8000-000000000052"
		caseID     = "00000000-0000-4000-8000-000000000053"
		subjectID  = "00000000-0000-4000-8000-000000000054"
		otherOwner = "00000000-0000-4000-8000-000000000055"
		spaceID    = "00000000-0000-4000-8000-000000000056"
		otherSpace = "00000000-0000-4000-8000-000000000057"
	)
	for _, tc := range []struct {
		name      string
		claim     RetentionClaim
		caseScope retentionFencingUnitScope
		queries   int
	}{
		{
			name:      "purpose changed",
			claim:     RetentionClaim{JobID: jobID, FileID: fileID, CaseID: caseID, SubjectID: subjectID, Purpose: dv.PurposeIdentity, LeaseUntil: time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)},
			caseScope: retentionFencingUnitScope{purpose: dv.PurposeSpace, subjectID: subjectID, spaceID: spaceID},
			queries:   2,
		},
		{
			name:      "subject changed",
			claim:     RetentionClaim{JobID: jobID, FileID: fileID, CaseID: caseID, SubjectID: subjectID, Purpose: dv.PurposeIdentity, LeaseUntil: time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)},
			caseScope: retentionFencingUnitScope{purpose: dv.PurposeIdentity, subjectID: otherOwner},
			queries:   2,
		},
		{
			name:      "space changed",
			claim:     RetentionClaim{JobID: jobID, FileID: fileID, CaseID: caseID, SubjectID: subjectID, Purpose: dv.PurposeSpace, SpaceID: spaceID, LeaseUntil: time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)},
			caseScope: retentionFencingUnitScope{purpose: dv.PurposeSpace, subjectID: subjectID, spaceID: otherSpace},
			queries:   3,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx := &retentionFencingUnitTx{claim: tc.claim, caseScope: &tc.caseScope}
			repo := newWithTransactionBeginner(retentionFencingUnitBeginner{tx: tx})
			deleteCalled := false
			err := repo.ProcessClaimedRetentionJob(context.Background(), tc.claim, func(context.Context) error {
				deleteCalled = true
				return nil
			}, time.Minute)
			if !errors.Is(err, dv.ErrConflict) || deleteCalled {
				t.Fatalf("ProcessClaimedRetentionJob() error=%v deleteCalled=%t, want scope conflict before deletion", err, deleteCalled)
			}
			if tx.committed || tx.rollbacks != 1 || tx.queryCount != tc.queries || tx.unexpectedSQL != "" || tx.unexpectedExec != "" || len(tx.execs) != 0 {
				t.Fatalf("committed=%t rollbacks=%d queries=%d want=%d unexpected query=%q exec=%q execs=%v", tx.committed, tx.rollbacks, tx.queryCount, tc.queries, tx.unexpectedSQL, tx.unexpectedExec, tx.execs)
			}
		})
	}
}

func TestProcessClaimedRetentionJobRunsDeletionForExactLiveLease(t *testing.T) {
	claim := RetentionClaim{
		JobID: "00000000-0000-4000-8000-000000000021", FileID: "00000000-0000-4000-8000-000000000022",
		CaseID: "00000000-0000-4000-8000-000000000023", SubjectID: "00000000-0000-4000-8000-000000000024",
		Purpose: dv.PurposeIdentity, Attempt: 2, LeaseUntil: time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC),
	}
	tx := &retentionFencingUnitTx{claim: claim, leaseMatches: true, leaseLive: true}
	repo := newWithTransactionBeginner(retentionFencingUnitBeginner{tx: tx})
	deleteCalled := false
	err := repo.ProcessClaimedRetentionJob(context.Background(), claim, func(context.Context) error {
		deleteCalled = true
		tx.trace = append(tx.trace, "delete")
		return nil
	}, time.Minute)
	if err != nil || !deleteCalled {
		t.Fatalf("ProcessClaimedRetentionJob() error=%v deleteCalled=%t, want successful deletion", err, deleteCalled)
	}
	if !tx.committed || tx.rollbacks != 1 {
		t.Fatalf("transaction committed=%t rollbacks=%d, want commit and deferred rollback", tx.committed, tx.rollbacks)
	}
	if tx.unexpectedSQL != "" || tx.unexpectedExec != "" || tx.queryCount != 5 || len(tx.execs) != 2 || tx.execs[0] != "complete" || tx.execs[1] != "audit" || len(tx.trace) != 4 || tx.trace[0] != "delete" || tx.trace[1] != "complete" || tx.trace[2] != "audit" || tx.trace[3] != "commit" {
		t.Fatalf("query count=%d unexpected query=%q unexpected exec=%q execs=%v trace=%v", tx.queryCount, tx.unexpectedSQL, tx.unexpectedExec, tx.execs, tx.trace)
	}
}

func TestProcessClaimedRetentionJobDoesNotClassifyCanceledDeletionAsStorageFailure(t *testing.T) {
	claim := RetentionClaim{
		JobID: "00000000-0000-4000-8000-000000000051", FileID: "00000000-0000-4000-8000-000000000052",
		CaseID: "00000000-0000-4000-8000-000000000053", SubjectID: "00000000-0000-4000-8000-000000000054",
		Purpose: dv.PurposeIdentity, Attempt: 2, LeaseUntil: time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC),
	}
	tx := &retentionFencingUnitTx{claim: claim, leaseMatches: true, leaseLive: true}
	repo := newWithTransactionBeginner(retentionFencingUnitBeginner{tx: tx})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err := repo.ProcessClaimedRetentionJob(ctx, claim, func(context.Context) error {
		tx.trace = append(tx.trace, "delete")
		cancel()
		return errors.New("synthetic delete interrupted")
	}, time.Minute)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ProcessClaimedRetentionJob() error=%v, want context cancellation", err)
	}
	if tx.committed || tx.rollbacks != 1 || len(tx.execs) != 0 || tx.unexpectedExec != "" {
		t.Fatalf("committed=%t rollbacks=%d execs=%v unexpected exec=%q, want rollback without retry/completion writes", tx.committed, tx.rollbacks, tx.execs, tx.unexpectedExec)
	}
	if len(tx.trace) != 1 || tx.trace[0] != "delete" {
		t.Fatalf("trace=%v, want interrupted delete only", tx.trace)
	}
}

func TestProcessClaimedRetentionJobRequestsSanitizedRetryUpdateAfterStorageFailure(t *testing.T) {
	claim := RetentionClaim{
		JobID: "00000000-0000-4000-8000-000000000031", FileID: "00000000-0000-4000-8000-000000000032",
		CaseID: "00000000-0000-4000-8000-000000000033", SubjectID: "00000000-0000-4000-8000-000000000034",
		Purpose: dv.PurposeIdentity, Attempt: 3, LeaseUntil: time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC),
	}
	const rawStorageFailure = "synthetic private-path=/identity/fixture-31"
	const retryDelay = 37 * time.Second
	tx := &retentionFencingUnitTx{claim: claim, leaseMatches: true, leaseLive: true}
	repo := newWithTransactionBeginner(retentionFencingUnitBeginner{tx: tx})
	deleteCalled := false
	err := repo.ProcessClaimedRetentionJob(context.Background(), claim, func(context.Context) error {
		deleteCalled = true
		tx.trace = append(tx.trace, "delete")
		return errors.New(rawStorageFailure)
	}, retryDelay)
	if err != nil || !deleteCalled {
		t.Fatalf("ProcessClaimedRetentionJob() error=%v deleteCalled=%t, want retry recorded", err, deleteCalled)
	}
	if !tx.committed || tx.rollbacks != 1 || tx.unexpectedSQL != "" || tx.unexpectedExec != "" {
		t.Fatalf("committed=%t rollbacks=%d unexpected query=%q exec=%q", tx.committed, tx.rollbacks, tx.unexpectedSQL, tx.unexpectedExec)
	}
	if len(tx.execs) != 1 || tx.execs[0] != "retry" || len(tx.execArgs) != 1 || len(tx.execArgs[0]) != 5 {
		t.Fatalf("retry execs=%v args=%v, want exactly one fenced retry update", tx.execs, tx.execArgs)
	}
	args := tx.execArgs[0]
	if args[0] != claim.JobID || args[1] != claim.FileID || args[2] != retryDelay.String() || args[3] != "retention_store_error" || args[4] != claim.LeaseUntil {
		t.Fatalf("retry update args=%#v, want job/file, bounded delay, sanitized code and exact lease", args)
	}
	for _, arg := range args {
		if value, ok := arg.(string); ok && strings.Contains(value, rawStorageFailure) {
			t.Fatal("raw storage error was persisted instead of a sanitized code")
		}
	}
	if len(tx.trace) != 3 || tx.trace[0] != "delete" || tx.trace[1] != "retry" || tx.trace[2] != "commit" {
		t.Fatalf("callback/retry/commit order=%v, want delete then retry update then commit", tx.trace)
	}
}
