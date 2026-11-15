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

type retentionClaimSuccessTx struct {
	pgx.Tx
	candidate     RetentionClaim
	leaseUntil    time.Time
	queries       []string
	candidateArgs []any
	spaceLockArgs []any
	updateArgs    []any
	trace         []string
	commitCalls   int
	rollbacks     int
	unexpectedSQL string
}

func (tx *retentionClaimSuccessTx) QueryRow(_ context.Context, query string, args ...any) pgx.Row {
	tx.queries = append(tx.queries, query)
	switch {
	case strings.Contains(query, "FROM public.document_verification_job j"):
		tx.trace = append(tx.trace, "candidate")
		tx.candidateArgs = append([]any(nil), args...)
		return commitSerializationFailureRow{values: []any{tx.candidate.JobID, tx.candidate.FileID, tx.candidate.CaseID, tx.candidate.SubjectID, string(tx.candidate.Purpose), tx.candidate.SpaceID, tx.candidate.Attempt}}
	case strings.Contains(query, "document_verification_lock_space_row"):
		tx.trace = append(tx.trace, "space_lock")
		tx.spaceLockArgs = append([]any(nil), args...)
		return commitSerializationFailureRow{values: []any{true}}
	case strings.Contains(query, "SELECT id FROM public.document_verification_case"):
		tx.trace = append(tx.trace, "case_lock")
		return commitSerializationFailureRow{values: []any{tx.candidate.CaseID}}
	case strings.Contains(query, "SELECT id FROM public.document_verification_file"):
		tx.trace = append(tx.trace, "file_lock")
		return commitSerializationFailureRow{values: []any{tx.candidate.FileID}}
	case strings.Contains(query, "SELECT EXISTS(SELECT 1 FROM public.document_verification_job WHERE file_id=$1 AND kind='scan'"):
		tx.trace = append(tx.trace, "scan_recheck")
		return commitSerializationFailureRow{values: []any{false}}
	case strings.Contains(query, "UPDATE public.document_verification_job SET state='procesando'"):
		tx.trace = append(tx.trace, "claim_update")
		tx.updateArgs = append([]any(nil), args...)
		return commitSerializationFailureRow{values: []any{tx.candidate.Attempt + 1, tx.leaseUntil}}
	default:
		tx.unexpectedSQL = query
		return commitSerializationFailureRow{err: errors.New("unexpected retention claim query")}
	}
}

func (tx *retentionClaimSuccessTx) Commit(context.Context) error {
	tx.commitCalls++
	tx.trace = append(tx.trace, "commit")
	return nil
}

func (tx *retentionClaimSuccessTx) Rollback(context.Context) error {
	tx.rollbacks++
	tx.trace = append(tx.trace, "rollback")
	return nil
}

type retentionClaimSuccessBeginner struct {
	tx      pgx.Tx
	calls   int
	options pgx.TxOptions
}

func (b *retentionClaimSuccessBeginner) BeginTx(_ context.Context, options pgx.TxOptions) (pgx.Tx, error) {
	b.calls++
	b.options = options
	return b.tx, nil
}

