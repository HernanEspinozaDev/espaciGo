package documentverificationpg

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
	reviewerhttp "github.com/HernanEspinozaDev/espaciGo/internal/documentverification/transport/http"
	"github.com/jackc/pgx/v5"
)

type reviewerReadCommitSequenceBeginner struct {
	transactions []pgx.Tx
	beginCalls   int
}

func (b *reviewerReadCommitSequenceBeginner) BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error) {
	if b.beginCalls >= len(b.transactions) {
		return nil, errors.New("unexpected extra reviewer read transaction")
	}
	tx := b.transactions[b.beginCalls]
	b.beginCalls++
	return tx, nil
}

type reviewerReadCommitEventTx struct {
	*reviewerCaseScopeReadTx
	events      *[]string
	commitErr   error
	commitCalls int
}

func (tx *reviewerReadCommitEventTx) Commit(ctx context.Context) error {
	tx.commitCalls++
	*tx.events = append(*tx.events, "commit")
	if tx.commitErr != nil {
		return tx.commitErr
	}
	return tx.reviewerCaseScopeReadTx.Commit(ctx)
}

func TestReviewerIdentityDetailDiscardsDeclarationWhenSecondScopedCommitFails(t *testing.T) {
	const (
		actor         = "11111111-1111-4111-8111-111111111111"
		subject       = "22222222-2222-4222-8222-222222222222"
		caseID        = "33333333-3333-4333-8333-333333333333"
		declarationID = "44444444-4444-4444-8444-444444444444"
	)
	commitErr := errors.New("synthetic second reviewer detail commit diagnostic")
	newReadTx := func() (*reviewerAuditFakeTx, *reviewerCaseScopeReadTx) {
		base := &reviewerAuditFakeTx{
			actor: actor, subject: subject, caseID: caseID, declarationID: declarationID,
			consentedAt: time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC),
		}
		return base, &reviewerCaseScopeReadTx{
			reviewerAuditFakeTx: base,
			grants:              []dv.ReviewerGrant{{ReviewerID: actor, Capability: dv.CapabilityIdentity, ScopeID: subject}},
		}
	}
	firstBase, firstReadTx := newReadTx()
	secondBase, secondReadTx := newReadTx()
	events := []string{}
	firstTx := &reviewerReadCommitEventTx{reviewerCaseScopeReadTx: firstReadTx, events: &events}
	secondTx := &reviewerReadCommitEventTx{reviewerCaseScopeReadTx: secondReadTx, events: &events, commitErr: commitErr}
	beginner := &reviewerReadCommitSequenceBeginner{transactions: []pgx.Tx{firstTx, secondTx}}
	repo := newWithTransactionBeginner(beginner)
	caseService, err := dv.NewService(repo)
	if err != nil {
		t.Fatal(err)
	}
	store := &reviewerReadCommitFailureStore{}
	declarationReads := 0
	declarationAccount, declarationRevision := "", ""
	declarations := dv.ReviewerIdentityDeclarationReaderFunc(func(_ context.Context, accountID, revisionID string) (dv.ReviewerIdentityDeclaration, error) {
		declarationReads++
		events = append(events, "declaration")
		declarationAccount, declarationRevision = accountID, revisionID
		return dv.ReviewerIdentityDeclaration{
			FirstName: "SyntheticPrivateName", LastName: "SyntheticPrivateSurname",
			RUT: "SyntheticPrivateRUT", DateOfBirth: "2041-06-07",
		}, nil
	})
	access, err := dv.NewReviewerAccessServiceWithIdentityDeclarationReader(repo, caseService, store, declarations)
	if err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	reviewerhttp.RegisterReviewerCasesRoutes(mux, reviewerhttp.NewReviewerCasesHandler(reviewerReadCommitFailureHTTPAuth{actor: actor}, access))
	request := httptest.NewRequest(http.MethodGet, "/api/v1/document-verification/reviewer/cases/"+caseID, nil)
	request.Header.Set("Authorization", "Bearer synthetic-reviewer-session")
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable || response.Body.String() != "temporarily unavailable\n" {
		t.Fatalf("status=%d body=%q, want neutral 503", response.Code, response.Body.String())
	}
	if response.Header().Get("ETag") != "" || response.Header().Get("Content-Disposition") != "" ||
		response.Header().Get("Cache-Control") != "private, no-store" || response.Header().Get("Vary") != "Authorization" ||
		response.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("commit-failure response headers=%v", response.Header())
	}
	for _, sensitive := range []string{actor, subject, caseID, declarationID, commitErr.Error(), "SyntheticPrivateName", "SyntheticPrivateSurname", "SyntheticPrivateRUT", "2041-06-07", "identity_declaration"} {
		if strings.Contains(response.Body.String(), sensitive) {
			t.Fatalf("commit-failure response disclosed %q: %q", sensitive, response.Body.String())
		}
	}
	if beginner.beginCalls != 2 {
		t.Fatalf("read transactions begun=%d, want exactly two and no final read after failed commit", beginner.beginCalls)
	}
	if !firstBase.committed || secondBase.committed || firstTx.commitCalls != 1 || secondTx.commitCalls != 1 ||
		firstReadTx.rollbackCalls != 1 || secondReadTx.rollbackCalls != 1 || len(firstBase.execs) != 0 || len(secondBase.execs) != 0 {
		t.Fatalf("first commit=%v second commit=%v commit calls=%d/%d rollback calls=%d/%d writes=%d/%d",
			firstBase.committed, secondBase.committed, firstTx.commitCalls, secondTx.commitCalls,
			firstReadTx.rollbackCalls, secondReadTx.rollbackCalls, len(firstBase.execs), len(secondBase.execs))
	}
	if got := strings.Join(events, ","); got != "commit,declaration,commit" {
		t.Fatalf("read/commit event order=%q, want first commit, declaration read, failed second commit", got)
	}
	for i, readTx := range []*reviewerCaseScopeReadTx{firstReadTx, secondReadTx} {
		if readTx.caseReadCalls != 1 || readTx.grantQueryCalls != 1 || readTx.currentDeclarationCalls != 1 || readTx.fileQueryCalls != 1 {
			t.Fatalf("transaction %d case/grant/freshness/file reads=%d/%d/%d/%d, want 1/1/1/1",
				i+1, readTx.caseReadCalls, readTx.grantQueryCalls, readTx.currentDeclarationCalls, readTx.fileQueryCalls)
		}
	}
	if declarationReads != 1 || declarationAccount != subject || declarationRevision != declarationID || store.reads != 0 {
		t.Fatalf("declaration reads/account/revision=%d/%q/%q store reads=%d, want 1/%q/%q/0",
			declarationReads, declarationAccount, declarationRevision, store.reads, subject, declarationID)
	}
}
