package media

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

const (
	contentTestOwnerID = "00000000-0000-4000-8000-000000000011"
	contentTestSpaceID = "00000000-0000-4000-8000-000000000012"
	contentTestPhotoID = "00000000-0000-4000-8000-000000000013"
)

type contentSpaceIDs struct{}

func (contentSpaceIDs) ID() (string, error) { return contentTestPhotoID, nil }

type contentSpaceRepositoryStub struct {
	Repository
	photo        Photo
	photoErr     error
	variant      Variant
	variantErr   error
	spaceCalls   int
	spaceArgs    []string
	variantCalls int
	variantArgs  []string
}

func (r *contentSpaceRepositoryStub) GetSpace(_ context.Context, owner, spaceID, photoID string) (Photo, error) {
	r.spaceCalls++
	r.spaceArgs = []string{owner, spaceID, photoID}
	photo := r.photo
	photo.Variants = append([]Variant(nil), r.photo.Variants...)
	return photo, r.photoErr
}

func (r *contentSpaceRepositoryStub) GetVariant(_ context.Context, owner, photoID, name string) (Variant, error) {
	r.variantCalls++
	r.variantArgs = []string{owner, photoID, name}
	return r.variant, r.variantErr
}

type contentSpaceObjectStoreStub struct {
	ObjectStore
	body       []byte
	err        error
	getCalls   int
	getKey     string
	generation int64
}

func (s *contentSpaceObjectStoreStub) Get(_ context.Context, key string, generation int64) ([]byte, error) {
	s.getCalls++
	s.getKey, s.generation = key, generation
	return append([]byte(nil), s.body...), s.err
}

func newContentSpaceService(t *testing.T, repo *contentSpaceRepositoryStub, store *contentSpaceObjectStoreStub) *Service {
	t.Helper()
	service, err := NewService(repo, store, contentSpaceIDs{}, time.Now)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	return service
}

func contentSpaceFixture() (Photo, []byte, Variant) {
	body := []byte("synthetic private image variant")
	variant := Variant{
		Name: VariantDisplay, MIME: "image/webp", Size: int64(len(body)),
		SHA256: digestHex(body), ObjectKey: "private/synthetic/display.webp", Generation: 17,
	}
	photo := Photo{
		ID: contentTestPhotoID, SpaceID: contentTestSpaceID, OwnerID: contentTestOwnerID,
		State: "ready", Variants: []Variant{variant},
	}
	return photo, body, variant
}

func TestContentSpaceReturnsScopedMetadataAndVerifiedBytes(t *testing.T) {
	photo, body, variant := contentSpaceFixture()
	repo := &contentSpaceRepositoryStub{photo: photo, variant: variant}
	store := &contentSpaceObjectStoreStub{body: body}
	service := newContentSpaceService(t, repo, store)

	gotPhoto, gotVariant, gotBody, err := service.ContentSpace(context.Background(), contentTestOwnerID, contentTestSpaceID, contentTestPhotoID, VariantDisplay)
	if err != nil {
		t.Fatalf("ContentSpace() error = %v", err)
	}
	if !reflect.DeepEqual(repo.spaceArgs, []string{contentTestOwnerID, contentTestSpaceID, contentTestPhotoID}) || repo.spaceCalls != 1 {
		t.Fatalf("scoped photo lookup calls=%d args=%v", repo.spaceCalls, repo.spaceArgs)
	}
	if !reflect.DeepEqual(repo.variantArgs, []string{contentTestOwnerID, contentTestPhotoID, string(VariantDisplay)}) || repo.variantCalls != 1 {
		t.Fatalf("variant lookup calls=%d args=%v", repo.variantCalls, repo.variantArgs)
	}
	if store.getCalls != 1 || store.getKey != variant.ObjectKey || store.generation != variant.Generation {
		t.Fatalf("object read calls=%d key=%q generation=%d", store.getCalls, store.getKey, store.generation)
	}
	if !reflect.DeepEqual(gotVariant, variant) || !reflect.DeepEqual(gotBody, body) {
		t.Fatalf("returned variant=%+v body=%q, want variant=%+v body=%q", gotVariant, gotBody, variant, body)
	}
	if gotPhoto.ID != contentTestPhotoID || gotPhoto.SpaceID != contentTestSpaceID || len(gotPhoto.Variants) != 1 ||
		gotPhoto.Variants[0].URL != "/api/v1/spaces/"+contentTestSpaceID+"/gallery/"+contentTestPhotoID+"/variants/display" {
		t.Fatalf("returned photo metadata=%+v", gotPhoto)
	}
}

