package bookingpg

import (
	"bytes"
	"context"
	"errors"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/booking/expiry"
	"github.com/HernanEspinozaDev/espaciGo/internal/booking"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
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
	var operation booking.PaymentOperation
	var existingFingerprint []byte
	err = tx.QueryRow(ctx, `SELECT id::text,reserva_id::text,arrendatario_id::text,clave_idempotencia,huella_solicitud,resultado_solicitado,estado,creada_en
FROM public.reserva_pago_ensayo_operacion WHERE reserva_id=$1 AND arrendatario_id=$2 AND clave_idempotencia=$3 FOR UPDATE`, reservationID, renter, key).Scan(
		&operation.ID, &operation.ReservationID, &operation.RenterID, &operation.IdempotencyKey, &existingFingerprint, &operation.Requested, &operation.State, &operation.CreatedAt)
	if err == nil {
		operation.ReservationState = reservation.State
		operation.PayExpiresAt = reservation.PayExpiresAt
		if !bytes.Equal(existingFingerprint, fingerprint) || operation.Requested != requested {
			return booking.PaymentOperation{}, false, booking.ErrConflict
		}
		if err = tx.Commit(ctx); err != nil {
			return booking.PaymentOperation{}, false, err
		}
		return operation, false, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return booking.PaymentOperation{}, false, err
	}
	expired, err := expiry.LockedReservation(ctx, tx, reservationID, now)
	if err != nil {
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
	return booking.PaymentOperation{ID: operationID, ReservationID: reservationID, RenterID: renter, IdempotencyKey: key, Fingerprint: append([]byte(nil), fingerprint...), Requested: requested, State: "pendiente", CreatedAt: now, ReservationState: reservation.State, PayExpiresAt: reservation.PayExpiresAt}, true, nil
}

func (r *Repository) PendingPayments(ctx context.Context, limit int) ([]booking.PaymentOperation, error) {
	if limit < 1 || limit > 500 {
		return nil, booking.ErrInvalid
	}
	rows, err := r.pool.Query(ctx, `SELECT o.id::text,o.reserva_id::text,o.arrendatario_id::text,o.clave_idempotencia,o.huella_solicitud,o.resultado_solicitado,o.estado,o.creada_en,r.estado,r.pago_vence_en
FROM public.reserva_pago_ensayo_operacion o JOIN public.reserva_ensayo_local r ON r.id=o.reserva_id
LEFT JOIN public.reserva_pago_fake_resultado_ensayo f ON f.operacion_id=o.id AND f.estado='resultado'
WHERE o.estado='pendiente' OR (o.estado='vencida' AND f.operacion_id IS NOT NULL AND NOT EXISTS(
  SELECT 1 FROM public.reserva_pago_evento_ensayo e WHERE e.operacion_id=o.id AND e.proveedor_evento_id=f.proveedor_evento_id))
ORDER BY o.creada_en,o.id LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := make([]booking.PaymentOperation, 0)
	for rows.Next() {
		var value booking.PaymentOperation
		if err := rows.Scan(&value.ID, &value.ReservationID, &value.RenterID, &value.IdempotencyKey, &value.Fingerprint, &value.Requested, &value.State, &value.CreatedAt, &value.ReservationState, &value.PayExpiresAt); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

// SaveFakePaymentResult durably records the result produced by the local fake.
// The operation ID is the fake's idempotency key: repeated starts return the
// same immutable event rather than representing another charge.
func (r *Repository) SaveFakePaymentResult(ctx context.Context, event booking.PaymentEvent, recordedAt time.Time) (booking.PaymentEvent, bool, error) {
	if event.OperationID == "" || event.EventID == "" || (event.Outcome != "exito_simulado" && event.Outcome != "rechazo_simulado") {
		return booking.PaymentEvent{}, false, booking.ErrInvalid
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return booking.PaymentEvent{}, false, err
	}
	defer tx.Rollback(ctx)
	var requested string
	err = tx.QueryRow(ctx, `SELECT resultado_solicitado FROM public.reserva_pago_ensayo_operacion WHERE id=$1 FOR UPDATE`, event.OperationID).Scan(&requested)
	if err != nil {
		return booking.PaymentEvent{}, false, mapErr(err)
	}
	if requested == "exito" && event.Outcome != "exito_simulado" || requested == "rechazo" && event.Outcome != "rechazo_simulado" || requested == "sin_respuesta" && event.Outcome != "exito_simulado" {
		return booking.PaymentEvent{}, false, booking.ErrConflict
	}
	command, err := tx.Exec(ctx, `INSERT INTO public.reserva_pago_fake_resultado_ensayo(operacion_id,estado,proveedor_evento_id,resultado,registrado_en)
VALUES($1,'resultado',$2,$3,$4) ON CONFLICT(operacion_id) DO NOTHING`, event.OperationID, event.EventID, event.Outcome, recordedAt.UTC())
	if err != nil {
		return booking.PaymentEvent{}, false, mapErr(err)
	}
	var stored booking.PaymentEvent
	stored.OperationID = event.OperationID
	err = tx.QueryRow(ctx, `SELECT proveedor_evento_id,resultado FROM public.reserva_pago_fake_resultado_ensayo WHERE operacion_id=$1 AND estado='resultado'`, event.OperationID).Scan(&stored.EventID, &stored.Outcome)
	if err != nil {
		return booking.PaymentEvent{}, false, err
	}
	if stored.EventID != event.EventID || stored.Outcome != event.Outcome {
		return booking.PaymentEvent{}, false, booking.ErrConflict
	}
	if err = tx.Commit(ctx); err != nil {
		return booking.PaymentEvent{}, false, mapErr(err)
	}
	return stored, command.RowsAffected() == 1, nil
}

func (r *Repository) RecordFakePaymentTimeout(ctx context.Context, operationID string, recordedAt time.Time) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var requested string
	if err = tx.QueryRow(ctx, `SELECT resultado_solicitado FROM public.reserva_pago_ensayo_operacion WHERE id=$1 FOR UPDATE`, operationID).Scan(&requested); err != nil {
		return mapErr(err)
	}
	if requested != "sin_respuesta" {
		return booking.ErrConflict
	}
	_, err = tx.Exec(ctx, `INSERT INTO public.reserva_pago_fake_resultado_ensayo(operacion_id,estado,registrado_en)
VALUES($1,'sin_respuesta',$2) ON CONFLICT(operacion_id) DO NOTHING`, operationID, recordedAt.UTC())
	if err != nil {
		return mapErr(err)
	}
	var state string
	if err = tx.QueryRow(ctx, `SELECT estado FROM public.reserva_pago_fake_resultado_ensayo WHERE operacion_id=$1`, operationID).Scan(&state); err != nil {
		return err
	}
	if state != "sin_respuesta" {
		return booking.ErrConflict
	}
	return tx.Commit(ctx)
}

func (r *Repository) FindFakePaymentResult(ctx context.Context, operationID string) (*booking.PaymentEvent, error) {
	var event booking.PaymentEvent
	event.OperationID = operationID
	var state string
	var eventID, outcome pgtype.Text
	err := r.pool.QueryRow(ctx, `SELECT estado,proveedor_evento_id,resultado FROM public.reserva_pago_fake_resultado_ensayo WHERE operacion_id=$1`, operationID).Scan(&state, &eventID, &outcome)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if state == "sin_respuesta" {
		return nil, booking.ErrSimulatedNoResponse
	}
	if state != "resultado" || !eventID.Valid || !outcome.Valid {
		return nil, booking.ErrConflict
	}
	event.EventID, event.Outcome = eventID.String, outcome.String
	return &event, nil
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
	var operationRequested string
	if err = tx.QueryRow(ctx, `SELECT resultado_solicitado FROM public.reserva_pago_ensayo_operacion WHERE id=$1`, event.OperationID).Scan(&operationRequested); err != nil {
		return false, err
	}
	// A verified late callback is itself the fake's registered result. It may
	// resolve a persisted timeout marker, but cannot replace another result.
	_, err = tx.Exec(ctx, `INSERT INTO public.reserva_pago_fake_resultado_ensayo(operacion_id,estado,proveedor_evento_id,resultado,registrado_en)
VALUES($1,'resultado',$2,$3,$4)
ON CONFLICT(operacion_id) DO UPDATE SET estado='resultado',proveedor_evento_id=EXCLUDED.proveedor_evento_id,resultado=EXCLUDED.resultado,registrado_en=EXCLUDED.registrado_en
WHERE public.reserva_pago_fake_resultado_ensayo.estado='sin_respuesta'`, event.OperationID, event.EventID, event.Outcome, now.UTC())
	if err != nil {
		return false, mapErr(err)
	}
	var fakeEventID, fakeOutcome string
	if err = tx.QueryRow(ctx, `SELECT proveedor_evento_id,resultado FROM public.reserva_pago_fake_resultado_ensayo WHERE operacion_id=$1`, event.OperationID).Scan(&fakeEventID, &fakeOutcome); err != nil {
		return false, err
	}
	if fakeEventID != event.EventID || fakeOutcome != event.Outcome {
		return false, booking.ErrConflict
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
	if _, err = tx.Exec(ctx, `INSERT INTO public.reserva_pago_evento_aplicacion_ensayo(evento_id,estado,codigo_resultado,creada_en) VALUES($1,'pendiente',CASE WHEN $2='pendiente' THEN NULL ELSE 'operacion_terminal' END,$3)`, eventRowID, operationState, now.UTC()); err != nil {
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
	var authenticatedAt time.Time
	err = tx.QueryRow(ctx, `SELECT e.resultado,a.estado,e.autenticado_en FROM public.reserva_pago_evento_ensayo e
JOIN public.reserva_pago_evento_aplicacion_ensayo a ON a.evento_id=e.id WHERE e.id=$1 FOR UPDATE OF a`, eventRowID).Scan(&outcome, &applicationState, &authenticatedAt)
	if err != nil {
		return booking.Reservation{}, err
	}
	if applicationState == "aplicada" || applicationState == "ignorada" || applicationState == "vencida" {
		if err = tx.Commit(ctx); err != nil {
			return booking.Reservation{}, err
		}
		return value, nil
	}
	if operationState != "pendiente" || value.State != "pendiente_de_pago" {
		if _, err = tx.Exec(ctx, `UPDATE public.reserva_pago_evento_aplicacion_ensayo SET estado='pendiente_conciliacion',codigo_resultado='operacion_terminal',procesado_en=NULL WHERE evento_id=$1`, eventRowID); err != nil {
			return booking.Reservation{}, err
		}
		if err = tx.Commit(ctx); err != nil {
			return booking.Reservation{}, err
		}
		return value, booking.ErrConflict
	}
	// A callback authenticated before the payment deadline remains timely if
	// processing was delayed. A callback received at/after the deadline is
	// kept for manual reconciliation and never reactivates an expired hold.
	if !value.PayExpiresAt.After(authenticatedAt) {
		now := clock().UTC()
		if _, err = expiry.LockedReservation(ctx, tx, reservationID, now); err != nil {
			return booking.Reservation{}, err
		}
		if _, err = tx.Exec(ctx, `UPDATE public.reserva_pago_evento_aplicacion_ensayo SET estado='pendiente_conciliacion',codigo_resultado='resultado_tardio',procesado_en=NULL WHERE evento_id=$1`, eventRowID); err != nil {
			return booking.Reservation{}, err
		}
		if err = tx.Commit(ctx); err != nil {
			return booking.Reservation{}, err
		}
		return value, booking.ErrConflict
	}
	now := clock().UTC()
	comparisonAt := authenticatedAt
	expired, err := expiry.LockedReservation(ctx, tx, reservationID, comparisonAt)
	if err != nil {
		return booking.Reservation{}, err
	}
	if expired {
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
