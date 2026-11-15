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

const (
	staleReviewerReadActor       = "11111111-1111-4111-8111-111111111111"
	staleReviewerReadSubject     = "22222222-2222-4222-8222-222222222222"
	staleReviewerReadCase        = "33333333-3333-4333-8333-333333333333"
	staleReviewerReadDeclaration = "44444444-4444-4444-8444-444444444444"
	staleReviewerReadLatest      = "55555555-5555-4555-8555-555555555555"
)

type staleReviewerReadTx struct {
	*reviewerAuditFakeTx
	latestDeclarationID string
	caseState           dv.CaseState
	currentQueryErr     error
	currentQueryCalls   int
	currentQueryArgs    []any
	fileQueryCalls      int
}

func (tx *staleReviewerReadTx) QueryRow(ctx context.Context, query string, args ...any) pgx.Row {
	if strings.Contains(query, identityRevisionCurrentQueryShape) {
		tx.currentQueryCalls++
		tx.currentQueryArgs = append([]any(nil), args...)
		if len(args) != 2 || args[0] != staleReviewerReadSubject || args[1] != staleReviewerReadDeclaration {
			return commitSerializationFailureRow{err: errors.New("review declaration freshness query arguments mismatch")}
		}
		if tx.currentQueryErr != nil {
			return commitSerializationFailureRow{err: tx.currentQueryErr}
		}
		return commitSerializationFailureRow{values: []any{tx.latestDeclarationID == tx.declarationID}}
	}
	if strings.Contains(query, "SELECT subject_id::text,space_id::text FROM public.document_verification_case") {
		if len(args) != 1 || args[0] != tx.caseID {
			return commitSerializationFailureRow{err: errors.New("review case scope query arguments mismatch")}
		}
		return commitSerializationFailureRow{values: []any{tx.subject, nil}}
	}
	if strings.Contains(query, "SELECT "+reviewerSnapshotColumns) {
		return commitSerializationFailureRow{values: tx.snapshotValues()}
	}
	return tx.reviewerAuditFakeTx.QueryRow(ctx, query, args...)
}

func (tx *staleReviewerReadTx) Query(ctx context.Context, query string, args ...any) (pgx.Rows, error) {
	if strings.Contains(query, "SELECT "+reviewerSnapshotColumns) && strings.Contains(query, "AND EXISTS (") {
		return &reviewerAuditFakeRows{rows: [][]any{tx.snapshotValues()}}, nil
	}
	if strings.Contains(query, "FROM public.document_verification_file") {
		tx.fileQueryCalls++
		return &reviewerAuditFakeRows{}, nil
	}
	return tx.reviewerAuditFakeTx.Query(ctx, query, args...)
}

func (tx *staleReviewerReadTx) snapshotValues() []any {
	state := tx.caseState
	if state == "" {
		state = dv.CaseInReview
	}
	return []any{
		tx.caseID, string(dv.PurposeIdentity), tx.subject, nil, string(state), int64(7), "",
		nil, nil, nil, nil, nil, tx.declarationID, string(dv.DocumentFormatNew),
		dv.IdentityCaptureConsentVersion, tx.consentedAt, "",
	}
}

type staleReviewerReadBeginner struct{ tx pgx.Tx }

func (b staleReviewerReadBeginner) BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error) {
	return b.tx, nil
}

