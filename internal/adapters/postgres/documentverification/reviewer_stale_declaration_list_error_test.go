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

type staleReviewerQueueFreshnessErrorTx struct {
	*staleReviewerReadTx
	rollbackCalls int
}

func (tx *staleReviewerQueueFreshnessErrorTx) Rollback(ctx context.Context) error {
	tx.rollbackCalls++
	return tx.staleReviewerReadTx.Rollback(ctx)
}

func TestReviewerQueueFailsClosedWhenDeclarationFreshnessLookupErrors(t *testing.T) {
	lookupErr := errors.New("synthetic declaration freshness lookup failure")
	baseTx := &staleReviewerReadTx{
		reviewerAuditFakeTx: &reviewerAuditFakeTx{
			actor: staleReviewerReadActor, subject: staleReviewerReadSubject,
			caseID: staleReviewerReadCase, declarationID: staleReviewerReadDeclaration,
			consentedAt: time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC),
		},
		latestDeclarationID: staleReviewerReadLatest,
		currentQueryErr:     lookupErr,
	}
	tx := &staleReviewerQueueFreshnessErrorTx{staleReviewerReadTx: baseTx}
	repo := newWithTransactionBeginner(staleReviewerReadBeginner{tx: tx})

	cases, err := repo.ListReviewerCases(context.Background(), staleReviewerReadActor)
	if !errors.Is(err, lookupErr) {
		t.Fatalf("ListReviewerCases error=%v, want original lookup error", err)
	}
	if cases != nil {
		t.Fatalf("ListReviewerCases returned partial results: %+v", cases)
	}
	if tx.currentQueryCalls != 1 || len(tx.currentQueryArgs) != 2 ||
		tx.currentQueryArgs[0] != staleReviewerReadSubject || tx.currentQueryArgs[1] != staleReviewerReadDeclaration {
		t.Fatalf("freshness lookup calls=%d args=%v", tx.currentQueryCalls, tx.currentQueryArgs)
	}
	if tx.fileQueryCalls != 0 || tx.committed || tx.rollbackCalls != 1 || len(tx.execs) != 0 {
		t.Fatalf("file reads=%d committed=%v rollbacks=%d writes=%d", tx.fileQueryCalls, tx.committed, tx.rollbackCalls, len(tx.execs))
	}
}

type staleReviewerQueueLateFreshnessErrorTx struct {
	*staleReviewerQueueFreshnessErrorTx
	candidates          [][]any
	grantRows           [][]any
	grantScopesByCase   map[string][]string
	lookupErr           error
	firstSubject        string
	firstDeclaration    string
	secondSubject       string
	secondDeclaration   string
	candidateListCalls  int
	candidateReadIDs    []string
	currentCandidateID  string
	freshnessLookupCall int
	freshnessArgs       [][2]string
	events              []string
}

func (tx *staleReviewerQueueLateFreshnessErrorTx) Query(ctx context.Context, query string, args ...any) (pgx.Rows, error) {
	if strings.Contains(query, "SELECT "+reviewerSnapshotColumns) && strings.Contains(query, "AND EXISTS (") {
		tx.candidateListCalls++
		if len(args) != 1 || args[0] != staleReviewerReadActor {
			return nil, errors.New("review candidate list query arguments mismatch")
		}
		tx.events = append(tx.events, "list")
		return &reviewerAuditFakeRows{rows: tx.candidates}, nil
	}
	if strings.Contains(query, "FROM public.document_verification_reviewer_grant") {
		if len(args) != 1 || args[0] != staleReviewerReadActor {
			return nil, errors.New("review grant query arguments mismatch")
		}
		scopes := make([]string, 0, len(tx.grantRows))
		for _, row := range tx.grantRows {
			if len(row) != 3 || row[0] != args[0] || row[1] != string(dv.CapabilityIdentity) {
				return nil, errors.New("review grant fixture row mismatch")
			}
			scope, ok := row[2].(string)
			if !ok {
				return nil, errors.New("review grant scope is not a string")
			}
			scopes = append(scopes, scope)
			tx.events = append(tx.events, "grant:"+tx.currentCandidateID+":"+scope)
		}
		if tx.grantScopesByCase == nil {
			tx.grantScopesByCase = make(map[string][]string)
		}
		tx.grantScopesByCase[tx.currentCandidateID] = append([]string(nil), scopes...)
		return &reviewerAuditFakeRows{rows: tx.grantRows}, nil
	}
	if strings.Contains(query, "FROM public.document_verification_file") {
		if len(args) != 1 || args[0] != tx.currentCandidateID {
			return nil, errors.New("review file query arguments mismatch")
		}
		tx.events = append(tx.events, "files:"+tx.currentCandidateID)
	}
	return tx.staleReviewerQueueFreshnessErrorTx.Query(ctx, query, args...)
}

