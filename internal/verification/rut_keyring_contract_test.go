package verification

import (
	"strings"
	"testing"
)

func TestRutHMACKeyringMatchesDurableVersionGrammar(t *testing.T) {
	secret := []byte("synthetic-hmac-key-material-32bytes")
	for _, version := range []string{"v2.2026", "v1_legacy", "2-active"} {
		ring, err := NewRutHMACKeyring(version, map[string][]byte{version: secret})
		if err != nil {
			t.Fatalf("durable-valid version rejected: %s", version)
		}
		set, err := ring.Fingerprints("11222333-9")
		if err != nil || ValidateRutFingerprintSet(set) != nil {
			t.Fatalf("valid keyring produced rejected set for %s", version)
		}
	}
	for _, version := range []string{"_v1", "-v1", ".v1", "v1:bad", "v1 space", strings.Repeat("v", 33)} {
		if _, err := NewRutHMACKeyring(version, map[string][]byte{version: secret}); err == nil {
			t.Fatalf("durable-invalid version accepted: %s", version)
		}
	}
}

func TestRutHMACKeyringZeroValueFailsClosed(t *testing.T) {
	for _, ring := range []*RutHMACKeyring{nil, new(RutHMACKeyring)} {
		if _, err := ring.Fingerprints("11222333-9"); err == nil {
			t.Fatal("unconfigured keyring produced a successful empty set")
		}
	}
}

func TestRutFingerprintSetRejectsUnsortedAndEmptyCandidates(t *testing.T) {
	a := "v1:" + strings.Repeat("a", 64)
	b := "v2:" + strings.Repeat("b", 64)
	for _, set := range []RutFingerprintSet{{Active: a}, {Active: a, Candidates: []string{b, a}}, {Active: "_v1:" + strings.Repeat("a", 64), Candidates: []string{"_v1:" + strings.Repeat("a", 64)}}} {
		if ValidateRutFingerprintSet(set) == nil {
			t.Fatal("unsafe fingerprint set accepted")
		}
	}
}
