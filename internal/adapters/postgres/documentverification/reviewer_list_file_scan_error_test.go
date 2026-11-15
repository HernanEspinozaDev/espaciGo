package documentverificationpg

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
	"github.com/jackc/pgx/v5"
)

type reviewerListFileScanFailureBeginner struct{ tx pgx.Tx }

func (b reviewerListFileScanFailureBeginner) BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error) {
	return b.tx, nil
}

type reviewerListFileScanFailureRows struct {
	*reviewerListAdapterRows
	scanCalls int
}

func (rows *reviewerListFileScanFailureRows) Scan(dest ...any) error {
	rows.scanCalls++
	return rows.reviewerListAdapterRows.Scan(dest...)
}

type reviewerListFileScanFailureTx struct {
	*reviewerListAdapterTx
	fileRows      pgx.Rows
	fileQueryArgs []any
}

func (tx *reviewerListFileScanFailureTx) Query(ctx context.Context, query string, args ...any) (pgx.Rows, error) {
	if strings.Contains(query, "FROM public.document_verification_file") {
		tx.fileQueryCalls++
		tx.fileQueryArgs = append([]any(nil), args...)
		if !reflect.DeepEqual(args, []any{reviewerListAdapterCaseID}) {
			return nil, errors.New("reviewer file metadata query used unexpected case")
		}
		return tx.fileRows, nil
	}
	return tx.reviewerListAdapterTx.Query(ctx, query, args...)
}

func TestListReviewerCasesReturnsNoCandidatesWhenFileMetadataScanFails(t *testing.T) {
	grant := dv.ReviewerGrant{
		ReviewerID: reviewerListAdapterActor,
		Capability: dv.CapabilityIdentity,
		ScopeID:    reviewerListAdapterOwner,
	}
	wantErr := errors.New("synthetic reviewer file metadata scan failure")
	digest := bytes.Repeat([]byte{0x31}, 32)

	for _, tc := range []struct {
		name    string
		scanErr error
	}{
		{name: "valid metadata control"},
		{name: "scan failure", scanErr: wantErr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repository, baseTx := reviewerListAdapterService(t, []dv.ReviewerGrant{grant})
			baseRows := &reviewerListAdapterRows{
				reviewerAuditFakeRows: &reviewerAuditFakeRows{rows: [][]any{{
					"55555555-5555-4555-8555-555555555555", string(dv.FileClean), int64(128), "image/png", int64(1), digest, nil,
				}}},
				scanErr: tc.scanErr,
			}
			fileRows := &reviewerListFileScanFailureRows{reviewerListAdapterRows: baseRows}
			tx := &reviewerListFileScanFailureTx{reviewerListAdapterTx: baseTx, fileRows: fileRows}
			repository.transactions = reviewerListFileScanFailureBeginner{tx: tx}

			cases, err := repository.ListReviewerCases(context.Background(), reviewerListAdapterActor)
			if tc.scanErr != nil {
				if cases != nil || !errors.Is(err, wantErr) {
					t.Fatalf("cases=%v err=%v, want nil candidates and scan error", cases, err)
				}
				if baseTx.reviewerAuditFakeTx.committed || baseTx.rollbackCalls != 1 {
					t.Fatalf("committed=%t rollback calls=%d, want no commit and one fake rollback callback", baseTx.reviewerAuditFakeTx.committed, baseTx.rollbackCalls)
				}
			} else {
				if err != nil || len(cases) != 1 || len(cases[0].Files) != 1 ||
					!bytes.Equal(cases[0].Files[0].ContentSHA256[:], digest) || !baseTx.reviewerAuditFakeTx.committed {
					t.Fatalf("cases=%v err=%v committed=%t, want one valid metadata row and fake commit", cases, err, baseTx.reviewerAuditFakeTx.committed)
				}
			}

			if fileRows.scanCalls != 1 || fileRows.index != 1 || fileRows.closeCalls != 1 ||
				baseTx.candidateRows == nil || baseTx.candidateRows.closeCalls != 1 {
				t.Fatalf("metadata scans=%d index=%d metadata closes=%d candidate rows=%v, want one scan and both row sets closed once",
					fileRows.scanCalls, fileRows.index, fileRows.closeCalls, baseTx.candidateRows)
			}
			if baseTx.candidateListCalls != 1 || baseTx.candidateReadCalls != 1 || baseTx.grantQueryCalls != 1 ||
				baseTx.currentDeclarationCalls != 1 || baseTx.fileQueryCalls != 1 ||
				!reflect.DeepEqual(tx.fileQueryArgs, []any{reviewerListAdapterCaseID}) {
				t.Fatalf("candidate/read/grant/declaration/file calls=%d/%d/%d/%d/%d file args=%v",
					baseTx.candidateListCalls, baseTx.candidateReadCalls, baseTx.grantQueryCalls,
					baseTx.currentDeclarationCalls, baseTx.fileQueryCalls, tx.fileQueryArgs)
			}
		})
	}
}
