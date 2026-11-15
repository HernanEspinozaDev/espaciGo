package verification

import (
	"strings"
	"testing"
)

func TestRutHMACKeyringFingerprintsLegacyAndActive(t *testing.T) {
	keys := map[string][]byte{"v1": []byte("legacy-synthetic-hmac-secret"), "v2": []byte("active-synthetic-hmac-secret")}
	kr, err := NewRutHMACKeyring("v2", keys)
	if err != nil {
		t.Fatal(err)
	}
	got, err := kr.Fingerprints("11.222.333–9")
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := ComputeRutFingerprint("11222333-9", keys["v1"], "v1")
	if err != nil {
		t.Fatal(err)
	}
	active, err := ComputeRutFingerprint("11222333-9", keys["v2"], "v2")
	if err != nil {
		t.Fatal(err)
	}
	if got.Active != active || len(got.Candidates) != 2 || got.Candidates[0] != legacy || got.Candidates[1] != active {
		t.Fatalf("unexpected fingerprints: %#v", got)
	}
}

func TestRutHMACKeyringCopiesKeysAndVersions(t *testing.T) {
	secret := []byte("synthetic-key-material-strong")
	keys := map[string][]byte{"v1": secret}
	kr, err := NewRutHMACKeyring("v1", keys)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := ComputeRutFingerprint("11222333-9", secret, "v1")
	secret[0] ^= 0xff
	keys["v1"][1] ^= 0xff
	got, err := kr.Fingerprints("11222333-9")
	if err != nil || got.Active != want {
		t.Fatalf("key material was not copied: %#v, %v", got, err)
	}
	versions := kr.Versions()
	versions[0] = "corrupted"
	if again := kr.Versions(); len(again) != 1 || again[0] != "v1" {
		t.Fatalf("versions not defensive: %v", again)
	}
}

func TestRutHMACKeyringOrderIndependent(t *testing.T) {
	a := map[string][]byte{"v1": []byte("legacy-synthetic-hmac-secret"), "v2": []byte("active-synthetic-hmac-secret")}
	b := map[string][]byte{"v2": a["v2"], "v1": a["v1"]}
	ka, ea := NewRutHMACKeyring("v2", a)
	kb, eb := NewRutHMACKeyring("v2", b)
	if ea != nil || eb != nil {
		t.Fatalf("constructors: %v %v", ea, eb)
	}
	fa, _ := ka.Fingerprints("11222333-9")
	fb, _ := kb.Fingerprints("11222333-9")
	if strings.Join(ka.Versions(), ",") != "v1,v2" || strings.Join(kb.Versions(), ",") != "v1,v2" || fa.Active != fb.Active || strings.Join(fa.Candidates, ",") != strings.Join(fb.Candidates, ",") {
		t.Fatal("map insertion order affected output")
	}
}

func TestRutFingerprintSetValidationRejectsInvalidSets(t *testing.T) {
	valid := RutFingerprintSet{Active: "v2:" + strings.Repeat("a", 64), Candidates: []string{"v1:" + strings.Repeat("b", 64), "v2:" + strings.Repeat("a", 64)}}
	if err := ValidateRutFingerprintSet(valid); err != nil {
		t.Fatalf("valid set: %v", err)
	}
	cases := []RutFingerprintSet{
		{Active: valid.Active, Candidates: []string{valid.Candidates[0], valid.Candidates[0], valid.Active}},
		{Active: "v3:" + strings.Repeat("a", 64), Candidates: []string{valid.Candidates[0]}},
		{Active: "v2:" + strings.Repeat("A", 64), Candidates: []string{"v2:" + strings.Repeat("A", 64)}},
		{Active: "v2:" + strings.Repeat("a", 64), Candidates: []string{"v2:" + strings.Repeat("b", 64)}},
		{Active: "v1:" + strings.Repeat("a", 64), Candidates: []string{"bad!x:" + strings.Repeat("a", 64)}},
	}
	for i, tc := range cases {
		if err := ValidateRutFingerprintSet(tc); err == nil {
			t.Errorf("case %d accepted", i)
		}
	}
}

func TestRutHMACKeyringRejectsInvalidConfigurationWithoutSecrets(t *testing.T) {

	cases := []struct {
		active string
		keys   map[string][]byte
	}{
		{"v2", map[string][]byte{"v1": []byte("legacy-synthetic-hmac-secret")}},
		{"v1", map[string][]byte{"v1": []byte("too-short")}},
		{"v1", map[string][]byte{"bad!": []byte("synthetic-key-material-strong")}},
	}
	for _, tc := range cases {
		_, err := NewRutHMACKeyring(tc.active, tc.keys)
		if err == nil {
			t.Fatal("invalid configuration accepted")
		}
		for _, secret := range tc.keys {
			if strings.Contains(err.Error(), string(secret)) {
				t.Fatal("error leaked the supplied secret")
			}
		}
	}
}
