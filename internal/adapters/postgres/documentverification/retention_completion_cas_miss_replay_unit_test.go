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

type retentionCompletionCASMissTx struct {
	*retentionFencingUnitTx
	completionAttempts int
}

func (tx *retentionCompletionCASMissTx) Exec(ctx context.Context, query string, args ...any) (pgconn.CommandTag, error) {
	if strings.Contains(query, "UPDATE public.document_verification_job SET state='completado'") {
		tx.completionAttempts++
		tx.trace = append(tx.trace, "complete_zero_rows")
		return pgconn.NewCommandTag("UPDATE 0"), nil
	}
	return tx.retentionFencingUnitTx.Exec(ctx, query, args...)
}

type retentionCompletionCASMissBeginner struct{ tx pgx.Tx }

func (b retentionCompletionCASMissBeginner) BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error) {
	return b.tx, nil
}

func TestProcessClaimedRetentionJobRejectsCompletionCASMissAfterDeletion(t *testing.T) {
	const (
		jobID     = "00000000-0000-4000-8000-0000000000b1"
		fileID    = "00000000-0000-4000-8000-0000000000b2"
		caseID    = "00000000-0000-4000-8000-0000000000b3"
		subjectID = "00000000-0000-4000-8000-0000000000b4"
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
	if err := store.PutIdentityDocument(ctx, subjectID, caseID, fileID, []byte("synthetic encrypted identity document")); err != nil {
		t.Fatal(err)
	}

	firstBase := &retentionFencingUnitTx{claim: claim, leaseMatches: true, leaseLive: true}
	firstTx := &retentionCompletionCASMissTx{retentionFencingUnitTx: firstBase}
	firstRepo := newWithTransactionBeginner(retentionCompletionCASMissBeginner{tx: firstTx})
	firstDeleteCalled := false
	err = firstRepo.ProcessClaimedRetentionJob(ctx, claim, func(ctx context.Context) error {
		firstDeleteCalled = true
		return store.DeleteIdentityDocument(ctx, subjectID, caseID, fileID)
	}, time.Minute)
	if !errors.Is(err, dv.ErrConflict) || !firstDeleteCalled {
		t.Fatalf("first ProcessClaimedRetentionJob() error=%v delete=%t; want completion conflict after deletion", err, firstDeleteCalled)
	}
	if firstTx.completionAttempts != 1 || firstBase.committed || firstBase.rollbacks != 1 || len(firstBase.execs) != 0 || firstBase.unexpectedExec != "" || !reflect.DeepEqual(firstBase.trace, []string{"complete_zero_rows"}) {
		t.Fatalf("completion attempts=%d committed=%t rollbacks=%d execs=%v trace=%v unexpected=%q", firstTx.completionAttempts, firstBase.committed, firstBase.rollbacks, firstBase.execs, firstBase.trace, firstBase.unexpectedExec)
	}
	if _, err := store.ReadIdentityDocument(ctx, subjectID, caseID, fileID); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("document after completion CAS miss: err=%v, want not-exist", err)
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
