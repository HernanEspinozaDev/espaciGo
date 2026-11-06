package documentcodec

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"errors"
	"testing"
)

func TestCodecRoundTripsScopedDocument(t *testing.T) {
	key := bytes.Repeat([]byte{0x42}, 32)
	scope, err := IdentityScope("11111111-1111-4111-8111-111111111111", "44444444-4444-4444-8444-444444444444", "55555555-5555-4555-8555-555555555555")
	if err != nil {
		t.Fatal(err)
	}
	plain := []byte("synthetic")
	sealed, err := Seal(key, scope, plain)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Open(key, scope, sealed)
	if err != nil || !bytes.Equal(got, plain) {
		t.Fatalf("Open()=%q, %v", got, err)
	}
}

func TestCodecRejectsInvalidKeyAndEmptyOrOversizeContent(t *testing.T) {
	scope, _ := IdentityScope("11111111-1111-4111-8111-111111111111", "44444444-4444-4444-8444-444444444444", "55555555-5555-4555-8555-555555555555")
	for _, tc := range []struct {
		name         string
		key, content []byte
	}{{"short-key", []byte("short"), []byte("x")}, {"empty", bytes.Repeat([]byte{1}, 32), nil}, {"large", bytes.Repeat([]byte{1}, 32), make([]byte, MaxDocumentBytes+1)}} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Seal(tc.key, scope, tc.content); !errors.Is(err, ErrInvalid) {
				t.Fatalf("Seal error=%v, want ErrInvalid", err)
			}
		})
	}
}

func TestCodecLegacyAESGCMCompatibilityAndTamperRejection(t *testing.T) {
	key := bytes.Repeat([]byte{0x42}, 32)
	scope, _ := IdentityScope("11111111-1111-4111-8111-111111111111", "44444444-4444-4444-8444-444444444444", "55555555-5555-4555-8555-555555555555")
	plain := []byte("legacy independent fixture")
	legacy := legacySeal(t, key, scope.aad, bytes.Repeat([]byte{7}, 12), plain)
	got, err := Open(key, scope, legacy)
	if err != nil || !bytes.Equal(got, plain) {
		t.Fatalf("legacy Open=%q, %v", got, err)
	}
	encoded, err := Seal(key, scope, plain)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := legacyOpen(t, key, scope.aad, encoded)
	if err != nil || !bytes.Equal(decoded, plain) {
		t.Fatalf("legacy decoder=%q, %v", decoded, err)
	}
	for _, part := range []string{"header", "nonce", "ciphertext", "tag", "truncated"} {
		t.Run(part, func(t *testing.T) {
			broken := append([]byte(nil), encoded...)
			switch part {
			case "header":
				broken[0] ^= 1
			case "nonce":
				broken[len(magic)] ^= 1
			case "ciphertext":
				broken[len(magic)+12] ^= 1
			case "tag":
				broken[len(broken)-1] ^= 1
			case "truncated":
				broken = broken[:len(broken)-1]
			}
			if out, e := Open(key, scope, broken); !errors.Is(e, ErrCorrupt) || out != nil {
				t.Fatalf("Open returned %d bytes, %v", len(out), e)
			}
		})
	}
	if out, e := Open(key, scope, append(encoded, 1)); !errors.Is(e, ErrCorrupt) || out != nil {
		t.Fatalf("oversize envelope returned %d, %v", len(out), e)
	}
}

func legacySeal(t *testing.T, key []byte, aad string, nonce, plaintext []byte) []byte {
	t.Helper()
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	a, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	out := append([]byte(magic), nonce...)
	return a.Seal(out, nonce, plaintext, []byte(aad))
}
func legacyOpen(t *testing.T, key []byte, aad string, envelope []byte) ([]byte, error) {
	t.Helper()
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	a, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	n := len(magic)
	return a.Open(nil, envelope[n:n+12], envelope[n+12:], []byte(aad))
}
