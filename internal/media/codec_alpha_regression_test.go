package media

import (
	"bytes"
	"image"
	"image/color"
	"testing"

	"github.com/deepteams/webp"
)

func TestEncodeVariantPreservesAlphaExactly(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 32, 32))
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			src.SetNRGBA(x, y, color.NRGBA{R: 120, G: 90, B: 180, A: uint8((x*7 + y*11) % 256)})
		}
	}
	variant, err := encodeVariant(VariantThumbnail, src, 480)
	if err != nil {
		t.Fatal(err)
	}
	decoded, _, err := image.Decode(bytes.NewReader(variant.Bytes))
	if err != nil {
		t.Fatal(err)
	}
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			_, _, _, got := decoded.At(x, y).RGBA()
			if got>>8 != uint32(src.NRGBAAt(x, y).A) {
				t.Fatalf("alpha changed at %d,%d: got=%d want=%d", x, y, got>>8, src.NRGBAAt(x, y).A)
			}
		}
	}
}

func TestEncodeVariantCompressesSyntheticAlphaWithoutChangingOpaqueCodec(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 1600, 1200))
	for y := 0; y < 1200; y++ {
		for x := 0; x < 1600; x++ {
			src.SetNRGBA(x, y, color.NRGBA{R: 120, G: 90, B: 180, A: 96})
		}
	}
	for _, tc := range []struct {
		name   VariantName
		side   int
		budget int64
	}{{VariantDisplay, 1600, 614400}, {VariantThumbnail, 480, 102400}} {
		variant, err := encodeVariant(tc.name, src, tc.side)
		if err != nil {
			t.Fatal(err)
		}
		if variant.Size > tc.budget {
			t.Fatalf("constant-alpha %s bytes=%d exceed synthetic budget=%d", tc.name, variant.Size, tc.budget)
		}
	}
}

func TestEncodeVariantOpaqueBytesRemainUnchanged(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 120, 80))
	for y := 0; y < 80; y++ {
		for x := 0; x < 120; x++ {
			src.SetNRGBA(x, y, color.NRGBA{R: uint8(x * 2), G: uint8(y * 3), B: 110, A: 255})
		}
	}
	var legacy bytes.Buffer
	if err := webp.Encode(&legacy, src, &webp.EncoderOptions{Quality: 80, Method: 4}); err != nil {
		t.Fatal(err)
	}
	variant, err := encodeVariant(VariantThumbnail, src, 480)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(legacy.Bytes(), variant.Bytes) {
		t.Fatal("opaque RGB codec bytes changed")
	}
}