func (tx *staleReviewerQueueLateFreshnessErrorTx) QueryRow(ctx context.Context, query string, args ...any) pgx.Row {
	if strings.Contains(query, identityRevisionCurrentQueryShape) {
		tx.freshnessLookupCall++
		tx.currentQueryCalls++
		tx.currentQueryArgs = append([]any(nil), args...)
		if len(args) != 2 {
			return commitSerializationFailureRow{err: errors.New("review declaration freshness query arguments mismatch")}
		}
		subject, subjectOK := args[0].(string)
		declaration, declarationOK := args[1].(string)
		if !subjectOK || !declarationOK {
			return commitSerializationFailureRow{err: errors.New("review declaration freshness query arguments are not strings")}
		}
		tx.freshnessArgs = append(tx.freshnessArgs, [2]string{subject, declaration})
		tx.events = append(tx.events, "freshness:"+subject+":"+declaration)
		if tx.freshnessLookupCall == 1 && subject == tx.firstSubject && declaration == tx.firstDeclaration {
			return commitSerializationFailureRow{values: []any{true}}
		}
		if tx.freshnessLookupCall == 2 && subject == tx.secondSubject && declaration == tx.secondDeclaration {
			return commitSerializationFailureRow{err: tx.lookupErr}
		}
		return commitSerializationFailureRow{err: errors.New("unexpected reviewer freshness lookup order or scope")}
	}
	if strings.Contains(query, "SELECT "+reviewerSnapshotColumns) {
		if len(args) != 2 || args[1] != staleReviewerReadActor {
			return commitSerializationFailureRow{err: errors.New("review candidate query arguments mismatch")}
		}
		candidateID, ok := args[0].(string)
		if !ok {
			return commitSerializationFailureRow{err: errors.New("review candidate ID argument is not a string")}
		}
		tx.candidateReadIDs = append(tx.candidateReadIDs, candidateID)
		tx.currentCandidateID = candidateID
		tx.events = append(tx.events, "case:"+candidateID)
		for _, candidate := range tx.candidates {
			if candidate[0] == candidateID {
				return commitSerializationFailureRow{values: candidate}
			}
		}
		return commitSerializationFailureRow{err: pgx.ErrNoRows}
	}
	return tx.staleReviewerQueueFreshnessErrorTx.QueryRow(ctx, query, args...)
}