func TestStaleIdentityDeclarationIsHiddenFromReviewerReadPaths(t *testing.T) {
	lookupErr := errors.New("synthetic declaration lookup failure")
	for _, tc := range []struct {
		name          string
		method        string
		latestID      string
		caseState     dv.CaseState
		currentErr    error
		wantErr       error
		wantResults   int
		wantCallback  bool
		wantCurrent   int
		wantFileReads int
		wantCommit    bool
	}{
		{
			name:          "queue omits stale case",
			method:        "list",
			latestID:      staleReviewerReadLatest,
			wantCurrent:   1,
			wantResults:   0,
			wantFileReads: 0,
			wantCommit:    true,
		},
		{
			name:          "queue keeps current case",
			method:        "list",
			latestID:      staleReviewerReadDeclaration,
			wantCurrent:   1,
			wantResults:   1,
			wantFileReads: 1,
			wantCommit:    true,
		},
		{
			name:          "single read hides stale case",
			method:        "load",
			latestID:      staleReviewerReadLatest,
			wantCurrent:   1,
			wantErr:       dv.ErrNotFound,
			wantFileReads: 0,
		},
		{
			name:          "single read keeps current case",
			method:        "load",
			latestID:      staleReviewerReadDeclaration,
			wantCurrent:   1,
			wantResults:   1,
			wantFileReads: 1,
			wantCommit:    true,
		},
		{
			name:          "scoped detail read hides stale case",
			method:        "scoped",
			latestID:      staleReviewerReadLatest,
			wantCurrent:   1,
			wantErr:       dv.ErrNotFound,
			wantFileReads: 0,
		},
		{
			name:          "scoped detail read keeps current case",
			method:        "scoped",
			latestID:      staleReviewerReadDeclaration,
			wantCurrent:   1,
			wantResults:   1,
			wantCallback:  true,
			wantFileReads: 1,
			wantCommit:    true,
		},
		{
			name:          "scoped detail keeps terminal history despite newer declaration",
			method:        "scoped",
			latestID:      staleReviewerReadLatest,
			caseState:     dv.CaseApproved,
			wantResults:   1,
			wantCallback:  true,
			wantCurrent:   0,
			wantFileReads: 1,
			wantCommit:    true,
		},
		{
			name:          "scoped detail fails closed when freshness lookup errors",
			method:        "scoped",
			latestID:      staleReviewerReadLatest,
			currentErr:    lookupErr,
			wantErr:       lookupErr,
			wantCurrent:   1,
			wantFileReads: 0,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx := &staleReviewerReadTx{
				reviewerAuditFakeTx: &reviewerAuditFakeTx{
					actor: staleReviewerReadActor, subject: staleReviewerReadSubject,
					caseID: staleReviewerReadCase, declarationID: staleReviewerReadDeclaration,
					consentedAt: time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC),
				},
				latestDeclarationID: tc.latestID,
				caseState:           tc.caseState,
				currentQueryErr:     tc.currentErr,
			}
			repo := newWithTransactionBeginner(staleReviewerReadBeginner{tx: tx})
			var resultCount int
			callbackCalled := false
			var err error
			switch tc.method {
			case "list":
				var cases []dv.Snapshot
				cases, err = repo.ListReviewerCases(context.Background(), staleReviewerReadActor)
				resultCount = len(cases)
			case "load":
				var snapshot dv.Snapshot
				snapshot, err = repo.LoadReviewerCase(context.Background(), staleReviewerReadActor, staleReviewerReadCase)
				if err == nil && snapshot.ID == staleReviewerReadCase {
					resultCount = 1
				}
			case "scoped":
				err = repo.WithReviewerCaseForRead(context.Background(), staleReviewerReadActor, staleReviewerReadCase, func(_ context.Context, snapshot dv.Snapshot, authorization dv.Authorization) error {
					callbackCalled = true
					if snapshot.ID != staleReviewerReadCase || authorization.ActorID != staleReviewerReadActor || len(authorization.ReviewerGrants) != 1 {
						t.Fatalf("callback snapshot=%+v authorization=%+v", snapshot, authorization)
					}
					resultCount = 1
					return nil
				})
			default:
				t.Fatalf("unexpected read method %q", tc.method)
			}

			if !errors.Is(err, tc.wantErr) || resultCount != tc.wantResults || callbackCalled != tc.wantCallback {
				t.Fatalf("error=%v results=%d callback=%v, want error=%v results=%d callback=%v", err, resultCount, callbackCalled, tc.wantErr, tc.wantResults, tc.wantCallback)
			}
			if tx.currentQueryCalls != tc.wantCurrent || (tc.wantCurrent == 1 && (len(tx.currentQueryArgs) != 2 || tx.currentQueryArgs[0] != staleReviewerReadSubject || tx.currentQueryArgs[1] != staleReviewerReadDeclaration)) {
				t.Fatalf("declaration-current query calls=%d args=%v", tx.currentQueryCalls, tx.currentQueryArgs)
			}
			if tx.fileQueryCalls != tc.wantFileReads || tx.committed != tc.wantCommit || len(tx.execs) != 0 {
				t.Fatalf("file reads=%d committed=%v writes=%d", tx.fileQueryCalls, tx.committed, len(tx.execs))
			}
		})
	}
}
