package documentverificationpg

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
	"github.com/jackc/pgx/v5"
)

const (
	reviewerListAdapterActor   = "11111111-1111-4111-8111-111111111111"
	reviewerListAdapterOwner   = "22222222-2222-4222-8222-222222222222"
	reviewerListAdapterCaseID  = "33333333-3333-4333-8333-333333333333"
	reviewerListAdapterDeclID  = "44444444-4444-4444-8444-444444444444"
	reviewerListAdapterSpaceID = "66666666-6666-4666-8666-666666666666"
)

type reviewerListAdapterBeginner struct{ tx *reviewerListAdapterTx }

func (b reviewerListAdapterBeginner) BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error) {
	return b.tx, nil
}

type reviewerListAdapterRows struct {
	*reviewerAuditFakeRows
	err        error
	scanErr    error
	closeCalls int
}

func (rows *reviewerListAdapterRows) Scan(dest ...any) error {
	if rows.scanErr != nil {
		return rows.scanErr
	}
	return rows.reviewerAuditFakeRows.Scan(dest...)
}

func (rows *reviewerListAdapterRows) Err() error { return rows.err }

func (rows *reviewerListAdapterRows) Close() {
	rows.closeCalls++
	rows.reviewerAuditFakeRows.Close()
}

type reviewerListAdapterTx struct {
	*reviewerAuditFakeTx
	grants                  []dv.ReviewerGrant
	candidateListErr        error
	candidateRowsErr        error
	candidateRowsScanErr    error
	grantQueryErr           error
	candidateListArgs       []any
	candidateReadArgs       []any
	grantQueryArgs          []any
	candidateRows           *reviewerListAdapterRows
	candidateListCalls      int
	candidateReadCalls      int
	grantQueryCalls         int
	currentDeclarationCalls int
	fileQueryCalls          int
	rollbackCalls           int
	candidateSnapshot       []any
	spaceOwner              bool
	spaceOwnerErr           error
	spaceLockCalls          int
	spaceLockArgs           []any
}

func (tx *reviewerListAdapterTx) Rollback(ctx context.Context) error {
	tx.rollbackCalls++
	return tx.reviewerAuditFakeTx.Rollback(ctx)
}

func (tx *reviewerListAdapterTx) QueryRow(ctx context.Context, query string, args ...any) pgx.Row {
	switch {
	case strings.Contains(query, "document_verification_lock_active_actor"):
		return commitSerializationFailureRow{values: []any{true}}
	case strings.Contains(query, "document_verification_lock_space_owner"):
		tx.spaceLockCalls++
		tx.spaceLockArgs = append([]any(nil), args...)
		if tx.spaceOwnerErr != nil {
			return commitSerializationFailureRow{err: tx.spaceOwnerErr}
		}
		return commitSerializationFailureRow{values: []any{tx.spaceOwner}}
	case strings.Contains(query, identityRevisionCurrentQueryShape):
		tx.currentDeclarationCalls++
		if !reflect.DeepEqual(args, []any{reviewerListAdapterOwner, reviewerListAdapterDeclID}) {
			return commitSerializationFailureRow{err: errors.New("review declaration freshness query used unexpected subject or revision")}
		}
		return commitSerializationFailureRow{values: []any{true}}
	case strings.Contains(query, "SELECT "+reviewerSnapshotColumns):
		tx.candidateReadCalls++
		tx.candidateReadArgs = append([]any(nil), args...)
		if !reflect.DeepEqual(args, []any{reviewerListAdapterCaseID, reviewerListAdapterActor}) {
			return commitSerializationFailureRow{err: errors.New("reviewer list reread used unexpected case or actor")}
		}
		return commitSerializationFailureRow{values: tx.snapshotValues()}
	default:
		return commitSerializationFailureRow{err: errors.New("unexpected reviewer list QueryRow: " + query)}
	}
}

func (tx *reviewerListAdapterTx) Query(_ context.Context, query string, args ...any) (pgx.Rows, error) {
	switch {
	case strings.Contains(query, "FROM public.document_verification_case c"):
		tx.candidateListCalls++
		tx.candidateListArgs = append([]any(nil), args...)
		if !reflect.DeepEqual(args, []any{reviewerListAdapterActor}) {
			return nil, errors.New("reviewer candidate query used unexpected actor")
		}
		if tx.candidateListErr != nil {
			return nil, tx.candidateListErr
		}
		tx.candidateRows = &reviewerListAdapterRows{
			reviewerAuditFakeRows: &reviewerAuditFakeRows{rows: [][]any{tx.snapshotValues()}},
			err:                   tx.candidateRowsErr,
			scanErr:               tx.candidateRowsScanErr,
		}
		return tx.candidateRows, nil
	case strings.Contains(query, "FROM public.document_verification_reviewer_grant"):
		tx.grantQueryCalls++
		tx.grantQueryArgs = append([]any(nil), args...)
		if !reflect.DeepEqual(args, []any{reviewerListAdapterActor}) {
			return nil, errors.New("reviewer grant reread used unexpected actor")
		}
		if tx.grantQueryErr != nil {
			return nil, tx.grantQueryErr
		}
		rows := make([][]any, 0, len(tx.grants))
		for _, grant := range tx.grants {
			rows = append(rows, []any{grant.ReviewerID, string(grant.Capability), grant.ScopeID})
		}
		return &reviewerAuditFakeRows{rows: rows}, nil
	case strings.Contains(query, "FROM public.document_verification_file"):
		tx.fileQueryCalls++
		return &reviewerAuditFakeRows{}, nil
	default:
		return nil, errors.New("unexpected reviewer list Query: " + query)
	}
}