func TestClaimReadyRetentionJobReturnsClaimAfterCommit(t *testing.T) {
	const (
		jobID     = "00000000-0000-4000-8000-000000000151"
		fileID    = "00000000-0000-4000-8000-000000000152"
		caseID    = "00000000-0000-4000-8000-000000000153"
		subjectID = "00000000-0000-4000-8000-000000000154"
	)
	leaseUntil := time.Date(2026, 11, 11, 12, 2, 0, 0, time.UTC)
	candidate := RetentionClaim{
		JobID: jobID, FileID: fileID, CaseID: caseID, SubjectID: subjectID,
		Purpose: dv.PurposeIdentity, Attempt: 2,
	}
	tx := &retentionClaimSuccessTx{candidate: candidate, leaseUntil: leaseUntil}
	beginner := &retentionClaimSuccessBeginner{tx: tx}
	repo := newWithTransactionBeginner(beginner)
	got, err := repo.ClaimReadyRetentionJob(context.Background(), 2*time.Minute)
	if err != nil {
		t.Fatalf("ClaimReadyRetentionJob() error=%v", err)
	}
	want := RetentionClaim{
		JobID: jobID, FileID: fileID, CaseID: caseID, SubjectID: subjectID,
		Purpose: dv.PurposeIdentity, Attempt: 3, LeaseUntil: leaseUntil,
	}
	if got == nil || *got != want {
		t.Fatalf("ClaimReadyRetentionJob() claim=%+v, want %+v", got, want)
	}
	if len(tx.queries) != 5 || !strings.Contains(tx.queries[0], "FROM public.document_verification_job j") || !strings.Contains(tx.queries[1], "SELECT id FROM public.document_verification_case") || !strings.Contains(tx.queries[2], "SELECT id FROM public.document_verification_file") || !strings.Contains(tx.queries[3], "SELECT EXISTS(SELECT 1 FROM public.document_verification_job WHERE file_id=$1 AND kind='scan'") || !strings.Contains(tx.queries[4], "UPDATE public.document_verification_job SET state='procesando'") {
		t.Fatalf("queries=%q; want candidate, case lock, file lock, scan recheck, claim update", tx.queries)
	}
	if len(tx.updateArgs) != 3 || tx.updateArgs[0] != jobID || tx.updateArgs[1] != (2*time.Minute).String() || tx.updateArgs[2] != nil {
		t.Fatalf("claim update args=%#v; want job ID, lease duration and nil ready-before filter", tx.updateArgs)
	}
	if len(tx.candidateArgs) != 2 || tx.candidateArgs[0] != nil || tx.candidateArgs[1] != nil {
		t.Fatalf("candidate query args=%#v; want nil job and ready-before filters", tx.candidateArgs)
	}
	wantTrace := []string{"candidate", "case_lock", "file_lock", "scan_recheck", "claim_update", "commit", "rollback"}
	if len(tx.trace) != len(wantTrace) {
		t.Fatalf("trace=%v; want %v", tx.trace, wantTrace)
	}
	for i := range wantTrace {
		if tx.trace[i] != wantTrace[i] {
			t.Fatalf("trace=%v; want %v", tx.trace, wantTrace)
		}
	}
	if tx.commitCalls != 1 || tx.rollbacks != 1 || tx.unexpectedSQL != "" {
		t.Fatalf("commit calls=%d rollbacks=%d unexpected SQL=%q", tx.commitCalls, tx.rollbacks, tx.unexpectedSQL)
	}
	if beginner.calls != 1 || beginner.options != (pgx.TxOptions{IsoLevel: pgx.ReadCommitted}) {
		t.Fatalf("BeginTx calls=%d options=%+v; want one ReadCommitted transaction", beginner.calls, beginner.options)
	}
}

func TestClaimReadyRetentionJobNormalizesCandidateCutoffToUTC(t *testing.T) {
	const (
		jobID     = "00000000-0000-4000-8000-000000000161"
		fileID    = "00000000-0000-4000-8000-000000000162"
		caseID    = "00000000-0000-4000-8000-000000000163"
		subjectID = "00000000-0000-4000-8000-000000000164"
	)
	zone := time.FixedZone("synthetic-offset", -7*60*60)
	readyBefore := time.Date(2026, 11, 11, 5, 30, 0, 0, zone)
	readyBeforeUTC := readyBefore.UTC()
	leaseUntil := time.Date(2026, 11, 11, 13, 32, 0, 0, time.UTC)
	candidate := RetentionClaim{
		JobID: jobID, FileID: fileID, CaseID: caseID, SubjectID: subjectID,
		Purpose: dv.PurposeIdentity, Attempt: 2,
	}
	tx := &retentionClaimSuccessTx{candidate: candidate, leaseUntil: leaseUntil}
	beginner := &retentionClaimSuccessBeginner{tx: tx}
	repo := newWithTransactionBeginner(beginner)
	got, err := repo.claimReadyRetentionJob(context.Background(), 2*time.Minute, jobID, readyBefore)
	if err != nil {
		t.Fatalf("claimReadyRetentionJob() error=%v", err)
	}
	if got == nil || got.JobID != jobID || got.Attempt != 3 || got.LeaseUntil != leaseUntil {
		t.Fatalf("claimReadyRetentionJob() claim=%+v; want claimed job, incremented attempt, and returned lease", got)
	}
	if len(tx.candidateArgs) != 2 || tx.candidateArgs[0] != jobID || tx.candidateArgs[1] != readyBeforeUTC || readyBeforeUTC.Location() != time.UTC {
		t.Fatalf("candidate query args=%#v; want exact job ID and UTC cutoff %v", tx.candidateArgs, readyBeforeUTC)
	}
	if len(tx.updateArgs) != 3 || tx.updateArgs[0] != jobID || tx.updateArgs[1] != (2*time.Minute).String() || tx.updateArgs[2] != readyBeforeUTC {
		t.Fatalf("claim update args=%#v; want exact job ID, lease duration and UTC cutoff", tx.updateArgs)
	}
	if beginner.calls != 1 || beginner.options != (pgx.TxOptions{IsoLevel: pgx.ReadCommitted}) || tx.commitCalls != 1 || tx.rollbacks != 1 || tx.unexpectedSQL != "" {
		t.Fatalf("begin calls=%d options=%+v commits=%d rollbacks=%d unexpected SQL=%q", beginner.calls, beginner.options, tx.commitCalls, tx.rollbacks, tx.unexpectedSQL)
	}
}

