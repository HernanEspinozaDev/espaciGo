package bookingpg

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/booking/expiry"
	contractspg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/contracts"
	"github.com/HernanEspinozaDev/espaciGo/internal/booking"
	"github.com/HernanEspinozaDev/espaciGo/internal/spaces"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func (r *Repository) Fixture(ctx context.Context, actor string) (booking.Fixture, error) {
	var f booking.Fixture
	err := r.pool.QueryRow(ctx, `SELECT e.id::text,f.anfitrion_id::text,f.arrendatario_id::text,e.titulo,e.categoria_codigo,t.modalidad,t.precio_base_clp,t.moneda,e.zona_horaria
FROM public.reserva_ensayo_local_fixture f JOIN public.espacio e ON e.id=f.espacio_id AND e.propietario_id=f.anfitrion_id
JOIN LATERAL(SELECT modalidad,precio_base_clp,moneda FROM public.tarifa_espacio WHERE espacio_id=e.id ORDER BY version DESC LIMIT 1)t ON true
WHERE f.habilitada AND e.estado='borrador' AND (f.anfitrion_id=$1 OR f.arrendatario_id=$1)
ORDER BY e.id LIMIT 1`, actor).Scan(&f.SpaceID, &f.OwnerID, &f.RenterID, &f.Title, &f.Category, &f.RateUnit, &f.Price, &f.Currency, &f.TimeZone)
	if err != nil {
		return booking.Fixture{}, mapErr(err)
	}
	return f, nil
}

