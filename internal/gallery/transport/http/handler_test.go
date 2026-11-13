package galleryhttp

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/gallery"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
	"github.com/HernanEspinozaDev/espaciGo/internal/media"
	"github.com/HernanEspinozaDev/espaciGo/internal/verification"
)

func TestCoverStaleIfMatchReturnsPreconditionFailed(t *testing.T) {
	files := &filesStub{items: map[string][]byte{}}
	repo := &repoStub{
		items:       []gallery.Photo{{ID: testPhoto}},
		idempotency: map[string]gallery.Photo{},
	}
	svc, err := gallery.NewService(repo, files, idsStub{}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandler(authStub{}, svc, nil)
	r := httptest.NewRequest(http.MethodPut, "/api/v1/spaces/"+testSpace+"/cover", strings.NewReader(`{"photo_id":"`+testPhoto+`"}`))
	r.Header.Set("Authorization", "Bearer test")
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("If-Match", `"`+base64.RawURLEncoding.EncodeToString([]byte(testSpace+"\x00"+"2026-10-08T00:00:00Z"))+`"`)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error response: %v; body=%s", err, w.Body.String())
	}
	if w.Code != http.StatusPreconditionFailed || body.Error.Code != "precondition_failed" {
		t.Fatalf("status=%d code=%q body=%s; want 412 precondition_failed", w.Code, body.Error.Code, w.Body.String())
	}
}

type staleMediaCoverRepoStub struct {
	media.Repository
	current  string
	selected string
}

func (r *staleMediaCoverRepoStub) SelectSpaceCoverCAS(_ context.Context, _, _, _ string, expected time.Time) (time.Time, error) {
	if expected.Format(time.RFC3339Nano) != r.current {
		return time.Time{}, media.ErrPrecondition
	}
	r.selected = testPhoto
	return expected.Add(time.Second), nil
}

func TestRealMediaCoverStaleIfMatchPreservesSelectionAndRevision(t *testing.T) {
	current := "2026-10-09T00:00:00Z"
	stale := "2026-10-08T00:00:00Z"
	mediaRepo := &staleMediaCoverRepoStub{current: current, selected: "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee"}
	mediaService, err := media.NewService(mediaRepo, &replacementObjectStoreStub{}, replacementIDsStub{}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	spaceService, err := gallery.NewService(&repoStub{items: []gallery.Photo{{ID: testPhoto}}, idempotency: map[string]gallery.Photo{}}, &filesStub{items: map[string][]byte{}}, idsStub{}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandlerWithMedia(authStub{}, spaceService, mediaService, nil)
	r := httptest.NewRequest(http.MethodPut, "/api/v1/spaces/"+testSpace+"/cover", strings.NewReader(`{"photo_id":"`+testPhoto+`"}`))
	r.Header.Set("Authorization", "Bearer synthetic-test-token")
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("If-Match", `"`+base64.RawURLEncoding.EncodeToString([]byte(testSpace+"\x00"+stale))+`"`)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v body=%s", err, w.Body.String())
	}
	if w.Code != http.StatusPreconditionFailed || body.Error.Code != "precondition_failed" {
		t.Fatalf("status=%d code=%q body=%s", w.Code, body.Error.Code, w.Body.String())
	}
	if mediaRepo.selected != "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee" || mediaRepo.current != current {
		t.Fatalf("media cover mutated: selected=%s revision=%s", mediaRepo.selected, mediaRepo.current)
	}
}

func TestCoverRequiresIfMatchBeforeReadingBody(t *testing.T) {
	files := &filesStub{items: map[string][]byte{}}
	repo := &repoStub{idempotency: map[string]gallery.Photo{}}
	svc, err := gallery.NewService(repo, files, idsStub{}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandler(authStub{}, svc, nil)
	r := httptest.NewRequest(http.MethodPut, "/api/v1/spaces/"+testSpace+"/cover", strings.NewReader("not json"))
	r.Header.Set("Authorization", "Bearer test")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusPreconditionRequired {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestMissingImageFieldRemovesMultipartTempFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	part, err := w.CreateFormFile("other", "large.bin")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(bytes.Repeat([]byte("x"), 2<<20)); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodPost, "/upload", &body)
	r.Header.Set("Content-Type", w.FormDataContentType())
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		t.Fatal(err)
	}
	if _, _, err := r.FormFile("image"); err == nil {
		t.Fatal("expected missing image field")
	}
	removeMultipartTempFiles(r)
	entries, err := os.ReadDir(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("multipart temp files remain: %s", filepath.Join(tmp, entries[0].Name()))
	}
}

const testOwner = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
const testSpace = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
const testPhoto = "cccccccc-cccc-4ccc-8ccc-cccccccccccc"

type authStub struct{}

func (authStub) Authorize(context.Context, identity.Secret, identity.Role, identity.ActivityKind) (identity.Principal, error) {
	return identity.Principal{AccountID: testOwner, Roles: []identity.Role{identity.RoleLandlord}}, nil
}

type filesStub struct {
	mu    sync.Mutex
	items map[string][]byte
}

func (f *filesStub) Put(_ context.Context, id string, b []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.items[id] = append([]byte{}, b...)
	return nil
}
func (f *filesStub) Get(_ context.Context, id string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	b, ok := f.items[id]
	if !ok {
		return nil, context.Canceled
	}
	return append([]byte{}, b...), nil
}
func (f *filesStub) Delete(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.items, id)
	return nil
}

type idsStub struct{}

func (idsStub) ID() (string, error) { return testPhoto, nil }

type repoStub struct {
	items       []gallery.Photo
	idempotency map[string]gallery.Photo
}

func (r *repoStub) ReserveCandidate(context.Context, string, string, string, time.Time) error {
	return nil
}
func (r *repoStub) BeginCandidate(context.Context, string, string, string) (gallery.CandidateWriter, error) {
	return &candidateWriterStub{repo: r}, nil
}
func (r *repoStub) QueueCandidateCleanup(context.Context, string, time.Time) error { return nil }
func (r *repoStub) ClaimCandidateCleanup(context.Context, int) ([]string, error)   { return nil, nil }
func (r *repoStub) CompleteCandidateCleanup(context.Context, string) error         { return nil }
func (r *repoStub) FailCandidateCleanup(context.Context, string, time.Time) error  { return nil }

type candidateWriterStub struct{ repo *repoStub }

