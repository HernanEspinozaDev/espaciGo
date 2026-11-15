package publiccatalog

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"
)

type recordingPublicMediaRepository struct {
	publicMediaRepository
	reads      int
	variantErr error
}

func (r *recordingPublicMediaRepository) PublicMediaVariant(ctx context.Context, spaceID, imageID, variant string) (PublicImage, error) {
	r.reads++
	if r.variantErr != nil {
		return PublicImage{}, r.variantErr
	}
	return r.publicMediaRepository.PublicMediaVariant(ctx, spaceID, imageID, variant)
}

type recordingPublicMediaStore struct {
	*publicMediaStore
	reads int
}

func (s *recordingPublicMediaStore) Get(ctx context.Context, key string, generation int64) ([]byte, error) {
	s.reads++
	return s.publicMediaStore.Get(ctx, key, generation)
}

func TestMediaImageContentRejectsInvalidRequestBeforeRepositoryOrStorageRead(t *testing.T) {
	body := []byte("normalized-webp-fixture")
	digest := sha256.Sum256(body)
	baseStore := &publicMediaStore{body: body, key: "media/v1/spaces/a/photo/display.webp", generation: 7}
	repo := &recordingPublicMediaRepository{publicMediaRepository: publicMediaRepository{item: PublicImage{
		MIME: "image/webp", SHA256: hex.EncodeToString(digest[:]), Size: int64(len(body)), ObjectKey: baseStore.key, Generation: baseStore.generation,
	}}}
	store := &recordingPublicMediaStore{publicMediaStore: baseStore}
	legacy := PublicImageStoreFunc(func(context.Context, string) ([]byte, error) { return nil, ErrNotFound })
	service, err := NewWithImageAndMediaStore(repo, []byte("01234567890123456789012345678901"), legacy, store)
	if err != nil {
		t.Fatal(err)
	}

	validSpaceID := "00000000-0000-4000-8000-000000000001"
	validImageID := "00000000-0000-4000-8000-000000000002"
	cases := []struct {
		name, spaceID, imageID, variant string
	}{
		{name: "invalid space ID", spaceID: "not-a-uuid", imageID: validImageID, variant: "display"},
		{name: "invalid image ID", spaceID: validSpaceID, imageID: "not-a-uuid", variant: "display"},
		{name: "unsupported variant", spaceID: validSpaceID, imageID: validImageID, variant: "avatar"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := service.MediaImageContent(context.Background(), tc.spaceID, tc.imageID, tc.variant)
			if !errors.Is(err, ErrNotFound) {
				t.Fatalf("MediaImageContent error=%v, want ErrNotFound", err)
			}
			if repo.reads != 0 || store.reads != 0 {
				t.Fatalf("invalid request reached dependencies: repository=%d storage=%d", repo.reads, store.reads)
			}
		})
	}
}

func TestMediaImageContentRejectsInvalidMetadataBeforeStorageRead(t *testing.T) {
	body := []byte("normalized-webp-fixture")
	digest := sha256.Sum256(body)
	baseItem := PublicImage{
		MIME: "image/webp", SHA256: hex.EncodeToString(digest[:]), Size: int64(len(body)),
		ObjectKey: "media/v1/spaces/a/photo/display.webp", Generation: 7,
	}
	cases := []struct {
		name   string
		mutate func(*PublicImage)
	}{
		{name: "wrong MIME", mutate: func(item *PublicImage) { item.MIME = "image/png" }},
		{name: "zero size", mutate: func(item *PublicImage) { item.Size = 0 }},
		{name: "oversized object", mutate: func(item *PublicImage) { item.Size = 10<<20 + 1 }},
		{name: "missing object key", mutate: func(item *PublicImage) { item.ObjectKey = "" }},
		{name: "nonpositive generation", mutate: func(item *PublicImage) { item.Generation = 0 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			item := baseItem
			tc.mutate(&item)
			baseStore := &publicMediaStore{body: body, key: baseItem.ObjectKey, generation: baseItem.Generation}
			repo := &recordingPublicMediaRepository{publicMediaRepository: publicMediaRepository{item: item}}
			store := &recordingPublicMediaStore{publicMediaStore: baseStore}
			legacy := PublicImageStoreFunc(func(context.Context, string) ([]byte, error) { return nil, ErrNotFound })
			service, err := NewWithImageAndMediaStore(repo, []byte("01234567890123456789012345678901"), legacy, store)
			if err != nil {
				t.Fatal(err)
			}

			_, _, err = service.MediaImageContent(context.Background(), "00000000-0000-4000-8000-000000000001", "00000000-0000-4000-8000-000000000002", "display")
			if !errors.Is(err, ErrNotFound) {
				t.Fatalf("invalid metadata error=%v, want ErrNotFound", err)
			}
			if repo.reads != 1 || store.reads != 0 {
				t.Fatalf("metadata checks/read counts: repository=%d storage=%d, want 1/0", repo.reads, store.reads)
			}
		})
	}
}

