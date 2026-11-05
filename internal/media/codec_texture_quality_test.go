package media

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/deepteams/webp"
)

// This opt-in experiment compares encoder qualities on identical 1600px
// synthetic texture references. Pseudorandom texture is not representative photography.
func TestCodecTextureQualityExperiment(t *testing.T) {
	if os.Getenv("ESPACIGO_MEDIA_TEXTURE_QUALITY") != "1" {
		t.Skip("set ESPACIGO_MEDIA_TEXTURE_QUALITY=1 to run texture experiment")
	}
	outDir := os.Getenv("ESPACIGO_MEDIA_TEXTURE_OUTPUT_DIR")
	if !filepath.IsAbs(outDir) {
		t.Fatal("ESPACIGO_MEDIA_TEXTURE_OUTPUT_DIR must be an absolute scratch directory")
	}
	if err := os.MkdirAll(outDir, 0700); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, format  string
		w, h, texture int
	}{{"exterior_texture", "jpeg", 2000, 1500, 1}, {"webp_texture", "webp", 1600, 1200, 1}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			source := textureQualitySource(tc.w, tc.h, tc.texture)
			var input bytes.Buffer
			var err error
			if tc.format == "jpeg" {
				err = jpegEncodeTexture(&input, source)
			} else {
				err = webp.Encode(&input, source, &webp.EncoderOptions{Quality: 90, Method: 4})
			}
			if err != nil {
				t.Fatal(err)
			}
			decoded, format, err := image.Decode(bytes.NewReader(input.Bytes()))
			if err != nil || format != tc.format {
				t.Fatalf("decode format=%q err=%v", format, err)
			}
			reference := resize(decoded, 1600)
			writeTexturePNG(t, filepath.Join(outDir, tc.name+"_reference.png"), reference)
			writeTexturePNG(t, filepath.Join(outDir, tc.name+"_reference_crop.png"), textureCrop(reference))
			for _, q := range []int{80, 70, 60} {
				var encoded bytes.Buffer
				start := time.Now()
				err := webp.Encode(&encoded, reference, &webp.EncoderOptions{Quality: float32(q), Method: 4, AlphaCompression: 1, AlphaFiltering: 1, AlphaQuality: 100})
				elapsed := time.Since(start)
				if err != nil {
					t.Fatal(err)
				}
				got, gotFormat, err := image.Decode(bytes.NewReader(encoded.Bytes()))
				if err != nil || gotFormat != "webp" {
					t.Fatalf("q%d decode format=%q err=%v", q, gotFormat, err)
				}
				if got.Bounds() != reference.Bounds() {
					t.Fatalf("q%d dimensions %v != %v", q, got.Bounds(), reference.Bounds())
				}
				if err := experimentCheckMetadata(encoded.Bytes(), reference.Bounds()); err != nil {
					t.Fatal(err)
				}
				_, alphaMAE, _ := experimentError(reference, got)
				if experimentHasAlpha(reference) && alphaMAE != 0 {
					t.Fatalf("q%d alpha changed", q)
				}
				psnr, ssim := textureMetrics(reference, got)
				stem := fmt.Sprintf("%s_q%d", tc.name, q)
				if err := os.WriteFile(filepath.Join(outDir, stem+".webp"), encoded.Bytes(), 0600); err != nil {
					t.Fatal(err)
				}
				writeTexturePNG(t, filepath.Join(outDir, stem+"_decoded.png"), got)
				writeTexturePNG(t, filepath.Join(outDir, stem+"_crop.png"), textureCrop(got))
				t.Logf("scenario=%s quality=%d bytes=%d encode_ms=%.3f PSNR_RGB_dB=%.4f SSIM=%.6f dimensions=%dx%d", tc.name, q, encoded.Len(), float64(elapsed.Microseconds())/1000, psnr, ssim, got.Bounds().Dx(), got.Bounds().Dy())
			}
		})
	}
}

func jpegEncodeTexture(w *bytes.Buffer, img image.Image) error {
	return jpeg.Encode(w, img, &jpeg.Options{Quality: 90})
}

func textureQualitySource(w, h, texture int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	seed := uint32(0x5a17)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := color.NRGBA{R: uint8(80 + (x*120)/w), G: uint8(60 + (y*150)/h), B: 110, A: 255}
			if texture == 1 {
				seed ^= seed << 13
				seed ^= seed >> 17
				seed ^= seed << 5
				c = color.NRGBA{R: uint8(seed), G: uint8(seed >> 8), B: uint8(seed >> 16), A: 255}
			}
			if x%160 < 5 || y%120 < 5 {
				c = color.NRGBA{R: 16, G: 20, B: 24, A: 255}
			}
			img.SetNRGBA(x, y, c)
		}
	}
	return img
}

func writeTexturePNG(t *testing.T, path string, img image.Image) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func textureCrop(img image.Image) image.Image {
	b := img.Bounds()
	w, h := b.Dx()/2, b.Dy()/2
	return img.(interface {
		SubImage(image.Rectangle) image.Image
	}).SubImage(image.Rect(b.Min.X+w/2, b.Min.Y+h/2, b.Min.X+3*w/2, b.Min.Y+3*h/2))
}

func textureMetrics(a, b image.Image) (psnr, ssim float64) {
	bounds := a.Bounds()
	var squared float64
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			ca, cb := colorNRGBA(a.At(x, y)), colorNRGBA(b.At(x, y))
			for _, pair := range [][2]uint8{{ca.R, cb.R}, {ca.G, cb.G}, {ca.B, cb.B}} {
				d := float64(pair[0]) - float64(pair[1])
				squared += d * d
			}
		}
	}
	mse := squared / float64(bounds.Dx()*bounds.Dy()*3)
	if mse == 0 {
		psnr = math.Inf(1)
	} else {
		psnr = 10 * math.Log10(255*255/mse)
	}
	// Global luminance SSIM; stable, dependency-free, and explicitly not a local/windowed variant.
	var sx, sy, sxx, syy, sxy float64
	n := float64(bounds.Dx() * bounds.Dy())
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			ca, cb := colorNRGBA(a.At(x, y)), colorNRGBA(b.At(x, y))
			lx := .2126*float64(ca.R) + .7152*float64(ca.G) + .0722*float64(ca.B)
			ly := .2126*float64(cb.R) + .7152*float64(cb.G) + .0722*float64(cb.B)
			sx += lx
			sy += ly
			sxx += lx * lx
			syy += ly * ly
			sxy += lx * ly
		}
	}
	mx, my := sx/n, sy/n
	vx, vy := sxx/n-mx*mx, syy/n-my*my
	cov := sxy/n - mx*my
	c1, c2 := 6.5025, 58.5225
	ssim = ((2*mx*my + c1) * (2*cov + c2)) / ((mx*mx + my*my + c1) * (vx + vy + c2))
	return
}
