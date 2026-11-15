package media

import (
	"image"
	"image/color"
	"testing"
)

func TestOrientMapsAllEightEXIFTransforms(t *testing.T) {
	source := image.NewNRGBA(image.Rect(0, 0, 2, 3))
	for y := 0; y < 3; y++ {
		for x := 0; x < 2; x++ {
			source.SetNRGBA(x, y, color.NRGBA{R: uint8(y*2 + x + 1), A: 0xff})
		}
	}
	cases := []struct {
		orientation uint16
		rows        [][]uint8
	}{
		{1, [][]uint8{{1, 2}, {3, 4}, {5, 6}}},
		{2, [][]uint8{{2, 1}, {4, 3}, {6, 5}}},
		{3, [][]uint8{{6, 5}, {4, 3}, {2, 1}}},
		{4, [][]uint8{{5, 6}, {3, 4}, {1, 2}}},
		{5, [][]uint8{{1, 3, 5}, {2, 4, 6}}},
		{6, [][]uint8{{5, 3, 1}, {6, 4, 2}}},
		{7, [][]uint8{{6, 4, 2}, {5, 3, 1}}},
		{8, [][]uint8{{2, 4, 6}, {1, 3, 5}}},
	}
	for _, tc := range cases {
		t.Run(string(rune('0'+tc.orientation)), func(t *testing.T) {
			got, ok := orient(source, tc.orientation).(*image.NRGBA)
			if !ok {
				t.Fatalf("orientation %d returned %T, want *image.NRGBA", tc.orientation, orient(source, tc.orientation))
			}
			if got.Bounds().Dx() != len(tc.rows[0]) || got.Bounds().Dy() != len(tc.rows) {
				t.Fatalf("orientation %d dimensions=%dx%d, want %dx%d", tc.orientation, got.Bounds().Dx(), got.Bounds().Dy(), len(tc.rows[0]), len(tc.rows))
			}
			for y, row := range tc.rows {
				for x, want := range row {
					if pixel := got.NRGBAAt(x, y).R; pixel != want {
						t.Errorf("orientation %d pixel (%d,%d)=%d, want %d", tc.orientation, x, y, pixel, want)
					}
				}
			}
		})
	}
}
