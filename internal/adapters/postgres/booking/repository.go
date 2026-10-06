package bookingpg

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/booking"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func (r *Repository) Fixture(ctx context.Context, actor string) (booking.Fixture, error) {
	items, err := r.Catalog(ctx, actor, booking.CatalogFilter{})
	if err != nil {
		return booking.Fixture{}, err
	}
	if len(items) == 0 {
		return booking.Fixture{}, booking.ErrNotFound
	}
	item := items[0]
	var f booking.Fixture
	err = r.pool.QueryRow(ctx, `SELECT anfitrion_id::text,arrendatario_id::text FROM public.reserva_ensayo_local_fixture WHERE espacio_id=$1 AND habilitada`, item.SpaceID).Scan(&f.OwnerID, &f.RenterID)
	if err != nil {
		return booking.Fixture{}, mapErr(err)
	}
	f.SpaceID, f.Title, f.Category, f.RateUnit, f.Price, f.Currency, f.TimeZone = item.SpaceID, item.Title, item.CategoryCode, item.RateUnit, item.Price, item.Currency, item.TimeZone
	return f, nil
}

func (r *Repository) Catalog(ctx context.Context, actor string, filter booking.CatalogFilter) ([]booking.CatalogItem, error) {
	rows, err := r.pool.Query(ctx, `SELECT e.id::text,e.categoria_codigo,k.nombre,e.titulo,e.descripcion,t.modalidad,t.precio_base_clp,t.moneda,e.zona_horaria,c.perfil_version,p.perfil,c.valores,
CASE WHEN $3::timestamptz IS NULL THEN NULL ELSE NOT EXISTS(SELECT 1 FROM public.ocupacion o WHERE o.espacio_id=e.id AND o.activo AND o.intervalo && tstzrange($3,$4,'[)')) END
FROM public.reserva_ensayo_local_fixture f
JOIN public.espacio e ON e.id=f.espacio_id
JOIN public.categoria_espacio k ON k.codigo=e.categoria_codigo AND k.activa
JOIN public.espacio_caracteristicas c ON c.espacio_id=e.id AND c.categoria_codigo=e.categoria_codigo
JOIN public.categoria_perfil_atributos p ON p.categoria_codigo=c.categoria_codigo AND p.version=c.perfil_version
JOIN LATERAL(SELECT modalidad,precio_base_clp,moneda FROM public.tarifa_espacio WHERE espacio_id=e.id ORDER BY version DESC LIMIT 1)t ON true
WHERE f.habilitada AND e.estado='borrador' AND e.propietario_id=f.anfitrion_id
AND (f.anfitrion_id=$1 OR f.arrendatario_id=$1)
AND ($2::text='' OR e.categoria_codigo=$2)
AND ($3::timestamptz IS NULL OR NOT EXISTS(SELECT 1 FROM public.ocupacion o WHERE o.espacio_id=e.id AND o.activo AND o.intervalo && tstzrange($3,$4,'[)')))
ORDER BY k.orden,e.titulo,e.id`, actor, filter.CategoryCode, filter.StartAt, filter.EndAt)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []booking.CatalogItem{}
	for rows.Next() {
		var v booking.CatalogItem
		var available sql.NullBool
		if err = rows.Scan(&v.SpaceID, &v.CategoryCode, &v.CategoryName, &v.Title, &v.Description, &v.RateUnit, &v.Price, &v.Currency, &v.TimeZone, &v.ProfileVersion, &v.Profile, &v.Attributes, &available); err != nil {
			return nil, err
		}
		if available.Valid {
			value := available.Bool
			v.Available = &value
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (r *Repository) CatalogDetail(ctx context.Context, actor, spaceID string) (booking.CatalogItem, error) {
	var v booking.CatalogItem
	err := r.pool.QueryRow(ctx, `SELECT e.id::text,e.categoria_codigo,k.nombre,e.titulo,e.descripcion,t.modalidad,t.precio_base_clp,t.moneda,e.zona_horaria,c.perfil_version,p.perfil,c.valores
FROM public.reserva_ensayo_local_fixture f
JOIN public.espacio e ON e.id=f.espacio_id
JOIN public.categoria_espacio k ON k.codigo=e.categoria_codigo AND k.activa
JOIN public.espacio_caracteristicas c ON c.espacio_id=e.id AND c.categoria_codigo=e.categoria_codigo
JOIN public.categoria_perfil_atributos p ON p.categoria_codigo=c.categoria_codigo AND p.version=c.perfil_version
JOIN LATERAL(SELECT modalidad,precio_base_clp,moneda FROM public.tarifa_espacio WHERE espacio_id=e.id ORDER BY version DESC LIMIT 1)t ON true
WHERE f.espacio_id=$1 AND f.habilitada AND e.estado='borrador' AND e.propietario_id=f.anfitrion_id AND (f.anfitrion_id=$2 OR f.arrendatario_id=$2)`, spaceID, actor).Scan(&v.SpaceID, &v.CategoryCode, &v.CategoryName, &v.Title, &v.Description, &v.RateUnit, &v.Price, &v.Currency, &v.TimeZone, &v.ProfileVersion, &v.Profile, &v.Attributes)
	return v, mapErr(err)
}

func (r *Repository) Quote(ctx context.Context, renter, spaceID, id string, start, end time.Time, clock func() time.Time, ttl time.Duration) (booking.Quote, error) {
	preNow := clock().UTC()
	if !start.After(preNow) {
		return booking.Quote{}, booking.ErrInvalid
	}
	if err := r.Expire(ctx, preNow); err != nil {
		return booking.Quote{}, err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return booking.Quote{}, err
	}
	defer tx.Rollback(ctx)
	now := clock().UTC()
	if !start.After(now) {
		return booking.Quote{}, booking.ErrInvalid
	}
	expires := now.Add(ttl)
	var q booking.Quote
	var amount int64
	var hostID, renterID string
	err = tx.QueryRow(ctx, `SELECT x.espacio_id::text,x.anfitrion_id::text,x.arrendatario_id::text,t.version,t.modalidad,t.precio_base_clp,t.moneda,e.zona_horaria,e.reglas_uso,e.categoria_codigo,c.perfil_version,c.valores FROM public.reserva_ensayo_local_fixture x JOIN public.espacio e ON e.id=x.espacio_id JOIN public.espacio_caracteristicas c ON c.espacio_id=e.id AND c.categoria_codigo=e.categoria_codigo JOIN LATERAL(SELECT version,modalidad,precio_base_clp,moneda FROM public.tarifa_espacio WHERE espacio_id=e.id ORDER BY version DESC LIMIT 1)t ON true WHERE x.habilitada AND x.espacio_id=$2 AND x.arrendatario_id=$1 AND e.estado='borrador' AND e.propietario_id=x.anfitrion_id FOR SHARE OF e`, renter, spaceID).Scan(&q.SpaceID, &hostID, &renterID, &q.RateVersion, &q.RateUnit, &q.UnitPrice, &q.Currency, &q.TimeZone, &q.Conditions, &q.CategoryCode, &q.ProfileVersion, &q.ProfileValues)
	if err != nil {
		return booking.Quote{}, mapErr(err)
	}
	units, err := booking.PriceUnits(q.RateUnit, start, end, q.TimeZone)
	if err != nil {
		return booking.Quote{}, booking.ErrInvalid
	}
	if q.UnitPrice > int64(^uint64(0)>>1)/units {
		return booking.Quote{}, booking.ErrInvalid
	}
	amount = units * q.UnitPrice
	var available bool
	err = tx.QueryRow(ctx, `SELECT NOT EXISTS(SELECT 1 FROM public.ocupacion WHERE espacio_id=$1 AND activo AND intervalo && tstzrange($2,$3,'[)'))`, q.SpaceID, start, end).Scan(&available)
	if err != nil {
		return booking.Quote{}, err
	}
	if !available {
		return booking.Quote{}, booking.ErrConflict
	}
	q.ID = id
	q.Units = units
	q.Subtotal = amount
	q.StartAt = start
	q.EndAt = end
	q.CreatedAt = now
	q.ExpiresAt = expires
	_, err = tx.Exec(ctx, `INSERT INTO public.cotizacion_reserva_ensayo(id,espacio_id,anfitrion_id,arrendatario_id,tarifa_version,modalidad,precio_unitario_clp,moneda,unidades,subtotal_clp,inicio,termino,zona_horaria,condiciones_snapshot,creada_en,vence_en,categoria_codigo,perfil_version,perfil_valores_snapshot) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19)`, id, q.SpaceID, hostID, renterID, q.RateVersion, q.RateUnit, q.UnitPrice, q.Currency, units, amount, start, end, q.TimeZone, q.Conditions, now, expires, q.CategoryCode, q.ProfileVersion, q.ProfileValues)
	if err != nil {
		return booking.Quote{}, mapErr(err)
	}
	if err = tx.Commit(ctx); err != nil {
		return booking.Quote{}, err
	}
	return q, nil
}

const reservationCols = `id::text,cotizacion_id::text,espacio_id::text,anfitrion_id::text,arrendatario_id::text,estado,modalidad,precio_unitario_clp,moneda,unidades,subtotal_clp,inicio,termino,zona_horaria,condiciones_snapshot,pago_vence_en,anfitrion_vence_en,creada_en,actualizada_en`

func scanReservation(row pgx.Row) (booking.Reservation, error) {
	var v booking.Reservation
	err := row.Scan(&v.ID, &v.QuoteID, &v.SpaceID, &v.HostID, &v.RenterID, &v.State, &v.RateUnit, &v.UnitPrice, &v.Currency, &v.Units, &v.Subtotal, &v.StartAt, &v.EndAt, &v.TimeZone, &v.Conditions, &v.PayExpiresAt, &v.HostExpiresAt, &v.CreatedAt, &v.UpdatedAt)
	return v, mapErr(err)
}

func (r *Repository) Create(ctx context.Context, renter, quoteID, key string, fingerprint []byte, id, occupancyID string, payTTL time.Duration, clock func() time.Time) (booking.Reservation, error) {
	preNow := clock().UTC()
	if err := r.Expire(ctx, preNow); err != nil {
		return booking.Reservation{}, err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return booking.Reservation{}, err
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, renter+":"+key)
	if err != nil {
		return booking.Reservation{}, err
	}
	var priorID, priorHash string
	err = tx.QueryRow(ctx, `SELECT id::text,encode(huella_solicitud,'hex') FROM public.reserva_ensayo_local WHERE arrendatario_id=$1 AND clave_idempotencia=$2`, renter, key).Scan(&priorID, &priorHash)
	if err == nil {
		if priorHash != fmtHex(fingerprint) {
			return booking.Reservation{}, booking.ErrConflict
		}
		v, e := scanReservation(tx.QueryRow(ctx, `SELECT `+reservationCols+` FROM public.reserva_ensayo_local WHERE id=$1`, priorID))
		if e != nil {
			return booking.Reservation{}, e
		}
		if e = tx.Commit(ctx); e != nil {
			return booking.Reservation{}, e
		}
		return v, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return booking.Reservation{}, err
	}
	// Read the Backend clock only after the idempotency lookup while the new
	// request transaction and its serialization lock are active.
	now := clock().UTC()
	payExpiresAt := now.Add(payTTL)
	var quoteExists, quoteUsable, intervalFuture, available bool
	err = tx.QueryRow(ctx, `SELECT
EXISTS(SELECT 1 FROM public.cotizacion_reserva_ensayo q JOIN public.reserva_ensayo_local_fixture f ON f.espacio_id=q.espacio_id AND f.anfitrion_id=q.anfitrion_id AND f.arrendatario_id=q.arrendatario_id WHERE f.habilitada AND q.id=$1 AND q.arrendatario_id=$2),
EXISTS(SELECT 1 FROM public.cotizacion_reserva_ensayo q JOIN public.reserva_ensayo_local_fixture f ON f.espacio_id=q.espacio_id AND f.anfitrion_id=q.anfitrion_id AND f.arrendatario_id=q.arrendatario_id WHERE f.habilitada AND q.id=$1 AND q.arrendatario_id=$2 AND q.vence_en>$3),
EXISTS(SELECT 1 FROM public.cotizacion_reserva_ensayo q JOIN public.reserva_ensayo_local_fixture f ON f.espacio_id=q.espacio_id AND f.anfitrion_id=q.anfitrion_id AND f.arrendatario_id=q.arrendatario_id WHERE f.habilitada AND q.id=$1 AND q.arrendatario_id=$2 AND q.inicio>$3),
NOT EXISTS(SELECT 1 FROM public.ocupacion o JOIN public.cotizacion_reserva_ensayo q ON q.espacio_id=o.espacio_id WHERE q.id=$1 AND q.arrendatario_id=$2 AND o.activo AND o.intervalo && (SELECT tstzrange(inicio,termino,'[)') FROM public.cotizacion_reserva_ensayo WHERE id=$1))`, quoteID, renter, now).Scan(&quoteExists, &quoteUsable, &intervalFuture, &available)
	if err != nil {
		return booking.Reservation{}, err
	}
	if !quoteExists {
		return booking.Reservation{}, booking.ErrNotFound
	}
	if !quoteUsable {
		return booking.Reservation{}, booking.ErrNotFound
	}
	if !intervalFuture {
		return booking.Reservation{}, booking.ErrConflict
	}
	if !available {
		return booking.Reservation{}, booking.ErrConflict
	}
	var v booking.Reservation
	err = tx.QueryRow(ctx, `INSERT INTO public.reserva_ensayo_local(id,cotizacion_id,espacio_id,anfitrion_id,arrendatario_id,clave_idempotencia,huella_solicitud,ocupacion_id,estado,precio_unitario_clp,unidades,subtotal_clp,modalidad,moneda,inicio,termino,zona_horaria,condiciones_snapshot,pago_vence_en,creada_en,actualizada_en)
SELECT $1,q.id,q.espacio_id,q.anfitrion_id,q.arrendatario_id,$4,$5,$6,'pendiente_de_pago',q.precio_unitario_clp,q.unidades,q.subtotal_clp,q.modalidad,q.moneda,q.inicio,q.termino,q.zona_horaria,q.condiciones_snapshot,$7,$8,$8 FROM public.cotizacion_reserva_ensayo q JOIN public.reserva_ensayo_local_fixture f ON f.espacio_id=q.espacio_id AND f.anfitrion_id=q.anfitrion_id AND f.arrendatario_id=q.arrendatario_id AND f.habilitada JOIN public.espacio e ON e.id=q.espacio_id WHERE q.id=$2 AND q.arrendatario_id=$3 AND q.vence_en>$8 AND q.inicio>$8 AND e.estado='borrador' AND e.propietario_id=f.anfitrion_id RETURNING `+reservationCols, id, quoteID, renter, key, fingerprint, occupancyID, payExpiresAt, now).Scan(&v.ID, &v.QuoteID, &v.SpaceID, &v.HostID, &v.RenterID, &v.State, &v.RateUnit, &v.UnitPrice, &v.Currency, &v.Units, &v.Subtotal, &v.StartAt, &v.EndAt, &v.TimeZone, &v.Conditions, &v.PayExpiresAt, &v.HostExpiresAt, &v.CreatedAt, &v.UpdatedAt)
	if err != nil {
		return booking.Reservation{}, mapErr(err)
	}
	_, err = tx.Exec(ctx, `INSERT INTO public.ocupacion(id,espacio_id,reserva_id,intervalo,tipo,activo,expira_en) VALUES($1,$2,$3,tstzrange($4,$5,'[)'),'retencion',true,$6)`, occupancyID, v.SpaceID, v.ID, v.StartAt, v.EndAt, v.PayExpiresAt)
	if err != nil {
		return booking.Reservation{}, mapErr(err)
	}
	err = appendTransition(ctx, tx, v.ID, nil, "pendiente_de_pago", renter, "solicitud local con retención atómica", now)
	if err != nil {
		return booking.Reservation{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return booking.Reservation{}, mapErr(err)
	}
	return v, nil
}

func (r *Repository) Get(ctx context.Context, actor, id string) (booking.Detail, error) {
	v, err := scanReservation(r.pool.QueryRow(ctx, `SELECT `+reservationCols+` FROM public.reserva_ensayo_local WHERE id=$1 AND (arrendatario_id=$2 OR anfitrion_id=$2)`, id, actor))
	if err != nil {
		return booking.Detail{}, err
	}
	rows, err := r.pool.Query(ctx, `SELECT secuencia,estado_anterior,estado_nuevo,actor_id::text,motivo,creada_en FROM public.reserva_ensayo_transicion WHERE reserva_id=$1 ORDER BY secuencia`, id)
	if err != nil {
		return booking.Detail{}, err
	}
	defer rows.Close()
	out := booking.Detail{Reservation: v, History: []booking.Transition{}}
	for rows.Next() {
		var x booking.Transition
		var actor sql.NullString
		if err = rows.Scan(&x.Sequence, &x.From, &x.To, &actor, &x.Reason, &x.At); err != nil {
			return booking.Detail{}, err
		}
		if actor.Valid {
			x.Actor = &actor.String
		}
		out.History = append(out.History, x)
	}
	return out, rows.Err()
}
func (r *Repository) List(ctx context.Context, actor string) ([]booking.Reservation, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+reservationCols+` FROM public.reserva_ensayo_local WHERE arrendatario_id=$1 OR anfitrion_id=$1 ORDER BY actualizada_en DESC,id`, actor)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []booking.Reservation{}
	for rows.Next() {
		v, e := scanReservation(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (r *Repository) Pay(ctx context.Context, renter, id, outcome, key string, now, hostDeadline time.Time) (booking.Reservation, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return booking.Reservation{}, err
	}
	defer tx.Rollback(ctx)
	v, err := scanReservation(tx.QueryRow(ctx, `SELECT `+reservationCols+` FROM public.reserva_ensayo_local WHERE id=$1 AND arrendatario_id=$2 FOR UPDATE`, id, renter))
	if err != nil {
		return booking.Reservation{}, err
	}
	result := map[string]string{"exito": "exito_simulado", "rechazo": "rechazo_simulado", "sin_respuesta": "sin_respuesta_simulada"}[outcome]
	var old string
	err = tx.QueryRow(ctx, `SELECT resultado FROM public.reserva_pago_ensayo WHERE reserva_id=$1 AND clave_idempotencia=$2`, id, key).Scan(&old)
	if err == nil {
		if old != result {
			return booking.Reservation{}, booking.ErrConflict
		}
		if err = tx.Commit(ctx); err != nil {
			return booking.Reservation{}, err
		}
		return v, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return booking.Reservation{}, err
	}
	var unresolved bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.reserva_pago_ensayo WHERE reserva_id=$1 AND resultado='sin_respuesta_simulada')`, id).Scan(&unresolved)
	if err != nil {
		return booking.Reservation{}, err
	}
	if unresolved {
		return booking.Reservation{}, booking.ErrConflict
	}
	if v.State != "pendiente_de_pago" || !now.Before(v.PayExpiresAt) {
		return booking.Reservation{}, booking.ErrConflict
	}
	_, err = tx.Exec(ctx, `INSERT INTO public.reserva_pago_ensayo(id,reserva_id,resultado,clave_idempotencia,creada_en) VALUES(gen_random_uuid(),$1,$2,$3,$4)`, id, result, key, now)
	if err != nil {
		return booking.Reservation{}, err
	}
	if outcome == "sin_respuesta" {
		if err = tx.Commit(ctx); err != nil {
			return booking.Reservation{}, err
		}
		return v, nil
	}
	next := "pagada"
	reason := "pago local simulado exitoso"
	if outcome == "rechazo" {
		next = "cancelada_por_pago"
		reason = "rechazo local simulado"
	}
	if outcome == "exito" {
		v.HostExpiresAt = ptr(hostDeadline)
		_, err = tx.Exec(ctx, `UPDATE public.ocupacion SET tipo='reserva',expira_en=$2 WHERE reserva_id=$1 AND activo`, id, *v.HostExpiresAt)
	} else {
		_, err = tx.Exec(ctx, `UPDATE public.ocupacion SET activo=false,desactivada_en=$2 WHERE reserva_id=$1 AND activo`, id, now)
	}
	if err != nil {
		return booking.Reservation{}, err
	}
	_, err = tx.Exec(ctx, `UPDATE public.reserva_ensayo_local SET estado=$2,anfitrion_vence_en=$3,actualizada_en=$4 WHERE id=$1`, id, next, v.HostExpiresAt, now)
	if err != nil {
		return booking.Reservation{}, err
	}
	err = appendTransition(ctx, tx, id, ptrString("pendiente_de_pago"), next, renter, reason, now)
	if err != nil {
		return booking.Reservation{}, err
	}
	v.State = next
	v.UpdatedAt = now
	if err = tx.Commit(ctx); err != nil {
		return booking.Reservation{}, mapErr(err)
	}
	return v, nil
}

func (r *Repository) Decide(ctx context.Context, host, id, decision string, now time.Time) (booking.Reservation, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return booking.Reservation{}, err
	}
	defer tx.Rollback(ctx)
	v, err := scanReservation(tx.QueryRow(ctx, `SELECT `+reservationCols+` FROM public.reserva_ensayo_local WHERE id=$1 AND anfitrion_id=$2 FOR UPDATE`, id, host))
	if err != nil {
		return booking.Reservation{}, err
	}
	if v.State != "pagada" || v.HostExpiresAt == nil || !now.Before(*v.HostExpiresAt) {
		return booking.Reservation{}, booking.ErrConflict
	}
	next, reason := "aprobada_host", "aprobación del anfitrión en ensayo local"
	if decision == "rechazar" {
		next, reason = "rechazada_arrendador", "rechazo del anfitrión en ensayo local"
		_, err = tx.Exec(ctx, `UPDATE public.ocupacion SET activo=false,desactivada_en=$2 WHERE reserva_id=$1 AND activo`, id, now)
		if err != nil {
			return booking.Reservation{}, err
		}
		_, err = tx.Exec(ctx, `INSERT INTO public.reserva_pago_ensayo(id,reserva_id,resultado,clave_idempotencia,creada_en) VALUES(gen_random_uuid(),$1,'devolucion_simulada',$2,$3)`, id, "devolucion:"+id, now)
		if err != nil {
			return booking.Reservation{}, err
		}
	} else {
		_, err = tx.Exec(ctx, `UPDATE public.ocupacion SET expira_en=NULL WHERE reserva_id=$1 AND activo`, id)
		if err != nil {
			return booking.Reservation{}, err
		}
	}
	_, err = tx.Exec(ctx, `UPDATE public.reserva_ensayo_local SET estado=$2,actualizada_en=$3 WHERE id=$1`, id, next, now)
	if err != nil {
		return booking.Reservation{}, err
	}
	err = appendTransition(ctx, tx, id, ptrString("pagada"), next, host, reason, now)
	if err != nil {
		return booking.Reservation{}, err
	}
	v.State = next
	v.UpdatedAt = now
	if err = tx.Commit(ctx); err != nil {
		return booking.Reservation{}, mapErr(err)
	}
	return v, nil
}

func (r *Repository) Cancel(ctx context.Context, renter, id string, now time.Time) (booking.Reservation, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return booking.Reservation{}, err
	}
	defer tx.Rollback(ctx)
	v, err := scanReservation(tx.QueryRow(ctx, `SELECT `+reservationCols+` FROM public.reserva_ensayo_local WHERE id=$1 AND arrendatario_id=$2 FOR UPDATE`, id, renter))
	if err != nil {
		return booking.Reservation{}, err
	}
	if v.State != "pendiente_de_pago" {
		return booking.Reservation{}, booking.ErrConflict
	}
	_, err = tx.Exec(ctx, `UPDATE public.ocupacion SET activo=false,desactivada_en=$2 WHERE reserva_id=$1 AND activo`, id, now)
	if err != nil {
		return booking.Reservation{}, err
	}
	_, err = tx.Exec(ctx, `UPDATE public.reserva_ensayo_local SET estado='cancelada_arrendatario',actualizada_en=$2 WHERE id=$1`, id, now)
	if err != nil {
		return booking.Reservation{}, err
	}
	err = appendTransition(ctx, tx, id, ptrString("pendiente_de_pago"), "cancelada_arrendatario", renter, "cancelación local antes del pago", now)
	if err != nil {
		return booking.Reservation{}, err
	}
	v.State = "cancelada_arrendatario"
	v.UpdatedAt = now
	if err = tx.Commit(ctx); err != nil {
		return booking.Reservation{}, mapErr(err)
	}
	return v, nil
}

func (r *Repository) Expire(ctx context.Context, now time.Time) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `SELECT id::text,estado FROM public.reserva_ensayo_local WHERE (estado='pendiente_de_pago' AND pago_vence_en<=$1) OR (estado='pagada' AND anfitrion_vence_en<=$1) FOR UPDATE SKIP LOCKED`, now)
	if err != nil {
		return err
	}
	type expired struct{ id, old string }
	all := []expired{}
	for rows.Next() {
		var x expired
		if err = rows.Scan(&x.id, &x.old); err != nil {
			rows.Close()
			return err
		}
		all = append(all, x)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, x := range all {
		next, reason := "vencida_pago", "venció plazo de pago local"
		if x.old == "pagada" {
			next, reason = "vencida_host", "venció plazo de respuesta del anfitrión"
		}
		_, err = tx.Exec(ctx, `UPDATE public.ocupacion SET activo=false,desactivada_en=$2 WHERE reserva_id=$1 AND activo`, x.id, now)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE public.reserva_ensayo_local SET estado=$2,actualizada_en=$3 WHERE id=$1`, x.id, next, now)
		if err != nil {
			return err
		}
		err = appendTransition(ctx, tx, x.id, ptrString(x.old), next, nil, reason, now)
		if err != nil {
			return err
		}
		if x.old == "pagada" {
			_, err = tx.Exec(ctx, `INSERT INTO public.reserva_pago_ensayo(id,reserva_id,resultado,clave_idempotencia,creada_en) VALUES(gen_random_uuid(),$1,'devolucion_simulada',$2,$3)`, x.id, "devolucion-expiracion:"+x.id, now)
			if err != nil {
				return err
			}
		}
	}
	return tx.Commit(ctx)
}

// appendTransition allocates the next per-reservation sequence while callers
// hold the reservation row lock (or have just created the row) in this tx.
func appendTransition(ctx context.Context, tx pgx.Tx, reservationID string, from *string, to string, actor any, reason string, at time.Time) error {
	_, err := tx.Exec(ctx, `INSERT INTO public.reserva_ensayo_transicion(id,reserva_id,secuencia,estado_anterior,estado_nuevo,actor_id,motivo,creada_en)
SELECT gen_random_uuid(),$1,COALESCE(MAX(secuencia),0)+1,$2,$3,$4,$5,$6
FROM public.reserva_ensayo_transicion WHERE reserva_id=$1`, reservationID, from, to, actor, reason, at)
	return err
}

func fmtHex(b []byte) string {
	const h = "0123456789abcdef"
	out := make([]byte, len(b)*2)
	for i, v := range b {
		out[i*2] = h[v>>4]
		out[i*2+1] = h[v&15]
	}
	return string(out)
}
func ptr(t time.Time) *time.Time { return &t }
func ptrString(s string) *string { return &s }
func mapErr(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return booking.ErrNotFound
	}
	var p *pgconn.PgError
	if errors.As(err, &p) {
		switch p.Code {
		case "23P01", "23505":
			return booking.ErrConflict
		case "23514", "23503", "22003", "22007", "22008":
			return booking.ErrInvalid
		}
	}
	return err
}
