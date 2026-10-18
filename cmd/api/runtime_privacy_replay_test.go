package main

import (
	"testing"
)

func TestRuntimePrivacyReplayStoreConfiguration(t *testing.T) {
	values := map[string]string{}
	getenv := func(key string) string { return values[key] }
	if bucket, prefix, enabled, err := privacyReplayStoreConfig(getenv); err != nil || enabled || bucket != "" || prefix != "" {
		t.Fatalf("default local replay config bucket=%q prefix=%q enabled=%t err=%v", bucket, prefix, enabled, err)
	}
	values["LOCAL_PRIVACY_REPLAY_FILE"] = "/private/local/privacy-replay.json"
	if _, _, enabled, err := privacyReplayStoreConfig(getenv); err != nil || enabled {
		t.Fatalf("existing file sidecar config enabled=%t err=%v", enabled, err)
	}
	values["LOCAL_PRIVACY_REPLAY_BACKEND"] = "gcs"
	if _, _, _, err := privacyReplayStoreConfig(getenv); err == nil {
		t.Fatal("GCS replay backend accepted a missing bucket and a local sidecar path")
	}
	delete(values, "LOCAL_PRIVACY_REPLAY_FILE")
	values["LOCAL_PRIVACY_REPLAY_BUCKET"] = "synthetic-private-bucket"
	values["LOCAL_PRIVACY_REPLAY_PREFIX"] = "../other"
	if _, err := newRuntimePrivacyReplayStore(t.Context(), getenv); err == nil {
		t.Fatal("GCS replay backend accepted a traversal prefix")
	}
	values["LOCAL_PRIVACY_REPLAY_PREFIX"] = "privacy"
	if bucket, prefix, enabled, err := privacyReplayStoreConfig(getenv); err != nil || !enabled || bucket != "synthetic-private-bucket" || prefix != "privacy" {
		t.Fatalf("GCS replay config bucket=%q prefix=%q enabled=%t err=%v", bucket, prefix, enabled, err)
	}
	values["LOCAL_PRIVACY_REPLAY_BACKEND"] = "unsupported"
	if _, _, _, err := privacyReplayStoreConfig(getenv); err == nil {
		t.Fatal("runtime replay store accepted an unsupported backend")
	}
}