func TestMediaImageContentRejectsStorageBytesWithWrongSizeOrDigest(t *testing.T) {
	body := []byte("normalized-webp-fixture")
	digest := sha256.Sum256(body)
	baseItem := PublicImage{
		MIME: "image/webp", SHA256: hex.EncodeToString(digest[:]), Size: int64(len(body)),
		ObjectKey: "media/v1/spaces/a/photo/display.webp", Generation: 7,
	}
	cases := []struct {
		name   string
		mutate func(*PublicImage)
	}{
		{name: "stored size mismatch", mutate: func(item *PublicImage) { item.Size++ }},
		{name: "stored digest mismatch", mutate: func(item *PublicImage) {
			digest := []byte(item.SHA256)
			if digest[0] == '0' {
				digest[0] = '1'
			} else {
				digest[0] = '0'
			}
			item.SHA256 = string(digest)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			item := baseItem
			tc.mutate(&item)
			baseStore := &publicMediaStore{body: body, key: baseItem.ObjectKey, generation: baseItem.Generation}
			repo := &recordingPublicMediaRepository{publicMediaRepository: publicMediaRepository{item: item}}
			store := &recordingPublicMediaStore{publicMediaStore: baseStore}
			legacy := PublicImageStoreFunc(func(context.Context, string) ([]byte, error) { return nil, ErrNotFound })
			service, err := NewWithImageAndMediaStore(repo, []byte("01234567890123456789012345678901"), legacy, store)
			if err != nil {
				t.Fatal(err)
			}

			_, got, err := service.MediaImageContent(context.Background(), "00000000-0000-4000-8000-000000000001", "00000000-0000-4000-8000-000000000002", "display")
			if !errors.Is(err, ErrNotFound) || len(got) != 0 {
				t.Fatalf("invalid stored bytes=%q error=%v, want no bytes and ErrNotFound", got, err)
			}
			if repo.reads != 1 || store.reads != 1 {
				t.Fatalf("integrity check/read counts: repository=%d storage=%d, want 1/1", repo.reads, store.reads)
			}
		})
	}
}

func TestMediaImageContentHandlesMissingRepositoryCapabilityAndReadErrors(t *testing.T) {
	const spaceID = "00000000-0000-4000-8000-000000000001"
	const imageID = "00000000-0000-4000-8000-000000000002"
	body := []byte("normalized-webp-fixture")
	baseStore := &publicMediaStore{body: body, key: "media/v1/spaces/a/photo/display.webp", generation: 7}
	store := &recordingPublicMediaStore{publicMediaStore: baseStore}
	legacy := PublicImageStoreFunc(func(context.Context, string) ([]byte, error) { return nil, ErrNotFound })

	t.Run("repository without media metadata capability", func(t *testing.T) {
		store.reads = 0
		service, err := NewWithImageAndMediaStore(memoryRepository{}, []byte("01234567890123456789012345678901"), legacy, store)
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := service.MediaImageContent(context.Background(), spaceID, imageID, "display"); !errors.Is(err, ErrNotFound) || store.reads != 0 {
			t.Fatalf("error=%v storage_reads=%d", err, store.reads)
		}
	})

	t.Run("repository read error propagates without storage access", func(t *testing.T) {
		failure := errors.New("synthetic media metadata failure")
		repo := &recordingPublicMediaRepository{
			publicMediaRepository: publicMediaRepository{},
			variantErr:            failure,
		}
		store.reads = 0
		service, err := NewWithImageAndMediaStore(repo, []byte("01234567890123456789012345678901"), legacy, store)
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := service.MediaImageContent(context.Background(), spaceID, imageID, "display"); !errors.Is(err, failure) || repo.reads != 1 || store.reads != 0 {
			t.Fatalf("error=%v repository_reads=%d storage_reads=%d", err, repo.reads, store.reads)
		}
	})
}
