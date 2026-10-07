package bookingpg

import (
	"bytes"
	"context"
	"errors"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/booking/expiry"
	"github.com/HernanEspinozaDev/espaciGo/internal/booking"
	"github.com/jackc/pgx/v5"
)

var _ booking.PaymentLifecycleRepository = (*Repository)(nil)

func (r *Repository) BeginPayment(ctx context.Context, renter, reservationID, requested, key string, fingerprint []byte, operationID string, clock func() time.Time) (booking.PaymentOperation, bool, error) {
	if len(fingerprint) != 32 {
		return booking.PaymentOperation{}, false, booking.ErrInvalid
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return booking.PaymentOperation{}, false, err
	}
	defer tx.Rollback(ctx)
	reservation, err := scanReservation(tx.QueryRow(ctx, `SELECT `+reservationCols+` FROM public.reserva_ensayo_local WHERE id=$1 AND arrendatario_id=$2 FOR UPDATE`, reservationID, renter))
	if err != nil {
		return booking.PaymentOperation{}, false, err
	}
	now := clock().UTC()
	expired, err := expiry.LockedReservation(ctx, tx, reservationID, now)
	if err != nil {
		return booking.PaymentOperation{}, false, err
	}
	var operation booking.PaymentOperation
	var existingFingerprint []byte
	err = tx.QueryRow(ctx, `SELECT id::text,reserva_id::text,arrendatario_id::text,clave_idempotencia,huella_solicitud,resultado_solicitado,estado,creada_en
FROM public.reserva_pago_ensayo_operacion WHERE reserva_id=$1 AND arrendatario_id=$2 AND clave_idempotencia=$3 FOR UPDATE`, reservationID, renter, key).Scan(
		&operation.ID, &operation.ReservationID, &operation.RenterID, &operation.IdempotencyKey, &existingFingerprint, &operation.Requested, &operation.State, &operation.CreatedAt)
	if err == nil {
		if !bytes.Equal(existingFingerprint, fingerprint) || operation.Requested != requested {
			return booking.PaymentOperation{}, false, booking.ErrConflict
		}
		if expired && operation.State == "pendiente" {
			if _, err = tx.Exec(ctx, `UPDATE public.reserva_pago_ensayo_operacion SET estado='vencida',actualizada_en=$2 WHERE id=$1`, operation.ID, now); err != nil {
				return booking.PaymentOperation{}, false, err
			}
			operation.State = "vencida"
		}
		if err = tx.Commit(ctx); err != nil {
			return booking.PaymentOperation{}, false, err
		}
		return operation, false, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return booking.PaymentOperation{}, false, err
	}
	if expired || reservation.State != "pendiente_de_pago" || !reservation.PayExpiresAt.After(now) {
		if err = tx.Commit(ctx); err != nil {
			return booking.PaymentOperation{}, false, err
		}
		return booking.PaymentOperation{}, false, booking.ErrConflict
	}
	var pending bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.reserva_pago_ensayo_operacion WHERE reserva_id=$1 AND estado='pendiente')`, reservationID).Scan(&pending); err != nil {
		return booking.PaymentOperation{}, false, err
	}
	if pending {
		return booking.PaymentOperation{}, false, booking.ErrConflict
	}
	_, err = tx.Exec(ctx, `INSERT INTO public.reserva_pago_ensayo_operacion(id,reserva_id,arrendatario_id,clave_idempotencia,huella_solicitud,resultado_solicitado,estado,creada_en,actualizada_en)
VALUES($1,$2,$3,$4,$5,$6,'pendiente',$7,$7)`, operationID, reservationID, renter, key, fingerprint, requested, now)
	if err != nil {
		return booking.PaymentOperation{}, false, mapErr(err)
	}
	if err = tx.Commit(ctx); err != nil {
		return booking.PaymentOperation{}, false, mapErr(err)
	}
	return booking.PaymentOperation{ID: operationID, ReservationID: reservationID, RenterID: renter, IdempotencyKey: key, Fingerprint: append([]byte(nil), fingerprint...), Requested: requested, State: "pendiente", CreatedAt: now}, true, nil
}

func (r *Repository) PendingPayments(ctx context.Context, limit int) ([]booking.PaymentOperation, error) {
	if limit < 1 || limit > 500 {
		return nil, booking.ErrInvalid
	}
	rows, err := r.pool.Query(ctx, `SELECT id::text,reserva_id::text,arrendatario_id::text,clave_idempotencia,huella_solicitud,resultado_solicitado,estado,creada_en
