package documentverificationpg

import (
	"context"
	"errors"
	"strings"
	"testing"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
	"github.com/jackc/pgx/v5"
)

type reviewerTransitionBeginner struct{ tx pgx.Tx }

func (b reviewerTransitionBeginner) BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error) {
	return b.tx, nil
}

type reviewerTransitionScopeTx struct {
	commitSerializationFailureTx
	owner, reviewer   string
	ownerStillCurrent bool
	lockedOwner       string
}

func (tx *reviewerTransitionScopeTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	switch {
	case strings.Contains(sql, "SELECT space_id::text,subject_id::text FROM public.document_verification_case"):
		return commitSerializationFailureRow{values: []any{tx.spaceID, tx.owner}}
	case strings.Contains(sql, "SELECT space_id::text FROM public.document_verification_case"):
		return commitSerializationFailureRow{values: []any{tx.spaceID}}
	case strings.Contains(sql, "document_verification_lock_space_owner"):
		tx.lockedOwner = args[1].(string)
		return commitSerializationFailureRow{values: []any{tx.ownerStillCurrent && tx.lockedOwner == tx.owner}}
	case strings.Contains(sql, "SELECT id::text,purpose,subject_id::text,space_id::text,state,revision"):
		return commitSerializationFailureRow{values: []any{tx.caseID, "space", tx.owner, tx.spaceID, "enviada", int64(1), "", nil, nil, nil, nil, nil}}
	case strings.Contains(sql, "WHERE purpose='identity'"):
		return commitSerializationFailureRow{err: pgx.ErrNoRows}
	default:
		return tx.commitSerializationFailureTx.QueryRow(ctx, sql, args...)
	}
}
func TestReviewerSpaceTransitionLocksCurrentSubjectNotReviewerAsOwner(t *testing.T) {
	const owner = "00000000-0000-4000-8000-000000000041"
	const reviewer = "00000000-0000-4000-8000-000000000042"
	for _, current := range []bool{true, false} {
		tx := &reviewerTransitionScopeTx{commitSerializationFailureTx: commitSerializationFailureTx{caseID: "00000000-0000-4000-8000-000000000043", spaceID: "00000000-0000-4000-8000-000000000044", actor: reviewer}, owner: owner, reviewer: reviewer, ownerStillCurrent: current}
		repo := newWithTransactionBeginner(reviewerTransitionBeginner{tx})
		reached := false
		stop := errors.New("stop before persistence")
		err := repo.Transition(context.Background(), reviewer, dv.PurposeSpace, tx.caseID, 1, dv.TransitionAudit{}, func(x *dv.Snapshot, a dv.Authorization) error {
			reached = true
			if a.SpaceOwnerID != owner || a.ActorID != reviewer || a.SubjectID != owner {
				t.Fatal("incorrect current-owner/reviewer authority")
			}
			return stop
		})
		if current && (!reached || !errors.Is(err, stop)) {
			t.Fatalf("legitimate reviewer blocked before domain assignment check: %v", err)
		}
		if !current && (reached || !errors.Is(err, dv.ErrConflict)) {
			t.Fatalf("stale owner allowed: reached=%v error=%v", reached, err)
		}
		if tx.lockedOwner != owner {
			t.Fatal("space lock used reviewer instead of immutable case subject")
		}
	}
}
