package gallerypg

import (
	"context"
	"errors"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/accountlock"
	"github.com/HernanEspinozaDev/espaciGo/internal/gallery"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

const activePhotoColumns = `id::text,fixture_code,mime_type,sha256,size_bytes,creada_en`

func scanPhoto(row pgx.Row) (gallery.Photo, error) {
	var item gallery.Photo
	err := row.Scan(&item.ID, &item.Fixture, &item.MIME, &item.SHA256, &item.Size, &item.CreatedAt)
	return item, err
}

func (r *Repository) Add(ctx context.Context, owner, spaceID string, item gallery.Photo, key string) (gallery.Photo, bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return gallery.Photo{}, false, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	active, err := accountlock.LockActive(ctx, tx, owner)
	if err != nil {
		return gallery.Photo{}, false, err
	}
	if !active {
		return gallery.Photo{}, false, gallery.ErrNotFound
	}
	var state string
	err = tx.QueryRow(ctx, `SELECT estado FROM public.espacio WHERE id=$1 AND propietario_id=$2 FOR UPDATE`, spaceID, owner).Scan(&state)
	if errors.Is(err, pgx.ErrNoRows) {
		return gallery.Photo{}, false, gallery.ErrNotFound
	}
	if err != nil {
		return gallery.Photo{}, false, err
	}
	prior, err := scanPhoto(tx.QueryRow(ctx, `SELECT `+activePhotoColumns+` FROM public.espacio_galeria_sintetica_local WHERE espacio_id=$1 AND propietario_id=$2 AND clave_idempotencia=$3`, spaceID, owner, key))
	if err == nil {
		if err = tx.Commit(ctx); err != nil {
			return gallery.Photo{}, false, err
		}
		return prior, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return gallery.Photo{}, false, err
	}
	var count int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM public.espacio_galeria_sintetica_local WHERE espacio_id=$1 AND propietario_id=$2 AND estado='activa'`, spaceID, owner).Scan(&count); err != nil {
		return gallery.Photo{}, false, err
	}
	if count >= gallery.MaxPhotosPerSpace {
		return gallery.Photo{}, false, gallery.ErrLimit
	}
	_, err = tx.Exec(ctx, `INSERT INTO public.espacio_galeria_sintetica_local(id,espacio_id,propietario_id,archivo_id,fixture_code,mime_type,sha256,size_bytes,estado,creada_en,clave_idempotencia)
		VALUES($1,$2,$3,$1,$4,$5,$6,$7,'activa',$8,$9)`, item.ID, spaceID, owner, item.Fixture, item.MIME, item.SHA256, item.Size, item.CreatedAt.UTC(), key)
	if err != nil {
		return gallery.Photo{}, false, err
	}
	if err = tx.Commit(ctx); err != nil {
		return gallery.Photo{}, false, err
	}
	return item, false, nil
}

func (r *Repository) List(ctx context.Context, owner, spaceID string) ([]gallery.Photo, error) {
	var exists bool
	if err := r.pool.QueryRow(ctx, `SELECT true FROM public.espacio s JOIN public.usuario u ON u.id=s.propietario_id AND u.estado='activo' WHERE s.id=$1 AND s.propietario_id=$2`, spaceID, owner).Scan(&exists); errors.Is(err, pgx.ErrNoRows) {
		return nil, gallery.ErrNotFound
	} else if err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx, `SELECT `+activePhotoColumns+` FROM public.espacio_galeria_sintetica_local WHERE espacio_id=$1 AND propietario_id=$2 AND estado='activa' ORDER BY creada_en,id`, spaceID, owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []gallery.Photo{}
	for rows.Next() {
		item, e := scanPhoto(rows)
		if e != nil {
			return nil, e
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) Get(ctx context.Context, owner, spaceID, photoID string) (gallery.Photo, error) {
	item, err := scanPhoto(r.pool.QueryRow(ctx, `SELECT g.id::text,g.fixture_code,g.mime_type,g.sha256,g.size_bytes,g.creada_en
		FROM public.espacio_galeria_sintetica_local g
		JOIN public.espacio s ON s.id=g.espacio_id AND s.propietario_id=g.propietario_id
		JOIN public.usuario u ON u.id=s.propietario_id AND u.estado='activo'
		WHERE g.id=$1 AND g.espacio_id=$2 AND g.propietario_id=$3 AND g.estado='activa' AND g.archivo_id IS NOT NULL`, photoID, spaceID, owner))
	if errors.Is(err, pgx.ErrNoRows) {
		return gallery.Photo{}, gallery.ErrNotFound
	}
	return item, err
}

func (r *Repository) Remove(ctx context.Context, owner, spaceID, photoID string) (gallery.Removal, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return gallery.Removal{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	active, err := accountlock.LockActive(ctx, tx, owner)
	if err != nil {
		return gallery.Removal{}, err
	}
	if !active {
		return gallery.Removal{}, gallery.ErrNotFound
	}
	var spaceState string
	err = tx.QueryRow(ctx, `SELECT estado FROM public.espacio WHERE id=$1 AND propietario_id=$2 FOR UPDATE`, spaceID, owner).Scan(&spaceState)
	if errors.Is(err, pgx.ErrNoRows) {
		return gallery.Removal{}, gallery.ErrNotFound
	}
	if err != nil {
		return gallery.Removal{}, err
	}
	var state string
	err = tx.QueryRow(ctx, `SELECT estado FROM public.espacio_galeria_sintetica_local WHERE id=$1 AND espacio_id=$2 AND propietario_id=$3 FOR UPDATE`, photoID, spaceID, owner).Scan(&state)
	if errors.Is(err, pgx.ErrNoRows) {
		return gallery.Removal{}, gallery.ErrNotFound
	}
	if err != nil {
		return gallery.Removal{}, err
	}
	reused := state != "activa"
	if !reused {
		if _, err = tx.Exec(ctx, `UPDATE public.espacio_galeria_sintetica_local SET estado='retirada',retirada_en=clock_timestamp(),proximo_intento_en=clock_timestamp() WHERE id=$1 AND estado='activa'`, photoID); err != nil {
			return gallery.Removal{}, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return gallery.Removal{}, err
	}
	return gallery.Removal{PhotoID: photoID, Removed: true, Reused: reused}, nil
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
