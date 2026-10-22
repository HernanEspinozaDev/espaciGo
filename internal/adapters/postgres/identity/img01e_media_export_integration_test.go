package postgres_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"strings"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/credentials"
	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/mediafs"
	identitypg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/identity"
	mediapg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/media"
	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/ownerexport"
	"github.com/HernanEspinozaDev/espaciGo/internal/media"
	"github.com/HernanEspinozaDev/espaciGo/internal/privacy"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestIMG01EProfileMediaUploadExportAndRevocation(t *testing.T) {
	h := newAuthHarness(t)
	runtimePool := newRuntimePool(t, h)
	runtimePool.Close()
	assertIMG01ERuntimeMediaPrivileges(t, h)
	owner := h.register(t, "img01e-media-export@ejemplo.invalid")
	h.verify(t)

	store, err := mediafs.New(t.TempDir() + "/private-media")
	if err != nil {
		t.Fatal(err)
	}
	service, err := media.NewService(mediapg.New(h.pool), store, credentials.Generator{}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	var input bytes.Buffer
	fixture := image.NewRGBA(image.Rect(0, 0, 24, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 24; x++ {
			fixture.Set(x, y, color.RGBA{R: uint8(x * 7), G: uint8(y * 11), B: 90, A: 255})
		}
	}
	if err := jpeg.Encode(&input, fixture, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}

	uploaded, err := service.UploadProfile(h.ctx, owner, "img01e-profile-upload-0001", input.Bytes())
	if err != nil || uploaded.Photo.ID == "" || len(uploaded.Photo.Variants) != 1 || uploaded.Photo.Variants[0].MIME != "image/webp" {
		t.Fatalf("upload profile media result=%+v err=%v", uploaded, err)
	}
	variant := uploaded.Photo.Variants[0]
	photo, storedVariant, content, err := service.ProfileContent(h.ctx, owner)
	if err != nil || photo.ID != uploaded.Photo.ID || storedVariant.MIME != "image/webp" || len(content) == 0 || int64(len(content)) != variant.Size || storedVariant.SHA256 != variant.SHA256 {
		t.Fatalf("read uploaded profile media photo=%+v variant=%+v bytes=%d err=%v", photo, storedVariant, len(content), err)
	}

	archive := ownerexport.NewWithMediaStore(h.pool, nil, store)
	sections, files, exclusions, err := archive.ExportAdditionalOwnData(h.ctx, owner)
	if err != nil {
		t.Fatalf("export owner media: %v", err)
	}
	var mediaSection struct {
		Assets []struct {
			ID       string `json:"id"`
			Purpose  string `json:"purpose"`
			Variants []struct {
				Name          string `json:"name"`
				MIME          string `json:"mime_type"`
				Size          int64  `json:"size_bytes"`
				SHA256        string `json:"sha256"`
				FileAvailable bool   `json:"file_available"`
				ArchiveFile   string `json:"archive_file"`
			} `json:"variants"`
		} `json:"assets"`
	}
	if err := json.Unmarshal(sections["media_assets"], &mediaSection); err != nil {
		t.Fatalf("decode media export section: %v", err)
	}
	if len(mediaSection.Assets) != 1 || mediaSection.Assets[0].ID != uploaded.Photo.ID || mediaSection.Assets[0].Purpose != "perfil" || len(mediaSection.Assets[0].Variants) != 1 {
		t.Fatalf("unexpected real media export section: %s", sections["media_assets"])
	}
	exportedVariant := mediaSection.Assets[0].Variants[0]
	wantName := "files/media/perfil/" + uploaded.Photo.ID + "/avatar.webp"
	if exportedVariant.Name != "avatar" || exportedVariant.MIME != "image/webp" || exportedVariant.Size != int64(len(content)) || !exportedVariant.FileAvailable || exportedVariant.ArchiveFile != wantName {
		t.Fatalf("export metadata does not describe verified avatar bytes: %+v", exportedVariant)
	}
	if strings.Contains(string(sections["media_assets"]), "object_key") || strings.Contains(string(sections["media_assets"]), "bucket_name") || strings.Contains(string(sections["media_assets"]), "generation") {
		t.Fatalf("media export leaked private storage coordinates: %s", sections["media_assets"])
	}
	if len(exclusions) != 0 {
		t.Fatalf("unexpected media export exclusions: %v", exclusions)
	}
	assertIMG01EPrivacyWorkerCannotMutateUnclaimedMedia(t, h, owner, uploaded.Photo.ID)
	found := false
	for _, file := range files {
		if file.Name == wantName {
			found = file.MediaType == "image/webp" && bytes.Equal(file.Content, content)
		}
	}
	if !found {
		t.Fatalf("verified WebP avatar missing from archive files: %s", wantName)
	}

	if err := service.RemoveProfile(h.ctx, owner); err != nil {
		t.Fatalf("revoke profile avatar: %v", err)
	}
	if _, _, _, err := service.ProfileContent(h.ctx, owner); !errors.Is(err, media.ErrNotFound) {
		t.Fatalf("avatar remains readable after committed removal: %v", err)
	}
	workerMediaService, _ := newIMG01EPrivacyWorker(t, h, store)
	if err := workerMediaService.CleanOnce(h.ctx, 10); err != nil {
		t.Fatalf("clean retired avatar variant: %v", err)
	}
	if _, err := store.Generation(h.ctx, variant.ObjectKey); !errors.Is(err, media.ErrNotFound) {
		t.Fatalf("retired avatar object remains after cleanup: %v", err)
	}
}

func assertIMG01EPrivacyWorkerCannotMutateUnclaimedMedia(t *testing.T, h *authHarness, owner, uploadedID string) {
	t.Helper()
	otherID := "00000000-0000-4000-8000-000000000073"
	_, err := h.pool.Exec(h.ctx, `INSERT INTO public.media_asset
		(id,propietario_id,proposito,estado,idempotency_key,input_sha256,lease_until)
		VALUES($1,$2,'perfil','pending','img01e-negative-worker-0001',repeat('0',64),clock_timestamp()+interval '1 hour')`, otherID, owner)
	if err != nil {
		t.Fatalf("seed unrelated pending asset: %v", err)
	}
	config := h.pool.Config().Copy()
	config.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, `SET ROLE espacigo_privacy_cleanup_worker`)
		return err
	}
	worker, err := pgxpool.NewWithConfig(h.ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer worker.Close()
	future := time.Date(2099, time.January, 1, 0, 0, 0, 0, time.UTC)
	var claimedID string
	var claimedToken int64
	if err := worker.QueryRow(h.ctx, `SELECT id,fencing_token FROM public.claim_private_media_cleanup(100,$1)`, future).Scan(&claimedID, &claimedToken); err == nil {
		t.Error("worker claim accepted a future timestamp")
	}
	for name, query := range map[string]string{
		"complete": `SELECT public.complete_private_media_cleanup($1,0,$2)`,
		"retry":    `SELECT public.fail_private_media_cleanup($1,0,$2)`,
	} {
		var result bool
		if err := worker.QueryRow(h.ctx, query, uploadedID, future).Scan(&result); err == nil {
			t.Errorf("worker %s accepted a future timestamp", name)
		}
	}

	for name, query := range map[string]string{
		"update unrelated asset":    `UPDATE public.media_asset SET estado='deleted' WHERE id=$1`,
		"delete unrelated variants": `DELETE FROM public.media_variant WHERE media_id=$1`,
		"delete unrelated intents":  `DELETE FROM public.media_variant_intent WHERE media_id=$1`,
	} {
		if _, err := worker.Exec(h.ctx, query, otherID); err == nil {
			t.Errorf("worker unexpectedly could %s", name)
		}
	}
	var completed bool
	if err := worker.QueryRow(h.ctx, `SELECT public.complete_private_media_cleanup($1,0,clock_timestamp())`, uploadedID).Scan(&completed); err != nil || completed {
		t.Fatalf("worker completed an asset without a current claim: completed=%t err=%v", completed, err)
	}
	var state string
	if err := h.pool.QueryRow(h.ctx, `SELECT estado FROM public.media_asset WHERE id=$1`, otherID).Scan(&state); err != nil || state != "pending" {
		t.Fatalf("unrelated asset state=%q err=%v", state, err)
	}
}

func assertIMG01ERuntimeMediaPrivileges(t *testing.T, h *authHarness) {
	t.Helper()
	var unexpected []string
	rows, err := h.pool.Query(h.ctx, `SELECT table_name || ':' || privilege_name
		FROM (VALUES
			('media_asset','SELECT',true),('media_asset','INSERT',true),('media_asset','UPDATE',true),('media_asset','DELETE',false),('media_asset','TRUNCATE',false),
			('media_variant','SELECT',true),('media_variant','INSERT',true),('media_variant','UPDATE',false),('media_variant','DELETE',false),('media_variant','TRUNCATE',false),
			('space_media','SELECT',true),('space_media','INSERT',true),('space_media','UPDATE',true),('space_media','DELETE',false),('space_media','TRUNCATE',false),
			('profile_media','SELECT',true),('profile_media','INSERT',true),('profile_media','UPDATE',true),('profile_media','DELETE',true),('profile_media','TRUNCATE',false),
			('gallery_item_order','SELECT',true),('gallery_item_order','INSERT',true),('gallery_item_order','UPDATE',true),('gallery_item_order','DELETE',true),('gallery_item_order','TRUNCATE',false),
			('media_variant_intent','SELECT',true),('media_variant_intent','INSERT',true),('media_variant_intent','UPDATE',true),('media_variant_intent','DELETE',true),('media_variant_intent','TRUNCATE',false)
		) AS expected(table_name, privilege_name, allowed)
		WHERE has_table_privilege('espacigo_runtime', 'public.' || table_name, privilege_name) IS DISTINCT FROM allowed
		ORDER BY table_name, privilege_name`)
	if err != nil {
		t.Fatalf("inspect runtime media privileges: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var privilege string
		if err := rows.Scan(&privilege); err != nil {
			t.Fatalf("scan runtime media privilege: %v", err)
		}
		unexpected = append(unexpected, privilege)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read runtime media privileges: %v", err)
	}
	if len(unexpected) > 0 {
		t.Fatalf("runtime media privilege contract mismatch: %v", unexpected)
	}
}

func seedIMG01EMediaForSuppression(t *testing.T, h *authHarness, owner, spaceID string) (*media.Service, *mediafs.Store, media.Photo, media.Photo) {
	t.Helper()
	store, err := mediafs.New(t.TempDir() + "/suppression-private-media")
	if err != nil {
		t.Fatal(err)
	}
	service, err := media.NewService(mediapg.New(h.pool), store, credentials.Generator{}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	var input bytes.Buffer
	fixture := image.NewRGBA(image.Rect(0, 0, 24, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 24; x++ {
			fixture.Set(x, y, color.RGBA{R: uint8(x * 7), G: uint8(y * 11), B: 90, A: 255})
		}
	}
	if err := jpeg.Encode(&input, fixture, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	profile, err := service.UploadProfile(h.ctx, owner, "img01e-suppress-profile-0001", input.Bytes())
	if err != nil {
		t.Fatalf("upload synthetic avatar for suppression test: %v", err)
	}
	spacePhoto, err := service.UploadSpace(h.ctx, owner, spaceID, "img01e-suppress-space-0001", input.Bytes())
	if err != nil {
		t.Fatalf("upload synthetic gallery photo for suppression test: %v", err)
	}
	return service, store, profile.Photo, spacePhoto.Photo
}

func seedIMG01EHistoricalMediaStates(t *testing.T, h *authHarness, owner, spaceID string) {
	t.Helper()
	_, err := h.pool.Exec(h.ctx, `INSERT INTO public.media_asset
		(id,propietario_id,espacio_id,proposito,estado,idempotency_key,input_sha256,fencing_token,lease_until,created_at,updated_at,deleted_at)
		VALUES
		('82000000-0000-4000-8000-000000000030',$1,NULL,'perfil','failed','img01e-failed-state',repeat('a',64),1,NULL,$3,$3,NULL),
		('82000000-0000-4000-8000-000000000031',$1,$2,'espacio','deleting','img01e-deleting-state',repeat('b',64),1,$3,$3,$3,NULL),
		('82000000-0000-4000-8000-000000000032',$1,NULL,'perfil','deleted','img01e-deleted-state',repeat('c',64),1,NULL,$3,$3,$3)`, owner, spaceID, h.now)
	if err != nil {
		t.Fatalf("seed historical media lifecycle states: %v", err)
	}
}

func assertIMG01EPrivacyMediaMetadataRedacted(t *testing.T, h *authHarness, owner string) {
	t.Helper()
	rows, err := h.pool.Query(h.ctx, `SELECT id::text,estado,idempotency_key,input_sha256,fencing_token,lease_until,deleted_at
		FROM public.media_asset WHERE propietario_id=$1 AND id BETWEEN '82000000-0000-4000-8000-000000000030'::uuid AND '82000000-0000-4000-8000-000000000032'::uuid ORDER BY id`, owner)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	want := []struct {
		id, state string
		deleted   bool
	}{
		{"82000000-0000-4000-8000-000000000030", "failed", false},
		{"82000000-0000-4000-8000-000000000031", "deleting", false},
		{"82000000-0000-4000-8000-000000000032", "deleted", true},
	}
	for _, expected := range want {
		if !rows.Next() {
			t.Fatalf("historical media state %s missing", expected.id)
		}
		var id, state, key, digest string
		var token int64
		var leaseUntil, deletedAt *time.Time
		if err := rows.Scan(&id, &state, &key, &digest, &token, &leaseUntil, &deletedAt); err != nil {
			t.Fatal(err)
		}
		if id != expected.id || state != expected.state || key != "suppressed:"+id || digest != strings.Repeat("0", 64) || token != 1 {
			t.Fatalf("privacy redaction changed lifecycle or kept identifying metadata: id=%s state=%s key=%s digest=%s token=%d", id, state, key, digest, token)
		}
		if expected.state == "deleting" && (leaseUntil == nil || !leaseUntil.Equal(h.now)) {
			t.Fatalf("in-progress cleanup lease changed: %v", leaseUntil)
		}
		if expected.state != "deleting" && leaseUntil != nil {
			t.Fatalf("terminal media gained a cleanup lease: state=%s lease=%v", state, leaseUntil)
		}
		if expected.deleted && (deletedAt == nil || !deletedAt.Equal(h.now)) {
			t.Fatalf("deleted media timestamp changed: %v", deletedAt)
		}
		if !expected.deleted && deletedAt != nil {
			t.Fatalf("non-deleted media gained deleted_at: state=%s at=%v", state, deletedAt)
		}
	}
	if rows.Next() {
		t.Fatal("unexpected historical media row in lifecycle-state regression")
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
}

func newIMG01EPrivacyWorker(t *testing.T, h *authHarness, store *mediafs.Store) (*media.Service, *privacy.Service) {
	t.Helper()
	config := h.pool.Config().Copy()
	config.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, `SET ROLE espacigo_privacy_cleanup_worker`)
		return err
	}
	pool, err := pgxpool.NewWithConfig(h.ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	mediaService, err := media.NewService(mediapg.New(pool), store, credentials.Generator{}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	privacyService, err := privacy.NewService(identitypg.NewSuppressionCleanupWorkerRepository(pool))
	if err != nil {
		t.Fatal(err)
	}
	return mediaService, privacyService
}