func (w *candidateWriterStub) Add(_ context.Context, p gallery.Photo, key string, _ time.Time) (gallery.Photo, bool, error) {
	if old, ok := w.repo.idempotency[key]; ok {
		return old, true, nil
	}
	w.repo.idempotency[key] = p
	w.repo.items = append(w.repo.items, p)
	return p, false, nil
}
func (*candidateWriterStub) QueueCleanup(context.Context, time.Time) error { return nil }
func (*candidateWriterStub) Close() error                                  { return nil }
func (r *repoStub) List(context.Context, string, string) ([]gallery.Photo, error) {
	return append([]gallery.Photo{}, r.items...), nil
}
func (r *repoStub) Get(_ context.Context, _, _, id string) (gallery.Photo, error) {
	for _, p := range r.items {
		if strings.EqualFold(p.ID, id) {
			return p, nil
		}
	}
	return gallery.Photo{}, gallery.ErrNotFound
}
func (r *repoStub) Remove(_ context.Context, _, _, id string) (gallery.Removal, error) {
	return gallery.Removal{PhotoID: id, Removed: true}, nil
}
func (r *repoStub) SelectCover(_ context.Context, _, spaceID, photoID string) (gallery.Cover, error) {
	return r.SelectCoverCAS(context.Background(), testOwner, spaceID, photoID, time.Time{})
}
func (r *repoStub) SelectCoverCAS(_ context.Context, _, spaceID, photoID string, expected time.Time) (gallery.Cover, error) {
	if _, err := r.Get(context.Background(), testOwner, spaceID, photoID); err != nil {
		return gallery.Cover{}, err
	}
	updated := time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC)
	if !expected.IsZero() && !updated.Equal(expected) {
		return gallery.Cover{}, gallery.ErrConflict
	}
	return gallery.Cover{SpaceID: spaceID, PhotoID: photoID, Selected: true, UpdatedAt: updated}, nil
}
func (r *repoStub) SetPublicInCatalog(_ context.Context, _, spaceID, photoID string, enabled bool) (gallery.Photo, error) {
	for i := range r.items {
		if r.items[i].ID == photoID {
			r.items[i].PublicInCatalog = enabled
			return r.items[i], nil
		}
	}
	return gallery.Photo{}, gallery.ErrNotFound
}
func (r *repoStub) PendingCleanup(context.Context, int) ([]string, error)    { return nil, nil }
func (r *repoStub) CompleteCleanup(context.Context, string, time.Time) error { return nil }
func (r *repoStub) FailCleanup(context.Context, string, time.Time) error     { return nil }

func TestDeleteMalformedIfMatchBeforeMutation(t *testing.T) {
	files := &filesStub{items: map[string][]byte{}}
	repo := &repoStub{idempotency: map[string]gallery.Photo{}}
	repo.items = []gallery.Photo{{ID: testPhoto}}
	svc, err := gallery.NewService(repo, files, idsStub{}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(authStub{}, svc, nil)
	request := httptest.NewRequest(http.MethodDelete, "/api/v1/spaces/"+testSpace+"/gallery/"+testPhoto, nil)
	request.Header.Set("Authorization", "Bearer synthetic-test-token")
	request.Header.Set("If-Match", "bad")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s; want 400", response.Code, response.Body.String())
	}
}

func TestDeleteRequiresIfMatchBeforeMutation(t *testing.T) {
	files := &filesStub{items: map[string][]byte{}}
	repo := &repoStub{idempotency: map[string]gallery.Photo{}}
	item := gallery.Photo{ID: testPhoto}
	repo.items = []gallery.Photo{item}
	svc, err := gallery.NewService(repo, files, idsStub{}, func() time.Time { return time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC) })
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(authStub{}, svc, []string{"http://localhost"})
	request := httptest.NewRequest(http.MethodDelete, "/api/v1/spaces/"+testSpace+"/gallery/"+testPhoto, nil)
	request.Header.Set("Authorization", "Bearer synthetic-test-token")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusPreconditionRequired {
		t.Fatalf("status=%d body=%s; want 428", response.Code, response.Body.String())
	}
}

