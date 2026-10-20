package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HernanEspinozaDev/espaciGo/internal/gcp06sqlinspect"
)

func TestReadCredentialsRequiresMode0400AndPinnedShape(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		mode       os.FileMode
		wantError  bool
	}{
		{name: "valid", body: `{"username":"gcp06_inspect_012345abcdef","password":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"}`, mode: 0400},
		{name: "writable", body: `{"username":"gcp06_inspect_012345abcdef","password":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"}`, mode: 0600, wantError: true},
		{name: "wrong login", body: `{"username":"postgres","password":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"}`, mode: 0400, wantError: true},
		{name: "unknown field", body: `{"username":"gcp06_inspect_012345abcdef","password":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA","extra":"x"}`, mode: 0400, wantError: true},
		{name: "password escape", body: `{"username":"gcp06_inspect_012345abcdef","password":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA\u0041"}`, mode: 0400, wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "credential.json")
			if err := os.WriteFile(path, []byte(tc.body), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(path, tc.mode); err != nil {
				t.Fatal(err)
			}
			credential, err := readCredentials(path)
			if tc.wantError {
				if err == nil {
					t.Fatal("invalid credential was accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if credential.Username != "gcp06_inspect_012345abcdef" || len(credential.Password) != 43 {
				t.Fatal("credential shape did not match the one-run contract")
			}
			clear(credential.Password)
		})
	}
}

func TestRunRejectsAnyNonCandidateBeforeOpeningSecret(t *testing.T) {
	getenv := func(key string) string {
		if key == "GCP06_RUN_ID" {
			return "other-run"
		}
		return ""
	}
	var stdout, stderr bytes.Buffer
	err := run(nil, getenv, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "pinned GCP-06 candidate") || stdout.Len() != 0 {
		t.Fatal("wrong candidate was not rejected before credentials or SQL were accessed")
	}
}

func TestCLIUsesFixedCandidateDatabase(t *testing.T) {
	if got := databaseURL("gcp06_inspect_012345abcdef"); !strings.Contains(got, "/"+gcp06sqlinspect.DatabaseName+"?") || strings.Contains(got, "password") {
		t.Fatalf("inspector connection URL is not fixed and password-free: %q", got)
	}
}
