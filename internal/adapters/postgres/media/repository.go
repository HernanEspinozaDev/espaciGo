package mediapg

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/accountlock"
	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/spaceversion"
	"github.com/HernanEspinozaDev/espaciGo/internal/media"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func (r *Repository) PlanVariant(ctx context.Context, id string, token int64, name media.VariantName, key string) error {
	cmd, err := r.pool.Exec(ctx, `INSERT INTO public.media_variant_intent(media_id,variant,object_key)
		SELECT id,$3,$4 FROM public.media_asset WHERE id=$1 AND fencing_token=$2 AND estado='pending'
		ON CONFLICT(media_id,variant) DO UPDATE SET object_key=EXCLUDED.object_key,provider=NULL,bucket_name=NULL,generation=NULL,updated_at=clock_timestamp()`, id, token, string(name), key)
	if err != nil {
		return mapError(err)
	}
	if cmd.RowsAffected() != 1 {
		return media.ErrConflict
	}
	return nil
}

func (r *Repository) Reserve(ctx context.Context, assetID, owner, spaceID string, purpose media.Purpose, idempotencyKey, inputHash string, at, leaseUntil time.Time) (media.Reservation, bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return media.Reservation{}, false, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	active, err := accountlock.LockActive(ctx, tx, owner)
	if err != nil {
		return media.Reservation{}, false, err
	}
	if !active {
		return media.Reservation{}, false, media.ErrNotFound
	}
	var scope any
	if purpose == media.PurposeSpace {
		scope = spaceID
		var ok bool
		if err = tx.QueryRow(ctx, `SELECT true FROM public.espacio WHERE id=$1 AND propietario_id=$2 AND estado<>'eliminada' FOR UPDATE`, spaceID, owner).Scan(&ok); errors.Is(err, pgx.ErrNoRows) {
			return media.Reservation{}, false, media.ErrNotFound
		} else if err != nil {
			return media.Reservation{}, false, err
		}
	}
	var prior media.Reservation
	err = tx.QueryRow(ctx, `SELECT id::text,propietario_id::text,COALESCE(espacio_id::text,''),proposito,estado,fencing_token,COALESCE(lease_until,'-infinity'::timestamptz),input_sha256
		FROM public.media_asset WHERE propietario_id=$1 AND proposito=$2 AND espacio_id IS NOT DISTINCT FROM $3::uuid AND idempotency_key=$4 FOR UPDATE`, owner, string(purpose), scope, idempotencyKey).Scan(&prior.ID, &prior.OwnerID, &prior.SpaceID, &prior.Purpose, &prior.State, &prior.Token, &prior.LeaseTill, &prior.InputHash)
	if err == nil {
		if prior.InputHash != inputHash {
			return media.Reservation{}, false, media.ErrConflict
		}
		if prior.State == "ready" {
			if err = tx.Commit(ctx); err != nil {
				return media.Reservation{}, false, err
			}
			return prior, true, nil
		}
		if prior.State != "pending" {
			return media.Reservation{}, false, media.ErrConflict
		}
		if prior.LeaseTill.After(at.UTC()) {
			if err = tx.Commit(ctx); err != nil {
				return media.Reservation{}, false, err
			}
			return prior, true, nil
		}
		var hasArtifacts bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.media_variant WHERE media_id=$1) OR EXISTS(SELECT 1 FROM public.media_variant_intent WHERE media_id=$1)`, prior.ID).Scan(&hasArtifacts); err != nil {
			return media.Reservation{}, false, err
		}
		if hasArtifacts {
			return media.Reservation{}, false, media.ErrConflict
		}
		prior.Token++
		_, err = tx.Exec(ctx, `UPDATE public.media_asset SET fencing_token=$2,lease_until=$3,updated_at=$4 WHERE id=$1`, prior.ID, prior.Token, leaseUntil, at.UTC())
		if err != nil {
			return media.Reservation{}, false, mapError(err)
		}
		prior.LeaseTill = leaseUntil
		if err = tx.Commit(ctx); err != nil {
			return media.Reservation{}, false, err
		}
		return prior, false, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return media.Reservation{}, false, err
	}
	var spaceArg any
	if purpose == media.PurposeSpace {
		spaceArg = spaceID
	}
	_, err = tx.Exec(ctx, `INSERT INTO public.media_asset(id,propietario_id,espacio_id,proposito,estado,idempotency_key,input_sha256,lease_until)
		VALUES($1,$2,$3,$4,'pending',$5,$6,$7)`, assetID, owner, spaceArg, string(purpose), idempotencyKey, inputHash, leaseUntil)
	if err != nil {
		return media.Reservation{}, false, mapError(err)
	}
	if err = tx.Commit(ctx); err != nil {
		return media.Reservation{}, false, err
	}
	return media.Reservation{ID: assetID, OwnerID: owner, SpaceID: spaceID, Purpose: purpose, Token: 1, LeaseTill: leaseUntil, State: "pending", InputHash: inputHash}, false, nil
}

func (r *Repository) RecordVariant(ctx context.Context, id string, token int64, v media.Variant) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	intent, err := tx.Exec(ctx, `UPDATE public.media_variant_intent i SET provider=$4,bucket_name=$5,generation=$6,updated_at=clock_timestamp()
		FROM public.media_asset a WHERE i.media_id=$1 AND i.variant=$3 AND i.object_key=$7 AND a.id=i.media_id AND a.fencing_token=$2 AND a.estado='pending'`, id, token, string(v.Name), v.Provider, v.Bucket, v.Generation, v.ObjectKey)
	if err != nil {
		return mapError(err)
	}
	if intent.RowsAffected() != 1 {
		return media.ErrConflict
	}
	cmd, err := tx.Exec(ctx, `INSERT INTO public.media_variant(media_id,variant,provider,bucket_name,object_key,generation,mime_type,size_bytes,sha256,width,height)
		SELECT id,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12 FROM public.media_asset WHERE id=$1 AND fencing_token=$2 AND estado='pending'`, id, token, string(v.Name), v.Provider, v.Bucket, v.ObjectKey, v.Generation, v.MIME, v.Size, v.SHA256, v.Width, v.Height)
	if err != nil {
		return mapError(err)
	}
	if cmd.RowsAffected() != 1 {
		return media.ErrConflict
	}
	if _, err = tx.Exec(ctx, `DELETE FROM public.media_variant_intent WHERE media_id=$1 AND variant=$2`, id, string(v.Name)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *Repository) FinalizeSpace(ctx context.Context, id string, token int64, owner, spaceID string, at time.Time) (media.Photo, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return media.Photo{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	active, err := accountlock.LockActive(ctx, tx, owner)
	if err != nil {
		return media.Photo{}, err
	}
	if !active {
		return media.Photo{}, media.ErrNotFound
	}
	var exists bool
	if err = tx.QueryRow(ctx, `SELECT true FROM public.espacio WHERE id=$1 AND propietario_id=$2 AND estado<>'eliminada' FOR UPDATE`, spaceID, owner).Scan(&exists); errors.Is(err, pgx.ErrNoRows) {
		return media.Photo{}, media.ErrNotFound
	} else if err != nil {
		return media.Photo{}, err
	}
	var state string
	err = tx.QueryRow(ctx, `SELECT estado FROM public.media_asset WHERE id=$1 AND propietario_id=$2 AND espacio_id=$3 AND proposito='espacio' AND fencing_token=$4 AND lease_until>clock_timestamp() FOR UPDATE`, id, owner, spaceID, token).Scan(&state)
	if errors.Is(err, pgx.ErrNoRows) || state != "pending" {
		return media.Photo{}, media.ErrConflict
	}
	if err != nil {
		return media.Photo{}, err
	}
	var variants int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM public.media_variant WHERE media_id=$1 AND variant IN ('display','thumbnail')`, id).Scan(&variants); err != nil {
		return media.Photo{}, err
	}
	if variants != 2 {
		return media.Photo{}, media.ErrConflict
	}
	var position int
	if err = tx.QueryRow(ctx, `SELECT COALESCE(max(position),0)+1 FROM public.gallery_item_order WHERE espacio_id=$1`, spaceID).Scan(&position); err != nil {
		return media.Photo{}, err
	}
	if position > media.MaxSpaceMedia {
		return media.Photo{}, media.ErrLimit
	}
	_, err = tx.Exec(ctx, `INSERT INTO public.space_media(id,espacio_id,propietario_id,media_id,created_at) VALUES($1,$2,$3,$1,$4)`, id, spaceID, owner, at)
	if err != nil {
		return media.Photo{}, mapError(err)
	}
	_, err = tx.Exec(ctx, `INSERT INTO public.gallery_item_order(id,espacio_id,propietario_id,space_media_id,position) VALUES(gen_random_uuid(),$1,$2,$3,$4)`, spaceID, owner, id, position)
	if err != nil {
		return media.Photo{}, mapError(err)
	}
	_, err = tx.Exec(ctx, `UPDATE public.media_asset SET estado='ready',updated_at=$3,lease_until=NULL WHERE id=$1 AND fencing_token=$2`, id, token, at)
	if err != nil {
		return media.Photo{}, mapError(err)
	}
	if _, err = spaceversion.Bump(ctx, tx, spaceID); err != nil {
		return media.Photo{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return media.Photo{}, err
	}
	return r.GetSpace(ctx, owner, spaceID, id)
}

