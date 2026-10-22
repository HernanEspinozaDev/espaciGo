package cloudstorage

import "testing"

func TestValidateMediaKeyRestrictsGeneratedVersionedNamespaces(t *testing.T) {
	valid := []string{
		"media/v1/spaces/00000000-0000-4000-8000-000000000001/00000000-0000-4000-8000-000000000002/1/display.webp",
		"media/v1/spaces/00000000-0000-4000-8000-000000000001/00000000-0000-4000-8000-000000000002/12/thumbnail.webp",
		"media/v1/profiles/00000000-0000-4000-8000-000000000001/00000000-0000-4000-8000-000000000002/1/avatar.webp",
	}
	for _, key := range valid {
		if err := validateMediaKey(key); err != nil {
			t.Errorf("rejected %q: %v", key, err)
		}
	}
	invalid := []string{"g03o-1017/media/v1/a.webp", "media/v1/spaces/owner/photo/1/original.jpg", "media/v1/spaces/00000000-0000-4000-8000-000000000001/00000000-0000-4000-8000-000000000002/0/display.webp", "media/v1/profiles/00000000-0000-4000-8000-000000000001/00000000-0000-4000-8000-000000000002/1/display.webp"}
	for _, key := range invalid {
		if err := validateMediaKey(key); err == nil {
			t.Errorf("accepted %q", key)
		}
	}
}
