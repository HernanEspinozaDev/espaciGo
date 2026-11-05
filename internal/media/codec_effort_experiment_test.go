package media

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"testing"
	"time"

	"github.com/deepteams/webp"
)

// This opt-in experiment uses only deterministic synthetic corpus inputs. In
// particular, its pseudorandom texture is not a proxy for representative photos.
func TestCodecEffortExperiment(t *testing.T) {
	if os.Getenv("ESPACIGO_MEDIA_EFFORT_EXPERIMENT") != "1" {
		t.Skip("set ESPACIGO_MEDIA_EFFORT_EXPERIMENT=1 to run encoder experiment")
	}
	cases := []struct {
		name, format string
		w, h         int
		texture      int
		orientation  uint16
	}{
		{"interior_edges", "jpeg", 2400, 1600, 0, 1},
		{"exterior_texture", "jpeg", 2000, 1500, 1, 1},
		{"portrait_orientation", "jpeg", 1800, 1200, 0, 6},
		{"transparent_plan", "png", 1600, 1200, 2, 1},
		{"small_image", "png", 240, 160, 0, 1},
		{"webp_texture", "webp", 1600, 1200, 1, 1},
	}
	variants := []struct {
		name string
		opts webp.EncoderOptions
	}{
		{"q80_m4", webp.EncoderOptions{Quality: 80, Method: 4}},
		{"q80_m4_alpha_exact", webp.EncoderOptions{Quality: 80, Method: 4, AlphaCompression: 1, AlphaFiltering: 1, AlphaQuality: 100}},
		{"q80_m6", webp.EncoderOptions{Quality: 80, Method: 6}},
		{"lossless_m6", webp.EncoderOptions{Lossless: true, Quality: 80, Method: 6}},
	}
	fmt.Println("scenario\tvariant\tsize_bytes\tencode_ms\tdimensions\trgb_mae\talpha_mae\texact_pixels")
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := experimentSource(t, tc.w, tc.h, tc.texture)
			var input bytes.Buffer
			var err error
			switch tc.format {
			case "jpeg":
				err = jpeg.Encode(&input, src, &jpeg.Options{Quality: 90})
			case "png":
				err = png.Encode(&input, src)
			case "webp":
				err = webp.Encode(&input, src, &webp.EncoderOptions{Quality: 90, Method: 4})
			}
			if err != nil {
				t.Fatal(err)
			}
			raw := input.Bytes()
			if tc.orientation != 1 {
				raw = withJPEGOrientation(raw, tc.orientation)
			}
			decodedSource, format, err := image.Decode(bytes.NewReader(raw))
			if err != nil || format != tc.format {
				t.Fatalf("decode input format=%q err=%v", format, err)
			}
			if tc.format == "jpeg" {
				decodedSource = orient(decodedSource, jpegOrientation(raw))
			}
			for _, target := range []int{1600, 480} {
				reference := resize(decodedSource, target)
				for _, variant := range variants {
					var output bytes.Buffer
					start := time.Now()
					err := webp.Encode(&output, reference, &variant.opts)
					elapsed := time.Since(start)
					if err != nil {
						t.Fatalf("%s encode: %v", variant.name, err)
					}
					got, gotFormat, err := image.Decode(bytes.NewReader(output.Bytes()))
					if err != nil || gotFormat != "webp" {
						t.Fatalf("%s decode format=%q err=%v", variant.name, gotFormat, err)
					}
					if got.Bounds() != reference.Bounds() {
						t.Fatalf("%s dimensions got=%v want=%v", variant.name, got.Bounds(), reference.Bounds())
					}
					rgb, alpha, exact := experimentError(reference, got)
					if variant.name == "q80_m4_alpha_exact" && alpha != 0 {
						t.Fatal("compressed alpha is not exact")
					}
					if variant.opts.Lossless && len(output.Bytes()) < experimentBaselineSize(t, reference) && !exact {
						t.Fatalf("smaller lossless output is not pixel-exact")
					}
					if !webpHasAlpha(got) && experimentHasAlpha(reference) {
						t.Fatalf("%s lost alpha channel", variant.name)
					}
					if err := experimentCheckMetadata(output.Bytes(), reference.Bounds()); err != nil {
						t.Fatal(err)
					}
					fmt.Printf("%s_%d\t%s\t%d\t%.3f\t%dx%d\t%.4f\t%.4f\t%t\n", tc.name, target, variant.name, output.Len(), float64(elapsed.Microseconds())/1000, got.Bounds().Dx(), got.Bounds().Dy(), rgb, alpha, exact)
				}
			}
		})
	}
}

