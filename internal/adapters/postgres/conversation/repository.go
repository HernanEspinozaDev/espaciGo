package conversationpg

import (
	"context"
	"errors"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/booking/expiry"
	"github.com/HernanEspinozaDev/espaciGo/internal/conversation"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

const messageColumns = `id::text,reserva_id::text,autor_id::text,secuencia,cuerpo,creada_en`

func scanMessage(row pgx.Row) (conversation.Message, error) {
	var v conversation.Message
	err := row.Scan(&v.ID, &v.ReservationID, &v.AuthorID, &v.Sequence, &v.Body, &v.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return conversation.Message{}, conversation.ErrNotFound
	}
	return v, err
}

func (r *Repository) List(ctx context.Context, actor, reservationID string, before *int64, limit int) (conversation.Page, error) {
	var participant bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.reserva_ensayo_local WHERE id=$1 AND (anfitrion_id=$2 OR arrendatario_id=$2))`, reservationID, actor).Scan(&participant)
	if err != nil {
		return conversation.Page{}, err
	}
	if !participant {
		return conversation.Page{}, conversation.ErrNotFound
	}
	rows, err := r.pool.Query(ctx, `SELECT `+messageColumns+` FROM public.mensaje_reserva_ensayo
WHERE reserva_id=$1 AND ($2::bigint IS NULL OR secuencia<$2)
ORDER BY secuencia DESC LIMIT $3`, reservationID, before, limit+1)
	if err != nil {
		return conversation.Page{}, err
	}
	defer rows.Close()
	descending := make([]conversation.Message, 0, limit+1)
	for rows.Next() {
		var item conversation.Message
		if err = rows.Scan(&item.ID, &item.ReservationID, &item.AuthorID, &item.Sequence, &item.Body, &item.CreatedAt); err != nil {
			return conversation.Page{}, err
		}
		descending = append(descending, item)
	}
	if err = rows.Err(); err != nil {
		return conversation.Page{}, err
	}
	hasOlder := len(descending) > limit
	if hasOlder {
		descending = descending[:limit]
	}
	items := make([]conversation.Message, len(descending))
	for i := range descending {
		items[len(descending)-1-i] = descending[i]
	}
	page := conversation.Page{Items: items}
	if hasOlder && len(items) > 0 {
		cursor := items[0].Sequence
		page.OlderCursor = &cursor
	}
	return page, nil
}

func (r *Repository) MarkRead(ctx context.Context, actor, reservationID string, throughSequence int64) (int64, error) {
	var participant bool
	if err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.reserva_ensayo_local
WHERE id=$1 AND (anfitrion_id=$2 OR arrendatario_id=$2))`, reservationID, actor).Scan(&participant); err != nil {
		return 0, err
	}
	if !participant {
		return 0, conversation.ErrNotFound
	}
	var cursor int64
	err := r.pool.QueryRow(ctx, `INSERT INTO public.reserva_mensaje_lectura(reserva_id,participante_id,ultima_secuencia_leida,actualizada_en)
SELECT $1,$2,$3,clock_timestamp()
WHERE EXISTS(SELECT 1 FROM public.mensaje_reserva_ensayo WHERE reserva_id=$1 AND secuencia=$3)
ON CONFLICT (reserva_id,participante_id) DO UPDATE SET
ultima_secuencia_leida=GREATEST(reserva_mensaje_lectura.ultima_secuencia_leida,EXCLUDED.ultima_secuencia_leida),
actualizada_en=CASE WHEN EXCLUDED.ultima_secuencia_leida>reserva_mensaje_lectura.ultima_secuencia_leida THEN EXCLUDED.actualizada_en ELSE reserva_mensaje_lectura.actualizada_en END
RETURNING ultima_secuencia_leida`, reservationID, actor, throughSequence).Scan(&cursor)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, conversation.ErrInvalid
	}
	return cursor, err
}

func (r *Repository) Send(ctx context.Context, actor, reservationID, key, body string, fingerprint []byte, id string, now func() time.Time) (conversation.Message, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return conversation.Message{}, err
	}
	defer tx.Rollback(ctx)
	var state string
	err = tx.QueryRow(ctx, `SELECT estado FROM public.reserva_ensayo_local
WHERE id=$1 AND (anfitrion_id=$2 OR arrendatario_id=$2) FOR UPDATE`, reservationID, actor).Scan(&state)
	if errors.Is(err, pgx.ErrNoRows) {
		return conversation.Message{}, conversation.ErrNotFound
	}
	if err != nil {
		return conversation.Message{}, err
	}
	var prior conversation.Message
	var priorHash []byte
	err = tx.QueryRow(ctx, `SELECT `+messageColumns+`,huella_solicitud FROM public.mensaje_reserva_ensayo
WHERE reserva_id=$1 AND autor_id=$2 AND clave_idempotencia=$3`, reservationID, actor, key).Scan(&prior.ID, &prior.ReservationID, &prior.AuthorID, &prior.Sequence, &prior.Body, &prior.CreatedAt, &priorHash)
	if err == nil {
		if !equalHash(priorHash, fingerprint) {
			return conversation.Message{}, conversation.ErrConflict
		}
		if err = tx.Commit(ctx); err != nil {
			return conversation.Message{}, err
		}
		return prior, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return conversation.Message{}, err
	}
	// Sample the injected backend clock only after acquiring the reservation
	// lock and resolving idempotency. A send that waited on this row must be
	// judged at the time it actually obtains the lock, not when it began.
	createdAt := now().UTC()
	expired, err := expiry.LockedReservation(ctx, tx, reservationID, createdAt)
	if err != nil {
		return conversation.Message{}, err
	}
	if expired {
		if err = tx.Commit(ctx); err != nil {
			return conversation.Message{}, err
		}
		return conversation.Message{}, conversation.ErrConflict
	}
	if state != "pendiente_de_pago" && state != "pagada" && state != "aprobada_host" {
		return conversation.Message{}, conversation.ErrConflict
	}
	item, err := scanMessage(tx.QueryRow(ctx, `INSERT INTO public.mensaje_reserva_ensayo
(id,reserva_id,autor_id,clave_idempotencia,huella_solicitud,cuerpo,creada_en)
	VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING `+messageColumns, id, reservationID, actor, key, fingerprint, body, createdAt))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return conversation.Message{}, conversation.ErrConflict
		}
		return conversation.Message{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return conversation.Message{}, err
	}
	return item, nil
}

func equalHash(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var diff byte
	for i := range a {
		diff |= a[i] ^ b[i]
	}
	return diff == 0
}
