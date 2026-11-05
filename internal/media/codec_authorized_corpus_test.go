package media

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"image"
	"image/jpeg"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/deepteams/webp"
)

type authorizedMetric struct {
	Case       string  `json:"case"`
	Kind       string  `json:"kind"`
	MaxSide    int     `json:"max_side"`
	Quality    int     `json:"quality"`
	Bytes      int     `json:"bytes"`
	EncodeMS   float64 `json:"encode_ms"`
	Width      int     `json:"width"`
	Height     int     `json:"height"`
	PSNR       float64 `json:"psnr_rgb_db"`
	SSIM       float64 `json:"ssim_global_luminance"`
	AlphaError uint8   `json:"alpha_error_max"`
	Caveat     string  `json:"reference_caveat"`
}

func TestConvertAuthorizedCorpusExperiment(t *testing.T) {
	if os.Getenv("ESPACIGO_AUTHORIZED_CORPUS_EXPERIMENT") != "1" {
		t.Skip("set ESPACIGO_AUTHORIZED_CORPUS_EXPERIMENT=1")
	}
	dir := os.Getenv("ESPACIGO_AUTHORIZED_CORPUS_DIR")
	if !filepath.IsAbs(dir) {
		t.Fatal("ESPACIGO_AUTHORIZED_CORPUS_DIR must be absolute")
	}
	out := filepath.Join(dir, "results")
	if err := os.MkdirAll(out, 0700); err != nil {
		t.Fatal(err)
	}
	rows := []authorizedMetric{}
	production := []authorizedMetric{}
	manifestRaw, err := os.ReadFile(filepath.Join("..", "..", "planning", "frontend_next", "evidence", "gallery-authorized-corpus.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Sources []struct{ File, SHA256 string } `json:"sources"`
	}
	if err := json.Unmarshal(manifestRaw, &manifest); err != nil {
		t.Fatal(err)
	}
	checksums := make(map[string]string)
	for _, source := range manifest.Sources {
		checksums[source.File] = source.SHA256
	}
	if len(checksums) != 3 {
		t.Fatal("expected exactly three manifest-pinned authorized photographs")
	}
	inputs := []struct{ name, file, kind string }{{"living_room", "living_room.jpg", "authorized_photo"}, {"white_office", "white_office.jpg", "authorized_photo"}, {"wood_texture", "wood_texture.jpg", "authorized_photo"}}
	for _, c := range inputs {
		raw, e := os.ReadFile(filepath.Join(dir, c.file))
		if e != nil {
			t.Fatal(e)
		}
		h := sha256.Sum256(raw)
		if checksums[c.file] == "" || checksums[c.file] != hex.EncodeToString(h[:]) {
			t.Fatalf("missing or mismatched manifest checksum %s", c.file)
		}
		img, _, e := image.Decode(bytes.NewReader(raw))
		if e != nil {
			t.Fatal(e)
		}
		authorizedProductionMeasure(t, out, c.name, raw, img, &production)
		authorizedMeasure(t, out, c.name, img, c.kind, &rows)
	}
	for _, c := range []struct {
		name, format  string
		w, h, texture int
	}{{"transparent_plan", "png", 1600, 1200, 2}, {"exterior_texture", "jpeg", 2000, 1500, 1}, {"webp_texture", "webp", 1600, 1200, 1}} {
		src := experimentSource(t, c.w, c.h, c.texture)
		var input bytes.Buffer
		var e error
		switch c.format {
		case "png":
			e = png.Encode(&input, src)
		case "jpeg":
			e = jpeg.Encode(&input, src, &jpeg.Options{Quality: 90})
		case "webp":
			e = webp.Encode(&input, src, &webp.EncoderOptions{Quality: 90, Method: 4})
		}
		if e != nil {
			t.Fatal(e)
		}
		img, format, e := image.Decode(bytes.NewReader(input.Bytes()))
		if e != nil || format != c.format {
			t.Fatalf("fixture decode format=%q err=%v", format, e)
		}
		if c.name == "transparent_plan" && !experimentHasAlpha(img) {
			t.Fatal("transparent_plan must contain non-opaque alpha")
		}
		authorizedProductionMeasure(t, out, c.name, input.Bytes(), img, &production)
		authorizedMeasure(t, out, c.name, img, "synthetic", &rows)
	}
	b, e := json.MarshalIndent(rows, "", "  ")
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(out, "metrics.json"), append(b, '\n'), 0600); e != nil {
		t.Fatal(e)
	}
	b, e = json.MarshalIndent(production, "", "  ")
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(out, "production-metrics.json"), append(b, '\n'), 0600); e != nil {
		t.Fatal(e)
	}
	t.Logf("MEASURED_ALTERNATIVES=%d PRODUCTION_VARIANTS=%d REAL_ALPHA_ASSERTED=true MANIFEST_SHA_VERIFIED=true", len(rows), len(production))
}
func authorizedMeasure(t *testing.T, out, name string, src image.Image, kind string, rows *[]authorizedMetric) {
	t.Helper()
	for _, side := range []int{1600, 1440, 1280} {
		ref := resize(src, side)
		writeTexturePNG(t, filepath.Join(out, fmt.Sprintf("%s_%d_reference.png", name, side)), ref)
		writeTexturePNG(t, filepath.Join(out, fmt.Sprintf("%s_%d_reference_crop.png", name, side)), textureCrop(ref))
		qualities := []int{80, 75, 70}
		if name == "exterior_texture" || name == "webp_texture" {
			qualities = append(qualities, 60, 50, 40)
		}
		for _, q := range qualities {
			var buf bytes.Buffer
			start := time.Now()
			e := webp.Encode(&buf, ref, &webp.EncoderOptions{Quality: float32(q), Method: 4, AlphaCompression: 1, AlphaFiltering: 1, AlphaQuality: 100})
			ms := float64(time.Since(start).Microseconds()) / 1000
			if e != nil {
				t.Fatal(e)
			}
			got, _, e := image.Decode(bytes.NewReader(buf.Bytes()))
			if e != nil {
				t.Fatal(e)
			}
			psnr, ssim := textureMetrics(ref, got)
			maxA := uint8(0)
			for y := ref.Bounds().Min.Y; y < ref.Bounds().Max.Y; y++ {
				for x := ref.Bounds().Min.X; x < ref.Bounds().Max.X; x++ {
					a, b := colorNRGBA(ref.At(x, y)), colorNRGBA(got.At(x, y))
					d := int(a.A) - int(b.A)
					if d < 0 {
						d = -d
					}
					if uint8(d) > maxA {
						maxA = uint8(d)
					}
				}
			}
			stem := fmt.Sprintf("%s_%d_q%d", name, side, q)
			if e = os.WriteFile(filepath.Join(out, stem+".webp"), buf.Bytes(), 0600); e != nil {
				t.Fatal(e)
			}
			writeTexturePNG(t, filepath.Join(out, stem+"_crop.png"), textureCrop(got))
			*rows = append(*rows, authorizedMetric{name, kind, side, q, buf.Len(), ms, got.Bounds().Dx(), got.Bounds().Dy(), psnr, ssim, maxA, "Independent max-side resize reference; metrics exclude resize loss versus original."})
			if maxA != 0 {
				t.Errorf("%s side %d q%d alpha error=%d", name, side, q, maxA)
			}
			if math.IsNaN(psnr) {
				t.Fatal("invalid PSNR")
			}
		}
	}
}
