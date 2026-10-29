package mediapg

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
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

func img01fExerciseHTTPReplacement(t *testing.T, ctx context.Context, pool *pgxpool.Pool, owner, spaceID, targetID, idempotencyKey string, expected time.Time) string {
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
