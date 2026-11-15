package documentverificationpg

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
	"github.com/jackc/pgx/v5"
)

type retentionClaimMissingSpaceTx struct {
	pgx.Tx
	claim         RetentionClaim
	queries       []string
	spaceLockArgs []any
	committed     bool
	commitCalls   int
	commitError   error
	rollbacks     int
	unexpectedSQL string
}

func (tx *retentionClaimMissingSpaceTx) QueryRow(_ context.Context, query string, args ...any) pgx.Row {
	tx.queries = append(tx.queries, query)
	switch {
	case strings.Contains(query, "FROM public.document_verification_job j"):
		return commitSerializationFailureRow{values: []any{tx.claim.JobID, tx.claim.FileID, tx.claim.CaseID, tx.claim.SubjectID, string(tx.claim.Purpose), tx.claim.SpaceID, tx.claim.Attempt}}
	case strings.Contains(query, "document_verification_lock_space_row"):
		tx.spaceLockArgs = append([]any(nil), args...)
		return commitSerializationFailureRow{values: []any{false}}
	default:
		tx.unexpectedSQL = query
		return commitSerializationFailureRow{err: errors.New("retention claim continued after missing space lock")}
	}
}

func (tx *retentionClaimMissingSpaceTx) Commit(context.Context) error {
	tx.commitCalls++
	if tx.commitError != nil {
		return tx.commitError
	}
	tx.committed = true
	return nil
}

func (tx *retentionClaimMissingSpaceTx) Rollback(context.Context) error {
	tx.rollbacks++
	return nil
}

type retentionClaimMissingSpaceBeginner struct{ tx pgx.Tx }

func (b retentionClaimMissingSpaceBeginner) BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error) {
	return b.tx, nil
}

func TestClaimReadyRetentionJobDoesNotClaimWhenSpaceRowIsMissing(t *testing.T) {
	const (
		jobID     = "00000000-0000-4000-8000-000000000131"
		fileID    = "00000000-0000-4000-8000-000000000132"
		caseID    = "00000000-0000-4000-8000-000000000133"
		subjectID = "00000000-0000-4000-8000-000000000134"
		spaceID   = "00000000-0000-4000-8000-000000000135"
	)
	claim := RetentionClaim{
		JobID: jobID, FileID: fileID, CaseID: caseID, SubjectID: subjectID, SpaceID: spaceID,
		Purpose: dv.PurposeSpace, Attempt: 2, LeaseUntil: time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC),
	}
	tx := &retentionClaimMissingSpaceTx{claim: claim}
	repo := newWithTransactionBeginner(retentionClaimMissingSpaceBeginner{tx: tx})
	got, err := repo.ClaimReadyRetentionJob(context.Background(), time.Minute)
	if err != nil || got != nil {
		t.Fatalf("ClaimReadyRetentionJob()=(%+v,%v), want no claim and no error", got, err)
	}
	if len(tx.queries) != 2 || !strings.Contains(tx.queries[1], "document_verification_lock_space_row") || len(tx.spaceLockArgs) != 1 || tx.spaceLockArgs[0] != spaceID {
		t.Fatalf("queries=%q space lock args=%#v; want candidate followed by exact space-row lock", tx.queries, tx.spaceLockArgs)
	}
	if tx.unexpectedSQL != "" || !tx.committed || tx.commitCalls != 1 || tx.rollbacks != 1 {
		t.Fatalf("unexpected SQL=%q committed=%t commits=%d rollbacks=%d; want no further reads/update and committed no-work transaction", tx.unexpectedSQL, tx.committed, tx.commitCalls, tx.rollbacks)
	}
}

func TestClaimReadyRetentionJobPropagatesCommitFailureWhenSpaceRowIsMissing(t *testing.T) {
	const (
		jobID     = "00000000-0000-4000-8000-000000000191"
		fileID    = "00000000-0000-4000-8000-000000000192"
		caseID    = "00000000-0000-4000-8000-000000000193"
		subjectID = "00000000-0000-4000-8000-000000000194"
		spaceID   = "00000000-0000-4000-8000-000000000195"
	)
	commitErr := errors.New("synthetic no-work commit failure")
	claim := RetentionClaim{
		JobID: jobID, FileID: fileID, CaseID: caseID, SubjectID: subjectID, SpaceID: spaceID,
		Purpose: dv.PurposeSpace, Attempt: 2,
	}
	tx := &retentionClaimMissingSpaceTx{claim: claim, commitError: commitErr}
	repo := newWithTransactionBeginner(retentionClaimMissingSpaceBeginner{tx: tx})
	got, err := repo.ClaimReadyRetentionJob(context.Background(), time.Minute)
	if !errors.Is(err, commitErr) || got != nil {
		t.Fatalf("ClaimReadyRetentionJob()=(%+v,%v); want nil claim and no-work commit error", got, err)
	}
	if len(tx.queries) != 2 || !strings.Contains(tx.queries[1], "document_verification_lock_space_row") || len(tx.spaceLockArgs) != 1 || tx.spaceLockArgs[0] != spaceID {
		t.Fatalf("queries=%q space lock args=%#v; want candidate followed by exact space-row lock", tx.queries, tx.spaceLockArgs)
	}
	if tx.unexpectedSQL != "" || tx.committed || tx.commitCalls != 1 || tx.rollbacks != 1 {
		t.Fatalf("unexpected SQL=%q committed=%t commit attempts=%d rollbacks=%d; want no further reads/update, failed commit, no successful commit, and deferred rollback", tx.unexpectedSQL, tx.committed, tx.commitCalls, tx.rollbacks)
	}
}
