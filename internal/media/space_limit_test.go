package media

import "testing"

func TestMaxSpaceMedia(t *testing.T) {
	if MaxSpaceMedia != 30 {
		t.Fatalf("MaxSpaceMedia = %d, want 30", MaxSpaceMedia)
	}
}
