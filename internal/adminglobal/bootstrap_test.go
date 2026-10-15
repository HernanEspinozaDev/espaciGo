package adminglobal

import (
	"net"
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

func TestIsLocalDatabaseAddressRequiresLoopback(t *testing.T) {
	for _, test := range []struct {
		address string
		want    bool
	}{{"127.0.0.1", true}, {"::1", true}, {"10.0.0.4", false}, {"8.8.8.8", false}} {
		if got := IsLocalDatabaseAddress(net.ParseIP(test.address)); got != test.want {
			t.Errorf("IsLocalDatabaseAddress(%q)=%v, want %v", test.address, got, test.want)
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

func TestLocalActivationRequiresExplicitOrdinaryLocalDatabaseAndNoURLOverrides(t *testing.T) {
	valid := "postgres://owner:secret@127.0.0.1:5432/espacigo_local"
	if err := ValidateLocalActivationTarget("local-activation", valid, "espacigo_local"); err != nil {
		t.Fatalf("valid local activation URL rejected: %v", err)
	}
	for _, test := range []struct{ env, dsn, name string }{
		{"local-test", valid, "espacigo_local"},
		{"local-activation", "postgres://owner@database/espacigo_local", "espacigo_local"},
		{"local-activation", valid + "?host=remote", "espacigo_local"},
		{"local-activation", valid + "?service=remote", "espacigo_local"},
		{"local-activation", strings.Replace(valid, "espacigo_local", "espacigo_pgdata", 1), "espacigo_pgdata"},
		{"local-activation", strings.Replace(valid, "espacigo_local", "other", 1), "espacigo_local"},
	} {
		if err := ValidateLocalActivationTarget(test.env, test.dsn, test.name); err == nil {
			t.Fatalf("unsafe activation target accepted: %+v", test)
		}
	}
}
