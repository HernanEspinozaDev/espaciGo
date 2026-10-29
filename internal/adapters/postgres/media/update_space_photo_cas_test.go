package mediapg

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/media"
)

func TestUpdateSpacePhotoCASRejectsNonPositivePositionBeforeDatabase(t *testing.T) {
	cases := []struct {
		name     string
		position int
	}{
		{name: "zero", position: 0},
		{name: "negative", position: -1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repository := &Repository{}
			_, _, err := repository.UpdateSpacePhotoCAS(
				context.Background(),
				"owner",
				"space",
				"photo",
				nil,
				nil,
				&tc.position,
				time.Time{},
				time.Time{},
			)
			if !errors.Is(err, media.ErrInvalid) {
				t.Fatalf("UpdateSpacePhotoCAS() error = %v, want %v", err, media.ErrInvalid)
			}
		})
	}
}
