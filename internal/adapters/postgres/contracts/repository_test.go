package contractspg

import (
	"bytes"
	"strings"
	"testing"
)

func TestSyntheticPDFAndAtRestEncryption(t *testing.T) {
	pdf := syntheticPDF("ENSAYO SINTETICO LOCAL - SIN VALIDEZ JURIDICA | Reserva sintética")
	if !bytes.HasPrefix(pdf, []byte("%PDF-1.4")) || !bytes.HasSuffix(bytes.TrimSpace(pdf), []byte("%%EOF")) || !bytes.Contains(pdf, []byte("SIN VALIDEZ JURIDICA")) {
		t.Fatalf("generated artifact is not an explicitly synthetic PDF")
	}
	key := []byte(strings.Repeat("k", 32))
	if len(key) != 32 {
		t.Fatalf("test key length %d", len(key))
	}
	ciphertext, err := encryptPDF(key, pdf)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(ciphertext, []byte("%PDF")) || bytes.Contains(ciphertext, []byte("SIN VALIDEZ")) {
		t.Fatal("private document appears unencrypted at rest")
	}
	decoded, err := decryptPDF(key, ciphertext)
	if err != nil || !bytes.Equal(decoded, pdf) {
		t.Fatalf("decrypt round trip: err=%v", err)
	}
	ciphertext[len(ciphertext)-1] ^= 1
	if _, err = decryptPDF(key, ciphertext); err == nil || !strings.Contains(err.Error(), "message authentication failed") {
		t.Fatalf("tampering was not detected: %v", err)
	}
}
