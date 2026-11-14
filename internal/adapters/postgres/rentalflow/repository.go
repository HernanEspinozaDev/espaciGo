package rentalflowpg

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/rentalflow"
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
	db         DB
	beginner   TxBeginner
	now        func() time.Time
	newID      func() string
}

func NewRepository(db DB, beginner TxBeginner, now func() time.Time) (*Repository, error) {
	if db == nil || now == nil {
		return nil, rentalflow.ErrInvalid
	}
	return &Repository{
		db:       db,
		beginner: beginner,
		now:      now,
		newID:    func() string { return uuid.NewString() },
	}, nil
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
	if r == nil || ctx == nil || strings.TrimSpace(listing.ID) == "" || duration <= 0 {
		return nil, rentalflow.ErrInvalid
	}
	// Candidate generation delegates to domain interval verification
	return []rentalflow.Interval{}, nil
}

func (r *Repository) CreateValidatedQuote(
	ctx context.Context,
	renterID string,
	listing rentalflow.Listing,
	interval rentalflow.Interval,
	units int64,
	subtotal int64,
	expiresAt time.Time,
) (rentalflow.Quote, error) {
	if r == nil || ctx == nil || strings.TrimSpace(renterID) == "" || strings.TrimSpace(listing.ID) == "" {
		return rentalflow.Quote{}, rentalflow.ErrInvalid
	}
	if renterID == listing.OwnerID {
		return rentalflow.Quote{}, rentalflow.ErrConflict
	}

	now := r.now().UTC()
	quoteID := r.newID()

	const insertQ = `INSERT INTO public.cotizacion_reserva_ensayo (
    id, espacio_id, anfitrion_id, arrendatario_id,
    tarifa_version, modalidad, precio_unitario_clp, moneda,
    unidades, subtotal_clp, inicio, termino, zona_horaria,
    condiciones_snapshot, creada_en, vence_en
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)`

	condSnapshot := fmt.Sprintf("Condiciones sintéticas para %s", listing.Title)
	_, err := r.db.Exec(ctx, insertQ,
		quoteID, listing.ID, listing.OwnerID, renterID,
		listing.RateVersion, listing.RateUnit, listing.UnitPriceCLP, listing.Currency,
		units, subtotal, interval.StartAt, interval.EndAt, listing.TimeZone,
		condSnapshot, now, expiresAt,
	)
	if err != nil {
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
		CreatedAt:    now,
		ExpiresAt:    expiresAt,
	}, nil
}

func (r *Repository) GetForParticipant(ctx context.Context, actorID string, quoteID string) (rentalflow.Quote, error) {
	if r == nil || ctx == nil || strings.TrimSpace(actorID) == "" || strings.TrimSpace(quoteID) == "" {
		return rentalflow.Quote{}, rentalflow.ErrInvalid
	}
	const q = `SELECT
    q.id::text,
    q.espacio_id::text,
    q.anfitrion_id::text,
    q.arrendatario_id::text,
    COALESCE(e.titulo, ''),
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
FROM public.cotizacion_reserva_ensayo q
LEFT JOIN public.espacio e ON e.id = q.espacio_id
WHERE q.id = $1 AND (q.anfitrion_id = $2 OR q.arrendatario_id = $2)`

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
	if r == nil || ctx == nil || strings.TrimSpace(actorID) == "" || strings.TrimSpace(quoteID) == "" || strings.TrimSpace(subjectID) == "" {
		return rentalflow.ContractIdentity{}, rentalflow.ErrInvalid
	}

	// Verify actor participates in quote and subject is one of the parties
	const participantCheck = `SELECT 1 FROM public.cotizacion_reserva_ensayo
WHERE id = $1 AND (anfitrion_id = $2 OR arrendatario_id = $2)
  AND (anfitrion_id = $3 OR arrendatario_id = $3)`

	var check int
	err := r.db.QueryRow(ctx, participantCheck, quoteID, actorID, subjectID).Scan(&check)
	if errors.Is(err, pgx.ErrNoRows) {
		return rentalflow.ContractIdentity{}, rentalflow.ErrNotFound
	}
	if err != nil {
		return rentalflow.ContractIdentity{}, err
	}

	// Read approved identity verification from Didit attempts (V84)
	const kycQ = `SELECT declared_first_name || ' ' || declared_last_name, declared_rut_normalized
FROM public.identity_verification_attempts
WHERE account_id = $1 AND status = 'APPROVED' AND (verified_at IS NOT NULL AND verified_at <= $2)
ORDER BY updated_at DESC LIMIT 1`

	var name, rut sql.NullString
	err = r.db.QueryRow(ctx, kycQ, subjectID, asOf).Scan(&name, &rut)
	if errors.Is(err, pgx.ErrNoRows) || !name.Valid || !rut.Valid || strings.TrimSpace(name.String) == "" || strings.TrimSpace(rut.String) == "" {
		// Fallback to manual document verification case if Didit attempt not present
		const manualQ = `SELECT COALESCE(u.nombre, '') || ' ' || COALESCE(u.apellido, ''), COALESCE(u.rut, '')
FROM public.document_verification_case c
JOIN public.usuario u ON u.id = c.subject_id
WHERE c.purpose = 'identity' AND c.subject_id = $1 AND c.state = 'aprobada' AND (c.verified_at IS NOT NULL AND c.verified_at <= $2)
ORDER BY c.updated_at DESC LIMIT 1`
		err = r.db.QueryRow(ctx, manualQ, subjectID, asOf).Scan(&name, &rut)
		if errors.Is(err, pgx.ErrNoRows) || !name.Valid || !rut.Valid || strings.TrimSpace(name.String) == "" || strings.TrimSpace(rut.String) == "" {
			return rentalflow.ContractIdentity{}, rentalflow.ErrIdentityUnavailable
		}
		if err != nil {
			return rentalflow.ContractIdentity{}, err
		}
	} else if err != nil {
		return rentalflow.ContractIdentity{}, err
	}

	return rentalflow.ContractIdentity{
		SubjectID: subjectID,
		LegalName: strings.TrimSpace(name.String),
		RUT:       strings.TrimSpace(rut.String),
	}, nil
}
