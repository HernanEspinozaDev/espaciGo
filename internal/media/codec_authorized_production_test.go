package media

import (
	"bytes"
	"image"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func authorizedProductionMeasure(t *testing.T, out, name string, raw []byte, src image.Image, rows *[]authorizedMetric) {
	t.Helper()
	start := time.Now()
	variants, err := Convert(raw, PurposeSpace)
	elapsed := float64(time.Since(start).Microseconds()) / 1000
	if err != nil || len(variants) != 2 {
		t.Fatalf("production Convert %s failed: %v", name, err)
	}
	for _, v := range variants {
		side := 1600
		limit := 614400
		if v.Name == VariantThumbnail {
			side = 480
			limit = 102400
		} else if v.Name != VariantDisplay {
			t.Fatal("unexpected production variant")
		}
		ref := resize(src, side)
		quality := 80
		if v.Name == VariantDisplay && max(v.Width, v.Height) < max(ref.Bounds().Dx(), ref.Bounds().Dy()) {
			if max(v.Width, v.Height) != 1280 || v.Size > approvedDisplayByteBudget {
				t.Fatal("unexpected production fallback outside approved1280/Q75 floor")
			}
			side, quality = 1280, 75
			ref = resize(src, side)
		}
		got, format, err := image.Decode(bytes.NewReader(v.Bytes))
		if err != nil || format != "webp" || got.Bounds() != ref.Bounds() || v.Width != got.Bounds().Dx() || v.Height != got.Bounds().Dy() || v.Size != int64(len(v.Bytes)) {
			t.Fatal("production derivative metadata/dimension mismatch", name, v.Name)
		}
		if err = experimentCheckMetadata(v.Bytes, ref.Bounds()); err != nil {
			t.Fatal(err)
		}
		maxAlpha := uint8(0)
		for y := ref.Bounds().Min.Y; y < ref.Bounds().Max.Y; y++ {
			for x := ref.Bounds().Min.X; x < ref.Bounds().Max.X; x++ {
				a, b := colorNRGBA(ref.At(x, y)), colorNRGBA(got.At(x, y))
				d := int(a.A) - int(b.A)
				if d < 0 {
					d = -d
				}
				if uint8(d) > maxAlpha {
					maxAlpha = uint8(d)
				}
			}
		}
		if maxAlpha != 0 {
			t.Fatal("production alpha differs", name, v.Name)
		}
		psnr, ssim := textureMetrics(ref, got)
		stem := name + "_production_" + string(v.Name)
		if err = os.WriteFile(filepath.Join(out, stem+".webp"), v.Bytes, 0600); err != nil {
			t.Fatal(err)
		}
		writeTexturePNG(t, filepath.Join(out, stem+"_crop.png"), textureCrop(got))
		*rows = append(*rows, authorizedMetric{Case: name, Kind: string(v.Name), MaxSide: side, Quality: quality, Bytes: len(v.Bytes), EncodeMS: elapsed, Width: v.Width, Height: v.Height, PSNR: psnr, SSIM: ssim, AlphaError: maxAlpha, Caveat: "Production Convert, including approved1280/Q75 display fallback. EncodeMS is the shared total Convert duration, not per-variant time. Metrics use same-size independently resized decoded-input reference, excluding resize loss."})
		t.Logf("PRODUCTION case=%s variant=%s dimensions=%dx%d bytes=%d target=%d weight_pass=%t alpha_exact=true metadata_removed=true", name, v.Name, v.Width, v.Height, len(v.Bytes), limit, len(v.Bytes) <= limit)
	}

}
