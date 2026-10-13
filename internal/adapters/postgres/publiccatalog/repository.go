package publiccatalogpg

import (
	"context"
	"database/sql"
	"encoding/json"

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

func (r *Repository) AttributeFilters(ctx context.Context, category string) ([]publiccatalog.AttributeFilter, error) {
	rows, err := r.pool.Query(ctx, `SELECT f.codigo,f.etiqueta,f.tipo,f.perfil_version
FROM public.atributo_filtro_publico f
JOIN public.categoria_espacio c ON c.codigo=f.categoria_codigo AND c.activa
WHERE f.categoria_codigo=$1 ORDER BY f.orden,f.codigo`, category)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]publiccatalog.AttributeFilter, 0, 4)
	for rows.Next() {
		var item publiccatalog.AttributeFilter
		if err := rows.Scan(&item.Code, &item.Label, &item.Type, &item.ProfileVersion); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) HasRegion(ctx context.Context, code string) (bool, error) {
	var active bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.region_publica r JOIN public.catalogo_ubicacion_publica v ON v.version=r.catalogo_version AND v.activo WHERE r.codigo=$1)`, code).Scan(&active)
	return active, err
}

func (r *Repository) HasCommune(ctx context.Context, region, commune string) (bool, error) {
	var active bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.comuna_publica c JOIN public.catalogo_ubicacion_publica v ON v.version=c.catalogo_version AND v.activo WHERE c.codigo=$1 AND c.region_codigo=$2)`, commune, region).Scan(&active)
	return active, err
}