func TestAssembledRouterKeepsSpaceRoutesAndChecksGalleryErrors(t *testing.T) {
	files := &filesStub{items: map[string][]byte{}}
	repo := &repoStub{idempotency: map[string]gallery.Photo{}}
	svc, err := gallery.NewService(repo, files, idsStub{}, func() time.Time { return time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC) })
	if err != nil {
		t.Fatal(err)
	}
	galleryHandler := NewHandler(authStub{}, svc, []string{"http://localhost"})
	mux := http.NewServeMux()
	mux.Handle("/api/v1/spaces/{spaceID}/gallery", galleryHandler)
	mux.Handle("/api/v1/spaces/{spaceID}/cover", galleryHandler)
	mux.Handle("/api/v1/spaces/{spaceID}/gallery/{photoID}", galleryHandler)
	mux.Handle("/api/v1/spaces/{spaceID}/gallery/{photoID}/content", galleryHandler)
	mux.Handle("/api/v1/spaces/", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(209)
		_, _ = w.Write([]byte("existing space route"))
	}))
	preflight := httptest.NewRequest(http.MethodOptions, "/api/v1/spaces/"+testSpace+"/cover", nil)
	preflight.Header.Set("Origin", "http://localhost")
	preflightResponse := httptest.NewRecorder()
	mux.ServeHTTP(preflightResponse, preflight)
	if preflightResponse.Code != http.StatusNoContent || !strings.Contains(preflightResponse.Header().Get("Access-Control-Allow-Methods"), "PUT") || !strings.Contains(preflightResponse.Header().Get("Access-Control-Allow-Headers"), "If-Match") {
		t.Fatalf("cover preflight status=%d headers=%v", preflightResponse.Code, preflightResponse.Header())
	}
	// No Content-Type and an empty body deliberately preserve the synthetic POST path.
	create := httptest.NewRequest(http.MethodPost, "/api/v1/spaces/"+testSpace+"/gallery", nil)
	create.Header.Set("Authorization", "Bearer synthetic-test-token")
	create.Header.Set("Idempotency-Key", "gallery-test-key-01")
	created := httptest.NewRecorder()
	mux.ServeHTTP(created, create)
	if created.Code != http.StatusCreated {
		t.Fatalf("gallery create status=%d body=%s", created.Code, created.Body.String())
	}
	var response struct {
		Photo     gallery.Photo `json:"photo"`
		Reused    bool          `json:"reused"`
		Synthetic bool          `json:"synthetic"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &response); err != nil || response.Photo.ID != testPhoto || !response.Synthetic {
		t.Fatalf("gallery create response=%s err=%v", created.Body.String(), err)
	}
	invalidEmptyMultipartType := httptest.NewRequest(http.MethodPost, "/api/v1/spaces/"+testSpace+"/gallery", nil)
	invalidEmptyMultipartType.Header.Set("Authorization", "Bearer synthetic-test-token")
	invalidEmptyMultipartType.Header.Set("Content-Type", "multipart/form-dataXYZ")
	invalidEmptyMultipartResponse := httptest.NewRecorder()
	mux.ServeHTTP(invalidEmptyMultipartResponse, invalidEmptyMultipartType)
	if invalidEmptyMultipartResponse.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("invalid empty multipart media type status=%d body=%s", invalidEmptyMultipartResponse.Code, invalidEmptyMultipartResponse.Body.String())
	}
	invalidMultipart := httptest.NewRequest(http.MethodPost, "/api/v1/spaces/"+testSpace+"/gallery", strings.NewReader("not multipart"))
	invalidMultipart.Header.Set("Authorization", "Bearer synthetic-test-token")
	invalidMultipart.Header.Set("Content-Type", "multipart/form-dataXYZ; boundary=declared-boundary")
	invalidMultipartResponse := httptest.NewRecorder()
	mux.ServeHTTP(invalidMultipartResponse, invalidMultipart)
	if invalidMultipartResponse.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("invalid multipart media type status=%d body=%s", invalidMultipartResponse.Code, invalidMultipartResponse.Body.String())
	}
	coverRequest := httptest.NewRequest(http.MethodPut, "/api/v1/spaces/"+testSpace+"/cover", strings.NewReader(`{"photo_id":"`+testPhoto+`"}`))
	coverRequest.Header.Set("Authorization", "Bearer synthetic-test-token")
	coverRequest.Header.Set("Content-Type", "application/json")
	coverRequest.Header.Set("If-Match", `"`+base64.RawURLEncoding.EncodeToString([]byte(testSpace+"\x00"+"2026-10-09T00:00:00Z"))+`"`)
	cover := httptest.NewRecorder()
	mux.ServeHTTP(cover, coverRequest)
	if cover.Code != http.StatusOK || !strings.Contains(cover.Body.String(), `"photo_id":"`+testPhoto+`"`) || !strings.Contains(cover.Body.String(), `"selected":true`) {
		t.Fatalf("cover selection status=%d body=%s", cover.Code, cover.Body.String())
	}
	contentRequest := httptest.NewRequest(http.MethodGet, "/api/v1/spaces/"+testSpace+"/gallery/"+testPhoto+"/content", nil)
	contentRequest.Header.Set("Authorization", "Bearer synthetic-test-token")
	content := httptest.NewRecorder()
	mux.ServeHTTP(content, contentRequest)
	if content.Code != 200 || content.Header().Get("Content-Type") != "image/png" || !bytes.HasPrefix(content.Body.Bytes(), []byte("\x89PNG\r\n\x1a\n")) {
		t.Fatalf("private PNG content status=%d type=%s", content.Code, content.Header().Get("Content-Type"))
	}
	metadataRequest := httptest.NewRequest(http.MethodGet, "/api/v1/spaces/"+testSpace+"/gallery/"+strings.ToUpper(testPhoto), nil)
	metadataRequest.Header.Set("Authorization", "Bearer synthetic-test-token")
	metadata := httptest.NewRecorder()
	mux.ServeHTTP(metadata, metadataRequest)
	if metadata.Code != http.StatusOK || !bytes.Contains(metadata.Body.Bytes(), []byte(testPhoto)) {
		t.Fatalf("uppercase UUID metadata status=%d body=%s", metadata.Code, metadata.Body.String())
	}
	existing := httptest.NewRecorder()
	mux.ServeHTTP(existing, httptest.NewRequest(http.MethodPut, "/api/v1/spaces/"+testSpace+"/publication", nil))
	if existing.Code != 209 || existing.Body.String() != "existing space route" {
		t.Fatalf("existing publication route intercepted: %d %q", existing.Code, existing.Body.String())
	}
	unauthorized := httptest.NewRecorder()
	mux.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/api/v1/spaces/"+testSpace+"/gallery", nil))
	var errorBody struct {
		Error struct {
			Code      string `json:"code"`
			RequestID string `json:"request_id"`
		} `json:"error"`
	}
	if err := json.Unmarshal(unauthorized.Body.Bytes(), &errorBody); err != nil || unauthorized.Code != 401 || unauthorized.Header().Get("WWW-Authenticate") != "Bearer" || unauthorized.Header().Get("X-Request-ID") == "" || errorBody.Error.RequestID != unauthorized.Header().Get("X-Request-ID") {
		t.Fatalf("common 401 contract status=%d body=%s headers=%v err=%v", unauthorized.Code, unauthorized.Body.String(), unauthorized.Header(), err)
	}
}

type galleryListMediaRepoStub struct {
	media.Repository
	owner  string
	space  string
	photos []media.Photo
	err    error
}

func (r *galleryListMediaRepoStub) ListSpace(_ context.Context, owner, spaceID string) ([]media.Photo, error) {
	r.owner = owner
	r.space = spaceID
	return append([]media.Photo(nil), r.photos...), r.err
}

func TestGalleryListMergesSyntheticAndRealMediaInPositionOrder(t *testing.T) {
	const realPhotoID = "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee"
	now := func() time.Time { return time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC) }

	spaceRepo := &repoStub{
		items:       []gallery.Photo{{ID: testPhoto, Position: 2}},
		idempotency: map[string]gallery.Photo{},
	}
	spaceService, err := gallery.NewService(spaceRepo, &filesStub{items: map[string][]byte{}}, idsStub{}, now)
	if err != nil {
		t.Fatal(err)
	}
	mediaRepo := &galleryListMediaRepoStub{photos: []media.Photo{{
		ID:       realPhotoID,
		SpaceID:  testSpace,
		Position: 1,
		State:    "ready",
		Variants: []media.Variant{{Name: media.VariantDisplay}},
	}}}
	mediaService, err := media.NewService(mediaRepo, &replacementObjectStoreStub{}, replacementIDsStub{}, now)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandlerWithMedia(authStub{}, spaceService, mediaService, nil)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/spaces/"+testSpace+"/gallery", nil)
	request.Header.Set("Authorization", "Bearer synthetic-test-token")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	var body struct {
		Items []struct {
			ID       string          `json:"id"`
			Position int             `json:"position"`
			Variants []media.Variant `json:"variants"`
		} `json:"items"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode list response: %v; body=%s", err, response.Body.String())
	}
	if response.Code != http.StatusOK || len(body.Items) != 2 {
		t.Fatalf("status=%d items=%+v body=%s", response.Code, body.Items, response.Body.String())
	}
	if mediaRepo.owner != testOwner || mediaRepo.space != testSpace {
		t.Fatalf("media list scope owner=%q space=%q; want %q/%q", mediaRepo.owner, mediaRepo.space, testOwner, testSpace)
	}
	if body.Items[0].ID != realPhotoID || body.Items[0].Position != 1 || body.Items[1].ID != testPhoto || body.Items[1].Position != 2 {
		t.Fatalf("gallery order=%+v; want real photo at 1 then synthetic photo at 2", body.Items)
	}
	if len(body.Items[0].Variants) != 1 || body.Items[0].Variants[0].URL != "/api/v1/spaces/"+testSpace+"/gallery/"+realPhotoID+"/variants/display" {
		t.Fatalf("real media variant URL=%+v", body.Items[0].Variants)
	}
}

