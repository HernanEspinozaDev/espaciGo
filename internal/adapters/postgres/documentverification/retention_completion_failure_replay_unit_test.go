package documentverificationpg

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/documentfs"
	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type retentionCompletionFailureTx struct {
	*retentionFencingUnitTx
	err                error
	completionAttempts int
}

func (tx *retentionCompletionFailureTx) Exec(ctx context.Context, query string, args ...any) (pgconn.CommandTag, error) {
	if strings.Contains(query, "UPDATE public.document_verification_job SET state='completado'") {
		tx.completionAttempts++
		tx.trace = append(tx.trace, "complete_failed")
		return pgconn.CommandTag{}, tx.err
	}
	return tx.retentionFencingUnitTx.Exec(ctx, query, args...)
}

type retentionCompletionFailureBeginner struct{ tx pgx.Tx }

func (b retentionCompletionFailureBeginner) BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error) {
	return b.tx, nil
}

func TestProcessClaimedRetentionJobRetryIsIdempotentAfterCompletionFailure(t *testing.T) {
	const (
		jobID     = "00000000-0000-4000-8000-000000000091"
		fileID    = "00000000-0000-4000-8000-000000000092"
		caseID    = "00000000-0000-4000-8000-000000000093"
		subjectID = "00000000-0000-4000-8000-000000000094"
	)
	ctx := context.Background()
	claim := RetentionClaim{
		JobID: jobID, FileID: fileID, CaseID: caseID, SubjectID: subjectID,
		Purpose: dv.PurposeIdentity, LeaseUntil: time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC),
	}
	store, err := documentfs.New(filepath.Join(t.TempDir(), "private-documents"), []byte("01234567890123456789012345678901"))
	if err != nil {
		t.Fatal(err)
	}
	content := []byte("synthetic encrypted identity document")
	if err := store.PutIdentityDocument(ctx, subjectID, caseID, fileID, content); err != nil {
		t.Fatal(err)
	}

	completionErr := errors.New("synthetic completion update failure")
	firstBase := &retentionFencingUnitTx{claim: claim, leaseMatches: true, leaseLive: true}
	firstTx := &retentionCompletionFailureTx{retentionFencingUnitTx: firstBase, err: completionErr}
	firstRepo := newWithTransactionBeginner(retentionCompletionFailureBeginner{tx: firstTx})
	firstDeleteCalled := false
	if err := firstRepo.ProcessClaimedRetentionJob(ctx, claim, func(ctx context.Context) error {
		firstDeleteCalled = true
		return store.DeleteIdentityDocument(ctx, subjectID, caseID, fileID)
	}, time.Minute); !errors.Is(err, completionErr) {
		t.Fatalf("first ProcessClaimedRetentionJob() error=%v, want completion update failure", err)
	}
	if !firstDeleteCalled || firstTx.completionAttempts != 1 || firstBase.committed || firstBase.rollbacks != 1 || firstBase.unexpectedExec != "" || len(firstBase.execs) != 0 || !reflect.DeepEqual(firstBase.trace, []string{"complete_failed"}) {
		t.Fatalf("first delete=%t completion attempts=%d committed=%t rollbacks=%d execs=%v trace=%v unexpected=%q", firstDeleteCalled, firstTx.completionAttempts, firstBase.committed, firstBase.rollbacks, firstBase.execs, firstBase.trace, firstBase.unexpectedExec)
	}
	if _, err := store.ReadIdentityDocument(ctx, subjectID, caseID, fileID); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("document after failed completion update: err=%v, want not-exist", err)
	}

	secondBase := &retentionFencingUnitTx{claim: claim, leaseMatches: true, leaseLive: true}
	secondRepo := newWithTransactionBeginner(retentionFencingUnitBeginner{tx: secondBase})
	secondDeleteCalled := false
	if err := secondRepo.ProcessClaimedRetentionJob(ctx, claim, func(ctx context.Context) error {
		secondDeleteCalled = true
		return store.DeleteIdentityDocument(ctx, subjectID, caseID, fileID)
	}, time.Minute); err != nil {
		t.Fatalf("retry ProcessClaimedRetentionJob() error=%v", err)
	}
	if !secondDeleteCalled || !secondBase.committed || secondBase.rollbacks != 1 || secondBase.unexpectedExec != "" || !reflect.DeepEqual(secondBase.execs, []string{"complete", "audit"}) || !reflect.DeepEqual(secondBase.trace, []string{"complete", "audit", "commit"}) {
		t.Fatalf("retry delete=%t committed=%t rollbacks=%d execs=%v trace=%v unexpected=%q", secondDeleteCalled, secondBase.committed, secondBase.rollbacks, secondBase.execs, secondBase.trace, secondBase.unexpectedExec)
	}
	if _, err := store.ReadIdentityDocument(ctx, subjectID, caseID, fileID); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("document after retry: err=%v, want not-exist", err)
	}
}
