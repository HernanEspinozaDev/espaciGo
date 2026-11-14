package publiccatalog

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"testing"
)

func TestDecodeCursorRejectsMalformedPayloadWithValidSignature(t *testing.T) {
	service, err := New(memoryRepository{}, []byte("01234567890123456789012345678901"))
	if err != nil {
		t.Fatal(err)
	}
	raw := []byte("{")
	mac := hmac.New(sha256.New, service.key)
	_, _ = mac.Write(raw)
	token := "v3." + base64.RawURLEncoding.EncodeToString(raw) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	if _, err := service.decodeCursor(token); !errors.Is(err, ErrInvalid) {
		t.Fatalf("decodeCursor() error=%v, want ErrInvalid for signed malformed JSON", err)
	}
}
