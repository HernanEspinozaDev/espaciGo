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

type retentionQueryFailureRow struct{ err error }

func (row retentionQueryFailureRow) Scan(...any) error { return row.err }

type retentionQueryFailureTx struct {
	*retentionFencingUnitTx
	queryFragment string
	err           error
	failures      int
}

func (tx *retentionQueryFailureTx) QueryRow(ctx context.Context, query string, args ...any) pgx.Row {
	if strings.Contains(query, tx.queryFragment) {
		tx.failures++
		tx.trace = append(tx.trace, "query_failed")
		return retentionQueryFailureRow{err: tx.err}
	}
	return tx.retentionFencingUnitTx.QueryRow(ctx, query, args...)
}

type retentionQueryFailureBeginner struct{ tx pgx.Tx }

func (b retentionQueryFailureBeginner) BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error) {
	return b.tx, nil
}

func TestProcessClaimedRetentionJobStopsOnQueryFailureBeforeDeletion(t *testing.T) {
	const (
		jobID     = "00000000-0000-4000-8000-0000000000e1"
		fileID    = "00000000-0000-4000-8000-0000000000e2"
		caseID    = "00000000-0000-4000-8000-0000000000e3"
		subjectID = "00000000-0000-4000-8000-0000000000e4"
		spaceID   = "00000000-0000-4000-8000-0000000000e5"
	)
	for _, tc := range []struct {
		name          string
		purpose       dv.Purpose
		spaceID       string
		queryFragment string
	}{
		{name: "case lookup", purpose: dv.PurposeIdentity, queryFragment: "SELECT purpose,subject_id::text,COALESCE(space_id::text,'') FROM public.document_verification_case WHERE id=$1"},
		{name: "space lock", purpose: dv.PurposeSpace, spaceID: spaceID, queryFragment: "document_verification_lock_space_row"},
		{name: "locked case lookup", purpose: dv.PurposeIdentity, queryFragment: "FROM public.document_verification_case WHERE id=$1 FOR UPDATE"},
		{name: "eligible file lookup", purpose: dv.PurposeIdentity, queryFragment: "SELECT id::text FROM public.document_verification_file WHERE id=$1 AND case_id=$2 AND retention_at<=clock_timestamp() FOR UPDATE"},
		{name: "lease lookup", purpose: dv.PurposeIdentity, queryFragment: "SELECT state,COALESCE(lease_until=$2,false),COALESCE(lease_until>clock_timestamp(),false)"},
		{name: "active scan lookup", purpose: dv.PurposeIdentity, queryFragment: "SELECT EXISTS(SELECT 1 FROM public.document_verification_job WHERE file_id=$1 AND kind='scan'"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			claim := RetentionClaim{
				JobID: jobID, FileID: fileID, CaseID: caseID, SubjectID: subjectID,
				Purpose: tc.purpose, SpaceID: tc.spaceID, LeaseUntil: time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC),
			}
			queryErr := errors.New("synthetic retention read failure: " + tc.name)
			baseTx := &retentionFencingUnitTx{claim: claim, leaseMatches: true, leaseLive: true}
			tx := &retentionQueryFailureTx{retentionFencingUnitTx: baseTx, queryFragment: tc.queryFragment, err: queryErr}
			repo := newWithTransactionBeginner(retentionQueryFailureBeginner{tx: tx})
			deleteCalled := false
			err := repo.ProcessClaimedRetentionJob(context.Background(), claim, func(context.Context) error {
				deleteCalled = true
				return nil
			}, time.Minute)
			if !errors.Is(err, queryErr) || deleteCalled {
				t.Fatalf("ProcessClaimedRetentionJob() error=%v deleteCalled=%t; want query error before deletion", err, deleteCalled)
			}
			if tx.failures != 1 || baseTx.committed || baseTx.rollbacks != 1 || len(baseTx.execs) != 0 || baseTx.unexpectedSQL != "" || baseTx.unexpectedExec != "" || len(baseTx.trace) != 1 || baseTx.trace[0] != "query_failed" {
				t.Fatalf("failures=%d committed=%t rollbacks=%d execs=%v trace=%v unexpected query=%q exec=%q", tx.failures, baseTx.committed, baseTx.rollbacks, baseTx.execs, baseTx.trace, baseTx.unexpectedSQL, baseTx.unexpectedExec)
			}
		})
	}
}
