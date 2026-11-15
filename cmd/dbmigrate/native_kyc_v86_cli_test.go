package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestNativeV86CLIRejectsBeforeConnectionConfiguration(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "dbmigrate")
	build := exec.Command("go", "build", "-o", binary, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build migration CLI: %v %s", err, output)
	}
	for _, tc := range []struct {
		name    string
		args    []string
		message string
	}{
		{"missing confirmation", []string{"-native-kyc-v86"}, "requires its mode"},
		{"explicit false mode", []string{"-native-kyc-v86=false"}, "requires its mode"},
		{"tail missing confirmation", []string{"-native-kyc-tail-version", "87"}, "requires version 87 through 96"},
		{"tail explicit zero", []string{"-native-kyc-tail-version", "0"}, "requires version 87 through 96"},
		{"tail wrong version", []string{"-native-kyc-tail-version", "97"}, "requires version 87 through 96"},
		{"tail V91 unreviewed fingerprint", []string{"-native-kyc-tail-version", "91", "-confirm-native-kyc-tail", nativeAdminConfirmation, "-native-kyc-tail-source-sha256", strings.Repeat("0", 64)}, "requires version 87 through 96"},
		{"tail mixed mode", []string{"-native-kyc-tail-version", "87", "-confirm-native-kyc-tail", nativeAdminConfirmation, "-native-kyc-tail-source-sha256", nativeKYCTailReviewedFingerprint(87), "-native-kyc-v86"}, "cannot combine"},
		{"tail positional before mode", []string{"operand", "-native-kyc-tail-version", "88"}, "positional arguments are not supported"},
		{"tail separator", []string{"--", "-native-kyc-tail-version", "88"}, "positional arguments are not supported"},
		{"missing mode", []string{"-confirm-native-kyc-v86", nativeAdminConfirmation}, "requires its mode"},
		{"positional before mode", []string{"operand", "-native-kyc-v86"}, "positional arguments are not supported"},
		{"separator", []string{"--", "-native-kyc-v86"}, "positional arguments are not supported"},
		{"trailing operand", []string{"-native-kyc-v86", "operand"}, "positional arguments are not supported"},
		{"mixed mode", []string{"-native-kyc-v86", "-confirm-native-kyc-v86", nativeAdminConfirmation, "-native-kyc-v86-source-sha256", nativeV86SourceFingerprint, "-plan-only"}, "cannot combine"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command(binary, tc.args...)
			cmd.Env = append(os.Environ(), "DATABASE_URL=invalid-connection-settings")
			output, err := cmd.CombinedOutput()
			if err == nil || !strings.Contains(string(output), tc.message) {
				t.Fatalf("expected preconnection rejection %q, got err=%v output=%s", tc.message, err, output)
			}
		})
	}
	t.Run("valid tail mode reaches native preconnection guard", func(t *testing.T) {
		cmd := exec.Command(binary,
			"-native-kyc-tail-version", "95",
			"-confirm-native-kyc-tail", nativeAdminConfirmation,
			"-native-kyc-tail-source-sha256", nativeKYCTailReviewedFingerprint(95),
		)
		cmd.Env = []string{
			"PATH=" + os.Getenv("PATH"),
			"HOME=" + os.Getenv("HOME"),
			"TMPDIR=" + t.TempDir(),
			"DATABASE_URL=postgresql://espacigo_migrator_login@127.0.0.1:5432/wrong?sslmode=disable",
			"DATABASE_SET_ROLE=espacigo_schema_owner",
		}
		output, err := cmd.CombinedOutput()
		if err == nil || !strings.Contains(string(output), "native V86 requires the reviewed private passfile path") {
			t.Fatalf("valid V95 flags did not reach the native preconnection guard: err=%v output=%s", err, output)
		}
	})
}
