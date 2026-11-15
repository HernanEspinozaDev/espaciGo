package documentverificationpg

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
)

type reviewerReadCommitFailureTx struct {
	*reviewerCaseScopeReadTx
	commitErr   error
	commitCalls int
}

func (tx *reviewerReadCommitFailureTx) Commit(context.Context) error {
	tx.commitCalls++
	return tx.commitErr
}

type reviewerReadCommitFailureStore struct {
	reads int
}

func (s *reviewerReadCommitFailureStore) ReadIdentityDocument(context.Context, string, string, string) ([]byte, error) {
	s.reads++
	return nil, dv.ErrNotFound
}

func (s *reviewerReadCommitFailureStore) ReadSpaceDocument(context.Context, string, string, string, string) ([]byte, error) {
	s.reads++
	return nil, dv.ErrNotFound
}

func TestReviewerAccessLoadDetailDiscardsResultWhenReadCommitFails(t *testing.T) {
	const (
		actor         = "11111111-1111-4111-8111-111111111111"
		subject       = "22222222-2222-4222-8222-222222222222"
		caseID        = "33333333-3333-4333-8333-333333333333"
		declarationID = "44444444-4444-4444-8444-444444444444"
	)
	commitErr := errors.New("synthetic reviewer read commit failure")
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
		return dv.ReviewerIdentityDeclaration{}, nil
	})
	access, err := dv.NewReviewerAccessServiceWithIdentityDeclarationReader(repo, caseService, store, declarations)
	if err != nil {
		t.Fatal(err)
	}

	detail, err := access.LoadDetail(context.Background(), actor, caseID)
	if !errors.Is(err, commitErr) {
		t.Fatalf("error=%v, want synthetic commit failure", err)
	}
	if detail.Snapshot.ID != "" || detail.IdentityDeclaration != nil {
		t.Fatalf("detail=%+v, want zero detail on read-transaction commit failure", detail)
	}
	if tx.commitCalls != 1 || base.committed || readTx.rollbackCalls != 1 || len(base.execs) != 0 {
		t.Fatalf("commit calls=%d committed=%v rollback calls=%d writes=%d, want 1/false/1/0", tx.commitCalls, base.committed, readTx.rollbackCalls, len(base.execs))
	}
	if readTx.caseReadCalls != 1 || readTx.grantQueryCalls != 1 || readTx.currentDeclarationCalls != 1 || readTx.fileQueryCalls != 1 {
		t.Fatalf("case/grant/freshness/file reads=%d/%d/%d/%d, want 1/1/1/1", readTx.caseReadCalls, readTx.grantQueryCalls, readTx.currentDeclarationCalls, readTx.fileQueryCalls)
	}
	if declarationReads != 0 || store.reads != 0 {
		t.Fatalf("declaration/document reads=%d/%d, want 0/0 after failed transaction commit", declarationReads, store.reads)
	}
}

func TestReviewerAccessLoadDiscardsSnapshotWhenReadCommitFails(t *testing.T) {
	const (
		actor         = "11111111-1111-4111-8111-111111111111"
		subject       = "22222222-2222-4222-8222-222222222222"
		caseID        = "33333333-3333-4333-8333-333333333333"
		declarationID = "44444444-4444-4444-8444-444444444444"
	)
	commitErr := errors.New("synthetic reviewer snapshot commit failure")
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
	access, err := dv.NewReviewerAccessService(repo, caseService, store)
	if err != nil {
		t.Fatal(err)
	}

	snapshot, err := access.Load(context.Background(), actor, caseID)
	if !errors.Is(err, commitErr) {
		t.Fatalf("error=%v, want synthetic commit failure", err)
	}
	if !reflect.DeepEqual(snapshot, dv.Snapshot{}) {
		t.Fatalf("snapshot=%+v, want zero snapshot after read-transaction commit failure", snapshot)
	}
	if tx.commitCalls != 1 || base.committed || readTx.rollbackCalls != 1 || len(base.execs) != 0 {
		t.Fatalf("commit calls=%d committed=%v rollback calls=%d writes=%d, want 1/false/1/0", tx.commitCalls, base.committed, readTx.rollbackCalls, len(base.execs))
	}
	if readTx.caseReadCalls != 1 || readTx.grantQueryCalls != 1 || readTx.currentDeclarationCalls != 1 || readTx.fileQueryCalls != 1 {
		t.Fatalf("case/grant/freshness/file reads=%d/%d/%d/%d, want 1/1/1/1", readTx.caseReadCalls, readTx.grantQueryCalls, readTx.currentDeclarationCalls, readTx.fileQueryCalls)
	}
	if store.reads != 0 {
		t.Fatalf("document reads=%d, want 0 for metadata-only load", store.reads)
	}
}
