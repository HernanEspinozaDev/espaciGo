package pricingpg

import (
	"context"
	"errors"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/pricing"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func scanRate(row pgx.Row) (pricing.Rate, error) {
	var r pricing.Rate
	err := row.Scan(&r.SpaceID, &r.Version, &r.Unit, &r.Amount, &r.Currency, &r.CreatedAt)
	return r, mapError(err)
}

const rateCols = `espacio_id::text,version,modalidad,precio_base_clp,moneda,creada_en`

func (r *Repository) CurrentRate(ctx context.Context, owner, spaceID string) (pricing.Rate, error) {
	return scanRate(r.pool.QueryRow(ctx, `SELECT t.`+rateCols+` FROM public.tarifa_espacio t JOIN public.espacio e ON e.id=t.espacio_id WHERE e.id=$1 AND e.propietario_id=$2 AND e.estado='borrador' ORDER BY t.version DESC LIMIT 1`, spaceID, owner))
}
func (r *Repository) RateHistory(ctx context.Context, owner, spaceID string) ([]pricing.Rate, error) {
	rows, err := r.pool.Query(ctx, `SELECT t.`+rateCols+` FROM public.tarifa_espacio t JOIN public.espacio e ON e.id=t.espacio_id WHERE e.id=$1 AND e.propietario_id=$2 AND e.estado='borrador' ORDER BY t.version`, spaceID, owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []pricing.Rate{}
	for rows.Next() {
		item, err := scanRate(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
func (r *Repository) UpdateRate(ctx context.Context, owner, spaceID string, in pricing.RateInput) (pricing.Rate, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return pricing.Rate{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var unit string
	var amount int64
	err = tx.QueryRow(ctx, `SELECT modalidad_tarifa,precio_base_clp FROM public.espacio WHERE id=$1 AND propietario_id=$2 AND estado='borrador' FOR UPDATE`, spaceID, owner).Scan(&unit, &amount)
	if err != nil {
		return pricing.Rate{}, mapError(err)
	}
	if unit == in.Unit && amount == in.Amount {
		return scanRate(tx.QueryRow(ctx, `SELECT `+rateCols+` FROM public.tarifa_espacio WHERE espacio_id=$1 ORDER BY version DESC LIMIT 1`, spaceID))
	}
	_, err = tx.Exec(ctx, `UPDATE public.espacio SET modalidad_tarifa=$3,precio_base_clp=$4,actualizado_en=now() WHERE id=$1 AND propietario_id=$2 AND estado='borrador'`, spaceID, owner, in.Unit, in.Amount)
	if err != nil {
		return pricing.Rate{}, mapError(err)
	}
	item, err := scanRate(tx.QueryRow(ctx, `INSERT INTO public.tarifa_espacio(espacio_id,version,modalidad,precio_base_clp) SELECT $1,COALESCE(max(version),0)+1,$2,$3 FROM public.tarifa_espacio WHERE espacio_id=$1 RETURNING `+rateCols, spaceID, in.Unit, in.Amount))
	if err != nil {
		return pricing.Rate{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return pricing.Rate{}, err
	}
	return item, nil
}
func (r *Repository) CreateSimulation(ctx context.Context, owner, spaceID, id, zone string, rate pricing.Rate, start, end time.Time, units, subtotal int64) (pricing.Simulation, error) {
	var s pricing.Simulation
	err := r.pool.QueryRow(ctx, `INSERT INTO public.simulacion_precio_privada(id,espacio_id,propietario_id,tarifa_version,modalidad,precio_unitario_clp,moneda,unidades_facturadas,subtotal_clp,inicio,termino,zona_horaria)
	SELECT $1,e.id,e.propietario_id,$4,$5,$6,'CLP',$7,$8,$9,$10,$11 FROM public.espacio e WHERE e.id=$2 AND e.propietario_id=$3 AND e.estado='borrador'
	RETURNING id::text,espacio_id::text,tarifa_version,modalidad,precio_unitario_clp,unidades_facturadas,moneda,subtotal_clp,inicio,termino,zona_horaria,true,creada_en`, id, spaceID, owner, rate.Version, rate.Unit, rate.Amount, units, subtotal, start, end, zone).Scan(&s.ID, &s.SpaceID, &s.RateVersion, &s.RateUnit, &s.BasePrice, &s.BilledUnits, &s.Currency, &s.Subtotal, &s.StartAt, &s.EndAt, &s.TimeZone, &s.Private, &s.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return pricing.Simulation{}, pricing.ErrNotFound
	}
	if err != nil {
		return pricing.Simulation{}, err
	}
	s.StartAt = s.StartAt.UTC()
	s.EndAt = s.EndAt.UTC()
	s.CreatedAt = s.CreatedAt.UTC()
	return s, nil
}
func (r *Repository) GetSimulation(ctx context.Context, owner, spaceID, id string) (pricing.Simulation, error) {
	var s pricing.Simulation
	err := r.pool.QueryRow(ctx, `SELECT id::text,espacio_id::text,tarifa_version,modalidad,precio_unitario_clp,unidades_facturadas,moneda,subtotal_clp,inicio,termino,zona_horaria,true,creada_en FROM public.simulacion_precio_privada WHERE id=$1 AND espacio_id=$2 AND propietario_id=$3`, id, spaceID, owner).Scan(&s.ID, &s.SpaceID, &s.RateVersion, &s.RateUnit, &s.BasePrice, &s.BilledUnits, &s.Currency, &s.Subtotal, &s.StartAt, &s.EndAt, &s.TimeZone, &s.Private, &s.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return pricing.Simulation{}, pricing.ErrNotFound
	}
	if err != nil {
		return pricing.Simulation{}, err
	}
	s.StartAt = s.StartAt.UTC()
	s.EndAt = s.EndAt.UTC()
	s.CreatedAt = s.CreatedAt.UTC()
	return s, nil
}
func mapError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return pricing.ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && (pgErr.Code == "23503" || pgErr.Code == "23514" || pgErr.Code == "22003" || pgErr.Code == "23505") {
		return pricing.ErrInvalid
	}
	return err
}
