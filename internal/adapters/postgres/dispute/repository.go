package dispute

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/credentials"
	domain "github.com/HernanEspinozaDev/espaciGo/internal/dispute"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

var _ domain.Repository = (*Repository)(nil)

type scanner interface{ Scan(...any) error }

func scanDispute(row scanner) (domain.Dispute, error) {
	var item domain.Dispute
	err := row.Scan(&item.ID, &item.ReservationID, &item.HostID, &item.RenterID, &item.OpenedBy,
		&item.OpeningReason, &item.State, &item.OpenedAt, &item.ClosedBy, &item.CloseReason, &item.ClosedAt)
	item.OpenedAt = item.OpenedAt.UTC()
	if item.ClosedAt != nil {
		value := item.ClosedAt.UTC()
		item.ClosedAt = &value
	}
	return item, mapError(err)
}

const disputeColumns = `id::text,reserva_id::text,anfitrion_id::text,arrendatario_id::text,abierta_por::text,motivo_codigo,estado,abierta_en,cerrada_por::text,motivo_cierre_codigo,cerrada_en`

func (r *Repository) Open(ctx context.Context, actorID, reservationID, reasonCode, idempotencyKey string, fingerprint []byte, clock func() time.Time) (domain.Dispute, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Dispute{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	var hostID, renterID string
	err = tx.QueryRow(ctx, `SELECT anfitrion_id::text,arrendatario_id::text FROM public.reserva_ensayo_local WHERE id=$1`, reservationID).Scan(&hostID, &renterID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && actorID != hostID) {
		return domain.Dispute{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Dispute{}, mapError(err)
	}
	if err := lockAccounts(ctx, tx, hostID, renterID); err != nil {
		return domain.Dispute{}, err
	}
	var currentHost, currentRenter string
	err = tx.QueryRow(ctx, `SELECT anfitrion_id::text,arrendatario_id::text FROM public.reserva_ensayo_local WHERE id=$1 FOR SHARE`, reservationID).Scan(&currentHost, &currentRenter)
	if errors.Is(err, pgx.ErrNoRows) || err == nil && (currentHost != hostID || currentRenter != renterID || currentHost != actorID) {
		return domain.Dispute{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Dispute{}, mapError(err)
	}

	var existing domain.Dispute
	var existingFingerprint []byte
	err = tx.QueryRow(ctx, `SELECT `+disputeColumns+`,huella_solicitud FROM public.disputa_ensayo_local WHERE anfitrion_id=$1 AND clave_idempotencia=$2 FOR UPDATE`, actorID, idempotencyKey).Scan(
		&existing.ID, &existing.ReservationID, &existing.HostID, &existing.RenterID, &existing.OpenedBy,
		&existing.OpeningReason, &existing.State, &existing.OpenedAt, &existing.ClosedBy, &existing.CloseReason, &existing.ClosedAt, &existingFingerprint)
	if err == nil {
		if existing.ReservationID != reservationID || !equalBytes(existingFingerprint, fingerprint) {
			return domain.Dispute{}, domain.ErrConflict
		}
		existing.OpenedAt = existing.OpenedAt.UTC()
		if existing.ClosedAt != nil {
			value := existing.ClosedAt.UTC()
			existing.ClosedAt = &value
		}
		existing.Reused = true
		if err := tx.Commit(ctx); err != nil {
			return domain.Dispute{}, mapError(err)
		}
		return existing, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return domain.Dispute{}, mapError(err)
	}
	_ = existing

	var openID string
	err = tx.QueryRow(ctx, `SELECT id::text FROM public.disputa_ensayo_local WHERE reserva_id=$1 AND estado='abierta' FOR UPDATE`, reservationID).Scan(&openID)
	if err == nil {
		return domain.Dispute{}, domain.ErrConflict
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return domain.Dispute{}, mapError(err)
	}
	id, err := (credentials.Generator{}).ID()
	if err != nil {
		return domain.Dispute{}, err
	}
	openedAt := clock().UTC().Truncate(time.Microsecond)
	_, err = tx.Exec(ctx, `INSERT INTO public.disputa_ensayo_local (
		id,reserva_id,anfitrion_id,arrendatario_id,abierta_por,motivo_codigo,estado,
		clave_idempotencia,huella_solicitud,abierta_en)
		VALUES ($1,$2,$3,$4,$3,$5,'abierta',$6,$7,$8)`, id, reservationID, hostID, renterID, reasonCode, idempotencyKey, fingerprint, openedAt)
	if err != nil {
		return domain.Dispute{}, mapError(err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO public.disputa_ensayo_historial(disputa_id,estado_anterior,estado_nuevo,actor_id,motivo_codigo,ocurrida_en)
		VALUES ($1,NULL,'abierta',$2,$3,$4)`, id, actorID, reasonCode, openedAt); err != nil {
		return domain.Dispute{}, mapError(err)
	}
	item, err := scanDispute(tx.QueryRow(ctx, `SELECT `+disputeColumns+` FROM public.disputa_ensayo_local WHERE id=$1`, id))
	if err != nil {
		return domain.Dispute{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Dispute{}, mapError(err)
	}
	return item, nil
}

func (r *Repository) ListForParticipant(ctx context.Context, actorID, reservationID string) ([]domain.Dispute, error) {
	var participant bool
	if err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.reserva_ensayo_local WHERE id=$1 AND (anfitrion_id=$2 OR arrendatario_id=$2))`, reservationID, actorID).Scan(&participant); err != nil {
		return nil, mapError(err)
	}
	if !participant {
		return nil, domain.ErrNotFound
	}
	rows, err := r.pool.Query(ctx, `SELECT `+disputeColumns+` FROM public.disputa_ensayo_local WHERE reserva_id=$1 AND $2 IN (anfitrion_id::text,arrendatario_id::text) ORDER BY abierta_en,id`, reservationID, actorID)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	items := make([]domain.Dispute, 0)
	for rows.Next() {
		item, err := scanDispute(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, mapError(rows.Err())
}

func (r *Repository) ListOpen(ctx context.Context) ([]domain.Dispute, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+disputeColumns+` FROM public.disputa_ensayo_local WHERE estado='abierta' ORDER BY abierta_en,id`)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	items := make([]domain.Dispute, 0)
	for rows.Next() {
		item, err := scanDispute(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, mapError(rows.Err())
}

func (r *Repository) History(ctx context.Context, actorID, disputeID string) ([]domain.Transition, error) {
	rows, err := r.pool.Query(ctx, `SELECT h.secuencia,h.estado_anterior,h.estado_nuevo,h.actor_id::text,h.motivo_codigo,h.ocurrida_en
		FROM public.disputa_ensayo_local d JOIN public.disputa_ensayo_historial h ON h.disputa_id=d.id
		WHERE d.id=$1 AND $2 IN (d.anfitrion_id::text,d.arrendatario_id::text) ORDER BY h.secuencia`, disputeID, actorID)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	items := make([]domain.Transition, 0)
	for rows.Next() {
		var item domain.Transition
		if err := rows.Scan(&item.Sequence, &item.PreviousState, &item.NewState, &item.ActorID, &item.ReasonCode, &item.OccurredAt); err != nil {
			return nil, mapError(err)
		}
		item.OccurredAt = item.OccurredAt.UTC()
		items = append(items, item)
	}
	if err := mapError(rows.Err()); err != nil {
		return nil, err
	}
	if len(items) == 0 {
		var allowed bool
		if err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.disputa_ensayo_local WHERE id=$1 AND (anfitrion_id=$2 OR arrendatario_id=$2))`, disputeID, actorID).Scan(&allowed); err != nil {
			return nil, mapError(err)
		}
		if !allowed {
			return nil, domain.ErrNotFound
		}
	}
	return items, nil
}

func (r *Repository) Close(ctx context.Context, administratorID, disputeID, reasonCode string, clock func() time.Time) (domain.Dispute, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Dispute{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var hostID, renterID string
	err = tx.QueryRow(ctx, `SELECT anfitrion_id::text,arrendatario_id::text FROM public.disputa_ensayo_local WHERE id=$1`, disputeID).Scan(&hostID, &renterID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Dispute{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Dispute{}, mapError(err)
	}
	if err := lockAccounts(ctx, tx, hostID, renterID); err != nil {
		return domain.Dispute{}, err
	}
	var item domain.Dispute
	item, err = scanDispute(tx.QueryRow(ctx, `SELECT `+disputeColumns+` FROM public.disputa_ensayo_local WHERE id=$1 FOR UPDATE`, disputeID))
	if err != nil {
		return domain.Dispute{}, err
	}
	if item.State != "abierta" {
		return domain.Dispute{}, domain.ErrConflict
	}
	closedAt := clock().UTC().Truncate(time.Microsecond)
	_, err = tx.Exec(ctx, `UPDATE public.disputa_ensayo_local SET estado='cerrada',cerrada_por=$2,motivo_cierre_codigo=$3,cerrada_en=$4 WHERE id=$1 AND estado='abierta'`, disputeID, administratorID, reasonCode, closedAt)
	if err != nil {
		return domain.Dispute{}, mapError(err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO public.disputa_ensayo_historial(disputa_id,estado_anterior,estado_nuevo,actor_id,motivo_codigo,ocurrida_en)
		VALUES ($1,'abierta','cerrada',$2,$3,$4)`, disputeID, administratorID, reasonCode, closedAt); err != nil {
		return domain.Dispute{}, mapError(err)
	}
	item, err = scanDispute(tx.QueryRow(ctx, `SELECT `+disputeColumns+` FROM public.disputa_ensayo_local WHERE id=$1`, disputeID))
	if err != nil {
		return domain.Dispute{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Dispute{}, mapError(err)
	}
	return item, nil
}

func lockAccounts(ctx context.Context, tx pgx.Tx, accountIDs ...string) error {
	ids := append([]string(nil), accountIDs...)
	sort.Strings(ids)
	rows, err := tx.Query(ctx, `SELECT estado FROM public.usuario WHERE id::text=ANY($1::text[]) ORDER BY id::text FOR UPDATE`, ids)
	if err != nil {
		return mapError(err)
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var state string
		if err := rows.Scan(&state); err != nil {
			return mapError(err)
		}
		if state != "activo" {
			return domain.ErrConflict
		}
		count++
	}
	if err := rows.Err(); err != nil {
		return mapError(err)
	}
	if count != len(ids) {
		return domain.ErrNotFound
	}
	return nil
}

func mapError(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505", "23P01", "40001", "40P01":
			return domain.ErrConflict
		case "23503":
			return domain.ErrNotFound
		case "23514", "22P02", "22001":
			return domain.ErrInvalid
		}
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	return err
}

func equalBytes(left, right []byte) bool {
	if len(left) != len(right) {
		return false
	}
	var diff byte
	for index := range left {
		diff |= left[index] ^ right[index]
	}
	return diff == 0
}
