package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/HernanEspinozaDev/espaciGo/internal/gcp03restorecredential"
)

func TestRunRequiresExactApplyConfirmationBeforeCreatingAPIClients(t *testing.T) {
	for _, args := range [][]string{
		{},
		{"--apply"},
		{"--apply", "--confirm-target=pilot01"},
		{"--apply", "--confirm-target=" + gcp03restorecredential.Confirmation, "password-on-argv"},
	} {
		var stdout, stderr bytes.Buffer
		if code := run(args, &stdout, &stderr); code != 2 {
			t.Fatalf("run(%q) exit=%d, want 2", args, code)
		}
		if strings.Contains(stderr.String(), "password-on-argv") || stdout.Len() != 0 {
			t.Fatal("invalid command arguments were not rejected without exposing arguments")
		}
	}
}