func (r *Repository) Locations(ctx context.Context, query, region string, limit int) ([]publiccatalog.Location, error) {
	rows, err := r.pool.Query(ctx, `SELECT c.region_codigo,r.nombre,c.codigo,c.nombre
FROM public.comuna_publica c
JOIN public.region_publica r ON r.catalogo_version=c.catalogo_version AND r.codigo=c.region_codigo
JOIN public.catalogo_ubicacion_publica v ON v.version=c.catalogo_version AND v.activo
WHERE ($1='' OR c.region_codigo=$1)
  AND ($2='' OR strpos(lower(public.unaccent(c.nombre)),lower(public.unaccent($2)))>0
                OR strpos(lower(public.unaccent(r.nombre)),lower(public.unaccent($2)))>0)
ORDER BY c.region_codigo COLLATE "C",c.nombre COLLATE "C",c.codigo
LIMIT $3`, region, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]publiccatalog.Location, 0, limit)
	for rows.Next() {
		var item publiccatalog.Location
		if err := rows.Scan(&item.RegionCode, &item.RegionName, &item.CommuneCode, &item.CommuneName); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

const eligiblePublicSQL = ` FROM public.espacio e
JOIN public.categoria_espacio c ON c.codigo=e.categoria_codigo AND c.activa
JOIN LATERAL (SELECT modalidad,precio_base_clp,moneda FROM public.tarifa_espacio WHERE espacio_id=e.id ORDER BY version DESC LIMIT 1) t ON true
JOIN public.usuario owner ON owner.id=e.propietario_id AND owner.estado='activo'
LEFT JOIN public.region_publica r ON r.catalogo_version=e.ubicacion_catalogo_version AND r.codigo=e.region_codigo
LEFT JOIN public.comuna_publica m ON m.catalogo_version=e.ubicacion_catalogo_version AND m.region_codigo=e.region_codigo AND m.codigo=e.comuna_codigo
LEFT JOIN public.espacio_caracteristicas attrs ON attrs.espacio_id=e.id AND attrs.categoria_codigo=e.categoria_codigo
WHERE e.estado='activa'
AND NOT EXISTS(SELECT 1 FROM public.bloqueo_cuenta_administrativo_local b WHERE b.cuenta_id=owner.id)
AND EXISTS(SELECT 1 FROM public.elegibilidad_verificacion_local eligibility
 JOIN public.verificacion v ON v.id=eligibility.verificacion_id AND v.usuario_id=eligibility.usuario_id AND v.tipo=eligibility.tipo AND v.estado='aprobada'
 WHERE eligibility.usuario_id=owner.id AND eligibility.tipo='kyc' AND eligibility.estado='elegible')
AND NOT EXISTS(SELECT 1 FROM public.reserva_ensayo_local_fixture f WHERE f.espacio_id=e.id AND f.anfitrion_id=e.propietario_id AND f.habilitada)`

const projection = `SELECT e.id::text,e.categoria_codigo,c.nombre,e.titulo,e.descripcion,e.superficie_m2::double precision,t.modalidad,t.precio_base_clp,t.moneda,c.orden,lower(e.titulo) COLLATE "C"`

func (r *Repository) List(ctx context.Context, f publiccatalog.Filters, after *publiccatalog.Cursor, limit int) ([]publiccatalog.Space, error) {
	attributeJSON, err := json.Marshal(f.Attributes)
	if err != nil {
		return nil, err
	}
	if f.Attributes == nil {
		attributeJSON = []byte(`{}`)
	}
	var minArea, start, end any
	if f.MinAreaM2 != "" {
		minArea = f.MinAreaM2
	}
	if f.StartAt != nil {
		start = *f.StartAt
	}
	if f.EndAt != nil {
		end = *f.EndAt
	}
	args := []any{f.CategoryCode, f.RateUnit, f.MinUnitPriceCLP, f.MaxUnitPriceCLP, f.Query, f.RegionCode, f.CommuneCode, minArea, f.AttributeVersion, attributeJSON, start, end, f.MinTotalCLP, f.MaxTotalCLP}
	query := `WITH candidates AS (
SELECT e.id::text AS space_id,e.categoria_codigo,c.nombre AS category_name,e.titulo,e.descripcion,
       e.superficie_m2::double precision AS area_m2,t.modalidad,t.precio_base_clp,t.moneda,c.orden AS category_order,
       lower(e.titulo) COLLATE "C" AS title_key,e.zona_horaria,
       r.codigo AS region_code,r.nombre AS region_name,m.codigo AS commune_code,m.nombre AS commune_name,
       public.public_catalog_billed_units(t.modalidad,$11::timestamptz,$12::timestamptz,e.zona_horaria) AS billed_units,
       CASE WHEN $11::timestamptz IS NULL THEN NULL
            WHEN public.public_catalog_billed_units(t.modalidad,$11::timestamptz,$12::timestamptz,e.zona_horaria) IS NULL THEN NULL
            WHEN t.precio_base_clp::numeric * public.public_catalog_billed_units(t.modalidad,$11::timestamptz,$12::timestamptz,e.zona_horaria)::numeric > 9223372036854775807::numeric THEN NULL
            ELSE (t.precio_base_clp::numeric * public.public_catalog_billed_units(t.modalidad,$11::timestamptz,$12::timestamptz,e.zona_horaria)::numeric)::bigint END AS estimated_total,
       CASE WHEN $5='' THEN 0::real ELSE ts_rank(e.busqueda_publica,plainto_tsquery('public.espacigo_es_unaccent'::regconfig,$5)) END AS relevance_rank
` + eligiblePublicSQL + `
AND ($1='' OR e.categoria_codigo=$1)
  AND ($2='' OR t.modalidad=$2)
  AND ($3::bigint IS NULL OR t.precio_base_clp >= $3)
  AND ($4::bigint IS NULL OR t.precio_base_clp <= $4)
  AND ($5='' OR (numnode(plainto_tsquery('public.espacigo_es_unaccent'::regconfig,$5))>0 AND e.busqueda_publica @@ plainto_tsquery('public.espacigo_es_unaccent'::regconfig,$5)))
  AND ($6='' OR e.region_codigo=$6)
  AND ($7='' OR e.comuna_codigo=$7)
  AND ($8::numeric IS NULL OR e.superficie_m2 >= $8::numeric)
  AND ($10::jsonb='{}'::jsonb OR (attrs.perfil_version=$9 AND attrs.valores @> $10::jsonb))
)
SELECT space_id, categoria_codigo, category_name,titulo,descripcion,area_m2,modalidad,precio_base_clp,moneda,
       category_order,title_key,zona_horaria,region_code,region_name,commune_code,commune_name,estimated_total,relevance_rank
FROM candidates
WHERE ($13::bigint IS NULL OR estimated_total >= $13)
  AND ($14::bigint IS NULL OR estimated_total <= $14)`
	if after != nil {
		switch f.Sort {
		case "unit_price_asc":
			query += ` AND (precio_base_clp,space_id::uuid) > ($15::bigint,$16::uuid)`
			args = append(args, after.LastUnitPriceCLP, after.LastSpaceID)
		case "unit_price_desc":
			query += ` AND (precio_base_clp,space_id::uuid) < ($15::bigint,$16::uuid)`
			args = append(args, after.LastUnitPriceCLP, after.LastSpaceID)
		case "estimated_total_asc", "estimated_total_desc":
			if after.LastEstimatedTotalCLP == nil {
				query += ` AND estimated_total IS NULL AND space_id::uuid > $15::uuid`
				args = append(args, after.LastSpaceID)
			} else if f.Sort == "estimated_total_asc" {
				query += ` AND (estimated_total > $15::bigint OR (estimated_total=$15::bigint AND space_id::uuid>$16::uuid) OR estimated_total IS NULL)`
				args = append(args, *after.LastEstimatedTotalCLP, after.LastSpaceID)
			} else {
				query += ` AND (estimated_total < $15::bigint OR (estimated_total=$15::bigint AND space_id::uuid>$16::uuid) OR estimated_total IS NULL)`
				args = append(args, *after.LastEstimatedTotalCLP, after.LastSpaceID)
			}
		case "relevance":
			query += ` AND (relevance_rank < $15::real OR (relevance_rank=$15::real AND space_id::uuid>$16::uuid))`
			args = append(args, after.LastRelevanceRank, after.LastSpaceID)
		default:
			query += ` AND (category_order,title_key COLLATE "C",space_id::uuid) > ($15::smallint,$16::text COLLATE "C",$17::uuid)`
			args = append(args, after.LastCategoryOrder, after.LastTitleKey, after.LastSpaceID)
		}
	}
	switch f.Sort {
	case "unit_price_asc":
		query += ` ORDER BY precio_base_clp,space_id::uuid`
	case "unit_price_desc":
		query += ` ORDER BY precio_base_clp DESC,space_id::uuid DESC`
	case "estimated_total_asc":
		query += ` ORDER BY estimated_total ASC NULLS LAST,space_id::uuid`
	case "estimated_total_desc":
		query += ` ORDER BY estimated_total DESC NULLS LAST,space_id::uuid`
	case "relevance":
		query += ` ORDER BY relevance_rank DESC,space_id::uuid`
	default:
		query += ` ORDER BY category_order,title_key COLLATE "C",space_id::uuid`
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
		var timeZone, regionCode, regionName, communeCode, communeName sql.NullString
		var estimatedTotal sql.NullInt64
		if err := rows.Scan(&v.SpaceID, &v.CategoryCode, &v.CategoryName, &v.Title, &v.Description, &v.AreaM2, &v.RateUnit, &v.BasePriceCLP, &v.Currency, &v.CategoryOrder, &v.TitleKey, &timeZone, &regionCode, &regionName, &communeCode, &communeName, &estimatedTotal, &v.RelevanceRank); err != nil {
			return nil, err
		}
		if timeZone.Valid {
			v.TimeZone = timeZone.String
		}
		if regionCode.Valid {
			v.RegionCode = &regionCode.String
		}
		if regionName.Valid {
			v.RegionName = &regionName.String
		}
		if communeCode.Valid {
			v.CommuneCode = &communeCode.String
		}
		if communeName.Valid {
			v.CommuneName = &communeName.String
		}
		if estimatedTotal.Valid {
			v.EstimatedTotalCLP = &estimatedTotal.Int64
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
