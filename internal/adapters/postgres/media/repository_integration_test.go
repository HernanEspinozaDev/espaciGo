package mediapg

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/media"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestIMG01DProfileTriggerAndNullableCover(t *testing.T) {
	url := os.Getenv("IMG01D_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set IMG01D_TEST_DATABASE_URL to a disposable V72 database")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	newID := func() string {
		var b [16]byte
		if _, err := rand.Read(b[:]); err != nil {
			t.Fatal(err)
		}
		b[6] = (b[6] & 0x0f) | 0x40
		b[8] = (b[8] & 0x3f) | 0x80
		return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
	}
	owner, spaceID := newID(), newID()
	profileAsset, spaceAsset, relationID, orderID := newID(), newID(), newID(), newID()
	email := owner + "@img01d.invalid"
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM public.gallery_item_order WHERE id=$1`, orderID)
		_, _ = pool.Exec(ctx, `DELETE FROM public.space_media WHERE id=$1`, relationID)
		_, _ = pool.Exec(ctx, `DELETE FROM public.profile_media WHERE propietario_id=$1`, owner)
		_, _ = pool.Exec(ctx, `DELETE FROM public.media_asset WHERE id IN ($1,$2)`, profileAsset, spaceAsset)
		_, _ = pool.Exec(ctx, `DELETE FROM public.espacio WHERE id=$1`, spaceID)
		_, _ = pool.Exec(ctx, `DELETE FROM public.usuario WHERE id=$1`, owner)
	})
	if _, err = pool.Exec(ctx, `INSERT INTO public.usuario(id,correo_original,correo_normalizado,hash_clave,estado) VALUES($1,$2,$2,'test','activo')`, owner, email); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO public.espacio(id,propietario_id,categoria_codigo,titulo,descripcion,superficie_m2,capacidad_maxima,reglas_uso,modalidad_tarifa,precio_base_clp,direccion)
		VALUES($1,$2,'oficina','IMG-01D integration','Descripción sintética con más de cien caracteres para cumplir el mínimo que exige la ficha del espacio.',10,1,'Sin reglas adicionales','hora',6000,'Dirección sintética')`, spaceID, owner); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO public.media_asset(id,propietario_id,proposito,estado,idempotency_key,input_sha256)
		VALUES($1::uuid,$2,'perfil','ready','img01d-profile-'||substr($1::uuid::text,1,8),repeat('a',64))`, profileAsset, owner); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO public.profile_media(propietario_id,media_id) VALUES($1,$2)`, owner, profileAsset); err != nil {
		t.Fatalf("V72 profile relation trigger rejected its valid table shape: %v", err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO public.media_asset(id,propietario_id,espacio_id,proposito,estado,idempotency_key,input_sha256)
		VALUES($1::uuid,$2,$3,'espacio','ready','img01d-space-'||substr($1::uuid::text,1,8),repeat('b',64))`, spaceAsset, owner, spaceID); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO public.space_media(id,espacio_id,propietario_id,media_id,visible) VALUES($1,$2,$3,$4,true)`, relationID, spaceID, owner, spaceAsset); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO public.gallery_item_order(id,espacio_id,propietario_id,space_media_id,position) VALUES($1,$2,$3,$4,1)`, orderID, spaceID, owner, relationID); err != nil {
		t.Fatal(err)
	}
	r := New(pool)
	listed, err := r.ListSpace(ctx, owner, spaceID)
	if err != nil || len(listed) != 1 || listed[0].Cover {
		t.Fatalf("list without a cover=%+v err=%v", listed, err)
	}
	got, err := r.GetSpace(ctx, owner, spaceID, relationID)
	if err != nil || got.Cover {
		t.Fatalf("get without a cover=%+v err=%v", got, err)
	}
}

func TestIMG01CMigrationBackedRepositoryQueries(t *testing.T) {
	url := os.Getenv("IMG01C_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set IMG01C_TEST_DATABASE_URL to a disposable V70 database")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	r := New(pool)
	items, err := r.ListSpace(ctx, "00000000-0000-4000-8000-000000000001", "00000000-0000-4000-8000-000000000002")
	if err != nil || len(items) != 0 {
		t.Fatalf("empty media list=%v err=%v", items, err)
	}
	cleanup, err := r.ClaimCleanup(ctx, 10, time.Now().UTC())
	if err != nil || len(cleanup) != 0 {
		t.Fatalf("empty cleanup=%v err=%v", cleanup, err)
	}
	if err = r.PlanVariant(ctx, "00000000-0000-4000-8000-000000000003", 1, media.VariantDisplay, "media/v1/spaces/00000000-0000-4000-8000-000000000002/00000000-0000-4000-8000-000000000003/1/display.webp"); !errors.Is(err, media.ErrConflict) {
		t.Fatalf("missing reservation PlanVariant error=%v", err)
	}
}

func TestFinishVariantWriteSurvivesCleanupFence(t *testing.T) {
	url := os.Getenv("IMG01C_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set IMG01C_TEST_DATABASE_URL to a disposable V71 database")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	owner := "00000000-0000-4000-8000-000000000011"
	asset := "00000000-0000-4000-8000-000000000012"
	key := "media/v1/profiles/00000000-0000-4000-8000-000000000011/00000000-0000-4000-8000-000000000012/1/avatar.webp"
	_, err = pool.Exec(ctx, `INSERT INTO public.usuario(id,correo_original,correo_normalizado,hash_clave) VALUES($1,'img01c@example.test','img01c@example.test','test')`, owner)
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO public.media_asset(id,propietario_id,proposito,estado,idempotency_key,input_sha256,lease_until)
		VALUES($1,$2,'perfil','pending','img01c-fail-01',repeat('0',64),clock_timestamp()-interval '1 minute')`, asset, owner)
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO public.media_variant_intent(media_id,variant,object_key) VALUES($1,'avatar',$2)`, asset, key)
	if err != nil {
		t.Fatal(err)
	}
	r := New(pool)
	// Simulate the worker claiming first: it increments the fencing token and
	// marks the asset deleting while Put is still in flight.
	claimed, err := r.ClaimCleanup(ctx, 1, time.Now().UTC())
	if err != nil || len(claimed) != 1 || claimed[0].Intents[0].WriteComplete {
		t.Fatalf("initial cleanup claim=%+v err=%v", claimed, err)
	}
	if err = r.FinishVariantWrite(ctx, asset, media.VariantAvatar, key); err != nil {
		t.Fatal(err)
	}
	var state string
	var complete bool
	if err = pool.QueryRow(ctx, `SELECT a.estado,i.write_complete FROM public.media_asset a JOIN public.media_variant_intent i ON i.media_id=a.id WHERE a.id=$1`, asset).Scan(&state, &complete); err != nil {
		t.Fatal(err)
	}
	if state != "deleting" || !complete {
		t.Fatalf("state=%s write_complete=%t", state, complete)
	}
}

const img01fSyntheticDatabase = "espacigo_f6_synth_20261028"

func img01fTargetAllowed(host string, port uint16, database, user string) bool {
	return host == "127.0.0.1" && port == 15432 && database == img01fSyntheticDatabase && user == "postgres"
}

// TestSpacePhotoReplacementAtomic exercises the V76/V78 replacement and replay path,
// including the real HTTP content handler and private filesystem-backed variants.
// It uses only synthetic fixtures on the exact isolated target and never runs DDL.
func TestSpacePhotoReplacementAtomic(t *testing.T) {
	url := os.Getenv("IMG01F_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set IMG01F_TEST_DATABASE_URL to the local synthetic-test database")
	}
	if os.Getenv("IMG01F_TEST_STORAGE_ROOT") != img01fHTTPStorageRoot {
		t.Fatal("set IMG01F_TEST_STORAGE_ROOT to the authorized private scratch media root")
	}
	storageInfo, storageErr := os.Lstat(img01fHTTPStorageRoot)
	if storageErr != nil || !storageInfo.IsDir() || storageInfo.Mode()&os.ModeSymlink != 0 || storageInfo.Mode().Perm()&0o077 != 0 {
		t.Fatalf("authorized storage root must be private and non-symlink before fixtures: %v", storageErr)
	}
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal("invalid IMG01F_TEST_DATABASE_URL")
	}
	if !img01fTargetAllowed(cfg.ConnConfig.Host, cfg.ConnConfig.Port, cfg.ConnConfig.Database, cfg.ConnConfig.User) {
		t.Fatal("IMG01F_TEST_DATABASE_URL must use postgres at 127.0.0.1:15432/espacigo_f6_synth_20261028")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	appName := fmt.Sprintf("img01f-%d-%d", os.Getpid(), time.Now().UnixNano())
	if cfg.ConnConfig.RuntimeParams == nil {
		cfg.ConnConfig.RuntimeParams = make(map[string]string)
	}
	cfg.ConnConfig.RuntimeParams["application_name"] = appName
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err = pool.Ping(ctx); err != nil {
		t.Fatal(err)
	}
	// Fail closed before registering fixture cleanup or issuing writes when V76/V78
	// is absent. These catalog checks avoid schema_migrations permissions.
	var replacementColumnExists, replacementTriggerExists, replacementFunctionExists bool
	var expectedRevisionColumnExists, expectedRevisionTriggerExists bool
	if err = pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM pg_attribute
			WHERE attrelid = 'public.media_asset'::regclass
			  AND attname = 'replacement_target_space_media_id'
			  AND NOT attisdropped
		), EXISTS (
			SELECT 1
			FROM pg_trigger t
			JOIN pg_proc p ON p.oid = t.tgfoid
			JOIN pg_namespace n ON n.oid = p.pronamespace
			WHERE t.tgrelid = 'public.media_asset'::regclass
			  AND NOT t.tgisinternal
			  AND t.tgname = 'media_asset_slot_reservation'
			  AND n.nspname = 'public'
			  AND p.proname = 'img01b_enforce_gallery_slot'
		), EXISTS (
			SELECT 1 FROM pg_proc p
			JOIN pg_namespace n ON n.oid = p.pronamespace
			WHERE n.nspname = 'public'
			  AND p.proname = 'img01b_enforce_gallery_slot'
		), EXISTS (
			SELECT 1 FROM pg_attribute
			WHERE attrelid = 'public.media_asset'::regclass
			  AND attname = 'replacement_expected_revision'
			  AND NOT attisdropped
		), EXISTS (
			SELECT 1
			FROM pg_trigger t
			JOIN pg_proc p ON p.oid = t.tgfoid
			JOIN pg_namespace n ON n.oid = p.pronamespace
			WHERE t.tgrelid = 'public.media_asset'::regclass
			  AND NOT t.tgisinternal
			  AND t.tgname = 'media_asset_replacement_request_revision_guard'
			  AND n.nspname = 'public'
			  AND p.proname = 'img01b_guard_replacement_request_revision'
		)`).Scan(&replacementColumnExists, &replacementTriggerExists, &replacementFunctionExists, &expectedRevisionColumnExists, &expectedRevisionTriggerExists); err != nil {
		t.Fatalf("check V76/V78 replacement schema: %v", err)
	}
	if !replacementColumnExists || !replacementTriggerExists || !replacementFunctionExists || !expectedRevisionColumnExists || !expectedRevisionTriggerExists {
		t.Fatalf("V76/V78 replacement schema is not installed (target_column=%t target_trigger=%t target_function=%t revision_column=%t revision_trigger=%t)", replacementColumnExists, replacementTriggerExists, replacementFunctionExists, expectedRevisionColumnExists, expectedRevisionTriggerExists)
	}
	newID := func() string {
		var b [16]byte
		if _, err := rand.Read(b[:]); err != nil {
			t.Fatal(err)
		}
		b[6] = (b[6] & 0x0f) | 0x40
		b[8] = (b[8] & 0x3f) | 0x80
		return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
	}
	owner, foreignOwner, spaceID, oldID, nextID, orderID, keyProbeID := newID(), newID(), newID(), newID(), newID(), newID(), newID()
	staleSpace, staleOld, staleNew, staleOrder := newID(), newID(), newID(), newID()
	tag := "img01f-" + hex.EncodeToString([]byte(newID()))
	httpKey := newID()
	raceKey := newID()
	galleryKey := newID()
	now := time.Now().UTC().Truncate(time.Microsecond)
	lease := now.Add(10 * time.Minute)
	t.Cleanup(func() {
		c, stop := context.WithTimeout(context.Background(), 10*time.Second)
		defer stop()
		do := func(query string, args ...any) {
			if _, e := pool.Exec(c, query, args...); e != nil {
				t.Errorf("cleanup: %v", e)
			}
		}
		// Remove the HTTP-created child replacement before its target slot; its
		// persisted idempotency key is known even if the test failed mid-response.
		do(`DELETE FROM public.gallery_item_order WHERE espacio_id=$1 AND propietario_id=$2`, spaceID, owner)
		do(`UPDATE public.espacio SET portada_media_id=NULL WHERE id=$1 AND propietario_id=$2`, spaceID, owner)
		do(`DELETE FROM public.space_media WHERE id IN (SELECT id FROM public.media_asset WHERE idempotency_key=$1 AND espacio_id=$2 AND propietario_id=$3)`, raceKey, spaceID, owner)
		do(`DELETE FROM public.media_variant_intent WHERE media_id IN (SELECT id FROM public.media_asset WHERE idempotency_key=$1 AND espacio_id=$2 AND propietario_id=$3)`, raceKey, spaceID, owner)
		do(`DELETE FROM public.media_variant WHERE media_id IN (SELECT id FROM public.media_asset WHERE idempotency_key=$1 AND espacio_id=$2 AND propietario_id=$3)`, raceKey, spaceID, owner)
		do(`DELETE FROM public.media_asset WHERE idempotency_key=$1 AND espacio_id=$2 AND propietario_id=$3`, raceKey, spaceID, owner)
		do(`DELETE FROM public.space_media WHERE id IN (SELECT id FROM public.media_asset WHERE idempotency_key=$1 AND espacio_id=$2 AND propietario_id=$3)`, httpKey, spaceID, owner)
		do(`DELETE FROM public.media_variant_intent WHERE media_id IN (SELECT id FROM public.media_asset WHERE idempotency_key=$1 AND espacio_id=$2 AND propietario_id=$3)`, httpKey, spaceID, owner)
		do(`DELETE FROM public.media_variant WHERE media_id IN (SELECT id FROM public.media_asset WHERE idempotency_key=$1 AND espacio_id=$2 AND propietario_id=$3)`, httpKey, spaceID, owner)
		do(`DELETE FROM public.media_asset WHERE idempotency_key=$1 AND espacio_id=$2 AND propietario_id=$3`, httpKey, spaceID, owner)
		for _, sid := range []string{spaceID, staleSpace} {
			do(`DELETE FROM public.gallery_item_order WHERE espacio_id=$1 AND propietario_id=$2`, sid, owner)
			do(`UPDATE public.espacio SET portada_media_id=NULL WHERE id=$1 AND propietario_id=$2`, sid, owner)
			do(`DELETE FROM public.space_media WHERE id IN (SELECT id FROM public.media_asset WHERE espacio_id=$1 AND propietario_id=$2 AND replacement_target_space_media_id IS NOT NULL)`, sid, owner)
			do(`DELETE FROM public.media_variant_intent WHERE media_id IN (SELECT id FROM public.media_asset WHERE espacio_id=$1 AND propietario_id=$2 AND replacement_target_space_media_id IS NOT NULL)`, sid, owner)
			do(`DELETE FROM public.media_variant WHERE media_id IN (SELECT id FROM public.media_asset WHERE espacio_id=$1 AND propietario_id=$2 AND replacement_target_space_media_id IS NOT NULL)`, sid, owner)
			do(`DELETE FROM public.media_asset WHERE espacio_id=$1 AND propietario_id=$2 AND replacement_target_space_media_id IS NOT NULL`, sid, owner)
			do(`DELETE FROM public.space_media WHERE espacio_id=$1 AND propietario_id=$2`, sid, owner)
			do(`DELETE FROM public.media_variant_intent WHERE media_id IN (SELECT id FROM public.media_asset WHERE espacio_id=$1 AND propietario_id=$2)`, sid, owner)
			do(`DELETE FROM public.media_variant WHERE media_id IN (SELECT id FROM public.media_asset WHERE espacio_id=$1 AND propietario_id=$2)`, sid, owner)
			do(`DELETE FROM public.media_asset WHERE espacio_id=$1 AND propietario_id=$2`, sid, owner)
		}
		do(`DELETE FROM public.espacio WHERE id IN ($1,$2) AND propietario_id=$3`, spaceID, staleSpace, owner)
		do(`DELETE FROM public.media_variant_intent WHERE media_id=$1`, keyProbeID)
		do(`DELETE FROM public.media_variant WHERE media_id=$1`, keyProbeID)
		do(`DELETE FROM public.media_asset WHERE id=$1 AND propietario_id=$2`, keyProbeID, owner)
		do(`DELETE FROM public.usuario WHERE id=$1`, owner)
		verifyZero := func(table string, query string, args ...any) {
			var count int64
			if e := pool.QueryRow(c, query, args...).Scan(&count); e != nil {
				t.Errorf("cleanup verification %s: %v", table, e)
				return
			}
			if count != 0 {
				t.Errorf("cleanup left %s fixture rows: %d", table, count)
			}
		}
		verifyZero("usuario", `SELECT count(*) FROM public.usuario WHERE id=$1`, owner)
		verifyZero("espacio", `SELECT count(*) FROM public.espacio WHERE propietario_id=$1 OR id IN ($2,$3)`, owner, spaceID, staleSpace)
		verifyZero("media_asset", `SELECT count(*) FROM public.media_asset WHERE propietario_id=$1 OR id=$2`, owner, keyProbeID)
		verifyZero("space_media", `SELECT count(*) FROM public.space_media WHERE propietario_id=$1 OR espacio_id IN ($2,$3)`, owner, spaceID, staleSpace)
		verifyZero("gallery_item_order", `SELECT count(*) FROM public.gallery_item_order WHERE propietario_id=$1 OR espacio_id IN ($2,$3)`, owner, spaceID, staleSpace)
		verifyZero("media_variant", `SELECT count(*) FROM public.media_variant WHERE media_id IN (SELECT id FROM public.media_asset WHERE propietario_id=$1 OR id=$2)`, owner, keyProbeID)
		verifyZero("media_variant_intent", `SELECT count(*) FROM public.media_variant_intent WHERE media_id IN (SELECT id FROM public.media_asset WHERE propietario_id=$1 OR id=$2)`, owner, keyProbeID)
	})
	if _, err = pool.Exec(ctx, `INSERT INTO public.usuario(id,correo_original,correo_normalizado,hash_clave,estado) VALUES($1,$2,$2,'synthetic','activo')`, owner, tag+"@example.invalid"); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO public.espacio(id,propietario_id,categoria_codigo,titulo,descripcion,superficie_m2,capacidad_maxima,reglas_uso,modalidad_tarifa,precio_base_clp,direccion,estado,actualizado_en) VALUES($1,$2,'oficina','Synthetic replacement',repeat('Synthetic description. ',6),20,4,'Synthetic rules','dia',9000,'Synthetic address','borrador',$3)`, spaceID, owner, now); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO public.media_asset(id,propietario_id,espacio_id,proposito,estado,idempotency_key,input_sha256,fencing_token,created_at,updated_at) VALUES($1,$2,$3,'espacio','ready',$4,repeat('a',64),1,$5,$5)`, oldID, owner, spaceID, tag+"-old", now); err != nil {
		t.Fatal(err)
	}
	for i, v := range []string{"display", "thumbnail"} {
		if _, err = pool.Exec(ctx, `INSERT INTO public.media_variant(media_id,variant,provider,bucket_name,object_key,generation,mime_type,size_bytes,sha256,width,height) VALUES($1,$2,'gcs','synthetic-bucket',$3,$4,'image/webp',128,repeat('b',64),320,240)`, oldID, v, fmt.Sprintf("synthetic/%s/%s.webp", oldID, v), i+1); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = pool.Exec(ctx, `INSERT INTO public.space_media(id,espacio_id,propietario_id,media_id,alt_text,visible,created_at) VALUES($1,$2,$3,$1,'Inherited synthetic alt',true,$4)`, oldID, spaceID, owner, now); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO public.gallery_item_order(id,espacio_id,propietario_id,space_media_id,position) VALUES($1,$2,$3,$4,1)`, orderID, spaceID, owner, oldID); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `UPDATE public.espacio SET portada_media_id=$2 WHERE id=$1`, spaceID, oldID); err != nil {
		t.Fatal(err)
	}
	var revision time.Time
	if err = pool.QueryRow(ctx, `SELECT actualizado_en FROM public.espacio WHERE id=$1`, spaceID).Scan(&revision); err != nil {
		t.Fatal(err)
	}
	expected := revision.UTC().Format(time.RFC3339Nano)
	var ordinaryPairValid bool
	if err = pool.QueryRow(ctx, `SELECT replacement_target_space_media_id IS NULL AND replacement_expected_revision IS NULL FROM public.media_asset WHERE id=$1`, oldID).Scan(&ordinaryPairValid); err != nil || !ordinaryPairValid {
		t.Fatalf("ordinary upload must persist both replacement fields as NULL: valid=%t err=%v", ordinaryPairValid, err)
	}
	for _, partial := range []struct {
		id, key  string
		target   any
		revision any
	}{
		{id: newID(), key: tag + "-partial-target", target: oldID},
		{id: newID(), key: tag + "-partial-revision", revision: expected},
	} {
		_, pairErr := pool.Exec(ctx, `INSERT INTO public.media_asset(id,propietario_id,espacio_id,proposito,estado,idempotency_key,input_sha256,lease_until,created_at,updated_at,replacement_target_space_media_id,replacement_expected_revision)
			VALUES($1,$2,$3,'espacio','pending',$4,repeat('c',64),$5,$6,$6,$7::uuid,$8::timestamptz)`, partial.id, owner, spaceID, partial.key, lease, now, partial.target, partial.revision)
		var pairPGErr *pgconn.PgError
		if !errors.As(pairErr, &pairPGErr) || pairPGErr.Code != "23514" {
			t.Fatalf("partial replacement pair insert (%s) error=%v, want check violation", partial.key, pairErr)
		}
		var partialRows int
		if err = pool.QueryRow(ctx, `SELECT count(*) FROM public.media_asset WHERE id=$1`, partial.id).Scan(&partialRows); err != nil || partialRows != 0 {
			t.Fatalf("partial pair row persisted (%s): count=%d err=%v", partial.key, partialRows, err)
		}
	}
	repo := New(pool)
	reservation, _, err := repo.Reserve(ctx, nextID, owner, spaceID, media.PurposeSpace, tag+"-next", fmt.Sprintf("%064d", 1), now, lease, media.ReservationOptions{ReplacementTargetID: oldID, ExpectedRevision: expected})
	if err != nil {
		t.Fatalf("reserve replacement: %v", err)
	}
	v1Key := fmt.Sprintf("media/v1/spaces/%s/%s/%d/display.webp", spaceID, nextID, reservation.Token)
	v2Key := fmt.Sprintf("users/%s/spaces/%s/images/v2/%s/%s_%d_thumbnail.webp", owner, spaceID, nextID, now.UTC().Format("20060102T150405.000000000Z"), reservation.Token)
	for _, intent := range []struct{ variant, key string }{{"display", v1Key}, {"thumbnail", v2Key}} {
		if _, err = pool.Exec(ctx, `INSERT INTO public.media_variant_intent(media_id,variant,object_key) VALUES($1,$2,$3)`, nextID, intent.variant, intent.key); err != nil {
			t.Fatalf("V1/V2 intent key %s rejected: %v", intent.variant, err)
		}
	}
	if _, err = pool.Exec(ctx, `INSERT INTO public.media_asset(id,propietario_id,proposito,estado,idempotency_key,input_sha256,lease_until,created_at,updated_at) VALUES($1,$2,'perfil','pending',$3,repeat('f',64),$4,$5,$5)`, keyProbeID, owner, tag+"-key-probe", lease, now); err != nil {
		t.Fatalf("insert synthetic profile key-probe asset: %v", err)
	}
	_, keyErr := pool.Exec(ctx, `INSERT INTO public.media_variant_intent(media_id,variant,object_key) VALUES($1,'avatar','users/not-a-uuid/images/v2/not-valid.webp')`, keyProbeID)
	var keyPGErr *pgconn.PgError
	if !errors.As(keyErr, &keyPGErr) || keyPGErr.Code != "23514" {
		t.Fatalf("invalid V2 intent key error=%v, want check violation", keyErr)
	}
	for i, v := range []string{"display", "thumbnail"} {
		if _, err = pool.Exec(ctx, `INSERT INTO public.media_variant(media_id,variant,provider,bucket_name,object_key,generation,mime_type,size_bytes,sha256,width,height) VALUES($1,$2,'gcs','synthetic-bucket',$3,$4,'image/webp',128,repeat('b',64),320,240)`, nextID, v, fmt.Sprintf("synthetic/%s/%s.webp", nextID, v), i+1); err != nil {
			t.Fatal(err)
		}
	}
	// Hold a compatible table lock so FinalizeSpace reaches its cover update
	// after inserting the candidate slot, changing order, promoting the asset,
	// and retiring the old slot. Cancel that blocked statement to prove the
	// transaction rolls every earlier mutation back.
	blockerTx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = blockerTx.Rollback(context.Background()) }()
	if _, err = blockerTx.Exec(ctx, `LOCK TABLE public.espacio IN SHARE MODE`); err != nil {
		t.Fatal(err)
	}
	failedCtx, cancelFailed := context.WithCancel(ctx)
	failedFinalize := make(chan error, 1)
	go func() {
		_, finalizeErr := repo.FinalizeSpace(failedCtx, nextID, reservation.Token, owner, spaceID, expected, time.Now().UTC())
		failedFinalize <- finalizeErr
	}()
	blocked := false
	blockDeadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(blockDeadline) {
		var waiting bool
		if err = pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE application_name=$1 AND wait_event_type='Lock' AND query ILIKE 'UPDATE public.espacio SET portada_media_id%')`, appName).Scan(&waiting); err != nil {
			cancelFailed()
			t.Fatal(err)
		}
		if waiting {
			blocked = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !blocked {
		cancelFailed()
		_ = blockerTx.Rollback(context.Background())
		t.Fatalf("FinalizeSpace did not block at the post-mutation cover update; result=%v", <-failedFinalize)
	}
	cancelFailed()
	select {
	case finalizeErr := <-failedFinalize:
		if finalizeErr == nil {
			t.Fatal("canceled FinalizeSpace unexpectedly committed")
		}
	case <-time.After(5 * time.Second):
		_ = blockerTx.Rollback(context.Background())
		t.Fatal("canceled FinalizeSpace did not return")
	}
	if err = blockerTx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	var failedOldState, failedNewState string
	if err = pool.QueryRow(ctx, `SELECT estado FROM public.media_asset WHERE id=$1`, oldID).Scan(&failedOldState); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `SELECT estado FROM public.media_asset WHERE id=$1`, nextID).Scan(&failedNewState); err != nil {
		t.Fatal(err)
	}
	var failedCandidateSlots int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM public.space_media WHERE id=$1`, nextID).Scan(&failedCandidateSlots); err != nil {
		t.Fatal(err)
	}
	var failedOrderSlot string
	var failedPosition int
	if err = pool.QueryRow(ctx, `SELECT space_media_id::text,position FROM public.gallery_item_order WHERE id=$1`, orderID).Scan(&failedOrderSlot, &failedPosition); err != nil {
		t.Fatal(err)
	}
	var failedCover string
	if err = pool.QueryRow(ctx, `SELECT portada_media_id::text FROM public.espacio WHERE id=$1`, spaceID).Scan(&failedCover); err != nil {
		t.Fatal(err)
	}
	var failedRetired *time.Time
	var failedVisible bool
	if err = pool.QueryRow(ctx, `SELECT retired_at,visible FROM public.space_media WHERE id=$1`, oldID).Scan(&failedRetired, &failedVisible); err != nil {
		t.Fatal(err)
	}
	if failedOldState != "ready" || failedNewState != "pending" || failedCandidateSlots != 0 || failedOrderSlot != oldID || failedPosition != 1 || failedCover != oldID || failedRetired != nil || !failedVisible {
		t.Fatalf("failed finalize left partial state: old=%s new=%s candidate_slots=%d order=%s/%d cover=%s retired=%v visible=%t", failedOldState, failedNewState, failedCandidateSlots, failedOrderSlot, failedPosition, failedCover, failedRetired, failedVisible)
	}
	photo, err := repo.FinalizeSpace(ctx, nextID, reservation.Token, owner, spaceID, expected, time.Now().UTC())
	if err != nil {
		t.Fatalf("finalize replacement after rollback: %v", err)
	}
	if photo.ID != nextID || photo.State != "ready" || !photo.Visible || !photo.Cover || photo.AltText != "Inherited synthetic alt" || photo.Position != 1 {
		t.Fatalf("replacement photo=%+v", photo)
	}
	var gotOrder, gotSlot string
	var position int
	if err = pool.QueryRow(ctx, `SELECT id::text,space_media_id::text,position FROM public.gallery_item_order WHERE espacio_id=$1`, spaceID).Scan(&gotOrder, &gotSlot, &position); err != nil {
		t.Fatal(err)
	}
	if gotOrder != orderID || gotSlot != nextID || position != 1 {
		t.Fatalf("order identity/position changed: %s %s %d", gotOrder, gotSlot, position)
	}
	var retired *time.Time
	var visible bool
	if err = pool.QueryRow(ctx, `SELECT retired_at,visible FROM public.space_media WHERE id=$1`, oldID).Scan(&retired, &visible); err != nil {
		t.Fatal(err)
	}
	if retired == nil || visible {
		t.Fatalf("old slot retirement=%v visible=%t", retired, visible)
	}
	var oldState, newState string
	var oldLease *time.Time
	if err = pool.QueryRow(ctx, `SELECT estado,lease_until FROM public.media_asset WHERE id=$1`, oldID).Scan(&oldState, &oldLease); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `SELECT estado FROM public.media_asset WHERE id=$1`, nextID).Scan(&newState); err != nil {
		t.Fatal(err)
	}
	var oldVariantCount int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM public.media_variant WHERE media_id=$1`, oldID).Scan(&oldVariantCount); err != nil {
		t.Fatal(err)
	}
	if oldState != "deleting" || newState != "ready" || oldLease == nil || oldLease.After(time.Now().UTC()) || oldVariantCount != 2 {
		t.Fatalf("asset states/cleanup old=%s lease=%v variants=%d new=%s", oldState, oldLease, oldVariantCount, newState)
	}
	var after time.Time
	if err = pool.QueryRow(ctx, `SELECT actualizado_en FROM public.espacio WHERE id=$1`, spaceID).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if !after.After(revision) {
		t.Fatalf("space revision did not advance: %s -> %s", revision, after)
	}

	// A successful retry must match the original request revision, even though
	// finalization advanced the live space revision. A changed revision with
	// the same key/hash/target is a different idempotency request and conflicts.
	replayOptions := media.ReservationOptions{ReplacementTargetID: oldID, ExpectedRevision: expected}
	replayed, reused, err := repo.Reserve(ctx, newID(), owner, spaceID, media.PurposeSpace, tag+"-next", fmt.Sprintf("%064d", 1), now, lease, replayOptions)
	if err != nil || !reused || replayed.State != "ready" || replayed.ExpectedRevision != expected {
		t.Fatalf("same-revision ready replay=%+v reused=%t err=%v", replayed, reused, err)
	}
	changedExpected := revision.Add(time.Second).UTC().Format(time.RFC3339Nano)
	_, _, err = repo.Reserve(ctx, newID(), owner, spaceID, media.PurposeSpace, tag+"-next", fmt.Sprintf("%064d", 1), now, lease, media.ReservationOptions{ReplacementTargetID: oldID, ExpectedRevision: changedExpected})
	if !errors.Is(err, media.ErrConflict) {
		t.Fatalf("ready replay with changed ExpectedRevision error=%v, want ErrConflict", err)
	}
	_, mutationErr := pool.Exec(ctx, `UPDATE public.media_asset SET replacement_expected_revision=$2 WHERE id=$1`, nextID, changedExpected)
	var revisionGuardErr *pgconn.PgError
	if !errors.As(mutationErr, &revisionGuardErr) || revisionGuardErr.Code != "23514" {
		t.Fatalf("mutating stored replacement revision error=%v, want check violation", mutationErr)
	}
	_, targetMutationErr := pool.Exec(ctx, `UPDATE public.media_asset SET replacement_target_space_media_id=$2 WHERE id=$1`, nextID, spaceID)
	var targetGuardErr *pgconn.PgError
	if !errors.As(targetMutationErr, &targetGuardErr) || targetGuardErr.Code != "23514" {
		t.Fatalf("mutating stored replacement target error=%v, want check violation", targetMutationErr)
	}
	var storedExpected time.Time
	var storedTarget string
	if err = pool.QueryRow(ctx, `SELECT replacement_expected_revision,replacement_target_space_media_id::text FROM public.media_asset WHERE id=$1`, nextID).Scan(&storedExpected, &storedTarget); err != nil || storedExpected.UTC().Format(time.RFC3339Nano) != expected || storedTarget != oldID {
		t.Fatalf("stored replacement metadata changed: target=%s revision=%s err=%v", storedTarget, storedExpected.UTC().Format(time.RFC3339Nano), err)
	}
	httpPhotoID := img01fExerciseHTTPReplacement(t, ctx, pool, owner, foreignOwner, spaceID, nextID, httpKey, after)
	if httpPhotoID == "" {
		t.Fatal("HTTP replacement returned no photo ID")
	}
	img01fAssertReserveFinalizeRevisionRace(t, ctx, pool, repo, owner, spaceID, httpPhotoID, raceKey, newID(), appName, now, lease)
	img01fExerciseHTTPGalleryMutation(t, ctx, pool, owner, foreignOwner, spaceID, galleryKey)

	// Fill the one remaining active slot to 29, then race two reservations for the 30th.
	for position := 2; position < media.MaxSpaceMedia; position++ {
		assetID := newID()
		if _, err = pool.Exec(ctx, `INSERT INTO public.media_asset(id,propietario_id,espacio_id,proposito,estado,idempotency_key,input_sha256,created_at,updated_at) VALUES($1,$2,$3,'espacio','ready',$4,repeat('c',64),$5,$5)`, assetID, owner, spaceID, fmt.Sprintf("%s-cap-%02d", tag, position), now); err != nil {
			t.Fatalf("seed capacity asset %d: %v", position, err)
		}
		for i, variant := range []string{"display", "thumbnail"} {
			if _, err = pool.Exec(ctx, `INSERT INTO public.media_variant(media_id,variant,provider,bucket_name,object_key,generation,mime_type,size_bytes,sha256,width,height) VALUES($1,$2,'gcs','synthetic-bucket',$3,$4,'image/webp',128,repeat('c',64),320,240)`, assetID, variant, fmt.Sprintf("synthetic/%s/%s.webp", assetID, variant), i+1); err != nil {
				t.Fatalf("seed capacity variant %d/%s: %v", position, variant, err)
			}
		}
		if _, err = pool.Exec(ctx, `INSERT INTO public.space_media(id,espacio_id,propietario_id,media_id,alt_text,visible,created_at) VALUES($1,$2,$3,$1,'Synthetic capacity slot',true,$4)`, assetID, spaceID, owner, now); err != nil {
			t.Fatalf("seed capacity slot %d: %v", position, err)
		}
		if _, err = pool.Exec(ctx, `INSERT INTO public.gallery_item_order(id,espacio_id,propietario_id,space_media_id,position) VALUES(gen_random_uuid(),$1,$2,$3,$4)`, spaceID, owner, assetID, position); err != nil {
			t.Fatalf("seed capacity order %d: %v", position, err)
		}
	}
	var occupied int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM public.media_asset WHERE espacio_id=$1 AND proposito='espacio' AND estado IN ('pending','ready')`, spaceID).Scan(&occupied); err != nil || occupied != media.MaxSpaceMedia-1 {
		t.Fatalf("pre-race active assets=%d err=%v, want %d", occupied, err, media.MaxSpaceMedia-1)
	}
	type reserveOutcome struct {
		id          string
		reservation media.Reservation
		err         error
	}
	candidateIDs := []string{newID(), newID()}
	start := make(chan struct{})
	outcomes := make(chan reserveOutcome, len(candidateIDs))
	for i, candidateID := range candidateIDs {
		go func(i int, id string) {
			<-start
			reserved, _, reserveErr := repo.Reserve(ctx, id, owner, spaceID, media.PurposeSpace, fmt.Sprintf("%s-concurrent-%d", tag, i), fmt.Sprintf("%064d", i+3), now, lease, media.ReservationOptions{})
			outcomes <- reserveOutcome{id: id, reservation: reserved, err: reserveErr}
		}(i, candidateID)
	}
	close(start)
	var winner reserveOutcome
	successes, limits := 0, 0
	for range candidateIDs {
		outcome := <-outcomes
		if outcome.err == nil {
			successes++
			winner = outcome
		} else if errors.Is(outcome.err, media.ErrLimit) {
			limits++
		} else {
			t.Fatalf("concurrent capacity reservation %s: %v", outcome.id, outcome.err)
		}
	}
	if successes != 1 || limits != 1 {
		t.Fatalf("concurrent capacity results successes=%d limits=%d, want 1 each", successes, limits)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM public.media_asset WHERE espacio_id=$1 AND proposito='espacio' AND estado IN ('pending','ready')`, spaceID).Scan(&occupied); err != nil || occupied != media.MaxSpaceMedia {
		t.Fatalf("post-race active assets=%d err=%v, want %d", occupied, err, media.MaxSpaceMedia)
	}
	for i, variant := range []string{"display", "thumbnail"} {
		if _, err = pool.Exec(ctx, `INSERT INTO public.media_variant(media_id,variant,provider,bucket_name,object_key,generation,mime_type,size_bytes,sha256,width,height) VALUES($1,$2,'gcs','synthetic-bucket',$3,$4,'image/webp',128,repeat('d',64),320,240)`, winner.id, variant, fmt.Sprintf("synthetic/%s/%s.webp", winner.id, variant), i+1); err != nil {
			t.Fatalf("insert winning capacity variant %s: %v", variant, err)
		}
	}
	capacityPhoto, err := repo.FinalizeSpace(ctx, winner.id, winner.reservation.Token, owner, spaceID, "", time.Now().UTC())
	if err != nil || capacityPhoto.Position != media.MaxSpaceMedia || capacityPhoto.State != "ready" {
		t.Fatalf("finalize 30th slot=%+v err=%v", capacityPhoto, err)
	}
	var orderedSlots int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM public.gallery_item_order WHERE espacio_id=$1`, spaceID).Scan(&orderedSlots); err != nil || orderedSlots != media.MaxSpaceMedia {
		t.Fatalf("ordered slots=%d err=%v, want %d", orderedSlots, err, media.MaxSpaceMedia)
	}
	limitID, raw31ID := newID(), newID()
	if _, _, err = repo.Reserve(ctx, limitID, owner, spaceID, media.PurposeSpace, tag+"-cap-31", fmt.Sprintf("%064d", 31), now, lease, media.ReservationOptions{}); !errors.Is(err, media.ErrLimit) {
		t.Fatalf("31st application reservation error=%v, want ErrLimit", err)
	}
	_, directErr := pool.Exec(ctx, `INSERT INTO public.media_asset(id,propietario_id,espacio_id,proposito,estado,idempotency_key,input_sha256,created_at,updated_at) VALUES($1,$2,$3,'espacio','ready',$4,repeat('e',64),$5,$5)`, raw31ID, owner, spaceID, tag+"-raw-31", now)
	var pgErr *pgconn.PgError
	if !errors.As(directErr, &pgErr) || pgErr.Code != "23514" || pgErr.Message != "space media limit of 30 reached" {
		t.Fatalf("31st direct DB insert error=%v, want gallery-capacity check violation", directErr)
	}
	var raw31Count int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM public.media_asset WHERE id=$1`, raw31ID).Scan(&raw31Count); err != nil || raw31Count != 0 {
		t.Fatalf("rejected 31st asset rows=%d err=%v, want 0", raw31Count, err)
	}

	// Use fresh synthetic fixtures for stale-CAS: Reserve must reject before
	// inserting the candidate or mutating the target slot/order.
	staleTag := tag + "-stale"
	if _, err = pool.Exec(ctx, `INSERT INTO public.espacio(id,propietario_id,categoria_codigo,titulo,descripcion,superficie_m2,capacidad_maxima,reglas_uso,modalidad_tarifa,precio_base_clp,direccion,estado,actualizado_en) VALUES($1,$2,'oficina','Stale CAS synthetic',repeat('Synthetic description. ',6),20,4,'Synthetic rules','dia',9000,'Synthetic address','borrador',$3)`, staleSpace, owner, now); err != nil {
		t.Fatal(err)
	}
	for _, a := range []struct {
		id, state string
		until     any
	}{{staleOld, "ready", nil}} {
		if _, err = pool.Exec(ctx, `INSERT INTO public.media_asset(id,propietario_id,espacio_id,proposito,estado,idempotency_key,input_sha256,fencing_token,lease_until,created_at,updated_at) VALUES($1,$2,$3,'espacio',$4,$5,repeat('c',64),1,$6,$7,$7)`, a.id, owner, staleSpace, a.state, staleTag+"-"+a.state, a.until, now); err != nil {
			t.Fatal(err)
		}
		for i, v := range []string{"display", "thumbnail"} {
			if _, err = pool.Exec(ctx, `INSERT INTO public.media_variant(media_id,variant,provider,bucket_name,object_key,generation,mime_type,size_bytes,sha256,width,height) VALUES($1,$2,'gcs','synthetic-bucket',$3,$4,'image/webp',128,repeat('d',64),320,240)`, a.id, v, fmt.Sprintf("synthetic/%s/%s.webp", a.id, v), i+1); err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err = pool.Exec(ctx, `INSERT INTO public.space_media(id,espacio_id,propietario_id,media_id,alt_text,visible,created_at) VALUES($1,$2,$3,$1,'stale inherited alt',true,$4)`, staleOld, staleSpace, owner, now); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO public.gallery_item_order(id,espacio_id,propietario_id,space_media_id,position) VALUES($1,$2,$3,$4,1)`, staleOrder, staleSpace, owner, staleOld); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `UPDATE public.espacio SET portada_media_id=$2 WHERE id=$1`, staleSpace, staleOld); err != nil {
		t.Fatal(err)
	}
	var staleRevision time.Time
	if err = pool.QueryRow(ctx, `SELECT actualizado_en FROM public.espacio WHERE id=$1`, staleSpace).Scan(&staleRevision); err != nil {
		t.Fatal(err)
	}
	staleExpected := staleRevision.UTC().Format(time.RFC3339Nano)
	if _, err = pool.Exec(ctx, `UPDATE public.espacio SET actualizado_en=actualizado_en+interval '1 second' WHERE id=$1`, staleSpace); err != nil {
		t.Fatal(err)
	}
	_, _, err = repo.Reserve(ctx, staleNew, owner, staleSpace, media.PurposeSpace, staleTag+"-request", fmt.Sprintf("%064d", 2), now, lease, media.ReservationOptions{ReplacementTargetID: staleOld, ExpectedRevision: staleExpected})
	if !errors.Is(err, media.ErrPrecondition) {
		t.Fatalf("stale CAS error=%v, want ErrPrecondition", err)
	}
	var staleOldState string
	if err = pool.QueryRow(ctx, `SELECT estado FROM public.media_asset WHERE id=$1`, staleOld).Scan(&staleOldState); err != nil {
		t.Fatal(err)
	}
	if staleOldState != "ready" {
		t.Fatalf("stale CAS changed old asset state: got %q, want ready", staleOldState)
	}
	var staleNewCount int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM public.media_asset WHERE id=$1`, staleNew).Scan(&staleNewCount); err != nil {
		t.Fatal(err)
	}
	if staleNewCount != 0 {
		t.Fatalf("stale CAS inserted candidate asset: rows=%d, want 0", staleNewCount)
	}
	var staleRetired *time.Time
	var staleVisible bool
	if err = pool.QueryRow(ctx, `SELECT retired_at,visible FROM public.space_media WHERE id=$1`, staleOld).Scan(&staleRetired, &staleVisible); err != nil {
		t.Fatal(err)
	}
	if staleRetired != nil || !staleVisible {
		t.Fatalf("stale CAS changed old slot: retired_at=%v visible=%t, want active and visible", staleRetired, staleVisible)
	}
	var staleGotOrder, staleGotSlot string
	var stalePosition int
	if err = pool.QueryRow(ctx, `SELECT id::text,space_media_id::text,position FROM public.gallery_item_order WHERE espacio_id=$1`, staleSpace).Scan(&staleGotOrder, &staleGotSlot, &stalePosition); err != nil {
		t.Fatal(err)
	}
	if staleGotOrder != staleOrder || staleGotSlot != staleOld || stalePosition != 1 {
		t.Fatalf("stale CAS changed order identity/target/position: id=%s slot=%s position=%d", staleGotOrder, staleGotSlot, stalePosition)
	}
	var cover *string
	if err = pool.QueryRow(ctx, `SELECT portada_media_id::text FROM public.espacio WHERE id=$1`, staleSpace).Scan(&cover); err != nil {
		t.Fatal(err)
	}
	if cover == nil || *cover != staleOld {
		t.Fatalf("stale CAS changed cover: got %v, want %s", cover, staleOld)
	}
}
