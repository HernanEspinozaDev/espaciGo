package occupancy

import (
	"context"
	"errors"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/accountlock"
	domain "github.com/HernanEspinozaDev/espaciGo/internal/occupancy"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func (r *Repository) SetTimeZone(ctx context.Context, owner, spaceID, zone string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	active, err := accountlock.LockActive(ctx, tx, owner)
	if err != nil {
		return err
	}
	if !active {
		return domain.ErrNotFound
	}
	result, err := tx.Exec(ctx, `UPDATE public.espacio
		SET zona_horaria=$3, actualizado_en=now()
		WHERE id=$1 AND propietario_id=$2 AND estado='borrador'`, spaceID, owner, zone)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return tx.Commit(ctx)
}

func (r *Repository) TimeZone(ctx context.Context, owner, spaceID string) (string, error) {
	return r.timeZone(ctx, owner, spaceID)
}

func (r *Repository) Availability(ctx context.Context, owner, spaceID string, start, end time.Time) (domain.Availability, error) {
	var zone *string
	var available bool
	err := r.pool.QueryRow(ctx, `SELECT e.zona_horaria,
		NOT EXISTS (SELECT 1 FROM public.ocupacion o
			WHERE o.espacio_id=e.id AND o.activo AND o.intervalo && tstzrange($3,$4,'[)'))
		FROM public.espacio e WHERE e.id=$1 AND e.propietario_id=$2 AND e.estado='borrador'`,
		spaceID, owner, start, end).Scan(&zone, &available)
	if err != nil {
		return domain.Availability{}, mapError(err)
	}
	if zone == nil || *zone == "" {
		return domain.Availability{}, domain.ErrTimezoneRequired
	}
	return domain.Availability{SpaceID: spaceID, TimeZone: *zone, StartAt: start, EndAt: end, Available: available}, nil
}

func (r *Repository) ListBlocks(ctx context.Context, owner, spaceID string, start, end time.Time) (domain.Calendar, error) {
	zone, err := r.timeZone(ctx, owner, spaceID)
	if err != nil {
		return domain.Calendar{}, err
	}
	rows, err := r.pool.Query(ctx, `SELECT o.id::text, o.espacio_id::text,
		lower(o.intervalo), upper(o.intervalo), o.motivo, o.creada_en
		FROM public.ocupacion o JOIN public.espacio e ON e.id=o.espacio_id
		WHERE e.id=$1 AND e.propietario_id=$2 AND e.estado='borrador'
		AND o.tipo='bloqueo_manual' AND o.activo AND o.intervalo && tstzrange($3,$4,'[)')
		ORDER BY lower(o.intervalo), o.id`, spaceID, owner, start, end)
	if err != nil {
		return domain.Calendar{}, err
	}
	defer rows.Close()
	calendar := domain.Calendar{SpaceID: spaceID, TimeZone: zone, Items: []domain.Block{}}
	for rows.Next() {
		var block domain.Block
		block.TimeZone = zone
		if err := rows.Scan(&block.ID, &block.SpaceID, &block.StartAt, &block.EndAt, &block.Reason, &block.CreatedAt); err != nil {
			return domain.Calendar{}, err
		}
		block.StartAt, block.EndAt, block.CreatedAt = block.StartAt.UTC(), block.EndAt.UTC(), block.CreatedAt.UTC()
		calendar.Items = append(calendar.Items, block)
	}
	return calendar, rows.Err()
}

func (r *Repository) CreateBlock(ctx context.Context, owner, spaceID, blockID string, start, end time.Time, reason string) (domain.Block, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Block{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	active, err := accountlock.LockActive(ctx, tx, owner)
	if err != nil {
		return domain.Block{}, err
	}
	if !active {
		return domain.Block{}, domain.ErrNotFound
	}
	var zone *string
	err = tx.QueryRow(ctx, `SELECT zona_horaria FROM public.espacio WHERE id=$1 AND propietario_id=$2 AND estado='borrador'`, spaceID, owner).Scan(&zone)
	if err != nil {
		return domain.Block{}, mapError(err)
	}
	if zone == nil || *zone == "" {
		return domain.Block{}, domain.ErrTimezoneRequired
	}
	var block domain.Block
	block.TimeZone = *zone
	err = tx.QueryRow(ctx, `INSERT INTO public.ocupacion
		(id, espacio_id, reserva_id, intervalo, tipo, activo, motivo)
		SELECT $1,e.id,NULL,tstzrange($4,$5,'[)'),'bloqueo_manual',true,$6
		FROM public.espacio e WHERE e.id=$2 AND e.propietario_id=$3 AND e.estado='borrador'
		RETURNING id::text, espacio_id::text, lower(intervalo), upper(intervalo), motivo, creada_en`,
		blockID, spaceID, owner, start, end, reason).Scan(&block.ID, &block.SpaceID, &block.StartAt, &block.EndAt, &block.Reason, &block.CreatedAt)
	if err != nil {
		return domain.Block{}, mapError(err)
	}
	if err = tx.Commit(ctx); err != nil {
		return domain.Block{}, mapError(err)
	}
	block.StartAt, block.EndAt, block.CreatedAt = block.StartAt.UTC(), block.EndAt.UTC(), block.CreatedAt.UTC()
	return block, nil
}

func (r *Repository) DeleteBlock(ctx context.Context, owner, spaceID, blockID string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	active, err := accountlock.LockActive(ctx, tx, owner)
	if err != nil {
		return err
	}
	if !active {
		return domain.ErrNotFound
	}
	result, err := tx.Exec(ctx, `UPDATE public.ocupacion o SET activo=false, desactivada_en=now()
		FROM public.espacio e WHERE o.espacio_id=e.id AND e.id=$1 AND e.propietario_id=$2
		AND e.estado='borrador' AND o.id=$3 AND o.tipo='bloqueo_manual' AND o.activo`, spaceID, owner, blockID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return tx.Commit(ctx)
}

func (r *Repository) timeZone(ctx context.Context, owner, spaceID string) (string, error) {
	var zone *string
	err := r.pool.QueryRow(ctx, `SELECT zona_horaria FROM public.espacio
		WHERE id=$1 AND propietario_id=$2 AND estado='borrador'`, spaceID, owner).Scan(&zone)
	if err != nil {
		return "", mapError(err)
	}
	if zone == nil || *zone == "" {
		return "", domain.ErrTimezoneRequired
	}
	return *zone, nil
}

func mapError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23P01":
			return domain.ErrConflict
		case "23514", "22007", "22008", "22023":
			return domain.ErrInvalid
		}
	}
	return err
}
