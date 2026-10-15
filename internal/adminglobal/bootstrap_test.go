package adminglobal

import (
	"strings"
	"testing"
)

func TestBootstrapTargetRequiresExplicitLoopbackTestDatabase(t *testing.T) {
	name := "espacigo_gadm01_1234abcd"
	valid := "postgres://tester:secret@127.0.0.1:25458/" + name
	if err := ValidateBootstrapTarget("local-test", valid, name); err != nil {
		t.Fatalf("valid local target rejected: %v", err)
	}
	for _, test := range []struct{ env, dsn, name string }{
		{"production", valid, name},
		{"local-test", "postgres://tester:secret@database:5432/" + name, name},
		{"local-test", valid, "espacigo"},
		{"local-test", strings.Replace(valid, name, "other", 1), name},
	} {
		if err := ValidateBootstrapTarget(test.env, test.dsn, test.name); err == nil {
			t.Fatalf("unsafe bootstrap target accepted: %+v", test)
		}
	}
}

func TestCanonicalBootstrapActionAcceptsCLIAndPersistedRecoveryName(t *testing.T) {
	for input, expected := range map[string]string{
		"bootstrap":    "bootstrap",
		"recover":      "recuperacion",
		"recuperacion": "recuperacion",
	} {
		actual, ok := canonicalBootstrapAction(input)
		if !ok || actual != expected {
			t.Fatalf("canonicalBootstrapAction(%q) = %q, %v; want %q, true", input, actual, ok, expected)
		}
	}
	if _, ok := canonicalBootstrapAction("unknown"); ok {
		t.Fatal("unknown bootstrap action was accepted")
	}
}
