package documentverificationpg

import (
	"context"
	"errors"
	"testing"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
	"github.com/jackc/pgx/v5"
)

type reviewerListCommitFailureBeginner struct {
	tx pgx.Tx
}

func (b reviewerListCommitFailureBeginner) BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error) {
	return b.tx, nil
}

type reviewerListCommitFailureTx struct {
	*reviewerListAdapterTx
	commitErr   error
	commitCalls int
}

func (tx *reviewerListCommitFailureTx) Commit(context.Context) error {
	tx.commitCalls++
	return tx.commitErr
}

func TestListReviewerCasesReturnsNoCandidatesWhenCommitFails(t *testing.T) {
	wantErr := errors.New("synthetic reviewer list commit failure")
	grant := dv.ReviewerGrant{
		ReviewerID: reviewerListAdapterActor,
		Capability: dv.CapabilityIdentity,
		ScopeID:    reviewerListAdapterOwner,
	}
	repository, baseTx := reviewerListAdapterService(t, []dv.ReviewerGrant{grant})
	tx := &reviewerListCommitFailureTx{
		reviewerListAdapterTx: baseTx,
		commitErr:             wantErr,
	}
	repository.transactions = reviewerListCommitFailureBeginner{tx: tx}

	cases, err := repository.ListReviewerCases(context.Background(), reviewerListAdapterActor)

	if !errors.Is(err, wantErr) || cases != nil {
		t.Fatalf("cases=%v err=%v, want nil cases and the commit error", cases, err)
	}
	if tx.commitCalls != 1 || baseTx.rollbackCalls != 1 || baseTx.reviewerAuditFakeTx.committed {
		t.Fatalf("commit calls=%d rollback calls=%d committed=%v, want 1/1/false", tx.commitCalls, baseTx.rollbackCalls, baseTx.reviewerAuditFakeTx.committed)
	}
	if baseTx.candidateListCalls != 1 || baseTx.candidateReadCalls != 1 || baseTx.grantQueryCalls != 1 ||
		baseTx.currentDeclarationCalls != 1 || baseTx.fileQueryCalls != 1 {
		t.Fatalf("candidate-list/read/grant/current-declaration/file calls=%d/%d/%d/%d/%d, want one each before commit failure",
			baseTx.candidateListCalls, baseTx.candidateReadCalls, baseTx.grantQueryCalls,
			baseTx.currentDeclarationCalls, baseTx.fileQueryCalls)
	}
	if baseTx.candidateRows == nil || baseTx.candidateRows.closeCalls != 1 {
		t.Fatalf("candidate rows=%v, want the synthetic candidate rows closed once", baseTx.candidateRows)
	}
}
