package documentfs

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestPrivateStoreIdentityDeleteDoesNotRecursivelyRemoveDirectory(t *testing.T) {
	root := filepath.Join(t.TempDir(), "private-documents")
	store, err := New(root, make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	const (
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

	if err := store.DeleteIdentityDocument(ctx, subjectID, caseID, fileID); !errors.Is(err, ErrStorage) {
		t.Fatalf("DeleteIdentityDocument() error=%v; want ErrStorage for non-empty directory", err)
	}
	got, err := os.ReadFile(sentinelPath)
	if err != nil || string(got) != string(sentinel) {
		t.Fatalf("directory sentinel=%q err=%v; want preserved bytes", got, err)
	}
	if info, err := os.Lstat(objectPath); err != nil || !info.IsDir() {
		t.Fatalf("object path info=%v err=%v; want directory preserved", info, err)
	}
}
