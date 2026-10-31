package main

import (
	"testing"
	"time"
)

func TestDocumentScanConfigRequiresExplicitPrivateScannerAndFreshness(t *testing.T) {
	cases := []struct {
		name      string
		address   string
		maxAge    string
		wantReady bool
	}{
		{name: "disabled when absent"},
		{name: "valid loopback", address: "127.0.0.1:3310", maxAge: "48h", wantReady: true},
		{name: "valid private scanner", address: "172.18.0.4:3310", maxAge: "48h", wantReady: true},
		{name: "public scanner rejected", address: "8.8.8.8:3310", maxAge: "48h"},
		{name: "hostname rejected", address: "clamav:3310", maxAge: "48h"},
		{name: "scanner without freshness rejected", address: "127.0.0.1:3310"},
		{name: "freshness without scanner rejected", maxAge: "48h"},
		{name: "invalid port rejected", address: "127.0.0.1:0", maxAge: "48h"},
		{name: "unbounded freshness rejected", address: "127.0.0.1:3310", maxAge: "744h"},
		{name: "invalid freshness rejected", address: "127.0.0.1:3310", maxAge: "recent"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			values := map[string]string{
				"DOCUMENT_VERIFICATION_CLAMAV_ADDRESS":           tc.address,
				"DOCUMENT_VERIFICATION_CLAMAV_MAX_SIGNATURE_AGE": tc.maxAge,
			}
			got, enabled, err := documentScanConfigFromEnv(func(key string) string { return values[key] })
			if tc.wantReady {
				if err != nil || !enabled || got.address != tc.address || got.maxSignatureAge != 48*time.Hour {
					t.Fatalf("config=%+v enabled=%t err=%v", got, enabled, err)
				}
				return
			}
			if tc.name == "disabled when absent" {
				if err != nil || enabled {
					t.Fatalf("absent config enabled=%t err=%v", enabled, err)
				}
				return
			}
			if err == nil || enabled {
				t.Fatalf("invalid config enabled=%t err=%v", enabled, err)
			}
		})
	}
}
