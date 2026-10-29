package gallerypg

import (
	"context"
	"errors"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/credentials"
	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/accountlock"
	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/spaceversion"
	"github.com/HernanEspinozaDev/espaciGo/internal/gallery"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

const activePhotoColumns = `id::text,fixture_code,mime_type,sha256,size_bytes,creada_en,publicada_en_catalogo`

func (r *Repository) ReserveCandidate(ctx context.Context, owner, spaceID, fileID string, at time.Time) error {
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
		return gallery.ErrNotFound
	}
	var exists bool
	err = tx.QueryRow(ctx, `SELECT true FROM public.espacio WHERE id=$1 AND propietario_id=$2 AND estado<>'eliminada' FOR UPDATE`, spaceID, owner).Scan(&exists)
	if errors.Is(err, pgx.ErrNoRows) {
		return gallery.ErrNotFound
	}
	if err != nil {
		return err
	}
	var used bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.espacio_galeria_sintetica_local WHERE id=$1 OR archivo_id=$1)`, fileID).Scan(&used); err != nil {
		return err
	}
	if used {
		return gallery.ErrInvalid
	}
	_, err = tx.Exec(ctx, `INSERT INTO public.espacio_galeria_archivo_candidato_local(archivo_id,propietario_id,espacio_id,estado,creada_en)
		VALUES($1,$2,$3,'reservado',$4)`, fileID, owner, spaceID, at.UTC())
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *Repository) QueueCandidateCleanup(ctx context.Context, fileID string, at time.Time) error {
	_, err := r.pool.Exec(ctx, `UPDATE public.espacio_galeria_archivo_candidato_local
		SET estado='pendiente_limpieza',proximo_intento_en=$2,ultimo_codigo_error=CASE WHEN estado='reservado' THEN 'alta_no_confirmada' ELSE ultimo_codigo_error END
		WHERE archivo_id=$1 AND estado='reservado'`, fileID, at.UTC())
	return err
}

// BeginCandidate holds the candidate row lock across the external file write
// and the gallery insert. Cleanup workers use SKIP LOCKED, so an old-looking
// candidate cannot be deleted while its producer is still writing it.
func (r *Repository) BeginCandidate(ctx context.Context, owner, spaceID, fileID string) (gallery.CandidateWriter, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	failed := true
	defer func() {
		if failed {
			_ = tx.Rollback(context.Background())
		}
	}()
	active, err := accountlock.LockActive(ctx, tx, owner)
	if err != nil {
		return nil, err
	}
	if !active {
		return nil, gallery.ErrNotFound
	}
	var exists bool
	if err = tx.QueryRow(ctx, `SELECT true FROM public.espacio WHERE id=$1 AND propietario_id=$2 AND estado<>'eliminada' FOR UPDATE`, spaceID, owner).Scan(&exists); errors.Is(err, pgx.ErrNoRows) {
		return nil, gallery.ErrNotFound
	} else if err != nil {
		return nil, err
	}
	var state string
	if err = tx.QueryRow(ctx, `SELECT estado FROM public.espacio_galeria_archivo_candidato_local WHERE archivo_id=$1 AND propietario_id=$2 AND espacio_id=$3 FOR UPDATE`, fileID, owner, spaceID).Scan(&state); errors.Is(err, pgx.ErrNoRows) || (err == nil && state != "reservado") {
		return nil, gallery.ErrCandidateUnavailable
	} else if err != nil {
		return nil, err
	}
	failed = false
	return &candidateWriter{tx: tx, owner: owner, spaceID: spaceID, fileID: fileID}, nil
}

type candidateWriter struct {
	tx      pgx.Tx
	owner   string
	spaceID string
	fileID  string
	done    bool
}

func (w *candidateWriter) finish(ctx context.Context) error {
	if w.done {
		return nil
	}
	w.done = true
	return w.tx.Commit(ctx)
}

func (w *candidateWriter) Add(ctx context.Context, item gallery.Photo, key string, at time.Time) (gallery.Photo, bool, error) {
	if w.done || item.ID != w.fileID {
		return gallery.Photo{}, false, gallery.ErrCandidateUnavailable
	}
	prior, err := scanPhoto(w.tx.QueryRow(ctx, `SELECT `+activePhotoColumns+` FROM public.espacio_galeria_sintetica_local WHERE espacio_id=$1 AND propietario_id=$2 AND clave_idempotencia=$3`, w.spaceID, w.owner, key))
	if err == nil {
		_, err = w.tx.Exec(ctx, `UPDATE public.espacio_galeria_archivo_candidato_local SET estado='pendiente_limpieza',proximo_intento_en=$2,ultimo_codigo_error='alta_idempotente_reutilizada' WHERE archivo_id=$1 AND estado='reservado'`, w.fileID, at.UTC())
		if err == nil {
			err = w.finish(ctx)
		}
		return prior, true, err
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return gallery.Photo{}, false, err
	}
	var count int
	if err = w.tx.QueryRow(ctx, `SELECT count(*) FROM public.espacio_galeria_sintetica_local WHERE espacio_id=$1 AND propietario_id=$2 AND estado='activa'`, w.spaceID, w.owner).Scan(&count); err != nil {
		return gallery.Photo{}, false, err
	}
	if count >= gallery.MaxPhotosPerSpace {
		return gallery.Photo{}, false, gallery.ErrLimit
	}
	if _, err = w.tx.Exec(ctx, `INSERT INTO public.espacio_galeria_sintetica_local(id,espacio_id,propietario_id,archivo_id,fixture_code,mime_type,sha256,size_bytes,estado,creada_en,clave_idempotencia)
		VALUES($1,$2,$3,$1,$4,$5,$6,$7,'activa',$8,$9)`, item.ID, w.spaceID, w.owner, item.Fixture, item.MIME, item.SHA256, item.Size, item.CreatedAt.UTC(), key); err != nil {
		return gallery.Photo{}, false, err
	}
	var position int
	if err = w.tx.QueryRow(ctx, `SELECT COALESCE(max(position),0)+1 FROM public.gallery_item_order WHERE espacio_id=$1`, w.spaceID).Scan(&position); err != nil {
		return gallery.Photo{}, false, err
	}
	if _, err = w.tx.Exec(ctx, `INSERT INTO public.gallery_item_order(id,espacio_id,propietario_id,synthetic_photo_id,position) VALUES(gen_random_uuid(),$1,$2,$3,$4)`, w.spaceID, w.owner, item.ID, position); err != nil {
		return gallery.Photo{}, false, err
	}
	result, err := w.tx.Exec(ctx, `DELETE FROM public.espacio_galeria_archivo_candidato_local WHERE archivo_id=$1 AND estado='reservado'`, w.fileID)
	if err != nil {
		return gallery.Photo{}, false, err
	}
	if result.RowsAffected() != 1 {
		return gallery.Photo{}, false, gallery.ErrCandidateUnavailable
	}
	if _, err = spaceversion.Bump(ctx, w.tx, w.spaceID); err != nil {
		return gallery.Photo{}, false, err
	}
	if err = w.finish(ctx); err != nil {
		return gallery.Photo{}, false, err
	}
	return item, false, nil
}

func (w *candidateWriter) QueueCleanup(ctx context.Context, at time.Time) error {
	if w.done {
		return nil
	}
	_, err := w.tx.Exec(ctx, `UPDATE public.espacio_galeria_archivo_candidato_local SET estado='pendiente_limpieza',proximo_intento_en=$2,ultimo_codigo_error='alta_no_confirmada' WHERE archivo_id=$1 AND estado='reservado'`, w.fileID, at.UTC())
	if err != nil {
		return err
	}
	return w.finish(ctx)
}

func (w *candidateWriter) Close() error {
	if w.done {
		return nil
	}
	w.done = true
	err := w.tx.Rollback(context.Background())
	if errors.Is(err, pgx.ErrTxClosed) {
		return nil
	}
	return err
}

func (r *Repository) ClaimCandidateCleanup(ctx context.Context, limit int) ([]string, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	rows, err := tx.Query(ctx, `WITH claimable AS (
		SELECT archivo_id FROM public.espacio_galeria_archivo_candidato_local
		WHERE (estado='pendiente_limpieza' AND proximo_intento_en<=clock_timestamp())
		   OR (estado='reservado' AND creada_en<=clock_timestamp()-interval '1 minute')
		   OR (estado='limpiando' AND proximo_intento_en<=clock_timestamp())
		ORDER BY COALESCE(proximo_intento_en,creada_en),archivo_id
		FOR UPDATE SKIP LOCKED LIMIT $1
	)
	UPDATE public.espacio_galeria_archivo_candidato_local c
	SET estado='limpiando',proximo_intento_en=clock_timestamp()+interval '2 minutes'
	FROM claimable x WHERE c.archivo_id=x.archivo_id
	RETURNING c.archivo_id::text`, limit)
	if err != nil {
		return nil, err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return ids, nil
}

func (r *Repository) CompleteCandidateCleanup(ctx context.Context, fileID string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM public.espacio_galeria_archivo_candidato_local WHERE archivo_id=$1 AND estado='limpiando'`, fileID)
	return err
}

