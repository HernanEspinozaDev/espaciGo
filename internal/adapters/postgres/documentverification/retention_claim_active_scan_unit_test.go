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

type retentionClaimActiveScanTx struct {
	pgx.Tx
	claim          RetentionClaim
	queries        []string
	committed      bool
	rollbacks      int
	updateAttempts int
	unexpectedSQL  string
}

func (tx *retentionClaimActiveScanTx) QueryRow(_ context.Context, query string, _ ...any) pgx.Row {
	tx.queries = append(tx.queries, query)
	switch {
	case strings.Contains(query, "FROM public.document_verification_job j"):
		return commitSerializationFailureRow{values: []any{tx.claim.JobID, tx.claim.FileID, tx.claim.CaseID, tx.claim.SubjectID, string(tx.claim.Purpose), tx.claim.SpaceID, tx.claim.Attempt}}
	case strings.Contains(query, "SELECT id FROM public.document_verification_case"):
		return commitSerializationFailureRow{values: []any{tx.claim.CaseID}}
	case strings.Contains(query, "SELECT id FROM public.document_verification_file"):
		return commitSerializationFailureRow{values: []any{tx.claim.FileID}}
	case strings.Contains(query, "SELECT EXISTS(SELECT 1 FROM public.document_verification_job WHERE file_id=$1 AND kind='scan'"):
		return commitSerializationFailureRow{values: []any{true}}
	case strings.Contains(query, "UPDATE public.document_verification_job SET state='procesando'"):
		tx.updateAttempts++
		tx.unexpectedSQL = query
		return commitSerializationFailureRow{err: errors.New("retention claim update must not run while scan is active")}
	default:
		tx.unexpectedSQL = query
		return commitSerializationFailureRow{err: errors.New("unexpected retention claim query")}
	}
}

func (tx *retentionClaimActiveScanTx) Commit(context.Context) error {
	tx.committed = true
	return nil
}

func (tx *retentionClaimActiveScanTx) Rollback(context.Context) error {
	tx.rollbacks++
	return nil
}

type retentionClaimActiveScanBeginner struct{ tx pgx.Tx }

func (b retentionClaimActiveScanBeginner) BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error) {
	return b.tx, nil
}

func TestClaimReadyRetentionJobDeclinesClaimWhenScanAppearsBeforeRecheck(t *testing.T) {
	claim := RetentionClaim{
		JobID: "00000000-0000-4000-8000-000000000111", FileID: "00000000-0000-4000-8000-000000000112",
		CaseID: "00000000-0000-4000-8000-000000000113", SubjectID: "00000000-0000-4000-8000-000000000114",
		Purpose: dv.PurposeIdentity, Attempt: 2, LeaseUntil: time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC),
	}
	tx := &retentionClaimActiveScanTx{claim: claim}
	repo := newWithTransactionBeginner(retentionClaimActiveScanBeginner{tx: tx})
	got, err := repo.ClaimReadyRetentionJob(context.Background(), time.Minute)
	if err != nil || got != nil {
		t.Fatalf("ClaimReadyRetentionJob()=(%+v,%v), want no claim and no error", got, err)
	}
	if len(tx.queries) != 4 || !strings.Contains(tx.queries[3], "SELECT EXISTS(SELECT 1 FROM public.document_verification_job WHERE file_id=$1 AND kind='scan'") {
		t.Fatalf("queries=%q; want candidate, case lock, file lock, and active-scan recheck", tx.queries)
	}
	if tx.updateAttempts != 0 || tx.unexpectedSQL != "" || !tx.committed || tx.rollbacks != 1 {
		t.Fatalf("update attempts=%d unexpected SQL=%q committed=%t rollbacks=%d; want no claim update and committed no-work transaction", tx.updateAttempts, tx.unexpectedSQL, tx.committed, tx.rollbacks)
	}
}
