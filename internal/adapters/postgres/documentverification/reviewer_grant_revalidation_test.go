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
	"github.com/jackc/pgx/v5"
)

type reviewerGrantRevalidationBeginner struct {
	transactions []pgx.Tx
	calls        int
}

func (b *reviewerGrantRevalidationBeginner) BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error) {
	if b.calls >= len(b.transactions) {
		return nil, errors.New("unexpected transaction")
	}
	tx := b.transactions[b.calls]
	b.calls++
	return tx, nil
}

type reviewerGrantRevokedTx struct {
	*reviewerAuditFakeTx
	grantQueryCalls int
	fileQueryCalls  int
	rollbackCalls   int
}

func (tx *reviewerGrantRevokedTx) Query(ctx context.Context, query string, args ...any) (pgx.Rows, error) {
	switch {
	case strings.Contains(query, "FROM public.document_verification_reviewer_grant"):
		tx.grantQueryCalls++
		if len(args) != 1 || args[0] != tx.actor {
			return nil, errors.New("reviewer grant query actor mismatch")
		}
		return &reviewerAuditFakeRows{}, nil
	case strings.Contains(query, "FROM public.document_verification_file"):
		tx.fileQueryCalls++
		return tx.reviewerAuditFakeTx.Query(ctx, query, args...)
	default:
		return tx.reviewerAuditFakeTx.Query(ctx, query, args...)
	}
}

func (tx *reviewerGrantRevokedTx) Rollback(context.Context) error {
	tx.rollbackCalls++
	return nil
}

type reviewerGrantRevalidationAuthenticator struct{ actor string }

func (a reviewerGrantRevalidationAuthenticator) Authorize(context.Context, identity.Secret, identity.Role, identity.ActivityKind) (identity.Principal, error) {
	return identity.Principal{AccountID: a.actor}, nil
}

type reviewerGrantRevalidationStore struct{}

func (reviewerGrantRevalidationStore) ReadIdentityDocument(context.Context, string, string, string) ([]byte, error) {
	return nil, dv.ErrNotFound
}

func (reviewerGrantRevalidationStore) ReadSpaceDocument(context.Context, string, string, string, string) ([]byte, error) {
	return nil, dv.ErrNotFound
}

func TestIdentityReviewerGrantRevokedAfterPreflightStopsBeforeFileRead(t *testing.T) {
	const (
		actor         = "11111111-1111-4111-8111-111111111111"
		subject       = "22222222-2222-4222-8222-222222222222"
		caseID        = "33333333-3333-4333-8333-333333333333"
		declarationID = "44444444-4444-4444-8444-444444444444"
	)

	preflightTx := &reviewerCaseScopeReadTx{
		reviewerAuditFakeTx: &reviewerAuditFakeTx{
			actor:         actor,
			subject:       subject,
			caseID:        caseID,
			declarationID: declarationID,
			consentedAt:   time.Date(2026, time.November, 13, 12, 0, 0, 0, time.UTC),
		},
		grants: []dv.ReviewerGrant{{ReviewerID: actor, Capability: dv.CapabilityIdentity, ScopeID: subject}},
	}
	mutationBase := &reviewerAuditFakeTx{
		actor:         actor,
		subject:       subject,
		caseID:        caseID,
		declarationID: declarationID,
		consentedAt:   time.Date(2026, time.November, 13, 12, 0, 0, 0, time.UTC),
	}
	mutationTx := &reviewerGrantRevokedTx{reviewerAuditFakeTx: mutationBase}
	transactions := &reviewerGrantRevalidationBeginner{transactions: []pgx.Tx{preflightTx, mutationTx}}
	repo := newWithTransactionBeginner(transactions)
	domain, err := dv.NewService(repo)
	if err != nil {
		t.Fatal(err)
	}
	access, err := dv.NewReviewerAccessService(repo, domain, reviewerGrantRevalidationStore{})
	if err != nil {
		t.Fatal(err)
	}
	handler := reviewerhttp.NewReviewerCasesHandler(reviewerGrantRevalidationAuthenticator{actor: actor}, access)
	mux := http.NewServeMux()
	reviewerhttp.RegisterReviewerCasesRoutes(mux, handler)

	body := `{"expected_revision":7,"decision":"approve","document_format_observed":"new","document_expires_at_observed":"2030-05-01","identity_review_checklist":{"names_match":true,"rut_matches":true,"birth_date_and_majority_match":true,"portrait_matches_selfie":true,"evidence_legible":true,"document_unexpired":true,"issue_date_and_format_verified":true}}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/document-verification/reviewer/cases/"+caseID+"/review", strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer synthetic-reviewer-session")
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound || response.Body.String() != "not found\n" {
		t.Fatalf("response=%d %q", response.Code, response.Body.String())
	}
	if response.Header().Get("Cache-Control") != "private, no-store" || response.Header().Get("ETag") != "" {
		t.Fatalf("response headers=%v", response.Header())
	}
	if transactions.calls != 2 || !preflightTx.committed || preflightTx.grantQueryCalls != 1 || preflightTx.fileQueryCalls != 1 {
		t.Fatalf("preflight transactions=%d committed=%v grant queries=%d file reads=%d", transactions.calls, preflightTx.committed, preflightTx.grantQueryCalls, preflightTx.fileQueryCalls)
	}
	if mutationTx.grantQueryCalls != 1 {
		t.Fatalf("mutation grant queries=%d, want a fresh grant read", mutationTx.grantQueryCalls)
	}
	if mutationTx.fileQueryCalls != 0 {
		t.Fatalf("mutation read %d file metadata queries after the reviewer grant disappeared", mutationTx.fileQueryCalls)
	}
	if mutationTx.rollbackCalls != 1 || mutationTx.committed || len(mutationBase.execs) != 0 {
		t.Fatalf("mutation rollback=%d committed=%v writes=%d", mutationTx.rollbackCalls, mutationTx.committed, len(mutationBase.execs))
	}
}
