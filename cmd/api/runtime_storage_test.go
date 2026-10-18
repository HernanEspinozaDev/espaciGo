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
