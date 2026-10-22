package media

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"math"

	"github.com/deepteams/webp"
)

// Convert decodes a bounded still JPEG/PNG/WebP image, applies JPEG EXIF
// orientation, strips all source metadata by re-encoding pixels only, and
// returns the shared display/thumbnail or private avatar derivatives.
func Convert(raw []byte, purpose Purpose) ([]Variant, error) {
	if len(raw) == 0 || len(raw) > MaxInputBytes || (purpose != PurposeSpace && purpose != PurposeProfile) {
		return nil, ErrInvalid
	}
	format, _, err := ValidateInput(raw)
	if err != nil {
		return nil, err
	}
	decoded, actualFormat, err := image.Decode(bytes.NewReader(raw))
	if err != nil || actualFormat != format {
		return nil, ErrInvalid
	}
	if format == "jpeg" {
		decoded = orient(decoded, jpegOrientation(raw))
	}
	if purpose == PurposeProfile {
		v, err := encodeVariant(VariantAvatar, decoded, 512)
		if err != nil {
			return nil, err
		}
		return []Variant{v}, nil
	}
	display, err := encodeVariant(VariantDisplay, decoded, 1600)
	if err != nil {
		return nil, err
	}
	thumbnail, err := encodeVariant(VariantThumbnail, decoded, 480)
	if err != nil {
		return nil, err
	}
	return []Variant{display, thumbnail}, nil
}

func ValidateInput(raw []byte) (string, image.Config, error) {
	if len(raw) == 0 || len(raw) > MaxInputBytes {
		return "", image.Config{}, ErrInvalid
	}
	if bytes.HasPrefix(raw, []byte("RIFF")) && bytes.Contains(raw[:min(len(raw), 64)], []byte("WEBP")) {
		features, err := webp.GetFeatures(bytes.NewReader(raw))
		if err != nil || features.HasAnimation {
			return "", image.Config{}, ErrNotSupported
		}
	}
	if isAnimatedPNG(raw) {
		return "", image.Config{}, ErrNotSupported
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return "", image.Config{}, ErrInvalid
	}
	if format != "jpeg" && format != "png" && format != "webp" {
		return "", image.Config{}, ErrNotSupported
	}
	if !validDimensions(cfg.Width, cfg.Height) {
		return "", image.Config{}, ErrInvalid
	}
	return format, cfg, nil
}

func isAnimatedPNG(raw []byte) bool {
	if len(raw) < 8 || !bytes.Equal(raw[:8], []byte{137, 80, 78, 71, 13, 10, 26, 10}) {
		return false
	}
	for off := 8; off+12 <= len(raw); {
		n := uint64(binary.BigEndian.Uint32(raw[off : off+4]))
		chunkType := string(raw[off+4 : off+8])
		end := uint64(off) + 12 + n
		if end > uint64(len(raw)) {
			return false
		}
		if chunkType == "acTL" {
			return true
		}
		off = int(end)
		if chunkType == "IEND" {
			return false
		}
	}
	return false
}

func validDimensions(width, height int) bool {
	return width > 0 && height > 0 && width <= MaxSide && height <= MaxSide && int64(width)*int64(height) <= MaxPixels
}

func encodeVariant(name VariantName, src image.Image, longestSide int) (Variant, error) {
	img := resize(src, longestSide)
	var out bytes.Buffer
	if err := webp.Encode(&out, img, &webp.EncoderOptions{Quality: 80, Method: 4}); err != nil {
		return Variant{}, ErrInvalid
	}
	b := out.Bytes()
	if len(b) == 0 || len(b) > MaxInputBytes {
		return Variant{}, ErrInvalid
	}
	return Variant{Name: name, MIME: "image/webp", Size: int64(len(b)), SHA256: digestHex(b), Width: img.Bounds().Dx(), Height: img.Bounds().Dy(), Bytes: append([]byte(nil), b...)}, nil
}

