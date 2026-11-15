package rentalflowpg

import (
	"context"

	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/accountlock"
	"github.com/HernanEspinozaDev/espaciGo/internal/booking"
	"github.com/HernanEspinozaDev/espaciGo/internal/calendar"
	"github.com/HernanEspinozaDev/espaciGo/internal/pricing"
	"github.com/HernanEspinozaDev/espaciGo/internal/rentalflow"
	"github.com/HernanEspinozaDev/espaciGo/internal/verification"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type DB interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

type Tx interface {
	DB
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

type TxBeginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

type Repository struct {
	db              DB
	beginner        TxBeginner
	now             func() time.Time
	newID           func() string
	declarationKeys verification.DeclarationOpener
}

func NewRepository(db DB, beginner TxBeginner, now func() time.Time) (*Repository, error) {
	return NewRepositoryWithDeclarationKeyring(db, beginner, now, nil)
}

func NewRepositoryWithDeclarationKeyring(db DB, beginner TxBeginner, now func() time.Time, keys verification.DeclarationOpener) (*Repository, error) {
	if db == nil || now == nil {
		return nil, rentalflow.ErrInvalid
	}
	return &Repository{db: db, beginner: beginner, now: now, newID: func() string { return uuid.NewString() }, declarationKeys: keys}, nil
}

func (r *Repository) PublicListing(ctx context.Context, spaceID string) (rentalflow.Listing, error) {
	if r == nil || ctx == nil || strings.TrimSpace(spaceID) == "" {
		return rentalflow.Listing{}, rentalflow.ErrInvalid
	}
	const q = `SELECT e.id::text, e.propietario_id::text, e.estado, e.titulo, t.modalidad, t.precio_base_clp, t.moneda, t.version, e.zona_horaria
FROM public.espacio e
JOIN LATERAL (
    SELECT version, modalidad, precio_base_clp, moneda
    FROM public.tarifa_espacio
    WHERE espacio_id = e.id
    ORDER BY version DESC LIMIT 1
) t ON true
WHERE e.id = $1 AND e.estado = 'activa'`

	var l rentalflow.Listing
	err := r.db.QueryRow(ctx, q, spaceID).Scan(
		&l.ID, &l.OwnerID, &l.State, &l.Title,
		&l.RateUnit, &l.UnitPriceCLP, &l.Currency, &l.RateVersion, &l.TimeZone,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return rentalflow.Listing{}, rentalflow.ErrNotFound
	}
	if err != nil {
		return rentalflow.Listing{}, err
	}
	return l, nil
}

func (r *Repository) IntervalAvailable(ctx context.Context, listing rentalflow.Listing, interval rentalflow.Interval) (bool, error) {
	if r == nil || ctx == nil || strings.TrimSpace(listing.ID) == "" {
		return false, rentalflow.ErrInvalid
	}
	const q = `SELECT NOT EXISTS (
    SELECT 1 FROM public.ocupacion
    WHERE espacio_id = $1 AND activo AND intervalo && tstzrange($2, $3, '[)')
)`
	var available bool
	err := r.db.QueryRow(ctx, q, listing.ID, interval.StartAt, interval.EndAt).Scan(&available)
	if err != nil {
		return false, err
	}
	return available, nil
}

func (r *Repository) PublicIntervals(ctx context.Context, listing rentalflow.Listing, dateText string, duration int) ([]rentalflow.Interval, error) {
	if r == nil || ctx == nil || strings.TrimSpace(listing.ID) == "" || !rentalflow.ValidDuration(listing.RateUnit, duration) {
		return nil, rentalflow.ErrInvalid
	}
	loc, err := time.LoadLocation(listing.TimeZone)
	if err != nil {
		return nil, rentalflow.ErrInvalid
	}
	date, err := time.Parse("2006-01-02", dateText)
	if err != nil || date.Format("2006-01-02") != dateText {
		return nil, rentalflow.ErrInvalid
	}
	start, ok := calendar.FirstInstantOfDate(date.Year(), date.Month(), date.Day(), loc)
	if !ok {
		return nil, rentalflow.ErrInvalid
	}
	nextDate := date.AddDate(0, 0, 1)
	endOfDate, ok := calendar.FirstInstantOfDate(nextDate.Year(), nextDate.Month(), nextDate.Day(), loc)
	if !ok {
		return nil, rentalflow.ErrInvalid
	}
	var schedule booking.WeeklyHours
	const scheduleSQL = `(SELECT jsonb_build_object('enabled',h.activo,'days',(
SELECT jsonb_agg(jsonb_build_object('weekday',d.weekday,'periods',COALESCE((SELECT jsonb_agg(jsonb_build_object('open',CASE WHEN s.apertura_minuto=1440 THEN '24:00' ELSE to_char(make_time(s.apertura_minuto/60,s.apertura_minuto%60,0),'HH24:MI') END,'close',CASE WHEN s.cierre_minuto=1440 THEN '24:00' ELSE to_char(make_time(s.cierre_minuto/60,s.cierre_minuto%60,0),'HH24:MI') END) ORDER BY s.orden) FROM public.espacio_horario_semanal_tramo s WHERE s.espacio_id=h.espacio_id AND s.dia_iso=d.weekday),'[]'::jsonb)) ORDER BY d.weekday) FROM generate_series(1,7) d(weekday))) FROM public.espacio_horario_semanal h WHERE h.espacio_id=$1)`
	var rawSchedule []byte
	if err = r.db.QueryRow(ctx, `SELECT COALESCE(`+scheduleSQL+`, '{"enabled":false,"days":[]}')`, listing.ID).Scan(&rawSchedule); err != nil {
		return nil, err
	}
	if err = json.Unmarshal(rawSchedule, &schedule); err != nil {
		return nil, err
	}
	if schedule.Enabled {
		if listing.RateUnit != "hora" || len(schedule.Days) != 7 || booking.ValidateWeeklyHours(schedule) != nil {
			return nil, rentalflow.ErrUnavailable
		}
	}
	candidates := make([]rentalflow.Interval, 0)
	if listing.RateUnit == "hora" {
		for candidateStart := start; candidateStart.Before(endOfDate); candidateStart = candidateStart.Add(30 * time.Minute) {
			local := candidateStart.In(loc)
			_, minute, second := local.Clock()
			if minute%30 != 0 || second != 0 || local.Nanosecond() != 0 {
				continue
			}
			candidate := rentalflow.Interval{StartAt: candidateStart, EndAt: candidateStart.Add(time.Duration(duration) * time.Hour)}
			if schedule.Enabled && !booking.IntervalFitsWeeklyHours(candidate.StartAt, candidate.EndAt, listing.TimeZone, schedule) {
				continue
			}
			candidates = append(candidates, candidate)
		}
	} else if listing.RateUnit == "dia" || listing.RateUnit == "mes" {
		candidateEnd := endOfDate
		if listing.RateUnit == "dia" {
			candidateDate := date.AddDate(0, 0, duration)
			candidateEnd, ok = calendar.FirstInstantOfDate(candidateDate.Year(), candidateDate.Month(), candidateDate.Day(), loc)
		} else {
			candidateDate, monthOK := calendar.AddMonthsClamped(date, int64(duration))
			candidateEnd, ok = calendar.FirstInstantOfDate(candidateDate.Year(), candidateDate.Month(), candidateDate.Day(), loc)
			ok = ok && monthOK
		}
		if !ok {
			return nil, rentalflow.ErrInvalid
		}
		candidates = append(candidates, rentalflow.Interval{StartAt: start, EndAt: candidateEnd})
	} else {
		return nil, rentalflow.ErrInvalid
	}
	starts := make([]time.Time, len(candidates))
	ends := make([]time.Time, len(candidates))
	for i, candidate := range candidates {
		starts[i] = candidate.StartAt
		ends[i] = candidate.EndAt
	}
	const occupancySQL = `WITH candidates AS (
    SELECT start_at, end_at, ordinal
    FROM unnest($2::timestamptz[], $3::timestamptz[]) WITH ORDINALITY AS c(start_at, end_at, ordinal)
), available AS (
    SELECT c.start_at, c.end_at, c.ordinal
    FROM candidates c
    WHERE NOT EXISTS (
        SELECT 1 FROM public.ocupacion o
        WHERE o.espacio_id=$1 AND o.activo
          AND o.intervalo && tstzrange(c.start_at, c.end_at, '[)')
    )
)
SELECT COALESCE(array_agg(start_at ORDER BY ordinal), ARRAY[]::timestamptz[]),
       COALESCE(array_agg(end_at ORDER BY ordinal), ARRAY[]::timestamptz[])
FROM available`
	var availableStarts, availableEnds []time.Time
	if err = r.db.QueryRow(ctx, occupancySQL, listing.ID, starts, ends).Scan(&availableStarts, &availableEnds); err != nil {
		return nil, err
	}
	if len(availableStarts) != len(availableEnds) {
		return nil, rentalflow.ErrUnavailable
	}
	available := make([]rentalflow.Interval, 0, len(availableStarts))
	for i := range availableStarts {
		available = append(available, rentalflow.Interval{StartAt: availableStarts[i], EndAt: availableEnds[i]})
	}
	return available, nil
}

func (r *Repository) CreateValidatedQuote(
	ctx context.Context,
	renterID string,
	listing rentalflow.Listing,
	interval rentalflow.Interval,
	units int64,
	subtotal int64,
	createdAt time.Time,
) (rentalflow.Quote, error) {
	if r == nil || ctx == nil || strings.TrimSpace(renterID) == "" || strings.TrimSpace(listing.ID) == "" || createdAt.IsZero() {
		return rentalflow.Quote{}, rentalflow.ErrInvalid
	}
	if renterID == listing.OwnerID {
		return rentalflow.Quote{}, rentalflow.ErrConflict
	}
	if r.beginner == nil {
		return rentalflow.Quote{}, rentalflow.ErrUnavailable
	}

	createdAt = createdAt.UTC()
	if createdAt.After(r.now().UTC()) {
		return rentalflow.Quote{}, rentalflow.ErrInvalid
	}
	if !interval.StartAt.Before(interval.EndAt) || units <= 0 || uint64(units) > uint64(^uint(0)>>1) {
		return rentalflow.Quote{}, rentalflow.ErrInvalid
	}
	calculatedUnits, err := pricing.BilledUnits(listing.RateUnit, interval.StartAt, interval.EndAt, listing.TimeZone)
	if err != nil || calculatedUnits != units || listing.UnitPriceCLP <= 0 || units > int64(^uint64(0)>>1)/listing.UnitPriceCLP || subtotal != listing.UnitPriceCLP*units {
		return rentalflow.Quote{}, rentalflow.ErrInvalid
	}

	tx, err := r.beginner.Begin(ctx)
	if err != nil {
		return rentalflow.Quote{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	txRepo := &Repository{db: tx, now: r.now, newID: r.newID}
	// Suppression takes this same per-account advisory lock before it updates
	// spaces and deletes quotes. Lock both participants first, in stable order,
	// so suppression cannot pass its DELETE before this insert and no
	// space/account lock-order inversion can deadlock the transactions. Advisory
	// locks also preserve the runtime role's least-privilege SELECT-only access
	// to usuario; SELECT FOR SHARE would additionally require UPDATE privilege.
	if err := accountlock.LockAccountIDs(ctx, tx, renterID, listing.OwnerID); err != nil {
		return rentalflow.Quote{}, err
	}
	for _, participantID := range []string{renterID, listing.OwnerID} {
		var accountState string
		var administrativelyBlocked bool
		err = tx.QueryRow(ctx, `SELECT u.estado, EXISTS(SELECT 1 FROM public.bloqueo_cuenta_administrativo_local b WHERE b.cuenta_id=u.id)
			FROM public.usuario u WHERE u.id=$1`, participantID).Scan(&accountState, &administrativelyBlocked)
		if errors.Is(err, pgx.ErrNoRows) {
			return rentalflow.Quote{}, rentalflow.ErrNotFound
		}
		if err != nil {
			return rentalflow.Quote{}, err
		}
		if accountState != "activo" || administrativelyBlocked {
			return rentalflow.Quote{}, rentalflow.ErrNotFound
		}
	}
	var lockedSpaceID string
	err = tx.QueryRow(ctx, `SELECT id::text FROM public.espacio WHERE id=$1 AND estado='activa' FOR SHARE`, listing.ID).Scan(&lockedSpaceID)
	if errors.Is(err, pgx.ErrNoRows) {
		return rentalflow.Quote{}, rentalflow.ErrNotFound
	}
	if err != nil {
		return rentalflow.Quote{}, err
	}
	currentListing, err := txRepo.PublicListing(ctx, listing.ID)
	if err != nil {
		return rentalflow.Quote{}, err
	}
	if currentListing != listing {
		return rentalflow.Quote{}, rentalflow.ErrConflict
	}
	location, err := time.LoadLocation(currentListing.TimeZone)
	if err != nil {
		return rentalflow.Quote{}, rentalflow.ErrInvalid
	}
	dateText := interval.StartAt.In(location).Format("2006-01-02")
	available, err := txRepo.PublicIntervals(ctx, currentListing, dateText, int(units))
	if err != nil {
		return rentalflow.Quote{}, err
	}
	validInterval := false
	for _, candidate := range available {
		if candidate.StartAt.Equal(interval.StartAt) && candidate.EndAt.Equal(interval.EndAt) {
			validInterval = true
			break
		}
	}
	if !validInterval {
		return rentalflow.Quote{}, rentalflow.ErrConflict
	}

	createdAt = createdAt.UTC()
	expiresAt := createdAt.Add(rentalflow.QuoteLifetime)
	quoteID := r.newID()
	const insertQ = `INSERT INTO public.cotizacion_arriendo_comercial (
	    id, espacio_id, propietario_id, arrendatario_id, titulo_espacio,
	    tarifa_version, modalidad, precio_unitario_clp, moneda,
	    unidades, subtotal_clp, inicio, termino, zona_horaria, creada_en, vence_en
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)`
	_, err = tx.Exec(ctx, insertQ,
		quoteID, listing.ID, listing.OwnerID, renterID, listing.Title,
		listing.RateVersion, listing.RateUnit, listing.UnitPriceCLP, listing.Currency,
		units, subtotal, interval.StartAt, interval.EndAt, listing.TimeZone,
		createdAt, expiresAt,
	)
	if err != nil {
		return rentalflow.Quote{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return rentalflow.Quote{}, err
	}

	return rentalflow.Quote{
		ID:           quoteID,
		SpaceID:      listing.ID,
		OwnerID:      listing.OwnerID,
		RenterID:     renterID,
		SpaceTitle:   listing.Title,
		RateVersion:  listing.RateVersion,
		RateUnit:     listing.RateUnit,
		UnitPriceCLP: listing.UnitPriceCLP,
		Currency:     listing.Currency,
		Units:        units,
		SubtotalCLP:  subtotal,
		StartAt:      interval.StartAt,
		EndAt:        interval.EndAt,
		TimeZone:     listing.TimeZone,
		CreatedAt:    createdAt,
		ExpiresAt:    expiresAt,
	}, nil
}

func (r *Repository) PurgeExpiredCommercialQuotes(ctx context.Context, at time.Time, limit int) (int64, error) {
	if r == nil || ctx == nil || at.IsZero() || limit < 1 || limit > 500 {
		return 0, rentalflow.ErrInvalid
	}
	const q = `WITH due AS (
	SELECT id FROM public.cotizacion_arriendo_comercial
	WHERE vence_en <= $1
	ORDER BY vence_en,id
	LIMIT $2
)
DELETE FROM public.cotizacion_arriendo_comercial q
USING due
WHERE q.id=due.id`
	tag, err := r.db.Exec(ctx, q, at.UTC(), limit)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func (r *Repository) GetForParticipant(ctx context.Context, actorID string, quoteID string) (rentalflow.Quote, error) {
	if r == nil || ctx == nil || strings.TrimSpace(actorID) == "" || strings.TrimSpace(quoteID) == "" {
		return rentalflow.Quote{}, rentalflow.ErrInvalid
	}
	const q = `SELECT
    q.id::text,
    q.espacio_id::text,
    q.propietario_id::text,
    q.arrendatario_id::text,
    q.titulo_espacio,
    q.tarifa_version,
    q.modalidad,
    q.precio_unitario_clp,
    q.moneda,
    q.unidades,
    q.subtotal_clp,
    q.inicio,
    q.termino,
    q.zona_horaria,
    q.creada_en,
    q.vence_en
FROM public.cotizacion_arriendo_comercial q
WHERE q.id = $1 AND (q.propietario_id = $2 OR q.arrendatario_id = $2)`

	var quote rentalflow.Quote
	err := r.db.QueryRow(ctx, q, quoteID, actorID).Scan(
		&quote.ID,
		&quote.SpaceID,
		&quote.OwnerID,
		&quote.RenterID,
		&quote.SpaceTitle,
		&quote.RateVersion,
		&quote.RateUnit,
		&quote.UnitPriceCLP,
		&quote.Currency,
		&quote.Units,
		&quote.SubtotalCLP,
		&quote.StartAt,
		&quote.EndAt,
		&quote.TimeZone,
		&quote.CreatedAt,
		&quote.ExpiresAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return rentalflow.Quote{}, rentalflow.ErrNotFound
	}
	if err != nil {
		return rentalflow.Quote{}, err
	}
	return quote, nil
}

func (r *Repository) CurrentForRentalContract(
	ctx context.Context,
	actorID string,
	quoteID string,
	subjectID string,
	asOf time.Time,
) (rentalflow.ContractIdentity, error) {
	if r == nil || ctx == nil || strings.TrimSpace(actorID) == "" || strings.TrimSpace(quoteID) == "" || strings.TrimSpace(subjectID) == "" || asOf.IsZero() {
		return rentalflow.ContractIdentity{}, rentalflow.ErrInvalid
	}
	if r.db == nil {
		return rentalflow.ContractIdentity{}, rentalflow.ErrIdentityUnavailable
	}
	const participantCheck = `SELECT 1 FROM public.cotizacion_arriendo_comercial
WHERE id = $1 AND (propietario_id = $2 OR arrendatario_id = $2)
  AND (propietario_id = $3 OR arrendatario_id = $3)`
	if r.beginner == nil {
		var check int
		err := r.db.QueryRow(ctx, participantCheck, quoteID, actorID, subjectID).Scan(&check)
		if errors.Is(err, pgx.ErrNoRows) {
			return rentalflow.ContractIdentity{}, rentalflow.ErrNotFound
		}
		if err != nil {
			return rentalflow.ContractIdentity{}, err
		}
		return rentalflow.ContractIdentity{}, rentalflow.ErrIdentityUnavailable
	}
	tx, err := r.beginner.Begin(ctx)
	if err != nil {
		return rentalflow.ContractIdentity{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, `SET TRANSACTION ISOLATION LEVEL REPEATABLE READ READ ONLY`); err != nil {
		return rentalflow.ContractIdentity{}, err
	}
	var check int
	err = tx.QueryRow(ctx, participantCheck, quoteID, actorID, subjectID).Scan(&check)
	if errors.Is(err, pgx.ErrNoRows) {
		return rentalflow.ContractIdentity{}, rentalflow.ErrNotFound
	}
	if err != nil {
		return rentalflow.ContractIdentity{}, err
	}
	if err := ctx.Err(); err != nil {
		return rentalflow.ContractIdentity{}, err
	}
	if r.declarationKeys == nil {
		return rentalflow.ContractIdentity{}, rentalflow.ErrIdentityUnavailable
	}
	const declarationQ = `WITH current_case AS (
    SELECT id, subject_id, purpose, state, verified_at, verified_until,
           document_expires_at, revoked_at, identity_declaration_revision_id, updated_at
    FROM public.document_verification_case
    WHERE purpose='identity' AND subject_id=$1 AND state IN ('aprobada','revocada','vencida')
    ORDER BY updated_at DESC,id DESC LIMIT 1
)
SELECT current_case.identity_declaration_revision_id::text,
       d.account_id::text,d.id::text,d.revision,d.schema_version,d.key_version,d.ciphertext,
       d.created_at,d.consent_given_at,d.retention_at
FROM current_case
JOIN public.identity_declaration_revision d
  ON d.id=current_case.identity_declaration_revision_id AND d.account_id=current_case.subject_id
WHERE current_case.purpose='identity' AND current_case.subject_id=$1 AND current_case.state='aprobada'
  AND current_case.verified_at<=$2 AND current_case.verified_until>$2
  AND current_case.document_expires_at>$2 AND current_case.revoked_at IS NULL
  AND current_case.updated_at<=$2
  AND current_case.identity_declaration_revision_id=(
      SELECT latest.id FROM public.identity_declaration_revision latest
      WHERE latest.account_id=current_case.subject_id ORDER BY latest.revision DESC LIMIT 1
  )
  AND d.created_at<=$2 AND d.consent_given_at<=$2 AND d.retention_at>$2`
	var boundRevisionID string
	var row verification.EncryptedDeclarationRevision
	asOf = asOf.UTC()
	err = tx.QueryRow(ctx, declarationQ, subjectID, asOf).Scan(
		&boundRevisionID, &row.AccountID, &row.DeclarationID, &row.Revision,
		&row.SchemaVersion, &row.KeyVersion, &row.Ciphertext,
		&row.CreatedAt, &row.ConsentAt, &row.RetainUntil,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return rentalflow.ContractIdentity{}, rentalflow.ErrIdentityUnavailable
	}
	if err != nil {
		return rentalflow.ContractIdentity{}, err
	}
	projection, err := verification.DecodeDeclarationRevision(row, subjectID, boundRevisionID, asOf, r.declarationKeys)
	if err != nil {
		return rentalflow.ContractIdentity{}, rentalflow.ErrIdentityUnavailable
	}
	if err := ctx.Err(); err != nil {
		return rentalflow.ContractIdentity{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return rentalflow.ContractIdentity{}, err
	}
	if err := ctx.Err(); err != nil {
		return rentalflow.ContractIdentity{}, err
	}
	return rentalflow.ContractIdentity{SubjectID: subjectID, LegalName: projection.LegalFirstName + " " + projection.LegalLastName, RUT: projection.CanonicalRUT}, nil
}