func (r *Repository) Catalog(ctx context.Context, actor string, filter booking.CatalogFilter) ([]booking.CatalogItem, error) {
	attributeJSON, err := json.Marshal(filter.Attributes)
	if err != nil {
		return nil, booking.ErrInvalid
	}
	if filter.Attributes == nil {
		attributeJSON = []byte(`{}`)
	}
	var profileVersion any
	if filter.ProfileVersion > 0 {
		profileVersion = filter.ProfileVersion
	}
	var latitude, longitude, radius any
	if filter.Latitude != nil {
		latitude = *filter.Latitude
	}
	if filter.Longitude != nil {
		longitude = *filter.Longitude
	}
	if filter.RadiusKM != nil {
		radius = *filter.RadiusKM
	}
	rows, err := r.pool.Query(ctx, `SELECT e.id::text,e.categoria_codigo,k.nombre,e.titulo,e.descripcion,t.modalidad,t.precio_base_clp,t.moneda,e.zona_horaria,c.perfil_version,p.perfil,c.valores,
CASE WHEN $3::timestamptz IS NULL THEN NULL ELSE NOT EXISTS(SELECT 1 FROM public.ocupacion o WHERE o.espacio_id=e.id AND o.activo AND o.intervalo && tstzrange($3,$4,'[)')) END,
CASE WHEN $7::double precision IS NULL THEN NULL ELSE ST_Distance(g.punto,ST_SetSRID(ST_MakePoint($8::double precision,$7::double precision),4326)::geography) END,k.orden
FROM public.espacio e
LEFT JOIN public.reserva_ensayo_local_fixture f ON f.espacio_id=e.id AND f.anfitrion_id=e.propietario_id
JOIN public.categoria_espacio k ON k.codigo=e.categoria_codigo AND k.activa
JOIN public.espacio_caracteristicas c ON c.espacio_id=e.id AND c.categoria_codigo=e.categoria_codigo
JOIN public.categoria_perfil_atributos p ON p.categoria_codigo=c.categoria_codigo AND p.version=c.perfil_version
JOIN LATERAL(SELECT modalidad,precio_base_clp,moneda FROM public.tarifa_espacio WHERE espacio_id=e.id ORDER BY version DESC LIMIT 1)t ON true
LEFT JOIN public.reserva_ensayo_local_ubicacion_sintetica g ON g.espacio_id=e.id AND g.es_sintetica
WHERE (((f.habilitada AND e.estado='borrador') AND (f.anfitrion_id=$1 OR f.arrendatario_id=$1))
   OR (NOT COALESCE(f.habilitada,false) AND e.estado='activa' AND e.propietario_id<>$1 AND EXISTS(
       SELECT 1 FROM public.elegibilidad_verificacion_local eligibility
       JOIN public.verificacion verification ON verification.id=eligibility.verificacion_id AND verification.usuario_id=eligibility.usuario_id AND verification.tipo=eligibility.tipo AND verification.estado='aprobada'
       JOIN public.usuario owner ON owner.id=eligibility.usuario_id AND owner.estado='activo'
       WHERE eligibility.usuario_id=e.propietario_id AND eligibility.tipo='kyc' AND eligibility.estado='elegible')))
AND ($2::text='' OR e.categoria_codigo=$2)
AND ($3::timestamptz IS NULL OR NOT EXISTS(SELECT 1 FROM public.ocupacion o WHERE o.espacio_id=e.id AND o.activo AND o.intervalo && tstzrange($3,$4,'[)')))
AND ($5::integer IS NULL OR (c.perfil_version=$5 AND c.valores @> $6::jsonb))
AND e.zona_horaria IS NOT NULL
AND ($7::double precision IS NULL OR (g.punto IS NOT NULL
 AND ST_DWithin(g.punto,ST_SetSRID(ST_MakePoint($8::double precision,$7::double precision),4326)::geography,$9::double precision*1000.0+0.000001)
 AND ST_Distance(g.punto,ST_SetSRID(ST_MakePoint($8::double precision,$7::double precision),4326)::geography)<=$9::double precision*1000.0+0.000001))
ORDER BY k.orden,e.titulo,e.id`, actor, filter.CategoryCode, filter.StartAt, filter.EndAt, profileVersion, attributeJSON, latitude, longitude, radius)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []booking.CatalogItem{}
	for rows.Next() {
		var v booking.CatalogItem
		var available sql.NullBool
		var distanceMeters sql.NullFloat64
		if err = rows.Scan(&v.SpaceID, &v.CategoryCode, &v.CategoryName, &v.Title, &v.Description, &v.RateUnit, &v.Price, &v.Currency, &v.TimeZone, &v.ProfileVersion, &v.Profile, &v.Attributes, &available, &distanceMeters, &v.CategoryOrder); err != nil {
			return nil, err
		}
		if available.Valid {
			value := available.Bool
			v.Available = &value
		}
		if distanceMeters.Valid {
			v.DistanceMeters = distanceMeters.Float64
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (r *Repository) CatalogProfile(ctx context.Context, category string, version int) (spaces.Profile, error) {
	var raw []byte
	if err := r.pool.QueryRow(ctx, `SELECT perfil FROM public.categoria_perfil_atributos WHERE categoria_codigo=$1 AND version=$2`, category, version).Scan(&raw); err != nil {
		return spaces.Profile{}, mapErr(err)
	}
	var profile spaces.Profile
	if err := json.Unmarshal(raw, &profile); err != nil {
		return spaces.Profile{}, err
	}
	return profile, nil
}

func (r *Repository) CatalogDetail(ctx context.Context, actor, spaceID string) (booking.CatalogItem, error) {
	var v booking.CatalogItem
	err := r.pool.QueryRow(ctx, `SELECT e.id::text,e.categoria_codigo,k.nombre,e.titulo,e.descripcion,t.modalidad,t.precio_base_clp,t.moneda,e.zona_horaria,c.perfil_version,p.perfil,c.valores
FROM public.espacio e
LEFT JOIN public.reserva_ensayo_local_fixture f ON f.espacio_id=e.id AND f.anfitrion_id=e.propietario_id
JOIN public.categoria_espacio k ON k.codigo=e.categoria_codigo AND k.activa
JOIN public.espacio_caracteristicas c ON c.espacio_id=e.id AND c.categoria_codigo=e.categoria_codigo
JOIN public.categoria_perfil_atributos p ON p.categoria_codigo=c.categoria_codigo AND p.version=c.perfil_version
JOIN LATERAL(SELECT modalidad,precio_base_clp,moneda FROM public.tarifa_espacio WHERE espacio_id=e.id ORDER BY version DESC LIMIT 1)t ON true
WHERE e.id=$1 AND e.zona_horaria IS NOT NULL AND (((f.habilitada AND e.estado='borrador') AND (f.anfitrion_id=$2 OR f.arrendatario_id=$2))
 OR (NOT COALESCE(f.habilitada,false) AND e.estado='activa' AND e.propietario_id<>$2 AND EXISTS(
       SELECT 1 FROM public.elegibilidad_verificacion_local eligibility
       JOIN public.verificacion verification ON verification.id=eligibility.verificacion_id AND verification.usuario_id=eligibility.usuario_id AND verification.tipo=eligibility.tipo AND verification.estado='aprobada'
       JOIN public.usuario owner ON owner.id=eligibility.usuario_id AND owner.estado='activo'
       WHERE eligibility.usuario_id=e.propietario_id AND eligibility.tipo='kyc' AND eligibility.estado='elegible')))`, spaceID, actor).Scan(&v.SpaceID, &v.CategoryCode, &v.CategoryName, &v.Title, &v.Description, &v.RateUnit, &v.Price, &v.Currency, &v.TimeZone, &v.ProfileVersion, &v.Profile, &v.Attributes)
	return v, mapErr(err)
}

func (r *Repository) AvailableIntervals(ctx context.Context, actor, spaceID string, candidates []booking.AvailableInterval) ([]bool, error) {
	// Recheck allowlist and both participant identities at the time of each
	// availability request. No occupancy metadata is selected or returned.
	var authorized bool
	if err := r.pool.QueryRow(ctx, `SELECT EXISTS(
		SELECT 1 FROM public.espacio e LEFT JOIN public.reserva_ensayo_local_fixture f ON f.espacio_id=e.id AND f.anfitrion_id=e.propietario_id
		WHERE e.id=$1 AND e.zona_horaria IS NOT NULL AND (((f.habilitada AND e.estado='borrador') AND (f.anfitrion_id=$2 OR f.arrendatario_id=$2))
		OR (NOT COALESCE(f.habilitada,false) AND e.estado='activa' AND e.propietario_id<>$2 AND EXISTS(
			SELECT 1 FROM public.elegibilidad_verificacion_local eligibility
			JOIN public.verificacion verification ON verification.id=eligibility.verificacion_id AND verification.usuario_id=eligibility.usuario_id AND verification.tipo=eligibility.tipo AND verification.estado='aprobada'
			JOIN public.usuario owner ON owner.id=eligibility.usuario_id AND owner.estado='activo'
			WHERE eligibility.usuario_id=e.propietario_id AND eligibility.tipo='kyc' AND eligibility.estado='elegible'))))`, spaceID, actor).Scan(&authorized); err != nil {
		return nil, err
	}
	if !authorized {
		return nil, booking.ErrNotFound
	}
	starts, ends := make([]time.Time, len(candidates)), make([]time.Time, len(candidates))
	for i, candidate := range candidates {
		starts[i], ends[i] = candidate.StartAt.UTC(), candidate.EndAt.UTC()
	}
	available := make([]bool, len(candidates))
	if len(candidates) == 0 {
		return available, nil
	}
	rows, err := r.pool.Query(ctx, `SELECT c.ordinality,
		NOT EXISTS(SELECT 1 FROM public.ocupacion o
			WHERE o.espacio_id=$1 AND o.activo AND o.intervalo && tstzrange(c.starts,c.ends,'[)'))
		FROM unnest($2::timestamptz[],$3::timestamptz[]) WITH ORDINALITY AS c(starts,ends,ordinality)
		ORDER BY c.ordinality`, spaceID, starts, ends)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var ordinal int64
		var free bool
		if err = rows.Scan(&ordinal, &free); err != nil {
			return nil, err
		}
		if ordinal < 1 || ordinal > int64(len(available)) {
			return nil, errors.New("availability candidate ordinal out of range")
		}
		available[ordinal-1] = free
		count++
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if count != len(candidates) {
		return nil, errors.New("availability candidate result count mismatch")
	}
	return available, nil
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
	var quoteHost string
	if err := tx.QueryRow(ctx, `SELECT propietario_id::text FROM public.espacio WHERE id=$1`, spaceID).Scan(&quoteHost); err != nil {
		return booking.Quote{}, mapErr(err)
	}
	if err := lockActiveAccounts(ctx, tx, renter, quoteHost); err != nil {
		return booking.Quote{}, err
	}
	now := clock().UTC()
	if !start.After(now) {
		return booking.Quote{}, booking.ErrInvalid
	}
	expires := now.Add(ttl)
	var q booking.Quote
	var amount int64
	var hostID, renterID string
	err = tx.QueryRow(ctx, `SELECT e.id::text,e.propietario_id::text,$1::uuid::text,t.version,t.modalidad,t.precio_base_clp,t.moneda,e.zona_horaria,e.reglas_uso,e.categoria_codigo,c.perfil_version,c.valores,COALESCE(f.politica_cancelacion_version,$3)
FROM public.espacio e
LEFT JOIN public.reserva_ensayo_local_fixture f ON f.espacio_id=e.id AND f.anfitrion_id=e.propietario_id
JOIN public.espacio_caracteristicas c ON c.espacio_id=e.id AND c.categoria_codigo=e.categoria_codigo
JOIN LATERAL(SELECT version,modalidad,precio_base_clp,moneda FROM public.tarifa_espacio WHERE espacio_id=e.id ORDER BY version DESC LIMIT 1)t ON true
WHERE e.id=$2 AND e.zona_horaria IS NOT NULL AND (((f.habilitada AND e.estado='borrador') AND f.arrendatario_id=$1)
 OR (NOT COALESCE(f.habilitada,false) AND e.estado='activa' AND e.propietario_id<>$1 AND EXISTS(
       SELECT 1 FROM public.elegibilidad_verificacion_local eligibility
       JOIN public.verificacion verification ON verification.id=eligibility.verificacion_id AND verification.usuario_id=eligibility.usuario_id AND verification.tipo=eligibility.tipo AND verification.estado='aprobada'
       JOIN public.usuario owner ON owner.id=eligibility.usuario_id AND owner.estado='activo'
       WHERE eligibility.usuario_id=e.propietario_id AND eligibility.tipo='kyc' AND eligibility.estado='elegible')))
FOR SHARE OF e`, renter, spaceID, booking.LocalCancellationPolicyVersion).Scan(&q.SpaceID, &hostID, &renterID, &q.RateVersion, &q.RateUnit, &q.UnitPrice, &q.Currency, &q.TimeZone, &q.Conditions, &q.CategoryCode, &q.ProfileVersion, &q.ProfileValues, &q.CancellationPolicyVersion)
	if err != nil {
		return booking.Quote{}, mapErr(err)
	}
	if q.CancellationPolicyVersion != booking.LocalCancellationPolicyVersion {
		return booking.Quote{}, booking.ErrConflict
	}
	if q.RateUnit == "hora" {
		hours, scheduleErr := r.weeklyHoursTx(ctx, tx, q.SpaceID)
		if scheduleErr != nil {
			return booking.Quote{}, scheduleErr
		}
		if hours.Enabled && !booking.IntervalFitsWeeklyHours(start, end, q.TimeZone, hours) {
			return booking.Quote{}, booking.ErrConflict
		}
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
	retireAt := expires.AddDate(0, 0, 90)
	_, err = tx.Exec(ctx, `INSERT INTO public.cotizacion_reserva_ensayo(id,espacio_id,anfitrion_id,arrendatario_id,tarifa_version,modalidad,precio_unitario_clp,moneda,unidades,subtotal_clp,inicio,termino,zona_horaria,condiciones_snapshot,creada_en,vence_en,categoria_codigo,perfil_version,perfil_valores_snapshot,politica_cancelacion_version,retirar_en) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21)`, id, q.SpaceID, hostID, renterID, q.RateVersion, q.RateUnit, q.UnitPrice, q.Currency, units, amount, start, end, q.TimeZone, q.Conditions, now, expires, q.CategoryCode, q.ProfileVersion, q.ProfileValues, q.CancellationPolicyVersion, retireAt)
	if err != nil {
		return booking.Quote{}, mapErr(err)
	}
	if err = tx.Commit(ctx); err != nil {
		return booking.Quote{}, err
	}
	return q, nil
}

const reservationCols = `id::text,cotizacion_id::text,espacio_id::text,anfitrion_id::text,arrendatario_id::text,estado,modalidad,precio_unitario_clp,moneda,unidades,subtotal_clp,inicio,termino,zona_horaria,condiciones_snapshot,politica_cancelacion_version,pago_vence_en,anfitrion_vence_en,creada_en,actualizada_en`

func scanReservation(row pgx.Row) (booking.Reservation, error) {
	var v booking.Reservation
	err := scanReservationColumns(row, &v, false)
	return v, mapErr(err)
}

func scanReservationWithUnread(row pgx.Row) (booking.Reservation, error) {
	var v booking.Reservation
	err := scanReservationColumns(row, &v, true)
	return v, mapErr(err)
}

func scanReservationColumns(row pgx.Row, v *booking.Reservation, withUnread bool) error {
	columns := []any{&v.ID, &v.QuoteID, &v.SpaceID, &v.HostID, &v.RenterID, &v.State, &v.RateUnit, &v.UnitPrice, &v.Currency, &v.Units, &v.Subtotal, &v.StartAt, &v.EndAt, &v.TimeZone, &v.Conditions, &v.CancellationPolicyVersion, &v.PayExpiresAt, &v.HostExpiresAt, &v.CreatedAt, &v.UpdatedAt}
	if withUnread {
		columns = append(columns, &v.UnreadCount)
	}
	return row.Scan(columns...)
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
	var quoteHost string
	if err = tx.QueryRow(ctx, `SELECT anfitrion_id::text FROM public.cotizacion_reserva_ensayo WHERE id=$1 AND arrendatario_id=$2`, quoteID, renter).Scan(&quoteHost); err != nil {
		return booking.Reservation{}, mapErr(err)
	}
	if err = lockActiveAccounts(ctx, tx, renter, quoteHost); err != nil {
		return booking.Reservation{}, err
	}
	if err = requireLocalKYCEligibility(ctx, tx, renter, quoteHost); err != nil {
		return booking.Reservation{}, err
	}
	// Match the row lock used by UpdateOwn before validating the quoted tariff.
	// Keeping this lock through commit makes the tariff check and occupancy
	// creation serializable with a concurrent tariff update.
	var lockedSpace string
	err = tx.QueryRow(ctx, `SELECT e.id::text
FROM public.cotizacion_reserva_ensayo q
JOIN public.espacio e ON e.id=q.espacio_id AND e.propietario_id=q.anfitrion_id
LEFT JOIN public.reserva_ensayo_local_fixture f ON f.espacio_id=e.id AND f.anfitrion_id=e.propietario_id AND f.arrendatario_id=q.arrendatario_id
WHERE q.id=$1 AND q.arrendatario_id=$2 AND (((f.habilitada AND e.estado='borrador'))
 OR (NOT COALESCE(f.habilitada,false) AND e.estado='activa' AND e.propietario_id<>$2 AND EXISTS(
       SELECT 1 FROM public.elegibilidad_verificacion_local eligibility
       JOIN public.verificacion verification ON verification.id=eligibility.verificacion_id AND verification.usuario_id=eligibility.usuario_id AND verification.tipo=eligibility.tipo AND verification.estado='aprobada'
       JOIN public.usuario owner ON owner.id=eligibility.usuario_id AND owner.estado='activo'
       WHERE eligibility.usuario_id=e.propietario_id AND eligibility.tipo='kyc' AND eligibility.estado='elegible')))
FOR SHARE OF e`, quoteID, renter).Scan(&lockedSpace)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return booking.Reservation{}, err
	}
	// Read the Backend clock only after idempotency and space locks have been
	// acquired, so quote/start deadlines are revalidated after any lock wait.
	now := clock().UTC()
	payExpiresAt := now.Add(payTTL)
	if lockedSpace != "" {
		var rateUnit string
		var intervalStart, intervalEnd time.Time
		if err = tx.QueryRow(ctx, `SELECT modalidad,inicio,termino FROM public.cotizacion_reserva_ensayo WHERE id=$1 AND arrendatario_id=$2`, quoteID, renter).Scan(&rateUnit, &intervalStart, &intervalEnd); err != nil {
			return booking.Reservation{}, mapErr(err)
		}
		if rateUnit == "hora" {
			hours, scheduleErr := r.weeklyHoursTx(ctx, tx, lockedSpace)
			if scheduleErr != nil {
				return booking.Reservation{}, scheduleErr
			}
			if hours.Enabled && !booking.IntervalFitsWeeklyHours(intervalStart, intervalEnd, hours.TimeZone, hours) {
				return booking.Reservation{}, booking.ErrConflict
			}
		}
	}
	var quoteExists, quoteUsable, intervalFuture, available, rateCurrent, listingCurrent bool
	err = tx.QueryRow(ctx, `SELECT
EXISTS(SELECT 1 FROM public.cotizacion_reserva_ensayo q JOIN public.espacio e ON e.id=q.espacio_id AND e.propietario_id=q.anfitrion_id WHERE q.id=$1 AND q.arrendatario_id=$2),
EXISTS(SELECT 1 FROM public.cotizacion_reserva_ensayo q WHERE q.id=$1 AND q.arrendatario_id=$2 AND q.vence_en>$3),
EXISTS(SELECT 1 FROM public.cotizacion_reserva_ensayo q WHERE q.id=$1 AND q.arrendatario_id=$2 AND q.inicio>$3),
NOT EXISTS(SELECT 1 FROM public.ocupacion o JOIN public.cotizacion_reserva_ensayo q ON q.espacio_id=o.espacio_id WHERE q.id=$1 AND q.arrendatario_id=$2 AND o.activo AND o.intervalo && (SELECT tstzrange(inicio,termino,'[)') FROM public.cotizacion_reserva_ensayo WHERE id=$1)),
EXISTS(SELECT 1 FROM public.cotizacion_reserva_ensayo q JOIN LATERAL(SELECT version,modalidad,precio_base_clp,moneda FROM public.tarifa_espacio WHERE espacio_id=q.espacio_id ORDER BY version DESC LIMIT 1)t ON true WHERE q.id=$1 AND q.arrendatario_id=$2 AND t.version=q.tarifa_version AND t.modalidad=q.modalidad AND t.precio_base_clp=q.precio_unitario_clp AND t.moneda=q.moneda),
EXISTS(SELECT 1 FROM public.cotizacion_reserva_ensayo q JOIN public.espacio e ON e.id=q.espacio_id AND e.propietario_id=q.anfitrion_id LEFT JOIN public.reserva_ensayo_local_fixture f ON f.espacio_id=e.id AND f.anfitrion_id=e.propietario_id AND f.arrendatario_id=q.arrendatario_id WHERE q.id=$1 AND q.arrendatario_id=$2 AND (((f.habilitada AND e.estado='borrador')) OR (NOT COALESCE(f.habilitada,false) AND e.estado='activa' AND e.propietario_id<>$2 AND EXISTS(SELECT 1 FROM public.elegibilidad_verificacion_local eligibility JOIN public.verificacion verification ON verification.id=eligibility.verificacion_id AND verification.usuario_id=eligibility.usuario_id AND verification.tipo=eligibility.tipo AND verification.estado='aprobada' JOIN public.usuario owner ON owner.id=eligibility.usuario_id AND owner.estado='activo' WHERE eligibility.usuario_id=e.propietario_id AND eligibility.tipo='kyc' AND eligibility.estado='elegible'))))`, quoteID, renter, now).Scan(&quoteExists, &quoteUsable, &intervalFuture, &available, &rateCurrent, &listingCurrent)
	if err != nil {
		return booking.Reservation{}, err
	}
	if !quoteExists {
		return booking.Reservation{}, booking.ErrNotFound
	}
	if !quoteUsable {
		return booking.Reservation{}, booking.ErrNotFound
	}
	if !listingCurrent {
		return booking.Reservation{}, booking.ErrConflict
	}
	if !intervalFuture {
		return booking.Reservation{}, booking.ErrConflict
	}
	if !rateCurrent {
		return booking.Reservation{}, booking.ErrConflict
	}
	if !available {
		return booking.Reservation{}, booking.ErrConflict
	}
	var v booking.Reservation
	err = tx.QueryRow(ctx, `INSERT INTO public.reserva_ensayo_local(id,cotizacion_id,espacio_id,anfitrion_id,arrendatario_id,clave_idempotencia,huella_solicitud,ocupacion_id,estado,precio_unitario_clp,unidades,subtotal_clp,modalidad,moneda,inicio,termino,zona_horaria,condiciones_snapshot,politica_cancelacion_version,pago_vence_en,creada_en,actualizada_en)
SELECT $1,q.id,q.espacio_id,q.anfitrion_id,q.arrendatario_id,$4,$5,$6,'pendiente_de_pago',q.precio_unitario_clp,q.unidades,q.subtotal_clp,q.modalidad,q.moneda,q.inicio,q.termino,q.zona_horaria,q.condiciones_snapshot,q.politica_cancelacion_version,$7,$8,$8
FROM public.cotizacion_reserva_ensayo q JOIN public.espacio e ON e.id=q.espacio_id AND e.propietario_id=q.anfitrion_id
LEFT JOIN public.reserva_ensayo_local_fixture f ON f.espacio_id=e.id AND f.anfitrion_id=e.propietario_id AND f.arrendatario_id=q.arrendatario_id
WHERE q.id=$2 AND q.arrendatario_id=$3 AND q.vence_en>$8 AND q.inicio>$8 AND (((f.habilitada AND e.estado='borrador')) OR (NOT COALESCE(f.habilitada,false) AND e.estado='activa'))
RETURNING `+reservationCols, id, quoteID, renter, key, fingerprint, occupancyID, payExpiresAt, now).Scan(&v.ID, &v.QuoteID, &v.SpaceID, &v.HostID, &v.RenterID, &v.State, &v.RateUnit, &v.UnitPrice, &v.Currency, &v.Units, &v.Subtotal, &v.StartAt, &v.EndAt, &v.TimeZone, &v.Conditions, &v.CancellationPolicyVersion, &v.PayExpiresAt, &v.HostExpiresAt, &v.CreatedAt, &v.UpdatedAt)
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
	if err = r.attachRefund(ctx, r.pool, &v); err != nil {
		return booking.Detail{}, err
	}
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
	rows, err := r.pool.Query(ctx, `SELECT `+reservationCols+`,
(SELECT count(*) FROM public.mensaje_reserva_ensayo m
 WHERE m.reserva_id=r.id AND m.autor_id<>$1
 AND m.secuencia>COALESCE((SELECT c.ultima_secuencia_leida FROM public.reserva_mensaje_lectura c
   WHERE c.reserva_id=r.id AND c.participante_id=$1),0))
FROM public.reserva_ensayo_local r WHERE r.arrendatario_id=$1 OR r.anfitrion_id=$1
ORDER BY r.actualizada_en DESC,r.id`, actor)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []booking.Reservation{}
	for rows.Next() {
		v, e := scanReservationWithUnread(rows)
		if e != nil {
			return nil, e
		}
		if e = r.attachRefund(ctx, r.pool, &v); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (r *Repository) Decide(ctx context.Context, host, id, decision, reason string, clock func() time.Time) (booking.Reservation, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return booking.Reservation{}, err
	}
	defer tx.Rollback(ctx)
	if err := lockReservationParties(ctx, tx, id); err != nil {
		return booking.Reservation{}, err
	}
	v, err := scanReservation(tx.QueryRow(ctx, `SELECT `+reservationCols+` FROM public.reserva_ensayo_local WHERE id=$1 AND anfitrion_id=$2 FOR UPDATE`, id, host))
	if err != nil {
		return booking.Reservation{}, err
	}
	now := clock().UTC()
	expired, err := expiry.LockedReservation(ctx, tx, id, now)
	if err != nil {
		return booking.Reservation{}, err
	}
	if expired {
		if err = tx.Commit(ctx); err != nil {
			return booking.Reservation{}, err
		}
		return booking.Reservation{}, booking.ErrConflict
	}
	if v.State != "pagada" || v.HostExpiresAt == nil || !now.Before(*v.HostExpiresAt) {
		return booking.Reservation{}, booking.ErrConflict
	}
	next := "aprobada_host"
	historyReason := "aprobación del anfitrión en ensayo local"
	if decision == "rechazar" {
		next, historyReason = "rechazada_arrendador", "rechazo del anfitrión en ensayo local: "+reason
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
	err = appendTransition(ctx, tx, id, ptrString("pagada"), next, host, historyReason, now)
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

func (r *Repository) attachRefund(ctx context.Context, queryer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, v *booking.Reservation) error {
	err := queryer.QueryRow(ctx, `SELECT id::text,operacion_id::text,importe_clp,estado,ultimo_resultado,actualizada_en FROM public.reserva_devolucion_ensayo WHERE reserva_id=$1`, v.ID).Scan(&v.RefundID, &v.RefundOperationID, &v.RefundAmountCLP, &v.RefundState, &v.RefundLastResult, &v.RefundUpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	return err
}

func (r *Repository) CancellationPreview(ctx context.Context, renter, id string, now time.Time) (booking.CancellationPreview, error) {
	var preview booking.CancellationPreview
	var state string
	var payDeadline time.Time
	err := r.pool.QueryRow(ctx, `SELECT r.id::text,r.politica_cancelacion_version,r.estado,r.inicio,r.pago_vence_en,r.moneda,
COALESCE((SELECT sum(p.importe_clp) FROM public.reserva_pago_ensayo p WHERE p.reserva_id=r.id AND p.resultado='exito_simulado'),0)
FROM public.reserva_ensayo_local r WHERE r.id=$1 AND r.arrendatario_id=$2`, id, renter).Scan(&preview.ReservationID, &preview.PolicyVersion, &state, &preview.Deadline, &payDeadline, &preview.Currency, &preview.AmountCLP)
	if err != nil {
		return booking.CancellationPreview{}, mapErr(err)
	}
	if preview.PolicyVersion != booking.LocalCancellationPolicyVersion {
		return booking.CancellationPreview{}, booking.ErrConflict
	}
	preview.RefundLabel = "Devolución simulada — sin movimiento de dinero"
	switch state {
	case "pendiente_de_pago":
		preview.Deadline = payDeadline
		preview.Eligible = now.Before(payDeadline)
		preview.ReasonCode = "sin_devolucion"
		preview.AmountCLP = 0
	case "pagada", "aprobada_host", "firma_parcial", "lista_para_checkin":
		preview.Eligible = now.Before(preview.Deadline) && preview.AmountCLP > 0
		if preview.Eligible {
			preview.ReasonCode = "devolucion_simulada_completa"
		} else if !now.Before(preview.Deadline) {
			preview.ReasonCode = "inicio_alcanzado"
		} else {
			preview.ReasonCode = "pago_fake_no_confirmado"
		}
	default:
		preview.ReasonCode = "estado_no_cancelable"
	}
	return preview, nil
}

func (r *Repository) Cancel(ctx context.Context, renter, id, key, reason string, fingerprint []byte, clock func() time.Time) (booking.CancellationResult, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return booking.CancellationResult{}, err
	}
	defer tx.Rollback(ctx)
	if err := lockReservationParties(ctx, tx, id); err != nil {
		return booking.CancellationResult{}, err
	}
	v, err := scanReservation(tx.QueryRow(ctx, `SELECT `+reservationCols+` FROM public.reserva_ensayo_local WHERE id=$1 AND arrendatario_id=$2 FOR UPDATE`, id, renter))
	if err != nil {
		return booking.CancellationResult{}, err
	}
	var priorKey, priorHash string
	err = tx.QueryRow(ctx, `SELECT clave_idempotencia,encode(huella_solicitud,'hex') FROM public.reserva_cancelacion_ensayo WHERE reserva_id=$1`, id).Scan(&priorKey, &priorHash)
	if err == nil {
		if priorKey != key || priorHash != fmtHex(fingerprint) {
			return booking.CancellationResult{}, booking.ErrConflict
		}
		if err = r.attachRefund(ctx, tx, &v); err != nil {
			return booking.CancellationResult{}, err
		}
		if err = tx.Commit(ctx); err != nil {
			return booking.CancellationResult{}, err
		}
		result := cancellationResult(v)
		result.Replayed = true
		return result, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return booking.CancellationResult{}, err
	}
	// Read Backend time after the reservation lock. If a deadline elapsed while
	// this operation waited, persist the expiry transition in this transaction.
	now := clock().UTC()
	expired, err := expiry.LockedReservation(ctx, tx, id, now)
	if err != nil {
		return booking.CancellationResult{}, err
	}
	if expired {
		if err = tx.Commit(ctx); err != nil {
			return booking.CancellationResult{}, err
		}
		return booking.CancellationResult{}, booking.ErrConflict
	}
	if v.CancellationPolicyVersion != booking.LocalCancellationPolicyVersion {
		return booking.CancellationResult{}, booking.ErrConflict
	}
	if err = contractspg.CancelForReservation(ctx, tx, id, renter, now); err != nil {
		return booking.CancellationResult{}, err
	}
	var refundAmount *int64
	if v.State == "pendiente_de_pago" {
		if !now.Before(v.PayExpiresAt) {
			return booking.CancellationResult{}, booking.ErrConflict
		}
	} else if v.State == "pagada" || v.State == "aprobada_host" || v.State == "firma_parcial" || v.State == "lista_para_checkin" {
		if !now.Before(v.StartAt) {
			return booking.CancellationResult{}, booking.ErrConflict
		}
		var paid int64
		if err = tx.QueryRow(ctx, `SELECT COALESCE(sum(importe_clp),0) FROM public.reserva_pago_ensayo WHERE reserva_id=$1 AND resultado='exito_simulado'`, id).Scan(&paid); err != nil {
			return booking.CancellationResult{}, err
		}
		if paid <= 0 {
			return booking.CancellationResult{}, booking.ErrConflict
		}
		refundAmount = &paid
	} else {
		return booking.CancellationResult{}, booking.ErrConflict
	}
	_, err = tx.Exec(ctx, `UPDATE public.ocupacion SET activo=false,desactivada_en=$2 WHERE reserva_id=$1 AND activo`, id, now)
	if err != nil {
		return booking.CancellationResult{}, err
	}
	_, err = tx.Exec(ctx, `UPDATE public.reserva_ensayo_local SET estado='cancelada_arrendatario',actualizada_en=$2 WHERE id=$1`, id, now)
	if err != nil {
		return booking.CancellationResult{}, err
	}
	if _, err = tx.Exec(ctx, `UPDATE public.reserva_pago_ensayo_operacion SET estado='vencida',actualizada_en=$2 WHERE reserva_id=$1 AND estado='pendiente'`, id, now); err != nil {
		return booking.CancellationResult{}, err
	}
	historyReason := "cancelación local antes del pago"
	if refundAmount != nil {
		historyReason = "cancelación local con devolución simulada pendiente"
	}
	if reason != "" {
		historyReason += ": " + reason
	}
	err = appendTransition(ctx, tx, id, ptrString(v.State), "cancelada_arrendatario", renter, historyReason, now)
	if err != nil {
		return booking.CancellationResult{}, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO public.reserva_cancelacion_ensayo(id,reserva_id,arrendatario_id,clave_idempotencia,huella_solicitud,motivo,creada_en) VALUES(gen_random_uuid(),$1,$2,$3,$4,$5,$6)`, id, renter, key, fingerprint, reason, now)
	if err != nil {
		return booking.CancellationResult{}, mapErr(err)
	}
	if refundAmount != nil {
		_, err = tx.Exec(ctx, `INSERT INTO public.reserva_devolucion_ensayo(id,reserva_id,operacion_id,importe_clp,moneda,estado,creada_en,actualizada_en) VALUES(gen_random_uuid(),$1,gen_random_uuid(),$2,'CLP','pendiente',$3,$3)`, id, *refundAmount, now)
		if err != nil {
			return booking.CancellationResult{}, mapErr(err)
		}
	}
	v, err = scanReservation(tx.QueryRow(ctx, `SELECT `+reservationCols+` FROM public.reserva_ensayo_local WHERE id=$1`, id))
	if err != nil {
		return booking.CancellationResult{}, err
	}
	if err = r.attachRefund(ctx, tx, &v); err != nil {
		return booking.CancellationResult{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return booking.CancellationResult{}, mapErr(err)
	}
	result := cancellationResult(v)
	if refundAmount != nil {
		result.RefundAmountCLP = refundAmount
		result.RefundState = "pendiente"
	}
	return result, nil
}

func cancellationResult(v booking.Reservation) booking.CancellationResult {
	result := booking.CancellationResult{Reservation: v, RefundState: "no_aplica"}
	if v.RefundState != nil {
		result.RefundState = *v.RefundState
	}
	result.RefundAmountCLP = v.RefundAmountCLP
	return result
}

func (r *Repository) RefundOperation(ctx context.Context, renter, id string) (booking.RefundResult, error) {
	var value booking.RefundResult
	err := r.pool.QueryRow(ctx, `SELECT d.reserva_id::text,d.operacion_id::text,d.importe_clp,d.moneda,d.estado,COALESCE(d.ultimo_resultado,''),d.actualizada_en
FROM public.reserva_devolucion_ensayo d JOIN public.reserva_ensayo_local r ON r.id=d.reserva_id
WHERE d.reserva_id=$1 AND r.arrendatario_id=$2 AND r.estado IN ('cancelada_arrendatario','cancelada_por_firma')`, id, renter).Scan(&value.ReservationID, &value.OperationID, &value.AmountCLP, &value.Currency, &value.State, &value.LastResult, &value.UpdatedAt)
	if err != nil {
		return booking.RefundResult{}, mapErr(err)
	}
	return value, nil
}

func (r *Repository) RecordRefund(ctx context.Context, renter, id, result string, now time.Time) (booking.RefundResult, error) {
	if result != "exito_simulado" && result != "fallo_simulado" && result != "sin_respuesta_simulada" {
		return booking.RefundResult{}, booking.ErrInvalid
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return booking.RefundResult{}, err
	}
	defer tx.Rollback(ctx)
	if err := lockReservationParties(ctx, tx, id); err != nil {
		return booking.RefundResult{}, err
	}
	var state, operationID, currency, lastResult string
	var amount int64
	var updatedAt time.Time
	err = tx.QueryRow(ctx, `SELECT d.operacion_id::text,d.importe_clp,d.moneda,d.estado,COALESCE(d.ultimo_resultado,''),d.actualizada_en FROM public.reserva_devolucion_ensayo d JOIN public.reserva_ensayo_local r ON r.id=d.reserva_id WHERE d.reserva_id=$1 AND r.arrendatario_id=$2 AND r.estado IN ('cancelada_arrendatario','cancelada_por_firma') FOR UPDATE OF r,d`, id, renter).Scan(&operationID, &amount, &currency, &state, &lastResult, &updatedAt)
	if err != nil {
		return booking.RefundResult{}, mapErr(err)
	}
	if state == "completada" {
		if err = tx.Commit(ctx); err != nil {
			return booking.RefundResult{}, err
		}
		return booking.RefundResult{ReservationID: id, OperationID: operationID, AmountCLP: amount, Currency: currency, State: state, LastResult: lastResult, UpdatedAt: updatedAt, Reused: true}, nil
	}
	var sequence int64
	if err = tx.QueryRow(ctx, `SELECT COALESCE(MAX(secuencia),0)+1 FROM public.reserva_devolucion_intento_ensayo WHERE devolucion_id=(SELECT id FROM public.reserva_devolucion_ensayo WHERE reserva_id=$1)`, id).Scan(&sequence); err != nil {
		return booking.RefundResult{}, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO public.reserva_devolucion_intento_ensayo(id,devolucion_id,secuencia,resultado,creada_en) SELECT gen_random_uuid(),id,$2,$3,$4 FROM public.reserva_devolucion_ensayo WHERE reserva_id=$1`, id, sequence, result, now)
	if err != nil {
		return booking.RefundResult{}, err
	}
	completed := result == "exito_simulado"
	state = "pendiente"
	var completedAt any
	if completed {
		state = "completada"
		completedAt = now
	}
	_, err = tx.Exec(ctx, `UPDATE public.reserva_devolucion_ensayo SET estado=$2,ultimo_resultado=$3,actualizada_en=$4,completada_en=$5 WHERE reserva_id=$1`, id, state, result, now, completedAt)
	if err != nil {
		return booking.RefundResult{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return booking.RefundResult{}, err
	}
	return booking.RefundResult{ReservationID: id, OperationID: operationID, AmountCLP: amount, Currency: currency, State: state, LastResult: result, UpdatedAt: now}, nil
}

func (r *Repository) NoticeRecipients(ctx context.Context, actor, id string) ([]string, error) {
	var hostEmail, renterEmail string
	err := r.pool.QueryRow(ctx, `SELECT h.correo_original,a.correo_original FROM public.reserva_ensayo_local r JOIN public.usuario h ON h.id=r.anfitrion_id JOIN public.usuario a ON a.id=r.arrendatario_id WHERE r.id=$1 AND (r.anfitrion_id=$2 OR r.arrendatario_id=$2)`, id, actor).Scan(&hostEmail, &renterEmail)
	if err != nil {
		return nil, mapErr(err)
	}
	return []string{hostEmail, renterEmail}, nil
}

func (r *Repository) Expire(ctx context.Context, now time.Time) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `SELECT id::text FROM public.reserva_ensayo_local WHERE (estado='pendiente_de_pago' AND pago_vence_en<=$1) OR (estado='pagada' AND anfitrion_vence_en<=$1) FOR UPDATE SKIP LOCKED`, now)
	if err != nil {
		return err
	}
	all := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		all = append(all, id)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, id := range all {
		if _, err = expiry.LockedReservation(ctx, tx, id, now); err != nil {
			return err
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
