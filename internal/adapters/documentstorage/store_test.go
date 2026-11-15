package documentstorage

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/cloudstorage"
	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/documentcodec"
)

const (
	testOwnerID = "11111111-1111-4111-8111-111111111111"
	testSpaceID = "22222222-2222-4222-8222-222222222222"
	testCaseID  = "33333333-3333-4333-8333-333333333333"
	testFileID  = "44444444-4444-4444-8444-444444444444"
)

var testKey = []byte("0123456789abcdef0123456789abcdef")

var _ BlobStore = (*cloudstorage.Store)(nil)

type memoryBlobs struct {
	objects      map[string][]byte
	getErr       error
	getHook      func()
	putErr       error
	putHook      func()
	delErr       error
	delHook      func()
	lastPutID    string
	lastDeleteID string
	puts         int
	gets         int
	dels         int
}

func (m *memoryBlobs) Put(_ context.Context, id string, blob []byte) error {
	m.puts++
	if m.putHook != nil {
		m.putHook()
	}
	if m.putErr != nil {
		return m.putErr
	}
	if m.objects == nil {
		m.objects = make(map[string][]byte)
	}
	if _, exists := m.objects[id]; exists {
		return errors.New("object already exists")
	}
	m.lastPutID = id
	m.objects[id] = append([]byte(nil), blob...)
	return nil
}

func (m *memoryBlobs) Get(_ context.Context, id string) ([]byte, error) {
	m.gets++
	if m.getHook != nil {
		m.getHook()
	}
	if m.getErr != nil {
		return nil, m.getErr
	}
	blob, ok := m.objects[id]
	if !ok {
		return nil, errors.New("object missing")
	}
	return append([]byte(nil), blob...), nil
}

func (m *memoryBlobs) Delete(_ context.Context, id string) error {
	m.dels++
	m.lastDeleteID = id
	if m.delHook != nil {
		m.delHook()
	}
	if m.delErr != nil {
		return m.delErr
	}
	delete(m.objects, id)
	return nil
}

func TestStoreEncryptsIdentityBeforePutAndRoundTrips(t *testing.T) {
	blobs := &memoryBlobs{}
	store, err := New(blobs, testKey)
	if err != nil {
		t.Fatal(err)
	}
	plaintext := []byte("synthetic private identity image")
	if err := store.PutIdentityDocument(context.Background(), testOwnerID, testCaseID, testFileID, plaintext); err != nil {
		t.Fatal(err)
	}
	envelope := blobs.objects[blobs.lastPutID]
	if bytes.Equal(envelope, plaintext) || len(envelope) <= len(plaintext) {
		t.Fatalf("blob store received plaintext or an invalid envelope: %x", envelope)
	}
	scope, err := documentcodec.IdentityScope(testOwnerID, testCaseID, testFileID)
	if err != nil {
		t.Fatal(err)
	}
	opened, err := documentcodec.Open(testKey, scope, envelope)
	if err != nil || !bytes.Equal(opened, plaintext) {
		t.Fatalf("stored envelope does not open with identity scope: %q, %v", opened, err)
	}
	got, err := store.ReadIdentityDocument(context.Background(), testOwnerID, testCaseID, testFileID)
	if err != nil || !bytes.Equal(got, plaintext) {
		t.Fatalf("identity round trip=%q, %v", got, err)
	}
}

