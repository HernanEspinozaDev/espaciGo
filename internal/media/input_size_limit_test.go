package media

import (
	"bytes"
	"image"
	"image/png"
	"testing"
)

func TestValidateInputRejectsOversizedPaddedValidPNG(t *testing.T) {
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, image.NewNRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	raw := append(encoded.Bytes(), make([]byte, MaxInputBytes+1-encoded.Len())...)
	format, config, err := ValidateInput(raw)
	if err != ErrInvalid {
		t.Fatalf("oversized valid PNG error=%v, want ErrInvalid", err)
	}
	if format != "" || config != (image.Config{}) {
		t.Fatalf("oversized input returned parsed metadata: format=%q config=%+v", format, config)
	}
}
