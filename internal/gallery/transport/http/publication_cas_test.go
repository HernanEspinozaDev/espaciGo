package galleryhttp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/gallery"
	"github.com/HernanEspinozaDev/espaciGo/internal/media"
	"github.com/HernanEspinozaDev/espaciGo/internal/spaces"
	spaceshttp "github.com/HernanEspinozaDev/espaciGo/internal/spaces/transport/http"
)

type publicationCASRepo struct {
	*repoStub
	legacyCalls  int
	casCalls     int
	mutations    int
	ownerID      string
	spaceID      string
	photoID      string
	expected     time.Time
	enabled      bool
	stale        bool
	notSynthetic bool
	renewed      time.Time
}

func (r *publicationCASRepo) SetPublicInCatalog(context.Context, string, string, string, bool) (gallery.Photo, error) {
	r.legacyCalls++
	return gallery.Photo{}, nil
}

func (r *publicationCASRepo) SetPublicInCatalogCAS(_ context.Context, ownerID, spaceID, photoID string, enabled bool, expected time.Time) (gallery.Photo, time.Time, error) {
	r.casCalls++
	r.ownerID, r.spaceID, r.photoID = ownerID, spaceID, photoID
	r.expected, r.enabled = expected, enabled
	if r.notSynthetic {
		return gallery.Photo{}, time.Time{}, gallery.ErrNotFound
	}
	if r.stale {
		return gallery.Photo{}, time.Time{}, gallery.ErrConflict
	}
	r.mutations++
	return gallery.Photo{ID: photoID, PublicInCatalog: enabled}, r.renewed, nil
}

type publicationMediaCASRepo struct {
	media.Repository
	casCalls    int
	legacyCalls int
	ownerID     string
	spaceID     string
	photoID     string
	expected    time.Time
	visible     bool
	visibleSet  bool
	renewed     time.Time
	err         error
}

func (r *publicationMediaCASRepo) UpdateSpacePhoto(_ context.Context, ownerID, spaceID, photoID string, _ *string, visible *bool, _ *int, _ time.Time) (media.Photo, error) {
	r.legacyCalls++
	return publicationMediaPhoto(ownerID, spaceID, photoID, visible), nil
}

func (r *publicationMediaCASRepo) UpdateSpacePhotoCAS(_ context.Context, ownerID, spaceID, photoID string, _ *string, visible *bool, _ *int, expected, _ time.Time) (media.Photo, time.Time, error) {
	r.casCalls++
	r.ownerID, r.spaceID, r.photoID = ownerID, spaceID, photoID
	r.expected = expected
	if visible != nil {
		r.visible, r.visibleSet = *visible, true
	}
	if r.err != nil {
		return media.Photo{}, time.Time{}, r.err
	}
	return publicationMediaPhoto(ownerID, spaceID, photoID, visible), r.renewed, nil
}

func publicationMediaPhoto(ownerID, spaceID, photoID string, visible *bool) media.Photo {
	photo := media.Photo{ID: photoID, OwnerID: ownerID, SpaceID: spaceID, State: "ready"}
	if visible != nil {
		photo.Visible = *visible
	}
	return photo
}

