package media

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"

	"github.com/deepteams/webp"
)

func TestConvertProducesBoundedWebPVariantsAndPreservesAlpha(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 800, 400))
	for y := 0; y < 400; y++ {
		for x := 0; x < 800; x++ {
			if x < 400 {
				img.SetNRGBA(x, y, color.NRGBA{R: 240, A: 255})
			} else {
				img.SetNRGBA(x, y, color.NRGBA{B: 240, A: 128})
			}
		}
	}
	var src bytes.Buffer
	if err := png.Encode(&src, img); err != nil {
		t.Fatal(err)
	}
	variants, err := Convert(src.Bytes(), PurposeSpace)
	if err != nil {
		t.Fatal(err)
	}
	if len(variants) != 2 || variants[0].Name != VariantDisplay || variants[1].Name != VariantThumbnail {
		t.Fatalf("variants=%+v", variants)
	}
	if variants[0].Width != 800 || variants[0].Height != 400 || variants[1].Width != 480 || variants[1].Height != 240 {
		t.Fatalf("dimensions=%dx%d %dx%d", variants[0].Width, variants[0].Height, variants[1].Width, variants[1].Height)
	}
	features, err := webp.GetFeatures(bytes.NewReader(variants[0].Bytes))
	if err != nil || features.HasAnimation {
		t.Fatalf("display feature=%+v err=%v", features, err)
	}
	decoded, _, err := image.Decode(bytes.NewReader(variants[0].Bytes))
	if err != nil {
		t.Fatal(err)
	}
	_, _, _, alpha := decoded.At(600, 200).RGBA()
	if alpha == 0xffff {
		t.Fatalf("alpha channel was lost: %x", alpha)
	}
}

func TestConvertAppliesJPEGExifOrientationAndDropsMetadata(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 3, 2))
	img.Set(0, 0, color.NRGBA{R: 255, A: 255})
	img.Set(2, 1, color.NRGBA{B: 255, A: 255})
	var jpegBytes bytes.Buffer
	if err := jpeg.Encode(&jpegBytes, img, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatal(err)
	}
	raw := withJPEGOrientation(jpegBytes.Bytes(), 6)
	if got := jpegOrientation(raw); got != 6 {
		t.Fatalf("orientation=%d", got)
	}
	variants, err := Convert(raw, PurposeSpace)
	if err != nil {
		t.Fatal(err)
	}
	if variants[0].Width != 2 || variants[0].Height != 3 {
		t.Fatalf("oriented dimensions=%dx%d", variants[0].Width, variants[0].Height)
	}
	if bytes.Contains(variants[0].Bytes, []byte("Exif")) {
		t.Fatal("source EXIF metadata was retained")
	}
}

func TestValidateInputRejectsAnimatedPNGAndDimensionBombs(t *testing.T) {
	animated := []byte{137, 80, 78, 71, 13, 10, 26, 10, 0, 0, 0, 8, 'a', 'c', 'T', 'L', 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0}
	if _, _, err := ValidateInput(animated); err != ErrNotSupported {
		t.Fatalf("animated PNG error=%v", err)
	}
	configOnly := pngConfig(12001, 1)
	if _, _, err := ValidateInput(configOnly); err != ErrInvalid {
		t.Fatalf("oversized dimensions error=%v", err)
	}
	if _, _, err := ValidateInput([]byte("<svg onload=alert(1) />")); err != ErrInvalid && err != ErrNotSupported {
		t.Fatalf("SVG error=%v", err)
	}
}

func FuzzValidateInputNeverPanics(f *testing.F) {
	f.Add([]byte("not-an-image"))
	var seed bytes.Buffer
	_ = png.Encode(&seed, image.NewNRGBA(image.Rect(0, 0, 2, 2)))
	f.Add(seed.Bytes())
	f.Fuzz(func(t *testing.T, input []byte) {
		if len(input) > MaxInputBytes {
			t.Skip()
		}
		_, _, _ = ValidateInput(input)
	})
}

func BenchmarkConvertRepresentativeJPEG(b *testing.B) {
	img := image.NewNRGBA(image.Rect(0, 0, 1600, 1200))
	for y := 0; y < 1200; y++ {
		for x := 0; x < 1600; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: uint8(x), G: uint8(y), B: uint8(x + y), A: 255})
		}
	}
	var src bytes.Buffer
	if err := jpeg.Encode(&src, img, &jpeg.Options{Quality: 85}); err != nil {
		b.Fatal(err)
	}
	raw := append([]byte(nil), src.Bytes()...)
	b.SetBytes(int64(len(raw)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Convert(raw, PurposeSpace); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkConvertMaximumPixelsJPEG(b *testing.B) {
	img := image.NewNRGBA(image.Rect(0, 0, 5000, 5000))
	for y := 0; y < 5000; y++ {
		for x := 0; x < 5000; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: uint8(x*31 + y), G: uint8(y*17 + x), B: uint8(x + y*3), A: 255})
		}
	}
	var src bytes.Buffer
	if err := jpeg.Encode(&src, img, &jpeg.Options{Quality: 80}); err != nil {
		b.Fatal(err)
	}
	raw := append([]byte(nil), src.Bytes()...)
	b.SetBytes(int64(len(raw)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Convert(raw, PurposeSpace); err != nil {
			b.Fatal(err)
		}
	}
}

func withJPEGOrientation(jpegData []byte, orientation uint16) []byte {
	tiff := []byte{'I', 'I', 42, 0, 8, 0, 0, 0, 1, 0, 0x12, 0x01, 3, 0, 1, 0, 0, 0, byte(orientation), byte(orientation >> 8), 0, 0, 0, 0, 0, 0}
	segment := append([]byte("Exif\x00\x00"), tiff...)
	length := uint16(len(segment) + 2)
	out := make([]byte, 0, len(jpegData)+len(segment)+4)
	out = append(out, jpegData[:2]...)
	out = append(out, 0xff, 0xe1, byte(length>>8), byte(length))
	out = append(out, segment...)
	out = append(out, jpegData[2:]...)
	return out
}

func pngConfig(width, height uint32) []byte {
	var b bytes.Buffer
	b.Write([]byte{137, 80, 78, 71, 13, 10, 26, 10})
	writeChunk := func(kind string, data []byte) {
		_ = binary.Write(&b, binary.BigEndian, uint32(len(data)))
		b.WriteString(kind)
		b.Write(data)
		crcData := append([]byte(kind), data...)
		_ = binary.Write(&b, binary.BigEndian, crc32.ChecksumIEEE(crcData))
	}
	ihdr := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdr[:4], width)
	binary.BigEndian.PutUint32(ihdr[4:8], height)
	ihdr[8] = 8
	ihdr[9] = 6
	writeChunk("IHDR", ihdr)
	writeChunk("IEND", nil)
	return b.Bytes()
}
