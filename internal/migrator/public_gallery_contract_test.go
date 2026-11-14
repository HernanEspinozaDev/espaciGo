package migrator

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	mediapg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/media"
	catalogpg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/publiccatalog"
	"github.com/HernanEspinozaDev/espaciGo/internal/media"
	"github.com/HernanEspinozaDev/espaciGo/internal/publiccatalog"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Called only by the existing disposable, network-none full-schema CI harness.
// Metadata and repository integration only: this does not upload objects or
// exercise the browser, GCS, or the shared application database.
func assertRuntimePublicGallerySync(t *testing.T, ctx context.Context, admin *pgx.Conn, pool *pgxpool.Pool) {
	t.Helper()
	owner, spaceID := uuid.NewString(), uuid.NewString()
	mustExec := func(query string, args ...any) {
		t.Helper()
		if _, err := admin.Exec(ctx, query, args...); err != nil {
			t.Fatal(err)
		}
	}
	// This fixture owns precisely one new account and space in an otherwise
	// empty disposable DB. Remove its rows before the harness's zero-account
	// postcheck, even if an assertion fails. Never touch an existing fixture.
	defer func() {
		cleanupCtx := context.Background()
		for _, query := range []string{
			`UPDATE public.espacio SET portada_media_id=NULL WHERE id=$1`,
			`DELETE FROM public.gallery_item_order WHERE espacio_id=$1`,
			`DELETE FROM public.media_variant_intent WHERE media_id IN (SELECT id FROM public.media_asset WHERE espacio_id=$1)`,
			`DELETE FROM public.media_variant WHERE media_id IN (SELECT id FROM public.media_asset WHERE espacio_id=$1)`,
			// Remove the replacement before its referenced retired slot.
			`DELETE FROM public.space_media sm WHERE sm.espacio_id=$1 AND NOT EXISTS(SELECT 1 FROM public.media_asset a WHERE a.replacement_target_space_media_id=sm.id)`,
			`DELETE FROM public.media_asset a WHERE a.espacio_id=$1 AND NOT EXISTS(SELECT 1 FROM public.space_media sm WHERE sm.media_id=a.id)`,
			`DELETE FROM public.space_media WHERE espacio_id=$1`,
			`DELETE FROM public.media_asset WHERE espacio_id=$1`,
			`DELETE FROM public.tarifa_espacio WHERE espacio_id=$1`,
			`DELETE FROM public.espacio WHERE id=$1`,
		} {
			if _, err := admin.Exec(cleanupCtx, query, spaceID); err != nil {
				t.Errorf("remove owned public-gallery fixture: %v", err)
				return
			}
		}
		if _, err := admin.Exec(cleanupCtx, `DELETE FROM public.usuario WHERE id=$1`, owner); err != nil {
			t.Errorf("remove owned gallery account: %v", err)
		}
	}()
	mustExec(`INSERT INTO public.usuario(id,correo_original,correo_normalizado,hash_clave,estado) VALUES($1,$2,$2,'synthetic-only','activo')`, owner, owner+"@gallery-ci.invalid")
	mustExec(`INSERT INTO public.espacio(id,propietario_id,categoria_codigo,titulo,descripcion,superficie_m2,capacidad_maxima,reglas_uso,modalidad_tarifa,precio_base_clp,direccion,estado,zona_horaria)
		VALUES($1,$2,'oficina','Public gallery CI',repeat('Descripción sintética para la galería. ',4),10,1,'Solo ensayo','hora',6000,'Dirección sintética','activa','UTC')`, spaceID, owner)
	mustExec(`INSERT INTO public.tarifa_espacio(espacio_id,version,modalidad,precio_base_clp) VALUES($1,1,'hora',6000)`, spaceID)

	mediaRepo, catalogRepo := mediapg.New(pool), catalogpg.New(pool)
	uploadMetadata := func(options media.ReservationOptions) media.Photo {
		t.Helper()
		id := uuid.NewString()
		now := time.Now().UTC()
		reservation, reused, err := mediaRepo.Reserve(ctx, id, owner, spaceID, media.PurposeSpace, "gallery-ci-"+id, strings.Repeat("a", 64), now, now.Add(time.Minute), options)
		if err != nil || reused {
			t.Fatalf("reserve new gallery photo: reused=%v err=%v", reused, err)
		}
		for _, name := range []media.VariantName{media.VariantDisplay, media.VariantThumbnail} {
			key := "media/v1/spaces/" + spaceID + "/" + id + "/1/" + string(name) + ".webp"
			if err := mediaRepo.PlanVariant(ctx, id, reservation.Token, name, key); err != nil {
				t.Fatal(err)
			}
			variant := media.Variant{Name: name, Provider: "gcs", Bucket: "synthetic-gallery-ci", ObjectKey: key, Generation: 1, MIME: "image/webp", Size: 12, SHA256: strings.Repeat("b", 64), Width: 10, Height: 10}
			if err := mediaRepo.RecordVariant(ctx, id, reservation.Token, variant); err != nil {
				t.Fatal(err)
			}
		}
		photo, err := mediaRepo.FinalizeSpace(ctx, id, reservation.Token, owner, spaceID, options.ExpectedRevision, now)
		if err != nil {
			t.Fatalf("finalize gallery photo: %v", err)
		}
		return photo
	}
	first := uploadMetadata(media.ReservationOptions{})
	if !first.Visible || first.State != "ready" {
		t.Fatalf("new publication gallery photo must be ready and visible: %+v", first)
	}
	// Neither nullable cover column is assigned: detail must not fail scanning
	// NULL into a bool, nor require a separate publication for this new photo.
	detail, err := catalogRepo.Detail(ctx, spaceID)
	if err != nil || len(detail.Images) != 1 || detail.Images[0].ImageID != first.ID || detail.Images[0].Source != "media" || detail.Images[0].IsCover {
		t.Fatalf("public detail immediately after upload without cover: images=%+v err=%v", detail.Images, err)
	}
	if _, err := mediaRepo.SelectSpaceCoverCAS(ctx, owner, spaceID, first.ID, first.SpaceRevision); err != nil {
		t.Fatal(err)
	}
	items, err := catalogRepo.List(ctx, publiccatalog.Filters{}, nil, 48)
	if err != nil || len(items) != 1 || items[0].SpaceID != spaceID || items[0].CoverImageID == nil || *items[0].CoverImageID != first.ID || len(items[0].Images) != 1 || items[0].Images[0].Source != "media" || !items[0].Images[0].IsCover {
		t.Fatalf("public card must retain the modern-media discriminator: items=%+v err=%v", items, err)
	}
	second := uploadMetadata(media.ReservationOptions{})
	detail, err = catalogRepo.Detail(ctx, spaceID)
	if err != nil || len(detail.Images) != 2 || detail.Images[0].ImageID != first.ID || !detail.Images[0].IsCover || detail.Images[1].ImageID != second.ID || detail.Images[1].IsCover {
		t.Fatalf("adding a photo to an active listing must update its public gallery immediately: images=%+v err=%v", detail.Images, err)
	}
	variant, err := catalogRepo.PublicMediaVariant(ctx, spaceID, second.ID, "display")
	if err != nil || variant.Generation != 1 || variant.MIME != "image/webp" {
		t.Fatalf("public variant after upload: %+v err=%v", variant, err)
	}
	// An explicit per-photo withdrawal remains authoritative. Replacing a
	// withdrawn photo must inherit false rather than make it public again.
	hidden := false
	_, revision, err := mediaRepo.UpdateSpacePhotoCAS(ctx, owner, spaceID, second.ID, nil, &hidden, nil, second.SpaceRevision, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	replacement := uploadMetadata(media.ReservationOptions{ReplacementTargetID: second.ID, ExpectedRevision: revision.UTC().Format(time.RFC3339Nano)})
	if replacement.Visible {
		t.Fatal("replacement must preserve an explicit visibility withdrawal")
	}
	if _, err := catalogRepo.PublicMediaVariant(ctx, spaceID, replacement.ID, "display"); !errors.Is(err, publiccatalog.ErrNotFound) {
		t.Fatalf("withdrawn replacement must remain private: %v", err)
	}
	for _, state := range []string{"borrador", "oculta"} {
		mustExec(`UPDATE public.espacio SET estado=$2 WHERE id=$1`, spaceID, state)
		if _, err := catalogRepo.Detail(ctx, spaceID); !errors.Is(err, publiccatalog.ErrNotFound) {
			t.Fatalf("%s listing detail must remain private: %v", state, err)
		}
		if _, err := catalogRepo.PublicMediaVariant(ctx, spaceID, first.ID, "display"); !errors.Is(err, publiccatalog.ErrNotFound) {
			t.Fatalf("%s listing image must remain private: %v", state, err)
		}
		own, err := mediaRepo.ListSpace(ctx, owner, spaceID)
		if err != nil || len(own) != 2 {
			t.Fatalf("owner gallery remains available while %s: %+v err=%v", state, own, err)
		}
	}
}
