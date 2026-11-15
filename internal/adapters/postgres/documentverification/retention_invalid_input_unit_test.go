package documentverificationpg

import (
	"context"
	"errors"
	"testing"
	"time"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
	"github.com/jackc/pgx/v5"
)

type retentionNoBeginTx struct{ beginCalls int }

func (tx *retentionNoBeginTx) BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error) {
	tx.beginCalls++
	return nil, errors.New("unexpected transaction begin for invalid retention input")
}

func TestProcessClaimedRetentionJobRejectsInvalidInputBeforeTransaction(t *testing.T) {
	validClaim := RetentionClaim{
		JobID: "00000000-0000-4000-8000-0000000000d1", FileID: "00000000-0000-4000-8000-0000000000d2",
		CaseID: "00000000-0000-4000-8000-0000000000d3", SubjectID: "00000000-0000-4000-8000-0000000000d4",
		Purpose: dv.PurposeIdentity, LeaseUntil: time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC),
	}
	for _, tc := range []struct {
		name       string
		claim      RetentionClaim
		deleteFn   func(context.Context) error
		retryDelay time.Duration
		nilRepo    bool
	}{
		{name: "nil delete callback", claim: validClaim, retryDelay: time.Minute},
		{name: "zero retry delay", claim: validClaim, deleteFn: func(context.Context) error { return nil }},
		{name: "negative retry delay", claim: validClaim, deleteFn: func(context.Context) error { return nil }, retryDelay: -time.Second},
		{name: "invalid job id", claim: RetentionClaim{FileID: validClaim.FileID, CaseID: validClaim.CaseID, SubjectID: validClaim.SubjectID, Purpose: validClaim.Purpose, LeaseUntil: validClaim.LeaseUntil}, deleteFn: func(context.Context) error { return nil }, retryDelay: time.Minute},
		{name: "zero lease", claim: RetentionClaim{JobID: validClaim.JobID, FileID: validClaim.FileID, CaseID: validClaim.CaseID, SubjectID: validClaim.SubjectID, Purpose: validClaim.Purpose}, deleteFn: func(context.Context) error { return nil }, retryDelay: time.Minute},
		{name: "nil repository", claim: validClaim, deleteFn: func(context.Context) error { return nil }, retryDelay: time.Minute, nilRepo: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			beginner := &retentionNoBeginTx{}
			repo := newWithTransactionBeginner(beginner)
			if tc.nilRepo {
				repo = nil
			}
			callbackCalls := 0
			deleteFn := tc.deleteFn
			if deleteFn != nil {
				deleteFn = func(ctx context.Context) error {
					callbackCalls++
					return tc.deleteFn(ctx)
				}
			}
			err := repo.ProcessClaimedRetentionJob(context.Background(), tc.claim, deleteFn, tc.retryDelay)
			if !errors.Is(err, dv.ErrInvalid) || beginner.beginCalls != 0 || callbackCalls != 0 {
				t.Fatalf("error=%v begin calls=%d callback calls=%d; want ErrInvalid before transaction or deletion", err, beginner.beginCalls, callbackCalls)
			}
		})
	}
}
