package documentverificationpg

import (
	"context"
	"errors"
	"testing"
	"time"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
)

func TestClaimReadyRetentionJobRejectsInvalidInputBeforeTransaction(t *testing.T) {
	for _, tc := range []struct {
		name    string
		lease   time.Duration
		jobID   string
		nilRepo bool
		noTx    bool
		useOnly bool
	}{
		{name: "nil repository", lease: time.Minute, nilRepo: true},
		{name: "nil transaction beginner", lease: time.Minute, noTx: true},
		{name: "zero lease", lease: 0},
		{name: "negative lease", lease: -time.Second},
		{name: "invalid filtered job ID", lease: time.Minute, jobID: "not-a-uuid", useOnly: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			beginner := &retentionNoBeginTx{}
			repo := newWithTransactionBeginner(beginner)
			if tc.nilRepo {
				repo = nil
			} else if tc.noTx {
				repo = &Repository{}
			}
			var claim *RetentionClaim
			var err error
			if tc.useOnly {
				claim, err = repo.claimReadyRetentionJob(context.Background(), tc.lease, tc.jobID, time.Time{})
			} else {
				claim, err = repo.ClaimReadyRetentionJob(context.Background(), tc.lease)
			}
			if !errors.Is(err, dv.ErrInvalid) || claim != nil || beginner.beginCalls != 0 {
				t.Fatalf("claim=%+v error=%v BeginTx calls=%d; want ErrInvalid before transaction", claim, err, beginner.beginCalls)
			}
		})
	}
}
