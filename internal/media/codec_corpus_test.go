package media

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"

	"github.com/deepteams/webp"
)

// Synthetic scenarios measure texture/edges/alpha, not photographic quality.
// SLO misses are reported, not treated as an unapproved hard upload limit.
func TestConvertSyntheticScenarioCorpus(t *testing.T) {
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
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			img := image.NewNRGBA(image.Rect(0, 0, tc.w, tc.h))
			seed := uint32(0x5a17)
			for y := 0; y < tc.h; y++ {
				for x := 0; x < tc.w; x++ {
					c := color.NRGBA{R: uint8(80 + (x*120)/tc.w), G: uint8(60 + (y*150)/tc.h), B: 110, A: 255}
					switch tc.texture {
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
			var src bytes.Buffer
			var err error
			switch tc.format {
			case "jpeg":
				err = jpeg.Encode(&src, img, &jpeg.Options{Quality: 90})
			case "png":
				err = png.Encode(&src, img)
			case "webp":
				err = webp.Encode(&src, img, &webp.EncoderOptions{Quality: 90, Method: 4})
			}
			if err != nil {
				t.Fatal(err)
			}
			raw := src.Bytes()
			if tc.orientation != 1 {
				raw = withJPEGOrientation(raw, tc.orientation)
			}
			variants, err := Convert(raw, PurposeSpace)
			if err != nil {
				t.Fatal(err)
			}
			if len(variants) != 2 {
				t.Fatalf("got %d variants", len(variants))
			}
			for _, v := range variants {
				maxSide, slo := 1600, int64(600*1024)
				if v.Name == VariantThumbnail {
					maxSide, slo = 480, 100*1024
				} else if v.Name != VariantDisplay {
					t.Fatalf("unexpected variant %q", v.Name)
				}
				decoded, format, err := image.Decode(bytes.NewReader(v.Bytes))
				if err != nil || format != "webp" {
					t.Fatalf("decode format=%q err=%v", format, err)
				}
				if v.MIME != "image/webp" || v.Size != int64(len(v.Bytes)) || v.SHA256 != digestHex(v.Bytes) || decoded.Bounds().Dx() != v.Width || decoded.Bounds().Dy() != v.Height {
					t.Fatal("variant metadata does not match encoded bytes")
				}
				if v.Width > maxSide || v.Height > maxSide || v.Width < 1 || v.Height < 1 {
					t.Fatalf("invalid dimensions %dx%d", v.Width, v.Height)
				}
				if tc.orientation == 6 && v.Width >= v.Height {
					t.Fatal("portrait EXIF orientation not applied")
				}
				if tc.name == "small_image" && (v.Width != tc.w || v.Height != tc.h) {
					t.Fatal("small input was enlarged")
				}
				for off := 12; off < len(v.Bytes); {
					if off+8 > len(v.Bytes) {
						t.Fatal("truncated RIFF chunk")
					}
					kind := string(v.Bytes[off : off+4])
					n := int(binary.LittleEndian.Uint32(v.Bytes[off+4 : off+8]))
					if kind == "EXIF" || kind == "XMP " || kind == "ICCP" || kind == "ANIM" || kind == "ANMF" {
						t.Fatalf("unexpected metadata/animation chunk %q", kind)
					}
					off += 8 + n + n%2
					if off > len(v.Bytes) {
						t.Fatal("invalid RIFF chunk size")
					}
				}
				t.Logf("input_format=%s input_bytes=%d variant=%s dimensions=%dx%d bytes=%d slo_bytes=%d slo_met=%t", tc.format, len(raw), v.Name, v.Width, v.Height, v.Size, slo, v.Size <= slo)
			}
		})
	}
}
