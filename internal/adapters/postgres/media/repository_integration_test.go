package mediapg

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/media"
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
