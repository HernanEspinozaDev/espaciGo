package mediapg

import (
	"errors"
	"testing"

	"github.com/HernanEspinozaDev/espaciGo/internal/media"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestMapErrorCheckViolation(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    error
	}{
		{
			name:    "space media limit of 10 reached",
			message: "space media limit of 10 reached",
			want:    media.ErrLimit,
		},
		{
			name:    "space media limit of 30 reached",
			message: "space media limit of 30 reached",
			want:    media.ErrLimit,
		},
		{
			name:    "unrelated check violation",
			message: "some other check constraint failed",
			want:    media.ErrInvalid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := mapError(&pgconn.PgError{Code: "23514", Message: tt.message})
			if !errors.Is(err, tt.want) {
				t.Fatalf("mapError() = %v, want %v", err, tt.want)
			}
		})
	}
}
