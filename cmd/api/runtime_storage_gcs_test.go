package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/documentgcs"
)

func TestRuntimeGCSDocumentStoreKeepsPurposesInSeparateBuckets(t *testing.T) {
	parent := t.TempDir()
	keyringPath := filepath.Join(parent, "keyring.json")
	keyring, err := json.Marshal(struct {
		CurrentVersion string            `json:"current_version"`
		Keys           map[string]string `json:"keys"`
	}{CurrentVersion: "v1", Keys: map[string]string{"v1": strings.Repeat("01", 32)}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyringPath, keyring, 0o600); err != nil {
		t.Fatal(err)
	}
	values := map[string]string{
		"DOCUMENT_VERIFICATION_STORAGE_BACKEND":         "gcs",
		"DOCUMENT_VERIFICATION_IDENTITY_GCS_BUCKET":     "synthetic-identity-bucket",
		"DOCUMENT_VERIFICATION_SPACE_GCS_BUCKET":        "synthetic-space-bucket",
		"DOCUMENT_VERIFICATION_ENCRYPTION_KEYRING_FILE": keyringPath,
	}
	getenv := func(key string) string { return values[key] }
	stores := make(map[string]*runtimeDocumentObjectFake)
	prefixes := make(map[string]string)
	factory := func(_ context.Context, bucket, prefix string) (documentgcs.ObjectStore, error) {
		if _, exists := stores[bucket]; exists {
			return nil, errors.New("bucket factory called more than once")
		}
		store := &runtimeDocumentObjectFake{objects: make(map[string]runtimeDocumentBlob)}
		stores[bucket] = store
		prefixes[bucket] = prefix
		return store, nil
	}
	documentStore, err := newRuntimeDocumentStoreWithFactory(getenv, "", factory)
	if err != nil || documentStore == nil {
		t.Fatalf("runtime GCS document store = %v, %v", documentStore, err)
	}
	if len(stores) != 2 || prefixes["synthetic-identity-bucket"] != "identity-documents" || prefixes["synthetic-space-bucket"] != "space-documents" {
		t.Fatalf("GCS stores were not separated by purpose: stores=%v prefixes=%v", stores, prefixes)
	}

	ctx := context.Background()
	const owner = "11111111-1111-4111-8111-111111111111"
	const otherOwner = "66666666-6666-4666-8666-666666666666"
	const space = "22222222-2222-4222-8222-222222222222"
	const caseID = "33333333-3333-4333-8333-333333333333"
	const fileID = "44444444-4444-4444-8444-444444444444"
	if err := documentStore.PutIdentityDocument(ctx, owner, caseID, fileID, []byte("synthetic identity")); err != nil {
		t.Fatal(err)
	}
	if err := documentStore.PutSpaceDocument(ctx, owner, space, caseID, fileID, []byte("synthetic space")); err != nil {
		t.Fatal(err)
	}
	identityObject := stores["synthetic-identity-bucket"].objects[fileID]
	spaceObject := stores["synthetic-space-bucket"].objects[fileID]
	if len(stores["synthetic-identity-bucket"].objects) != 1 || len(stores["synthetic-space-bucket"].objects) != 1 || bytes.Contains(identityObject.blob, []byte("synthetic identity")) || bytes.Contains(spaceObject.blob, []byte("synthetic space")) {
		t.Fatal("runtime GCS stores did not persist opaque UUIDs with encrypted payloads")
	}
	if identityObject.metadata["key_version"] != "v1" || spaceObject.metadata["key_version"] != "v1" {
		t.Fatal("runtime GCS stores omitted the key version selector")
	}
	if got, err := documentStore.ReadIdentityDocument(ctx, owner, caseID, fileID); err != nil || string(got) != "synthetic identity" {
		t.Fatalf("identity read = %q, %v", got, err)
	}
	if got, err := documentStore.ReadSpaceDocument(ctx, owner, space, caseID, fileID); err != nil || string(got) != "synthetic space" {
		t.Fatalf("space read = %q, %v", got, err)
	}
	if _, err := documentStore.ReadIdentityDocument(ctx, otherOwner, caseID, fileID); err == nil {
		t.Fatal("identity ciphertext decrypted with another owner's AAD")
	}
}

func TestRuntimeGCSDocumentStoreAcceptsCloudRunSecretVolumeMode(t *testing.T) {
	parent := t.TempDir()
	keyringPath := filepath.Join(parent, "keyring.json")
	keyring := []byte(`{"current_version":"v1","keys":{"v1":"` + strings.Repeat("01", 32) + `"}}`)
	if err := os.WriteFile(keyringPath, keyring, 0o444); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(keyringPath, 0o444); err != nil {
		t.Fatal(err)
	}
	values := map[string]string{
		"DOCUMENT_VERIFICATION_STORAGE_BACKEND":         "gcs",
		"DOCUMENT_VERIFICATION_IDENTITY_GCS_BUCKET":     "synthetic-identity-bucket",
		"DOCUMENT_VERIFICATION_SPACE_GCS_BUCKET":        "synthetic-space-bucket",
		"DOCUMENT_VERIFICATION_ENCRYPTION_KEYRING_FILE": keyringPath,
	}
	factory := func(context.Context, string, string) (documentgcs.ObjectStore, error) {
		return &runtimeDocumentObjectFake{objects: make(map[string]runtimeDocumentBlob)}, nil
	}
	store, err := newRuntimeDocumentStoreWithFactory(func(key string) string { return values[key] }, "", factory)
	if err != nil || store == nil {
		t.Fatalf("read-only Cloud Run secret volume keyring = %v, %v", store, err)
	}
}

func TestRuntimeGCSDocumentStoreRejectsMissingKeyringWithoutPanic(t *testing.T) {
	parent := t.TempDir()
	values := map[string]string{
		"DOCUMENT_VERIFICATION_STORAGE_BACKEND":         "gcs",
		"DOCUMENT_VERIFICATION_IDENTITY_GCS_BUCKET":     "synthetic-identity-bucket",
		"DOCUMENT_VERIFICATION_SPACE_GCS_BUCKET":        "synthetic-space-bucket",
		"DOCUMENT_VERIFICATION_ENCRYPTION_KEYRING_FILE": filepath.Join(parent, "missing-keyring.json"),
	}
	calls := 0
	factory := func(context.Context, string, string) (documentgcs.ObjectStore, error) {
		calls++
		return &runtimeDocumentObjectFake{objects: make(map[string]runtimeDocumentBlob)}, nil
	}
	if store, err := newRuntimeDocumentStoreWithFactory(func(key string) string { return values[key] }, "", factory); err == nil || store != nil {
		t.Fatalf("missing keyring configuration = %v, %v; want rejection", store, err)
	}
	if calls != 0 {
		t.Fatalf("object-store factory called %d times before keyring rejection", calls)
	}
}

func TestRuntimeGCSDocumentStoreRejectsSharedBucketBeforeFactory(t *testing.T) {
	parent := t.TempDir()
	keyringPath := filepath.Join(parent, "keyring.json")
	keyring := []byte(`{"current_version":"v1","keys":{"v1":"` + strings.Repeat("01", 32) + `"}}`)
	if err := os.WriteFile(keyringPath, keyring, 0o600); err != nil {
		t.Fatal(err)
	}
	values := map[string]string{
		"DOCUMENT_VERIFICATION_STORAGE_BACKEND":         "gcs",
		"DOCUMENT_VERIFICATION_IDENTITY_GCS_BUCKET":     "same-bucket",
		"DOCUMENT_VERIFICATION_SPACE_GCS_BUCKET":        "same-bucket",
		"DOCUMENT_VERIFICATION_ENCRYPTION_KEYRING_FILE": keyringPath,
	}
	calls := 0
	factory := func(context.Context, string, string) (documentgcs.ObjectStore, error) {
		calls++
		return &runtimeDocumentObjectFake{objects: make(map[string]runtimeDocumentBlob)}, nil
	}
	if store, err := newRuntimeDocumentStoreWithFactory(func(key string) string { return values[key] }, "", factory); err == nil || store != nil {
		t.Fatalf("shared GCS bucket configuration = %v, %v; want rejection", store, err)
	}
	if calls != 0 {
		t.Fatalf("object-store factory called %d times before config rejection", calls)
	}
}

func TestRuntimeGCSDocumentStoreRejectsGenericPrivateBucketBeforeFactory(t *testing.T) {
	parent := t.TempDir()
	keyringPath := filepath.Join(parent, "keyring.json")
	keyring := []byte(`{"current_version":"v1","keys":{"v1":"` + strings.Repeat("01", 32) + `"}}`)
	if err := os.WriteFile(keyringPath, keyring, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name           string
		identityBucket string
		spaceBucket    string
	}{
		{name: "identity documents alias generic private bucket", identityBucket: "generic-private-bucket", spaceBucket: "space-documents-bucket"},
		{name: "space documents alias generic private bucket", identityBucket: "identity-documents-bucket", spaceBucket: "generic-private-bucket"},
	} {
		t.Run(test.name, func(t *testing.T) {
			values := map[string]string{
				"BLOB_STORE_BACKEND":                            "gcs",
				"CLOUD_STORAGE_BUCKET":                          "generic-private-bucket",
				"DOCUMENT_VERIFICATION_STORAGE_BACKEND":         "gcs",
				"DOCUMENT_VERIFICATION_IDENTITY_GCS_BUCKET":     test.identityBucket,
				"DOCUMENT_VERIFICATION_SPACE_GCS_BUCKET":        test.spaceBucket,
				"DOCUMENT_VERIFICATION_ENCRYPTION_KEYRING_FILE": keyringPath,
			}
			calls := 0
			factory := func(context.Context, string, string) (documentgcs.ObjectStore, error) {
				calls++
				return &runtimeDocumentObjectFake{objects: make(map[string]runtimeDocumentBlob)}, nil
			}
			store, err := newRuntimeDocumentStoreWithFactory(func(key string) string { return values[key] }, "", factory)
			if err == nil || store != nil {
				t.Fatalf("document bucket alias configuration = %v, %v; want rejection", store, err)
			}
			if calls != 0 {
				t.Fatalf("object-store factory called %d times before config rejection", calls)
			}
		})
	}
}

func TestRuntimeGCSMediaStoreRequiresBucketSeparateFromPrivateBlobs(t *testing.T) {
	tests := []struct {
		name   string
		values map[string]string
	}{
		{
			name: "missing dedicated media bucket",
			values: map[string]string{
				"BLOB_STORE_BACKEND":   "gcs",
				"CLOUD_STORAGE_BUCKET": "generic-private-bucket",
			},
		},
		{
			name: "media aliases generic evidence bucket",
			values: map[string]string{
				"BLOB_STORE_BACKEND":         "gcs",
				"CLOUD_STORAGE_BUCKET":       "generic-private-bucket",
				"CLOUD_STORAGE_MEDIA_BUCKET": "generic-private-bucket",
			},
		},
		{
			name: "media aliases identity document bucket",
			values: map[string]string{
				"BLOB_STORE_BACKEND":                        "gcs",
				"CLOUD_STORAGE_BUCKET":                      "generic-private-bucket",
				"CLOUD_STORAGE_MEDIA_BUCKET":                "media-bucket",
				"DOCUMENT_VERIFICATION_IDENTITY_GCS_BUCKET": "media-bucket",
			},
		},
		{
			name: "media aliases space document bucket",
			values: map[string]string{
				"BLOB_STORE_BACKEND":                     "gcs",
				"CLOUD_STORAGE_BUCKET":                   "generic-private-bucket",
				"CLOUD_STORAGE_MEDIA_BUCKET":             "media-bucket",
				"DOCUMENT_VERIFICATION_SPACE_GCS_BUCKET": "media-bucket",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, err := newRuntimeMediaStore(context.Background(), func(key string) string { return tt.values[key] }, "")
			if err == nil || store != nil {
				t.Fatalf("GCS media configuration = %v, %v; want rejection before client creation", store, err)
			}
		})
	}
}

type runtimeDocumentBlob struct {
	blob     []byte
	metadata map[string]string
}

type runtimeDocumentObjectFake struct {
	objects map[string]runtimeDocumentBlob
}

func (s *runtimeDocumentObjectFake) PutWithMetadata(_ context.Context, id string, blob []byte, metadata map[string]string) error {
	if _, exists := s.objects[id]; exists {
		return errors.New("object already exists")
	}
	s.objects[id] = runtimeDocumentBlob{blob: append([]byte(nil), blob...), metadata: cloneRuntimeMetadata(metadata)}
	return nil
}

func (s *runtimeDocumentObjectFake) GetWithMetadata(_ context.Context, id string, maxBytes int64) ([]byte, map[string]string, error) {
	object, exists := s.objects[id]
	if !exists {
		return nil, nil, errors.New("object not found")
	}
	if maxBytes > 0 && int64(len(object.blob)) > maxBytes {
		return nil, nil, errors.New("object too large")
	}
	return append([]byte(nil), object.blob...), cloneRuntimeMetadata(object.metadata), nil
}

func (s *runtimeDocumentObjectFake) Delete(_ context.Context, id string) error {
	delete(s.objects, id)
	return nil
}

func cloneRuntimeMetadata(metadata map[string]string) map[string]string {
	cloned := make(map[string]string, len(metadata))
	for key, value := range metadata {
		cloned[key] = value
	}
	return cloned
}
