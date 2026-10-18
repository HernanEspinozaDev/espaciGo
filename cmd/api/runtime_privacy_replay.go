package main

import (
	"context"
	"errors"
	"strings"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/cloudstorage"
	"github.com/HernanEspinozaDev/espaciGo/internal/privacy"
)

const defaultPrivacyReplayObjectPrefix = "privacy"

func newRuntimePrivacyReplayStore(ctx context.Context, getenv func(string) string) (privacy.SuppressionReplayStore, error) {
	bucket, prefix, enabled, err := privacyReplayStoreConfig(getenv)
	if err != nil || !enabled {
		return nil, err
	}
	return cloudstorage.NewReplayRegistry(ctx, bucket, prefix)
}

func privacyReplayStoreConfig(getenv func(string) string) (bucket, prefix string, enabled bool, err error) {
	backend := strings.TrimSpace(getenv("LOCAL_PRIVACY_REPLAY_BACKEND"))
	if backend == "" {
		backend = "filesystem"
	}
	bucket = strings.TrimSpace(getenv("LOCAL_PRIVACY_REPLAY_BUCKET"))
	prefix = strings.TrimSpace(getenv("LOCAL_PRIVACY_REPLAY_PREFIX"))
	filePath := strings.TrimSpace(getenv("LOCAL_PRIVACY_REPLAY_FILE"))
	switch backend {
	case "filesystem":
		if bucket != "" || prefix != "" {
			return "", "", false, errors.New("GCS replay settings require the GCS backend")
		}
		return "", "", false, nil
	case "gcs":
		if bucket == "" || filePath != "" {
			return "", "", false, errors.New("GCS replay requires its bucket and cannot use the local sidecar path")
		}
		if prefix == "" {
			prefix = defaultPrivacyReplayObjectPrefix
		}
		return bucket, prefix, true, nil
	default:
		return "", "", false, errors.New("LOCAL_PRIVACY_REPLAY_BACKEND must be filesystem or gcs")
	}
}
