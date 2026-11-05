package media

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/deepteams/webp"
)

// Optional synthetic inputs for the native browser regression. Expected output
// is exported only after the independent original-pixel floor assertions pass.
func exportNativeQualityInput(t *testing.T, name string, raw []byte, variants []Variant, rejected bool) {
	t.Helper()
	dir := os.Getenv("ESPACIGO_NATIVE_GALLERY_INPUT_DIR")
	if dir == "" {
		return
	}
	info, err := os.Lstat(dir)
	if err != nil || !filepath.IsAbs(dir) || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0700 {
		t.Fatal("native input export requires an existing private absolute directory")
	}
	digest := sha256.Sum256(raw)
	metadata := map[string]any{"input_sha256": hex.EncodeToString(digest[:]), "rejected_at_approved_floor": rejected}
	if !rejected {
		for _, v := range variants {
			hash := sha256.Sum256(v.Bytes)
			metadata[string(v.Name)] = map[string]any{"sha256": hex.EncodeToString(hash[:]), "bytes": v.Size, "width": v.Width, "height": v.Height}
		}
	}
	encoded, err := json.Marshal(metadata)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, name), raw, 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, name+".json"), encoded, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestDisplayBudgetFallbackUsesOriginalPixels1280Q75(t *testing.T) {
	for _, format := range []string{"jpeg", "webp"} {
		t.Run(format, func(t *testing.T) {
			src := textureQualitySource(2000, 1500, 1)
			if format == "webp" {
				src = textureQualitySource(1600, 1200, 1)
			}
			var raw bytes.Buffer
			var err error
			if format == "jpeg" {
				err = jpegEncodeTexture(&raw, src)
			} else {
				err = webp.Encode(&raw, src, &webp.EncoderOptions{Quality: 90, Method: 4})
			}
			if err != nil {
				t.Fatal(err)
			}
			decoded, _, err := image.Decode(bytes.NewReader(raw.Bytes()))
			if err != nil {
				t.Fatal(err)
			}
			var expected bytes.Buffer
			if err = webp.Encode(&expected, resize(decoded, 1280), &webp.EncoderOptions{Quality: 75, Method: 4, AlphaCompression: 1, AlphaFiltering: 1, AlphaQuality: 100}); err != nil {
				t.Fatal(err)
			}
			variants, err := Convert(raw.Bytes(), PurposeSpace)
			if err != nil {
				t.Fatal(err)
			}
			display := variants[0]
			if display.Width != 1280 || display.Height != 960 || display.Size > 614400 || !bytes.Equal(display.Bytes, expected.Bytes()) {
				t.Fatalf("display must use the approved1280/Q75 original-pixel fallback; got%dx%d size%d", display.Width, display.Height, display.Size)
			}
			if err = experimentCheckMetadata(display.Bytes, image.Rect(0, 0, 1280, 960)); err != nil {
				t.Fatal(err)
			}
			exportNativeQualityInput(t, "codec-"+format+"."+format, raw.Bytes(), variants, false)
		})
	}
}
func TestDisplayBudgetPreservesExistingUnderBudgetOutput(t *testing.T) {
	src := experimentSource(t, 1600, 1200, 2)
	for _, name := range []VariantName{VariantDisplay, VariantThumbnail, VariantAvatar} {
		side := 1600
		if name == VariantThumbnail {
			side = 480
		}
		if name == VariantAvatar {
			side = 512
		}
		var expected bytes.Buffer
		if err := webp.Encode(&expected, resize(src, side), &webp.EncoderOptions{Quality: 80, Method: 4, AlphaCompression: 1, AlphaFiltering: 1, AlphaQuality: 100}); err != nil {
			t.Fatal(err)
		}
		got, err := encodeVariant(name, src, side)
		if err != nil || !bytes.Equal(got.Bytes, expected.Bytes()) {
			t.Fatal("existing under-budget output changed", name, err)
		}
	}
}
func TestDisplayBudgetRejectsRatherThanDegradingBelowApprovedFloor(t *testing.T) {
	src := textureQualitySource(1600, 1200, 1)
	seed := uint32(0x7915)
	for y := 0; y < 1200; y++ {
		for x := 0; x < 1600; x++ {
			seed ^= seed << 13
			seed ^= seed >> 17
			seed ^= seed << 5
			src.Pix[y*src.Stride+x*4+3] = uint8(seed)
		}
	}
	var raw bytes.Buffer
	if err := png.Encode(&raw, src); err != nil {
		t.Fatal(err)
	}
	if len(raw.Bytes()) > MaxInputBytes {
		t.Fatal("negative fixture must be a valid-size source")
	}
	var floor bytes.Buffer
	if err := webp.Encode(&floor, resize(src, 1280), &webp.EncoderOptions{Quality: 75, Method: 4, AlphaCompression: 1, AlphaFiltering: 1, AlphaQuality: 100}); err != nil {
		t.Fatal(err)
	}
	if floor.Len() <= 614400 {
		t.Fatal("negative must actually exceed budget at approved floor")
	}
	got, err := Convert(raw.Bytes(), PurposeSpace)
	if !errors.Is(err, ErrInvalid) || got != nil {
		t.Fatal("oversized approved-floor output must be rejected, not degraded or returned")
	}
	exportNativeQualityInput(t, "overfloor.png", raw.Bytes(), nil, true)
}