func (tx *reviewerListAdapterTx) snapshotValues() []any {
	if tx.candidateSnapshot != nil {
		return tx.candidateSnapshot
	}
	return reviewerListAdapterSnapshotValues()
}

func reviewerListAdapterSnapshotValues() []any {
	return []any{
		reviewerListAdapterCaseID, "identity", reviewerListAdapterOwner, nil,
		"en_revision", int64(7), "", nil, nil, nil, nil, nil,
		reviewerListAdapterDeclID, "new", dv.IdentityCaptureConsentVersion,
		time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC), "",
	}
}

func reviewerListAdapterService(t *testing.T, grants []dv.ReviewerGrant) (*Repository, *reviewerListAdapterTx) {
	t.Helper()
	tx := &reviewerListAdapterTx{
		reviewerAuditFakeTx: &reviewerAuditFakeTx{
			actor: reviewerListAdapterActor, subject: reviewerListAdapterOwner,
			caseID: reviewerListAdapterCaseID, declarationID: reviewerListAdapterDeclID,
		},
		grants: grants,
	}
	return newWithTransactionBeginner(reviewerListAdapterBeginner{tx: tx}), tx
}

func TestListReviewerCasesRevalidatesExactGrantBeforeLoadingFiles(t *testing.T) {
	tests := []struct {
		name      string
		grant     dv.ReviewerGrant
		wantCases int
		wantFiles int
	}{
		{
			name: "current exact identity grant",
			grant: dv.ReviewerGrant{
				ReviewerID: reviewerListAdapterActor,
				Capability: dv.CapabilityIdentity,
				ScopeID:    reviewerListAdapterOwner,
			},
			wantCases: 1,
			wantFiles: 1,
		},
		{
			name: "reloaded grant with different subject scope denies candidate",
			grant: dv.ReviewerGrant{
				ReviewerID: reviewerListAdapterActor,
				Capability: dv.CapabilityIdentity,
				ScopeID:    "55555555-5555-4555-8555-555555555555",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repository, tx := reviewerListAdapterService(t, []dv.ReviewerGrant{tc.grant})
			cases, err := repository.ListReviewerCases(context.Background(), reviewerListAdapterActor)
			if err != nil {
				t.Fatal(err)
			}
			if len(cases) != tc.wantCases || tx.fileQueryCalls != tc.wantFiles {
				t.Fatalf("cases=%d file reads=%d, want %d/%d", len(cases), tx.fileQueryCalls, tc.wantCases, tc.wantFiles)
			}
			wantCurrentDeclarationCalls := 0
			if tc.wantCases == 1 {
				wantCurrentDeclarationCalls = 1
			}
			if tx.candidateListCalls != 1 || tx.candidateReadCalls != 1 || tx.grantQueryCalls != 1 || tx.currentDeclarationCalls != wantCurrentDeclarationCalls || !tx.reviewerAuditFakeTx.committed {
				t.Fatalf("candidate-list/read/grant/current-declaration calls=%d/%d/%d/%d committed=%v", tx.candidateListCalls, tx.candidateReadCalls, tx.grantQueryCalls, tx.currentDeclarationCalls, tx.reviewerAuditFakeTx.committed)
			}
			if !reflect.DeepEqual(tx.candidateReadArgs, []any{reviewerListAdapterCaseID, reviewerListAdapterActor}) ||
				!reflect.DeepEqual(tx.grantQueryArgs, []any{reviewerListAdapterActor}) {
				t.Fatalf("candidate read args=%v grant args=%v", tx.candidateReadArgs, tx.grantQueryArgs)
			}
			if tc.wantCases == 1 && (cases[0].ID != reviewerListAdapterCaseID || cases[0].SubjectID != reviewerListAdapterOwner || cases[0].Files == nil) {
				t.Fatalf("returned reviewer case=%+v", cases[0])
			}
		})
	}
}

