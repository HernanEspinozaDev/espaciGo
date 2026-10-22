package mediapg

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/media"
	"github.com/jackc/pgx/v5/pgxpool"
)

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

func TestFailReservationMarksFinishedWriterIntentForCleanup(t *testing.T) {
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
		VALUES($1,$2,'perfil','pending','img01c-fail-01',repeat('0',64),clock_timestamp()+interval '1 minute')`, asset, owner)
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO public.media_variant_intent(media_id,variant,object_key) VALUES($1,'avatar',$2)`, asset, key)
	if err != nil {
		t.Fatal(err)
	}
	r := New(pool)
	if err = r.FailReservation(ctx, asset, 1, time.Now().UTC()); err != nil {
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
