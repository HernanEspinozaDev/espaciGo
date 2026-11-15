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

type retentionMissingRowTx struct {
	*retentionFencingUnitTx
	queryFragment string
}

func (tx *retentionMissingRowTx) QueryRow(ctx context.Context, query string, args ...any) pgx.Row {
	if strings.Contains(query, tx.queryFragment) {
		return retentionMissingRow{}
	}
	return tx.retentionFencingUnitTx.QueryRow(ctx, query, args...)
}

type retentionMissingRow struct{}

func (retentionMissingRow) Scan(...any) error { return pgx.ErrNoRows }

type retentionMissingRowBeginner struct{ tx pgx.Tx }

func (b retentionMissingRowBeginner) BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error) {
	return b.tx, nil
}

func TestProcessClaimedRetentionJobStopsWhenFencedRowIsMissing(t *testing.T) {
	const (
		jobID     = "00000000-0000-4000-8000-000000000061"
		fileID    = "00000000-0000-4000-8000-000000000062"
		caseID    = "00000000-0000-4000-8000-000000000063"
		subjectID = "00000000-0000-4000-8000-000000000064"
	)
	for _, tc := range []struct {
		name          string
		queryFragment string
	}{
		{
			name:          "case no longer exists",
			queryFragment: "SELECT purpose,subject_id::text,COALESCE(space_id::text,'') FROM public.document_verification_case WHERE id=$1",
		},
		{
			name:          "file is missing or no longer due for retention",
			queryFragment: "SELECT id::text FROM public.document_verification_file WHERE id=$1 AND case_id=$2 AND retention_at<=clock_timestamp() FOR UPDATE",
		},
		{
			name:          "retention job no longer matches claim",
			queryFragment: "SELECT state,COALESCE(lease_until=$2,false),COALESCE(lease_until>clock_timestamp(),false)",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			claim := RetentionClaim{
				JobID: jobID, FileID: fileID, CaseID: caseID, SubjectID: subjectID,
				Purpose: dv.PurposeIdentity, LeaseUntil: time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC),
			}
			baseTx := &retentionFencingUnitTx{claim: claim, leaseMatches: true, leaseLive: true}
			tx := &retentionMissingRowTx{retentionFencingUnitTx: baseTx, queryFragment: tc.queryFragment}
			repo := newWithTransactionBeginner(retentionMissingRowBeginner{tx: tx})
			deleteCalled := false
			err := repo.ProcessClaimedRetentionJob(context.Background(), claim, func(context.Context) error {
				deleteCalled = true
				return nil
			}, time.Minute)
			if !errors.Is(err, dv.ErrConflict) || deleteCalled {
				t.Fatalf("ProcessClaimedRetentionJob() error=%v deleteCalled=%t; want conflict before deletion", err, deleteCalled)
			}
			if baseTx.committed || baseTx.rollbacks != 1 || baseTx.unexpectedSQL != "" || baseTx.unexpectedExec != "" || len(baseTx.execs) != 0 {
				t.Fatalf("committed=%t rollbacks=%d unexpected query=%q exec=%q execs=%v", baseTx.committed, baseTx.rollbacks, baseTx.unexpectedSQL, baseTx.unexpectedExec, baseTx.execs)
			}
		})
	}
}
