package documentverificationpg

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
)

func TestProcessClaimedRetentionJobReturnsRetryCommitFailure(t *testing.T) {
	claim := RetentionClaim{
		JobID: "00000000-0000-4000-8000-000000000101", FileID: "00000000-0000-4000-8000-000000000102",
		CaseID: "00000000-0000-4000-8000-000000000103", SubjectID: "00000000-0000-4000-8000-000000000104",
		Purpose: dv.PurposeIdentity, LeaseUntil: time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC),
	}
	const rawStoreError = "synthetic private-path=/identity/fixture-retry-commit"
	commitErr := errors.New("synthetic retry commit failure")
	baseTx := &retentionFencingUnitTx{claim: claim, leaseMatches: true, leaseLive: true}
	tx := &retentionCommitFailureTx{retentionFencingUnitTx: baseTx, err: commitErr}
	repo := newWithTransactionBeginner(retentionCommitFailureBeginner{tx: tx})
	deleteCalls := 0
	err := repo.ProcessClaimedRetentionJob(context.Background(), claim, func(context.Context) error {
		deleteCalls++
		baseTx.trace = append(baseTx.trace, "delete")
		return errors.New(rawStoreError)
	}, 37*time.Second)
	if !errors.Is(err, commitErr) || deleteCalls != 1 {
		t.Fatalf("error=%v delete calls=%d; want retry commit error after one failed deletion", err, deleteCalls)
	}
	if baseTx.committed || baseTx.rollbacks != 1 || baseTx.unexpectedSQL != "" || baseTx.unexpectedExec != "" || len(baseTx.execs) != 1 || baseTx.execs[0] != "retry" {
		t.Fatalf("committed=%t rollbacks=%d execs=%v unexpected query=%q exec=%q", baseTx.committed, baseTx.rollbacks, baseTx.execs, baseTx.unexpectedSQL, baseTx.unexpectedExec)
	}
	if len(baseTx.execArgs) != 1 || len(baseTx.execArgs[0]) != 5 {
		t.Fatalf("retry args=%v; want one retry update with five arguments", baseTx.execArgs)
	}
	args := baseTx.execArgs[0]
	if args[0] != claim.JobID || args[1] != claim.FileID || args[2] != (37*time.Second).String() || args[3] != retentionStoreError || args[4] != claim.LeaseUntil {
		t.Fatalf("retry args=%#v; want IDs, bounded delay, allowlisted code and exact lease", args)
	}
	for _, arg := range args {
		if value, ok := arg.(string); ok && strings.Contains(value, rawStoreError) {
			t.Fatal("raw storage failure was included in retry arguments")
		}
	}
	wantTrace := []string{"delete", "retry", "commit_failed"}
	if len(baseTx.trace) != len(wantTrace) {
		t.Fatalf("trace=%v, want %v", baseTx.trace, wantTrace)
	}
	for i := range wantTrace {
		if baseTx.trace[i] != wantTrace[i] {
			t.Fatalf("trace=%v, want %v", baseTx.trace, wantTrace)
		}
	}
}