func TestStoreDeleteRequiresExactIdentityScope(t *testing.T) {
	blobs := &memoryBlobs{}
	store, err := New(blobs, testKey)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.PutIdentityDocument(context.Background(), testOwnerID, testCaseID, testFileID, []byte("synthetic identity evidence")); err != nil {
		t.Fatal(err)
	}
	objectID := blobs.lastPutID
	for _, mismatch := range []struct {
		name, ownerID, caseID, fileID string
	}{
		{name: "owner", ownerID: "55555555-5555-4555-8555-555555555555", caseID: testCaseID, fileID: testFileID},
		{name: "case", ownerID: testOwnerID, caseID: "66666666-6666-4666-8666-666666666666", fileID: testFileID},
		{name: "file", ownerID: testOwnerID, caseID: testCaseID, fileID: "77777777-7777-4777-8777-777777777777"},
	} {
		if err := store.DeleteIdentityDocument(context.Background(), mismatch.ownerID, mismatch.caseID, mismatch.fileID); err != nil {
			t.Fatalf("delete with mismatched %s returned error: %v", mismatch.name, err)
		}
		if _, exists := blobs.objects[objectID]; !exists {
			t.Fatalf("delete with mismatched %s removed the identity object", mismatch.name)
		}
	}
	if err := store.DeleteIdentityDocument(context.Background(), testOwnerID, testCaseID, testFileID); err != nil {
		t.Fatal(err)
	}
	if _, exists := blobs.objects[objectID]; exists {
		t.Fatal("exact-scope delete did not remove the object")
	}
	if blobs.lastDeleteID == "" || blobs.lastDeleteID == testFileID {
		t.Fatalf("blob object key was not scoped: %q", blobs.lastDeleteID)
	}
}

func TestStoreEncryptsReadsAndDeletesSpaceDocumentInItsScope(t *testing.T) {
	blobs := &memoryBlobs{}
	store, err := New(blobs, testKey)
	if err != nil {
		t.Fatal(err)
	}
	plaintext := []byte("synthetic private space document")
	if err := store.PutSpaceDocument(context.Background(), testOwnerID, testSpaceID, testCaseID, testFileID, plaintext); err != nil {
		t.Fatal(err)
	}
	objectID := blobs.lastPutID
	if objectID == "" || objectID == testFileID {
		t.Fatalf("space object key is not scoped and opaque: %q", objectID)
	}
	if bytes.Equal(blobs.objects[objectID], plaintext) {
		t.Fatal("blob store received space-document plaintext")
	}
	got, err := store.ReadSpaceDocument(context.Background(), testOwnerID, testSpaceID, testCaseID, testFileID)
	if err != nil || !bytes.Equal(got, plaintext) {
		t.Fatalf("space round trip=%q, %v", got, err)
	}
	if err := store.DeleteSpaceDocument(context.Background(), testOwnerID, testSpaceID, testCaseID, testFileID); err != nil {
		t.Fatal(err)
	}
	if _, exists := blobs.objects[objectID]; exists {
		t.Fatal("space-document delete did not remove the scoped object")
	}
}

func TestStoreRejectsIdentityEnvelopeUnderDifferentOwnerOrPurpose(t *testing.T) {
	blobs := &memoryBlobs{}
	store, err := New(blobs, testKey)
	if err != nil {
		t.Fatal(err)
	}
	plaintext := []byte("synthetic private identity image")
	if err := store.PutIdentityDocument(context.Background(), testOwnerID, testCaseID, testFileID, plaintext); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		read func() ([]byte, error)
	}{
		{name: "different owner", read: func() ([]byte, error) {
			return store.ReadIdentityDocument(context.Background(), "55555555-5555-4555-8555-555555555555", testCaseID, testFileID)
		}},
		{name: "space purpose", read: func() ([]byte, error) {
			return store.ReadSpaceDocument(context.Background(), testOwnerID, testSpaceID, testCaseID, testFileID)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.read()
			if !errors.Is(err, ErrStorage) || len(got) != 0 {
				t.Fatalf("cross-scope read=%q, %v; want empty content and ErrStorage", got, err)
			}
		})
	}
}

func TestStoreReturnsNoPlaintextForCorruptEnvelopeOrBlobReadFailure(t *testing.T) {
	for _, tc := range []struct {
		name        string
		corrupt     bool
		readFailure bool
	}{
		{name: "corrupt envelope", corrupt: true},
		{name: "blob read failure", readFailure: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			blobs := &memoryBlobs{}
			store, err := New(blobs, testKey)
			if err != nil {
				t.Fatal(err)
			}
			if tc.corrupt {
				if err := store.PutIdentityDocument(context.Background(), testOwnerID, testCaseID, testFileID, []byte("synthetic private identity image")); err != nil {
					t.Fatal(err)
				}
				blob := blobs.objects[blobs.lastPutID]
				blob[len(blob)-1] ^= 1
			}
			if tc.readFailure {
				blobs.getErr = errors.New("synthetic private backend failure")
			}
			got, err := store.ReadIdentityDocument(context.Background(), testOwnerID, testCaseID, testFileID)
			if !errors.Is(err, ErrStorage) || len(got) != 0 {
				t.Fatalf("read returned %q, %v; want empty content and ErrStorage", got, err)
			}
		})
	}
}

