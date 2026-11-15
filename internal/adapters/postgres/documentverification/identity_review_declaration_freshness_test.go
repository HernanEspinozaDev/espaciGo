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
	identityReviewFreshnessActor      = "11111111-1111-4111-8111-111111111111"
	identityReviewFreshnessSubject    = "22222222-2222-4222-8222-222222222222"
	identityReviewFreshnessCase       = "33333333-3333-4333-8333-333333333333"
	identityReviewFreshnessLinked     = "44444444-4444-4444-8444-444444444444"
	identityReviewFreshnessLatest     = "55555555-5555-4555-8555-555555555555"
	identityRevisionCurrentQueryShape = "SELECT COALESCE((SELECT revision.id::text=$2"
)

type identityReviewFreshnessTx struct {
	*reviewerAuditFakeTx
	latestDeclarationID string
	currentQueryCalls   int
	currentQueryArgs    []any
	currentQueryErr     error
	fileQueryCalls      int
	rollbackCalls       int
}

func (tx *identityReviewFreshnessTx) QueryRow(ctx context.Context, query string, args ...any) pgx.Row {
	if strings.Contains(query, identityRevisionCurrentQueryShape) {
		tx.currentQueryCalls++
		tx.currentQueryArgs = append([]any(nil), args...)
		if len(args) != 2 || args[0] != identityReviewFreshnessSubject || args[1] != tx.declarationID {
			return commitSerializationFailureRow{err: errors.New("declaration freshness query arguments mismatch")}
		}
		if tx.currentQueryErr != nil {
			return commitSerializationFailureRow{err: tx.currentQueryErr}
		}
		return commitSerializationFailureRow{values: []any{tx.declarationID == tx.latestDeclarationID}}
	}
	return tx.reviewerAuditFakeTx.QueryRow(ctx, query, args...)
}

func (tx *identityReviewFreshnessTx) Query(ctx context.Context, query string, args ...any) (pgx.Rows, error) {
	if strings.Contains(query, "FROM public.document_verification_file") {
		tx.fileQueryCalls++
		return &reviewerAuditFakeRows{}, nil
	}
	return tx.reviewerAuditFakeTx.Query(ctx, query, args...)
}

func (tx *identityReviewFreshnessTx) Rollback(ctx context.Context) error {
	tx.rollbackCalls++
	return tx.reviewerAuditFakeTx.Rollback(ctx)
}

type identityReviewFreshnessBeginner struct{ tx pgx.Tx }

func (b identityReviewFreshnessBeginner) BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error) {
	return b.tx, nil
}

func TestIdentityReviewRejectsCaseLinkedToOlderDeclarationRevision(t *testing.T) {
	stop := errors.New("stop after authorized review callback")
	queryFailure := errors.New("synthetic declaration read failure")
	yes := true
	checklist := &dv.IdentityReviewChecklist{
		NamesMatch: &yes, RUTMatches: &yes, BirthDateAndMajorityMatch: &yes,
		PortraitMatchesSelfie: &yes, EvidenceLegible: &yes, DocumentUnexpired: &yes,
		IssueDateAndFormatVerified: &yes,
	}
	for _, tc := range []struct {
		name                string
		latestDeclarationID string
		currentQueryErr     error
		wantErr             error
		wantCallback        bool
		wantFileReads       int
	}{
		{
			name:                "case declaration is current",
			latestDeclarationID: identityReviewFreshnessLinked,
			wantErr:             stop,
			wantCallback:        true,
			wantFileReads:       1,
		},
		{
			name:                "newer declaration was saved",
			latestDeclarationID: identityReviewFreshnessLatest,
			wantErr:             dv.ErrConflict,
			wantCallback:        false,
			wantFileReads:       0,
		},
		{
			name:                "latest declaration lookup fails closed",
			latestDeclarationID: identityReviewFreshnessLinked,
			currentQueryErr:     queryFailure,
			wantErr:             queryFailure,
			wantCallback:        false,
			wantFileReads:       0,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx := &identityReviewFreshnessTx{
				reviewerAuditFakeTx: &reviewerAuditFakeTx{
					actor:         identityReviewFreshnessActor,
					subject:       identityReviewFreshnessSubject,
					caseID:        identityReviewFreshnessCase,
					declarationID: identityReviewFreshnessLinked,
					consentedAt:   time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC),
				},
				latestDeclarationID: tc.latestDeclarationID,
				currentQueryErr:     tc.currentQueryErr,
			}
			repo := newWithTransactionBeginner(identityReviewFreshnessBeginner{tx: tx})
			reached := false
			err := repo.Transition(
				context.Background(), identityReviewFreshnessActor, dv.PurposeIdentity,
				identityReviewFreshnessCase, 7,
				dv.TransitionAudit{
					Action: "review_decision", ResultCode: string(dv.CaseApproved),
					DocumentFormatObserved:    dv.DocumentFormatNew,
					DocumentExpiresAtObserved: time.Date(2030, 5, 1, 0, 0, 0, 0, time.UTC),
					IdentityReviewChecklist:   checklist,
				},
				func(snapshot *dv.Snapshot, authorization dv.Authorization) error {
					reached = true
					if snapshot.IdentityDeclarationRevisionID != identityReviewFreshnessLinked || authorization.ActorID != identityReviewFreshnessActor {
						t.Fatalf("review callback received snapshot=%+v authorization=%+v", snapshot, authorization)
					}
					return stop
				},
			)

			if !errors.Is(err, tc.wantErr) || reached != tc.wantCallback {
				t.Fatalf("transition error=%v callback=%v, want error=%v callback=%v", err, reached, tc.wantErr, tc.wantCallback)
			}
			if tx.currentQueryCalls != 1 || len(tx.currentQueryArgs) != 2 ||
				tx.currentQueryArgs[0] != identityReviewFreshnessSubject || tx.currentQueryArgs[1] != identityReviewFreshnessLinked {
				t.Fatalf("declaration-current query calls=%d args=%v", tx.currentQueryCalls, tx.currentQueryArgs)
			}
			if tx.fileQueryCalls != tc.wantFileReads {
				t.Fatalf("file metadata reads=%d, want %d", tx.fileQueryCalls, tc.wantFileReads)
			}
			if len(tx.execs) != 0 || tx.committed || tx.rollbackCalls != 1 {
				t.Fatalf("writes=%d committed=%v rollbacks=%d", len(tx.execs), tx.committed, tx.rollbackCalls)
			}
		})
	}
}

