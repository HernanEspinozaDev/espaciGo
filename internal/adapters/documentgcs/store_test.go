package documentgcs

import (
	"bytes"
	"context"
	"testing"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/cloudstorage"
)

var _ ObjectStore = (*cloudstorage.Store)(nil)

func TestConstructorAndKeyValidation(t *testing.T) {
	if _, err := New(nil, nil, nil); err == nil {
		t.Fatal("empty configuration accepted")
	}
	if _, err := New(nil, nil, map[string][]byte{"v1": make([]byte, 31)}); err == nil {
		t.Fatal("invalid key accepted")
	}
	if _, err := New(newMemoryStore(), newMemoryStore(), map[string][]byte{"v1": make([]byte, 32), "v2": make([]byte, 32)}, WithCurrentKeyVersion("v2")); err == nil {
		t.Fatal("duplicate key material under distinct versions accepted")
	}
}

func TestEncryptedIdentityAndSpaceRoundTrip(t *testing.T) {
	ctx := context.Background()
	identity, space := newMemoryStore(), newMemoryStore()
	s, err := New(identity, space, map[string][]byte{"v1": make([]byte, 32)}, WithCurrentKeyVersion("v1"))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.PutIdentityDocument(ctx, ownerID, caseID, fileID, []byte("private identity")); err != nil {
		t.Fatal(err)
	}
	if err := s.PutSpaceDocument(ctx, ownerID, spaceID, caseID, fileID, []byte("private space")); err != nil {
		t.Fatal(err)
	}
	if _, ok := identity.objects[fileID]; !ok {
		t.Fatal("identity object key must be the opaque file UUID, not the scope path")
	}
	if bytes.Contains(identity.objects[fileID].blob, []byte("private identity")) {
		t.Fatal("plaintext stored")
	}
	if _, ok := space.objects[fileID]; !ok {
		t.Fatal("space object not in distinct store")
	}
	if got, err := s.ReadIdentityDocument(ctx, ownerID, caseID, fileID); err != nil || string(got) != "private identity" {
		t.Fatalf("identity read %q %v", got, err)
	}
	if got, err := s.ReadSpaceDocument(ctx, ownerID, spaceID, caseID, fileID); err != nil || string(got) != "private space" {
		t.Fatalf("space read %q %v", got, err)
	}
	if _, err := s.ReadIdentityDocument(ctx, ownerID, caseID, otherFileID); err == nil {
		t.Fatal("AAD/object mismatch read succeeded")
	}
	if _, err := s.ReadIdentityDocument(ctx, otherOwnerID, caseID, fileID); err == nil {
		t.Fatal("identity ciphertext decrypted under another owner's AAD")
	}
	if _, err := s.ReadSpaceDocument(ctx, ownerID, otherSpaceID, caseID, fileID); err == nil {
		t.Fatal("space ciphertext decrypted under another space's AAD")
	}
	identity.objects[fileID] = space.objects[fileID]
	if _, err := s.ReadIdentityDocument(ctx, ownerID, caseID, fileID); err == nil {
		t.Fatal("space ciphertext decrypted under identity-purpose AAD")
	}
}

func TestRotationAndFailClosedVersions(t *testing.T) {
	ctx := context.Background()
	oldKey, newKey := make([]byte, 32), make([]byte, 32)
	newKey[0] = 1
	old, _ := New(newMemoryStore(), newMemoryStore(), map[string][]byte{"v1": oldKey}, WithCurrentKeyVersion("v1"))
	if err := old.PutIdentityDocument(ctx, ownerID, caseID, fileID, []byte("doc")); err != nil {
		t.Fatal(err)
	}
	store := old.identity.(*memoryStore)
	rotated, _ := New(store, newMemoryStore(), map[string][]byte{"v1": oldKey, "v2": newKey}, WithCurrentKeyVersion("v2"))
	if got, err := rotated.ReadIdentityDocument(ctx, ownerID, caseID, fileID); err != nil || string(got) != "doc" {
		t.Fatalf("old-key read %q %v", got, err)
	}
	if err := rotated.PutIdentityDocument(ctx, ownerID, caseID, otherFileID, []byte("new")); err != nil {
		t.Fatal(err)
	}
	if store.objects[otherFileID].metadata[keyVersionMetadata] != "v2" {
		t.Fatal("current key version missing")
	}
	key := fileID
	store.objects[key].metadata[keyVersionMetadata] = "v2"
	if _, err := rotated.ReadIdentityDocument(ctx, ownerID, caseID, fileID); err == nil {
		t.Fatal("tampered metadata selected another valid key version")
	}
	store.objects[key].metadata[keyVersionMetadata] = "bad/version"
	if _, err := rotated.ReadIdentityDocument(ctx, ownerID, caseID, fileID); err == nil {
		t.Fatal("invalid version accepted")
	}
	store.objects[key].metadata[keyVersionMetadata] = "unknown"
	if _, err := rotated.ReadIdentityDocument(ctx, ownerID, caseID, fileID); err == nil {
		t.Fatal("unknown version accepted")
	}
}

func TestSizeDeleteAndMissingRead(t *testing.T) {
	ctx := context.Background()
	raw := make([]byte, 10<<20)
	raw[0] = 1
	s, _ := New(newMemoryStore(), newMemoryStore(), map[string][]byte{"v1": make([]byte, 32)}, WithCurrentKeyVersion("v1"))
	if err := s.PutIdentityDocument(ctx, ownerID, caseID, fileID, raw); err != nil {
		t.Fatal(err)
	}
	if err := s.PutIdentityDocument(ctx, ownerID, caseID, otherFileID, append(raw, 1)); err == nil {
		t.Fatal("oversize write accepted")
	}
	if err := s.DeleteIdentityDocument(ctx, ownerID, caseID, fileID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ReadIdentityDocument(ctx, ownerID, caseID, fileID); err == nil {
		t.Fatal("deleted document read")
	}
	if err := s.DeleteIdentityDocument(ctx, ownerID, caseID, fileID); err != nil {
		t.Fatalf("idempotent delete: %v", err)
	}
}

func TestCloseReleasesBothPurposeStores(t *testing.T) {
	identity := &closeTrackingStore{memoryStore: newMemoryStore()}
	space := &closeTrackingStore{memoryStore: newMemoryStore()}
	store, err := New(identity, space, map[string][]byte{"v1": make([]byte, 32)})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if identity.closeCalls != 1 || space.closeCalls != 1 {
		t.Fatalf("closed identity=%d space=%d, want once each", identity.closeCalls, space.closeCalls)
	}
}

type closeTrackingStore struct {
	*memoryStore
	closeCalls int
}

func (s *closeTrackingStore) Close() error {
	s.closeCalls++
	return nil
}

const ownerID = "11111111-1111-4111-8111-111111111111"
const spaceID = "22222222-2222-4222-8222-222222222222"
const caseID = "33333333-3333-4333-8333-333333333333"
const fileID = "44444444-4444-4444-8444-444444444444"
const otherFileID = "55555555-5555-4555-8555-555555555555"
const otherOwnerID = "66666666-6666-4666-8666-666666666666"
const otherSpaceID = "77777777-7777-4777-8777-777777777777"
