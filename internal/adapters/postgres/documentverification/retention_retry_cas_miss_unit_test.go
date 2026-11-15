package documentverificationpg

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type retentionRetryCASMissTx struct {
	*retentionFencingUnitTx
	retryAttempts int
	retryArgs     []any
}

func (tx *retentionRetryCASMissTx) Exec(ctx context.Context, query string, args ...any) (pgconn.CommandTag, error) {
	if strings.Contains(query, "SET state='reintentar'") {
		tx.retryAttempts++
		tx.retryArgs = append([]any(nil), args...)
		tx.trace = append(tx.trace, "retry_zero_rows")
		return pgconn.NewCommandTag("UPDATE 0"), nil
	}
	return tx.retentionFencingUnitTx.Exec(ctx, query, args...)
}

type retentionRetryCASMissBeginner struct{ tx pgx.Tx }

func (b retentionRetryCASMissBeginner) BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error) {
	return b.tx, nil
}

func TestProcessClaimedRetentionJobRejectsRetryCASMiss(t *testing.T) {
	const (
		jobID         = "00000000-0000-4000-8000-0000000000c1"
		fileID        = "00000000-0000-4000-8000-0000000000c2"
		caseID        = "00000000-0000-4000-8000-0000000000c3"
		subjectID     = "00000000-0000-4000-8000-0000000000c4"
		privateMarker = "/synthetic/private/retry-object"
	)
	claim := RetentionClaim{
		JobID: jobID, FileID: fileID, CaseID: caseID, SubjectID: subjectID,
		Purpose: dv.PurposeIdentity, LeaseUntil: time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC),
	}
	storageErr := errors.New("synthetic storage failure: " + privateMarker)
	baseTx := &retentionFencingUnitTx{claim: claim, leaseMatches: true, leaseLive: true}
	tx := &retentionRetryCASMissTx{retentionFencingUnitTx: baseTx}
	repo := newWithTransactionBeginner(retentionRetryCASMissBeginner{tx: tx})
	callbackCalls := 0
	err := repo.ProcessClaimedRetentionJob(context.Background(), claim, func(context.Context) error {
		callbackCalls++
		return storageErr
	}, time.Minute)
	if !errors.Is(err, dv.ErrConflict) || callbackCalls != 1 {
		t.Fatalf("ProcessClaimedRetentionJob() error=%v callback calls=%d; want retry CAS conflict after one callback", err, callbackCalls)
	}
	if tx.retryAttempts != 1 || len(tx.retryArgs) != 5 || tx.retryArgs[0] != jobID || tx.retryArgs[1] != fileID || tx.retryArgs[2] != time.Minute.String() || tx.retryArgs[3] != retentionStoreError || tx.retryArgs[4] != claim.LeaseUntil {
		t.Fatalf("retry attempts=%d args=%#v; want exact IDs, delay, allowlisted code, and lease", tx.retryAttempts, tx.retryArgs)
	}
	for _, arg := range tx.retryArgs {
		if value, ok := arg.(string); ok && strings.Contains(value, privateMarker) {
			t.Fatalf("raw storage detail reached retry update arguments: %q", value)
		}
	}
	if baseTx.committed || baseTx.rollbacks != 1 || len(baseTx.execs) != 0 || baseTx.unexpectedExec != "" || !reflect.DeepEqual(baseTx.trace, []string{"retry_zero_rows"}) {
		t.Fatalf("committed=%t rollbacks=%d execs=%v trace=%v unexpected=%q", baseTx.committed, baseTx.rollbacks, baseTx.execs, baseTx.trace, baseTx.unexpectedExec)
	}
}
