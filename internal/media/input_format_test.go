package media

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"testing"
	"time"

	"github.com/deepteams/webp"
	"github.com/deepteams/webp/animation"
)

func TestValidateInputRejectsDecodedGIFAsUnsupported(t *testing.T) {
	palette := color.Palette{color.NRGBA{A: 0xff}, color.NRGBA{R: 0xff, A: 0xff}}
	frame := image.NewPaletted(image.Rect(0, 0, 1, 1), palette)
	frame.SetColorIndex(0, 0, 1)
	var encoded bytes.Buffer
	if err := gif.Encode(&encoded, frame, nil); err != nil {
		t.Fatal(err)
	}

	format, config, err := ValidateInput(encoded.Bytes())
	if err != ErrNotSupported {
		t.Fatalf("decoded GIF error=%v, want ErrNotSupported", err)
	}
	if format != "" || config != (image.Config{}) {
		t.Fatalf("unsupported format returned parsed metadata: format=%q config=%+v", format, config)
	}
}

func TestValidateInputRejectsAnimatedWebP(t *testing.T) {
	const width, height = 2, 2
	var encoded bytes.Buffer
	encoder := animation.NewEncoder(&encoded, width, height, nil)
	for _, pixel := range []color.NRGBA{{R: 255, A: 255}, {B: 255, A: 255}} {
		frame := image.NewNRGBA(image.Rect(0, 0, width, height))
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				frame.SetNRGBA(x, y, pixel)
			}
		}
		if err := encoder.AddFrame(frame, 100*time.Millisecond); err != nil {
			t.Fatal(err)
		}
	}
	if err := encoder.Close(); err != nil {
		t.Fatal(err)
	}

	features, err := webp.GetFeatures(bytes.NewReader(encoded.Bytes()))
	if err != nil || !features.HasAnimation || features.FrameCount != 2 {
		t.Fatalf("synthetic WebP features=%+v err=%v", features, err)
	}
	format, config, err := ValidateInput(encoded.Bytes())
	if err != ErrNotSupported {
		t.Fatalf("animated WebP error=%v, want ErrNotSupported", err)
	}
	if format != "" || config != (image.Config{}) {
		t.Fatalf("unsupported format returned metadata: format=%q config=%+v", format, config)
	}
	if _, convertErr := Convert(encoded.Bytes(), PurposeSpace); convertErr != ErrNotSupported {
		t.Fatalf("Convert(animated WebP) error=%v, want ErrNotSupported", convertErr)
	}
}
