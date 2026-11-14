package media

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"testing"
)

func TestConvertIgnoresMalformedJPEGExifOrientationMetadata(t *testing.T) {
	source := image.NewNRGBA(image.Rect(0, 0, 3, 2))
	source.Set(0, 0, color.NRGBA{R: 255, A: 255})
	source.Set(2, 1, color.NRGBA{B: 255, A: 255})
	var encoded bytes.Buffer
	if err := jpeg.Encode(&encoded, source, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		tiff []byte
	}{
		{name: "unknown byte order", tiff: []byte{'R', 'R', 42, 0, 8, 0, 0, 0, 0, 0}},
		{name: "invalid TIFF magic", tiff: []byte{'I', 'I', 43, 0, 8, 0, 0, 0, 0, 0}},
		{name: "IFD offset outside segment", tiff: []byte{'I', 'I', 42, 0, 0xff, 0xff, 0, 0}},
		{name: "truncated IFD entry", tiff: []byte{'I', 'I', 42, 0, 8, 0, 0, 0, 1, 0}},
		{name: "orientation tag absent", tiff: []byte{'I', 'I', 42, 0, 8, 0, 0, 0, 0, 0}},
		{name: "orientation outside EXIF range", tiff: littleEndianOrientationTIFF(9)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw := withJPEGExifTIFF(encoded.Bytes(), tc.tiff)
			if got := jpegOrientation(raw); got != 1 {
				t.Fatalf("malformed EXIF orientation=%d, want default 1", got)
			}
			variants, err := Convert(raw, PurposeSpace)
			if err != nil {
				t.Fatal(err)
			}
			if variants[0].Width != 3 || variants[0].Height != 2 {
				t.Fatalf("malformed metadata changed image dimensions to %dx%d", variants[0].Width, variants[0].Height)
			}
		})
	}
}

func littleEndianOrientationTIFF(orientation uint16) []byte {
	tiff := make([]byte, 26)
	copy(tiff, []byte{'I', 'I'})
	binary.LittleEndian.PutUint16(tiff[2:4], 42)
	binary.LittleEndian.PutUint32(tiff[4:8], 8)
	binary.LittleEndian.PutUint16(tiff[8:10], 1)
	binary.LittleEndian.PutUint16(tiff[10:12], 0x0112)
	binary.LittleEndian.PutUint16(tiff[12:14], 3)
	binary.LittleEndian.PutUint32(tiff[14:18], 1)
	binary.LittleEndian.PutUint16(tiff[18:20], orientation)
	return tiff
}

func withJPEGExifTIFF(jpegData, tiff []byte) []byte {
	segment := append([]byte("Exif\x00\x00"), tiff...)
	length := uint16(len(segment) + 2)
	out := make([]byte, 0, len(jpegData)+len(segment)+4)
	out = append(out, jpegData[:2]...)
	out = append(out, 0xff, 0xe1, byte(length>>8), byte(length))
	out = append(out, segment...)
	return append(out, jpegData[2:]...)
}
