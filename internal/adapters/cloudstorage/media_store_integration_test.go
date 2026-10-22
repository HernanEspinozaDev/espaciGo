package cloudstorage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/credentials"
	"github.com/HernanEspinozaDev/espaciGo/internal/media"
)

// This opt-in integration test writes exactly one generated WebP object under
// media/v1 and removes only the generation it created. GCS soft delete may
// retain that deleted generation according to the bucket's existing policy.
func TestMediaStoreGCSCreateReadGenerationAndCleanup(t *testing.T) {
	bucket := os.Getenv("IMG01E_GCS_TEST_BUCKET")
	if bucket == "" {
		t.Skip("set IMG01E_GCS_TEST_BUCKET to run the isolated GCS contract test")
	}
	if bucket != "espacigo-test-files-447373903153-gcp02" {
		t.Fatalf("refusing GCS contract test against unreviewed bucket %q", bucket)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	store, err := NewMediaStore(ctx, bucket)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Errorf("close GCS media store: %v", err)
		}
	})

	spaceID, err := (credentials.Generator{}).ID()
	if err != nil {
		t.Fatal(err)
	}
	assetID, err := (credentials.Generator{}).ID()
	if err != nil {
		t.Fatal(err)
	}
	input := image.NewRGBA(image.Rect(0, 0, 32, 24))
	for y := 0; y < 24; y++ {
		for x := 0; x < 32; x++ {
			input.Set(x, y, color.RGBA{R: uint8(x * 7), G: uint8(y * 9), B: 91, A: 255})
		}
	}
	var source bytes.Buffer
	if err := jpeg.Encode(&source, input, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	variants, err := media.Convert(source.Bytes(), media.PurposeSpace)
	if err != nil {
		t.Fatal(err)
	}
	var display []byte
	for _, variant := range variants {
		if variant.Name == media.VariantDisplay {
			display = variant.Bytes
			break
		}
	}
	if len(display) == 0 {
		t.Fatal("WebP display fixture was not generated")
	}
	key := "media/v1/spaces/" + spaceID + "/" + assetID + "/1/display.webp"
	stored, err := store.Put(ctx, key, display, "image/webp")
	if err != nil {
		t.Fatalf("create-only GCS write: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		if err := store.Delete(cleanupCtx, key, stored.Generation); err != nil {
			t.Errorf("cleanup exact GCS media generation: %v", err)
		}
	})
	if stored.Provider != "gcs" || stored.Bucket != bucket || stored.ObjectKey != key || stored.Generation <= 0 || stored.Size != int64(len(display)) {
		t.Fatalf("unexpected GCS object result: %+v", stored)
	}
	digest := sha256.Sum256(display)
	if stored.SHA256 != hex.EncodeToString(digest[:]) {
		t.Fatalf("stored SHA-256=%q does not match fixture", stored.SHA256)
	}
	if _, err := store.Put(ctx, key, display, "image/webp"); err == nil {
		t.Fatal("create-only retry overwrote an existing media object")
	}
	got, err := store.Get(ctx, key, stored.Generation)
	if err != nil || !bytes.Equal(got, display) {
		t.Fatalf("generation-specific GCS read size=%d err=%v", len(got), err)
	}
	if _, err := store.Get(ctx, key, stored.Generation+1); err == nil {
		t.Fatal("GCS read accepted a non-existent generation")
	}
	if err := store.Delete(ctx, key, stored.Generation+1); err == nil {
		t.Fatal("GCS cleanup accepted a stale generation")
	}
	current, err := store.Generation(ctx, key)
	if err != nil || current != stored.Generation {
		t.Fatalf("stale delete changed live generation: generation=%d err=%v", current, err)
	}
	if err := store.Delete(ctx, key, stored.Generation); err != nil {
		t.Fatalf("exact-generation GCS cleanup: %v", err)
	}
	if _, err := store.Generation(ctx, key); !errors.Is(err, media.ErrNotFound) {
		t.Fatalf("GCS object remains live after cleanup: %v", err)
	}
}