func resize(src image.Image, maxSide int) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= maxSide && h <= maxSide {
		return src
	}
	scale := float64(maxSide) / float64(max(w, h))
	dw, dh := max(1, int(math.Round(float64(w)*scale))), max(1, int(math.Round(float64(h)*scale)))
	out := image.NewNRGBA(image.Rect(0, 0, dw, dh))
	for y := 0; y < dh; y++ {
		sy := (float64(y)+0.5)*float64(h)/float64(dh) - 0.5
		y0 := max(0, int(math.Floor(sy)))
		y1 := min(h-1, y0+1)
		fy := math.Max(0, sy-float64(y0))
		for x := 0; x < dw; x++ {
			sx := (float64(x)+0.5)*float64(w)/float64(dw) - 0.5
			x0 := max(0, int(math.Floor(sx)))
			x1 := min(w-1, x0+1)
			fx := math.Max(0, sx-float64(x0))
			c00 := colorNRGBA(src.At(b.Min.X+x0, b.Min.Y+y0))
			c10 := colorNRGBA(src.At(b.Min.X+x1, b.Min.Y+y0))
			c01 := colorNRGBA(src.At(b.Min.X+x0, b.Min.Y+y1))
			c11 := colorNRGBA(src.At(b.Min.X+x1, b.Min.Y+y1))
			out.SetNRGBA(x, y, interpolate(c00, c10, c01, c11, fx, fy))
		}
	}
	return out
}

func colorNRGBA(c color.Color) color.NRGBA {
	r, g, b, a := c.RGBA()
	if a == 0 {
		return color.NRGBA{}
	}
	return color.NRGBA{R: uint8((r * 0xffff / a) >> 8), G: uint8((g * 0xffff / a) >> 8), B: uint8((b * 0xffff / a) >> 8), A: uint8(a >> 8)}
}

func interpolate(a, b, c, d color.NRGBA, fx, fy float64) color.NRGBA {
	blend := func(a, b, c, d uint8) uint8 {
		v := (float64(a)*(1-fx)+float64(b)*fx)*(1-fy) + (float64(c)*(1-fx)+float64(d)*fx)*fy
		return uint8(math.Round(math.Max(0, math.Min(255, v))))
	}
	return color.NRGBA{R: blend(a.R, b.R, c.R, d.R), G: blend(a.G, b.G, c.G, d.G), B: blend(a.B, b.B, c.B, d.B), A: blend(a.A, b.A, c.A, d.A)}
}

func orient(src image.Image, orientation uint16) image.Image {
	if orientation < 2 || orientation > 8 {
		return src
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dw, dh := w, h
	if orientation >= 5 {
		dw, dh = h, w
	}
	out := image.NewNRGBA(image.Rect(0, 0, dw, dh))
	for y := 0; y < dh; y++ {
		for x := 0; x < dw; x++ {
			var sx, sy int
			switch orientation {
			case 2:
				sx, sy = w-1-x, y
			case 3:
				sx, sy = w-1-x, h-1-y
			case 4:
				sx, sy = x, h-1-y
			case 5:
				sx, sy = y, x
			case 6:
				sx, sy = y, h-1-x
			case 7:
				sx, sy = w-1-y, h-1-x
			case 8:
				sx, sy = w-1-y, x
			}
			out.Set(x, y, src.At(b.Min.X+sx, b.Min.Y+sy))
		}
	}
	return out
}

func jpegOrientation(raw []byte) uint16 {
	if len(raw) < 4 || raw[0] != 0xff || raw[1] != 0xd8 {
		return 1
	}
	for off := 2; off+4 <= len(raw); {
		if raw[off] != 0xff {
			return 1
		}
		marker := raw[off+1]
		if marker == 0xda || marker == 0xd9 {
			return 1
		}
		length := int(binary.BigEndian.Uint16(raw[off+2 : off+4]))
		if length < 2 || off+2+length > len(raw) {
			return 1
		}
		segment := raw[off+4 : off+2+length]
		if marker == 0xe1 && len(segment) >= 14 && bytes.Equal(segment[:6], []byte("Exif\x00\x00")) {
			if value := parseTIFFOrientation(segment[6:]); value >= 1 && value <= 8 {
				return value
			}
		}
		off += 2 + length
	}
	return 1
}

func parseTIFFOrientation(data []byte) uint16 {
	if len(data) < 8 {
		return 1
	}
	var order binary.ByteOrder
	switch string(data[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return 1
	}
	if order.Uint16(data[2:4]) != 42 {
		return 1
	}
	off := uint64(order.Uint32(data[4:8]))
	if off+2 > uint64(len(data)) {
		return 1
	}
	count := uint64(order.Uint16(data[off : off+2]))
	if count > (uint64(len(data))-off-2)/12 {
		return 1
	}
	for i := uint64(0); i < count; i++ {
		entry := data[off+2+i*12 : off+2+(i+1)*12]
		if order.Uint16(entry[:2]) == 0x0112 && order.Uint16(entry[2:4]) == 3 && order.Uint32(entry[4:8]) == 1 {
			return order.Uint16(entry[8:10])
		}
	}
	return 1
}