func TestStorePutCancellationAndBackendFailureAreFailClosed(t *testing.T) {
	t.Run("canceled context never reaches blob store", func(t *testing.T) {
		blobs := &memoryBlobs{}
		store, err := New(blobs, testKey)
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err = store.PutIdentityDocument(ctx, testOwnerID, testCaseID, testFileID, []byte("synthetic private identity image"))
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled put error=%v, want context.Canceled", err)
		}
		if blobs.puts != 0 || len(blobs.objects) != 0 {
			t.Fatalf("canceled put reached blob store: puts=%d objects=%d", blobs.puts, len(blobs.objects))
		}
	})

	t.Run("backend detail is not exposed", func(t *testing.T) {
		blobs := &memoryBlobs{putErr: errors.New("synthetic backend failure: /private/identity/path")}
		store, err := New(blobs, testKey)
		if err != nil {
			t.Fatal(err)
		}

		err = store.PutIdentityDocument(context.Background(), testOwnerID, testCaseID, testFileID, []byte("synthetic private identity image"))
		if !errors.Is(err, ErrStorage) || errors.Is(err, blobs.putErr) {
			t.Fatalf("backend put error=%v, want sanitized ErrStorage", err)
		}
		if strings.Contains(err.Error(), "/private/identity/path") {
			t.Fatalf("backend detail leaked through error: %v", err)
		}
		if len(blobs.objects) != 0 {
			t.Fatalf("failed put left a stored object: %d", len(blobs.objects))
		}
	})
}

func TestStoreReadDoesNotReturnPlaintextWhenContextCancelsDuringBlobRead(t *testing.T) {
	blobs := &memoryBlobs{}
	store, err := New(blobs, testKey)
	if err != nil {
		t.Fatal(err)
	}
	plaintext := []byte("synthetic private identity image")
	if err := store.PutIdentityDocument(context.Background(), testOwnerID, testCaseID, testFileID, plaintext); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	blobs.getHook = cancel

	got, err := store.ReadIdentityDocument(ctx, testOwnerID, testCaseID, testFileID)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("read error=%v, want context.Canceled", err)
	}
	if len(got) != 0 {
		t.Fatalf("read returned private plaintext after cancellation: %q", got)
	}
}

func TestStoreReadPropagatesCancellationReturnedByBlobStore(t *testing.T) {
	blobs := &memoryBlobs{getErr: context.Canceled}
	store, err := New(blobs, testKey)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	blobs.getHook = cancel

	got, err := store.ReadIdentityDocument(ctx, testOwnerID, testCaseID, testFileID)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("read error=%v, want context.Canceled", err)
	}
	if len(got) != 0 {
		t.Fatalf("canceled read returned content: %q", got)
	}
}

func TestStoreDeletePropagatesCancellationReturnedByBlobStore(t *testing.T) {
	blobs := &memoryBlobs{delErr: context.Canceled}
	store, err := New(blobs, testKey)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	blobs.delHook = cancel

	err = store.DeleteIdentityDocument(ctx, testOwnerID, testCaseID, testFileID)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("delete error=%v, want context.Canceled", err)
	}
	if blobs.dels != 1 {
		t.Fatalf("delete calls=%d, want one in-flight attempt", blobs.dels)
	}
}

func TestStorePutPropagatesCancellationReturnedByBlobStore(t *testing.T) {
	blobs := &memoryBlobs{putErr: context.Canceled}
	store, err := New(blobs, testKey)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	blobs.putHook = cancel

	err = store.PutIdentityDocument(ctx, testOwnerID, testCaseID, testFileID, []byte("synthetic private identity image"))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("put error=%v, want context.Canceled", err)
	}
	if blobs.puts != 1 || len(blobs.objects) != 0 {
		t.Fatalf("canceled put left an object or made the wrong calls: puts=%d objects=%d", blobs.puts, len(blobs.objects))
	}
}
