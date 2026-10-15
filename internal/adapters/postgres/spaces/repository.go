package spacespg

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
	_ "time/tzdata"

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

const fields = `e.id::text,e.categoria_codigo,c.nombre,e.titulo,e.descripcion,e.superficie_m2::float8,e.capacidad_maxima,e.reglas_uso,e.modalidad_tarifa,e.precio_base_clp,e.direccion,e.estado,COALESCE(ec.perfil_version,1),COALESCE(ec.valores,'{}'::jsonb),e.zona_horaria,COALESCE(e.portada_foto_id::text,''),e.region_codigo,e.comuna_codigo`
const joins = ` FROM public.espacio e JOIN public.categoria_espacio c ON c.codigo=e.categoria_codigo LEFT JOIN public.espacio_caracteristicas ec ON ec.espacio_id=e.id`

func scan(row pgx.Row) (spaces.Draft, error) {
	var d spaces.Draft
	var raw []byte
	var coverPhotoID string
	err := row.Scan(&d.ID, &d.CategoryCode, &d.CategoryName, &d.Title, &d.Description, &d.AreaM2, &d.Capacity, &d.UsageRules, &d.RateUnit, &d.BasePriceCLP, &d.Address, &d.State, &d.AttributeSchemaVersion, &raw, &d.TimeZone, &coverPhotoID, &d.RegionCode, &d.CommuneCode)
	if err == nil {
		if coverPhotoID != "" {
			d.CoverPhotoID = &coverPhotoID
		}
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
	var blocked bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.bloqueo_cuenta_administrativo_local WHERE cuenta_id=$1)`, owner).Scan(&blocked); err != nil {
		return err
	}
	if blocked {
		return spaces.ErrPublicationConflict
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
	_, err = tx.Exec(ctx, `INSERT INTO public.espacio(id,propietario_id,categoria_codigo,titulo,descripcion,superficie_m2,capacidad_maxima,reglas_uso,modalidad_tarifa,precio_base_clp,direccion,ubicacion_catalogo_version,region_codigo,comuna_codigo) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,CASE WHEN $12='' THEN NULL ELSE 'SUBDERE_DPA_2018' END,NULLIF($12,''),NULLIF($13,''))`, id, owner, in.CategoryCode, in.Title, in.Description, in.AreaM2, in.Capacity, in.UsageRules, in.RateUnit, in.BasePriceCLP, in.Address, in.RegionCode, in.CommuneCode)
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
	rows, err := r.pool.Query(ctx, `SELECT `+fields+joins+` WHERE e.propietario_id=$1 AND e.estado<>'eliminada' ORDER BY e.actualizado_en DESC,e.id`, owner)
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
	return scan(r.pool.QueryRow(ctx, `SELECT `+fields+joins+` WHERE e.propietario_id=$1 AND e.id=$2 AND e.estado<>'eliminada'`, owner, id))
}

func (r *Repository) DeleteOwn(ctx context.Context, owner, id, correlationID string) (spaces.Deletion, error) {
	return r.delete(ctx, owner, owner, id, correlationID, time.Time{}, "", false)
}

func (r *Repository) DeleteAsAdministrator(ctx context.Context, actor, owner, id, correlationID string, expectedUpdatedAt time.Time, reason string) (spaces.Deletion, error) {
	return r.delete(ctx, actor, owner, id, correlationID, expectedUpdatedAt, reason, true)
}

func (r *Repository) delete(ctx context.Context, actor, owner, id, correlationID string, expectedUpdatedAt time.Time, reason string, administrative bool) (spaces.Deletion, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return spaces.Deletion{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if administrative {
		if actor == owner {
			return spaces.Deletion{}, spaces.ErrNotFound
		}
		if actor < owner {
			if err = lockGlobalAdministrator(ctx, tx, actor); err != nil {
				return spaces.Deletion{}, err
			}
			if err = lockActiveRetirementOwner(ctx, tx, owner); err != nil {
				return spaces.Deletion{}, err
			}
		} else {
			if err = lockActiveRetirementOwner(ctx, tx, owner); err != nil {
				return spaces.Deletion{}, err
			}
			if err = lockGlobalAdministrator(ctx, tx, actor); err != nil {
				return spaces.Deletion{}, err
			}
		}
	} else if err = lockActiveOwner(ctx, tx, owner); err != nil {
		return spaces.Deletion{}, err
	}
	var state string
	var updatedAt time.Time
	err = tx.QueryRow(ctx, `SELECT estado,actualizado_en FROM public.espacio WHERE propietario_id=$1 AND id=$2 FOR UPDATE`, owner, id).Scan(&state, &updatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return spaces.Deletion{}, spaces.ErrNotFound
	}
	if err != nil {
		return spaces.Deletion{}, err
	}
	var prior spaces.Deletion
	var summary []byte
	err = tx.QueryRow(ctx, `SELECT espacio_id::text,ocurrida_en,archivos_pendientes,resumen FROM public.espacio_eliminacion_historial_local WHERE espacio_id=$1`, id).
		Scan(&prior.SpaceID, &prior.OccurredAt, &prior.FilesPending, &summary)
	if err == nil {
		if err = json.Unmarshal(summary, &prior); err != nil {
			return spaces.Deletion{}, err
		}
		prior.SpaceID = id
		prior.State = "eliminada"
		prior.Reused = true
		if err = tx.Commit(ctx); err != nil {
			return spaces.Deletion{}, err
		}
		return prior, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return spaces.Deletion{}, err
	}
	if administrative && !updatedAt.Equal(expectedUpdatedAt) {
		return spaces.Deletion{}, spaces.ErrPublicationConflict
	}
	if state != "activa" && state != "oculta" {
		return spaces.Deletion{}, spaces.ErrPublicationConflict
	}
	// The space row lock serializes this check with quote/reservation creation.
	// Use the database clock only after both owner and space locks are held.
	var now time.Time
	if err = tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&now); err != nil {
		return spaces.Deletion{}, err
	}
	var blocked bool
	err = tx.QueryRow(ctx, `SELECT
		EXISTS(SELECT 1 FROM public.reserva_ensayo_local r WHERE r.espacio_id=$1
		  AND r.estado NOT IN ('finalizada','cancelada_por_pago','rechazada_arrendador','vencida_pago','vencida_host','cancelada_arrendatario','cancelada_por_firma','cerrada'))
		OR EXISTS(SELECT 1 FROM public.reserva_ensayo_local r WHERE r.espacio_id=$1 AND r.estado='finalizada'
		  AND ((SELECT max(o.ocurrio_en) FROM public.operacion_arriendo_ensayo_local o WHERE o.reserva_id=r.id AND o.tipo='checkout') IS NULL
		       OR $2 < (SELECT max(o.ocurrio_en) + interval '24 hours' FROM public.operacion_arriendo_ensayo_local o WHERE o.reserva_id=r.id AND o.tipo='checkout')
		       OR NOT EXISTS(SELECT 1 FROM public.reserva_liquidacion_ensayo_local l WHERE l.reserva_id=r.id AND l.estado='liquidada')
		       OR EXISTS(SELECT 1 FROM public.reclamo_dano_ensayo_local c WHERE c.reserva_id=r.id AND c.estado='abierto')))
		OR EXISTS(SELECT 1 FROM public.reclamo_dano_ensayo_local c JOIN public.reserva_ensayo_local r ON r.id=c.reserva_id WHERE r.espacio_id=$1 AND c.estado='abierto')
		OR EXISTS(SELECT 1 FROM public.disputa_ensayo_local d JOIN public.reserva_ensayo_local r ON r.id=d.reserva_id WHERE r.espacio_id=$1 AND d.estado='abierta')
		OR EXISTS(SELECT 1 FROM public.reserva_devolucion_ensayo d JOIN public.reserva_ensayo_local r ON r.id=d.reserva_id WHERE r.espacio_id=$1 AND d.estado='pendiente')
		OR EXISTS(SELECT 1 FROM public.reserva_pago_ensayo_operacion o JOIN public.reserva_ensayo_local r ON r.id=o.reserva_id WHERE r.espacio_id=$1 AND o.estado IN ('pendiente','por_conciliar'))
		OR EXISTS(SELECT 1 FROM public.reserva_pago_evento_aplicacion_ensayo a JOIN public.reserva_pago_evento_ensayo e ON e.id=a.evento_id JOIN public.reserva_pago_ensayo_operacion o ON o.id=e.operacion_id JOIN public.reserva_ensayo_local r ON r.id=o.reserva_id WHERE r.espacio_id=$1 AND a.estado IN ('pendiente','pendiente_conciliacion'))
		OR EXISTS(SELECT 1 FROM public.reserva_garantia_ensayo_local g JOIN public.reserva_ensayo_local r ON r.id=g.reserva_id WHERE r.espacio_id=$1
		  AND (g.estado IN ('pendiente_pago','pendiente_autorizacion','por_conciliar','captura_pendiente','captura_por_conciliar','liberacion_pendiente','liberacion_por_conciliar')
		       OR g.autorizado_clp > g.capturado_clp + g.liberado_clp))
		OR EXISTS(SELECT 1 FROM public.reserva_garantia_operacion_ensayo_local o JOIN public.reserva_garantia_ensayo_local g ON g.id=o.garantia_id JOIN public.reserva_ensayo_local r ON r.id=g.reserva_id WHERE r.espacio_id=$1 AND o.estado IN ('pendiente','por_conciliar'))
		OR EXISTS(SELECT 1 FROM public.reserva_garantia_evento_aplicacion_ensayo_local a JOIN public.reserva_garantia_evento_ensayo_local e ON e.id=a.evento_id JOIN public.reserva_garantia_operacion_ensayo_local o ON o.id=e.operacion_id JOIN public.reserva_garantia_ensayo_local g ON g.id=o.garantia_id JOIN public.reserva_ensayo_local r ON r.id=g.reserva_id WHERE r.espacio_id=$1 AND a.estado IN ('pendiente','por_conciliar'))
		OR EXISTS(SELECT 1 FROM public.reserva_decision_financiera_ensayo_local d JOIN public.reserva_ensayo_local r ON r.id=d.reserva_id WHERE r.espacio_id=$1 AND d.estado IN ('pendiente','por_conciliar'))
		OR EXISTS(SELECT 1 FROM public.reserva_liquidacion_ensayo_local l JOIN public.reserva_ensayo_local r ON r.id=l.reserva_id WHERE r.espacio_id=$1 AND l.estado<>'liquidada')`, id, now).Scan(&blocked)
	if err != nil {
		return spaces.Deletion{}, err
	}
	if blocked {
		return spaces.Deletion{}, spaces.ErrDeleteBlocked
	}
	var fixtureEnabled bool
	err = tx.QueryRow(ctx, `SELECT habilitada FROM public.reserva_ensayo_local_fixture WHERE espacio_id=$1 FOR UPDATE`, id).Scan(&fixtureEnabled)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return spaces.Deletion{}, err
	}
	if err == nil && fixtureEnabled {
		return spaces.Deletion{}, spaces.ErrDeleteBlocked
	}
	var filesPending int
	if err = tx.QueryRow(ctx, `SELECT (SELECT count(*) FROM public.espacio_galeria_sintetica_local WHERE espacio_id=$1 AND archivo_id IS NOT NULL AND estado<>'activa') + (SELECT count(*) FROM public.espacio_galeria_sintetica_local WHERE espacio_id=$1 AND estado='activa' AND archivo_id IS NOT NULL) + (SELECT count(*) FROM public.espacio_galeria_archivo_candidato_local WHERE espacio_id=$1)`, id).Scan(&filesPending); err != nil {
		return spaces.Deletion{}, err
	}
	// Retire gallery metadata and queue bytes for the already durable cleanup worker.
	if _, err = tx.Exec(ctx, `UPDATE public.espacio_galeria_sintetica_local SET estado='retirada',retirada_en=$2,proximo_intento_en=$2,ultimo_codigo_error='publicacion_eliminada' WHERE espacio_id=$1 AND estado='activa'`, id, now); err != nil {
		return spaces.Deletion{}, err
	}
	if _, err = tx.Exec(ctx, `UPDATE public.espacio_galeria_archivo_candidato_local SET estado='pendiente_limpieza',proximo_intento_en=$2,ultimo_codigo_error='publicacion_eliminada' WHERE espacio_id=$1 AND estado='reservado'`, id, now); err != nil {
		return spaces.Deletion{}, err
	}
	if _, err = tx.Exec(ctx, `UPDATE public.espacio SET portada_foto_id=NULL WHERE id=$1`, id); err != nil {
		return spaces.Deletion{}, err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM public.reserva_ensayo_local_fixture WHERE espacio_id=$1 AND NOT habilitada`, id); err != nil {
		return spaces.Deletion{}, err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM public.espacio_horario_semanal WHERE espacio_id=$1`, id); err != nil {
		return spaces.Deletion{}, err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM public.espacio_caracteristicas WHERE espacio_id=$1`, id); err != nil {
		return spaces.Deletion{}, err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM public.simulacion_precio_privada WHERE espacio_id=$1`, id); err != nil {
		return spaces.Deletion{}, err
	}
	if _, err = tx.Exec(ctx, `UPDATE public.espacio SET estado='eliminada',titulo='Publicación retirada',descripcion='Contenido retirado. Los hechos históricos y los snapshots necesarios se conservan según la política local ratificada.',superficie_m2=0.01,capacidad_maxima=1,reglas_uso='Retirado',modalidad_tarifa='hora',precio_base_clp=5001,direccion='Retirada',zona_horaria=NULL,actualizado_en=$2 WHERE id=$1`, id, now); err != nil {
		return spaces.Deletion{}, mapError(err)
	}
	if _, err = tx.Exec(ctx, `SELECT public.purge_unreferenced_local_publication_rates($1)`, id); err != nil {
		return spaces.Deletion{}, err
	}
	removed := []string{"catalogo", "contenido_publicacion", "atributos", "horario_semanal", "simulaciones_privadas", "fixture_deshabilitado"}
	preserved := []string{"espacio_tecnico", "reservas_y_ocupaciones", "cotizaciones_y_snapshots", "contratos_pagos_reclamos_e_historiales", "tarifas_referenciadas", "historial_publicacion", "galeria_en_limpieza_durable"}
	result := spaces.Deletion{SpaceID: id, State: "eliminada", OccurredAt: now.UTC(), Removed: removed, Preserved: preserved, FilesPending: filesPending}
	encoded, err := json.Marshal(result)
	if err != nil {
		return spaces.Deletion{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO public.espacio_eliminacion_historial_local(espacio_id,actor_id,ocurrida_en,correlacion_id,archivos_pendientes,resumen) VALUES($1,$2,$3,$4,$5,$6::jsonb)`, id, actor, now, correlationID, filesPending, encoded); err != nil {
		return spaces.Deletion{}, mapError(err)
	}
	if administrative {
		auditID, idErr := r.ids.ID()
		if idErr != nil {
			return spaces.Deletion{}, idErr
		}
		if _, err = tx.Exec(ctx, `INSERT INTO public.evento_auditoria_local(id,actor_id,cuenta_objetivo_id,recurso_tipo,recurso_id,accion,resultado,motivo_codigo,correlacion_id,ocurrido_en,registrado_en,retirar_en) VALUES($1,$2,$3,'publicacion_global',$4,'admin.global.publications.retire','exito',$5,$6,$7,clock_timestamp(),$7::timestamptz+interval '5 years')`, auditID, actor, owner, id, reason, correlationID, now); err != nil {
			return spaces.Deletion{}, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return spaces.Deletion{}, err
	}
	return result, nil
}