func experimentSource(t *testing.T, w, h, texture int) *image.NRGBA {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	seed := uint32(0x5a17)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := color.NRGBA{R: uint8(80 + (x*120)/w), G: uint8(60 + (y*150)/h), B: 110, A: 255}
			switch texture {
			case 1:
				seed ^= seed << 13
				seed ^= seed >> 17
				seed ^= seed << 5
				c = color.NRGBA{R: uint8(seed), G: uint8(seed >> 8), B: uint8(seed >> 16), A: 255}
			case 2:
				c.A = 96
			}
			if x%160 < 5 || y%120 < 5 {
				c = color.NRGBA{R: 16, G: 20, B: 24, A: 255}
			}
			img.SetNRGBA(x, y, c)
		}
	}
	return img
}

func experimentError(a, b image.Image) (rgbMAE, alphaMAE float64, exact bool) {
	bounds := a.Bounds()
	var rgbSum, alphaSum uint64
	pixels := uint64(bounds.Dx() * bounds.Dy())
	exact = true
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			ca, cb := colorNRGBA(a.At(x, y)), colorNRGBA(b.At(x, y))
			for _, pair := range [][2]uint8{{ca.R, cb.R}, {ca.G, cb.G}, {ca.B, cb.B}} {
				d := int(pair[0]) - int(pair[1])
				if d < 0 {
					d = -d
				}
				rgbSum += uint64(d)
			}
			d := int(ca.A) - int(cb.A)
			if d < 0 {
				d = -d
			}
			alphaSum += uint64(d)
			if ca != cb {
				exact = false
			}
		}
	}
	return float64(rgbSum) / float64(pixels*3), float64(alphaSum) / float64(pixels), exact
}

func experimentBaselineSize(t *testing.T, img image.Image) int {
	t.Helper()
	var b bytes.Buffer
	if err := webp.Encode(&b, img, &webp.EncoderOptions{Quality: 80, Method: 4}); err != nil {
		t.Fatal(err)
	}
	return b.Len()
}

func experimentHasAlpha(img image.Image) bool {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if colorNRGBA(img.At(x, y)).A != 255 {
				return true
			}
		}
	}
	return false
}
func webpHasAlpha(img image.Image) bool { return experimentHasAlpha(img) }

func experimentCheckMetadata(data []byte, bounds image.Rectangle) error {
	if len(data) < 12 || string(data[:4]) != "RIFF" || string(data[8:12]) != "WEBP" {
		return fmt.Errorf("invalid WebP RIFF header")
	}
	if int(binary.LittleEndian.Uint32(data[4:8]))+8 != len(data) {
		return fmt.Errorf("RIFF size mismatch")
	}
	for off := 12; off < len(data); {
		if off+8 > len(data) {
			return fmt.Errorf("truncated RIFF chunk")
		}
		kind := string(data[off : off+4])
		n := int(binary.LittleEndian.Uint32(data[off+4 : off+8]))
		if kind == "EXIF" || kind == "XMP " || kind == "ICCP" {
			return fmt.Errorf("unexpected metadata chunk %q", kind)
		}
		off += 8 + n + n%2
		if off > len(data) {
			return fmt.Errorf("invalid RIFF chunk length")
		}
	}
	_ = bounds
	return nil
}