func TestListReviewerCasesRollsBackWhenGrantReadFails(t *testing.T) {
	wantErr := errors.New("synthetic grant query failure")
	matchingGrant := dv.ReviewerGrant{
		ReviewerID: reviewerListAdapterActor,
		Capability: dv.CapabilityIdentity,
		ScopeID:    reviewerListAdapterOwner,
	}
	repository, tx := reviewerListAdapterService(t, []dv.ReviewerGrant{matchingGrant})
	tx.grantQueryErr = wantErr

	cases, err := repository.ListReviewerCases(context.Background(), reviewerListAdapterActor)
	if !errors.Is(err, wantErr) || cases != nil {
		t.Fatalf("cases=%v err=%v, want nil cases and original grant-read error", cases, err)
	}
	if tx.rollbackCalls != 1 || tx.committed || tx.fileQueryCalls != 0 {
		t.Fatalf("rollback calls=%d committed=%v file reads=%d, want 1/false/0", tx.rollbackCalls, tx.committed, tx.fileQueryCalls)
	}
	if tx.candidateListCalls != 1 || tx.candidateReadCalls != 1 || tx.grantQueryCalls != 1 {
		t.Fatalf("candidate-list/read/grant calls=%d/%d/%d, want one each before failing", tx.candidateListCalls, tx.candidateReadCalls, tx.grantQueryCalls)
	}
}

func TestListReviewerCasesRollsBackWhenCandidateQueryFails(t *testing.T) {
	wantErr := errors.New("synthetic reviewer candidate query failure")
	repository, tx := reviewerListAdapterService(t, nil)
	tx.candidateListErr = wantErr

	cases, err := repository.ListReviewerCases(context.Background(), reviewerListAdapterActor)
	if !errors.Is(err, wantErr) || cases != nil {
		t.Fatalf("cases=%v err=%v, want nil cases and original candidate-query error", cases, err)
	}
	if tx.rollbackCalls != 1 || tx.committed || tx.candidateReadCalls != 0 || tx.grantQueryCalls != 0 || tx.fileQueryCalls != 0 {
		t.Fatalf("rollback=%d committed=%v candidate reads=%d grant reads=%d file reads=%d, want 1/false/0/0/0", tx.rollbackCalls, tx.committed, tx.candidateReadCalls, tx.grantQueryCalls, tx.fileQueryCalls)
	}
	if tx.candidateListCalls != 1 {
		t.Fatalf("candidate list calls=%d, want 1", tx.candidateListCalls)
	}
}

func TestListReviewerCasesDiscardsCandidatesWhenRowsIterationFails(t *testing.T) {
	wantErr := errors.New("synthetic reviewer candidate iteration failure")
	repository, tx := reviewerListAdapterService(t, nil)
	tx.candidateRowsErr = wantErr

	cases, err := repository.ListReviewerCases(context.Background(), reviewerListAdapterActor)
	if !errors.Is(err, wantErr) || cases != nil {
		t.Fatalf("cases=%v err=%v, want nil cases and original rows error", cases, err)
	}
	if tx.candidateRows == nil || tx.candidateRows.closeCalls != 1 {
		t.Fatalf("candidate rows=%v, want rows closed exactly once", tx.candidateRows)
	}
	if tx.rollbackCalls != 1 || tx.committed || tx.candidateReadCalls != 0 || tx.grantQueryCalls != 0 || tx.fileQueryCalls != 0 {
		t.Fatalf("rollback=%d committed=%v candidate reads=%d grant reads=%d file reads=%d, want 1/false/0/0/0", tx.rollbackCalls, tx.committed, tx.candidateReadCalls, tx.grantQueryCalls, tx.fileQueryCalls)
	}
	if tx.candidateListCalls != 1 || tx.candidateRows.index != 1 {
		t.Fatalf("candidate list calls=%d scanned rows=%d, want 1/1 before iteration error", tx.candidateListCalls, tx.candidateRows.index)
	}
}

func TestListReviewerCasesDiscardsCandidatesWhenRowScanFails(t *testing.T) {
	wantErr := errors.New("synthetic reviewer candidate scan failure")
	repository, tx := reviewerListAdapterService(t, nil)
	tx.candidateRowsScanErr = wantErr

	cases, err := repository.ListReviewerCases(context.Background(), reviewerListAdapterActor)
	if !errors.Is(err, wantErr) || cases != nil {
		t.Fatalf("cases=%v err=%v, want nil cases and original scan error", cases, err)
	}
	if tx.candidateRows == nil || tx.candidateRows.closeCalls != 1 || tx.candidateRows.index != 1 {
		t.Fatalf("candidate rows=%+v, want one scanned row and one close", tx.candidateRows)
	}
	if tx.rollbackCalls != 1 || tx.committed || tx.candidateReadCalls != 0 || tx.grantQueryCalls != 0 || tx.fileQueryCalls != 0 {
		t.Fatalf("rollback=%d committed=%v candidate reads=%d grant reads=%d file reads=%d, want 1/false/0/0/0", tx.rollbackCalls, tx.committed, tx.candidateReadCalls, tx.grantQueryCalls, tx.fileQueryCalls)
	}
	if tx.candidateListCalls != 1 {
		t.Fatalf("candidate list calls=%d, want 1", tx.candidateListCalls)
	}
}
