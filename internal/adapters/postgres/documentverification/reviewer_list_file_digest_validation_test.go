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

type reviewerListFileMetadataRowsBeginner struct{ tx pgx.Tx }

func (b reviewerListFileMetadataRowsBeginner) BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error) {
	return b.tx, nil
}

type reviewerListFileMetadataRowsTx struct {
	*reviewerListAdapterTx
	fileRows      *reviewerListAdapterRows
	fileQueryArgs []any
}

func (tx *reviewerListFileMetadataRowsTx) Query(ctx context.Context, query string, args ...any) (pgx.Rows, error) {
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

func TestListReviewerCasesRejectsMalformedFileDigestWithoutReturningPartialCandidate(t *testing.T) {
	grant := dv.ReviewerGrant{
		ReviewerID: reviewerListAdapterActor,
		Capability: dv.CapabilityIdentity,
		ScopeID:    reviewerListAdapterOwner,
	}
	validDigestA := bytes.Repeat([]byte{0x31}, 32)
	validDigestB := bytes.Repeat([]byte{0x42}, 32)
	invalidDigest := bytes.Repeat([]byte{0x53}, 31)

	for _, tc := range []struct {
		name          string
		digests       [][]byte
		wantErr       bool
		wantFileCount int
	}{
		{name: "valid file metadata control", digests: [][]byte{validDigestA, validDigestB}, wantFileCount: 2},
		{name: "later malformed digest rejects whole candidate", digests: [][]byte{validDigestA, invalidDigest}, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repository, baseTx := reviewerListAdapterService(t, []dv.ReviewerGrant{grant})
			fileRows := &reviewerListAdapterRows{reviewerAuditFakeRows: &reviewerAuditFakeRows{rows: [][]any{
				{"55555555-5555-4555-8555-555555555555", string(dv.FileClean), int64(128), "image/png", int64(1), tc.digests[0], nil},
				{"66666666-6666-4666-8666-666666666666", string(dv.FileClean), int64(96), "image/jpeg", int64(0), tc.digests[1], nil},
			}}}
			tx := &reviewerListFileMetadataRowsTx{reviewerListAdapterTx: baseTx, fileRows: fileRows}
			repository.transactions = reviewerListFileMetadataRowsBeginner{tx: tx}

			cases, err := repository.ListReviewerCases(context.Background(), reviewerListAdapterActor)

			if tc.wantErr {
				if cases != nil || err == nil || err.Error() != "persisted document content hash has invalid length" {
					t.Fatalf("cases=%v err=%v, want nil cases and invalid persisted digest error", cases, err)
				}
				if baseTx.reviewerAuditFakeTx.committed || baseTx.rollbackCalls != 1 {
					t.Fatalf("committed=%t rollback calls=%d, want no commit and one fake rollback callback", baseTx.reviewerAuditFakeTx.committed, baseTx.rollbackCalls)
				}
			} else {
				if err != nil || len(cases) != 1 || len(cases[0].Files) != tc.wantFileCount {
					t.Fatalf("cases=%v err=%v, want one candidate with %d files", cases, err, tc.wantFileCount)
				}
				if !bytes.Equal(cases[0].Files[0].ContentSHA256[:], validDigestA) || !bytes.Equal(cases[0].Files[1].ContentSHA256[:], validDigestB) {
					t.Fatalf("returned file digests=%x/%x, want the exact synthetic 32-byte digests", cases[0].Files[0].ContentSHA256, cases[0].Files[1].ContentSHA256)
				}
				if !baseTx.reviewerAuditFakeTx.committed {
					t.Fatal("valid metadata control did not reach fake commit")
				}
			}

			if baseTx.candidateListCalls != 1 || baseTx.candidateReadCalls != 1 || baseTx.grantQueryCalls != 1 ||
				baseTx.currentDeclarationCalls != 1 || baseTx.fileQueryCalls != 1 ||
				!reflect.DeepEqual(tx.fileQueryArgs, []any{reviewerListAdapterCaseID}) {
				t.Fatalf("candidate/read/grant/declaration/file calls=%d/%d/%d/%d/%d file args=%v", baseTx.candidateListCalls, baseTx.candidateReadCalls, baseTx.grantQueryCalls, baseTx.currentDeclarationCalls, baseTx.fileQueryCalls, tx.fileQueryArgs)
			}
			if baseTx.candidateRows == nil || baseTx.candidateRows.closeCalls != 1 || fileRows.closeCalls != 1 {
				t.Fatalf("candidate rows=%v file rows close calls=%d; want both row sets closed once", baseTx.candidateRows, fileRows.closeCalls)
			}
		})
	}
}

func TestListReviewerCasesDiscardsCandidateWhenFileMetadataRowsIterationFails(t *testing.T) {
	grant := dv.ReviewerGrant{
		ReviewerID: reviewerListAdapterActor,
		Capability: dv.CapabilityIdentity,
		ScopeID:    reviewerListAdapterOwner,
	}
	wantErr := errors.New("synthetic file metadata rows iteration failure")
	repository, baseTx := reviewerListAdapterService(t, []dv.ReviewerGrant{grant})
	fileRows := &reviewerListAdapterRows{
		reviewerAuditFakeRows: &reviewerAuditFakeRows{rows: [][]any{{
			"55555555-5555-4555-8555-555555555555", string(dv.FileClean), int64(128), "image/png", int64(1), bytes.Repeat([]byte{0x31}, 32), nil,
		}}},
		err: wantErr,
	}
	tx := &reviewerListFileMetadataRowsTx{reviewerListAdapterTx: baseTx, fileRows: fileRows}
	repository.transactions = reviewerListFileMetadataRowsBeginner{tx: tx}

	cases, err := repository.ListReviewerCases(context.Background(), reviewerListAdapterActor)
	if cases != nil || !errors.Is(err, wantErr) {
		t.Fatalf("cases=%v err=%v, want nil candidates and the file rows iteration error", cases, err)
	}
	if fileRows.index != 1 || fileRows.current == nil || fileRows.closeCalls != 1 {
		t.Fatalf("file row index=%d current=%v close calls=%d, want one scanned row and one close", fileRows.index, fileRows.current, fileRows.closeCalls)
	}
	if baseTx.candidateRows == nil || baseTx.candidateRows.closeCalls != 1 ||
		baseTx.candidateListCalls != 1 || baseTx.candidateReadCalls != 1 || baseTx.grantQueryCalls != 1 ||
		baseTx.currentDeclarationCalls != 1 || baseTx.fileQueryCalls != 1 ||
		!reflect.DeepEqual(tx.fileQueryArgs, []any{reviewerListAdapterCaseID}) {
		t.Fatalf("candidate rows=%v; candidate/read/grant/declaration/file calls=%d/%d/%d/%d/%d file args=%v",
			baseTx.candidateRows, baseTx.candidateListCalls, baseTx.candidateReadCalls, baseTx.grantQueryCalls,
			baseTx.currentDeclarationCalls, baseTx.fileQueryCalls, tx.fileQueryArgs)
	}
	if baseTx.reviewerAuditFakeTx.committed || baseTx.rollbackCalls != 1 {
		t.Fatalf("committed=%t rollback calls=%d, want no commit and one fake rollback callback", baseTx.reviewerAuditFakeTx.committed, baseTx.rollbackCalls)
	}
}