func TestPublicationUsesIfMatchCASBeforeJSONAndRenewsETag(t *testing.T) {
	current := time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC)
	renewed := current.Add(time.Second)
	valid := spaceshttp.DraftETag(spaces.Draft{ID: testSpace, UpdatedAt: current})
	cases := []struct {
		name, header string
		want         int
		body         string
		stale        bool
	}{
		{"missing", "", http.StatusPreconditionRequired, "not json", false},
		{"malformed", "not-an-etag", http.StatusBadRequest, "not json", false},
		{"invalid JSON", valid, http.StatusUnprocessableEntity, "not json", false},
		{"stale", spaceshttp.DraftETag(spaces.Draft{ID: testSpace, UpdatedAt: current.Add(-time.Second)}), http.StatusPreconditionFailed, `{"public_in_catalog":true}`, true},
		{"success", valid, http.StatusOK, `{"public_in_catalog":true}`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &publicationCASRepo{repoStub: &repoStub{idempotency: map[string]gallery.Photo{}}, stale: tc.stale, renewed: renewed}
			svc, err := gallery.NewService(repo, &filesStub{items: map[string][]byte{}}, idsStub{}, time.Now)
			if err != nil {
				t.Fatal(err)
			}
			h := NewHandler(authStub{}, svc, nil)
			r := httptest.NewRequest(http.MethodPut, "/api/v1/spaces/"+testSpace+"/gallery/"+testPhoto+"/publication", strings.NewReader(tc.body))
			r.Header.Set("Authorization", "Bearer synthetic-test-token")
			if tc.header != "" {
				r.Header.Set("If-Match", tc.header)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.want {
				t.Fatalf("status=%d body=%s want %d", w.Code, w.Body.String(), tc.want)
			}
			wantCAS := 0
			if tc.name == "stale" || tc.name == "success" {
				wantCAS = 1
			}
			if repo.casCalls != wantCAS {
				t.Fatalf("CAS calls=%d want %d", repo.casCalls, wantCAS)
			}
			if repo.legacyCalls != 0 {
				t.Fatalf("legacy writer calls=%d", repo.legacyCalls)
			}
			wantMutations := 0
			if tc.name == "success" {
				wantMutations = 1
			}
			if repo.mutations != wantMutations {
				t.Fatalf("mutations=%d want %d", repo.mutations, wantMutations)
			}
			if wantCAS == 1 && (repo.ownerID != testOwner || repo.spaceID != testSpace || repo.photoID != testPhoto) {
				t.Fatalf("CAS scope owner=%q space=%q photo=%q", repo.ownerID, repo.spaceID, repo.photoID)
			}
			if tc.name == "success" {
				if !repo.expected.Equal(current) || !repo.enabled {
					t.Fatalf("CAS expected=%v enabled=%v", repo.expected, repo.enabled)
				}
				want := spaceshttp.DraftETag(spaces.Draft{ID: testSpace, UpdatedAt: renewed})
				if got := w.Header().Get("ETag"); got != want {
					t.Fatalf("ETag=%q want %q", got, want)
				}
			}
		})
	}
}

func TestPublicationRejectsWeakWildcardAndMultipleIfMatch(t *testing.T) {
	current := time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC)
	valid := spaceshttp.DraftETag(spaces.Draft{ID: testSpace, UpdatedAt: current})
	for _, headers := range [][]string{{"W/" + valid}, {"*"}, {valid, valid}} {
		repo := &publicationCASRepo{repoStub: &repoStub{idempotency: map[string]gallery.Photo{}}}
		svc, _ := gallery.NewService(repo, &filesStub{items: map[string][]byte{}}, idsStub{}, time.Now)
		h := NewHandler(authStub{}, svc, nil)
		r := httptest.NewRequest(http.MethodPut, "/api/v1/spaces/"+testSpace+"/gallery/"+testPhoto+"/publication", strings.NewReader(`{"public_in_catalog":false}`))
		r.Header.Set("Authorization", "Bearer synthetic-test-token")
		for _, value := range headers {
			r.Header.Add("If-Match", value)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != http.StatusBadRequest || repo.casCalls != 0 || repo.legacyCalls != 0 {
			t.Fatalf("headers=%v status=%d CAS=%d legacy=%d", headers, w.Code, repo.casCalls, repo.legacyCalls)
		}
	}
}