func TestGalleryListFailsClosedWhenRealMediaListingFails(t *testing.T) {
	const realPhotoID = "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee"
	now := func() time.Time { return time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC) }

	spaceService, err := gallery.NewService(&repoStub{
		items:       []gallery.Photo{{ID: testPhoto, Position: 1}},
		idempotency: map[string]gallery.Photo{},
	}, &filesStub{items: map[string][]byte{}}, idsStub{}, now)
	if err != nil {
		t.Fatal(err)
	}
	mediaRepo := &galleryListMediaRepoStub{
		photos: []media.Photo{{ID: realPhotoID, SpaceID: testSpace, Position: 2, State: "ready"}},
		err:    context.DeadlineExceeded,
	}
	mediaService, err := media.NewService(mediaRepo, &replacementObjectStoreStub{}, replacementIDsStub{}, now)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandlerWithMedia(authStub{}, spaceService, mediaService, nil)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/spaces/"+testSpace+"/gallery", nil)
	request.Header.Set("Authorization", "Bearer synthetic-test-token")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	var body struct {
		Error struct {
			Code      string `json:"code"`
			Message   string `json:"message"`
			RequestID string `json:"request_id"`
		} `json:"error"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error response: %v; body=%s", err, response.Body.String())
	}
	if response.Code != http.StatusServiceUnavailable || body.Error.Code != "media_unavailable" {
		t.Fatalf("status=%d code=%q body=%s; want 503 media_unavailable", response.Code, body.Error.Code, response.Body.String())
	}
	if body.Error.RequestID == "" || body.Error.RequestID != response.Header().Get("X-Request-ID") {
		t.Fatalf("request id body=%q header=%q", body.Error.RequestID, response.Header().Get("X-Request-ID"))
	}
	if strings.Contains(response.Body.String(), testPhoto) || strings.Contains(response.Body.String(), realPhotoID) || strings.Contains(response.Body.String(), context.DeadlineExceeded.Error()) {
		t.Fatalf("partial gallery or internal error leaked in response: %s", response.Body.String())
	}
	if mediaRepo.owner != testOwner || mediaRepo.space != testSpace {
		t.Fatalf("media list scope owner=%q space=%q; want %q/%q", mediaRepo.owner, mediaRepo.space, testOwner, testSpace)
	}
}

const replacementAssetID = "dddddddd-dddd-4ddd-8ddd-dddddddddddd"

type replacementIDsStub struct{}

func (replacementIDsStub) ID() (string, error) { return replacementAssetID, nil }

type replacementRepoStub struct {
	media.Repository
	reservation              media.Reservation
	photo                    media.Photo
	reserveCalls             int
	planCalls                int
	finalizeCalls            int
	getCalls                 int
	key, inputHash, targetID string
	ownerID, reservedSpaceID string
	purpose                  media.Purpose
	reserveRevision          string
	finalizeRevision         string
	recordedVariants         []media.Variant
	reserveErr               error
}

func (r *replacementRepoStub) Reserve(_ context.Context, id, owner, spaceID string, purpose media.Purpose, key, inputHash string, _ time.Time, leaseTill time.Time, options media.ReservationOptions) (media.Reservation, bool, error) {
	r.reserveCalls++
	if r.reserveErr != nil {
		return media.Reservation{}, false, r.reserveErr
	}
	if r.reserveCalls > 1 {
		if key == r.key {
			if owner != r.ownerID || spaceID != r.reservedSpaceID || purpose != r.purpose || inputHash != r.inputHash || options.ReplacementTargetID != r.targetID || options.ExpectedRevision != r.reserveRevision {
				return media.Reservation{}, false, media.ErrConflict
			}
			return r.reservation, true, nil
		}
		if r.finalizeCalls > 0 && options.ReplacementTargetID == r.targetID && options.ExpectedRevision == r.reserveRevision {
			return media.Reservation{}, false, media.ErrPrecondition
		}
		return media.Reservation{}, false, media.ErrConflict
	}
	r.key = key
	r.inputHash = inputHash
	r.targetID = options.ReplacementTargetID
	r.reserveRevision = options.ExpectedRevision
	r.ownerID = owner
	r.reservedSpaceID = spaceID
	r.purpose = purpose
	r.reservation = media.Reservation{ID: id, OwnerID: owner, SpaceID: spaceID, Purpose: purpose, Token: 7, LeaseTill: leaseTill, State: "pending", InputHash: inputHash, ReplacementTargetID: options.ReplacementTargetID, ExpectedRevision: options.ExpectedRevision}
	return r.reservation, false, nil
}
func (r *replacementRepoStub) PlanVariant(context.Context, string, int64, media.VariantName, string) error {
	r.planCalls++
	return nil
}
func (*replacementRepoStub) FinishVariantWrite(context.Context, string, media.VariantName, string) error {
	return nil
}
func (r *replacementRepoStub) RecordVariant(_ context.Context, _ string, _ int64, variant media.Variant) error {
	r.recordedVariants = append(r.recordedVariants, variant)
	return nil
}
func (r *replacementRepoStub) FinalizeSpace(_ context.Context, id string, _ int64, owner, spaceID, expectedRevision string, at time.Time) (media.Photo, error) {
	r.finalizeCalls++
	r.finalizeRevision = expectedRevision
	r.reservation.State = "ready"
	r.photo = media.Photo{ID: id, SpaceID: spaceID, OwnerID: owner, State: "ready", Position: 1, Visible: true, CreatedAt: at, Variants: append([]media.Variant(nil), r.recordedVariants...)}
	return r.photo, nil
}
func (r *replacementRepoStub) GetSpace(context.Context, string, string, string) (media.Photo, error) {
	r.getCalls++
	return r.photo, nil
}

type pendingUploadRepoStub struct {
	replacementRepoStub
	lookupPhotoID string
	lookupState   string
	lookupErr     error
}

func (*pendingUploadRepoStub) Reserve(_ context.Context, id, owner, spaceID string, purpose media.Purpose, _ string, inputHash string, _ time.Time, leaseUntil time.Time, options media.ReservationOptions) (media.Reservation, bool, error) {
	return media.Reservation{
		ID:                  id,
		OwnerID:             owner,
		SpaceID:             spaceID,
		Purpose:             purpose,
		Token:               1,
		LeaseTill:           leaseUntil,
		State:               "pending",
		InputHash:           inputHash,
		ReplacementTargetID: options.ReplacementTargetID,
		ExpectedRevision:    options.ExpectedRevision,
	}, true, nil
}

func (r *pendingUploadRepoStub) LookupSpaceUpload(_ context.Context, _, _, _ string) (string, string, error) {
	return r.lookupPhotoID, r.lookupState, r.lookupErr
}

type replacementObjectStoreStub struct{ putCalls int }

func (s *replacementObjectStoreStub) Put(_ context.Context, key string, body []byte, _ string) (media.StoredObject, error) {
	s.putCalls++
	digest := sha256.Sum256(body)
	return media.StoredObject{ObjectKey: key, Provider: "memory", Generation: 1, Size: int64(len(body)), SHA256: hex.EncodeToString(digest[:])}, nil
}
func (*replacementObjectStoreStub) Get(context.Context, string, int64) ([]byte, error) {
	return nil, media.ErrNotFound
}
func (*replacementObjectStoreStub) Generation(context.Context, string) (int64, error) {
	return 0, media.ErrNotFound
}
func (*replacementObjectStoreStub) Delete(context.Context, string, int64) error { return nil }

func newReplacementHandlerForTest(t *testing.T, now time.Time) (http.Handler, *replacementRepoStub, *replacementObjectStoreStub) {
	t.Helper()
	spaceRepo := &repoStub{idempotency: map[string]gallery.Photo{}}
	spaceService, err := gallery.NewService(spaceRepo, &filesStub{items: map[string][]byte{}}, idsStub{}, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	mediaRepo := &replacementRepoStub{}
	objectStore := &replacementObjectStoreStub{}
	mediaService, err := media.NewService(mediaRepo, objectStore, replacementIDsStub{}, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	return NewHandlerWithMedia(authStub{}, spaceService, mediaService, nil), mediaRepo, objectStore
}

func TestUploadDuplicatePendingReturnsAcceptedState(t *testing.T) {
	image, err := verification.SyntheticPNG()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 28, 0, 0, 0, 0, time.UTC)
	spaceService, err := gallery.NewService(
		&repoStub{idempotency: map[string]gallery.Photo{}},
		&filesStub{items: map[string][]byte{}},
		idsStub{},
		func() time.Time { return now },
	)
	if err != nil {
		t.Fatal(err)
	}
	mediaService, err := media.NewService(
		&pendingUploadRepoStub{},
		&replacementObjectStoreStub{},
		replacementIDsStub{},
		func() time.Time { return now },
	)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandlerWithMedia(authStub{}, spaceService, mediaService, nil)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("image", "synthetic.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = part.Write(image); err != nil {
		t.Fatal(err)
	}
	if err = writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/spaces/"+testSpace+"/gallery",
		&body,
	)
	request.Header.Set("Authorization", "Bearer synthetic-test-token")
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Idempotency-Key", "pending-upload-key-01")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	var result struct {
		Photo  media.Photo `json:"photo"`
		Reused bool        `json:"reused"`
	}
	if err = json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode pending upload response: %v body=%s", err, response.Body.String())
	}
	if response.Code != http.StatusAccepted || response.Header().Get("Retry-After") != "2" || !result.Reused || result.Photo.State != "pending" {
		t.Fatalf("pending upload status=%d retry-after=%q body=%s", response.Code, response.Header().Get("Retry-After"), response.Body.String())
	}
}

func TestUploadSpaceCreatesReadyPhotoThroughHTTPHandler(t *testing.T) {
	imageBytes, err := verification.SyntheticPNG()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 28, 0, 0, 0, 0, time.UTC)
	handler, repo, objectStore := newReplacementHandlerForTest(t, now)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("image", "synthetic.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(imageBytes); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/spaces/"+testSpace+"/gallery", &body)
	request.Header.Set("Authorization", "Bearer synthetic-test-token")
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Idempotency-Key", "synthetic-initial-upload-01")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	var result struct {
		Photo  media.Photo `json:"photo"`
		Reused bool        `json:"reused"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode upload response: %v body=%s", err, response.Body.String())
	}
	if response.Code != http.StatusCreated || response.Header().Get("Cache-Control") != "no-store" || result.Reused || result.Photo.ID != replacementAssetID || result.Photo.SpaceID != testSpace || result.Photo.State != "ready" || len(result.Photo.Variants) != 2 {
		t.Fatalf("upload status=%d cache=%q reused=%t photo=%+v body=%s", response.Code, response.Header().Get("Cache-Control"), result.Reused, result.Photo, response.Body.String())
	}
	if repo.reserveCalls != 1 || repo.key != "synthetic-initial-upload-01" || repo.purpose != media.PurposeSpace || repo.targetID != "" || repo.reserveRevision != "" || repo.planCalls != 2 || repo.finalizeCalls != 1 || len(repo.recordedVariants) != 2 || objectStore.putCalls != 2 {
		t.Fatalf("upload interactions reserve=%d key=%q purpose=%q target=%q revision=%q plan=%d finalize=%d record=%d puts=%d", repo.reserveCalls, repo.key, repo.purpose, repo.targetID, repo.reserveRevision, repo.planCalls, repo.finalizeCalls, len(repo.recordedVariants), objectStore.putCalls)
	}
	for index, name := range []media.VariantName{media.VariantDisplay, media.VariantThumbnail} {
		variant := result.Photo.Variants[index]
		wantURL := "/api/v1/spaces/" + testSpace + "/gallery/" + replacementAssetID + "/variants/" + string(name)
		if variant.Name != name || variant.MIME != "image/webp" || variant.URL != wantURL {
			t.Errorf("response variant[%d]=%+v, want name=%s MIME=image/webp URL=%s", index, variant, name, wantURL)
		}
	}
}

