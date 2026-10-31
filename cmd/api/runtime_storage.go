package main

import (
	"context"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/cloudstorage"
	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/documentfs"
	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/evidencefs"
	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/mediafs"
	"github.com/HernanEspinozaDev/espaciGo/internal/media"
)

type runtimeFileStore interface {
	Put(context.Context, string, []byte) error
	Get(context.Context, string) ([]byte, error)
	Delete(context.Context, string) error
}

func newRuntimeDocumentStore(getenv func(string) string, evidenceRoot string) (*documentfs.Store, error) {
	root := strings.TrimSpace(getenv("DOCUMENT_VERIFICATION_STORAGE_ROOT"))
	keyPath := strings.TrimSpace(getenv("DOCUMENT_VERIFICATION_ENCRYPTION_KEY_FILE"))
	if root == "" && keyPath == "" {
		return nil, nil
	}
	if root == "" || keyPath == "" || !filepath.IsAbs(root) || filepath.Clean(root) != root || !filepath.IsAbs(keyPath) || filepath.Clean(keyPath) != keyPath || !filepath.IsAbs(evidenceRoot) || filepath.Clean(evidenceRoot) != evidenceRoot {
		return nil, errors.New("document verification storage configuration is incomplete or invalid")
	}
	canonicalRoot, err := canonicalStoragePath(root)
	if err != nil {
		return nil, errors.New("document verification storage root cannot be resolved")
	}
	canonicalEvidenceRoot, err := canonicalStoragePath(evidenceRoot)
	if err != nil {
		return nil, errors.New("evidence storage root cannot be resolved")
	}
	if pathsOverlap(canonicalRoot, canonicalEvidenceRoot) {
		return nil, errors.New("document verification storage must be separate from existing evidence storage")
	}
	keyInfo, err := os.Lstat(keyPath)
	if err != nil || !keyInfo.Mode().IsRegular() || keyInfo.Mode()&os.ModeSymlink != 0 || keyInfo.Mode().Perm() != 0o600 {
		return nil, errors.New("document verification encryption key must be a private regular file")
	}
	encodedKey, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, errors.New("document verification encryption key is unavailable")
	}
	key, err := hex.DecodeString(strings.TrimSpace(string(encodedKey)))
	for i := range encodedKey {
		encodedKey[i] = 0
	}
	if err != nil || len(key) != 32 {
		return nil, errors.New("document verification encryption key is invalid")
	}
	store, err := documentfs.New(root, key)
	for i := range key {
		key[i] = 0
	}
	if err != nil {
		return nil, err
	}
	return store, nil
}

func canonicalStoragePath(path string) (string, error) {
	return resolveStoragePath(filepath.Clean(path), 0)
}

func resolveStoragePath(path string, symlinkDepth int) (string, error) {
	if symlinkDepth > 255 {
		return "", errors.New("storage path contains too many symlinks")
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err == nil {
		return filepath.Clean(resolved), nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	if info, statErr := os.Lstat(path); statErr == nil && info.Mode()&os.ModeSymlink != 0 {
		return resolveStorageSymlink(path, symlinkDepth)
	} else if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
		return "", statErr
	}

	parent := filepath.Dir(path)
	if parent == path {
		return "", err
	}
	var resolvedParent string
	if info, statErr := os.Lstat(parent); statErr == nil && info.Mode()&os.ModeSymlink != 0 {
		resolvedParent, err = resolveStorageSymlink(parent, symlinkDepth)
	} else if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
		return "", statErr
	} else {
		resolvedParent, err = resolveStoragePath(parent, symlinkDepth)
	}
	if err != nil {
		return "", err
	}
	return filepath.Join(resolvedParent, filepath.Base(path)), nil
}

func resolveStorageSymlink(path string, symlinkDepth int) (string, error) {
	target, err := os.Readlink(path)
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(path), target)
	}
	return resolveStoragePath(filepath.Clean(target), symlinkDepth+1)
}

func pathsOverlap(a, b string) bool {
	within := func(root, path string) bool {
		rel, err := filepath.Rel(root, path)
		return err == nil && (rel == "." || rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
	}
	return within(a, b) || within(b, a)
}

func newRuntimeMediaStore(ctx context.Context, getenv func(string) string, evidenceRoot string) (media.ObjectStore, error) {
	backend, bucket, _, err := runtimeFileStoreConfig(getenv)
	if err != nil {
		return nil, err
	}
	switch backend {
	case "filesystem":
		if evidenceRoot == "" {
			return nil, errors.New("M03_EVIDENCE_DIR is required for filesystem media storage")
		}
		return mediafs.New(filepath.Join(evidenceRoot, "private-media"))
	case "gcs":
		return cloudstorage.NewMediaStore(ctx, bucket)
	default:
		return nil, errors.New("media storage configuration is invalid")
	}
}

func newRuntimeFileStore(ctx context.Context, getenv func(string) string, evidenceRoot string) (runtimeFileStore, error) {
	backend, bucket, prefix, err := runtimeFileStoreConfig(getenv)
	if err != nil {
		return nil, err
	}
	switch backend {
	case "", "filesystem":
		return evidencefs.New(evidenceRoot)
	case "gcs":
		return cloudstorage.NewWithPrefix(ctx, bucket, prefix)
	default:
		return nil, errors.New("BLOB_STORE_BACKEND must be filesystem or gcs")
	}
}

func runtimeFileStoreConfig(getenv func(string) string) (backend, bucket, prefix string, err error) {
	backend = getenv("BLOB_STORE_BACKEND")
	if backend == "" {
		backend = "filesystem"
	}
	switch backend {
	case "filesystem":
		return backend, "", "", nil
	case "gcs":
		bucket = getenv("CLOUD_STORAGE_BUCKET")
		if bucket == "" {
			return "", "", "", errors.New("CLOUD_STORAGE_BUCKET is required for the GCS backend")
		}
		return backend, bucket, getenv("CLOUD_STORAGE_OBJECT_PREFIX"), nil
	default:
		return "", "", "", errors.New("BLOB_STORE_BACKEND must be filesystem or gcs")
	}
}
