package publiccatalogpg

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/HernanEspinozaDev/espaciGo/internal/media"
	"github.com/HernanEspinozaDev/espaciGo/internal/publiccatalog"
	publiccataloghttp "github.com/HernanEspinozaDev/espaciGo/internal/publiccatalog/transport/http"
)

type publicImageFlowFileStore struct {
	body  []byte
	key   string
	calls int
	err   error
}

func (s *publicImageFlowFileStore) Get(_ context.Context, key string) ([]byte, error) {
	s.calls++
	s.key = key
	if s.err != nil {
		return nil, s.err
	}
	return append([]byte(nil), s.body...), nil
}

type publicImageFlowObjectStore struct {
	body       []byte
	key        string
	generation int64
	calls      int
	gotKey     string
	gotGen     int64
	err        error
}

func (*publicImageFlowObjectStore) Put(context.Context, string, []byte, string) (media.StoredObject, error) {
	return media.StoredObject{}, errors.New("unexpected Put")
}
func (s *publicImageFlowObjectStore) Get(_ context.Context, key string, generation int64) ([]byte, error) {
	s.calls++
	s.gotKey, s.gotGen = key, generation
	if key != s.key || generation != s.generation {
		return nil, media.ErrNotFound
	}
	if s.err != nil {
		return nil, s.err
	}
	return append([]byte(nil), s.body...), nil
}
func (*publicImageFlowObjectStore) Generation(context.Context, string) (int64, error) {
	return 0, errors.New("unexpected Generation")
}
func (*publicImageFlowObjectStore) Delete(context.Context, string, int64) error {
	return errors.New("unexpected Delete")
}

