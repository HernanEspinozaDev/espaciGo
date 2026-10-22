package mediafs

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestStoreCreateOnlyAndExactGeneration(t *testing.T) {
	s, err := New(filepath.Join(t.TempDir(), "private"))
	if err != nil {
		t.Fatal(err)
	}
	key := "media/v1/spaces/00000000-0000-4000-8000-000000000001/00000000-0000-4000-8000-000000000002/1/display.webp"
	first, err := s.Put(context.Background(), key, []byte("webp"), "image/webp")
	if err != nil {
		t.Fatal(err)
	}
	if generation, err := s.Generation(context.Background(), key); err != nil || generation != first.Generation {
		t.Fatalf("generation=%d err=%v", generation, err)
	}
	if _, err = s.Put(context.Background(), key, []byte("replace"), "image/webp"); err == nil {
		t.Fatal("second create unexpectedly replaced object")
	}
	if err = s.Delete(context.Background(), key, 2); err == nil {
		t.Fatal("wrong generation unexpectedly deleted object")
	}
	body, err := s.Get(context.Background(), key, first.Generation)
	if err != nil || string(body) != "webp" {
		t.Fatalf("get body=%q err=%v", body, err)
	}
	if err = s.Delete(context.Background(), key, first.Generation); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(s.root, filepath.FromSlash(key))); !os.IsNotExist(err) {
		t.Fatalf("object remains: %v", err)
	}
}

func TestStoreRejectsPathTraversalAndNonPrivateMime(t *testing.T) {
	s, err := New(filepath.Join(t.TempDir(), "private"))
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"media/v1/../secret", "media/v1\\secret", "/media/v1/secret", "other/key"} {
		if _, err := s.Put(context.Background(), key, []byte("x"), "image/webp"); err == nil {
			t.Fatalf("accepted unsafe key %q", key)
		}
	}
	if _, err := s.Put(context.Background(), "media/v1/a.webp", []byte("x"), "image/png"); err == nil {
		t.Fatal("accepted non-WebP content")
	}
}
