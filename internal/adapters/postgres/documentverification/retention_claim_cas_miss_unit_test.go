package documentverificationpg

import (
	"context"
	"strings"
	"testing"
	"time"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
	"github.com/jackc/pgx/v5"
)

func TestClaimReadyRetentionJobReturnsNoWorkWhenConditionalUpdateLosesCAS(t *testing.T) {
	claim := RetentionClaim{
		JobID: "00000000-0000-4000-8000-000000000171", FileID: "00000000-0000-4000-8000-000000000172",
		CaseID: "00000000-0000-4000-8000-000000000173", SubjectID: "00000000-0000-4000-8000-000000000174",
		Purpose: dv.PurposeIdentity, Attempt: 2,
	}
	tx := &retentionClaimReadFailureTx{
		claim: claim, failFragment: "UPDATE public.document_verification_job SET state='procesando'", failErr: pgx.ErrNoRows,
	}
	repo := newWithTransactionBeginner(retentionClaimReadFailureBeginner{tx: tx})
	got, err := repo.ClaimReadyRetentionJob(context.Background(), time.Minute)
	if err != nil || got != nil {
		t.Fatalf("ClaimReadyRetentionJob()=(%+v,%v); want no claim and no error after conditional update miss", got, err)
	}
	wantFragments := []string{
		"FROM public.document_verification_job j",
		"SELECT id FROM public.document_verification_case",
		"SELECT id FROM public.document_verification_file",
		"SELECT EXISTS(SELECT 1 FROM public.document_verification_job WHERE file_id=$1 AND kind='scan'",
		"UPDATE public.document_verification_job SET state='procesando'",
	}
	if len(tx.queries) != len(wantFragments) {
		t.Fatalf("query count=%d; want %d in order", len(tx.queries), len(wantFragments))
	}
	for i, fragment := range wantFragments {
		if !strings.Contains(tx.queries[i], fragment) {
			t.Fatalf("query[%d]=%q; want fragment %q", i, tx.queries[i], fragment)
		}
	}
	if tx.commitCalls != 0 || tx.rollbackCalls != 1 || tx.unexpectedSQL != "" {
		t.Fatalf("commits=%d rollbacks=%d unexpected SQL=%q; want no commit and one deferred rollback", tx.commitCalls, tx.rollbackCalls, tx.unexpectedSQL)
	}
}
