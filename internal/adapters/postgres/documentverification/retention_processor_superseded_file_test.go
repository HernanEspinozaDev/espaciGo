package documentverificationpg

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/documentfs"
	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
)

func TestRetentionProcessorDeletesSupersededIdentityFileAndPreservesCurrent(t *testing.T) {
	root := filepath.Join(t.TempDir(), "private-documents")
	store, err := documentfs.New(root, make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	const (
		jobID         = "11111111-1111-4111-8111-111111111111"
		subjectID     = "22222222-2222-4222-8222-222222222222"
		caseID        = "33333333-3333-4333-8333-333333333333"
		supersededID  = "44444444-4444-4444-8444-444444444444"
		currentFileID = "55555555-5555-4555-8555-555555555555"
	)
	ctx := context.Background()
	superseded := []byte("synthetic superseded identity evidence")
	current := []byte("synthetic current identity evidence")
	if err := store.PutIdentityDocument(ctx, subjectID, caseID, supersededID, superseded); err != nil {
		t.Fatal(err)
	}
	if err := store.PutIdentityDocument(ctx, subjectID, caseID, currentFileID, current); err != nil {
		t.Fatal(err)
	}
	claim := retentionClaim(dv.PurposeIdentity)
	claim.JobID, claim.FileID, claim.CaseID, claim.SubjectID = jobID, supersededID, caseID, subjectID
	jobs := &retentionFake{claim: claim}
	processor := NewRetentionProcessor(jobs, store, time.Minute, time.Second)

	if err := processor.ProcessOne(ctx); err != nil {
		t.Fatal(err)
	}
	if jobs.complete != 1 || jobs.retry != 0 {
		t.Fatalf("fake completion=%d retry=%d; want completed cleanup", jobs.complete, jobs.retry)
	}
	supersededPath := filepath.Join(root, "users", subjectID, "identity", caseID, supersededID+".bin")
	if _, err := os.Lstat(supersededPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("superseded object at %q error=%v; want absent", supersededPath, err)
	}
	currentPath := filepath.Join(root, "users", subjectID, "identity", caseID, currentFileID+".bin")
	if _, err := os.Lstat(currentPath); err != nil {
		t.Fatalf("current object at %q is not present: %v", currentPath, err)
	}
	got, err := store.ReadIdentityDocument(ctx, subjectID, caseID, currentFileID)
	if err != nil || !bytes.Equal(got, current) {
		t.Fatalf("current identity read=%q err=%v; want unchanged current evidence", got, err)
	}
}
