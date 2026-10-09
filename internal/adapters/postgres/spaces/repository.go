package spacespg

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/accountlock"
	verificationdb "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/verification/dbgen"
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

const fields = `e.id::text,e.categoria_codigo,c.nombre,e.titulo,e.descripcion,e.superficie_m2::float8,e.capacidad_maxima,e.reglas_uso,e.modalidad_tarifa,e.precio_base_clp,e.direccion,e.estado,COALESCE(ec.perfil_version,1),COALESCE(ec.valores,'{}'::jsonb)`
const joins = ` FROM public.espacio e JOIN public.categoria_espacio c ON c.codigo=e.categoria_codigo LEFT JOIN public.espacio_caracteristicas ec ON ec.espacio_id=e.id`

func scan(row pgx.Row) (spaces.Draft, error) {
	var d spaces.Draft
	var raw []byte
	err := row.Scan(&d.ID, &d.CategoryCode, &d.CategoryName, &d.Title, &d.Description, &d.AreaM2, &d.Capacity, &d.UsageRules, &d.RateUnit, &d.BasePriceCLP, &d.Address, &d.State, &d.AttributeSchemaVersion, &raw)
	if err == nil {
		d.Attributes = map[string]any{}
		err = json.Unmarshal(raw, &d.Attributes)
	}
	return d, mapError(err)
}