func TestPublicationInvalidJSONDoesNotCallCASWriters(t *testing.T) {
	current := time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC)
	galleryRepo := &publicationCASRepo{repoStub: &repoStub{idempotency: map[string]gallery.Photo{}}, notSynthetic: true}
	galleryService, err := gallery.NewService(galleryRepo, &filesStub{items: map[string][]byte{}}, idsStub{}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	mediaRepo := &publicationMediaCASRepo{renewed: current.Add(time.Second)}
	mediaService, err := media.NewService(mediaRepo, &replacementObjectStoreStub{}, replacementIDsStub{}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandlerWithMedia(authStub{}, galleryService, mediaService, nil)
	request := httptest.NewRequest(http.MethodPut, "/api/v1/spaces/"+testSpace+"/gallery/"+testPhoto+"/publication", strings.NewReader("not json"))
	request.Header.Set("Authorization", "Bearer synthetic-test-token")
	request.Header.Set("If-Match", spaceshttp.DraftETag(spaces.Draft{ID: testSpace, UpdatedAt: current}))
	response := httptest.NewRecorder()
	h.ServeHTTP(response, request)

	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d body=%s; want 422", response.Code, response.Body.String())
	}
	if galleryRepo.casCalls != 0 || galleryRepo.legacyCalls != 0 || galleryRepo.mutations != 0 || mediaRepo.casCalls != 0 || mediaRepo.legacyCalls != 0 {
		t.Fatalf("gallery CAS=%d legacy=%d mutations=%d; media CAS=%d legacy=%d", galleryRepo.casCalls, galleryRepo.legacyCalls, galleryRepo.mutations, mediaRepo.casCalls, mediaRepo.legacyCalls)
	}
	if got := response.Header().Get("ETag"); got != "" {
		t.Fatalf("invalid JSON response unexpectedly returned ETag %q", got)
	}
}

func TestPublicationMultimediaFallbackUsesCASAndRenewsETag(t *testing.T) {
	current := time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC)
	renewed := current.Add(time.Second)
	galleryRepo := &publicationCASRepo{repoStub: &repoStub{idempotency: map[string]gallery.Photo{}}, notSynthetic: true}
	galleryService, err := gallery.NewService(galleryRepo, &filesStub{items: map[string][]byte{}}, idsStub{}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	mediaRepo := &publicationMediaCASRepo{renewed: renewed}
	mediaService, err := media.NewService(mediaRepo, &replacementObjectStoreStub{}, replacementIDsStub{}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandlerWithMedia(authStub{}, galleryService, mediaService, nil)
	request := httptest.NewRequest(http.MethodPut, "/api/v1/spaces/"+testSpace+"/gallery/"+testPhoto+"/publication", strings.NewReader(`{"public_in_catalog":true}`))
	request.Header.Set("Authorization", "Bearer synthetic-test-token")
	request.Header.Set("If-Match", spaceshttp.DraftETag(spaces.Draft{ID: testSpace, UpdatedAt: current}))
	response := httptest.NewRecorder()
	h.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s; want 200", response.Code, response.Body.String())
	}
	if galleryRepo.casCalls != 1 || galleryRepo.legacyCalls != 0 || galleryRepo.mutations != 0 {
		t.Fatalf("gallery CAS=%d legacy=%d mutations=%d; want lookup only", galleryRepo.casCalls, galleryRepo.legacyCalls, galleryRepo.mutations)
	}
	if galleryRepo.ownerID != testOwner || galleryRepo.spaceID != testSpace || galleryRepo.photoID != testPhoto || !galleryRepo.expected.Equal(current) {
		t.Fatalf("gallery CAS scope owner=%q space=%q photo=%q expected=%v", galleryRepo.ownerID, galleryRepo.spaceID, galleryRepo.photoID, galleryRepo.expected)
	}
	if mediaRepo.casCalls != 1 || mediaRepo.legacyCalls != 0 {
		t.Fatalf("media CAS=%d legacy=%d; want CAS=1 and legacy=0", mediaRepo.casCalls, mediaRepo.legacyCalls)
	}
	if mediaRepo.ownerID != testOwner || mediaRepo.spaceID != testSpace || mediaRepo.photoID != testPhoto || !mediaRepo.expected.Equal(current) || !mediaRepo.visibleSet || !mediaRepo.visible {
		t.Fatalf("media CAS scope owner=%q space=%q photo=%q expected=%v visible=%t set=%t", mediaRepo.ownerID, mediaRepo.spaceID, mediaRepo.photoID, mediaRepo.expected, mediaRepo.visible, mediaRepo.visibleSet)
	}
	wantETag := spaceshttp.DraftETag(spaces.Draft{ID: testSpace, UpdatedAt: renewed})
	if got := response.Header().Get("ETag"); got != wantETag {
		t.Fatalf("ETag=%q want %q", got, wantETag)
	}
	var photo media.Photo
	if err := json.Unmarshal(response.Body.Bytes(), &photo); err != nil || photo.ID != testPhoto || !photo.Visible {
		t.Fatalf("photo=%+v err=%v body=%s", photo, err, response.Body.String())
	}
}