func TestPublicImageAdapterMetadataFlowsThroughServiceAndHTTP(t *testing.T) {
	const (
		spaceID = "11111111-1111-4111-8111-111111111111"
		imageID = "22222222-2222-4222-8222-222222222222"
	)
	var pngBuffer bytes.Buffer
	if err := png.Encode(&pngBuffer, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	pngBody := pngBuffer.Bytes()
	invalidPNGBody := []byte("not a PNG image")
	webpBody := []byte("synthetic normalized WebP bytes")
	pngDigest := sha256.Sum256(pngBody)
	invalidPNGDigest := sha256.Sum256(invalidPNGBody)
	webpDigest := sha256.Sum256(webpBody)
	wrongDigest := sha256.Sum256([]byte("different synthetic image bytes"))
	cases := []struct {
		name            string
		path            string
		row             []any
		body            []byte
		fileID          string
		objectKey       string
		generation      int64
		storeGeneration int64
		wantStatus      int
		storeErr        error
	}{
		{
			name:   "synthetic PNG file",
			path:   "/api/v1/public/spaces/" + spaceID + "/images/" + imageID,
			row:    []any{"image/png", hex.EncodeToString(pngDigest[:]), "synthetic-file-17", int64(len(pngBody))},
			body:   pngBody,
			fileID: "synthetic-file-17",
		},
		{
			name:       "PNG digest mismatch",
			path:       "/api/v1/public/spaces/" + spaceID + "/images/" + imageID,
			row:        []any{"image/png", hex.EncodeToString(wrongDigest[:]), "synthetic-file-17", int64(len(pngBody))},
			body:       pngBody,
			fileID:     "synthetic-file-17",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "PNG declared size mismatch",
			path:       "/api/v1/public/spaces/" + spaceID + "/images/" + imageID,
			row:        []any{"image/png", hex.EncodeToString(pngDigest[:]), "synthetic-file-17", int64(len(pngBody) + 1)},
			body:       pngBody,
			fileID:     "synthetic-file-17",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "PNG signature mismatch",
			path:       "/api/v1/public/spaces/" + spaceID + "/images/" + imageID,
			row:        []any{"image/png", hex.EncodeToString(invalidPNGDigest[:]), "synthetic-file-18", int64(len(invalidPNGBody))},
			body:       invalidPNGBody,
			fileID:     "synthetic-file-18",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "PNG file-store error",
			path:       "/api/v1/public/spaces/" + spaceID + "/images/" + imageID,
			row:        []any{"image/png", hex.EncodeToString(pngDigest[:]), "synthetic-file-17", int64(len(pngBody))},
			body:       pngBody,
			fileID:     "synthetic-file-17",
			storeErr:   errors.New("synthetic private image store failure"),
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "public WebP media variant",
			path:       "/api/v1/public/spaces/" + spaceID + "/images/" + imageID + "/variants/display",
			row:        []any{"image/webp", hex.EncodeToString(webpDigest[:]), int64(len(webpBody)), "synthetic/spaces/space-17/image-3/display.webp", "fixture-bucket", int64(9)},
			body:       webpBody,
			objectKey:  "synthetic/spaces/space-17/image-3/display.webp",
			generation: 9,
		},
		{
			name:       "WebP object-store error",
			path:       "/api/v1/public/spaces/" + spaceID + "/images/" + imageID + "/variants/display",
			row:        []any{"image/webp", hex.EncodeToString(webpDigest[:]), int64(len(webpBody)), "synthetic/spaces/space-17/image-3/display.webp", "fixture-bucket", int64(9)},
			body:       webpBody,
			objectKey:  "synthetic/spaces/space-17/image-3/display.webp",
			generation: 9,
			storeErr:   errors.New("synthetic private image store failure"),
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "WebP digest mismatch",
			path:       "/api/v1/public/spaces/" + spaceID + "/images/" + imageID + "/variants/display",
			row:        []any{"image/webp", hex.EncodeToString(wrongDigest[:]), int64(len(webpBody)), "synthetic/spaces/space-17/image-3/display.webp", "fixture-bucket", int64(9)},
			body:       webpBody,
			objectKey:  "synthetic/spaces/space-17/image-3/display.webp",
			generation: 9,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "WebP declared size mismatch",
			path:       "/api/v1/public/spaces/" + spaceID + "/images/" + imageID + "/variants/display",
			row:        []any{"image/webp", hex.EncodeToString(webpDigest[:]), int64(len(webpBody) + 1), "synthetic/spaces/space-17/image-3/display.webp", "fixture-bucket", int64(9)},
			body:       webpBody,
			objectKey:  "synthetic/spaces/space-17/image-3/display.webp",
			generation: 9,
			wantStatus: http.StatusNotFound,
		},
		{
			name:            "stale WebP generation",
			path:            "/api/v1/public/spaces/" + spaceID + "/images/" + imageID + "/variants/display",
			row:             []any{"image/webp", hex.EncodeToString(webpDigest[:]), int64(len(webpBody)), "synthetic/spaces/space-17/image-3/display.webp", "fixture-bucket", int64(9)},
			body:            webpBody,
			objectKey:       "synthetic/spaces/space-17/image-3/display.webp",
			generation:      9,
			storeGeneration: 8,
			wantStatus:      http.StatusNotFound,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pool := &publicImageAdapterPool{row: tc.row}
			repo := &Repository{pool: pool}
			files := &publicImageFlowFileStore{body: tc.body, err: tc.storeErr}
			storeGeneration := tc.storeGeneration
			if storeGeneration == 0 {
				storeGeneration = tc.generation
			}
			objects := &publicImageFlowObjectStore{body: tc.body, key: tc.objectKey, generation: storeGeneration, err: tc.storeErr}
			var service *publiccatalog.Service
			var err error
			if tc.objectKey == "" {
				service, err = publiccatalog.NewWithImageStore(repo, []byte("01234567890123456789012345678901"), files)
			} else {
				service, err = publiccatalog.NewWithImageAndMediaStore(repo, []byte("01234567890123456789012345678901"), files, objects)
			}
			if err != nil {
				t.Fatalf("construct service: %v", err)
			}
			response := httptest.NewRecorder()
			publiccataloghttp.New(service).ServeHTTP(response, httptest.NewRequest(http.MethodGet, tc.path, nil))
			wantStatus := tc.wantStatus
			if wantStatus == 0 {
				wantStatus = http.StatusOK
			}
			if response.Code != wantStatus {
				t.Fatalf("status=%d, want %d; body=%s", response.Code, wantStatus, response.Body.String())
			}
			if wantStatus == http.StatusOK && (response.Header().Get("Content-Type") != tc.row[0] || response.Header().Get("Content-Length") != fmt.Sprint(len(tc.body)) || !bytes.Equal(response.Body.Bytes(), tc.body)) {
				t.Fatalf("status=%d mime=%q length=%q body_matches=%v", response.Code, response.Header().Get("Content-Type"), response.Header().Get("Content-Length"), bytes.Equal(response.Body.Bytes(), tc.body))
			}
			if wantStatus != http.StatusOK && (response.Header().Get("Cache-Control") != "no-store" || bytes.Contains(response.Body.Bytes(), tc.body) || strings.HasPrefix(response.Header().Get("Content-Type"), "image/")) {
				t.Fatalf("unavailable image response cache=%q leaked_object=%v mime=%q body=%s", response.Header().Get("Cache-Control"), bytes.Contains(response.Body.Bytes(), tc.body), response.Header().Get("Content-Type"), response.Body.String())
			}
			if tc.storeErr != nil && strings.Contains(response.Body.String(), tc.storeErr.Error()) {
				t.Fatalf("store error leaked in response: %s", response.Body.String())
			}
			if tc.objectKey == "" {
				if !reflect.DeepEqual(pool.args, []any{spaceID, imageID}) || files.calls != 1 || files.key != tc.fileID || objects.calls != 0 {
					t.Errorf("adapter args=%v file store calls=%d key=%q object store calls=%d", pool.args, files.calls, files.key, objects.calls)
				}
				return
			}
			if !reflect.DeepEqual(pool.args, []any{spaceID, imageID, "display"}) || objects.calls != 1 || objects.gotKey != tc.objectKey || objects.gotGen != tc.generation || files.calls != 0 {
				t.Errorf("adapter args=%v object store calls=%d key=%q generation=%d file store calls=%d", pool.args, objects.calls, objects.gotKey, objects.gotGen, files.calls)
			}
			if wantStatus == http.StatusOK && (response.Header().Get("ETag") != `"`+hex.EncodeToString(webpDigest[:])+`"` || response.Header().Get("Cache-Control") != "no-store") {
				t.Errorf("ETag=%q cache=%q", response.Header().Get("ETag"), response.Header().Get("Cache-Control"))
			}
		})
	}
}
