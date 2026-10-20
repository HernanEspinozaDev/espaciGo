package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/HernanEspinozaDev/espaciGo/internal/gcp06sqlaccess"
)

func TestRunRejectsInvalidInvocationBeforeCloudCredentials(t *testing.T) {
	cases := [][]string{
		{},
		{"unknown", "--apply"},
		{"prepare", "--apply"},
		{"prepare", "--apply", "--confirm-target=wrong"},
		{"prepare", "--apply", "--confirm-target=" + gcp06sqlaccess.PrepareConfirm, "unexpected"},
		{"cleanup", "--apply", "--login=gcp06_inspect_012345abcdef", "--secret-version=1", "--confirm-target=wrong"},
	}
	for _, args := range cases {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if err := run(args, &stdout, &stderr); err == nil {
				t.Fatal("expected validation failure")
			}
			if strings.Contains(stderr.String(), "password") || strings.Contains(stdout.String(), "password") {
				t.Fatal("invalid invocation emitted credential content")
			}
		})
	}
}
