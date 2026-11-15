package media

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func TestValidateInputRejectsSingleFrameAPNG(t *testing.T) {
	raw := singleFrameAPNG(t)
	decoded, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("synthetic APNG default frame is not a valid PNG image: %v", err)
	}
	if decoded.Bounds().Dx() != 2 || decoded.Bounds().Dy() != 2 {
		t.Fatalf("decoded default frame dimensions=%dx%d, want 2x2", decoded.Bounds().Dx(), decoded.Bounds().Dy())
	}

	format, config, err := ValidateInput(raw)
	if err != ErrNotSupported {
		t.Fatalf("APNG error=%v, want ErrNotSupported", err)
	}
	if format != "" || config != (image.Config{}) {
		t.Fatalf("unsupported APNG returned metadata: format=%q config=%+v", format, config)
	}
	if _, err := Convert(raw, PurposeSpace); err != ErrNotSupported {
		t.Fatalf("Convert(APNG) error=%v, want ErrNotSupported", err)
	}
}

func singleFrameAPNG(t *testing.T) []byte {
	t.Helper()
	const width, height = 2, 2
	frame := image.NewNRGBA(image.Rect(0, 0, width, height))
	frame.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 255})
	frame.SetNRGBA(1, 1, color.NRGBA{B: 255, A: 255})
	var staticPNG bytes.Buffer
	if err := png.Encode(&staticPNG, frame); err != nil {
		t.Fatal(err)
	}
	base := staticPNG.Bytes()
	firstChunkEnd := 8 + 12 + 13 // PNG signature followed by the 13-byte IHDR chunk.

	out := append([]byte(nil), base[:firstChunkEnd]...)
	animationControl := make([]byte, 8)
	binary.BigEndian.PutUint32(animationControl[:4], 1) // One animation frame.
	out = appendPNGTestChunk(out, "acTL", animationControl)

	frameControl := make([]byte, 26)
	binary.BigEndian.PutUint32(frameControl[0:4], 0) // First animation sequence number.
	binary.BigEndian.PutUint32(frameControl[4:8], width)
	binary.BigEndian.PutUint32(frameControl[8:12], height)
	binary.BigEndian.PutUint16(frameControl[20:22], 1)  // Delay numerator.
	binary.BigEndian.PutUint16(frameControl[22:24], 10) // Delay denominator.
	out = appendPNGTestChunk(out, "fcTL", frameControl)
	return append(out, base[firstChunkEnd:]...)
}

func appendPNGTestChunk(out []byte, kind string, data []byte) []byte {
	chunk := make([]byte, 12+len(data))
	binary.BigEndian.PutUint32(chunk[:4], uint32(len(data)))
	copy(chunk[4:8], kind)
	copy(chunk[8:8+len(data)], data)
	binary.BigEndian.PutUint32(chunk[8+len(data):], crc32.ChecksumIEEE(chunk[4:8+len(data)]))
	return append(out, chunk...)
}
