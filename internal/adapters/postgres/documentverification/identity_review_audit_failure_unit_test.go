package documentverificationpg

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type identityReviewAuditFailureTx struct {
	*reviewerAuditFakeTx
	auditInsertErr error
	commitCalls    int
	rollbackCalls  int
}

func (tx *identityReviewAuditFailureTx) Exec(ctx context.Context, query string, args ...any) (pgconn.CommandTag, error) {
	tag, err := tx.reviewerAuditFakeTx.Exec(ctx, query, args...)
	if err != nil {
		return tag, err
	}
	if strings.Contains(query, "INSERT INTO public.document_verification_event") {
		return pgconn.CommandTag{}, tx.auditInsertErr
	}
	return tag, nil
}

func (tx *identityReviewAuditFailureTx) Commit(context.Context) error {
	tx.commitCalls++
	tx.committed = true
	return nil
}

func (tx *identityReviewAuditFailureTx) Rollback(context.Context) error {
	tx.rollbackCalls++
	return nil
}

type identityReviewAuditFailureBeginner struct{ tx pgx.Tx }

func (b identityReviewAuditFailureBeginner) BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error) {
	return b.tx, nil
}

func TestIdentityReviewDoesNotCommitWhenAuditInsertFails(t *testing.T) {
	const actor = "11111111-1111-4111-8111-111111111111"
	const subject = "22222222-2222-4222-8222-222222222222"
	const caseID = "33333333-3333-4333-8333-333333333333"
	const declarationID = "44444444-4444-4444-8444-444444444444"

	consentedAt := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	auditErr := errors.New("synthetic audit insert failure")
	tx := &identityReviewAuditFailureTx{
		reviewerAuditFakeTx: &reviewerAuditFakeTx{
			actor: actor, subject: subject, caseID: caseID, declarationID: declarationID,
			consentedAt: consentedAt,
		},
		auditInsertErr: auditErr,
	}
	repo := newWithTransactionBeginner(identityReviewAuditFailureBeginner{tx: tx})
	checklist := &dv.IdentityReviewChecklist{
		NamesMatch: boolPointer(true), RUTMatches: boolPointer(true), BirthDateAndMajorityMatch: boolPointer(true),
		PortraitMatchesSelfie: boolPointer(true), EvidenceLegible: boolPointer(true), DocumentUnexpired: boolPointer(true),
		IssueDateAndFormatVerified: boolPointer(true),
	}
	expiry := time.Date(2031, 4, 6, 3, 0, 0, 0, time.UTC)
	audit := dv.TransitionAudit{
		Action: "review_decision", ResultCode: string(dv.CaseApproved),
		DocumentFormatObserved: dv.DocumentFormatLegacy, DocumentExpiresAtObserved: expiry,
		IdentityReviewChecklist: checklist,
	}

	callbackCalled := false
	err := repo.Transition(context.Background(), actor, dv.PurposeIdentity, caseID, 7, audit, func(x *dv.Snapshot, _ dv.Authorization) error {
		callbackCalled = true
		x.State = dv.CaseApproved
		x.ReviewerID = actor
		x.VerifiedAt = expiry.Add(-time.Hour)
		x.VerifiedUntil = expiry
		x.DocumentFormatObserved = dv.DocumentFormatLegacy
		x.DocumentExpiresAt = expiry
		return nil
	})

	if !errors.Is(err, auditErr) {
		t.Fatalf("Transition() error=%v, want audit insert error", err)
	}
	if len(tx.execs) != 2 || !strings.Contains(tx.execs[0].sql, "UPDATE public.document_verification_case") || !strings.Contains(tx.execs[1].sql, "INSERT INTO public.document_verification_event") {
		t.Fatalf("exec attempts=%+v, want case update followed by audit insert", tx.execs)
	}
	if !callbackCalled {
		t.Fatal("Transition() did not invoke the decision callback before the audit insert failure")
	}
	if tx.commitCalls != 0 || tx.committed || tx.rollbackCalls != 1 {
		t.Fatalf("commit calls=%d committed=%t rollback calls=%d; want no commit and one rollback callback", tx.commitCalls, tx.committed, tx.rollbackCalls)
	}
}
