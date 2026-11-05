package spacespg

import (
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/spaces"
)

func TestRevisionMatches(t *testing.T) {
	revision := time.Date(2026, 10, 27, 12, 0, 0, 123, time.UTC)
	if err := checkRevision(revision, revision); err != nil {
		t.Fatalf("equal revision: got %v, want nil", err)
	}
}

func TestRevisionMismatchIsPreconditionFailed(t *testing.T) {
	current := time.Date(2026, 10, 27, 12, 0, 0, 123, time.UTC)
	expected := current.Add(-time.Microsecond)
	if err := checkRevision(expected, current); err != spaces.ErrPreconditionFailed {
		t.Fatalf("stale revision: got %v, want %v", err, spaces.ErrPreconditionFailed)
	}
}
