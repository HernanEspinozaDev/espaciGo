package documentfs

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"os"
	"path/filepath"
	"testing"
)

func TestFSReadsLegacyEnvelopeAndPreservesKeyCopy(t *testing.T) {
	root := filepath.Join(t.TempDir(), "private")
	key := testEncryptionKey()
	original := append([]byte(nil), key...)
	store, err := New(root, key)
	if err != nil {
		t.Fatal(err)
	}
	for i := range key {
		key[i] = 0
	}
	relative := filepath.Join("users", ownerID, "identity", caseID, fileID+".bin")
	block, err := aes.NewCipher(original)
	if err != nil {
		t.Fatal(err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	nonce := bytes.Repeat([]byte{7}, 12)
	envelope := append([]byte("ESPACIGODOC1"), nonce...)
	plain := []byte("independent legacy synthetic identity")
	envelope = aead.Seal(envelope, nonce, plain, []byte(relative))
	target := filepath.Join(root, relative)
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, envelope, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := store.ReadIdentityDocument(context.Background(), ownerID, caseID, fileID)
	if err != nil || !bytes.Equal(got, plain) {
		t.Fatalf("legacy FS read failed: %v", err)
	}
	second := []byte("new synthetic space")
	if err := store.PutSpaceDocument(context.Background(), ownerID, spaceID, caseID, fileID, second); err != nil {
		t.Fatal(err)
	}
	relative = filepath.Join("users", ownerID, "spaces", spaceID, "documents", caseID, fileID+".bin")
	raw, err := os.ReadFile(filepath.Join(root, relative))
	if err != nil {
		t.Fatal(err)
	}
	magic := []byte("ESPACIGODOC1")
	if !bytes.HasPrefix(raw, magic) || len(raw) <= len(magic)+12 {
		t.Fatal("legacy envelope changed")
	}
	got, err = aead.Open(nil, raw[len(magic):len(magic)+12], raw[len(magic)+12:], []byte(relative))
	if err != nil || !bytes.Equal(got, second) {
		t.Fatalf("independent legacy read of new FS data failed: %v", err)
	}
	if err := filepath.WalkDir(root, func(p string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		info, e := d.Info()
		if e != nil {
			return e
		}
		want := os.FileMode(0o600)
		if d.IsDir() {
			want = 0o700
		}
		if info.Mode().Perm() != want {
			t.Errorf("private permissions mismatch: %s %o", p, info.Mode().Perm())
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
