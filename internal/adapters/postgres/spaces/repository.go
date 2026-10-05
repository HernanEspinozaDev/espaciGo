package spacespg

import (
	"context"
	"errors"

	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
	"github.com/HernanEspinozaDev/espaciGo/internal/spaces"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
	ids  identity.CredentialGenerator
}

func New(pool *pgxpool.Pool, ids identity.CredentialGenerator) *Repository {
	return &Repository{pool: pool, ids: ids}
}

const fields = `e.id::text, e.categoria_codigo, c.nombre, e.titulo, e.descripcion, e.superficie_m2::float8, e.capacidad_maxima, e.reglas_uso, e.modalidad_tarifa, e.precio_base_clp, e.direccion, e.estado`

func scan(row pgx.Row) (spaces.Draft, error) {
	var d spaces.Draft
	err := row.Scan(&d.ID, &d.CategoryCode, &d.CategoryName, &d.Title, &d.Description, &d.AreaM2, &d.Capacity, &d.UsageRules, &d.RateUnit, &d.BasePriceCLP, &d.Address, &d.State)
	return d, mapError(err)
}
func (r *Repository) Categories(ctx context.Context) ([]spaces.Category, error) {
	rows, err := r.pool.Query(ctx, `SELECT codigo,nombre FROM public.categoria_espacio WHERE activa ORDER BY orden`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]spaces.Category, 0)
	for rows.Next() {
		var c spaces.Category
		if err := rows.Scan(&c.Code, &c.Name); err != nil {
			return nil, err
		}
		items = append(items, c)
	}
	return items, rows.Err()
}
func (r *Repository) Create(ctx context.Context, owner string, in spaces.Input) (spaces.Draft, error) {
	id, err := r.ids.ID()
	if err != nil {
		return spaces.Draft{}, err
	}
	return scan(r.pool.QueryRow(ctx, `WITH inserted AS (INSERT INTO public.espacio (id,propietario_id,categoria_codigo,titulo,descripcion,superficie_m2,capacidad_maxima,reglas_uso,modalidad_tarifa,precio_base_clp,direccion) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) RETURNING *) SELECT `+fields+` FROM inserted e JOIN public.categoria_espacio c ON c.codigo=e.categoria_codigo`, id, owner, in.CategoryCode, in.Title, in.Description, in.AreaM2, in.Capacity, in.UsageRules, in.RateUnit, in.BasePriceCLP, in.Address))
}
func (r *Repository) ListOwn(ctx context.Context, owner string) ([]spaces.Draft, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+fields+` FROM public.espacio e JOIN public.categoria_espacio c ON c.codigo=e.categoria_codigo WHERE e.propietario_id=$1 ORDER BY e.actualizado_en DESC,e.id`, owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]spaces.Draft, 0)
	for rows.Next() {
		d, e := scan(rows)
		if e != nil {
			return nil, e
		}
		items = append(items, d)
	}
	return items, rows.Err()
}
func (r *Repository) GetOwn(ctx context.Context, owner, id string) (spaces.Draft, error) {
	return scan(r.pool.QueryRow(ctx, `SELECT `+fields+` FROM public.espacio e JOIN public.categoria_espacio c ON c.codigo=e.categoria_codigo WHERE e.propietario_id=$1 AND e.id=$2 AND e.estado='borrador'`, owner, id))
}
func (r *Repository) UpdateOwn(ctx context.Context, owner, id string, in spaces.Input) (spaces.Draft, error) {
	return scan(r.pool.QueryRow(ctx, `UPDATE public.espacio e SET categoria_codigo=$3,titulo=$4,descripcion=$5,superficie_m2=$6,capacidad_maxima=$7,reglas_uso=$8,modalidad_tarifa=$9,precio_base_clp=$10,direccion=$11,actualizado_en=now() FROM public.categoria_espacio c WHERE e.categoria_codigo=c.codigo AND e.propietario_id=$1 AND e.id=$2 AND e.estado='borrador' RETURNING `+fields, owner, id, in.CategoryCode, in.Title, in.Description, in.AreaM2, in.Capacity, in.UsageRules, in.RateUnit, in.BasePriceCLP, in.Address))
}
func mapError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return spaces.ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23503" {
		return spaces.ErrInvalid
	}
	return err
}