func TestClaimReadyRetentionJobLocksSpaceBeforeClaimForSpacePurpose(t *testing.T) {
	const (
		jobID     = "00000000-0000-4000-8000-000000000181"
		fileID    = "00000000-0000-4000-8000-000000000182"
		caseID    = "00000000-0000-4000-8000-000000000183"
		subjectID = "00000000-0000-4000-8000-000000000184"
		spaceID   = "00000000-0000-4000-8000-000000000185"
	)
	leaseUntil := time.Date(2026, 11, 11, 14, 2, 0, 0, time.UTC)
	candidate := RetentionClaim{
		JobID: jobID, FileID: fileID, CaseID: caseID, SubjectID: subjectID, SpaceID: spaceID,
		Purpose: dv.PurposeSpace, Attempt: 1,
	}
	tx := &retentionClaimSuccessTx{candidate: candidate, leaseUntil: leaseUntil}
	beginner := &retentionClaimSuccessBeginner{tx: tx}
	repo := newWithTransactionBeginner(beginner)
	got, err := repo.ClaimReadyRetentionJob(context.Background(), time.Minute)
	if err != nil {
		t.Fatalf("ClaimReadyRetentionJob() error=%v", err)
	}
	want := RetentionClaim{
		JobID: jobID, FileID: fileID, CaseID: caseID, SubjectID: subjectID, SpaceID: spaceID,
		Purpose: dv.PurposeSpace, Attempt: 2, LeaseUntil: leaseUntil,
	}
	if got == nil || *got != want {
		t.Fatalf("ClaimReadyRetentionJob() claim=%+v, want %+v", got, want)
	}
	if len(tx.spaceLockArgs) != 1 || tx.spaceLockArgs[0] != spaceID {
		t.Fatalf("space lock args=%#v; want exact space ID", tx.spaceLockArgs)
	}
	wantFragments := []string{
		"FROM public.document_verification_job j",
		"document_verification_lock_space_row",
		"SELECT id FROM public.document_verification_case",
		"SELECT id FROM public.document_verification_file",
		"SELECT EXISTS(SELECT 1 FROM public.document_verification_job WHERE file_id=$1 AND kind='scan'",
		"UPDATE public.document_verification_job SET state='procesando'",
	}
	if len(tx.queries) != len(wantFragments) {
		t.Fatalf("query count=%d; want %d in order", len(tx.queries), len(wantFragments))
	}
	for i, fragment := range wantFragments {
		if !strings.Contains(tx.queries[i], fragment) {
			t.Fatalf("query[%d]=%q; want fragment %q", i, tx.queries[i], fragment)
		}
	}
	wantTrace := []string{"candidate", "space_lock", "case_lock", "file_lock", "scan_recheck", "claim_update", "commit", "rollback"}
	if len(tx.trace) != len(wantTrace) {
		t.Fatalf("trace=%v; want %v", tx.trace, wantTrace)
	}
	for i := range wantTrace {
		if tx.trace[i] != wantTrace[i] {
			t.Fatalf("trace=%v; want %v", tx.trace, wantTrace)
		}
	}
	if beginner.calls != 1 || beginner.options != (pgx.TxOptions{IsoLevel: pgx.ReadCommitted}) || tx.commitCalls != 1 || tx.rollbacks != 1 || tx.unexpectedSQL != "" {
		t.Fatalf("begin calls=%d options=%+v commits=%d rollbacks=%d unexpected SQL=%q", beginner.calls, beginner.options, tx.commitCalls, tx.rollbacks, tx.unexpectedSQL)
	}
}
