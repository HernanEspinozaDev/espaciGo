package mediapg

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/credentials"
	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/mediafs"
	gallerypg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/gallery"
	"github.com/HernanEspinozaDev/espaciGo/internal/gallery"
	galleryhttp "github.com/HernanEspinozaDev/espaciGo/internal/gallery/transport/http"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
	"github.com/HernanEspinozaDev/espaciGo/internal/media"
	"github.com/HernanEspinozaDev/espaciGo/internal/verification"
	"github.com/jackc/pgx/v5/pgxpool"
)

const img01fHTTPStorageRoot = "/home/nandev/.hermes/cache/scratch/f6-espaciago-20261028/media"

type img01fHTTPAuthenticator struct{ owner string }

func (a img01fHTTPAuthenticator) Authorize(_ context.Context, _ identity.Secret, role identity.Role, _ identity.ActivityKind) (identity.Principal, error) {
	if role != identity.RoleLandlord {
		return identity.Principal{}, fmt.Errorf("unexpected test authorization role %q", role)
	}
	return identity.Principal{AccountID: a.owner, Roles: []identity.Role{identity.RoleLandlord}}, nil
}

type img01fHTTPGalleryFiles struct{}

func (img01fHTTPGalleryFiles) Put(context.Context, string, []byte) error { return nil }
func (img01fHTTPGalleryFiles) Get(context.Context, string) ([]byte, error) {
	return nil, fs.ErrNotExist
}
func (img01fHTTPGalleryFiles) Delete(context.Context, string) error { return nil }

func img01fHTTPETag(spaceID string, revision time.Time) string {
	payload := spaceID + "\x00" + revision.UTC().Format(time.RFC3339Nano)
	return `"` + base64.RawURLEncoding.EncodeToString([]byte(payload)) + `"`
}