func TestIdentitySubmissionRejectsStaleDeclarationBeforeReadingFiles(t *testing.T) {
	stop := errors.New("stop after owner submission callback")
	for _, tc := range []struct {
		name                string
		latestDeclarationID string
		wantErr             error
		wantCallback        bool
		wantFileReads       int
	}{
		{
			name:                "newer declaration was saved",
			latestDeclarationID: identityReviewFreshnessLatest,
			wantErr:             dv.ErrConflict,
			wantCallback:        false,
			wantFileReads:       0,
		},
		{
			name:                "case declaration is current",
			latestDeclarationID: identityReviewFreshnessLinked,
			wantErr:             stop,
			wantCallback:        true,
			wantFileReads:       1,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx := &identityReviewFreshnessTx{
				reviewerAuditFakeTx: &reviewerAuditFakeTx{
					actor:         identityReviewFreshnessSubject,
					subject:       identityReviewFreshnessSubject,
					caseID:        identityReviewFreshnessCase,
					declarationID: identityReviewFreshnessLinked,
					consentedAt:   time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC),
				},
				latestDeclarationID: tc.latestDeclarationID,
			}
			repo := newWithTransactionBeginner(identityReviewFreshnessBeginner{tx: tx})
			reached := false
			err := repo.Transition(
				context.Background(), identityReviewFreshnessSubject, dv.PurposeIdentity,
				identityReviewFreshnessCase, 7,
				dv.TransitionAudit{Action: "case_transitioned", ResultCode: string(dv.CaseSubmitted)},
				func(snapshot *dv.Snapshot, authorization dv.Authorization) error {
					reached = true
					if snapshot.IdentityDeclarationRevisionID != identityReviewFreshnessLinked || authorization.ActorID != identityReviewFreshnessSubject {
						t.Fatalf("submission callback received snapshot=%+v authorization=%+v", snapshot, authorization)
					}
					return stop
				},
			)

			if !errors.Is(err, tc.wantErr) || reached != tc.wantCallback {
				t.Fatalf("transition error=%v callback=%v, want error=%v callback=%v", err, reached, tc.wantErr, tc.wantCallback)
			}
			if tx.currentQueryCalls != 1 || len(tx.currentQueryArgs) != 2 ||
				tx.currentQueryArgs[0] != identityReviewFreshnessSubject || tx.currentQueryArgs[1] != identityReviewFreshnessLinked {
				t.Fatalf("declaration-current query calls=%d args=%v", tx.currentQueryCalls, tx.currentQueryArgs)
			}
			if tx.fileQueryCalls != tc.wantFileReads {
				t.Fatalf("file metadata reads=%d, want %d", tx.fileQueryCalls, tc.wantFileReads)
			}
			if len(tx.execs) != 0 || tx.committed || tx.rollbackCalls != 1 {
				t.Fatalf("writes=%d committed=%v rollbacks=%d", len(tx.execs), tx.committed, tx.rollbackCalls)
			}
		})
	}
}
