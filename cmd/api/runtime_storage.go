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
	switch getenv("BLOB_STORE_BACKEND") {
	case "", "filesystem":
		return evidencefs.New(evidenceRoot)
	case "gcs":
		bucket := getenv("CLOUD_STORAGE_BUCKET")
		if bucket == "" {
			return nil, errors.New("CLOUD_STORAGE_BUCKET is required for the GCS backend")
		}
		return cloudstorage.NewWithPrefix(ctx, bucket, getenv("CLOUD_STORAGE_OBJECT_PREFIX"))
	default:
		return nil, errors.New("BLOB_STORE_BACKEND must be filesystem or gcs")
	}
}