func img01fExerciseHTTPReplacement(t *testing.T, ctx context.Context, pool *pgxpool.Pool, owner, foreignOwner, spaceID, targetID, idempotencyKey string, expected time.Time) string {
	t.Helper()
	storageRoot := filepath.Clean(strings.TrimSpace(os.Getenv("IMG01F_TEST_STORAGE_ROOT")))
	if storageRoot != img01fHTTPStorageRoot {
		t.Fatal("IMG01F_TEST_STORAGE_ROOT must be the authorized private scratch media root")
	}
	rootInfo, err := os.Lstat(storageRoot)
	if err != nil || !rootInfo.IsDir() || rootInfo.Mode()&os.ModeSymlink != 0 || rootInfo.Mode().Perm()&0o077 != 0 {
		t.Fatalf("authorized storage root must be a private, non-symlink directory: %v", err)
	}
	resolvedRoot, err := filepath.EvalSymlinks(storageRoot)
	if err != nil || resolvedRoot != storageRoot {
		t.Fatal("authorized storage root must resolve to itself")
	}
	storageDir, err := os.MkdirTemp(storageRoot, "img01f-http-")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(storageDir, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(storageDir); err != nil {
			t.Errorf("remove exact synthetic media test directory: %v", err)
		}
	})
	store, err := mediafs.New(storageDir)
	if err != nil {
		t.Fatal(err)
	}
	galleryService, err := gallery.NewService(gallerypg.New(pool), img01fHTTPGalleryFiles{}, credentials.Generator{}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	mediaService, err := media.NewService(New(pool), store, credentials.Generator{}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	handler := galleryhttp.NewHandlerWithMedia(img01fHTTPAuthenticator{owner: owner}, galleryService, mediaService, []string{"http://127.0.0.1:4205"})
	imageBytes, err := verification.SyntheticPNG()
	if err != nil {
		t.Fatal(err)
	}
	request := func(revision time.Time) *httptest.ResponseRecorder {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		part, err := writer.CreateFormFile("image", "synthetic-replacement.png")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write(imageBytes); err != nil {
			t.Fatal(err)
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodPut, "/api/v1/spaces/"+spaceID+"/gallery/"+targetID+"/content", &body)
		req.Header.Set("Authorization", "Bearer synthetic-http-test-token")
		req.Header.Set("Origin", "http://127.0.0.1:4205")
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("Idempotency-Key", idempotencyKey)
		req.Header.Set("If-Match", img01fHTTPETag(spaceID, revision))
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		return response
	}

	created := request(expected)
	if created.Code != http.StatusCreated {
		t.Fatalf("replacement HTTP status=%d body=%s", created.Code, created.Body.String())
	}
	var result struct {
		Photo  media.Photo `json:"photo"`
		Reused bool        `json:"reused"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	newPhotoID := result.Photo.ID
	if newPhotoID == "" || newPhotoID == targetID || result.Photo.State != "ready" || result.Photo.Position != 1 || !result.Photo.Cover || result.Photo.AltText != "Inherited synthetic alt" || result.Reused {
		t.Fatalf("replacement response=%+v reused=%t", result.Photo, result.Reused)
	}

	var assetState, storedTarget string
	var storedRevision time.Time
	if err := pool.QueryRow(ctx, `SELECT estado,replacement_target_space_media_id::text,replacement_expected_revision
		FROM public.media_asset WHERE id=$1`, newPhotoID).Scan(&assetState, &storedTarget, &storedRevision); err != nil {
		t.Fatal(err)
	}
	if assetState != "ready" || storedTarget != targetID || storedRevision.UTC().Format(time.RFC3339Nano) != expected.UTC().Format(time.RFC3339Nano) {
		t.Fatalf("stored replacement asset state=%s target=%s revision=%s", assetState, storedTarget, storedRevision.UTC().Format(time.RFC3339Nano))
	}
	var currentCover, orderedSlot string
	var position int
	if err := pool.QueryRow(ctx, `SELECT e.portada_media_id::text,o.space_media_id::text,o.position
		FROM public.espacio e JOIN public.gallery_item_order o ON o.espacio_id=e.id WHERE e.id=$1`, spaceID).Scan(&currentCover, &orderedSlot, &position); err != nil {
		t.Fatal(err)
	}
	if currentCover != newPhotoID || orderedSlot != newPhotoID || position != 1 {
		t.Fatalf("replacement slot/cover=%s/%s position=%d", orderedSlot, currentCover, position)
	}

	get := httptest.NewRequest(http.MethodGet, "/api/v1/spaces/"+spaceID+"/gallery/"+newPhotoID+"/variants/display", nil)
	get.Header.Set("Authorization", "Bearer synthetic-http-test-token")
	get.Header.Set("Origin", "http://127.0.0.1:4205")
	content := httptest.NewRecorder()
	handler.ServeHTTP(content, get)
	if content.Code != http.StatusOK || content.Header().Get("Content-Type") != "image/webp" || content.Header().Get("Cache-Control") != "private, no-store" || len(content.Body.Bytes()) < 12 || string(content.Body.Bytes()[0:4]) != "RIFF" || string(content.Body.Bytes()[8:12]) != "WEBP" {
		t.Fatalf("filesystem-backed variant response status=%d type=%s cache=%s bytes=%d", content.Code, content.Header().Get("Content-Type"), content.Header().Get("Cache-Control"), content.Body.Len())
	}
	digest := sha256.Sum256(content.Body.Bytes())
	if content.Header().Get("X-Content-SHA256") != fmt.Sprintf("%x", digest) {
		t.Fatal("filesystem-backed variant digest header does not match returned bytes")
	}
	foreignHandler := galleryhttp.NewHandlerWithMedia(img01fHTTPAuthenticator{owner: foreignOwner}, galleryService, mediaService, []string{"http://127.0.0.1:4205"})
	foreignGet := httptest.NewRequest(http.MethodGet, "/api/v1/spaces/"+spaceID+"/gallery/"+newPhotoID+"/variants/display", nil)
	foreignGet.Header.Set("Authorization", "Bearer synthetic-http-test-token")
	foreignGet.Header.Set("Origin", "http://127.0.0.1:4205")
	foreignResponse := httptest.NewRecorder()
	foreignHandler.ServeHTTP(foreignResponse, foreignGet)
	if foreignResponse.Code != http.StatusNotFound {
		t.Fatalf("foreign-owner private variant status=%d, want not found", foreignResponse.Code)
	}

	restartProbe := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestSpaceMediaVariantAfterProcessRestart$", "-test.v")
	restartProbe.Env = append(os.Environ(),
		"IMG01F_RESTART_CHECK=1",
		"IMG01F_RESTART_STORAGE_DIR="+storageDir,
		"IMG01F_RESTART_OWNER_ID="+owner,
		"IMG01F_RESTART_SPACE_ID="+spaceID,
		"IMG01F_RESTART_PHOTO_ID="+newPhotoID,
	)
	restartOutput, restartErr := restartProbe.CombinedOutput()
	if restartErr != nil || !strings.Contains(string(restartOutput), "--- PASS: TestSpaceMediaVariantAfterProcessRestart") {
		t.Fatalf("private variant was not rehydrated by a fresh process: %v", restartErr)
	}

	replayed := request(expected)
	if replayed.Code != http.StatusOK {
		t.Fatalf("same-revision ready replay status=%d body=%s", replayed.Code, replayed.Body.String())
	}
	var replayResult struct {
		Photo  media.Photo `json:"photo"`
		Reused bool        `json:"reused"`
	}
	if err := json.Unmarshal(replayed.Body.Bytes(), &replayResult); err != nil || !replayResult.Reused || replayResult.Photo.ID != newPhotoID {
		t.Fatalf("same-revision ready replay photo=%+v reused=%t err=%v", replayResult.Photo, replayResult.Reused, err)
	}
	changedRevision := request(expected.Add(time.Second))
	if changedRevision.Code != http.StatusConflict {
		t.Fatalf("different-revision replay status=%d body=%s, want conflict", changedRevision.Code, changedRevision.Body.String())
	}

	statusRequest := httptest.NewRequest(http.MethodGet, "/api/v1/spaces/"+spaceID+"/gallery/upload-status", nil)
	statusRequest.Header.Set("Authorization", "Bearer synthetic-http-test-token")
	statusRequest.Header.Set("Origin", "http://127.0.0.1:4205")
	statusRequest.Header.Set("Idempotency-Key", idempotencyKey)
	statusResponse := httptest.NewRecorder()
	handler.ServeHTTP(statusResponse, statusRequest)
	var uploadStatus struct {
		PhotoID string `json:"photo_id"`
		State   string `json:"state"`
	}
	if err := json.Unmarshal(statusResponse.Body.Bytes(), &uploadStatus); err != nil || statusResponse.Code != http.StatusOK || uploadStatus.PhotoID != newPhotoID || uploadStatus.State != "ready" {
		t.Fatalf("authoritative upload status=%d body=%s parsed=%+v err=%v", statusResponse.Code, statusResponse.Body.String(), uploadStatus, err)
	}

	fileCount := 0
	if err := filepath.WalkDir(storageDir, func(_ string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			fileCount++
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if fileCount != 2 {
		t.Fatalf("filesystem object count=%d, want two sanitized variants", fileCount)
	}
	return newPhotoID
}

func img01fExerciseHTTPGalleryMutation(t *testing.T, ctx context.Context, pool *pgxpool.Pool, owner, foreignOwner, spaceID, idempotencyKey string) {
	t.Helper()
	storageRoot := filepath.Clean(strings.TrimSpace(os.Getenv("IMG01F_TEST_STORAGE_ROOT")))
	if storageRoot != img01fHTTPStorageRoot {
		t.Fatal("IMG01F_TEST_STORAGE_ROOT must be the authorized private scratch media root")
	}
	rootInfo, err := os.Lstat(storageRoot)
	if err != nil || !rootInfo.IsDir() || rootInfo.Mode()&os.ModeSymlink != 0 || rootInfo.Mode().Perm()&0o077 != 0 {
		t.Fatalf("authorized storage root must be a private, non-symlink directory: %v", err)
	}
	resolvedRoot, err := filepath.EvalSymlinks(storageRoot)
	if err != nil || resolvedRoot != storageRoot {
		t.Fatal("authorized storage root must resolve to itself")
	}
	storageDir, err := os.MkdirTemp(storageRoot, "img01f-gallery-http-")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(storageDir, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(storageDir); err != nil {
			t.Errorf("remove exact synthetic gallery storage directory: %v", err)
		}
	})
	store, err := mediafs.New(storageDir)
	if err != nil {
		t.Fatal(err)
	}
	galleryService, err := gallery.NewService(gallerypg.New(pool), img01fHTTPGalleryFiles{}, credentials.Generator{}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	mediaService, err := media.NewService(New(pool), store, credentials.Generator{}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	handler := galleryhttp.NewHandlerWithMedia(img01fHTTPAuthenticator{owner: owner}, galleryService, mediaService, []string{"http://127.0.0.1:4205"})
	imageBytes, err := verification.SyntheticPNG()
	if err != nil {
		t.Fatal(err)
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("image", "synthetic-gallery-upload.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(imageBytes); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/spaces/"+spaceID+"/gallery", &body)
	req.Header.Set("Authorization", "Bearer synthetic-http-test-token")
	req.Header.Set("Origin", "http://127.0.0.1:4205")
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Idempotency-Key", idempotencyKey)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	if response.Code != http.StatusCreated {
		t.Fatalf("gallery upload HTTP status=%d body=%s", response.Code, response.Body.String())
	}
	etag := response.Header().Get("ETag")
	if etag == "" {
		t.Fatal("successful gallery upload omitted the updated space ETag required for the next CAS mutation")
	}
	var result struct {
		Photo media.Photo `json:"photo"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Photo.ID == "" || result.Photo.State != "ready" || result.Photo.Position != 2 || result.Photo.Cover {
		t.Fatalf("gallery upload response state=%s position=%d cover=%t", result.Photo.State, result.Photo.Position, result.Photo.Cover)
	}
	list := func(h http.Handler) []struct {
		ID       string `json:"id"`
		Position int    `json:"position"`
		Cover    bool   `json:"cover"`
	} {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/spaces/"+spaceID+"/gallery", nil)
		req.Header.Set("Authorization", "Bearer synthetic-http-test-token")
		req.Header.Set("Origin", "http://127.0.0.1:4205")
		response := httptest.NewRecorder()
		h.ServeHTTP(response, req)
		var body struct {
			Items []struct {
				ID       string `json:"id"`
				Position int    `json:"position"`
				Cover    bool   `json:"cover"`
			} `json:"items"`
		}
		if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &body) != nil {
			t.Fatalf("gallery list HTTP status=%d", response.Code)
		}
		return body.Items
	}
	items := list(handler)
	if len(items) != 2 {
		t.Fatalf("gallery list count=%d, want two active photos", len(items))
	}
	if items[0].Position != 1 || items[1].Position != 2 || !items[0].Cover || items[1].Cover || items[1].ID != result.Photo.ID {
		t.Fatalf("gallery list positions=%d,%d covers=%t,%t new_photo_at_position_2=%t", items[0].Position, items[1].Position, items[0].Cover, items[1].Cover, items[1].ID == result.Photo.ID)
	}
	foreignHandler := galleryhttp.NewHandlerWithMedia(img01fHTTPAuthenticator{owner: foreignOwner}, galleryService, mediaService, []string{"http://127.0.0.1:4205"})
	foreignList := httptest.NewRecorder()
	foreignRequest := httptest.NewRequest(http.MethodGet, "/api/v1/spaces/"+spaceID+"/gallery", nil)
	foreignRequest.Header.Set("Authorization", "Bearer synthetic-http-test-token")
	foreignRequest.Header.Set("Origin", "http://127.0.0.1:4205")
	foreignHandler.ServeHTTP(foreignList, foreignRequest)
	if foreignList.Code != http.StatusNotFound {
		t.Fatalf("foreign-owner gallery list status=%d, want not found", foreignList.Code)
	}
	coverBody, err := json.Marshal(map[string]string{"photo_id": result.Photo.ID})
	if err != nil {
		t.Fatal(err)
	}
	coverRequest := httptest.NewRequest(http.MethodPut, "/api/v1/spaces/"+spaceID+"/cover", bytes.NewReader(coverBody))
	coverRequest.Header.Set("Authorization", "Bearer synthetic-http-test-token")
	coverRequest.Header.Set("Origin", "http://127.0.0.1:4205")
	coverRequest.Header.Set("Content-Type", "application/json")
	coverRequest.Header.Set("If-Match", etag)
	coverResponse := httptest.NewRecorder()
	handler.ServeHTTP(coverResponse, coverRequest)
	if coverResponse.Code != http.StatusOK || coverResponse.Header().Get("ETag") == "" || coverResponse.Header().Get("ETag") == etag {
		t.Fatalf("select gallery cover status=%d renewed_etag=%t", coverResponse.Code, coverResponse.Header().Get("ETag") != "" && coverResponse.Header().Get("ETag") != etag)
	}
	var selected gallery.Cover
	if err := json.Unmarshal(coverResponse.Body.Bytes(), &selected); err != nil || selected.PhotoID != result.Photo.ID || !selected.Selected {
		t.Fatalf("cover selection response selected=%t err=%v", selected.Selected, err)
	}
	var persistedCover string
	if err := pool.QueryRow(ctx, `SELECT portada_media_id::text FROM public.espacio WHERE id=$1`, spaceID).Scan(&persistedCover); err != nil || persistedCover != result.Photo.ID {
		t.Fatalf("persisted cover matches uploaded photo=%t err=%v", persistedCover == result.Photo.ID, err)
	}
	items = list(handler)
	if len(items) != 2 {
		t.Fatalf("gallery list after cover selection count=%d, want two active photos", len(items))
	}
	if items[0].Cover || !items[1].Cover || items[1].ID != result.Photo.ID {
		t.Fatalf("gallery list did not rehydrate selected cover: selected_cover=%t", items[1].Cover && items[1].ID == result.Photo.ID)
	}
	etag = coverResponse.Header().Get("ETag")
	positionBody, err := json.Marshal(map[string]int{"position": 1})
	if err != nil {
		t.Fatal(err)
	}
	moveRequest := httptest.NewRequest(http.MethodPut, "/api/v1/spaces/"+spaceID+"/gallery/"+result.Photo.ID, bytes.NewReader(positionBody))
	moveRequest.Header.Set("Authorization", "Bearer synthetic-http-test-token")
	moveRequest.Header.Set("Origin", "http://127.0.0.1:4205")
	moveRequest.Header.Set("Content-Type", "application/json")
	moveRequest.Header.Set("If-Match", etag)
	moveResponse := httptest.NewRecorder()
	handler.ServeHTTP(moveResponse, moveRequest)
	if moveResponse.Code != http.StatusOK || moveResponse.Header().Get("ETag") == "" || moveResponse.Header().Get("ETag") == etag {
		t.Fatalf("reorder gallery photo status=%d renewed_etag=%t", moveResponse.Code, moveResponse.Header().Get("ETag") != "" && moveResponse.Header().Get("ETag") != etag)
	}
	var moved media.Photo
	if err := json.Unmarshal(moveResponse.Body.Bytes(), &moved); err != nil || moved.ID != result.Photo.ID || moved.Position != 1 || !moved.Cover {
		t.Fatalf("reordered photo position=%d cover=%t err=%v", moved.Position, moved.Cover, err)
	}
	items = list(handler)
	if len(items) != 2 {
		t.Fatalf("gallery list after reorder count=%d, want two active photos", len(items))
	}
	if items[0].ID != result.Photo.ID || items[0].Position != 1 || !items[0].Cover || items[1].Position != 2 || items[1].Cover {
		t.Fatalf("gallery order/cover after reorder correct=%t", items[0].ID == result.Photo.ID && items[0].Position == 1 && items[0].Cover && items[1].Position == 2 && !items[1].Cover)
	}
	photoBeforeRemoval, err := mediaService.GetSpace(ctx, owner, spaceID, result.Photo.ID)
	if err != nil {
		t.Fatalf("load media before logical removal: %v", err)
	}
	var display media.Variant
	for _, variant := range photoBeforeRemoval.Variants {
		if variant.Name == media.VariantDisplay {
			display = variant
			break
		}
	}
	if display.ObjectKey == "" || display.Generation == 0 {
		t.Fatal("uploaded display variant metadata missing before retirement")
	}
	removeRequest := httptest.NewRequest(http.MethodDelete, "/api/v1/spaces/"+spaceID+"/gallery/"+result.Photo.ID, nil)
	removeRequest.Header.Set("Authorization", "Bearer synthetic-http-test-token")
	removeRequest.Header.Set("Origin", "http://127.0.0.1:4205")
	removeRequest.Header.Set("If-Match", moveResponse.Header().Get("ETag"))
	removeResponse := httptest.NewRecorder()
	handler.ServeHTTP(removeResponse, removeRequest)
	if removeResponse.Code != http.StatusOK || removeResponse.Header().Get("ETag") == "" {
		t.Fatalf("logical photo retirement status=%d has_etag=%t", removeResponse.Code, removeResponse.Header().Get("ETag") != "")
	}
	var removal gallery.Removal
	if err := json.Unmarshal(removeResponse.Body.Bytes(), &removal); err != nil || !removal.Removed || removal.PhotoID != result.Photo.ID {
		t.Fatalf("logical removal response removed=%t err=%v", removal.Removed, err)
	}
	var retired, visible bool
	var assetState string
	if err := pool.QueryRow(ctx, `SELECT sm.retired_at IS NOT NULL,sm.visible,a.estado FROM public.space_media sm JOIN public.media_asset a ON a.id=sm.media_id WHERE sm.id=$1`, result.Photo.ID).Scan(&retired, &visible, &assetState); err != nil || !retired || visible || assetState != "deleting" {
		t.Fatalf("retirement persisted=%t visible=%t asset_state=%s err=%v", retired, visible, assetState, err)
	}
	storedBytes, err := store.Get(ctx, display.ObjectKey, display.Generation)
	if err != nil || int64(len(storedBytes)) != display.Size || fmt.Sprintf("%x", sha256.Sum256(storedBytes)) != display.SHA256 {
		t.Fatalf("logical retirement preserves private object bytes=%t err=%v", err == nil && len(storedBytes) > 0, err)
	}
	retiredContent := httptest.NewRecorder()
	retiredRequest := httptest.NewRequest(http.MethodGet, "/api/v1/spaces/"+spaceID+"/gallery/"+result.Photo.ID+"/variants/display", nil)
	retiredRequest.Header.Set("Authorization", "Bearer synthetic-http-test-token")
	retiredRequest.Header.Set("Origin", "http://127.0.0.1:4205")
	handler.ServeHTTP(retiredContent, retiredRequest)
	if retiredContent.Code != http.StatusNotFound {
		t.Fatalf("retired private variant status=%d, want not found", retiredContent.Code)
	}
	items = list(handler)
	if len(items) != 1 || items[0].Position != 1 || items[0].Cover {
		t.Fatalf("gallery after retirement count=%d remaining_position=%d remaining_cover=%t", len(items), func() int {
			if len(items) == 0 {
				return 0
			}
			return items[0].Position
		}(), len(items) == 1 && items[0].Cover)
	}
}

func TestSpaceMediaVariantAfterProcessRestart(t *testing.T) {
	if os.Getenv("IMG01F_RESTART_CHECK") != "1" {
		t.Skip("child-process probe for persisted private media")
	}
	cfg, err := pgxpool.ParseConfig(os.Getenv("IMG01F_TEST_DATABASE_URL"))
	if err != nil || !img01fTargetAllowed(cfg.ConnConfig.Host, cfg.ConnConfig.Port, cfg.ConnConfig.Database, cfg.ConnConfig.User) {
		t.Fatal("restart probe must use the authorized isolated PostgreSQL target")
	}
	storageRoot := filepath.Clean(strings.TrimSpace(os.Getenv("IMG01F_TEST_STORAGE_ROOT")))
	storageDir := filepath.Clean(strings.TrimSpace(os.Getenv("IMG01F_RESTART_STORAGE_DIR")))
	if storageRoot != img01fHTTPStorageRoot || filepath.Dir(storageDir) != storageRoot {
		t.Fatal("restart probe storage must remain inside the authorized private scratch root")
	}
	rootInfo, rootErr := os.Lstat(storageRoot)
	storeInfo, storeErr := os.Lstat(storageDir)
	resolved, resolveErr := filepath.EvalSymlinks(storageDir)
	if rootErr != nil || storeErr != nil || resolveErr != nil || !rootInfo.IsDir() || !storeInfo.IsDir() || rootInfo.Mode()&os.ModeSymlink != 0 || storeInfo.Mode()&os.ModeSymlink != 0 || rootInfo.Mode().Perm()&0o077 != 0 || storeInfo.Mode().Perm()&0o077 != 0 || resolved != storageDir {
		t.Fatal("restart probe requires existing private non-symlink storage")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if cfg.ConnConfig.RuntimeParams == nil {
		cfg.ConnConfig.RuntimeParams = make(map[string]string)
	}
	cfg.ConnConfig.RuntimeParams["application_name"] = fmt.Sprintf("img01f-restart-%d", os.Getpid())
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal("restart probe could not connect to the authorized PostgreSQL target")
	}
	defer pool.Close()
	if err = pool.Ping(ctx); err != nil {
		t.Fatal("restart probe could not rehydrate media metadata")
	}
	store, err := mediafs.New(storageDir)
	if err != nil {
		t.Fatal("restart probe could not reopen private storage")
	}
	galleryService, err := gallery.NewService(gallerypg.New(pool), img01fHTTPGalleryFiles{}, credentials.Generator{}, time.Now)
	if err != nil {
		t.Fatal("restart probe could not initialize gallery service")
	}
	mediaService, err := media.NewService(New(pool), store, credentials.Generator{}, time.Now)
	if err != nil {
		t.Fatal("restart probe could not initialize media service")
	}
	handler := galleryhttp.NewHandlerWithMedia(img01fHTTPAuthenticator{owner: os.Getenv("IMG01F_RESTART_OWNER_ID")}, galleryService, mediaService, []string{"http://127.0.0.1:4205"})
	server := httptest.NewServer(handler)
	defer server.Close()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/api/v1/spaces/"+os.Getenv("IMG01F_RESTART_SPACE_ID")+"/gallery/"+os.Getenv("IMG01F_RESTART_PHOTO_ID")+"/variants/display", nil)
	if err != nil {
		t.Fatal("restart probe could not construct private content request")
	}
	request.Header.Set("Authorization", "Bearer synthetic-http-test-token")
	request.Header.Set("Origin", "http://127.0.0.1:4205")
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatal("fresh HTTP server could not read the persisted private variant")
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal("fresh HTTP server returned an unreadable private variant")
	}
	digest := sha256.Sum256(body)
	if response.StatusCode != http.StatusOK || response.Header.Get("Content-Type") != "image/webp" || response.Header.Get("Cache-Control") != "private, no-store" || response.Header.Get("X-Content-SHA256") != fmt.Sprintf("%x", digest) || len(body) < 12 || string(body[0:4]) != "RIFF" || string(body[8:12]) != "WEBP" {
		t.Fatal("fresh process could not rehydrate the persisted private WebP variant")
	}
}