func (r *Repository) FinalizeProfile(ctx context.Context, owner, id string, token int64, at time.Time) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	active, err := accountlock.LockActive(ctx, tx, owner)
	if err != nil {
		return err
	}
	if !active {
		return media.ErrNotFound
	}
	var state string
	err = tx.QueryRow(ctx, `SELECT estado FROM public.media_asset WHERE id=$1 AND propietario_id=$2 AND espacio_id IS NULL AND proposito='perfil' AND fencing_token=$3 AND lease_until>clock_timestamp() FOR UPDATE`, id, owner, token).Scan(&state)
	if errors.Is(err, pgx.ErrNoRows) || state != "pending" {
		return media.ErrConflict
	}
	if err != nil {
		return err
	}
	var exists bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.media_variant WHERE media_id=$1 AND variant='avatar')`, id).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return media.ErrConflict
	}
	var previous string
	err = tx.QueryRow(ctx, `SELECT media_id::text FROM public.profile_media WHERE propietario_id=$1 FOR UPDATE`, owner).Scan(&previous)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if previous != "" {
		_, err = tx.Exec(ctx, `UPDATE public.media_asset SET estado='deleting',updated_at=$2,lease_until=$2 WHERE id=$1 AND estado='ready'`, previous, at)
		if err != nil {
			return err
		}
	}
	_, err = tx.Exec(ctx, `INSERT INTO public.profile_media(propietario_id,media_id,updated_at) VALUES($1,$2,$3)
		ON CONFLICT(propietario_id) DO UPDATE SET media_id=EXCLUDED.media_id,updated_at=EXCLUDED.updated_at`, owner, id, at)
	if err != nil {
		return mapError(err)
	}
	_, err = tx.Exec(ctx, `UPDATE public.media_asset SET estado='ready',updated_at=$3,lease_until=NULL WHERE id=$1 AND fencing_token=$2`, id, token, at)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *Repository) FailReservation(ctx context.Context, id string, token int64, at time.Time) error {
	_, err := r.pool.Exec(ctx, `UPDATE public.media_asset SET estado=CASE WHEN EXISTS(SELECT 1 FROM public.media_variant v WHERE v.media_id=media_asset.id) OR EXISTS(SELECT 1 FROM public.media_variant_intent i WHERE i.media_id=media_asset.id) THEN 'deleting' ELSE 'failed' END,
		lease_until=$3,updated_at=$3 WHERE id=$1 AND fencing_token=$2 AND estado='pending'`, id, token, at)
	return err
}

func (r *Repository) ListSpace(ctx context.Context, owner, spaceID string) ([]media.Photo, error) {
	rows, err := r.pool.Query(ctx, `SELECT a.id::text,o.position,sm.alt_text,sm.visible,(e.portada_media_id=sm.id) AS cover,a.created_at,a.estado
		FROM public.space_media sm JOIN public.media_asset a ON a.id=sm.media_id JOIN public.espacio e ON e.id=sm.espacio_id
		LEFT JOIN public.gallery_item_order o ON o.space_media_id=sm.id
		WHERE sm.espacio_id=$1 AND sm.propietario_id=$2 AND sm.retired_at IS NULL AND a.estado='ready'
		ORDER BY o.position,sm.id`, spaceID, owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []media.Photo{}
	for rows.Next() {
		var p media.Photo
		p.OwnerID = owner
		p.SpaceID = spaceID
		if err = rows.Scan(&p.ID, &p.Position, &p.AltText, &p.Visible, &p.Cover, &p.CreatedAt, &p.State); err != nil {
			return nil, err
		}
		p.Variants, err = r.variants(ctx, p.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *Repository) GetSpace(ctx context.Context, owner, spaceID, id string) (media.Photo, error) {
	var p media.Photo
	p.OwnerID = owner
	p.SpaceID = spaceID
	err := r.pool.QueryRow(ctx, `SELECT a.id::text,o.position,sm.alt_text,sm.visible,(e.portada_media_id=sm.id),a.created_at,a.estado
		FROM public.space_media sm JOIN public.media_asset a ON a.id=sm.media_id JOIN public.espacio e ON e.id=sm.espacio_id
		JOIN public.gallery_item_order o ON o.space_media_id=sm.id
		WHERE sm.id=$1 AND sm.espacio_id=$2 AND sm.propietario_id=$3 AND sm.retired_at IS NULL AND a.estado='ready'`, id, spaceID, owner).
		Scan(&p.ID, &p.Position, &p.AltText, &p.Visible, &p.Cover, &p.CreatedAt, &p.State)
	if errors.Is(err, pgx.ErrNoRows) {
		return media.Photo{}, media.ErrNotFound
	}
	if err != nil {
		return media.Photo{}, err
	}
	p.Variants, err = r.variants(ctx, p.ID)
	return p, err
}

func (r *Repository) UpdateSpacePhoto(ctx context.Context, owner, spaceID, id string, alt *string, visible *bool, position *int, at time.Time) (media.Photo, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return media.Photo{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	active, err := accountlock.LockActive(ctx, tx, owner)
	if err != nil {
		return media.Photo{}, err
	}
	if !active {
		return media.Photo{}, media.ErrNotFound
	}
	var ok bool
	if err = tx.QueryRow(ctx, `SELECT true FROM public.espacio WHERE id=$1 AND propietario_id=$2 FOR UPDATE`, spaceID, owner).Scan(&ok); errors.Is(err, pgx.ErrNoRows) {
		return media.Photo{}, media.ErrNotFound
	} else if err != nil {
		return media.Photo{}, err
	}
	var old int
	if err = tx.QueryRow(ctx, `SELECT o.position FROM public.space_media sm JOIN public.gallery_item_order o ON o.space_media_id=sm.id WHERE sm.id=$1 AND sm.espacio_id=$2 AND sm.propietario_id=$3 AND sm.retired_at IS NULL FOR UPDATE OF sm,o`, id, spaceID, owner).Scan(&old); errors.Is(err, pgx.ErrNoRows) {
		return media.Photo{}, media.ErrNotFound
	} else if err != nil {
		return media.Photo{}, err
	}
	if alt != nil {
		_, err = tx.Exec(ctx, `UPDATE public.space_media SET alt_text=$4 WHERE id=$1 AND espacio_id=$2 AND propietario_id=$3`, id, spaceID, owner, *alt)
		if err != nil {
			return media.Photo{}, err
		}
	}
	if visible != nil {
		_, err = tx.Exec(ctx, `UPDATE public.space_media SET visible=$4 WHERE id=$1 AND espacio_id=$2 AND propietario_id=$3`, id, spaceID, owner, *visible)
		if err != nil {
			return media.Photo{}, err
		}
	}
	if position != nil && *position != old {
		var count int
		if err = tx.QueryRow(ctx, `SELECT count(*) FROM public.gallery_item_order WHERE espacio_id=$1`, spaceID).Scan(&count); err != nil {
			return media.Photo{}, err
		}
		if *position > count {
			return media.Photo{}, media.ErrInvalid
		}
		_, err = tx.Exec(ctx, `UPDATE public.gallery_item_order SET position=CASE
			WHEN space_media_id=$2 THEN $3
			WHEN $3<$4 AND position >= $3 AND position < $4 THEN position+1
			WHEN $3>$4 AND position > $4 AND position <= $3 THEN position-1
			ELSE position END WHERE espacio_id=$1`, spaceID, id, *position, old)
		if err != nil {
			return media.Photo{}, mapError(err)
		}
	}
	if _, err = spaceversion.Bump(ctx, tx, spaceID); err != nil {
		return media.Photo{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return media.Photo{}, err
	}
	return r.GetSpace(ctx, owner, spaceID, id)
}

func (r *Repository) RemoveSpacePhoto(ctx context.Context, owner, spaceID, id string, at time.Time) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	active, err := accountlock.LockActive(ctx, tx, owner)
	if err != nil {
		return err
	}
	if !active {
		return media.ErrNotFound
	}
	var ok bool
	if err = tx.QueryRow(ctx, `SELECT true FROM public.espacio WHERE id=$1 AND propietario_id=$2 FOR UPDATE`, spaceID, owner).Scan(&ok); errors.Is(err, pgx.ErrNoRows) {
		return media.ErrNotFound
	} else if err != nil {
		return err
	}
	var assetID string
	err = tx.QueryRow(ctx, `SELECT media_id::text FROM public.space_media WHERE id=$1 AND espacio_id=$2 AND propietario_id=$3 AND retired_at IS NULL FOR UPDATE`, id, spaceID, owner).Scan(&assetID)
	if errors.Is(err, pgx.ErrNoRows) {
		return media.ErrNotFound
	}
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE public.espacio SET portada_media_id=NULL WHERE id=$1 AND propietario_id=$2 AND portada_media_id=$3`, spaceID, owner, id); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM public.gallery_item_order WHERE space_media_id=$1`, id); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `WITH ranked AS (SELECT id,row_number() OVER (ORDER BY position,id)::smallint AS next_position FROM public.gallery_item_order WHERE espacio_id=$1) UPDATE public.gallery_item_order o SET position=ranked.next_position FROM ranked WHERE o.id=ranked.id`, spaceID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE public.space_media SET retired_at=$2,visible=false WHERE id=$1`, id, at); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE public.media_asset SET estado='deleting',updated_at=$2,lease_until=$2 WHERE id=$1 AND estado='ready'`, assetID, at); err != nil {
		return err
	}
	if _, err = spaceversion.Bump(ctx, tx, spaceID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *Repository) SelectSpaceCover(ctx context.Context, owner, spaceID, id string, at time.Time) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	active, err := accountlock.LockActive(ctx, tx, owner)
	if err != nil {
		return err
	}
	if !active {
		return media.ErrNotFound
	}
	var ok bool
	if err = tx.QueryRow(ctx, `SELECT true FROM public.espacio WHERE id=$1 AND propietario_id=$2 FOR UPDATE`, spaceID, owner).Scan(&ok); errors.Is(err, pgx.ErrNoRows) {
		return media.ErrNotFound
	} else if err != nil {
		return err
	}
	var state string
	err = tx.QueryRow(ctx, `SELECT a.estado FROM public.space_media sm JOIN public.media_asset a ON a.id=sm.media_id WHERE sm.id=$1 AND sm.espacio_id=$2 AND sm.propietario_id=$3 AND sm.retired_at IS NULL FOR UPDATE OF sm,a`, id, spaceID, owner).Scan(&state)
	if errors.Is(err, pgx.ErrNoRows) || state != "ready" {
		return media.ErrNotFound
	}
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE public.espacio SET portada_foto_id=NULL,portada_media_id=$3 WHERE id=$1 AND propietario_id=$2`, spaceID, owner, id); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE public.espacio SET actualizado_en=$3 WHERE id=$1 AND propietario_id=$2`, spaceID, owner, at)
	if err != nil {
		return err
	}
	if _, err = spaceversion.Bump(ctx, tx, spaceID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *Repository) GetVariant(ctx context.Context, owner, id, name string) (media.Variant, error) {
	var v media.Variant
	err := r.pool.QueryRow(ctx, `SELECT v.variant,v.provider,v.bucket_name,v.object_key,v.generation,v.mime_type,v.size_bytes,v.sha256,v.width,v.height
		FROM public.media_variant v JOIN public.media_asset a ON a.id=v.media_id JOIN public.space_media sm ON sm.media_id=a.id
		WHERE a.id=$1 AND a.propietario_id=$2 AND sm.retired_at IS NULL AND a.estado='ready' AND v.variant=$3`, id, owner, name).
		Scan(&v.Name, &v.Provider, &v.Bucket, &v.ObjectKey, &v.Generation, &v.MIME, &v.Size, &v.SHA256, &v.Width, &v.Height)
	if errors.Is(err, pgx.ErrNoRows) {
		return media.Variant{}, media.ErrNotFound
	}
	return v, err
}

func (r *Repository) GetProfilePhoto(ctx context.Context, owner string) (media.Photo, error) {
	var p media.Photo
	p.OwnerID = owner
	p.State = "ready"
	err := r.pool.QueryRow(ctx, `SELECT a.id::text,a.created_at FROM public.profile_media pm JOIN public.media_asset a ON a.id=pm.media_id WHERE pm.propietario_id=$1 AND a.estado='ready' AND a.proposito='perfil'`, owner).Scan(&p.ID, &p.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return media.Photo{}, media.ErrNotFound
	}
	if err != nil {
		return media.Photo{}, err
	}
	p.Variants, err = r.variants(ctx, p.ID)
	return p, err
}

func (r *Repository) DeleteProfilePhoto(ctx context.Context, owner string, at time.Time) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	active, err := accountlock.LockActive(ctx, tx, owner)
	if err != nil {
		return err
	}
	if !active {
		return media.ErrNotFound
	}
	var id string
	err = tx.QueryRow(ctx, `SELECT media_id::text FROM public.profile_media WHERE propietario_id=$1 FOR UPDATE`, owner).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return media.ErrNotFound
	}
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM public.profile_media WHERE propietario_id=$1`, owner); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE public.media_asset SET estado='deleting',updated_at=$2,lease_until=$2 WHERE id=$1 AND estado='ready'`, id, at); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *Repository) ClaimCleanup(ctx context.Context, limit int, at time.Time) ([]media.CleanupItem, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	rows, err := tx.Query(ctx, `WITH claimed AS (
		SELECT id FROM public.media_asset WHERE (estado='deleting' AND (lease_until IS NULL OR lease_until <= $2)) OR (estado='pending' AND lease_until <= $2)
		ORDER BY updated_at,id FOR UPDATE SKIP LOCKED LIMIT $1
	), changed AS (
		UPDATE public.media_asset a SET estado='deleting',fencing_token=a.fencing_token+1,lease_until=$2+interval '5 minutes',updated_at=$2
		FROM claimed c WHERE a.id=c.id RETURNING a.id::text,a.fencing_token
	) SELECT id,fencing_token FROM changed`, limit, at)
	if err != nil {
		return nil, err
	}
	type claim struct {
		id    string
		token int64
	}
	claims := []claim{}
	for rows.Next() {
		var c claim
		if err = rows.Scan(&c.id, &c.token); err != nil {
			rows.Close()
			return nil, err
		}
		claims = append(claims, c)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return nil, err
	}
	items := make([]media.CleanupItem, 0, len(claims))
	for _, c := range claims {
		vs, e := variantsFrom(ctx, tx, c.id)
		if e != nil && !errors.Is(e, pgx.ErrNoRows) {
			return nil, e
		}
		intents, e := intentsFrom(ctx, tx, c.id)
		if e != nil {
			return nil, e
		}
		items = append(items, media.CleanupItem{AssetID: c.id, Token: c.token, Variants: vs, Intents: intents})
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *Repository) CompleteCleanup(ctx context.Context, id string, token int64, at time.Time) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var ok bool
	if err = tx.QueryRow(ctx, `SELECT true FROM public.media_asset WHERE id=$1 AND estado='deleting' AND fencing_token=$2 FOR UPDATE`, id, token).Scan(&ok); errors.Is(err, pgx.ErrNoRows) {
		return media.ErrConflict
	}
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM public.media_variant WHERE media_id=$1`, id); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM public.media_variant_intent WHERE media_id=$1`, id); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE public.media_asset SET estado='deleted',deleted_at=$3,lease_until=NULL,updated_at=$3 WHERE id=$1 AND fencing_token=$2`, id, token, at); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *Repository) FailCleanup(ctx context.Context, id string, token int64, at time.Time) error {
	_, err := r.pool.Exec(ctx, `UPDATE public.media_asset SET lease_until=$3+interval '1 minute',updated_at=$3 WHERE id=$1 AND estado='deleting' AND fencing_token=$2`, id, token, at)
	return err
}