func TestPublicationMultimediaFallbackReturns412OnStaleCAS(t *testing.T) {
	current := time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC)
	stale := current.Add(-time.Second)
	galleryRepo := &publicationCASRepo{repoStub: &repoStub{idempotency: map[string]gallery.Photo{}}, notSynthetic: true}
	galleryService, err := gallery.NewService(galleryRepo, &filesStub{items: map[string][]byte{}}, idsStub{}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	mediaRepo := &publicationMediaCASRepo{renewed: current.Add(time.Second), err: media.ErrPrecondition}
	mediaService, err := media.NewService(mediaRepo, &replacementObjectStoreStub{}, replacementIDsStub{}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandlerWithMedia(authStub{}, galleryService, mediaService, nil)
	request := httptest.NewRequest(http.MethodPut, "/api/v1/spaces/"+testSpace+"/gallery/"+testPhoto+"/publication", strings.NewReader(`{"public_in_catalog":true}`))
	request.Header.Set("Authorization", "Bearer synthetic-test-token")
	request.Header.Set("If-Match", spaceshttp.DraftETag(spaces.Draft{ID: testSpace, UpdatedAt: stale}))
	response := httptest.NewRecorder()
	h.ServeHTTP(response, request)

	if response.Code != http.StatusPreconditionFailed {
		t.Fatalf("status=%d body=%s; want 412", response.Code, response.Body.String())
	}
	if galleryRepo.casCalls != 1 || galleryRepo.legacyCalls != 0 || galleryRepo.mutations != 0 || mediaRepo.casCalls != 1 || mediaRepo.legacyCalls != 0 {
		t.Fatalf("gallery CAS=%d legacy=%d mutations=%d; media CAS=%d legacy=%d", galleryRepo.casCalls, galleryRepo.legacyCalls, galleryRepo.mutations, mediaRepo.casCalls, mediaRepo.legacyCalls)
	}
	if galleryRepo.ownerID != testOwner || galleryRepo.spaceID != testSpace || galleryRepo.photoID != testPhoto || !galleryRepo.expected.Equal(stale) {
		t.Fatalf("gallery CAS scope owner=%q space=%q photo=%q expected=%v", galleryRepo.ownerID, galleryRepo.spaceID, galleryRepo.photoID, galleryRepo.expected)
	}
	if mediaRepo.ownerID != testOwner || mediaRepo.spaceID != testSpace || mediaRepo.photoID != testPhoto || !mediaRepo.expected.Equal(stale) || !mediaRepo.visibleSet || !mediaRepo.visible {
		t.Fatalf("media CAS scope owner=%q space=%q photo=%q expected=%v visible=%t set=%t", mediaRepo.ownerID, mediaRepo.spaceID, mediaRepo.photoID, mediaRepo.expected, mediaRepo.visible, mediaRepo.visibleSet)
	}
	if got := response.Header().Get("ETag"); got != "" {
		t.Fatalf("stale response unexpectedly returned ETag %q", got)
	}
}