func TestContentSpaceStopsBeforeVariantOrObjectReadWhenScopedPhotoIsUnavailable(t *testing.T) {
	photo, _, variant := contentSpaceFixture()
	for _, tc := range []struct {
		name     string
		owner    string
		spaceID  string
		photoID  string
		photoErr error
	}{
		{name: "invalid owner", owner: "not-a-uuid", spaceID: contentTestSpaceID, photoID: contentTestPhotoID},
		{name: "missing scoped photo", owner: contentTestOwnerID, spaceID: contentTestSpaceID, photoID: contentTestPhotoID, photoErr: ErrNotFound},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			repo := &contentSpaceRepositoryStub{photo: photo, photoErr: tc.photoErr, variant: variant}
			store := &contentSpaceObjectStoreStub{}
			service := newContentSpaceService(t, repo, store)

			gotPhoto, gotVariant, gotBody, err := service.ContentSpace(context.Background(), tc.owner, tc.spaceID, tc.photoID, VariantDisplay)
			if !errors.Is(err, ErrNotFound) || !reflect.DeepEqual(gotPhoto, Photo{}) || !reflect.DeepEqual(gotVariant, Variant{}) || gotBody != nil {
				t.Fatalf("ContentSpace() = (%+v, %+v, %q, %v), want empty not-found", gotPhoto, gotVariant, gotBody, err)
			}
			wantSpaceCalls := 1
			if tc.owner == "not-a-uuid" {
				wantSpaceCalls = 0
			}
			if repo.spaceCalls != wantSpaceCalls || repo.variantCalls != 0 || store.getCalls != 0 {
				t.Fatalf("calls: scoped photo=%d, variant=%d, object=%d; want %d, 0, 0", repo.spaceCalls, repo.variantCalls, store.getCalls, wantSpaceCalls)
			}
		})
	}
}

func TestContentSpaceStopsBeforeObjectReadWhenVariantMetadataFails(t *testing.T) {
	photo, _, variant := contentSpaceFixture()
	repo := &contentSpaceRepositoryStub{photo: photo, variant: variant, variantErr: ErrNotFound}
	store := &contentSpaceObjectStoreStub{}
	service := newContentSpaceService(t, repo, store)

	gotPhoto, gotVariant, gotBody, err := service.ContentSpace(context.Background(), contentTestOwnerID, contentTestSpaceID, contentTestPhotoID, VariantDisplay)
	if !errors.Is(err, ErrNotFound) || !reflect.DeepEqual(gotPhoto, Photo{}) || !reflect.DeepEqual(gotVariant, Variant{}) || gotBody != nil {
		t.Fatalf("ContentSpace() = (%+v, %+v, %q, %v), want empty not-found", gotPhoto, gotVariant, gotBody, err)
	}
	if repo.spaceCalls != 1 || repo.variantCalls != 1 || store.getCalls != 0 {
		t.Fatalf("calls: scoped photo=%d, variant=%d, object=%d; want 1, 1, 0", repo.spaceCalls, repo.variantCalls, store.getCalls)
	}
}

func TestContentSpaceMapsObjectStoreFailureAndRejectsCorruptedBytes(t *testing.T) {
	_, body, validVariant := contentSpaceFixture()
	cases := []struct {
		name     string
		variant  Variant
		storeErr error
	}{
		{name: "object store unavailable", variant: validVariant, storeErr: errors.New("synthetic storage detail")},
		{name: "stored size mismatch", variant: func() Variant { v := validVariant; v.Size++; return v }()},
		{name: "stored digest mismatch", variant: func() Variant { v := validVariant; v.SHA256 = "incorrect-digest"; return v }()},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			photo, _, _ := contentSpaceFixture()
			repo := &contentSpaceRepositoryStub{photo: photo, variant: tc.variant}
			store := &contentSpaceObjectStoreStub{body: body, err: tc.storeErr}
			service := newContentSpaceService(t, repo, store)

			gotPhoto, gotVariant, gotBody, err := service.ContentSpace(context.Background(), contentTestOwnerID, contentTestSpaceID, contentTestPhotoID, VariantDisplay)
			if !errors.Is(err, ErrUnavailable) || !reflect.DeepEqual(gotPhoto, Photo{}) || !reflect.DeepEqual(gotVariant, Variant{}) || gotBody != nil {
				t.Fatalf("ContentSpace() = (%+v, %+v, %q, %v), want empty unavailable", gotPhoto, gotVariant, gotBody, err)
			}
			if repo.spaceCalls != 1 || repo.variantCalls != 1 || store.getCalls != 1 ||
				store.getKey != tc.variant.ObjectKey || store.generation != tc.variant.Generation {
				t.Fatalf("calls: scoped photo=%d, variant=%d, object=%d key=%q generation=%d", repo.spaceCalls, repo.variantCalls, store.getCalls, store.getKey, store.generation)
			}
		})
	}
}
