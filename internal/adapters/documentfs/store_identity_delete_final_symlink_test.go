package documentfs

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestPrivateStoreIdentityDeleteRemovesFinalSymlinkWithoutFollowingTarget(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "private-documents")
	store, err := New(root, testEncryptionKey())
	if err != nil {
		t.Fatal(err)
	}
	const (
		owner  = "11111111-1111-4111-8111-111111111111"
		caseID = "22222222-2222-4222-8222-222222222222"
		fileID = "33333333-3333-4333-8333-333333333333"
	)
	ctx := context.Background()
	if err := store.PutIdentityDocument(ctx, owner, caseID, fileID, []byte("synthetic identity")); err != nil {
		t.Fatal(err)
	}
	objectPath := filepath.Join(root, "users", owner, "identity", caseID, fileID+".bin")
	if err := os.Remove(objectPath); err != nil {
		t.Fatal(err)
	}
	targetPath := filepath.Join(parent, "outside-target.bin")
	targetBytes := []byte("synthetic external sentinel")
	if err := os.WriteFile(targetPath, targetBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(targetPath, objectPath); err != nil {
		t.Fatal(err)
	}

	if err := store.DeleteIdentityDocument(ctx, owner, caseID, fileID); err != nil {
		t.Fatalf("DeleteIdentityDocument() error=%v; want symlink entry removal", err)
	}
	if _, err := os.Lstat(objectPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("identity symlink at %q error=%v; want absent", objectPath, err)
	}
	got, err := os.ReadFile(targetPath)
	if err != nil || !bytes.Equal(got, targetBytes) {
		t.Fatalf("symlink target content=%q err=%v; want unchanged sentinel", got, err)
	}
}
