package declarationcrypto

import (
	"bytes"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestKeyringSealOpenRoundTrip(t *testing.T) {
	key := bytes.Repeat([]byte{0x42}, 32)
	kr, err := New("v1", map[string][]byte{"v1": key})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	version, ciphertext, err := kr.Seal([]byte("aad"), []byte("synthetic declaration"))
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	if version != "v1" {
		t.Fatalf("Seal key version = %q, want v1", version)
	}
	plaintext, err := kr.Open(version, []byte("aad"), ciphertext)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if string(plaintext) != "synthetic declaration" {
		t.Fatalf("Open plaintext mismatch")
	}
}

func TestKeyringHistoricalKeyAndAuthenticationFailures(t *testing.T) {
	kr, err := New("new", map[string][]byte{
		"new": bytes.Repeat([]byte{0x11}, 32),
		"old": bytes.Repeat([]byte{0x22}, 32),
	})
	if err != nil {
		t.Fatal(err)
	}
	old, err := New("old", map[string][]byte{"old": bytes.Repeat([]byte{0x22}, 32)})
	if err != nil {
		t.Fatal(err)
	}
	version, ciphertext, err := old.Seal([]byte("context"), []byte("historical"))
	if err != nil {
		t.Fatal(err)
	}
	if got, err := kr.Open(version, []byte("context"), ciphertext); err != nil || string(got) != "historical" {
		t.Fatalf("historical Open = %q, %v", got, err)
	}
	if _, err := kr.Open(version, []byte("wrong"), ciphertext); err == nil {
		t.Fatal("wrong AAD accepted")
	}
	ciphertext[len(ciphertext)-1] ^= 1
	if _, err := kr.Open(version, []byte("context"), ciphertext); err == nil {
		t.Fatal("tampering accepted")
	}
	if _, err := kr.Open("unknown", nil, ciphertext); err == nil {
		t.Fatal("unknown version accepted")
	}
}

func TestNewRejectsInvalidConfiguration(t *testing.T) {
	cases := []struct {
		name, current string
		keys          map[string][]byte
	}{
		{"missing current", "v2", map[string][]byte{"v1": bytes.Repeat([]byte{1}, 32)}},
		{"bad id", "bad/id", map[string][]byte{"bad/id": bytes.Repeat([]byte{1}, 32)}},
		{"wrong key size", "v1", map[string][]byte{"v1": []byte("short")}},
		{"empty", "", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := New(tc.current, tc.keys); err == nil {
				t.Fatal("expected rejection")
			}
		})
	}
}

func TestLoadFileStrictAndSecure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "keyring.json")
	valid := `{"current":"v1","keys":{"v1":"` + hex.EncodeToString(bytes.Repeat([]byte{7}, 32)) + `"}}`
	write := func(data string, mode os.FileMode) {
		t.Helper()
		if err := os.WriteFile(path, []byte(data), mode); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(path, mode); err != nil {
			t.Fatal(err)
		}
	}
	write(valid, 0600)
	if _, err := LoadFile(path); err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	for _, bad := range []string{valid + ` {}`, `{"current":"v1","keys":{"v1":"` + hex.EncodeToString(bytes.Repeat([]byte{7}, 32)) + `"},"extra":true}`, `{"current":"missing","keys":{"v1":"` + hex.EncodeToString(bytes.Repeat([]byte{7}, 32)) + `"}}`, `not json`} {
		write(bad, 0600)
		if _, err := LoadFile(path); err == nil {
			t.Fatalf("accepted invalid JSON %q", bad[:min(len(bad), 24)])
		}
	}
	write(valid, 0644)
	if _, err := LoadFile(path); err == nil {
		t.Fatal("accepted non-0600 file")
	}
	if _, err := LoadFile(filepath.Join(dir, "missing")); err == nil {
		t.Fatal("accepted missing file")
	}
	if _, err := LoadFile("relative.json"); err == nil {
		t.Fatal("accepted relative path")
	}
	link := filepath.Join(dir, "link.json")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadFile(link); err == nil {
		t.Fatal("accepted symlink")
	}
	write(stringsOf(' ', 64*1024+1), 0600)
	if _, err := LoadFile(path); err == nil {
		t.Fatal("accepted oversized file")
	}
}

func TestKeyringPlaintextSizeBoundary(t *testing.T) {
	kr, err := New("v1", map[string][]byte{"v1": bytes.Repeat([]byte{3}, 32)})
	if err != nil {
		t.Fatal(err)
	}
	for _, size := range []int{0, 64*1024 + 1} {
		if _, _, err := kr.Seal(nil, make([]byte, size)); err == nil {
			t.Fatalf("accepted plaintext size %d", size)
		}
	}
	if _, _, err := kr.Seal(nil, make([]byte, 64*1024)); err != nil {
		t.Fatalf("rejected maximum plaintext: %v", err)
	}
	if _, err := kr.Open("v1", nil, nil); err == nil {
		t.Fatal("accepted short ciphertext")
	}
}

func stringsOf(r rune, n int) string { return string(bytes.Repeat([]byte(string(r)), n)) }
