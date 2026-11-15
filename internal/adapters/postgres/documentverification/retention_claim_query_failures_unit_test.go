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

type retentionClaimReadFailureTx struct {
	pgx.Tx
	claim         RetentionClaim
	failFragment  string
	failErr       error
	queries       []string
	commitCalls   int
	rollbackCalls int
	unexpectedSQL string
}

func (tx *retentionClaimReadFailureTx) QueryRow(_ context.Context, query string, _ ...any) pgx.Row {
	tx.queries = append(tx.queries, query)
	if strings.Contains(query, tx.failFragment) {
		return commitSerializationFailureRow{err: tx.failErr}
	}
	switch {
	case strings.Contains(query, "FROM public.document_verification_job j"):
		return commitSerializationFailureRow{values: []any{tx.claim.JobID, tx.claim.FileID, tx.claim.CaseID, tx.claim.SubjectID, string(tx.claim.Purpose), tx.claim.SpaceID, tx.claim.Attempt}}
	case strings.Contains(query, "document_verification_lock_space_row"):
		return commitSerializationFailureRow{values: []any{true}}
	case strings.Contains(query, "SELECT id FROM public.document_verification_case"):
		return commitSerializationFailureRow{values: []any{tx.claim.CaseID}}
	case strings.Contains(query, "SELECT id FROM public.document_verification_file"):
		return commitSerializationFailureRow{values: []any{tx.claim.FileID}}
	case strings.Contains(query, "SELECT EXISTS(SELECT 1 FROM public.document_verification_job WHERE file_id=$1 AND kind='scan'"):
		return commitSerializationFailureRow{values: []any{false}}
	case strings.Contains(query, "UPDATE public.document_verification_job SET state='procesando'"):
		return commitSerializationFailureRow{values: []any{tx.claim.Attempt + 1, tx.claim.LeaseUntil}}
	default:
		tx.unexpectedSQL = query
		return commitSerializationFailureRow{err: errors.New("unexpected retention claim read")}
	}
}

func (tx *retentionClaimReadFailureTx) Commit(context.Context) error {
	tx.commitCalls++
	return nil
}

func (tx *retentionClaimReadFailureTx) Rollback(context.Context) error {
	tx.rollbackCalls++
	return nil
}

type retentionClaimReadFailureBeginner struct{ tx pgx.Tx }

func (b retentionClaimReadFailureBeginner) BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error) {
	return b.tx, nil
}

func TestClaimReadyRetentionJobFailsClosedOnQueryErrors(t *testing.T) {
	const (
		jobID     = "00000000-0000-4000-8000-000000000141"
		fileID    = "00000000-0000-4000-8000-000000000142"
		caseID    = "00000000-0000-4000-8000-000000000143"
		subjectID = "00000000-0000-4000-8000-000000000144"
		spaceID   = "00000000-0000-4000-8000-000000000145"
	)
	for _, tc := range []struct {
		name         string
		purpose      dv.Purpose
		spaceID      string
		failFragment string
		queryCount   int
	}{
		{name: "candidate query", purpose: dv.PurposeIdentity, failFragment: "FROM public.document_verification_job j", queryCount: 1},
		{name: "space lock query", purpose: dv.PurposeSpace, spaceID: spaceID, failFragment: "document_verification_lock_space_row", queryCount: 2},
		{name: "case lock query", purpose: dv.PurposeIdentity, failFragment: "SELECT id FROM public.document_verification_case", queryCount: 2},
		{name: "file lock query", purpose: dv.PurposeIdentity, failFragment: "SELECT id FROM public.document_verification_file", queryCount: 3},
		{name: "active scan query", purpose: dv.PurposeIdentity, failFragment: "SELECT EXISTS(SELECT 1 FROM public.document_verification_job WHERE file_id=$1 AND kind='scan'", queryCount: 4},
		{name: "conditional claim update", purpose: dv.PurposeIdentity, failFragment: "UPDATE public.document_verification_job SET state='procesando'", queryCount: 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			claim := RetentionClaim{
				JobID: jobID, FileID: fileID, CaseID: caseID, SubjectID: subjectID, SpaceID: tc.spaceID,
				Purpose: tc.purpose, Attempt: 2, LeaseUntil: time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC),
			}
			queryErr := errors.New("synthetic retention claim query failure: " + tc.name)
			tx := &retentionClaimReadFailureTx{claim: claim, failFragment: tc.failFragment, failErr: queryErr}
			repo := newWithTransactionBeginner(retentionClaimReadFailureBeginner{tx: tx})
			got, err := repo.ClaimReadyRetentionJob(context.Background(), time.Minute)
			if !errors.Is(err, queryErr) || got != nil {
				t.Fatalf("ClaimReadyRetentionJob()=(%+v,%v); want nil claim and injected query error", got, err)
			}
			if len(tx.queries) != tc.queryCount || tx.commitCalls != 0 || tx.rollbackCalls != 1 || tx.unexpectedSQL != "" {
				t.Fatalf("queries=%d want=%d commits=%d rollbacks=%d unexpected SQL=%q", len(tx.queries), tc.queryCount, tx.commitCalls, tx.rollbackCalls, tx.unexpectedSQL)
			}
			if !strings.Contains(tx.queries[len(tx.queries)-1], tc.failFragment) {
				t.Fatalf("last query=%q; want failure at fragment %q", tx.queries[len(tx.queries)-1], tc.failFragment)
			}
		})
	}
}
