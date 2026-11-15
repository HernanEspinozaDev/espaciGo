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

func TestRetentionProcessorDeletesCorruptSpaceEnvelopeWithoutDeletingIdentity(t *testing.T) {
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
		spaceID   = "55555555-5555-4555-8555-555555555555"
	)
	ctx := context.Background()
	identityBytes := []byte("synthetic identity evidence")
	if err := store.PutIdentityDocument(ctx, subjectID, caseID, fileID, identityBytes); err != nil {
		t.Fatal(err)
	}
	if err := store.PutSpaceDocument(ctx, subjectID, spaceID, caseID, fileID, []byte("synthetic space evidence")); err != nil {
		t.Fatal(err)
	}
	spacePath := filepath.Join(root, "users", subjectID, "spaces", spaceID, "documents", caseID, fileID+".bin")
	corruptEnvelope, err := os.ReadFile(spacePath)
	if err != nil {
		t.Fatal(err)
	}
	corruptEnvelope[len(corruptEnvelope)-1] ^= 0xff
	if err := os.WriteFile(spacePath, corruptEnvelope, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ReadSpaceDocument(ctx, subjectID, spaceID, caseID, fileID); !errors.Is(err, documentfs.ErrStorage) {
		t.Fatalf("corrupt space read error=%v; want ErrStorage", err)
	}

	claim := retentionClaim(dv.PurposeSpace)
	claim.JobID, claim.FileID, claim.CaseID, claim.SubjectID, claim.SpaceID = jobID, fileID, caseID, subjectID, spaceID
	jobs := &retentionFake{claim: claim}
	processor := NewRetentionProcessor(jobs, store, time.Minute, time.Second)
	if err := processor.ProcessOne(ctx); err != nil {
		t.Fatalf("ProcessOne() error=%v; want corrupt space envelope cleanup", err)
	}
	if jobs.complete != 1 || jobs.retry != 0 {
		t.Fatalf("fake completion=%d retry=%d; want completed cleanup", jobs.complete, jobs.retry)
	}
	if _, err := os.Lstat(spacePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("corrupt space object path error=%v; want absent", err)
	}
	got, err := store.ReadIdentityDocument(ctx, subjectID, caseID, fileID)
	if err != nil || !bytes.Equal(got, identityBytes) {
		t.Fatalf("identity sibling after space cleanup=%q err=%v; want unchanged", got, err)
	}
}
