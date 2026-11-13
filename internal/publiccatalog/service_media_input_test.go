package publiccatalog

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"
)

type rejectedMediaVariantRepository struct {
	Repository
	calls int
}

func (r *rejectedMediaVariantRepository) PublicMediaVariant(context.Context, string, string, string) (PublicImage, error) {
	r.calls++
	return PublicImage{}, errors.New("unexpected public media lookup")
}

func (r *rejectedMediaVariantRepository) PublicImage(context.Context, string, string) (PublicImage, error) {
	r.calls++
	return PublicImage{}, errors.New("unexpected public image lookup")
}

type catalogRepositoryWithoutPublicMedia struct {
	Repository
}

type partialErrorPublicMediaRepository struct {
	Repository
	item  PublicImage
	err   error
	calls int
}

func (r *partialErrorPublicMediaRepository) PublicMediaVariant(context.Context, string, string, string) (PublicImage, error) {
	r.calls++
	return r.item, r.err
}

func (*partialErrorPublicMediaRepository) PublicImage(context.Context, string, string) (PublicImage, error) {
	return PublicImage{}, errors.New("unexpected legacy public image lookup")
}

func TestMediaImageContentRejectsInvalidInputBeforeRepositoryOrObjectStore(t *testing.T) {
	validSpaceID := "00000000-0000-4000-8000-000000000001"
	validImageID := "00000000-0000-4000-8000-000000000002"
	cases := []struct {
		name    string
		spaceID string
		imageID string
		variant string
	}{
		{name: "invalid space UUID", spaceID: "not-a-uuid", imageID: validImageID, variant: "display"},
		{name: "invalid image UUID", spaceID: validSpaceID, imageID: "not-a-uuid", variant: "display"},
		{name: "unsupported variant", spaceID: validSpaceID, imageID: validImageID, variant: "avatar"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &rejectedMediaVariantRepository{}
			store := &publicMediaStore{}
			legacy := PublicImageStoreFunc(func(context.Context, string) ([]byte, error) {
				return nil, ErrNotFound
			})
			service, err := NewWithImageAndMediaStore(
				repo,
				[]byte("01234567890123456789012345678901"),
				legacy,
				store,
			)
			if err != nil {
				t.Fatal(err)
			}

			item, content, err := service.MediaImageContent(context.Background(), tc.spaceID, tc.imageID, tc.variant)
			if !errors.Is(err, ErrNotFound) || item != (PublicImage{}) || len(content) != 0 {
				t.Fatalf("item=%+v content length=%d err=%v; want empty not-found", item, len(content), err)
			}
			if repo.calls != 0 || store.getCalls != 0 {
				t.Fatalf("repository lookups=%d object-store reads=%d; want 0 and 0", repo.calls, store.getCalls)
			}
		})
	}
}

func TestMediaImageContentRequiresPublicMediaRepositoryBeforeObjectRead(t *testing.T) {
	store := &publicMediaStore{}
	legacy := PublicImageStoreFunc(func(context.Context, string) ([]byte, error) {
		return nil, ErrNotFound
	})
	service, err := NewWithImageAndMediaStore(
		&catalogRepositoryWithoutPublicMedia{},
		[]byte("01234567890123456789012345678901"),
		legacy,
		store,
	)
	if err != nil {
		t.Fatal(err)
	}

	item, content, err := service.MediaImageContent(
		context.Background(),
		"00000000-0000-4000-8000-000000000001",
		"00000000-0000-4000-8000-000000000002",
		"display",
	)
	if !errors.Is(err, ErrNotFound) || item != (PublicImage{}) || len(content) != 0 {
		t.Fatalf("item=%+v content length=%d err=%v; want empty not-found", item, len(content), err)
	}
	if store.getCalls != 0 {
		t.Fatalf("object-store reads=%d; want 0", store.getCalls)
	}
}

func TestMediaImageContentDiscardsMetadataReturnedWithRepositoryError(t *testing.T) {
	body := []byte("private synthetic webp payload")
	digest := sha256.Sum256(body)
	lookupErr := errors.New("synthetic repository failure")
	const objectKey = "media/v1/spaces/synthetic/photo/display.webp"
	repo := &partialErrorPublicMediaRepository{
		item: PublicImage{
			MIME: "image/webp", SHA256: hex.EncodeToString(digest[:]), Size: int64(len(body)),
			ObjectKey: objectKey, Generation: 7,
		},
		err: lookupErr,
	}
	store := &publicMediaStore{body: body, key: objectKey, generation: 7}
	legacy := PublicImageStoreFunc(func(context.Context, string) ([]byte, error) { return nil, ErrNotFound })
	service, err := NewWithImageAndMediaStore(
		repo,
		[]byte("01234567890123456789012345678901"),
		legacy,
		store,
	)
	if err != nil {
		t.Fatal(err)
	}

	item, content, gotErr := service.MediaImageContent(
		context.Background(),
		"00000000-0000-4000-8000-000000000001",
		"00000000-0000-4000-8000-000000000002",
		"display",
	)
	if !errors.Is(gotErr, lookupErr) || item != (PublicImage{}) || len(content) != 0 {
		t.Fatalf("item=%+v content length=%d err=%v; want empty result and original repository error", item, len(content), gotErr)
	}
	if repo.calls != 1 || store.getCalls != 0 {
		t.Fatalf("repository calls=%d object-store reads=%d; want 1 and 0", repo.calls, store.getCalls)
	}
}
