package damageclaimpg

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"errors"
	"sort"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/damageclaim"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func (r *Repository) Open(ctx context.Context, actor, reservation, key string, input damageclaim.Input, fingerprint []byte, clock func() time.Time) (damageclaim.Claim, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return damageclaim.Claim{}, mapError(err)
	}
	defer tx.Rollback(context.Background())
	var host, renter string
	if err = tx.QueryRow(ctx, `SELECT anfitrion_id::text,arrendatario_id::text FROM public.reserva_ensayo_local WHERE id=$1`, reservation).Scan(&host, &renter); err != nil {
		return damageclaim.Claim{}, mapError(err)
	}
	if actor != host {
		return damageclaim.Claim{}, damageclaim.ErrNotFound
	}
	if err = lockAccounts(ctx, tx, host, renter); err != nil {
		return damageclaim.Claim{}, mapError(err)
	}
	var state string
	if err = tx.QueryRow(ctx, `SELECT estado FROM public.reserva_ensayo_local WHERE id=$1 FOR UPDATE`, reservation).Scan(&state); err != nil {
		return damageclaim.Claim{}, mapError(err)
	}
	now := clock().UTC().Truncate(time.Microsecond)
	prior, priorKey, priorFingerprint, priorErr := scanIdempotency(tx.QueryRow(ctx, `SELECT id::text,anfitrion_id::text,clave_idempotencia,huella_solicitud FROM public.reclamo_dano_ensayo_local WHERE reserva_id=$1`, reservation))
	if priorErr == nil {
		if prior.ActorID != actor || priorKey != key || subtle.ConstantTimeCompare(priorFingerprint, fingerprint) != 1 {
			return damageclaim.Claim{}, damageclaim.ErrConflict
		}
		if err = tx.Commit(ctx); err != nil {
			return damageclaim.Claim{}, mapError(err)
		}
		claim, loadErr := r.Get(ctx, actor, reservation)
		claim.Reused = loadErr == nil
		return claim, loadErr
	}
	if !errors.Is(priorErr, pgx.ErrNoRows) {
		return damageclaim.Claim{}, mapError(priorErr)
	}
	if state != "finalizada" {
		return damageclaim.Claim{}, damageclaim.ErrConflict
	}
	var checkoutID, evidenceID string
	var checkedOut time.Time
	err = tx.QueryRow(ctx, `SELECT o.id::text,e.id::text,o.ocurrio_en FROM public.operacion_arriendo_ensayo_local o JOIN public.operacion_arriendo_evidencia_ensayo_local e ON e.operacion_id=o.id WHERE o.reserva_id=$1 AND o.tipo='checkout' ORDER BY e.creada_en,e.id LIMIT 1`, reservation).Scan(&checkoutID, &evidenceID, &checkedOut)
	if err != nil {
		return damageclaim.Claim{}, damageclaim.ErrConflict
	}
	deadline := checkedOut.UTC().Add(24 * time.Hour)
	if !now.Before(deadline) {
		return damageclaim.Claim{}, damageclaim.ErrConflict
	}
	var claimID string
	err = tx.QueryRow(ctx, `INSERT INTO public.reclamo_dano_ensayo_local(id,reserva_id,anfitrion_id,arrendatario_id,checkout_operacion_id,checkout_evidencia_id,descripcion,estado,clave_idempotencia,huella_solicitud,abierto_en,plazo_reclamo_hasta) VALUES(gen_random_uuid(),$1,$2,$3,$4,$5,$6,'abierto',$7,$8,$9,$10) RETURNING id::text`, reservation, host, renter, checkoutID, evidenceID, input.Description, key, fingerprint, now, deadline).Scan(&claimID)
	if err != nil {
		return damageclaim.Claim{}, mapError(err)
	}
	updated, err := tx.Exec(ctx, `UPDATE public.reserva_ensayo_local SET estado='en_disputa',actualizada_en=$2 WHERE id=$1 AND estado='finalizada'`, reservation, now)
	if err != nil {
		return damageclaim.Claim{}, mapError(err)
	}
	if updated.RowsAffected() != 1 {
		return damageclaim.Claim{}, damageclaim.ErrConflict
	}
	if _, err = tx.Exec(ctx, `INSERT INTO public.reserva_ensayo_transicion(id,reserva_id,secuencia,estado_anterior,estado_nuevo,actor_id,motivo,creada_en) SELECT gen_random_uuid(),$1,COALESCE(MAX(secuencia),0)+1,'finalizada','en_disputa',$2,'reclamo_dano_sintetico',$3 FROM public.reserva_ensayo_transicion WHERE reserva_id=$1`, reservation, actor, now); err != nil {
		return damageclaim.Claim{}, mapError(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO public.reclamo_dano_historial_ensayo_local(reclamo_id,accion,actor_id,ocurrida_en) VALUES($1,'reclamo_abierto',$2,$3)`, claimID, actor, now); err != nil {
		return damageclaim.Claim{}, mapError(err)
	}
	if err = tx.Commit(ctx); err != nil {
		return damageclaim.Claim{}, mapError(err)
	}
	return r.Get(ctx, actor, reservation)
}

type idempotencyRow struct{ ID, ActorID string }

func scanIdempotency(row pgx.Row) (idempotencyRow, string, []byte, error) {
	var result idempotencyRow
	var key string
	var fingerprint []byte
	err := row.Scan(&result.ID, &result.ActorID, &key, &fingerprint)
	if err != nil {
		return result, "", nil, err
	}
	return result, key, fingerprint, nil
}

func (r *Repository) Get(ctx context.Context, actor, reservation string) (damageclaim.Claim, error) {
	var claim damageclaim.Claim
	err := r.pool.QueryRow(ctx, `SELECT c.id::text,c.reserva_id::text,c.anfitrion_id::text,c.arrendatario_id::text,c.checkout_operacion_id::text,c.checkout_evidencia_id::text,c.descripcion,c.estado,c.abierto_en,c.plazo_reclamo_hasta FROM public.reclamo_dano_ensayo_local c WHERE c.reserva_id=$1 AND $2 IN(c.anfitrion_id::text,c.arrendatario_id::text)`, reservation, actor).Scan(&claim.ID, &claim.ReservationID, &claim.HostID, &claim.RenterID, &claim.CheckoutOperationID, &claim.CheckoutEvidenceID, &claim.Description, &claim.State, &claim.OpenedAt, &claim.ClaimDeadlineAt)
	if err != nil {
		return damageclaim.Claim{}, mapError(err)
	}
	claim.OpenedAt = claim.OpenedAt.UTC()
	claim.ClaimDeadlineAt = claim.ClaimDeadlineAt.UTC()
	var defense damageclaim.Defense
	err = r.pool.QueryRow(ctx, `SELECT id::text,actor_id::text,descripcion,creado_en FROM public.reclamo_dano_descargo_ensayo_local WHERE reclamo_id=$1`, claim.ID).Scan(&defense.ID, &defense.ActorID, &defense.Description, &defense.CreatedAt)
	if err == nil {
		defense.CreatedAt = defense.CreatedAt.UTC()
		claim.Defense = &defense
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return damageclaim.Claim{}, mapError(err)
	}
	return claim, nil
}

func (r *Repository) Defend(ctx context.Context, actor, reservation, key string, input damageclaim.Input, fingerprint []byte, clock func() time.Time) (damageclaim.Defense, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return damageclaim.Defense{}, mapError(err)
	}
	defer tx.Rollback(context.Background())
	var host, renter string
	if err = tx.QueryRow(ctx, `SELECT anfitrion_id::text,arrendatario_id::text FROM public.reserva_ensayo_local WHERE id=$1`, reservation).Scan(&host, &renter); err != nil {
		return damageclaim.Defense{}, mapError(err)
	}
	if err = lockAccounts(ctx, tx, host, renter); err != nil {
		return damageclaim.Defense{}, mapError(err)
	}
	var reservationState string
	if err = tx.QueryRow(ctx, `SELECT estado FROM public.reserva_ensayo_local WHERE id=$1 FOR UPDATE`, reservation).Scan(&reservationState); err != nil {
		return damageclaim.Defense{}, mapError(err)
	}
	var claimID, claimState string
	err = tx.QueryRow(ctx, `SELECT id::text,estado FROM public.reclamo_dano_ensayo_local WHERE reserva_id=$1`, reservation).Scan(&claimID, &claimState)
	if err != nil {
		return damageclaim.Defense{}, mapError(err)
	}
	if actor != renter {
		return damageclaim.Defense{}, damageclaim.ErrNotFound
	}
	if reservationState != "en_disputa" || claimState != "abierto" {
		return damageclaim.Defense{}, damageclaim.ErrConflict
	}
	now := clock().UTC().Truncate(time.Microsecond)
	var prior damageclaim.Defense
	var priorKey string
	var priorFingerprint []byte
	err = tx.QueryRow(ctx, `SELECT id::text,actor_id::text,descripcion,creado_en,clave_idempotencia,huella_solicitud FROM public.reclamo_dano_descargo_ensayo_local WHERE reclamo_id=$1`, claimID).Scan(&prior.ID, &prior.ActorID, &prior.Description, &prior.CreatedAt, &priorKey, &priorFingerprint)
	if err == nil {
		if prior.ActorID != actor || priorKey != key || subtle.ConstantTimeCompare(priorFingerprint, fingerprint) != 1 {
			return damageclaim.Defense{}, damageclaim.ErrConflict
		}
		prior.CreatedAt = prior.CreatedAt.UTC()
		prior.Reused = true
		if err = tx.Commit(ctx); err != nil {
			return damageclaim.Defense{}, mapError(err)
		}
		return prior, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return damageclaim.Defense{}, mapError(err)
	}
	if err = tx.QueryRow(ctx, `INSERT INTO public.reclamo_dano_descargo_ensayo_local(id,reclamo_id,actor_id,descripcion,clave_idempotencia,huella_solicitud,creado_en) VALUES(gen_random_uuid(),$1,$2,$3,$4,$5,$6) RETURNING id::text,actor_id::text,descripcion,creado_en`, claimID, actor, input.Description, key, fingerprint, now).Scan(&prior.ID, &prior.ActorID, &prior.Description, &prior.CreatedAt); err != nil {
		return damageclaim.Defense{}, mapError(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO public.reclamo_dano_historial_ensayo_local(reclamo_id,accion,actor_id,ocurrida_en) VALUES($1,'descargo_registrado',$2,$3)`, claimID, actor, now); err != nil {
		return damageclaim.Defense{}, mapError(err)
	}
	if err = tx.Commit(ctx); err != nil {
		return damageclaim.Defense{}, mapError(err)
	}
	prior.CreatedAt = prior.CreatedAt.UTC()
	return prior, nil
}

func lockAccounts(ctx context.Context, tx pgx.Tx, host, renter string) error {
	ids := []string{host, renter}
	sort.Strings(ids)
	for i, id := range ids {
		if i > 0 && id == ids[i-1] {
			continue
		}
		var state string
		if err := tx.QueryRow(ctx, `SELECT estado FROM public.usuario WHERE id=$1 FOR UPDATE`, id).Scan(&state); err != nil {
			return err
		}
		if state != "activo" {
			return damageclaim.ErrConflict
		}
	}
	return nil
}
func mapError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
		return damageclaim.ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505", "23P01", "40001", "40P01":
			return damageclaim.ErrConflict
		case "23503":
			return damageclaim.ErrNotFound
		case "23514", "22P02", "22001":
			return damageclaim.ErrInvalid
		}
	}
	return err
}

var _ damageclaim.Repository = (*Repository)(nil)