func TestUploadSpaceMapsGalleryLimitBeforeWritingObjects(t *testing.T) {
	imageBytes, err := verification.SyntheticPNG()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)
	handler, repo, objectStore := newReplacementHandlerForTest(t, now)
	repo.reserveErr = media.ErrLimit

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("image", "synthetic.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(imageBytes); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/spaces/"+testSpace+"/gallery", &body)
	request.Header.Set("Authorization", "Bearer synthetic-test-token")
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Idempotency-Key", "synthetic-gallery-limit-01")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	var result struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode limit response: %v body=%s", err, response.Body.String())
	}
	if response.Code != http.StatusConflict || result.Error.Code != "gallery_limit" {
		t.Fatalf("status=%d code=%q body=%s; want 409 gallery_limit", response.Code, result.Error.Code, response.Body.String())
	}
	if repo.reserveCalls != 1 || repo.planCalls != 0 || repo.finalizeCalls != 0 || objectStore.putCalls != 0 {
		t.Fatalf("limit path reserve=%d plan=%d finalize=%d puts=%d; want reserve only", repo.reserveCalls, repo.planCalls, repo.finalizeCalls, objectStore.putCalls)
	}
}

func TestLookupSpaceUploadStatusReturnsCurrentReservation(t *testing.T) {
	now := time.Date(2026, 10, 28, 0, 0, 0, 0, time.UTC)
	spaceService, err := gallery.NewService(
		&repoStub{idempotency: map[string]gallery.Photo{}},
		&filesStub{items: map[string][]byte{}},
		idsStub{},
		func() time.Time { return now },
	)
	if err != nil {
		t.Fatal(err)
	}
	const photoID = "00000000-0000-4000-8000-000000000098"
	mediaService, err := media.NewService(
		&pendingUploadRepoStub{lookupPhotoID: photoID, lookupState: "ready"},
		&replacementObjectStoreStub{},
		replacementIDsStub{},
		func() time.Time { return now },
	)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandlerWithMedia(authStub{}, spaceService, mediaService, nil)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/spaces/"+testSpace+"/gallery/upload-status", nil)
	request.Header.Set("Authorization", "Bearer synthetic-test-token")
	request.Header.Set("Idempotency-Key", "synthetic-status-key")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	var result struct {
		PhotoID string `json:"photo_id"`
		State   string `json:"state"`
	}
	if err = json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode upload status response: %v body=%s", err, response.Body.String())
	}
	if response.Code != http.StatusOK || result.State != "ready" || result.PhotoID != photoID {
		t.Fatalf("upload status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestReplacementSucceedsAndIdempotentlyReplays(t *testing.T) {
	image, err := verification.SyntheticPNG()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC)
	handler, mediaRepo, objectStore := newReplacementHandlerForTest(t, now)
	timestamp := now.Format(time.RFC3339Nano)
	ifMatch := `"` + base64.RawURLEncoding.EncodeToString([]byte(testSpace+"\x00"+timestamp)) + `"`
	path := "/api/v1/spaces/" + testSpace + "/gallery/" + testPhoto + "/content"
	invoke := func(upload []byte, key string) *httptest.ResponseRecorder {
		t.Helper()
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		part, err := writer.CreateFormFile("image", "synthetic.png")
		if err != nil {
			t.Fatal(err)
		}
		if _, err = part.Write(upload); err != nil {
			t.Fatal(err)
		}
		if err = writer.Close(); err != nil {
			t.Fatal(err)
		}
		request := httptest.NewRequest(http.MethodPut, path, &body)
		request.Header.Set("Authorization", "Bearer synthetic-test-token")
		request.Header.Set("Content-Type", writer.FormDataContentType())
		request.Header.Set("Idempotency-Key", key)
		request.Header.Set("If-Match", ifMatch)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		return response
	}

	first := invoke(image, "replace-photo-key-01")
	var firstBody struct {
		Photo  media.Photo `json:"photo"`
		Reused bool        `json:"reused"`
	}
	if err = json.Unmarshal(first.Body.Bytes(), &firstBody); err != nil {
		t.Fatalf("decode successful replacement: %v body=%s", err, first.Body.String())
	}
	if first.Code != http.StatusCreated || firstBody.Reused || firstBody.Photo.ID != replacementAssetID || first.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("first replacement status=%d body=%s headers=%v", first.Code, first.Body.String(), first.Header())
	}
	if len(firstBody.Photo.Variants) != 2 {
		t.Fatalf("replacement response variants=%+v; want display and thumbnail metadata", firstBody.Photo.Variants)
	}
	seenVariants := map[media.VariantName]bool{}
	for _, variant := range firstBody.Photo.Variants {
		if variant.Name != media.VariantDisplay && variant.Name != media.VariantThumbnail {
			t.Fatalf("unexpected replacement variant: %+v", variant)
		}
		if variant.MIME != "image/webp" || variant.Size <= 0 || variant.SHA256 == "" || variant.Width <= 0 || variant.Height <= 0 {
			t.Fatalf("replacement variant lacks media metadata: %+v", variant)
		}
		seenVariants[variant.Name] = true
	}
	if !seenVariants[media.VariantDisplay] || !seenVariants[media.VariantThumbnail] {
		t.Fatalf("replacement response missing expected variants: %+v", firstBody.Photo.Variants)
	}
	if len(mediaRepo.recordedVariants) != 2 {
		t.Fatalf("repository recorded variants=%+v; want display and thumbnail", mediaRepo.recordedVariants)
	}
	for _, variant := range mediaRepo.recordedVariants {
		if variant.ObjectKey == "" || variant.Provider != "memory" || variant.Generation != 1 || variant.Size <= 0 || variant.SHA256 == "" {
			t.Fatalf("repository received incomplete stored-variant metadata: %+v", variant)
		}
	}
	digest := sha256.Sum256(image)
	wantHash := hex.EncodeToString(digest[:])
	if mediaRepo.reserveCalls != 1 || mediaRepo.key != "replace-photo-key-01" || mediaRepo.inputHash != wantHash || mediaRepo.targetID != testPhoto || mediaRepo.reserveRevision != timestamp || mediaRepo.finalizeRevision != timestamp || mediaRepo.finalizeCalls != 1 {
		t.Fatalf("replacement reservation/finalization mismatch: repo=%+v", mediaRepo)
	}
	writesAfterFirst := objectStore.putCalls
	if writesAfterFirst == 0 || mediaRepo.planCalls != writesAfterFirst {
		t.Fatalf("replacement did not persist planned variants: writes=%d plans=%d", writesAfterFirst, mediaRepo.planCalls)
	}

	second := invoke(image, "replace-photo-key-01")
	var secondBody struct {
		Photo  media.Photo `json:"photo"`
		Reused bool        `json:"reused"`
	}
	if err = json.Unmarshal(second.Body.Bytes(), &secondBody); err != nil {
		t.Fatalf("decode idempotent replay: %v body=%s", err, second.Body.String())
	}
	if second.Code != http.StatusOK || !secondBody.Reused || secondBody.Photo.ID != replacementAssetID || mediaRepo.reserveCalls != 2 || mediaRepo.getCalls != 1 || mediaRepo.finalizeCalls != 1 || objectStore.putCalls != writesAfterFirst {
		t.Fatalf("idempotent replay status=%d body=%s reserve=%d get=%d finalize=%d writes=%d", second.Code, second.Body.String(), mediaRepo.reserveCalls, mediaRepo.getCalls, mediaRepo.finalizeCalls, objectStore.putCalls)
	}

	differentKey := invoke(image, "replace-photo-key-02")
	if differentKey.Code != http.StatusPreconditionFailed || mediaRepo.reserveCalls != 3 || mediaRepo.getCalls != 1 || mediaRepo.finalizeCalls != 1 || objectStore.putCalls != writesAfterFirst {
		t.Fatalf("different idempotency key was treated as replay or wrote objects: status=%d reserve=%d get=%d finalize=%d writes=%d", differentKey.Code, mediaRepo.reserveCalls, mediaRepo.getCalls, mediaRepo.finalizeCalls, objectStore.putCalls)
	}
}

