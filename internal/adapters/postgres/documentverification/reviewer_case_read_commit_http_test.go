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
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
)

type reviewerReadCommitFailureHTTPAuth struct{ actor string }

func (a reviewerReadCommitFailureHTTPAuth) Authorize(context.Context, identity.Secret, identity.Role, identity.ActivityKind) (identity.Principal, error) {
	return identity.Principal{AccountID: a.actor}, nil
}

func TestReviewerIdentityDetailKeepsCommitFailureNeutralAtRegisteredHTTPRoute(t *testing.T) {
	const (
		actor         = "11111111-1111-4111-8111-111111111111"
		subject       = "22222222-2222-4222-8222-222222222222"
		caseID        = "33333333-3333-4333-8333-333333333333"
		declarationID = "44444444-4444-4444-8444-444444444444"
	)
	commitErr := errors.New("synthetic reviewer detail commit diagnostic")
	base := &reviewerAuditFakeTx{
		actor: actor, subject: subject, caseID: caseID, declarationID: declarationID,
		consentedAt: time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC),
	}
	readTx := &reviewerCaseScopeReadTx{
		reviewerAuditFakeTx: base,
		grants:              []dv.ReviewerGrant{{ReviewerID: actor, Capability: dv.CapabilityIdentity, ScopeID: subject}},
	}
	tx := &reviewerReadCommitFailureTx{reviewerCaseScopeReadTx: readTx, commitErr: commitErr}
	repo := newWithTransactionBeginner(reviewerCaseScopeReadBeginner{tx: tx})
	caseService, err := dv.NewService(repo)
	if err != nil {
		t.Fatal(err)
	}
	store := &reviewerReadCommitFailureStore{}
	declarationReads := 0
	declarations := dv.ReviewerIdentityDeclarationReaderFunc(func(context.Context, string, string) (dv.ReviewerIdentityDeclaration, error) {
		declarationReads++
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
		response.Header().Get("X-Content-Type-Options") != "nosniff" || response.Header().Get("Pragma") != "no-cache" ||
		response.Header().Get("Content-Type") != "text/plain; charset=utf-8" {
		t.Fatalf("commit-failure response headers=%v", response.Header())
	}
	for _, sensitive := range []string{actor, subject, caseID, declarationID, commitErr.Error(), "identity_declaration"} {
		if strings.Contains(response.Body.String(), sensitive) {
			t.Fatalf("commit-failure response disclosed %q: %q", sensitive, response.Body.String())
		}
	}
	if tx.commitCalls != 1 || base.committed || readTx.rollbackCalls != 1 || len(base.execs) != 0 {
		t.Fatalf("commit calls=%d committed=%v rollback calls=%d writes=%d, want 1/false/1/0", tx.commitCalls, base.committed, readTx.rollbackCalls, len(base.execs))
	}
	if readTx.caseReadCalls != 1 || readTx.grantQueryCalls != 1 || readTx.currentDeclarationCalls != 1 || readTx.fileQueryCalls != 1 {
		t.Fatalf("case/grant/freshness/file reads=%d/%d/%d/%d, want 1/1/1/1", readTx.caseReadCalls, readTx.grantQueryCalls, readTx.currentDeclarationCalls, readTx.fileQueryCalls)
	}
	if declarationReads != 0 || store.reads != 0 {
		t.Fatalf("declaration/document reads=%d/%d, want 0/0 after failed scoped read commit", declarationReads, store.reads)
	}
}
