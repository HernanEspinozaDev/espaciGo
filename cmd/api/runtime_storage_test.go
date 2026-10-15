package main

import "testing"

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
