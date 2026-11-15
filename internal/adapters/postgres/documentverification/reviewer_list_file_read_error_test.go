package documentverificationpg

import (
	"context"
	"errors"
	"strings"
	"testing"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
	"github.com/jackc/pgx/v5"
)

type reviewerListFileReadFailureBeginner struct {
	tx pgx.Tx
}

func (b reviewerListFileReadFailureBeginner) BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error) {
	return b.tx, nil
}

type reviewerListFileReadFailureTx struct {
	*reviewerListAdapterTx
	fileErr       error
	fileQueryArgs []any
}

func (tx *reviewerListFileReadFailureTx) Query(ctx context.Context, query string, args ...any) (pgx.Rows, error) {
	if strings.Contains(query, "FROM public.document_verification_file") {
		tx.fileQueryCalls++
		tx.fileQueryArgs = append([]any(nil), args...)
		if len(args) != 1 || args[0] != reviewerListAdapterCaseID {
			return nil, errors.New("reviewer file metadata query used unexpected case")
		}
		return nil, tx.fileErr
	}
	return tx.reviewerListAdapterTx.Query(ctx, query, args...)
}

func TestListReviewerCasesReturnsNoCandidatesWhenFileMetadataReadFails(t *testing.T) {
	wantErr := errors.New("synthetic reviewer file metadata query failure")
	grant := dv.ReviewerGrant{
		ReviewerID: reviewerListAdapterActor,
		Capability: dv.CapabilityIdentity,
		ScopeID:    reviewerListAdapterOwner,
	}
	repository, baseTx := reviewerListAdapterService(t, []dv.ReviewerGrant{grant})
	tx := &reviewerListFileReadFailureTx{
		reviewerListAdapterTx: baseTx,
		fileErr:               wantErr,
	}
	repository.transactions = reviewerListFileReadFailureBeginner{tx: tx}

	cases, err := repository.ListReviewerCases(context.Background(), reviewerListAdapterActor)

	if !errors.Is(err, wantErr) || cases != nil {
		t.Fatalf("cases=%v err=%v, want nil cases and the file metadata query error", cases, err)
	}
	if baseTx.candidateListCalls != 1 || baseTx.candidateReadCalls != 1 || baseTx.grantQueryCalls != 1 ||
		baseTx.currentDeclarationCalls != 1 || baseTx.fileQueryCalls != 1 {
		t.Fatalf("candidate-list/read/grant/current-declaration/file calls=%d/%d/%d/%d/%d, want one each before file query failure",
			baseTx.candidateListCalls, baseTx.candidateReadCalls, baseTx.grantQueryCalls,
			baseTx.currentDeclarationCalls, baseTx.fileQueryCalls)
	}
	if len(tx.fileQueryArgs) != 1 || tx.fileQueryArgs[0] != reviewerListAdapterCaseID {
		t.Fatalf("file query args=%v, want the exact synthetic case ID", tx.fileQueryArgs)
	}
	if baseTx.candidateRows == nil || baseTx.candidateRows.closeCalls != 1 {
		t.Fatalf("candidate rows=%v, want the synthetic candidate rows closed once", baseTx.candidateRows)
	}
	if baseTx.rollbackCalls != 1 || baseTx.reviewerAuditFakeTx.committed {
		t.Fatalf("rollback calls=%d committed=%v, want rollback callback once and no successful fake commit", baseTx.rollbackCalls, baseTx.reviewerAuditFakeTx.committed)
	}
}
