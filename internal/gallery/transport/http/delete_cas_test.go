package galleryhttp

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/gallery"
	"github.com/HernanEspinozaDev/espaciGo/internal/media"
)

type deleteCASGalleryRepoStub struct {
	*repoStub
	current time.Time
	next    time.Time
	calls   int
	removed bool
}

func (r *deleteCASGalleryRepoStub) RemoveCAS(_ context.Context, _, _, photoID string, expected time.Time) (gallery.Removal, time.Time, error) {
	r.calls++
	if !expected.Equal(r.current) {
		return gallery.Removal{}, time.Time{}, gallery.ErrConflict
	}
	r.removed = true
	return gallery.Removal{PhotoID: photoID, Removed: true}, r.next, nil
}

func deleteCASIfMatch(at time.Time) string {
	payload := testSpace + "\x00" + at.UTC().Format(time.RFC3339Nano)
	return `"` + base64.RawURLEncoding.EncodeToString([]byte(payload)) + `"`
}

func newDeleteCASTestHandler(t *testing.T, repo *deleteCASGalleryRepoStub) http.Handler {
	t.Helper()
	svc, err := gallery.NewService(repo, &filesStub{items: map[string][]byte{}}, idsStub{}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	return NewHandler(authStub{}, svc, nil)
}

func TestDeleteIfMatchReturnsRenewedETag(t *testing.T) {
	current := time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC)
	next := current.Add(time.Second)
	repo := &deleteCASGalleryRepoStub{repoStub: &repoStub{idempotency: map[string]gallery.Photo{}}, current: current, next: next}
	handler := newDeleteCASTestHandler(t, repo)
	request := httptest.NewRequest(http.MethodDelete, "/api/v1/spaces/"+testSpace+"/gallery/"+testPhoto, nil)
	request.Header.Set("Authorization", "Bearer synthetic-test-token")
	request.Header.Set("If-Match", deleteCASIfMatch(current))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s; want 200", response.Code, response.Body.String())
	}
	if got, want := response.Header().Get("ETag"), deleteCASIfMatch(next); got != want {
		t.Fatalf("ETag=%q; want %q", got, want)
	}
	if !repo.removed || repo.calls != 1 {
		t.Fatalf("removed=%v calls=%d; want one CAS mutation", repo.removed, repo.calls)
	}
}

func TestDeleteStaleIfMatchPreservesGallery(t *testing.T) {
	current := time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC)
	repo := &deleteCASGalleryRepoStub{repoStub: &repoStub{idempotency: map[string]gallery.Photo{}}, current: current, next: current.Add(time.Second)}
	handler := newDeleteCASTestHandler(t, repo)
	request := httptest.NewRequest(http.MethodDelete, "/api/v1/spaces/"+testSpace+"/gallery/"+testPhoto, nil)
	request.Header.Set("Authorization", "Bearer synthetic-test-token")
	request.Header.Set("If-Match", deleteCASIfMatch(current.Add(-time.Second)))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusPreconditionFailed {
		t.Fatalf("status=%d body=%s; want 412", response.Code, response.Body.String())
	}
	if repo.removed || repo.calls != 1 {
		t.Fatalf("removed=%v calls=%d; want one rejected CAS without mutation", repo.removed, repo.calls)
	}
}

type deleteCASRealMediaRepoStub struct {
	media.Repository
	current time.Time
	next    time.Time
	calls   int
	removed bool
}

func (r *deleteCASRealMediaRepoStub) RemoveSpacePhotoCAS(_ context.Context, _, _, _ string, expected, _ time.Time) (time.Time, error) {
	r.calls++
	if !expected.Equal(r.current) {
		return time.Time{}, media.ErrPrecondition
	}
	r.removed = true
	return r.next, nil
}

func TestRealMediaDeleteUsesCASAndReturnsRenewedETag(t *testing.T) {
	current := time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC)
	next := current.Add(time.Second)
	spaceRepo := &repoStub{idempotency: map[string]gallery.Photo{}}
	spaceService, err := gallery.NewService(spaceRepo, &filesStub{items: map[string][]byte{}}, idsStub{}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	mediaRepo := &deleteCASRealMediaRepoStub{current: current, next: next}
	mediaService, err := media.NewService(mediaRepo, &replacementObjectStoreStub{}, replacementIDsStub{}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandlerWithMedia(authStub{}, spaceService, mediaService, nil)
	request := httptest.NewRequest(http.MethodDelete, "/api/v1/spaces/"+testSpace+"/gallery/"+testPhoto, nil)
	request.Header.Set("Authorization", "Bearer synthetic-test-token")
	request.Header.Set("If-Match", deleteCASIfMatch(current))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s; want 200", response.Code, response.Body.String())
	}
	if got, want := response.Header().Get("ETag"), deleteCASIfMatch(next); got != want {
		t.Fatalf("ETag=%q; want %q", got, want)
	}
	if !mediaRepo.removed || mediaRepo.calls != 1 {
		t.Fatalf("removed=%v calls=%d; want one CAS mutation", mediaRepo.removed, mediaRepo.calls)
	}
}

func TestRealMediaDeleteStaleIfMatchPreservesPhoto(t *testing.T) {
	current := time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC)
	spaceRepo := &repoStub{idempotency: map[string]gallery.Photo{}}
	spaceService, err := gallery.NewService(spaceRepo, &filesStub{items: map[string][]byte{}}, idsStub{}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	mediaRepo := &deleteCASRealMediaRepoStub{current: current, next: current.Add(time.Second)}
	mediaService, err := media.NewService(mediaRepo, &replacementObjectStoreStub{}, replacementIDsStub{}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandlerWithMedia(authStub{}, spaceService, mediaService, nil)
	request := httptest.NewRequest(http.MethodDelete, "/api/v1/spaces/"+testSpace+"/gallery/"+testPhoto, nil)
	request.Header.Set("Authorization", "Bearer synthetic-test-token")
	request.Header.Set("If-Match", deleteCASIfMatch(current.Add(-time.Second)))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusPreconditionFailed {
		t.Fatalf("status=%d body=%s; want 412", response.Code, response.Body.String())
	}
	if mediaRepo.removed || mediaRepo.calls != 1 {
		t.Fatalf("removed=%v calls=%d; want one rejected CAS without mutation", mediaRepo.removed, mediaRepo.calls)
	}
}