func (r *Repository) variants(ctx context.Context, id string) ([]media.Variant, error) {
	return variantsFrom(ctx, r.pool, id)
}

type queryer interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func variantsFrom(ctx context.Context, q queryer, id string) ([]media.Variant, error) {
	rows, err := q.Query(ctx, `SELECT variant,provider,bucket_name,object_key,generation,mime_type,size_bytes,sha256,width,height FROM public.media_variant WHERE media_id=$1 ORDER BY variant`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []media.Variant{}
	for rows.Next() {
		var v media.Variant
		if err = rows.Scan(&v.Name, &v.Provider, &v.Bucket, &v.ObjectKey, &v.Generation, &v.MIME, &v.Size, &v.SHA256, &v.Width, &v.Height); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (r *Repository) intents(ctx context.Context, id string) ([]media.ObjectIntent, error) {
	return intentsFrom(ctx, r.pool, id)
}

func intentsFrom(ctx context.Context, q queryer, id string) ([]media.ObjectIntent, error) {
	rows, err := q.Query(ctx, `SELECT variant,object_key,COALESCE(generation,0) FROM public.media_variant_intent WHERE media_id=$1 ORDER BY variant`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []media.ObjectIntent{}
	for rows.Next() {
		var item media.ObjectIntent
		if err = rows.Scan(&item.Name, &item.ObjectKey, &item.Generation); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func mapError(err error) error {
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		switch pg.Code {
		case "23514":
			if strings.Contains(pg.Message, "space media limit of 10 reached") {
				return media.ErrLimit
			}
			return media.ErrInvalid
		case "23505":
			return media.ErrConflict
		case "23503":
			return media.ErrNotFound
		}
	}
	return err
}
