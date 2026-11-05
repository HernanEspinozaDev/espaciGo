package main

import "testing"

func TestDocumentScanOnlyUsesDedicatedStartupPath(t *testing.T) {
	if !usesDedicatedStartup(runtimeProfileDocumentScanOnly) {
		t.Fatal("document-scan-only profile must bypass unrelated application initialization")
	}
	if usesDedicatedStartup(runtimeProfileStandard) || usesDedicatedStartup(runtimeProfileWorkerFree) {
		t.Fatal("standard and worker-free profiles must retain the existing startup path")
	}
}
