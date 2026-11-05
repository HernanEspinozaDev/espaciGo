package galleryhttp

import (
	"context"
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

type metadataCASMediaRepoStub struct {
	media.Repository
	updateCalls     int
	casCalls        int
	currentRevision time.Time
	renewedRevision time.Time
}

func (r *metadataCASMediaRepoStub) UpdateSpacePhoto(_ context.Context, _, _, photoID string, alt *string, visible *bool, position *int, at time.Time) (media.Photo, error) {
	r.updateCalls++
	photo := media.Photo{ID: photoID, SpaceID: testSpace, OwnerID: testOwner, State: "ready", Visible: true, CreatedAt: at}
	if alt != nil {
		photo.AltText = *alt
	}
	if visible != nil {
		photo.Visible = *visible
	}
	if position != nil {
		photo.Position = *position
	}
	return photo, nil
}

func TestRealMediaMetadataPutRequiresIfMatchBeforeMutation(t *testing.T) {
	spaceService, err := gallery.NewService(
		&repoStub{idempotency: map[string]gallery.Photo{}},
		&filesStub{items: map[string][]byte{}}, idsStub{}, time.Now,
	)
	if err != nil {
		t.Fatal(err)
	}
	mediaRepo := &metadataCASMediaRepoStub{}
	mediaService, err := media.NewService(mediaRepo, &replacementObjectStoreStub{}, replacementIDsStub{}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandlerWithMedia(authStub{}, spaceService, mediaService, nil)
	request := httptest.NewRequest(http.MethodPut,
		"/api/v1/spaces/"+testSpace+"/gallery/"+testPhoto,
		strings.NewReader(`{"position":2}`),
	)
	request.Header.Set("Authorization", "Bearer synthetic-test-token")
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusPreconditionRequired {
		t.Fatalf("PUT /api/v1/spaces/%s/gallery/%s status=%d body=%s; want 428", testSpace, testPhoto, response.Code, response.Body.String())
	}
	if mediaRepo.updateCalls != 0 {
		t.Fatalf("media metadata mutation calls=%d; want 0 when If-Match is missing", mediaRepo.updateCalls)
	}
}

func (r *metadataCASMediaRepoStub) UpdateSpacePhotoCAS(_ context.Context, _, _, photoID string, alt *string, visible *bool, position *int, expected, _ time.Time) (media.Photo, time.Time, error) {
	r.casCalls++
	if !expected.Equal(r.currentRevision) {
		return media.Photo{}, time.Time{}, media.ErrPrecondition
	}
	photo := media.Photo{ID: photoID, SpaceID: testSpace, OwnerID: testOwner, State: "ready", Visible: true, CreatedAt: r.renewedRevision}
	if alt != nil {
		photo.AltText = *alt
	}
	if visible != nil {
		photo.Visible = *visible
	}
	if position != nil {
		photo.Position = *position
	}
	return photo, r.renewedRevision, nil
}

func TestRealMediaMetadataPutPreconditions(t *testing.T) {
	current := time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC)
	renewed := current.Add(time.Second)
	valid := spaceshttp.DraftETag(spaces.Draft{ID: testSpace, UpdatedAt: current})
	stale := spaceshttp.DraftETag(spaces.Draft{ID: testSpace, UpdatedAt: current.Add(-time.Second)})
	otherSpace := spaceshttp.DraftETag(spaces.Draft{ID: "dddddddd-dddd-4ddd-8ddd-dddddddddddd", UpdatedAt: current})
	cases := []struct {
		name, header string
		values       []string
		want         int
		wantCASCalls int
	}{
		{name: "weak", header: "W/" + valid, want: http.StatusBadRequest},
		{name: "wildcard", header: "*", want: http.StatusBadRequest},
		{name: "multiple", values: []string{valid, valid}, want: http.StatusBadRequest},
		{name: "malformed", header: `"not-an-etag"`, want: http.StatusBadRequest},
		{name: "wrong space", header: otherSpace, want: http.StatusPreconditionFailed},
		{name: "stale revision", header: stale, want: http.StatusPreconditionFailed, wantCASCalls: 1},
		{name: "valid CAS", header: valid, want: http.StatusOK, wantCASCalls: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spaceService, err := gallery.NewService(&repoStub{idempotency: map[string]gallery.Photo{}}, &filesStub{items: map[string][]byte{}}, idsStub{}, time.Now)
			if err != nil {
				t.Fatal(err)
			}
			mediaRepo := &metadataCASMediaRepoStub{currentRevision: current, renewedRevision: renewed}
			mediaService, err := media.NewService(mediaRepo, &replacementObjectStoreStub{}, replacementIDsStub{}, time.Now)
			if err != nil {
				t.Fatal(err)
			}
			handler := NewHandlerWithMedia(authStub{}, spaceService, mediaService, nil)
			request := httptest.NewRequest(http.MethodPut, "/api/v1/spaces/"+testSpace+"/gallery/"+testPhoto, strings.NewReader(`{"position":2}`))
			request.Header.Set("Authorization", "Bearer synthetic-test-token")
			request.Header.Set("Content-Type", "application/json")
			if tc.values != nil {
				for _, value := range tc.values {
					request.Header.Add("If-Match", value)
				}
			} else {
				request.Header.Set("If-Match", tc.header)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != tc.want {
				t.Fatalf("status=%d body=%s; want %d", response.Code, response.Body.String(), tc.want)
			}
			if mediaRepo.casCalls != tc.wantCASCalls || mediaRepo.updateCalls != 0 {
				t.Fatalf("CAS calls=%d legacy calls=%d; want CAS=%d and legacy=0", mediaRepo.casCalls, mediaRepo.updateCalls, tc.wantCASCalls)
			}
			if tc.name == "valid CAS" {
				wantTag := spaceshttp.DraftETag(spaces.Draft{ID: testSpace, UpdatedAt: renewed})
				if response.Header().Get("ETag") != wantTag || !strings.HasPrefix(response.Header().Get("ETag"), `"`) || strings.HasPrefix(response.Header().Get("ETag"), `W/`) {
					t.Fatalf("ETag=%q want strong %q", response.Header().Get("ETag"), wantTag)
				}
			}
		})
	}
}
