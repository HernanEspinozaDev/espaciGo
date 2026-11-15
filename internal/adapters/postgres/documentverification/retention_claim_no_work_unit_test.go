package documentverificationpg

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

type retentionClaimNoWorkTx struct {
	pgx.Tx
	queries       []string
	commitCalls   int
	rollbackCalls int
	trace         []string
}

func (tx *retentionClaimNoWorkTx) QueryRow(_ context.Context, query string, _ ...any) pgx.Row {
	tx.queries = append(tx.queries, query)
	tx.trace = append(tx.trace, "candidate_query")
	return commitSerializationFailureRow{err: pgx.ErrNoRows}
}

func (tx *retentionClaimNoWorkTx) Commit(context.Context) error {
	tx.commitCalls++
	tx.trace = append(tx.trace, "commit")
	return nil
}

func (tx *retentionClaimNoWorkTx) Rollback(context.Context) error {
	tx.rollbackCalls++
	tx.trace = append(tx.trace, "rollback")
	return nil
}

type retentionClaimNoWorkBeginner struct{ tx pgx.Tx }

func (b retentionClaimNoWorkBeginner) BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error) {
	return b.tx, nil
}

func TestClaimReadyRetentionJobReturnsNoWorkWhenCandidateQueryIsEmpty(t *testing.T) {
	tx := &retentionClaimNoWorkTx{}
	repo := newWithTransactionBeginner(retentionClaimNoWorkBeginner{tx: tx})
	claim, err := repo.ClaimReadyRetentionJob(context.Background(), time.Minute)
	if err != nil || claim != nil {
		t.Fatalf("ClaimReadyRetentionJob()=(%+v,%v), want no claim and no error", claim, err)
	}
	if len(tx.queries) != 1 || !strings.Contains(tx.queries[0], "FROM public.document_verification_job j") || tx.commitCalls != 0 || tx.rollbackCalls != 1 || len(tx.trace) != 2 || tx.trace[0] != "candidate_query" || tx.trace[1] != "rollback" {
		t.Fatalf("queries=%q commits=%d rollbacks=%d trace=%v; want candidate query then deferred rollback without commit", tx.queries, tx.commitCalls, tx.rollbackCalls, tx.trace)
	}
}
