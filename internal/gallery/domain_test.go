package gallery

import "testing"

func TestMaxPhotosPerSpaceIsThirty(t *testing.T) {
	if MaxPhotosPerSpace != 30 {
		t.Fatalf("MaxPhotosPerSpace = %d, want 30", MaxPhotosPerSpace)
	}
}
