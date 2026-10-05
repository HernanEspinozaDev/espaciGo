package evidencefs

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestPrivateSyntheticBlobLifecycleAndIdentifierValidation(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	store, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	id := "11111111-1111-4111-8111-111111111111"
	data := []byte("synthetic fixture only")
	if err := store.Put(context.Background(), id, data); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, id+".png")
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("blob mode info=%v err=%v", info, err)
	}
	got, err := store.Get(context.Background(), id)
	if err != nil || string(got) != string(data) {
		t.Fatalf("read=%q err=%v", got, err)
	}
	if err := store.Put(context.Background(), "../../public/evil", data); err == nil {
		t.Fatal("accepted path traversal identifier")
	}
	if err := store.Delete(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("blob remains after explicit delete: %v", err)
	}
}

func TestStoreRejectsPubliclyReadableDirectory(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := New(root); err == nil {
		t.Fatal("accepted non-private evidence directory")
	}
}