func TestReplacementMapsStaleCASTo412(t *testing.T) {
	image, err := verification.SyntheticPNG()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC)
	handler, mediaRepo, objectStore := newReplacementHandlerForTest(t, now)
	mediaRepo.reserveErr = media.ErrPrecondition
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("image", "synthetic.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = part.Write(image); err != nil {
		t.Fatal(err)
	}
	if err = writer.Close(); err != nil {
		t.Fatal(err)
	}
	timestamp := now.Format(time.RFC3339Nano)
	ifMatch := `"` + base64.RawURLEncoding.EncodeToString([]byte(testSpace+"\x00"+timestamp)) + `"`
	request := httptest.NewRequest(http.MethodPut, "/api/v1/spaces/"+testSpace+"/gallery/"+testPhoto+"/content", &body)
	request.Header.Set("Authorization", "Bearer synthetic-test-token")
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Idempotency-Key", "replace-photo-key-01")
	request.Header.Set("If-Match", ifMatch)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	var responseBody struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err = json.Unmarshal(response.Body.Bytes(), &responseBody); err != nil {
		t.Fatalf("decode stale precondition response: %v body=%s", err, response.Body.String())
	}
	if response.Code != http.StatusPreconditionFailed || responseBody.Error.Code != "precondition_failed" || mediaRepo.reserveCalls != 1 || mediaRepo.finalizeCalls != 0 || objectStore.putCalls != 0 {
		t.Fatalf("stale precondition status=%d body=%s reserve=%d finalize=%d objectWrites=%d", response.Code, response.Body.String(), mediaRepo.reserveCalls, mediaRepo.finalizeCalls, objectStore.putCalls)
	}
}

