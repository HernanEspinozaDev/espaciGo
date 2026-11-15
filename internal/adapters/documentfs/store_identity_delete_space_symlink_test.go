package documentfs

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestPrivateStoreIdentityDeleteDoesNotFollowSymlinkIntoSpaceObject(t *testing.T) {
	root := filepath.Join(t.TempDir(), "private-documents")
	store, err := New(root, make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	const (
		fileID    = "22222222-2222-4222-8222-222222222222"
		caseID    = "33333333-3333-4333-8333-333333333333"
		subjectID = "44444444-4444-4444-8444-444444444444"
		spaceID   = "55555555-5555-4555-8555-555555555555"
	)
	ctx := context.Background()
	spaceBytes := []byte("synthetic space evidence")
	if err := store.PutSpaceDocument(ctx, subjectID, spaceID, caseID, fileID, spaceBytes); err != nil {
		t.Fatal(err)
	}
	if err := store.PutIdentityDocument(ctx, subjectID, caseID, fileID, []byte("synthetic identity evidence")); err != nil {
		t.Fatal(err)
	}
	identityPath := filepath.Join(root, "users", subjectID, "identity", caseID, fileID+".bin")
	spacePath := filepath.Join(root, "users", subjectID, "spaces", spaceID, "documents", caseID, fileID+".bin")
	if err := os.Remove(identityPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(spacePath, identityPath); err != nil {
		t.Fatal(err)
	}

	if err := store.DeleteIdentityDocument(ctx, subjectID, caseID, fileID); err != nil {
		t.Fatalf("DeleteIdentityDocument() error=%v; want unlink of final symlink", err)
	}
	if _, err := os.Lstat(identityPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("identity symlink path error=%v; want absent", err)
	}
	got, err := store.ReadSpaceDocument(ctx, subjectID, spaceID, caseID, fileID)
	if err != nil || !bytes.Equal(got, spaceBytes) {
		t.Fatalf("space target after identity cleanup=%q err=%v; want unchanged", got, err)
	}
}