FROM public.reserva_pago_ensayo_operacion WHERE estado='pendiente' ORDER BY creada_en,id LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := make([]booking.PaymentOperation, 0)
	for rows.Next() {
		var value booking.PaymentOperation
		if err := rows.Scan(&value.ID, &value.ReservationID, &value.RenterID, &value.IdempotencyKey, &value.Fingerprint, &value.Requested, &value.State, &value.CreatedAt); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (r *Repository) PendingPaymentEventIDs(ctx context.Context, limit int) ([]string, error) {
	if limit < 1 || limit > 500 {
		return nil, booking.ErrInvalid
	}
	rows, err := r.pool.Query(ctx, `SELECT e.id::text FROM public.reserva_pago_evento_aplicacion_ensayo a
JOIN public.reserva_pago_evento_ensayo e ON e.id=a.evento_id
WHERE a.estado='pendiente' ORDER BY a.creada_en,e.id LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *Repository) RecordPaymentEvent(ctx context.Context, event booking.PaymentEvent, fingerprint []byte, now time.Time) (bool, error) {
	if len(fingerprint) != 32 || event.EventID == "" || event.OperationID == "" || (event.Outcome != "exito_simulado" && event.Outcome != "rechazo_simulado") {
		return false, booking.ErrInvalid
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	var operationState string
	err = tx.QueryRow(ctx, `SELECT estado FROM public.reserva_pago_ensayo_operacion WHERE id=$1 FOR UPDATE`, event.OperationID).Scan(&operationState)
	if err != nil {
		return false, mapErr(err)
	}
	var oldOperationID, oldOutcome string
	var oldFingerprint []byte
	err = tx.QueryRow(ctx, `SELECT operacion_id::text,resultado,huella_payload FROM public.reserva_pago_evento_ensayo WHERE proveedor_evento_id=$1`, event.EventID).Scan(&oldOperationID, &oldOutcome, &oldFingerprint)
	if err == nil {
		if oldOperationID != event.OperationID || oldOutcome != event.Outcome || !bytes.Equal(oldFingerprint, fingerprint) {
			return false, booking.ErrConflict
		}
		if err = tx.Commit(ctx); err != nil {
			return false, err
		}
		return true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return false, err
	}
	if operationState != "pendiente" {
		return false, booking.ErrConflict
	}
	var operationRequested string
	if err = tx.QueryRow(ctx, `SELECT resultado_solicitado FROM public.reserva_pago_ensayo_operacion WHERE id=$1`, event.OperationID).Scan(&operationRequested); err != nil {
		return false, err
	}
	// A fake timeout represents a lost response, so a later authenticated
	// callback may resolve it either way. Other fake outcomes must match their
	// requested result to prevent a mismatched event from settling the intent.
	if operationRequested == "exito" && event.Outcome != "exito_simulado" || operationRequested == "rechazo" && event.Outcome != "rechazo_simulado" {
		return false, booking.ErrConflict
	}
	var eventRowID string
	err = tx.QueryRow(ctx, `INSERT INTO public.reserva_pago_evento_ensayo(id,operacion_id,proveedor_evento_id,resultado,huella_payload,autenticado_en,recibido_en)
VALUES(gen_random_uuid(),$1,$2,$3,$4,$5,$5) RETURNING id::text`, event.OperationID, event.EventID, event.Outcome, fingerprint, now.UTC()).Scan(&eventRowID)
	if err != nil {
		return false, mapErr(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO public.reserva_pago_evento_aplicacion_ensayo(evento_id,estado,creada_en) VALUES($1,'pendiente',$2)`, eventRowID, now.UTC()); err != nil {
		return false, err
	}
	if err = tx.Commit(ctx); err != nil {
		return false, err
	}
	return false, nil
}

func (r *Repository) ApplyPaymentEvent(ctx context.Context, eventRowID string, clock func() time.Time, hostTTL time.Duration) (booking.Reservation, error) {
	var operationID, reservationID string
	err := r.pool.QueryRow(ctx, `SELECT e.operacion_id::text,o.reserva_id::text FROM public.reserva_pago_evento_ensayo e
JOIN public.reserva_pago_ensayo_operacion o ON o.id=e.operacion_id WHERE e.id=$1`, eventRowID).Scan(&operationID, &reservationID)
	if err != nil {
		return booking.Reservation{}, mapErr(err)
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return booking.Reservation{}, err
	}
	defer tx.Rollback(ctx)
	value, err := scanReservation(tx.QueryRow(ctx, `SELECT `+reservationCols+` FROM public.reserva_ensayo_local WHERE id=$1 FOR UPDATE`, reservationID))
	if err != nil {
		return booking.Reservation{}, err
	}
	var operationState string
	err = tx.QueryRow(ctx, `SELECT estado FROM public.reserva_pago_ensayo_operacion WHERE id=$1 FOR UPDATE`, operationID).Scan(&operationState)
	if err != nil {
		return booking.Reservation{}, err
	}
	var outcome, applicationState string
	err = tx.QueryRow(ctx, `SELECT e.resultado,a.estado FROM public.reserva_pago_evento_ensayo e
JOIN public.reserva_pago_evento_aplicacion_ensayo a ON a.evento_id=e.id WHERE e.id=$1 FOR UPDATE OF a`, eventRowID).Scan(&outcome, &applicationState)
	if err != nil {
		return booking.Reservation{}, err
	}
	if applicationState != "pendiente" {
		if err = tx.Commit(ctx); err != nil {
			return booking.Reservation{}, err
		}
		return value, nil
	}
	if operationState != "pendiente" || value.State != "pendiente_de_pago" {
		now := clock().UTC()
		if _, err = tx.Exec(ctx, `UPDATE public.reserva_pago_evento_aplicacion_ensayo SET estado='ignorada',codigo_resultado='operacion_terminal',procesado_en=$2 WHERE evento_id=$1`, eventRowID, now); err != nil {
			return booking.Reservation{}, err
		}
		if operationState == "pendiente" {
			if _, err = tx.Exec(ctx, `UPDATE public.reserva_pago_ensayo_operacion SET estado='vencida',actualizada_en=$2 WHERE id=$1`, operationID, now); err != nil {
				return booking.Reservation{}, err
			}
		}
		if err = tx.Commit(ctx); err != nil {
			return booking.Reservation{}, err
		}
		return value, booking.ErrConflict
	}
	now := clock().UTC()
	expired, err := expiry.LockedReservation(ctx, tx, reservationID, now)
	if err != nil {
		return booking.Reservation{}, err
	}
	if expired || !value.PayExpiresAt.After(now) {
		if _, err = tx.Exec(ctx, `UPDATE public.reserva_pago_ensayo_operacion SET estado='vencida',actualizada_en=$2 WHERE id=$1`, operationID, now); err != nil {
			return booking.Reservation{}, err
		}
		if _, err = tx.Exec(ctx, `UPDATE public.reserva_pago_evento_aplicacion_ensayo SET estado='vencida',codigo_resultado='reserva_vencida',procesado_en=$2 WHERE evento_id=$1`, eventRowID, now); err != nil {
			return booking.Reservation{}, err
		}
		if err = tx.Commit(ctx); err != nil {
			return booking.Reservation{}, err
		}
		return value, booking.ErrConflict
	}
	finalState, paymentResult, reason := "pagada", outcome, "pago fake confirmado desde evento autenticado"
	if outcome == "rechazo_simulado" {
		finalState, reason = "cancelada_por_pago", "rechazo fake confirmado desde evento autenticado"
		_, err = tx.Exec(ctx, `UPDATE public.ocupacion SET activo=false,desactivada_en=$2 WHERE reserva_id=$1 AND activo`, reservationID, now)
	} else {
		value.HostExpiresAt = ptr(now.Add(hostTTL))
		_, err = tx.Exec(ctx, `UPDATE public.ocupacion SET tipo='reserva',expira_en=$2 WHERE reserva_id=$1 AND activo`, reservationID, *value.HostExpiresAt)
	}
	if err != nil {
		return booking.Reservation{}, err
	}
	amount := int64(0)
	if outcome == "exito_simulado" {
		amount = value.Subtotal
	}
	_, err = tx.Exec(ctx, `INSERT INTO public.reserva_pago_ensayo(id,reserva_id,resultado,clave_idempotencia,creada_en,importe_clp)
VALUES(gen_random_uuid(),$1,$2,$3,$4,$5)`, reservationID, paymentResult, "payment-operation:"+operationID, now, amount)
	if err != nil {
		return booking.Reservation{}, mapErr(err)
	}
	if _, err = tx.Exec(ctx, `UPDATE public.reserva_ensayo_local SET estado=$2,anfitrion_vence_en=$3,actualizada_en=$4 WHERE id=$1`, reservationID, finalState, value.HostExpiresAt, now); err != nil {
		return booking.Reservation{}, err
	}
	if err = appendTransition(ctx, tx, reservationID, ptrString("pendiente_de_pago"), finalState, value.RenterID, reason, now); err != nil {
		return booking.Reservation{}, err
	}
	if _, err = tx.Exec(ctx, `UPDATE public.reserva_pago_ensayo_operacion SET estado='aplicada',resultado_final=$2,actualizada_en=$3 WHERE id=$1`, operationID, outcome, now); err != nil {
		return booking.Reservation{}, err
	}
	if _, err = tx.Exec(ctx, `UPDATE public.reserva_pago_evento_aplicacion_ensayo SET estado='aplicada',codigo_resultado='aplicado',procesado_en=$2 WHERE evento_id=$1`, eventRowID, now); err != nil {
		return booking.Reservation{}, err
	}
	value.State, value.UpdatedAt = finalState, now
	if err = tx.Commit(ctx); err != nil {
		return booking.Reservation{}, mapErr(err)
	}
	return value, nil
}