func img01fAssertReserveFinalizeRevisionRace(t *testing.T, ctx context.Context, pool *pgxpool.Pool, repo *Repository, owner, spaceID, targetID, idempotencyKey, candidateID, appName string, now, lease time.Time) {
	t.Helper()
	var revision time.Time
	if err := pool.QueryRow(ctx, `SELECT actualizado_en FROM public.espacio WHERE id=$1 AND propietario_id=$2`, spaceID, owner).Scan(&revision); err != nil {
		t.Fatal(err)
	}
	expected := revision.UTC().Format(time.RFC3339Nano)
	reservation, reused, err := repo.Reserve(ctx, candidateID, owner, spaceID, media.PurposeSpace, idempotencyKey, fmt.Sprintf("%064d", 77), now, lease, media.ReservationOptions{ReplacementTargetID: targetID, ExpectedRevision: expected})
	if err != nil || reused {
		t.Fatalf("reserve race candidate=%+v reused=%t err=%v", reservation, reused, err)
	}
	v1Key := fmt.Sprintf("media/v1/spaces/%s/%s/%d/display.webp", spaceID, candidateID, reservation.Token)
	v2Key := fmt.Sprintf("users/%s/spaces/%s/images/v2/%s/%s_%d_thumbnail.webp", owner, spaceID, candidateID, now.UTC().Format("20060102T150405.000000000Z"), reservation.Token)
	for index, variant := range []struct{ name, key string }{{"display", v1Key}, {"thumbnail", v2Key}} {
		if _, err := pool.Exec(ctx, `INSERT INTO public.media_variant_intent(media_id,variant,object_key) VALUES($1,$2,$3)`, candidateID, variant.name, variant.key); err != nil {
			t.Fatalf("record race candidate %s intent: %v", variant.name, err)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO public.media_variant(media_id,variant,provider,bucket_name,object_key,generation,mime_type,size_bytes,sha256,width,height) VALUES($1,$2,'gcs','synthetic-bucket',$3,$4,'image/webp',128,repeat('d',64),320,240)`, candidateID, variant.name, variant.key, index+1); err != nil {
			t.Fatalf("record race candidate %s variant: %v", variant.name, err)
		}
	}

	blocker, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = blocker.Rollback(context.Background()) }()
	if _, err := blocker.Exec(ctx, `UPDATE public.espacio SET actualizado_en=actualizado_en+interval '1 second' WHERE id=$1 AND propietario_id=$2`, spaceID, owner); err != nil {
		t.Fatal(err)
	}
	finalizeCtx, cancelFinalize := context.WithTimeout(ctx, 10*time.Second)
	defer cancelFinalize()
	finalized := make(chan error, 1)
	go func() {
		_, finalizeErr := repo.FinalizeSpace(finalizeCtx, candidateID, reservation.Token, owner, spaceID, expected, time.Now().UTC())
		finalized <- finalizeErr
	}()
	blocked := false
	blockDeadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(blockDeadline) {
		var waiting bool
		if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE application_name=$1 AND state='active' AND wait_event_type='Lock' AND position('SELECT actualizado_en FROM public.espacio' in query)>0)`, appName).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting {
			blocked = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !blocked {
		cancelFinalize()
		_ = blocker.Rollback(context.Background())
		select {
		case finalizeErr := <-finalized:
			t.Fatalf("FinalizeSpace did not wait on the concurrent space revision lock; result=%v", finalizeErr)
		case <-time.After(5 * time.Second):
			t.Fatal("FinalizeSpace returned neither a lock wait nor a result")
		}
	}
	if err := blocker.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case finalizeErr := <-finalized:
		if !errors.Is(finalizeErr, media.ErrPrecondition) {
			t.Fatalf("FinalizeSpace after concurrent revision change=%v, want ErrPrecondition", finalizeErr)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("FinalizeSpace did not return after releasing the controlled row lock")
	}

	var candidateState string
	if err := pool.QueryRow(ctx, `SELECT estado FROM public.media_asset WHERE id=$1`, candidateID).Scan(&candidateState); err != nil {
		t.Fatal(err)
	}
	var candidateSlots int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM public.space_media WHERE id=$1`, candidateID).Scan(&candidateSlots); err != nil {
		t.Fatal(err)
	}
	var targetState string
	var targetRetired *time.Time
	var targetVisible bool
	if err := pool.QueryRow(ctx, `SELECT a.estado,s.retired_at,s.visible FROM public.media_asset a JOIN public.space_media s ON s.media_id=a.id WHERE a.id=$1`, targetID).Scan(&targetState, &targetRetired, &targetVisible); err != nil {
		t.Fatal(err)
	}
	var cover, ordered string
	var position int
	if err := pool.QueryRow(ctx, `SELECT e.portada_media_id::text,o.space_media_id::text,o.position FROM public.espacio e JOIN public.gallery_item_order o ON o.espacio_id=e.id WHERE e.id=$1`, spaceID).Scan(&cover, &ordered, &position); err != nil {
		t.Fatal(err)
	}
	var after time.Time
	if err := pool.QueryRow(ctx, `SELECT actualizado_en FROM public.espacio WHERE id=$1`, spaceID).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if candidateState != "pending" || candidateSlots != 0 || targetState != "ready" || targetRetired != nil || !targetVisible || cover != targetID || ordered != targetID || position != 1 || !after.After(revision) {
		t.Fatalf("concurrent CAS left partial gallery state: candidate=%s slots=%d target=%s retired=%v visible=%t cover=%s order=%s/%d revision=%s->%s", candidateState, candidateSlots, targetState, targetRetired, targetVisible, cover, ordered, position, revision, after)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM public.media_variant_intent WHERE media_id=$1`, candidateID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM public.media_variant WHERE media_id=$1`, candidateID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM public.media_asset WHERE id=$1`, candidateID); err != nil {
		t.Fatal(err)
	}
}
