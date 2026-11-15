package media

import (
	"bytes"
	"image"
	"image/jpeg"
	"image/png"
	"testing"
)

func FuzzConvertReturnsBoundedPublicVariantsOrError(f *testing.F) {
	f.Add([]byte("not-an-image"))
	for _, encode := range []func() ([]byte, error){
		func() ([]byte, error) {
			var out bytes.Buffer
			err := png.Encode(&out, image.NewNRGBA(image.Rect(0, 0, 2, 2)))
			return out.Bytes(), err
		},
		func() ([]byte, error) {
			var out bytes.Buffer
			err := jpeg.Encode(&out, image.NewNRGBA(image.Rect(0, 0, 2, 2)), &jpeg.Options{Quality: 90})
			return out.Bytes(), err
		},
	} {
		seed, err := encode()
		if err != nil {
			f.Fatal(err)
		}
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, raw []byte) {
		if len(raw) > MaxInputBytes {
			raw = raw[:MaxInputBytes]
		}
		variants, err := Convert(raw, PurposeSpace)
		if err != nil {
			return
		}
		if len(variants) != 2 {
			t.Fatalf("successful conversion returned %d variants, want 2", len(variants))
		}
		for _, variant := range variants {
			if variant.MIME != "image/webp" || variant.Width <= 0 || variant.Height <= 0 || variant.Size != int64(len(variant.Bytes)) || len(variant.Bytes) == 0 || len(variant.Bytes) > MaxInputBytes {
				t.Fatalf("successful conversion returned invalid public variant: %+v", variant)
			}
		}
	})
}
