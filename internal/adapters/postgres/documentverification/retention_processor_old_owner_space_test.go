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

func TestRetentionProcessorDeletesOldOwnerSpaceCopyAndPreservesCurrentOwnerCopy(t *testing.T) {
	root := filepath.Join(t.TempDir(), "private-documents")
	store, err := documentfs.New(root, make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	const (
		jobID         = "11111111-1111-4111-8111-111111111111"
		spaceID       = "22222222-2222-4222-8222-222222222222"
		caseID        = "33333333-3333-4333-8333-333333333333"
		oldOwnerID    = "44444444-4444-4444-8444-444444444444"
		newOwnerID    = "55555555-5555-4555-8555-555555555555"
		oldFileID     = "66666666-6666-4666-8666-666666666666"
		currentFileID = "77777777-7777-4777-8777-777777777777"
	)
	ctx := context.Background()
	oldBytes := []byte("synthetic superseded space evidence")
	currentBytes := []byte("synthetic current owner space evidence")
	if err := store.PutSpaceDocument(ctx, oldOwnerID, spaceID, caseID, oldFileID, oldBytes); err != nil {
		t.Fatal(err)
	}
	if err := store.PutSpaceDocument(ctx, newOwnerID, spaceID, caseID, currentFileID, currentBytes); err != nil {
		t.Fatal(err)
	}

	claim := retentionClaim(dv.PurposeSpace)
	claim.JobID, claim.SubjectID, claim.SpaceID, claim.CaseID, claim.FileID = jobID, oldOwnerID, spaceID, caseID, oldFileID
	jobs := &retentionFake{claim: claim}
	processor := NewRetentionProcessor(jobs, store, time.Minute, time.Second)
	if err := processor.ProcessOne(ctx); err != nil {
		t.Fatalf("ProcessOne() error=%v; want old-owner cleanup", err)
	}
	if jobs.complete != 1 || jobs.retry != 0 {
		t.Fatalf("fake completion=%d retry=%d; want completed cleanup", jobs.complete, jobs.retry)
	}
	oldPath := filepath.Join(root, "users", oldOwnerID, "spaces", spaceID, "documents", caseID, oldFileID+".bin")
	if _, err := os.Lstat(oldPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("old-owner object path error=%v; want absent", err)
	}
	currentPath := filepath.Join(root, "users", newOwnerID, "spaces", spaceID, "documents", caseID, currentFileID+".bin")
	if _, err := os.Lstat(currentPath); err != nil {
		t.Fatalf("current-owner object path error=%v; want present", err)
	}
	got, err := store.ReadSpaceDocument(ctx, newOwnerID, spaceID, caseID, currentFileID)
	if err != nil || !bytes.Equal(got, currentBytes) {
		t.Fatalf("current-owner read=%q err=%v; want unchanged", got, err)
	}
}