func lockGlobalAdministrator(ctx context.Context, tx pgx.Tx, actor string) error {
	active, err := accountlock.LockActive(ctx, tx, actor)
	if err != nil {
		return err
	}
	if !active {
		return spaces.ErrNotFound
	}
	var allowed bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.administrador_global g JOIN public.usuario u ON u.id=g.usuario_id WHERE g.usuario_id=$1 AND (u.bloqueado_hasta IS NULL OR u.bloqueado_hasta<=now()) AND NOT EXISTS(SELECT 1 FROM public.bloqueo_cuenta_administrativo_local b WHERE b.cuenta_id=u.id))`, actor).Scan(&allowed); err != nil {
		return err
	}
	if !allowed {
		return spaces.ErrNotFound
	}
	return nil
}

func lockActiveRetirementOwner(ctx context.Context, tx pgx.Tx, owner string) error {
	active, err := accountlock.LockActive(ctx, tx, owner)
	if err != nil {
		return err
	}
	if !active {
		return spaces.ErrNotFound
	}
	var blocked bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.bloqueo_cuenta_administrativo_local WHERE cuenta_id=$1)`, owner).Scan(&blocked); err != nil {
		return err
	}
	if blocked {
		return spaces.ErrPublicationConflict
	}
	return nil
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
	if state == "activa" && !validIANAZone(draft.TimeZone) {
		return spaces.Draft{}, spaces.ErrTimeZoneRequired
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

func validIANAZone(zone *string) bool {
	if zone == nil || strings.TrimSpace(*zone) == "" || strings.TrimSpace(*zone) != *zone || *zone == "Local" || strings.Contains(*zone, "..") {
		return false
	}
	_, err := time.LoadLocation(*zone)
	return err == nil
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
	var oldTitle, oldDescription, oldUsageRules, rateUnit, oldRegionCode, oldCommuneCode string
	var oldCapacity int32
	var oldPrice int64
	err = tx.QueryRow(ctx, `SELECT titulo,descripcion,capacidad_maxima,reglas_uso,modalidad_tarifa,precio_base_clp,COALESCE(region_codigo,''),COALESCE(comuna_codigo,'') FROM public.espacio
		WHERE propietario_id=$1 AND id=$2 AND estado IN ('activa','oculta') FOR UPDATE`, owner, id).Scan(&oldTitle, &oldDescription, &oldCapacity, &oldUsageRules, &rateUnit, &oldPrice, &oldRegionCode, &oldCommuneCode)
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
	var nextLocationVersion, nextRegionCode, nextCommuneCode any
	updateLocation := false
	if in.RegionCode != nil && in.CommuneCode != nil && (*in.RegionCode != oldRegionCode || *in.CommuneCode != oldCommuneCode) {
		updateLocation = true
		if *in.RegionCode != "" {
			nextLocationVersion, nextRegionCode, nextCommuneCode = spaces.PublicLocationCatalogVersion, *in.RegionCode, *in.CommuneCode
		}
	}
	if nextTitle != nil || nextDescription != nil || nextCapacity != nil || nextUsageRules != nil || nextPrice != nil || updateLocation {
		if _, err = tx.Exec(ctx, `UPDATE public.espacio SET titulo=COALESCE($3,titulo),descripcion=COALESCE($4,descripcion),capacidad_maxima=COALESCE($5,capacidad_maxima),reglas_uso=COALESCE($6,reglas_uso),precio_base_clp=COALESCE($7,precio_base_clp),ubicacion_catalogo_version=CASE WHEN $8 THEN $9 ELSE ubicacion_catalogo_version END,region_codigo=CASE WHEN $8 THEN $10 ELSE region_codigo END,comuna_codigo=CASE WHEN $8 THEN $11 ELSE comuna_codigo END,actualizado_en=now()
			WHERE propietario_id=$1 AND id=$2 AND estado IN ('activa','oculta')`, owner, id, nextTitle, nextDescription, nextCapacity, nextUsageRules, nextPrice, updateLocation, nextLocationVersion, nextRegionCode, nextCommuneCode); err != nil {
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
	tag, err := tx.Exec(ctx, `UPDATE public.espacio SET categoria_codigo=$3,titulo=$4,descripcion=$5,superficie_m2=$6,capacidad_maxima=$7,reglas_uso=$8,modalidad_tarifa=$9,precio_base_clp=$10,direccion=$11,ubicacion_catalogo_version=CASE WHEN $12='' THEN NULL ELSE 'SUBDERE_DPA_2018' END,region_codigo=NULLIF($12,''),comuna_codigo=NULLIF($13,''),actualizado_en=now() WHERE propietario_id=$1 AND id=$2 AND estado='borrador'`, owner, id, in.CategoryCode, in.Title, in.Description, in.AreaM2, in.Capacity, in.UsageRules, in.RateUnit, in.BasePriceCLP, in.Address, in.RegionCode, in.CommuneCode)
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
