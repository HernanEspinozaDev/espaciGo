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

type singleRetentionTxBeginner struct{ tx pgx.Tx }

func (b singleRetentionTxBeginner) BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error) {
	return b.tx, nil
}

type cancelAfterRetentionExecTx struct {
	*retentionFencingUnitTx
	cancel        context.CancelFunc
	cancelOnQuery string
}

func (tx *cancelAfterRetentionExecTx) Exec(ctx context.Context, query string, args ...any) (pgconn.CommandTag, error) {
	tag, err := tx.retentionFencingUnitTx.Exec(ctx, query, args...)
	if strings.Contains(query, tx.cancelOnQuery) {
		tx.cancel()
	}
	return tag, err
}

func TestProcessClaimedRetentionJobStopsWhenContextCancelsAfterSuccessfulDelete(t *testing.T) {
	claim := RetentionClaim{
		JobID: "11111111-1111-4111-8111-111111111111", FileID: "22222222-2222-4222-8222-222222222222",
		CaseID: "33333333-3333-4333-8333-333333333333", SubjectID: "44444444-4444-4444-8444-444444444444",
		Purpose: dv.PurposeIdentity, Attempt: 1, LeaseUntil: time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC),
	}
	tx := &retentionFencingUnitTx{claim: claim, leaseMatches: true, leaseLive: true}
	repository := newWithTransactionBeginner(retentionFencingUnitBeginner{tx: tx})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	deleteCalls := 0

	err := repository.ProcessClaimedRetentionJob(ctx, claim, func(context.Context) error {
		deleteCalls++
		tx.trace = append(tx.trace, "delete")
		cancel()
		return nil
	}, time.Minute)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ProcessClaimedRetentionJob() error=%v, want context cancellation", err)
	}
	if deleteCalls != 1 || tx.committed || tx.rollbacks != 1 || len(tx.execs) != 0 {
		t.Fatalf("delete calls=%d committed=%t rollbacks=%d execs=%v; want one delete, rollback and no completion writes", deleteCalls, tx.committed, tx.rollbacks, tx.execs)
	}
	if len(tx.trace) != 1 || tx.trace[0] != "delete" {
		t.Fatalf("transaction trace=%v, want only the successful delete before cancellation", tx.trace)
	}
}

func TestProcessClaimedRetentionJobStopsBeforeAuditWhenContextCancelsAfterCompletionWrite(t *testing.T) {
	claim := RetentionClaim{
		JobID: "55555555-5555-4555-8555-555555555555", FileID: "66666666-6666-4666-8666-666666666666",
		CaseID: "77777777-7777-4777-8777-777777777777", SubjectID: "88888888-8888-4888-8888-888888888888",
		Purpose: dv.PurposeIdentity, Attempt: 1, LeaseUntil: time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC),
	}
	baseTx := &retentionFencingUnitTx{claim: claim, leaseMatches: true, leaseLive: true}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tx := &cancelAfterRetentionExecTx{
		retentionFencingUnitTx: baseTx,
		cancel:                 cancel,
		cancelOnQuery:          "SET state='completado'",
	}
	repository := newWithTransactionBeginner(singleRetentionTxBeginner{tx: tx})

	err := repository.ProcessClaimedRetentionJob(ctx, claim, func(context.Context) error {
		baseTx.trace = append(baseTx.trace, "delete")
		return nil
	}, time.Minute)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ProcessClaimedRetentionJob() error=%v, want cancellation after completion write", err)
	}
	if baseTx.committed || baseTx.rollbacks != 1 || len(baseTx.execs) != 1 || baseTx.execs[0] != "complete" {
		t.Fatalf("committed=%t rollbacks=%d execs=%v, want rollback after completion write without audit", baseTx.committed, baseTx.rollbacks, baseTx.execs)
	}
	if len(baseTx.trace) != 2 || baseTx.trace[0] != "delete" || baseTx.trace[1] != "complete" {
		t.Fatalf("transaction trace=%v, want delete then completion update only", baseTx.trace)
	}
}

func TestProcessClaimedRetentionJobRollsBackWhenContextCancelsAfterAuditWrite(t *testing.T) {
	claim := RetentionClaim{
		JobID: "99999999-9999-4999-8999-999999999999", FileID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
		CaseID: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", SubjectID: "cccccccc-cccc-4ccc-8ccc-cccccccccccc",
		Purpose: dv.PurposeIdentity, Attempt: 1, LeaseUntil: time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC),
	}
	baseTx := &retentionFencingUnitTx{claim: claim, leaseMatches: true, leaseLive: true}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tx := &cancelAfterRetentionExecTx{
		retentionFencingUnitTx: baseTx,
		cancel:                 cancel,
		cancelOnQuery:          "INSERT INTO public.document_verification_event",
	}
	repository := newWithTransactionBeginner(singleRetentionTxBeginner{tx: tx})

	err := repository.ProcessClaimedRetentionJob(ctx, claim, func(context.Context) error {
		baseTx.trace = append(baseTx.trace, "delete")
		return nil
	}, time.Minute)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ProcessClaimedRetentionJob() error=%v, want cancellation after audit write", err)
	}
	if baseTx.committed || baseTx.rollbacks != 1 || len(baseTx.execs) != 2 || baseTx.execs[0] != "complete" || baseTx.execs[1] != "audit" {
		t.Fatalf("committed=%t rollbacks=%d execs=%v, want rollback after completion and audit writes", baseTx.committed, baseTx.rollbacks, baseTx.execs)
	}
	if len(baseTx.trace) != 3 || baseTx.trace[0] != "delete" || baseTx.trace[1] != "complete" || baseTx.trace[2] != "audit" {
		t.Fatalf("transaction trace=%v, want delete, completion and audit before rollback", baseTx.trace)
	}
}