func TestReplacementRejectsMalformedAndOversizedMultipart(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	now := time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC)
	handler, mediaRepo, objectStore := newReplacementHandlerForTest(t, now)
	timestamp := now.Format(time.RFC3339Nano)
	ifMatch := `"` + base64.RawURLEncoding.EncodeToString([]byte(testSpace+"\x00"+timestamp)) + `"`
	path := "/api/v1/spaces/" + testSpace + "/gallery/" + testPhoto + "/content"
	invoke := func(contentType string, body []byte) *httptest.ResponseRecorder {
		t.Helper()
		request := httptest.NewRequest(http.MethodPut, path, bytes.NewReader(body))
		request.Header.Set("Authorization", "Bearer synthetic-test-token")
		request.Header.Set("Content-Type", contentType)
		request.Header.Set("Idempotency-Key", "replace-photo-key-01")
		request.Header.Set("If-Match", ifMatch)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		return response
	}

	unsupported := invoke("application/json", []byte(`{"image":"not a file"}`))
	if unsupported.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("unsupported content type status=%d body=%s", unsupported.Code, unsupported.Body.String())
	}

	invalidMultipartMediaType := invoke("multipart/form-dataXYZ; boundary=declared-boundary", []byte("not multipart"))
	if invalidMultipartMediaType.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("invalid multipart media type status=%d body=%s", invalidMultipartMediaType.Code, invalidMultipartMediaType.Body.String())
	}

	malformed := invoke("multipart/form-data; boundary=declared-boundary", []byte("--actual-boundary\r\nnot a valid multipart body\r\n--actual-boundary--\r\n"))
	if malformed.Code != http.StatusBadRequest {
		t.Fatalf("malformed multipart status=%d body=%s", malformed.Code, malformed.Body.String())
	}

	var missingImage bytes.Buffer
	missingImageWriter := multipart.NewWriter(&missingImage)
	if err := missingImageWriter.WriteField("other", "not an image"); err != nil {
		t.Fatal(err)
	}
	if err := missingImageWriter.Close(); err != nil {
		t.Fatal(err)
	}
	missing := invoke(missingImageWriter.FormDataContentType(), missingImage.Bytes())
	if missing.Code != http.StatusUnprocessableEntity {
		t.Fatalf("missing image field status=%d body=%s", missing.Code, missing.Body.String())
	}

	var oversizedBody bytes.Buffer
	oversizedWriter := multipart.NewWriter(&oversizedBody)
	largeFile, err := oversizedWriter.CreateFormFile("image", "synthetic.bin")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = largeFile.Write(bytes.Repeat([]byte("x"), media.MaxInputBytes+1)); err != nil {
		t.Fatal(err)
	}
	if err = oversizedWriter.Close(); err != nil {
		t.Fatal(err)
	}
	oversized := invoke(oversizedWriter.FormDataContentType(), oversizedBody.Bytes())
	if oversized.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized upload status=%d body=%s", oversized.Code, oversized.Body.String())
	}
	entries, err := os.ReadDir(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("multipart temporary files remain: %s", filepath.Join(tmp, entries[0].Name()))
	}
	if mediaRepo.reserveCalls != 0 || objectStore.putCalls != 0 {
		t.Fatalf("invalid requests reached media pipeline: reserves=%d writes=%d", mediaRepo.reserveCalls, objectStore.putCalls)
	}
}

func TestUploadRejectsMalformedMultipartAsBadRequest(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	now := time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC)
	handler, mediaRepo, objectStore := newReplacementHandlerForTest(t, now)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/spaces/"+testSpace+"/gallery", strings.NewReader("--actual-boundary\\r\\nnot a valid multipart body\\r\\n--actual-boundary--\\r\\n"))
	request.Header.Set("Authorization", "Bearer synthetic-test-token")
	request.Header.Set("Content-Type", "multipart/form-data; boundary=declared-boundary")
	request.Header.Set("Idempotency-Key", "upload-malformed-key-01")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("malformed multipart status=%d body=%s; want 400", response.Code, response.Body.String())
	}
	if mediaRepo.reserveCalls != 0 || objectStore.putCalls != 0 {
		t.Fatalf("malformed upload reached media pipeline: reserves=%d writes=%d", mediaRepo.reserveCalls, objectStore.putCalls)
	}
	entries, err := os.ReadDir(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("multipart temporary files remain after malformed upload: %s", filepath.Join(tmp, entries[0].Name()))
	}
}

func TestUploadRejectsOversizedMultipartAsRequestEntityTooLarge(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	now := time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC)
	handler, mediaRepo, objectStore := newReplacementHandlerForTest(t, now)
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	file, err := writer.CreateFormFile("image", "synthetic.bin")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = file.Write(bytes.Repeat([]byte("x"), media.MaxInputBytes+(1<<20)+1)); err != nil {
		t.Fatal(err)
	}
	if err = writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/spaces/"+testSpace+"/gallery", &body)
	request.Header.Set("Authorization", "Bearer synthetic-test-token")
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Idempotency-Key", "upload-oversized-key-01")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized multipart status=%d body=%s; want 413", response.Code, response.Body.String())
	}
	if mediaRepo.reserveCalls != 0 || objectStore.putCalls != 0 {
		t.Fatalf("oversized upload reached media pipeline: reserves=%d writes=%d", mediaRepo.reserveCalls, objectStore.putCalls)
	}
	entries, err := os.ReadDir(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("multipart temporary files remain after oversized upload: %s", filepath.Join(tmp, entries[0].Name()))
	}
}

func TestReplacementRequiresStrongIfMatchBeforeReadingUpload(t *testing.T) {
	files := &filesStub{items: map[string][]byte{}}
	repo := &repoStub{idempotency: map[string]gallery.Photo{}}
	svc, err := gallery.NewService(repo, files, idsStub{}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandlerWithMedia(authStub{}, svc, nil, nil)
	path := "/api/v1/spaces/" + testSpace + "/gallery/" + testPhoto + "/content"
	foreignRevision := `"` + base64.RawURLEncoding.EncodeToString([]byte("dddddddd-dddd-4ddd-8ddd-dddddddddddd\x002026-10-27T00:00:00Z")) + `"`

	for _, tc := range []struct {
		name       string
		ifMatch    string
		wantStatus int
		wantCode   string
	}{
		{name: "missing", wantStatus: http.StatusPreconditionRequired, wantCode: "precondition_required"},
		{name: "weak", ifMatch: `W/"tag"`, wantStatus: http.StatusBadRequest, wantCode: "invalid_precondition"},
		{name: "etag for another space", ifMatch: foreignRevision, wantStatus: http.StatusPreconditionFailed, wantCode: "precondition_failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPut, path, nil)
			r.Header.Set("Authorization", "Bearer synthetic-test-token")
			if tc.ifMatch != "" {
				r.Header.Set("If-Match", tc.ifMatch)
			}
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			var body struct {
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode error response: %v body=%s", err, w.Body.String())
			}
			if w.Code != tc.wantStatus || body.Error.Code != tc.wantCode {
				t.Fatalf("status=%d code=%q body=%s; want status=%d code=%q", w.Code, body.Error.Code, w.Body.String(), tc.wantStatus, tc.wantCode)
			}
		})
	}
}