func TestReviewerQueueDiscardsEarlierCandidatesWhenLaterFreshnessLookupErrors(t *testing.T) {
	lookupErr := errors.New("synthetic second declaration freshness lookup failure")
	const (
		firstCaseID       = staleReviewerReadCase
		secondCaseID      = "66666666-6666-4666-8666-666666666666"
		firstSubject      = staleReviewerReadSubject
		firstDeclaration  = staleReviewerReadDeclaration
		secondSubject     = "77777777-7777-4777-8777-777777777777"
		secondDeclaration = "88888888-8888-4888-8888-888888888888"
	)
	base := &staleReviewerReadTx{
		reviewerAuditFakeTx: &reviewerAuditFakeTx{
			actor: staleReviewerReadActor, subject: staleReviewerReadSubject,
			caseID: staleReviewerReadCase, declarationID: staleReviewerReadDeclaration,
			consentedAt: time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC),
		},
		latestDeclarationID: staleReviewerReadDeclaration,
	}
	first := base.snapshotValues()
	first[0] = firstCaseID
	first[2] = firstSubject
	first[12] = firstDeclaration
	second := append([]any(nil), first...)
	second[0] = secondCaseID
	second[2] = secondSubject
	second[12] = secondDeclaration
	tx := &staleReviewerQueueLateFreshnessErrorTx{
		staleReviewerQueueFreshnessErrorTx: &staleReviewerQueueFreshnessErrorTx{staleReviewerReadTx: base},
		candidates:                         [][]any{first, second},
		grantRows: [][]any{
			{staleReviewerReadActor, "review_identity", firstSubject},
			{staleReviewerReadActor, "review_identity", secondSubject},
		},
		lookupErr:         lookupErr,
		firstSubject:      firstSubject,
		firstDeclaration:  firstDeclaration,
		secondSubject:     secondSubject,
		secondDeclaration: secondDeclaration,
	}
	repo := newWithTransactionBeginner(staleReviewerReadBeginner{tx: tx})

	cases, err := repo.ListReviewerCases(context.Background(), staleReviewerReadActor)
	if !errors.Is(err, lookupErr) {
		t.Fatalf("ListReviewerCases error=%v, want second lookup error", err)
	}
	if cases != nil {
		t.Fatalf("ListReviewerCases leaked candidates accumulated before error: %+v", cases)
	}
	if tx.candidateListCalls != 1 || len(tx.candidateReadIDs) != 2 || tx.freshnessLookupCall != 2 || tx.currentQueryCalls != 2 {
		t.Fatalf("candidate lists=%d candidate reads=%d freshness lookups=%d counted lookups=%d", tx.candidateListCalls, len(tx.candidateReadIDs), tx.freshnessLookupCall, tx.currentQueryCalls)
	}
	wantReadIDs := []string{firstCaseID, secondCaseID}
	if len(tx.candidateReadIDs) != len(wantReadIDs) || tx.candidateReadIDs[0] != wantReadIDs[0] || tx.candidateReadIDs[1] != wantReadIDs[1] {
		t.Fatalf("candidate read IDs=%v, want %v", tx.candidateReadIDs, wantReadIDs)
	}
	wantFreshnessArgs := [][2]string{{firstSubject, firstDeclaration}, {secondSubject, secondDeclaration}}
	if len(tx.freshnessArgs) != len(wantFreshnessArgs) || tx.freshnessArgs[0] != wantFreshnessArgs[0] || tx.freshnessArgs[1] != wantFreshnessArgs[1] {
		t.Fatalf("freshness lookup args=%v, want %v", tx.freshnessArgs, wantFreshnessArgs)
	}
	wantEvents := []string{
		"list",
		"case:" + firstCaseID,
		"grant:" + firstCaseID + ":" + firstSubject,
		"grant:" + firstCaseID + ":" + secondSubject,
		"freshness:" + firstSubject + ":" + firstDeclaration,
		"files:" + firstCaseID,
		"case:" + secondCaseID,
		"grant:" + secondCaseID + ":" + firstSubject,
		"grant:" + secondCaseID + ":" + secondSubject,
		"freshness:" + secondSubject + ":" + secondDeclaration,
	}
	if strings.Join(tx.events, "|") != strings.Join(wantEvents, "|") {
		t.Fatalf("event trace=%v, want %v", tx.events, wantEvents)
	}
	for candidateID, matchingSubject := range map[string]string{
		firstCaseID:  firstSubject,
		secondCaseID: secondSubject,
	} {
		got := tx.grantScopesByCase[candidateID]
		if len(got) != 2 || got[0] != firstSubject || got[1] != secondSubject {
			t.Fatalf("candidate %s loaded grant scopes=%v, want actor-wide exact scopes [%s %s]", candidateID, got, firstSubject, secondSubject)
		}
		matched := false
		for _, scope := range got {
			if scope == matchingSubject {
				matched = true
				break
			}
		}
		if !matched {
			t.Fatalf("candidate %s subject %s has no exact identity grant in loaded scopes %v", candidateID, matchingSubject, got)
		}
	}
	if tx.fileQueryCalls != 1 || tx.committed || tx.rollbackCalls != 1 || len(tx.execs) != 0 {
		t.Fatalf("file reads=%d committed=%v rollbacks=%d writes=%d", tx.fileQueryCalls, tx.committed, tx.rollbackCalls, len(tx.execs))
	}
}
