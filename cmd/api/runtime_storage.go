package main

import (
	"context"
	"errors"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/cloudstorage"
	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/evidencefs"
)

type runtimeFileStore interface {
	Put(context.Context, string, []byte) error
	Get(context.Context, string) ([]byte, error)
	Delete(context.Context, string) error
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
