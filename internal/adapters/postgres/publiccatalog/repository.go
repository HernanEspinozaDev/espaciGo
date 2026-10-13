package publiccatalogpg

import (
	"context"

	"github.com/HernanEspinozaDev/espaciGo/internal/publiccatalog"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func (r *Repository) Categories(ctx context.Context) ([]publiccatalog.Category, error) {
	rows, err := r.pool.Query(ctx, `SELECT codigo,nombre,orden FROM public.categoria_espacio WHERE activa ORDER BY orden,codigo`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]publiccatalog.Category, 0, 8)
	for rows.Next() {
		var v publiccatalog.Category
		if err := rows.Scan(&v.Code, &v.Name, &v.Order); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (r *Repository) HasActiveCategory(ctx context.Context, code string) (bool, error) {
	var active bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.categoria_espacio WHERE codigo=$1 AND activa)`, code).Scan(&active)
	return active, err
}

const eligiblePublicSQL = ` FROM public.espacio e
JOIN public.categoria_espacio c ON c.codigo=e.categoria_codigo AND c.activa
JOIN LATERAL (SELECT modalidad,precio_base_clp,moneda FROM public.tarifa_espacio WHERE espacio_id=e.id ORDER BY version DESC LIMIT 1) t ON true
JOIN public.usuario owner ON owner.id=e.propietario_id AND owner.estado='activo'
WHERE e.estado='activa'
AND NOT EXISTS(SELECT 1 FROM public.bloqueo_cuenta_administrativo_local b WHERE b.cuenta_id=owner.id)
AND EXISTS(SELECT 1 FROM public.elegibilidad_verificacion_local eligibility
 JOIN public.verificacion v ON v.id=eligibility.verificacion_id AND v.usuario_id=eligibility.usuario_id AND v.tipo=eligibility.tipo AND v.estado='aprobada'
 WHERE eligibility.usuario_id=owner.id AND eligibility.tipo='kyc' AND eligibility.estado='elegible')
AND NOT EXISTS(SELECT 1 FROM public.reserva_ensayo_local_fixture f WHERE f.espacio_id=e.id AND f.anfitrion_id=e.propietario_id AND f.habilitada)`

const projection = `SELECT e.id::text,e.categoria_codigo,c.nombre,e.titulo,e.descripcion,e.superficie_m2::double precision,t.modalidad,t.precio_base_clp,t.moneda,c.orden,lower(e.titulo) COLLATE "C"`

func (r *Repository) List(ctx context.Context, f publiccatalog.Filters, after *publiccatalog.Cursor, limit int) ([]publiccatalog.Space, error) {
	query := projection + eligiblePublicSQL + ` AND ($1='' OR e.categoria_codigo=$1) AND ($2='' OR t.modalidad=$2) AND ($3::bigint IS NULL OR t.precio_base_clp >= $3) AND ($4::bigint IS NULL OR t.precio_base_clp <= $4)`
	args := []any{f.CategoryCode, f.RateUnit, f.MinUnitPriceCLP, f.MaxUnitPriceCLP}
	if after != nil {
		switch f.Sort {
		case "unit_price_asc":
			query += ` AND (t.precio_base_clp,e.id) > ($5::bigint,$6::uuid)`
			args = append(args, after.LastUnitPriceCLP, after.LastSpaceID)
		case "unit_price_desc":
			query += ` AND (t.precio_base_clp,e.id) < ($5::bigint,$6::uuid)`
			args = append(args, after.LastUnitPriceCLP, after.LastSpaceID)
		default:
			query += ` AND (c.orden,lower(e.titulo) COLLATE "C",e.id) > ($5::smallint,$6::text COLLATE "C",$7::uuid)`
			args = append(args, after.LastCategoryOrder, after.LastTitleKey, after.LastSpaceID)
		}
	}
	switch f.Sort {
	case "unit_price_asc":
		query += ` ORDER BY t.precio_base_clp,e.id`
	case "unit_price_desc":
		query += ` ORDER BY t.precio_base_clp DESC,e.id DESC`
	default:
		query += ` ORDER BY c.orden,lower(e.titulo) COLLATE "C",e.id`
	}
	args = append(args, limit)
	query += ` LIMIT $` + itoa(len(args))
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]publiccatalog.Space, 0, limit)
	for rows.Next() {
		var v publiccatalog.Space
		if err := rows.Scan(&v.SpaceID, &v.CategoryCode, &v.CategoryName, &v.Title, &v.Description, &v.AreaM2, &v.RateUnit, &v.BasePriceCLP, &v.Currency, &v.CategoryOrder, &v.TitleKey); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (r *Repository) Detail(ctx context.Context, id string) (publiccatalog.Space, error) {
	var v publiccatalog.Space
	err := r.pool.QueryRow(ctx, projection+eligiblePublicSQL+` AND e.id=$1`, id).Scan(&v.SpaceID, &v.CategoryCode, &v.CategoryName, &v.Title, &v.Description, &v.AreaM2, &v.RateUnit, &v.BasePriceCLP, &v.Currency, &v.CategoryOrder, &v.TitleKey)
	if err == pgx.ErrNoRows {
		return publiccatalog.Space{}, publiccatalog.ErrNotFound
	}
	return v, err
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for v > 0 {
		i--
		b[i] = byte('0' + v%10)
		v /= 10
	}
	return string(b[i:])
}
