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
	err = tx.QueryRow(ctx, `SELECT true FROM public.espacio WHERE id=$1 AND propietario_id=$2 FOR UPDATE`, spaceID, owner).Scan(&exists)
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
	if err = tx.QueryRow(ctx, `SELECT true FROM public.espacio WHERE id=$1 AND propietario_id=$2 FOR UPDATE`, spaceID, owner).Scan(&exists); errors.Is(err, pgx.ErrNoRows) {
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
	result, err := w.tx.Exec(ctx, `DELETE FROM public.espacio_galeria_archivo_candidato_local WHERE archivo_id=$1 AND estado='reservado'`, w.fileID)
	if err != nil {
		return gallery.Photo{}, false, err
	}
	if result.RowsAffected() != 1 {
		return gallery.Photo{}, false, gallery.ErrCandidateUnavailable
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
	err := row.Scan(&item.ID, &item.Fixture, &item.MIME, &item.SHA256, &item.Size, &item.CreatedAt)
	return item, err
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
