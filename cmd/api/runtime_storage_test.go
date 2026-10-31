package main

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestRuntimeFileStoreValidatesGCSConfigurationBeforeADC(t *testing.T) {
	values := map[string]string{"BLOB_STORE_BACKEND": "gcs"}
	if _, err := newRuntimeFileStore(t.Context(), func(key string) string { return values[key] }, ""); err == nil {
		t.Fatal("GCS backend accepted an empty bucket")
	}
	values["CLOUD_STORAGE_BUCKET"] = "synthetic-test-bucket"
	values["BLOB_STORE_BACKEND"] = "unsupported"
	if _, err := newRuntimeFileStore(t.Context(), func(key string) string { return values[key] }, ""); err == nil {
		t.Fatal("runtime store accepted an unsupported backend")
	}
}

func TestRuntimeFileStoreConfigurationSelectsSharedBackendAndPrefix(t *testing.T) {
	values := map[string]string{"BLOB_STORE_BACKEND": "gcs", "CLOUD_STORAGE_BUCKET": "private-bucket", "CLOUD_STORAGE_OBJECT_PREFIX": "private/evidence"}
	backend, bucket, prefix, err := runtimeFileStoreConfig(func(key string) string { return values[key] })
	if err != nil || backend != "gcs" || bucket != "private-bucket" || prefix != "private/evidence" {
		t.Fatalf("runtime blob config = %q %q %q, %v", backend, bucket, prefix, err)
	}
	values["BLOB_STORE_BACKEND"] = "filesystem"
	backend, bucket, prefix, err = runtimeFileStoreConfig(func(key string) string { return values[key] })
	if err != nil || backend != "filesystem" || bucket != "" || prefix != "" {
		t.Fatalf("filesystem runtime blob config = %q %q %q, %v", backend, bucket, prefix, err)
	}
}

func TestRuntimeDocumentStoreRequiresSeparatePrivateRootAndKey(t *testing.T) {
	parent := t.TempDir()
	evidenceRoot := filepath.Join(parent, "evidence")
	if err := os.Mkdir(evidenceRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	keyPath := filepath.Join(parent, "document-key")
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	if err := os.WriteFile(keyPath, []byte(hex.EncodeToString(key)), 0o600); err != nil {
		t.Fatal(err)
	}
	values := map[string]string{
		"DOCUMENT_VERIFICATION_STORAGE_ROOT":        filepath.Join(parent, "documents"),
		"DOCUMENT_VERIFICATION_ENCRYPTION_KEY_FILE": keyPath,
	}
	getenv := func(key string) string { return values[key] }
	store, err := newRuntimeDocumentStore(getenv, evidenceRoot)
	if err != nil || store == nil {
		t.Fatalf("private document store = %v, %v", store, err)
	}
	if info, err := os.Stat(values["DOCUMENT_VERIFICATION_STORAGE_ROOT"]); err != nil || info.Mode().Perm() != 0o700 {
		t.Fatalf("document root permissions=%v err=%v, want 0700", info, err)
	}

	values["DOCUMENT_VERIFICATION_STORAGE_ROOT"] = filepath.Join(evidenceRoot, "documents")
	if _, err := newRuntimeDocumentStore(getenv, evidenceRoot); err == nil {
		t.Fatal("document store accepted a root overlapping existing evidence storage")
	}

	alias := filepath.Join(parent, "alias")
	if err := os.Symlink(parent, alias); err != nil {
		t.Fatal(err)
	}
	values["DOCUMENT_VERIFICATION_STORAGE_ROOT"] = filepath.Join(alias, "evidence", "documents")
	if _, err := newRuntimeDocumentStore(getenv, evidenceRoot); err == nil {
		t.Fatal("document store accepted a root overlapping evidence storage through a symlinked ancestor")
	}
	brokenAlias := filepath.Join(parent, "broken-alias")
	if err := os.Symlink(filepath.Join(evidenceRoot, "not-created-yet"), brokenAlias); err != nil {
		t.Fatal(err)
	}
	values["DOCUMENT_VERIFICATION_STORAGE_ROOT"] = filepath.Join(brokenAlias, "documents")
	if _, err := newRuntimeDocumentStore(getenv, evidenceRoot); err == nil {
		t.Fatal("document store accepted a root beneath a symlink to a nonexistent path inside evidence storage")
	}

	values["DOCUMENT_VERIFICATION_STORAGE_ROOT"] = filepath.Join(parent, "documents")
	if err := os.Chmod(keyPath, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := newRuntimeDocumentStore(getenv, evidenceRoot); err == nil {
		t.Fatal("document store accepted a broadly readable encryption key file")
	}
}

func TestRuntimeDocumentStoreIsDisabledOnlyWhenConfigurationIsAbsent(t *testing.T) {
	store, err := newRuntimeDocumentStore(func(string) string { return "" }, t.TempDir())
	if err != nil || store != nil {
		t.Fatalf("unconfigured document storage = %v, %v; want disabled", store, err)
	}
	root := filepath.Join(t.TempDir(), "documents")
	if _, err := newRuntimeDocumentStore(func(key string) string {
		if key == "DOCUMENT_VERIFICATION_STORAGE_ROOT" {
			return root
		}
		return ""
	}, t.TempDir()); err == nil {
		t.Fatal("partial document storage configuration was accepted")
	}
}

func TestCanonicalStoragePathResolvesMissingDescendantsThroughSymlink(t *testing.T) {
	parent := t.TempDir()
	evidenceRoot := filepath.Join(parent, "evidence")
	if err := os.Mkdir(evidenceRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(parent, "missing-target-link")
	if err := os.Symlink(filepath.Join(evidenceRoot, "not-created-yet"), link); err != nil {
		t.Fatal(err)
	}

	canonicalDocumentRoot, err := canonicalStoragePath(filepath.Join(link, "documents"))
	if err != nil {
		t.Fatal(err)
	}
	canonicalEvidenceRoot, err := canonicalStoragePath(evidenceRoot)
	if err != nil {
		t.Fatal(err)
	}
	if !pathsOverlap(canonicalDocumentRoot, canonicalEvidenceRoot) {
		t.Fatalf("canonical paths do not overlap: document=%q evidence=%q", canonicalDocumentRoot, canonicalEvidenceRoot)
	}
}