func (r *Repository) FailCandidateCleanup(ctx context.Context, fileID string, at time.Time) error {
	_, err := r.pool.Exec(ctx, `UPDATE public.espacio_galeria_archivo_candidato_local
		SET estado='pendiente_limpieza',intentos_limpieza=intentos_limpieza+1,
		proximo_intento_en=$2::timestamptz+LEAST(interval '5 minutes',interval '5 seconds'*(intentos_limpieza+1)),ultimo_codigo_error='archivo_sintetico_no_eliminado'
		WHERE archivo_id=$1 AND estado='limpiando'`, fileID, at.UTC())
	return err
}

func scanPhoto(row pgx.Row) (gallery.Photo, error) {
	var item gallery.Photo
	err := row.Scan(&item.ID, &item.Fixture, &item.MIME, &item.SHA256, &item.Size, &item.CreatedAt, &item.PublicInCatalog)
	return item, err
}

func (r *Repository) List(ctx context.Context, owner, spaceID string) ([]gallery.Photo, error) {
	var exists bool
	if err := r.pool.QueryRow(ctx, `SELECT true FROM public.espacio s JOIN public.usuario u ON u.id=s.propietario_id AND u.estado='activo' WHERE s.id=$1 AND s.propietario_id=$2 AND s.estado<>'eliminada'`, spaceID, owner).Scan(&exists); errors.Is(err, pgx.ErrNoRows) {
		return nil, gallery.ErrNotFound
	} else if err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx, `SELECT g.id::text,g.fixture_code,g.mime_type,g.sha256,g.size_bytes,g.creada_en,g.publicada_en_catalogo,o.position FROM public.espacio_galeria_sintetica_local g JOIN public.gallery_item_order o ON o.synthetic_photo_id=g.id WHERE g.espacio_id=$1 AND g.propietario_id=$2 AND g.estado='activa' ORDER BY o.position,g.id`, spaceID, owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []gallery.Photo{}
	for rows.Next() {
		var item gallery.Photo
		e := rows.Scan(&item.ID, &item.Fixture, &item.MIME, &item.SHA256, &item.Size, &item.CreatedAt, &item.PublicInCatalog, &item.Position)
		if e != nil {
			return nil, e
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) Get(ctx context.Context, owner, spaceID, photoID string) (gallery.Photo, error) {
	item, err := scanPhoto(r.pool.QueryRow(ctx, `SELECT g.id::text,g.fixture_code,g.mime_type,g.sha256,g.size_bytes,g.creada_en,g.publicada_en_catalogo
		FROM public.espacio_galeria_sintetica_local g
		JOIN public.espacio s ON s.id=g.espacio_id AND s.propietario_id=g.propietario_id
		JOIN public.usuario u ON u.id=s.propietario_id AND u.estado='activo'
		WHERE g.id=$1 AND g.espacio_id=$2 AND g.propietario_id=$3 AND g.estado='activa' AND g.archivo_id IS NOT NULL`, photoID, spaceID, owner))
	if errors.Is(err, pgx.ErrNoRows) {
		return gallery.Photo{}, gallery.ErrNotFound
	}
	return item, err
}

func (r *Repository) SetPublicInCatalog(ctx context.Context, owner, spaceID, photoID string, enabled bool) (gallery.Photo, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return gallery.Photo{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	active, err := accountlock.LockActive(ctx, tx, owner)
	if err != nil {
		return gallery.Photo{}, err
	}
	if !active {
		return gallery.Photo{}, gallery.ErrNotFound
	}
	var exists bool
	if err = tx.QueryRow(ctx, `SELECT true FROM public.espacio WHERE id=$1 AND propietario_id=$2 AND estado<>'eliminada' FOR UPDATE`, spaceID, owner).Scan(&exists); errors.Is(err, pgx.ErrNoRows) {
		return gallery.Photo{}, gallery.ErrNotFound
	} else if err != nil {
		return gallery.Photo{}, err
	}
	item, err := scanPhoto(tx.QueryRow(ctx, `SELECT `+activePhotoColumns+` FROM public.espacio_galeria_sintetica_local WHERE id=$1 AND espacio_id=$2 AND propietario_id=$3 AND estado='activa' AND archivo_id IS NOT NULL FOR UPDATE`, photoID, spaceID, owner))
	if errors.Is(err, pgx.ErrNoRows) {
		return gallery.Photo{}, gallery.ErrNotFound
	}
	if err != nil {
		return gallery.Photo{}, err
	}
	if item.PublicInCatalog != enabled {
		item, err = scanPhoto(tx.QueryRow(ctx, `UPDATE public.espacio_galeria_sintetica_local SET publicada_en_catalogo=$2 WHERE id=$1 RETURNING `+activePhotoColumns, photoID, enabled))
		if err != nil {
			return gallery.Photo{}, err
		}
		if _, err = spaceversion.Bump(ctx, tx, spaceID); err != nil {
			return gallery.Photo{}, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return gallery.Photo{}, err
	}
	return item, nil
}

func (r *Repository) SetPublicInCatalogCAS(ctx context.Context, owner, spaceID, photoID string, enabled bool, expected time.Time) (gallery.Photo, time.Time, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return gallery.Photo{}, time.Time{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	active, err := accountlock.LockActive(ctx, tx, owner)
	if err != nil {
		return gallery.Photo{}, time.Time{}, err
	}
	if !active {
		return gallery.Photo{}, time.Time{}, gallery.ErrNotFound
	}
	var updated time.Time
	err = tx.QueryRow(ctx, `SELECT actualizado_en FROM public.espacio WHERE id=$1 AND propietario_id=$2 AND estado<>'eliminada' FOR UPDATE`, spaceID, owner).Scan(&updated)
	if errors.Is(err, pgx.ErrNoRows) {
		return gallery.Photo{}, time.Time{}, gallery.ErrNotFound
	}
	if err != nil {
		return gallery.Photo{}, time.Time{}, err
	}
	if !updated.Equal(expected.UTC()) {
		return gallery.Photo{}, time.Time{}, gallery.ErrConflict
	}
	item, err := scanPhoto(tx.QueryRow(ctx, `SELECT `+activePhotoColumns+` FROM public.espacio_galeria_sintetica_local WHERE id=$1 AND espacio_id=$2 AND propietario_id=$3 AND estado='activa' AND archivo_id IS NOT NULL FOR UPDATE`, photoID, spaceID, owner))
	if errors.Is(err, pgx.ErrNoRows) {
		return gallery.Photo{}, time.Time{}, gallery.ErrNotFound
	}
	if err != nil {
		return gallery.Photo{}, time.Time{}, err
	}
	if item.PublicInCatalog != enabled {
		item, err = scanPhoto(tx.QueryRow(ctx, `UPDATE public.espacio_galeria_sintetica_local SET publicada_en_catalogo=$2 WHERE id=$1 RETURNING `+activePhotoColumns, photoID, enabled))
		if err != nil {
			return gallery.Photo{}, time.Time{}, err
		}
		updated, err = spaceversion.Bump(ctx, tx, spaceID)
		if err != nil {
			return gallery.Photo{}, time.Time{}, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return gallery.Photo{}, time.Time{}, err
	}
	return item, updated, nil
}

func (r *Repository) Remove(ctx context.Context, owner, spaceID, photoID string) (gallery.Removal, error) {
	result, _, err := r.removeCAS(ctx, owner, spaceID, photoID, time.Time{})
	return result, err
}

func (r *Repository) RemoveCAS(ctx context.Context, owner, spaceID, photoID string, expected time.Time) (gallery.Removal, time.Time, error) {
	return r.removeCAS(ctx, owner, spaceID, photoID, expected)
}

func (r *Repository) removeCAS(ctx context.Context, owner, spaceID, photoID string, expected time.Time) (gallery.Removal, time.Time, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return gallery.Removal{}, time.Time{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	active, err := accountlock.LockActive(ctx, tx, owner)
	if err != nil {
		return gallery.Removal{}, time.Time{}, err
	}
	if !active {
		return gallery.Removal{}, time.Time{}, gallery.ErrNotFound
	}
	var updated time.Time
	var spaceState string
	err = tx.QueryRow(ctx, `SELECT estado,actualizado_en FROM public.espacio WHERE id=$1 AND propietario_id=$2 AND estado<>'eliminada' FOR UPDATE`, spaceID, owner).Scan(&spaceState, &updated)
	if errors.Is(err, pgx.ErrNoRows) {
		return gallery.Removal{}, time.Time{}, gallery.ErrNotFound
	}
	if err != nil {
		return gallery.Removal{}, time.Time{}, err
	}
	if !expected.IsZero() && !updated.Equal(expected.UTC()) {
		return gallery.Removal{}, time.Time{}, gallery.ErrConflict
	}
	var state string
	err = tx.QueryRow(ctx, `SELECT estado FROM public.espacio_galeria_sintetica_local WHERE id=$1 AND espacio_id=$2 AND propietario_id=$3 FOR UPDATE`, photoID, spaceID, owner).Scan(&state)
	if errors.Is(err, pgx.ErrNoRows) {
		return gallery.Removal{}, time.Time{}, gallery.ErrNotFound
	}
	if err != nil {
		return gallery.Removal{}, time.Time{}, err
	}
	reused := state != "activa"
	if !reused {
		if _, err = tx.Exec(ctx, `UPDATE public.espacio SET portada_foto_id=NULL WHERE id=$1 AND propietario_id=$2 AND portada_foto_id=$3`, spaceID, owner, photoID); err != nil {
			return gallery.Removal{}, time.Time{}, err
		}
		if _, err = tx.Exec(ctx, `DELETE FROM public.gallery_item_order WHERE synthetic_photo_id=$1`, photoID); err != nil {
			return gallery.Removal{}, time.Time{}, err
		}
		if _, err = tx.Exec(ctx, `WITH ranked AS (SELECT id,row_number() OVER (ORDER BY position,id)::smallint AS next_position FROM public.gallery_item_order WHERE espacio_id=$1) UPDATE public.gallery_item_order o SET position=ranked.next_position FROM ranked WHERE o.id=ranked.id`, spaceID); err != nil {
			return gallery.Removal{}, time.Time{}, err
		}
		if _, err = tx.Exec(ctx, `UPDATE public.espacio_galeria_sintetica_local SET estado='retirada',retirada_en=clock_timestamp(),proximo_intento_en=clock_timestamp() WHERE id=$1 AND estado='activa'`, photoID); err != nil {
			return gallery.Removal{}, time.Time{}, err
		}
		updated, err = spaceversion.Bump(ctx, tx, spaceID)
		if err != nil {
			return gallery.Removal{}, time.Time{}, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return gallery.Removal{}, time.Time{}, err
	}
	return gallery.Removal{PhotoID: photoID, Removed: true, Reused: reused}, updated, nil
}

func (r *Repository) SelectCover(ctx context.Context, owner, spaceID, photoID string) (gallery.Cover, error) {
	return r.SelectCoverCAS(ctx, owner, spaceID, photoID, time.Time{})
}

func (r *Repository) SelectCoverCAS(ctx context.Context, owner, spaceID, photoID string, expected time.Time) (gallery.Cover, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return gallery.Cover{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	active, err := accountlock.LockActive(ctx, tx, owner)
	if err != nil {
		return gallery.Cover{}, err
	}
	if !active {
		return gallery.Cover{}, gallery.ErrNotFound
	}
	var updated time.Time
	var oldCover string
	err = tx.QueryRow(ctx, `SELECT actualizado_en,COALESCE(portada_foto_id::text,'') FROM public.espacio WHERE id=$1 AND propietario_id=$2 AND estado<>'eliminada' FOR UPDATE`, spaceID, owner).Scan(&updated, &oldCover)
	if errors.Is(err, pgx.ErrNoRows) {
		return gallery.Cover{}, gallery.ErrNotFound
	}
	if err != nil {
		return gallery.Cover{}, err
	}
	if !expected.IsZero() && !updated.Equal(expected.UTC()) {
		return gallery.Cover{}, gallery.ErrConflict
	}
	var state string
	err = tx.QueryRow(ctx, `SELECT estado FROM public.espacio_galeria_sintetica_local WHERE id=$1 AND espacio_id=$2 AND propietario_id=$3 AND archivo_id IS NOT NULL FOR UPDATE`, photoID, spaceID, owner).Scan(&state)
	if errors.Is(err, pgx.ErrNoRows) || state != "activa" {
		return gallery.Cover{}, gallery.ErrNotFound
	}
	if err != nil {
		return gallery.Cover{}, err
	}
	if oldCover != photoID {
		if _, err = tx.Exec(ctx, `UPDATE public.espacio SET portada_media_id=NULL,portada_foto_id=$3 WHERE id=$1 AND propietario_id=$2`, spaceID, owner, photoID); err != nil {
			return gallery.Cover{}, err
		}
		updated, err = spaceversion.Bump(ctx, tx, spaceID)
		if err != nil {
			return gallery.Cover{}, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return gallery.Cover{}, err
	}
	return gallery.Cover{SpaceID: spaceID, PhotoID: photoID, Selected: true, UpdatedAt: updated}, nil
}

func lockAdministrativeGalleryAccounts(ctx context.Context, tx pgx.Tx, actor, owner string) error {
	if actor == owner {
		return gallery.ErrNotFound
	}
	lock := func(id string) error {
		active, err := accountlock.LockActive(ctx, tx, id)
		if err != nil {
			return err
		}
		if !active {
			return gallery.ErrNotFound
		}
		return nil
	}
	if actor == owner {
		if err := lock(actor); err != nil {
			return err
		}
	} else if actor < owner {
		if err := lock(actor); err != nil {
			return err
		}
		if err := lock(owner); err != nil {
			return err
		}
	} else {
		if err := lock(owner); err != nil {
			return err
		}
		if err := lock(actor); err != nil {
			return err
		}
	}
	var actorAllowed, ownerBlocked bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.administrador_global g JOIN public.usuario u ON u.id=g.usuario_id WHERE g.usuario_id=$1 AND (u.bloqueado_hasta IS NULL OR u.bloqueado_hasta<=now()) AND NOT EXISTS(SELECT 1 FROM public.bloqueo_cuenta_administrativo_local b WHERE b.cuenta_id=u.id))`, actor).Scan(&actorAllowed); err != nil {
		return err
	}
	if !actorAllowed {
		return gallery.ErrNotFound
	}
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.bloqueo_cuenta_administrativo_local WHERE cuenta_id=$1)`, owner).Scan(&ownerBlocked); err != nil {
		return err
	}
	if ownerBlocked {
		return gallery.ErrNotFound
	}
	return nil
}

func lockAdministrativeSpace(ctx context.Context, tx pgx.Tx, owner, spaceID string, expected time.Time) (string, time.Time, error) {
	var state string
	var updated time.Time
	err := tx.QueryRow(ctx, `SELECT estado,actualizado_en FROM public.espacio WHERE id=$1 AND propietario_id=$2 FOR UPDATE`, spaceID, owner).Scan(&state, &updated)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", time.Time{}, gallery.ErrNotFound
	}
	if err != nil {
		return "", time.Time{}, err
	}
	if !expected.IsZero() && !updated.Equal(expected.UTC()) {
		return "", time.Time{}, gallery.ErrConflict
	}
	if state == "eliminada" {
		return "", time.Time{}, gallery.ErrNotFound
	}
	return state, updated, nil
}

func auditAdministrativeGallery(ctx context.Context, tx pgx.Tx, actor, owner, spaceID, action, reason, correlation string, at time.Time) error {
	id, err := (credentials.Generator{}).ID()
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO public.evento_auditoria_local(id,actor_id,cuenta_objetivo_id,recurso_tipo,recurso_id,accion,resultado,motivo_codigo,correlacion_id,ocurrido_en,registrado_en,retirar_en) VALUES($1,$2,$3,'publicacion_global',$4,$5,'exito',$6,$7,$8,clock_timestamp(),$8::timestamptz+interval '5 years')`, id, actor, owner, spaceID, action, reason, correlation, at.UTC())
	return err
}

func (r *Repository) ListAsAdministrator(ctx context.Context, actor, owner, spaceID, correlation string) ([]gallery.Photo, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err = lockAdministrativeGalleryAccounts(ctx, tx, actor, owner); err != nil {
		return nil, err
	}
	var exists bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.espacio WHERE id=$1 AND propietario_id=$2)`, spaceID, owner).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, gallery.ErrNotFound
	}
	rows, err := tx.Query(ctx, `SELECT g.id::text,g.fixture_code,g.mime_type,g.sha256,g.size_bytes,g.creada_en,g.publicada_en_catalogo,g.estado,COALESCE(s.portada_foto_id=g.id,false),g.archivo_id IS NOT NULL AND g.estado<>'activa' FROM public.espacio_galeria_sintetica_local g JOIN public.espacio s ON s.id=g.espacio_id AND s.propietario_id=g.propietario_id WHERE g.espacio_id=$1 AND g.propietario_id=$2 ORDER BY g.creada_en,g.id`, spaceID, owner)
	if err != nil {
		return nil, err
	}
	items := make([]gallery.Photo, 0)
	for rows.Next() {
		var item gallery.Photo
		if err = rows.Scan(&item.ID, &item.Fixture, &item.MIME, &item.SHA256, &item.Size, &item.CreatedAt, &item.PublicInCatalog, &item.State, &item.Cover, &item.FilePending); err != nil {
			rows.Close()
			return nil, err
		}
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	if err = auditAdministrativeGallery(ctx, tx, actor, owner, spaceID, "admin.global.publications.gallery.read", "consulta_recurso", correlation, time.Now()); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *Repository) SetPublicInCatalogAsAdministrator(ctx context.Context, actor, owner, spaceID, photoID string, enabled bool, expected time.Time, correlation, reason string) (gallery.Photo, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return gallery.Photo{}, err
	}
	defer tx.Rollback(ctx)
	if err = lockAdministrativeGalleryAccounts(ctx, tx, actor, owner); err != nil {
		return gallery.Photo{}, err
	}
	if _, _, err = lockAdministrativeSpace(ctx, tx, owner, spaceID, expected); err != nil {
		return gallery.Photo{}, err
	}
	item, err := scanPhoto(tx.QueryRow(ctx, `UPDATE public.espacio_galeria_sintetica_local SET publicada_en_catalogo=$4 WHERE id=$1 AND espacio_id=$2 AND propietario_id=$3 AND estado='activa' AND archivo_id IS NOT NULL RETURNING `+activePhotoColumns, photoID, spaceID, owner, enabled))
	if errors.Is(err, pgx.ErrNoRows) {
		return gallery.Photo{}, gallery.ErrNotFound
	}
	if err != nil {
		return gallery.Photo{}, err
	}
	var at time.Time
	if err = tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&at); err != nil {
		return gallery.Photo{}, err
	}
	if _, err = spaceversion.Bump(ctx, tx, spaceID); err != nil {
		return gallery.Photo{}, err
	}
	if err = auditAdministrativeGallery(ctx, tx, actor, owner, spaceID, "admin.global.publications.gallery.visibility", reason, correlation, at); err != nil {
		return gallery.Photo{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return gallery.Photo{}, err
	}
	item.State = "activa"
	return item, nil
}

func (r *Repository) SelectCoverAsAdministrator(ctx context.Context, actor, owner, spaceID, photoID string, expected time.Time, correlation, reason string) (gallery.Cover, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return gallery.Cover{}, err
	}
	defer tx.Rollback(ctx)
	if err = lockAdministrativeGalleryAccounts(ctx, tx, actor, owner); err != nil {
		return gallery.Cover{}, err
	}
	if _, _, err = lockAdministrativeSpace(ctx, tx, owner, spaceID, expected); err != nil {
		return gallery.Cover{}, err
	}
	var exists bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.espacio_galeria_sintetica_local WHERE id=$1 AND espacio_id=$2 AND propietario_id=$3 AND estado='activa' AND archivo_id IS NOT NULL)`, photoID, spaceID, owner).Scan(&exists); err != nil {
		return gallery.Cover{}, err
	}
	if !exists {
		return gallery.Cover{}, gallery.ErrNotFound
	}
	var at time.Time
	if err = tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&at); err != nil {
		return gallery.Cover{}, err
	}
	if _, err = tx.Exec(ctx, `UPDATE public.espacio SET portada_foto_id=$3 WHERE id=$1 AND propietario_id=$2`, spaceID, owner, photoID); err != nil {
		return gallery.Cover{}, err
	}
	if _, err = spaceversion.Bump(ctx, tx, spaceID); err != nil {
		return gallery.Cover{}, err
	}
	if err = auditAdministrativeGallery(ctx, tx, actor, owner, spaceID, "admin.global.publications.gallery.cover", reason, correlation, at); err != nil {
		return gallery.Cover{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return gallery.Cover{}, err
	}
	return gallery.Cover{SpaceID: spaceID, PhotoID: photoID, Selected: true}, nil
}

func (r *Repository) RemoveAsAdministrator(ctx context.Context, actor, owner, spaceID, photoID string, expected time.Time, correlation, reason string) (gallery.Removal, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return gallery.Removal{}, err
	}
	defer tx.Rollback(ctx)
	if err = lockAdministrativeGalleryAccounts(ctx, tx, actor, owner); err != nil {
		return gallery.Removal{}, err
	}
	_, updated, err := lockAdministrativeSpace(ctx, tx, owner, spaceID, time.Time{})
	if err != nil {
		return gallery.Removal{}, err
	}
	var state string
	if err = tx.QueryRow(ctx, `SELECT estado FROM public.espacio_galeria_sintetica_local WHERE id=$1 AND espacio_id=$2 AND propietario_id=$3 FOR UPDATE`, photoID, spaceID, owner).Scan(&state); errors.Is(err, pgx.ErrNoRows) {
		return gallery.Removal{}, gallery.ErrNotFound
	} else if err != nil {
		return gallery.Removal{}, err
	}
	if state != "activa" {
		if err = tx.Commit(ctx); err != nil {
			return gallery.Removal{}, err
		}
		return gallery.Removal{PhotoID: photoID, Removed: true, Reused: true}, nil
	}
	if !updated.Equal(expected.UTC()) {
		return gallery.Removal{}, gallery.ErrConflict
	}
	var at time.Time
	if err = tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&at); err != nil {
		return gallery.Removal{}, err
	}
	if _, err = tx.Exec(ctx, `UPDATE public.espacio SET portada_foto_id=NULL WHERE id=$1 AND propietario_id=$2 AND portada_foto_id=$3`, spaceID, owner, photoID); err != nil {
		return gallery.Removal{}, err
	}
	if _, err = tx.Exec(ctx, `UPDATE public.espacio_galeria_sintetica_local SET estado='retirada',retirada_en=$2,proximo_intento_en=$2 WHERE id=$1 AND estado='activa'`, photoID, at); err != nil {
		return gallery.Removal{}, err
	}
	if _, err = spaceversion.Bump(ctx, tx, spaceID); err != nil {
		return gallery.Removal{}, err
	}
	if err = auditAdministrativeGallery(ctx, tx, actor, owner, spaceID, "admin.global.publications.gallery.remove", reason, correlation, at); err != nil {
		return gallery.Removal{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return gallery.Removal{}, err
	}
	return gallery.Removal{PhotoID: photoID, Removed: true}, nil
}

func (r *Repository) PendingCleanup(ctx context.Context, limit int) ([]string, error) {
	rows, err := r.pool.Query(ctx, `SELECT archivo_id::text FROM public.espacio_galeria_sintetica_local WHERE estado<>'activa' AND archivo_id IS NOT NULL AND proximo_intento_en<=clock_timestamp() ORDER BY proximo_intento_en,id LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *Repository) CompleteCleanup(ctx context.Context, fileID string, at time.Time) error {
	_, err := r.pool.Exec(ctx, `UPDATE public.espacio_galeria_sintetica_local SET archivo_id=NULL,limpia_en=$2,proximo_intento_en=NULL,ultimo_codigo_error=NULL WHERE archivo_id=$1 AND estado<>'activa'`, fileID, at.UTC())
	return err
}

func (r *Repository) FailCleanup(ctx context.Context, fileID string, at time.Time) error {
	_, err := r.pool.Exec(ctx, `UPDATE public.espacio_galeria_sintetica_local SET intentos_limpieza=intentos_limpieza+1,proximo_intento_en=$2::timestamptz+LEAST(interval '5 minutes',interval '5 seconds'*(intentos_limpieza+1)),ultimo_codigo_error='archivo_sintetico_no_eliminado' WHERE archivo_id=$1 AND estado<>'activa'`, fileID, at.UTC())
	return err
}