// lockActiveOwner shares the same row lock as privacy suppression, so a draft
// write authorized before the request cannot recreate scrubbed content after it.
func lockActiveOwner(ctx context.Context, tx pgx.Tx, owner string) error {
	active, err := accountlock.LockActive(ctx, tx, owner)
	if err != nil {
		return err
	}
	if !active {
		return spaces.ErrNotFound
	}
	return nil
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
func (r *Repository) Profile(ctx context.Context, category string, version int) (spaces.Profile, error) {
	var raw []byte
	var err error
	if version > 0 {
		err = r.pool.QueryRow(ctx, `SELECT perfil FROM public.categoria_perfil_atributos WHERE categoria_codigo=$1 AND version=$2`, category, version).Scan(&raw)
	} else {
		err = r.pool.QueryRow(ctx, `SELECT perfil FROM public.categoria_perfil_atributos WHERE categoria_codigo=$1 ORDER BY version DESC LIMIT 1`, category).Scan(&raw)
	}
	if err != nil {
		return spaces.Profile{}, mapError(err)
	}
	var p spaces.Profile
	if err = json.Unmarshal(raw, &p); err != nil {
		return spaces.Profile{}, err
	}
	return p, nil
}
func (r *Repository) Create(ctx context.Context, owner string, in spaces.Input) (spaces.Draft, error) {
	id, err := r.ids.ID()
	if err != nil {
		return spaces.Draft{}, err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return spaces.Draft{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockActiveOwner(ctx, tx, owner); err != nil {
		return spaces.Draft{}, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO public.espacio(id,propietario_id,categoria_codigo,titulo,descripcion,superficie_m2,capacidad_maxima,reglas_uso,modalidad_tarifa,precio_base_clp,direccion) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, id, owner, in.CategoryCode, in.Title, in.Description, in.AreaM2, in.Capacity, in.UsageRules, in.RateUnit, in.BasePriceCLP, in.Address)
	if err != nil {
		return spaces.Draft{}, mapError(err)
	}
	values, err := json.Marshal(in.Attributes)
	if err != nil {
		return spaces.Draft{}, spaces.ErrInvalid
	}
	_, err = tx.Exec(ctx, `INSERT INTO public.espacio_caracteristicas(espacio_id,categoria_codigo,perfil_version,valores) VALUES($1,$2,$3,$4::jsonb)`, id, in.CategoryCode, in.AttributeSchemaVersion, values)
	if err != nil {
		return spaces.Draft{}, mapError(err)
	}
	_, err = tx.Exec(ctx, `INSERT INTO public.tarifa_espacio(espacio_id,version,modalidad,precio_base_clp) VALUES($1,1,$2,$3)`, id, in.RateUnit, in.BasePriceCLP)
	if err != nil {
		return spaces.Draft{}, mapError(err)
	}
	d, err := scan(tx.QueryRow(ctx, `SELECT `+fields+joins+` WHERE e.id=$1`, id))
	if err != nil {
		return spaces.Draft{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return spaces.Draft{}, err
	}
	return d, nil
}
func (r *Repository) ListOwn(ctx context.Context, owner string) ([]spaces.Draft, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+fields+joins+` WHERE e.propietario_id=$1 ORDER BY e.actualizado_en DESC,e.id`, owner)
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
	return scan(r.pool.QueryRow(ctx, `SELECT `+fields+joins+` WHERE e.propietario_id=$1 AND e.id=$2`, owner, id))
}

func (r *Repository) SetPublicationState(ctx context.Context, owner, id, state, correlationID string) (spaces.Draft, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return spaces.Draft{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockActiveOwner(ctx, tx, owner); err != nil {
		return spaces.Draft{}, err
	}
	draft, err := scan(tx.QueryRow(ctx, `SELECT `+fields+joins+` WHERE e.propietario_id=$1 AND e.id=$2 FOR UPDATE OF e`, owner, id))
	if err != nil {
		return spaces.Draft{}, err
	}
	if draft.State == state {
		if err := tx.Commit(ctx); err != nil {
			return spaces.Draft{}, err
		}
		return draft, nil
	}
	// Keep enabled synthetic fixtures in the existing draft-only catalog and
	// booking flow. Lock the fixture row in the same transaction as the space
	// transition so an enable/disable operation cannot race this decision.
	var fixtureEnabled bool
	err = tx.QueryRow(ctx, `SELECT habilitada FROM public.reserva_ensayo_local_fixture WHERE espacio_id=$1 FOR UPDATE`, id).Scan(&fixtureEnabled)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return spaces.Draft{}, err
	}
	if err == nil && fixtureEnabled {
		return spaces.Draft{}, spaces.ErrEnabledFixture
	}
	if state == "activa" {
		if draft.State != "borrador" && draft.State != "oculta" {
			return spaces.Draft{}, spaces.ErrPublicationConflict
		}
		rows, err := verificationdb.New(tx).ListSyntheticEligibility(ctx, owner)
		if err != nil {
			return spaces.Draft{}, err
		}
		kycEligible := false
		for _, row := range rows {
			kind, kindOK := row.Type.(string)
			eligible, eligibleOK := row.Eligible.(bool)
			if !kindOK || !eligibleOK {
				return spaces.Draft{}, spaces.ErrInvalid
			}
			if kind == "kyc" {
				kycEligible = eligible
				break
			}
		}
		if !kycEligible {
			return spaces.Draft{}, spaces.ErrEligibilityRequired
		}
	} else if state == "oculta" {
		if draft.State != "activa" {
			return spaces.Draft{}, spaces.ErrPublicationConflict
		}
	} else {
		return spaces.Draft{}, spaces.ErrInvalid
	}
	if _, err = tx.Exec(ctx, `INSERT INTO public.espacio_publicacion_historial_local
		(espacio_id,actor_id,estado_anterior,estado_nuevo,ocurrida_en,correlacion_id)
		VALUES($1,$2,$3,$4,now(),$5)`, id, owner, draft.State, state, correlationID); err != nil {
		return spaces.Draft{}, mapError(err)
	}
	if _, err = tx.Exec(ctx, `UPDATE public.espacio SET estado=$3,actualizado_en=now() WHERE propietario_id=$1 AND id=$2`, owner, id, state); err != nil {
		return spaces.Draft{}, mapError(err)
	}
	draft, err = scan(tx.QueryRow(ctx, `SELECT `+fields+joins+` WHERE e.propietario_id=$1 AND e.id=$2`, owner, id))
	if err != nil {
		return spaces.Draft{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return spaces.Draft{}, err
	}
	return draft, nil
}

func (r *Repository) UpdatePublishedOwn(ctx context.Context, owner, id string, in spaces.PublishedContentInput) (spaces.Draft, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return spaces.Draft{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockActiveOwner(ctx, tx, owner); err != nil {
		return spaces.Draft{}, err
	}
	var oldTitle, oldDescription, oldUsageRules, rateUnit string
	var oldCapacity int32
	var oldPrice int64
	err = tx.QueryRow(ctx, `SELECT titulo,descripcion,capacidad_maxima,reglas_uso,modalidad_tarifa,precio_base_clp FROM public.espacio
		WHERE propietario_id=$1 AND id=$2 AND estado IN ('activa','oculta') FOR UPDATE`, owner, id).Scan(&oldTitle, &oldDescription, &oldCapacity, &oldUsageRules, &rateUnit, &oldPrice)
	if err != nil {
		return spaces.Draft{}, mapError(err)
	}
	var nextPrice any
	if in.BasePriceCLP != nil && *in.BasePriceCLP != oldPrice {
		var version int32
		if err = tx.QueryRow(ctx, `SELECT COALESCE(max(version),0)+1 FROM public.tarifa_espacio WHERE espacio_id=$1`, id).Scan(&version); err != nil {
			return spaces.Draft{}, err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO public.tarifa_espacio(espacio_id,version,modalidad,precio_base_clp)
			VALUES($1,$2,$3,$4)`, id, version, rateUnit, *in.BasePriceCLP); err != nil {
			return spaces.Draft{}, mapError(err)
		}
		nextPrice = *in.BasePriceCLP
	}
	var nextTitle any
	if in.Title != nil && *in.Title != oldTitle {
		nextTitle = *in.Title
	}
	var nextDescription any
	if in.Description != nil && *in.Description != oldDescription {
		nextDescription = *in.Description
	}
	var nextCapacity any
	if in.Capacity != nil && *in.Capacity != oldCapacity {
		nextCapacity = *in.Capacity
	}
	var nextUsageRules any
	if in.UsageRules != nil && *in.UsageRules != oldUsageRules {
		nextUsageRules = *in.UsageRules
	}
	if nextTitle != nil || nextDescription != nil || nextCapacity != nil || nextUsageRules != nil || nextPrice != nil {
		if _, err = tx.Exec(ctx, `UPDATE public.espacio SET titulo=COALESCE($3,titulo),descripcion=COALESCE($4,descripcion),capacidad_maxima=COALESCE($5,capacidad_maxima),reglas_uso=COALESCE($6,reglas_uso),precio_base_clp=COALESCE($7,precio_base_clp),actualizado_en=now()
			WHERE propietario_id=$1 AND id=$2 AND estado IN ('activa','oculta')`, owner, id, nextTitle, nextDescription, nextCapacity, nextUsageRules, nextPrice); err != nil {
			return spaces.Draft{}, mapError(err)
		}
	}
	draft, err := scan(tx.QueryRow(ctx, `SELECT `+fields+joins+` WHERE e.propietario_id=$1 AND e.id=$2`, owner, id))
	if err != nil {
		return spaces.Draft{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return spaces.Draft{}, err
	}
	return draft, nil
}

func (r *Repository) UpdateOwn(ctx context.Context, owner, id string, in spaces.Input) (spaces.Draft, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return spaces.Draft{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockActiveOwner(ctx, tx, owner); err != nil {
		return spaces.Draft{}, err
	}
	var oldUnit string
	var oldAmount int64
	err = tx.QueryRow(ctx, `SELECT modalidad_tarifa,precio_base_clp FROM public.espacio WHERE propietario_id=$1 AND id=$2 AND estado='borrador' FOR UPDATE`, owner, id).Scan(&oldUnit, &oldAmount)
	if err != nil {
		return spaces.Draft{}, mapError(err)
	}
	_, err = tx.Exec(ctx, `DELETE FROM public.espacio_caracteristicas WHERE espacio_id=$1`, id)
	if err != nil {
		return spaces.Draft{}, mapError(err)
	}
	tag, err := tx.Exec(ctx, `UPDATE public.espacio SET categoria_codigo=$3,titulo=$4,descripcion=$5,superficie_m2=$6,capacidad_maxima=$7,reglas_uso=$8,modalidad_tarifa=$9,precio_base_clp=$10,direccion=$11,actualizado_en=now() WHERE propietario_id=$1 AND id=$2 AND estado='borrador'`, owner, id, in.CategoryCode, in.Title, in.Description, in.AreaM2, in.Capacity, in.UsageRules, in.RateUnit, in.BasePriceCLP, in.Address)
	if err != nil {
		return spaces.Draft{}, mapError(err)
	}
	if tag.RowsAffected() != 1 {
		return spaces.Draft{}, spaces.ErrNotFound
	}
	if oldUnit != in.RateUnit || oldAmount != in.BasePriceCLP {
		_, err = tx.Exec(ctx, `INSERT INTO public.tarifa_espacio(espacio_id,version,modalidad,precio_base_clp) SELECT $1,COALESCE(max(version),0)+1,$2,$3 FROM public.tarifa_espacio WHERE espacio_id=$1`, id, in.RateUnit, in.BasePriceCLP)
		if err != nil {
			return spaces.Draft{}, mapError(err)
		}
	}
	values, err := json.Marshal(in.Attributes)
	if err != nil {
		return spaces.Draft{}, spaces.ErrInvalid
	}
	_, err = tx.Exec(ctx, `INSERT INTO public.espacio_caracteristicas(espacio_id,categoria_codigo,perfil_version,valores) VALUES($1,$2,$3,$4::jsonb)`, id, in.CategoryCode, in.AttributeSchemaVersion, values)
	if err != nil {
		return spaces.Draft{}, mapError(err)
	}
	d, err := scan(tx.QueryRow(ctx, `SELECT `+fields+joins+` WHERE e.id=$1`, id))
	if err != nil {
		return spaces.Draft{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return spaces.Draft{}, err
	}
	return d, nil
}
func mapError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return spaces.ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && (pgErr.Code == "23503" || pgErr.Code == "23514" || pgErr.Code == "22003" || pgErr.Code == "23505") {
		return spaces.ErrInvalid
	}
	return err
}
