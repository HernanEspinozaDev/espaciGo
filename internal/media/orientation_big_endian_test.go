package media

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"testing"
)

func TestConvertAppliesBigEndianJPEGExifOrientation(t *testing.T) {
	source := image.NewNRGBA(image.Rect(0, 0, 3, 2))
	source.Set(0, 0, color.NRGBA{R: 255, A: 255})
	source.Set(2, 1, color.NRGBA{B: 255, A: 255})
	var encoded bytes.Buffer
	if err := jpeg.Encode(&encoded, source, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatal(err)
	}
	raw := withBigEndianJPEGOrientation(encoded.Bytes(), 6)
	if got := jpegOrientation(raw); got != 6 {
		t.Fatalf("big-endian EXIF orientation=%d, want 6", got)
	}
	variants, err := Convert(raw, PurposeSpace)
	if err != nil {
		t.Fatal(err)
	}
	if variants[0].Width != 2 || variants[0].Height != 3 {
		t.Fatalf("oriented dimensions=%dx%d, want 2x3", variants[0].Width, variants[0].Height)
	}
}

func withBigEndianJPEGOrientation(jpegData []byte, orientation uint16) []byte {
	tiff := []byte{'M', 'M', 0, 42, 0, 0, 0, 8, 0, 1, 0x01, 0x12, 0, 3, 0, 0, 0, 1, byte(orientation >> 8), byte(orientation), 0, 0, 0, 0, 0, 0}
	segment := append([]byte("Exif\x00\x00"), tiff...)
	length := uint16(len(segment) + 2)
	out := make([]byte, 0, len(jpegData)+len(segment)+4)
	out = append(out, jpegData[:2]...)
	out = append(out, 0xff, 0xe1, byte(length>>8), byte(length))
	out = append(out, segment...)
	return append(out, jpegData[2:]...)
}

func FuzzJPEGOrientationNeverPanics(f *testing.F) {
	for _, seed := range [][]byte{
		{},
		{0xff, 0xd8},
		{0xff, 0xd8, 0xff, 0xe1, 0, 1},
		{0xff, 0xd8, 0xff, 0xe1, 0, 14, 'E', 'x', 'i', 'f', 0, 0, 'I', 'I', 42, 0},
		{0xff, 0xd8, 0xff, 0xda, 0, 2},
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, raw []byte) {
		if len(raw) > MaxInputBytes {
			raw = raw[:MaxInputBytes]
		}
		orientation := jpegOrientation(raw)
		if orientation < 1 || orientation > 8 {
			t.Fatalf("orientation=%d outside EXIF range", orientation)
		}
	})
}
