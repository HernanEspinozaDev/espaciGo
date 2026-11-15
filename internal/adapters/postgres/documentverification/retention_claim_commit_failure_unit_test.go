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

type retentionClaimCommitFailureTx struct {
	pgx.Tx
	claim         RetentionClaim
	queries       []string
	updateArgs    []any
	commitError   error
	commitCalls   int
	rollbackCalls int
	unexpectedSQL string
}

func (tx *retentionClaimCommitFailureTx) QueryRow(_ context.Context, query string, args ...any) pgx.Row {
	tx.queries = append(tx.queries, query)
	switch {
	case strings.Contains(query, "FROM public.document_verification_job j"):
		return commitSerializationFailureRow{values: []any{tx.claim.JobID, tx.claim.FileID, tx.claim.CaseID, tx.claim.SubjectID, string(tx.claim.Purpose), tx.claim.SpaceID, tx.claim.Attempt}}
	case strings.Contains(query, "SELECT id FROM public.document_verification_case"):
		return commitSerializationFailureRow{values: []any{tx.claim.CaseID}}
	case strings.Contains(query, "SELECT id FROM public.document_verification_file"):
		return commitSerializationFailureRow{values: []any{tx.claim.FileID}}
	case strings.Contains(query, "SELECT EXISTS(SELECT 1 FROM public.document_verification_job WHERE file_id=$1 AND kind='scan'"):
		return commitSerializationFailureRow{values: []any{false}}
	case strings.Contains(query, "UPDATE public.document_verification_job SET state='procesando'"):
		tx.updateArgs = append([]any(nil), args...)
		return commitSerializationFailureRow{values: []any{tx.claim.Attempt + 1, tx.claim.LeaseUntil}}
	default:
		tx.unexpectedSQL = query
		return commitSerializationFailureRow{err: errors.New("unexpected retention claim query")}
	}
}

func (tx *retentionClaimCommitFailureTx) Commit(context.Context) error {
	tx.commitCalls++
	return tx.commitError
}

func (tx *retentionClaimCommitFailureTx) Rollback(context.Context) error {
	tx.rollbackCalls++
	return nil
}

type retentionClaimCommitFailureBeginner struct{ tx pgx.Tx }

func (b retentionClaimCommitFailureBeginner) BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error) {
	return b.tx, nil
}

func TestClaimReadyRetentionJobDoesNotReturnClaimWhenCommitFails(t *testing.T) {
	const (
		jobID     = "00000000-0000-4000-8000-000000000121"
		fileID    = "00000000-0000-4000-8000-000000000122"
		caseID    = "00000000-0000-4000-8000-000000000123"
		subjectID = "00000000-0000-4000-8000-000000000124"
	)
	leaseUntil := time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
	claim := RetentionClaim{
		JobID: jobID, FileID: fileID, CaseID: caseID, SubjectID: subjectID,
		Purpose: dv.PurposeIdentity, Attempt: 2, LeaseUntil: leaseUntil,
	}
	commitErr := errors.New("synthetic retention claim commit failure")
	tx := &retentionClaimCommitFailureTx{claim: claim, commitError: commitErr}
	repo := newWithTransactionBeginner(retentionClaimCommitFailureBeginner{tx: tx})
	got, err := repo.ClaimReadyRetentionJob(context.Background(), time.Minute)
	if !errors.Is(err, commitErr) || got != nil {
		t.Fatalf("ClaimReadyRetentionJob()=(%+v,%v); want nil claim and commit error", got, err)
	}
	if len(tx.queries) != 5 || !strings.Contains(tx.queries[4], "UPDATE public.document_verification_job SET state='procesando'") {
		t.Fatalf("queries=%q; want candidate, locks, scan recheck, and claim update", tx.queries)
	}
	if len(tx.updateArgs) != 3 || tx.updateArgs[0] != jobID || tx.updateArgs[1] != (time.Minute).String() || tx.updateArgs[2] != nil {
		t.Fatalf("claim update args=%#v; want job ID, lease duration and no ready-before filter", tx.updateArgs)
	}
	if tx.commitCalls != 1 || tx.rollbackCalls != 1 || tx.unexpectedSQL != "" {
		t.Fatalf("commit calls=%d rollback calls=%d unexpected SQL=%q; want one failed commit and deferred rollback", tx.commitCalls, tx.rollbackCalls, tx.unexpectedSQL)
	}
}
