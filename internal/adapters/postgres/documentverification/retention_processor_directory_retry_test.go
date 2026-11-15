package documentverificationpg

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/documentfs"
	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
)

func TestRetentionProcessorRetriesWhenObjectPathIsNonEmptyDirectory(t *testing.T) {
	root := filepath.Join(t.TempDir(), "private-documents")
	store, err := documentfs.New(root, make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	const (
		jobID     = "11111111-1111-4111-8111-111111111111"
		fileID    = "22222222-2222-4222-8222-222222222222"
		caseID    = "33333333-3333-4333-8333-333333333333"
		subjectID = "44444444-4444-4444-8444-444444444444"
	)
	ctx := context.Background()
	if err := store.PutIdentityDocument(ctx, subjectID, caseID, fileID, []byte("synthetic identity evidence")); err != nil {
		t.Fatal(err)
	}
	objectPath := filepath.Join(root, "users", subjectID, "identity", caseID, fileID+".bin")
	if err := os.Remove(objectPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(objectPath, 0o700); err != nil {
		t.Fatal(err)
	}
	sentinelPath := filepath.Join(objectPath, "preserve.txt")
	sentinel := []byte("synthetic unrelated content")
	if err := os.WriteFile(sentinelPath, sentinel, 0o600); err != nil {
		t.Fatal(err)
	}

	claim := retentionClaim(dv.PurposeIdentity)
	claim.JobID, claim.FileID, claim.CaseID, claim.SubjectID = jobID, fileID, caseID, subjectID
	jobs := &retentionFake{claim: claim}
	processor := NewRetentionProcessor(jobs, store, time.Minute, 2*time.Second)
	if err := processor.ProcessOne(ctx); err != nil {
		t.Fatalf("ProcessOne() error=%v; want fake to persist retry outcome", err)
	}
	if jobs.complete != 0 || jobs.retry != 1 || jobs.retryCode != retentionStoreError || jobs.retryDelay != 2*time.Second {
		t.Fatalf("fake completion=%d retry=%d code=%q delay=%s; want no completion and allowlisted retry", jobs.complete, jobs.retry, jobs.retryCode, jobs.retryDelay)
	}
	got, err := os.ReadFile(sentinelPath)
	if err != nil || string(got) != string(sentinel) {
		t.Fatalf("directory sentinel=%q err=%v; want preserved bytes", got, err)
	}
}
