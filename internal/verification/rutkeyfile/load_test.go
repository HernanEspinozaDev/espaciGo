package rutkeyfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func privateTestDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestLoadPrivateKeyringPreservesLegacyAndActive(t *testing.T) {
	path := filepath.Join(privateTestDir(t), "rut-keys.json")
	data := `{"active":"v2.2026","keys":{"v1":"` + strings.Repeat("61", 32) + `","v2.2026":"` + strings.Repeat("62", 32) + `"}}`
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := decode([]byte(data)); err != nil {
		t.Fatal("valid synthetic JSON rejected before file metadata")
	}
	ring, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	set, err := ring.Fingerprints("11222333-9")
	if err != nil || len(set.Candidates) != 2 || !strings.HasPrefix(set.Active, "v2.2026:") {
		t.Fatal("private active+legacy keyring not loaded")
	}
}

func TestLoadRejectsAmbiguousOrInvalidJSONWithoutEchoingMaterial(t *testing.T) {
	secret := strings.Repeat("ab", 32)
	validKeys := `{"v1":"` + secret + `"}`
	for _, data := range []string{
		`{"active":"v1","active":"v1","keys":` + validKeys + `}`,
		`{"active":"v1","keys":{"v1":"` + secret + `","v1":"` + secret + `"}}`,
		`{"active":"v1","keys":` + validKeys + `,"keys":` + validKeys + `}`,
		`{"active":"v1","keys":` + validKeys + `,"extra":true}`,
		`{"active":"v2","keys":` + validKeys + `}`,
		`{"active":"v1","keys":{"v1":"private-invalid-hex-material"}}`,
		`{"active":"v1","keys":` + validKeys + `} {}`,
		`null`, `[]`, `{"active":"v1","keys":null}`,
	} {
		path := filepath.Join(privateTestDir(t), "keys.json")
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		_, err := Load(path)
		if err == nil {
			t.Fatal("ambiguous or invalid keyring accepted")
		}
		if strings.Contains(err.Error(), secret) || strings.Contains(err.Error(), "private-invalid-hex-material") || strings.Contains(err.Error(), path) {
			t.Fatal("private configuration echoed in error")
		}
	}
}

func TestLoadRejectsUnsafeFileMetadata(t *testing.T) {
	dir := privateTestDir(t)
	path := filepath.Join(dir, "keys.json")
	data := `{"active":"v1","keys":{"v1":"` + strings.Repeat("61", 32) + `"}}`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("world-readable keyring accepted")
	}
	if err := os.Chmod(path, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err != nil {
		t.Fatal("valid metadata control failed")
	}
	link := filepath.Join(dir, "linked.json")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(link); err == nil {
		t.Fatal("symlink keyring accepted")
	}
	if _, err := Load("relative.json"); err == nil {
		t.Fatal("relative keyring accepted")
	}
	if err := os.Chmod(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("nonprivate parent directory accepted")
	}
}

func TestLoadRejectsOversizedFile(t *testing.T) {
	path := filepath.Join(privateTestDir(t), "oversize.json")
	if err := os.WriteFile(path, []byte(strings.Repeat("x", 65537)), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("unbounded keyring file accepted")
	}
}
