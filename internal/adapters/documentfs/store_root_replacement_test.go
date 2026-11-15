package documentfs

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestPrivateStoreRootReplacementRejectsAccessWithoutMutatingEitherTree(t *testing.T) {
	for _, replacement := range []string{"symlink", "directory"} {
		t.Run(replacement, func(t *testing.T) {
			parent := t.TempDir()
			root := filepath.Join(parent, "private-documents")
			target := filepath.Join(parent, "replacement-target")
			store, err := New(root, testEncryptionKey())
			if err != nil {
				t.Fatal(err)
			}
			originalContent := []byte("original synthetic identity")
			if err := store.PutIdentityDocument(context.Background(), ownerID, caseID, fileID, originalContent); err != nil {
				t.Fatal(err)
			}
			targetStore, err := New(target, testEncryptionKey())
			if err != nil {
				t.Fatal(err)
			}
			targetContent := []byte("replacement synthetic identity")
			if err := targetStore.PutIdentityDocument(context.Background(), ownerID, caseID, fileID, targetContent); err != nil {
				t.Fatal(err)
			}

			originalObject := filepath.Join(root, identityPath(ownerID, caseID, fileID))
			targetObject := filepath.Join(target, identityPath(ownerID, caseID, fileID))
			originalBefore, err := os.ReadFile(originalObject)
			if err != nil {
				t.Fatal(err)
			}
			targetBefore, err := os.ReadFile(targetObject)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(root, root+"-original"); err != nil {
				t.Fatal(err)
			}
			if replacement == "symlink" {
				if err := os.Symlink(target, root); err != nil {
					t.Fatal(err)
				}
			} else if err := os.Rename(target, root); err != nil {
				t.Fatal(err)
			}

			if got, err := store.ReadIdentityDocument(context.Background(), ownerID, caseID, fileID); !errors.Is(err, ErrStorage) || got != nil {
				t.Fatalf("read through replaced root returned %q, %v; want no bytes and ErrStorage", got, err)
			}
			if err := store.DeleteIdentityDocument(context.Background(), ownerID, caseID, fileID); !errors.Is(err, ErrStorage) {
				t.Fatalf("delete through replaced root error=%v, want ErrStorage", err)
			}

			originalAfter, err := os.ReadFile(filepath.Join(root+"-original", identityPath(ownerID, caseID, fileID)))
			if err != nil || !bytes.Equal(originalAfter, originalBefore) {
				t.Fatalf("original root object changed: equal=%t err=%v", bytes.Equal(originalAfter, originalBefore), err)
			}
			replacementAfterPath := targetObject
			if replacement == "directory" {
				replacementAfterPath = filepath.Join(root, identityPath(ownerID, caseID, fileID))
			}
			targetAfter, err := os.ReadFile(replacementAfterPath)
			if err != nil || !bytes.Equal(targetAfter, targetBefore) {
				t.Fatalf("replacement target object changed: equal=%t err=%v", bytes.Equal(targetAfter, targetBefore), err)
			}
		})
	}
}
