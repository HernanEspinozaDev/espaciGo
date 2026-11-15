package documentverificationpg

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/documentfs"
	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
)

func TestRetentionProcessorDeletesCorruptIdentityEnvelope(t *testing.T) {
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
	corruptEnvelope, err := os.ReadFile(objectPath)
	if err != nil {
		t.Fatal(err)
	}
	corruptEnvelope[len(corruptEnvelope)-1] ^= 0xff
	if err := os.WriteFile(objectPath, corruptEnvelope, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ReadIdentityDocument(ctx, subjectID, caseID, fileID); !errors.Is(err, documentfs.ErrStorage) {
		t.Fatalf("corrupt identity read error=%v; want ErrStorage", err)
	}

	claim := retentionClaim(dv.PurposeIdentity)
	claim.JobID, claim.FileID, claim.CaseID, claim.SubjectID = jobID, fileID, caseID, subjectID
	jobs := &retentionFake{claim: claim}
	processor := NewRetentionProcessor(jobs, store, time.Minute, time.Second)
	if err := processor.ProcessOne(ctx); err != nil {
		t.Fatalf("ProcessOne() error=%v; want corrupt envelope cleanup", err)
	}
	if jobs.complete != 1 || jobs.retry != 0 {
		t.Fatalf("fake completion=%d retry=%d; want completed cleanup", jobs.complete, jobs.retry)
	}
	if _, err := os.Lstat(objectPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("corrupt object path error=%v; want absent", err)
	}
}
