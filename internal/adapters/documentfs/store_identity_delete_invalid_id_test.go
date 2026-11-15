package documentfs

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestPrivateStoreIdentityDeleteRejectsInvalidIDsWithoutTouchingValidObject(t *testing.T) {
	root := filepath.Join(t.TempDir(), "private-documents")
	store, err := New(root, testEncryptionKey())
	if err != nil {
		t.Fatal(err)
	}
	content := []byte("synthetic retained identity evidence")
	if err := store.PutIdentityDocument(context.Background(), ownerID, caseID, fileID, content); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, owner, caseID, fileID string
	}{
		{name: "owner", owner: "../escape", caseID: caseID, fileID: fileID},
		{name: "case", owner: ownerID, caseID: "../escape", fileID: fileID},
		{name: "file", owner: ownerID, caseID: caseID, fileID: "../escape"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := store.DeleteIdentityDocument(context.Background(), tc.owner, tc.caseID, tc.fileID); !errors.Is(err, ErrInvalid) {
				t.Fatalf("DeleteIdentityDocument() error=%v; want ErrInvalid", err)
			}
			got, err := store.ReadIdentityDocument(context.Background(), ownerID, caseID, fileID)
			if err != nil || !bytes.Equal(got, content) {
				t.Fatalf("valid identity object after rejected delete=%q err=%v; want unchanged", got, err)
			}
		})
	}
	validPath := filepath.Join(root, "users", ownerID, "identity", caseID, fileID+".bin")
	if _, err := os.Stat(validPath); err != nil {
		t.Fatalf("valid identity object path is missing after rejected deletes: %v", err)
	}
}
