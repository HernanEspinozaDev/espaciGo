package galleryhttp

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/gallery"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
)

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

func (r *repoStub) Add(_ context.Context, _, _ string, p gallery.Photo, key string) (gallery.Photo, bool, error) {
	if old, ok := r.idempotency[key]; ok {
		return old, true, nil
	}
	r.idempotency[key] = p
	r.items = append(r.items, p)
	return p, false, nil
}
func (r *repoStub) List(context.Context, string, string) ([]gallery.Photo, error) {
	return append([]gallery.Photo{}, r.items...), nil
}
func (r *repoStub) Get(_ context.Context, _, _, id string) (gallery.Photo, error) {
	for _, p := range r.items {
		if p.ID == id {
			return p, nil
		}
	}
	return gallery.Photo{}, gallery.ErrNotFound
}
func (r *repoStub) Remove(_ context.Context, _, _, id string) (gallery.Removal, error) {
	return gallery.Removal{PhotoID: id, Removed: true}, nil
}
func (r *repoStub) PendingCleanup(context.Context, int) ([]string, error)    { return nil, nil }
func (r *repoStub) CompleteCleanup(context.Context, string, time.Time) error { return nil }
func (r *repoStub) FailCleanup(context.Context, string, time.Time) error     { return nil }

func TestAssembledRouterKeepsSpaceRoutesAndChecksGalleryErrors(t *testing.T) {
	files := &filesStub{items: map[string][]byte{}}
	repo := &repoStub{idempotency: map[string]gallery.Photo{}}
	svc, err := gallery.NewService(repo, files, idsStub{}, func() time.Time { return time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC) })
	if err != nil {
		t.Fatal(err)
	}
	galleryHandler := NewHandler(authStub{}, svc, nil)
	mux := http.NewServeMux()
	mux.Handle("/api/v1/spaces/{spaceID}/gallery", galleryHandler)
	mux.Handle("/api/v1/spaces/{spaceID}/gallery/{photoID}", galleryHandler)
	mux.Handle("/api/v1/spaces/{spaceID}/gallery/{photoID}/content", galleryHandler)
	mux.Handle("/api/v1/spaces/", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(209)
		_, _ = w.Write([]byte("existing space route"))
	}))
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
	contentRequest := httptest.NewRequest(http.MethodGet, "/api/v1/spaces/"+testSpace+"/gallery/"+testPhoto+"/content", nil)
	contentRequest.Header.Set("Authorization", "Bearer synthetic-test-token")
	content := httptest.NewRecorder()
	mux.ServeHTTP(content, contentRequest)
	if content.Code != 200 || content.Header().Get("Content-Type") != "image/png" || !bytes.HasPrefix(content.Body.Bytes(), []byte("\x89PNG\r\n\x1a\n")) {
		t.Fatalf("private PNG content status=%d type=%s", content.Code, content.Header().Get("Content-Type"))
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