type variantReadRepositoryStub struct {
	media.Repository
	photo          media.Photo
	variant        media.Variant
	getSpaceErr    error
	getVariantErr  error
	getSpaceArgs   []string
	getVariantArgs []string
}

func (r *variantReadRepositoryStub) GetSpace(_ context.Context, owner, spaceID, photoID string) (media.Photo, error) {
	r.getSpaceArgs = []string{owner, spaceID, photoID}
	return r.photo, r.getSpaceErr
}

func (r *variantReadRepositoryStub) GetVariant(_ context.Context, owner, photoID, name string) (media.Variant, error) {
	r.getVariantArgs = []string{owner, photoID, name}
	return r.variant, r.getVariantErr
}

type variantReadObjectStoreStub struct {
	media.ObjectStore
	body       []byte
	objectKey  string
	generation int64
	getCalls   int
}

func (s *variantReadObjectStoreStub) Get(_ context.Context, objectKey string, generation int64) ([]byte, error) {
	s.getCalls++
	s.objectKey = objectKey
	s.generation = generation
	return append([]byte(nil), s.body...), nil
}

func TestVariantRouteServesPrivateMediaVariantAndRejectsUnsupportedName(t *testing.T) {
	body := []byte("synthetic webp variant bytes")
	digest := sha256.Sum256(body)
	checksum := hex.EncodeToString(digest[:])
	const objectKey = "private/synthetic/thumbnail.webp"
	const generation = int64(7)
	mediaRepo := &variantReadRepositoryStub{
		photo: media.Photo{ID: testPhoto, SpaceID: testSpace, OwnerID: testOwner, State: "ready"},
		variant: media.Variant{
			Name: media.VariantThumbnail, MIME: "image/webp", Size: int64(len(body)),
			SHA256: checksum, ObjectKey: objectKey, Generation: generation,
		},
	}
	store := &variantReadObjectStoreStub{body: body}
	mediaService, err := media.NewService(mediaRepo, store, replacementIDsStub{}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	galleryService, err := gallery.NewService(&repoStub{idempotency: map[string]gallery.Photo{}}, &filesStub{items: map[string][]byte{}}, idsStub{}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandlerWithMedia(authStub{}, galleryService, mediaService, nil)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/spaces/"+testSpace+"/gallery/"+testPhoto+"/variants/thumbnail", nil)
	request.Header.Set("Authorization", "Bearer synthetic-test-token")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Body.String() != string(body) {
		t.Fatalf("variant response status=%d body=%q", response.Code, response.Body.String())
	}
	if response.Header().Get("Content-Type") != "image/webp" || response.Header().Get("Cache-Control") != "private, no-store" || response.Header().Get("X-Content-Type-Options") != "nosniff" || response.Header().Get("X-Content-SHA256") != checksum {
		t.Fatalf("variant response headers=%v", response.Header())
	}
	if len(mediaRepo.getSpaceArgs) != 3 || mediaRepo.getSpaceArgs[0] != testOwner || mediaRepo.getSpaceArgs[1] != testSpace || mediaRepo.getSpaceArgs[2] != testPhoto {
		t.Fatalf("space repository scope=%v", mediaRepo.getSpaceArgs)
	}
	if len(mediaRepo.getVariantArgs) != 3 || mediaRepo.getVariantArgs[0] != testOwner || mediaRepo.getVariantArgs[1] != testPhoto || mediaRepo.getVariantArgs[2] != string(media.VariantThumbnail) {
		t.Fatalf("variant repository scope=%v", mediaRepo.getVariantArgs)
	}
	if store.getCalls != 1 || store.objectKey != objectKey || store.generation != generation {
		t.Fatalf("object read calls=%d key=%q generation=%d", store.getCalls, store.objectKey, store.generation)
	}

	unsupported := httptest.NewRequest(http.MethodGet, "/api/v1/spaces/"+testSpace+"/gallery/"+testPhoto+"/variants/avatar", nil)
	unsupported.Header.Set("Authorization", "Bearer synthetic-test-token")
	unsupportedResponse := httptest.NewRecorder()
	handler.ServeHTTP(unsupportedResponse, unsupported)
	if unsupportedResponse.Code != http.StatusNotFound || store.getCalls != 1 {
		t.Fatalf("unsupported variant status=%d object reads=%d", unsupportedResponse.Code, store.getCalls)
	}
}

func TestVariantRouteDoesNotReadObjectWhenSpaceScopedPhotoIsUnavailable(t *testing.T) {
	const objectBody = "private bytes must not be fetched"
	mediaRepo := &variantReadRepositoryStub{getSpaceErr: media.ErrNotFound}
	store := &variantReadObjectStoreStub{body: []byte(objectBody)}
	mediaService, err := media.NewService(mediaRepo, store, replacementIDsStub{}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	galleryService, err := gallery.NewService(&repoStub{idempotency: map[string]gallery.Photo{}}, &filesStub{items: map[string][]byte{}}, idsStub{}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandlerWithMedia(authStub{}, galleryService, mediaService, nil)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/spaces/"+testSpace+"/gallery/"+testPhoto+"/variants/thumbnail", nil)
	request.Header.Set("Authorization", "Bearer synthetic-test-token")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound || !strings.Contains(response.Body.String(), `"code":"not_found"`) {
		t.Fatalf("status=%d body=%s; want 404 not_found", response.Code, response.Body.String())
	}
	if len(mediaRepo.getSpaceArgs) != 3 || mediaRepo.getSpaceArgs[0] != testOwner || mediaRepo.getSpaceArgs[1] != testSpace || mediaRepo.getSpaceArgs[2] != testPhoto {
		t.Fatalf("space repository scope=%v", mediaRepo.getSpaceArgs)
	}
	if len(mediaRepo.getVariantArgs) != 0 || store.getCalls != 0 {
		t.Fatalf("variant lookup args=%v object reads=%d; want neither after scoped photo miss", mediaRepo.getVariantArgs, store.getCalls)
	}
	if strings.Contains(response.Body.String(), objectBody) {
		t.Fatalf("private object bytes leaked in error response: %s", response.Body.String())
	}
}
