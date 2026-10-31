package documentfs

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

const (
	ownerID = "11111111-1111-4111-8111-111111111111"
	otherID = "22222222-2222-4222-8222-222222222222"
	spaceID = "33333333-3333-4333-8333-333333333333"
	caseID  = "44444444-4444-4444-8444-444444444444"
	fileID  = "55555555-5555-4555-8555-555555555555"
)

func TestPrivateStoreReadsSyntheticSpaceDocumentAfterReopen(t *testing.T) {
	root := filepath.Join(t.TempDir(), "private-documents")
	first, err := New(root, testEncryptionKey())
	if err != nil {
		t.Fatal(err)
	}
	content := []byte("synthetic PDF bytes")
	if err := first.PutSpaceDocument(context.Background(), ownerID, spaceID, caseID, fileID, content); err != nil {
		t.Fatal(err)
	}
	second, err := New(root, testEncryptionKey())
	if err != nil {
		t.Fatal(err)
	}
	got, err := second.ReadSpaceDocument(context.Background(), ownerID, spaceID, caseID, fileID)
	if err != nil || !bytes.Equal(got, content) {
		t.Fatalf("read after reopen = %q, %v", got, err)
	}
	info, err := os.Stat(root)
	if err != nil || info.Mode().Perm() != 0o700 {
		t.Fatalf("root permissions=%v, err=%v", info, err)
	}
}

func TestPrivateStoreSeparatesIdentityAndSpacePurpose(t *testing.T) {
	store, err := New(filepath.Join(t.TempDir(), "private-documents"), testEncryptionKey())
	if err != nil {
		t.Fatal(err)
	}
	identityBytes := []byte("identity synthetic")
	spaceBytes := []byte("space synthetic")
	if err := store.PutIdentityDocument(context.Background(), ownerID, caseID, fileID, identityBytes); err != nil {
		t.Fatal(err)
	}
	if err := store.PutSpaceDocument(context.Background(), ownerID, spaceID, caseID, fileID, spaceBytes); err != nil {
		t.Fatal(err)
	}
	identityGot, err := store.ReadIdentityDocument(context.Background(), ownerID, caseID, fileID)
	if err != nil || !bytes.Equal(identityGot, identityBytes) {
		t.Fatalf("identity read=%q err=%v", identityGot, err)
	}
	spaceGot, err := store.ReadSpaceDocument(context.Background(), ownerID, spaceID, caseID, fileID)
	if err != nil || !bytes.Equal(spaceGot, spaceBytes) {
		t.Fatalf("space read=%q err=%v", spaceGot, err)
	}
}

func TestPrivateStoreUsesOwnerScopeAndDoesNotOverwrite(t *testing.T) {
	store, err := New(filepath.Join(t.TempDir(), "private-documents"), testEncryptionKey())
	if err != nil {
		t.Fatal(err)
	}
	content := []byte("owner scoped")
	if err := store.PutSpaceDocument(context.Background(), ownerID, spaceID, caseID, fileID, content); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ReadSpaceDocument(context.Background(), otherID, spaceID, caseID, fileID); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("foreign owner read error=%v, want not-exist", err)
	}
	if err := store.PutSpaceDocument(context.Background(), ownerID, spaceID, caseID, fileID, []byte("replacement")); !errors.Is(err, ErrObjectExists) {
		t.Fatalf("duplicate object error=%v, want ErrObjectExists", err)
	}
	got, err := store.ReadSpaceDocument(context.Background(), ownerID, spaceID, caseID, fileID)
	if err != nil || !bytes.Equal(got, content) {
		t.Fatalf("existing object changed: %q, %v", got, err)
	}
}

func TestPrivateStoreRejectsInvalidIDsOversizeAndCancelledContext(t *testing.T) {
	parent := t.TempDir()
	store, err := New(filepath.Join(parent, "private-documents"), testEncryptionKey())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.PutSpaceDocument(context.Background(), "../escape", spaceID, caseID, fileID, []byte("x")); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid ID error=%v", err)
	}
	if err := store.PutIdentityDocument(context.Background(), ownerID, caseID, fileID, make([]byte, MaxDocumentBytes+1)); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("oversize error=%v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := store.PutSpaceDocument(ctx, ownerID, spaceID, caseID, fileID, []byte("x")); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled put error=%v", err)
	}
	if _, err := os.Stat(filepath.Join(parent, "escape")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("invalid path created outside private root")
	}
}

func TestPrivateStoreRejectsSymlinkRootWithoutChangingTargetPermissions(t *testing.T) {
	parent := t.TempDir()
	target := filepath.Join(parent, "target")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(parent, "private-documents")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if _, err := New(link, testEncryptionKey()); !errors.Is(err, ErrStorage) {
		t.Fatalf("New(symlink root) error=%v, want ErrStorage", err)
	}
	info, err := os.Stat(target)
	if err != nil || info.Mode().Perm() != 0o755 {
		t.Fatalf("symlink target permissions=%v err=%v, want 0755", info, err)
	}
}

func TestPrivateStoreEncryptsDocumentsAtRest(t *testing.T) {
	root := filepath.Join(t.TempDir(), "private-documents")
	store, err := New(root, testEncryptionKey())
	if err != nil {
		t.Fatal(err)
	}
	content := []byte("highly synthetic document bytes")
	if err := store.PutSpaceDocument(context.Background(), ownerID, spaceID, caseID, fileID, content); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, spacePath(ownerID, spaceID, caseID, fileID))
	sealed, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(sealed, content) {
		t.Fatal("private document is stored as plaintext")
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("object permissions=%v err=%v, want 0600", info, err)
	}
	if _, err := New(root, []byte("short")); !errors.Is(err, ErrInvalid) {
		t.Fatalf("short encryption key error=%v, want ErrInvalid", err)
	}
}

func testEncryptionKey() []byte { return bytes.Repeat([]byte{0x42}, 32) }
