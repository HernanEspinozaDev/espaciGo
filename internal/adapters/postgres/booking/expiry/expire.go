package expiry

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// LockedReservation applies the existing local-booking expiry transition to a
// reservation row already locked by tx. It is shared by sweep operations and
// message sends so a write can never bypass a deadline merely because no read
// triggered the sweep first.
func LockedReservation(ctx context.Context, tx pgx.Tx, reservationID string, now time.Time) (bool, error) {
	var state string
	var paymentDeadline time.Time
	var hostDeadline *time.Time
	err := tx.QueryRow(ctx, `SELECT estado,pago_vence_en,anfitrion_vence_en
FROM public.reserva_ensayo_local WHERE id=$1 FOR UPDATE`, reservationID).Scan(&state, &paymentDeadline, &hostDeadline)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	next, reason := "", ""
	if state == "pendiente_de_pago" && !paymentDeadline.After(now) {
		// Do not race a callback that was durably authenticated before its
		// deadline. The payment reconciler will apply it using autenticado_en.
		var timelyCallback bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(
SELECT 1 FROM public.reserva_pago_evento_ensayo e
JOIN public.reserva_pago_ensayo_operacion p ON p.id=e.operacion_id
JOIN public.reserva_pago_evento_aplicacion_ensayo a ON a.evento_id=e.id
WHERE p.reserva_id=$1 AND e.autenticado_en<$2 AND a.estado IN ('pendiente','pendiente_conciliacion'))`, reservationID, paymentDeadline).Scan(&timelyCallback); err != nil {
			return false, err
		}
		if !timelyCallback {
			next, reason = "vencida_pago", "venció plazo de pago local"
		}
	} else if state == "pagada" && hostDeadline != nil && !hostDeadline.After(now) {
		next, reason = "vencida_host", "venció plazo de respuesta del anfitrión"
	}
	if next == "" {
		return false, nil
	}
	if _, err = tx.Exec(ctx, `UPDATE public.ocupacion SET activo=false,desactivada_en=$2 WHERE reserva_id=$1 AND activo`, reservationID, now); err != nil {
		return false, err
	}
	if _, err = tx.Exec(ctx, `UPDATE public.reserva_ensayo_local SET estado=$2,actualizada_en=$3 WHERE id=$1`, reservationID, next, now); err != nil {
		return false, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO public.reserva_ensayo_transicion(id,reserva_id,secuencia,estado_anterior,estado_nuevo,actor_id,motivo,creada_en)
SELECT gen_random_uuid(),$1,COALESCE(MAX(secuencia),0)+1,$2,$3,NULL,$4,$5
FROM public.reserva_ensayo_transicion WHERE reserva_id=$1`, reservationID, state, next, reason, now); err != nil {
		return false, err
	}
	if next == "vencida_pago" {
		if _, err = tx.Exec(ctx, `UPDATE public.reserva_pago_ensayo_operacion SET estado='vencida',actualizada_en=$2 WHERE reserva_id=$1 AND estado='pendiente'`, reservationID, now); err != nil {
			return false, err
		}
	}
	if state == "pagada" {
		if _, err = tx.Exec(ctx, `INSERT INTO public.reserva_pago_ensayo(id,reserva_id,resultado,clave_idempotencia,creada_en) VALUES(gen_random_uuid(),$1,'devolucion_simulada',$2,$3)`, reservationID, "devolucion-expiracion:"+reservationID, now); err != nil {
			return false, err
		}
	}
	if next == "vencida_host" {
		if err = closeGuaranteeForHostExpiry(ctx, tx, reservationID, now); err != nil {
			return false, err
		}
	}
	return true, nil
}

// closeGuaranteeForHostExpiry records the guarantee release obligation in the
// same transaction as the host-expiry transition. An uncertain authorization
// stays available for reconciliation; a late success will create its release
// only after it is recorded as authorized.
func closeGuaranteeForHostExpiry(ctx context.Context, tx pgx.Tx, reservationID string, now time.Time) error {
	var guaranteeID, state string
	var authorized, captured, released int64
	err := tx.QueryRow(ctx, `SELECT id::text,estado,autorizado_clp,capturado_clp,liberado_clp
		FROM public.reserva_garantia_ensayo_local WHERE reserva_id=$1 FOR UPDATE`, reservationID).
		Scan(&guaranteeID, &state, &authorized, &captured, &released)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	remaining := authorized - captured - released
	if remaining > 0 && (state == "autorizada" || state == "parcialmente_capturada" || state == "liberacion_pendiente" || state == "liberacion_por_conciliar") {
		key := "host-expiry-release:" + reservationID
		fingerprint, _ := json.Marshal([]any{"liberacion", remaining, "exito"})
		sum := sha256.Sum256(fingerprint)
		_, err = tx.Exec(ctx, `INSERT INTO public.reserva_garantia_operacion_ensayo_local(
			id,garantia_id,tipo,clave_idempotencia,huella_solicitud,importe_clp,resultado_solicitado,estado,
			primer_intento_en,ultimo_resultado,completada_en,creada_en,actualizada_en)
			VALUES(gen_random_uuid(),$1,'liberacion',$2,$3,$4,'exito','pendiente',$5,NULL,NULL,$5,$5)
			ON CONFLICT(garantia_id,tipo) DO NOTHING`, guaranteeID, key, sum[:], remaining, now)
		if err != nil {
			return err
		}
		var opState string
		if err = tx.QueryRow(ctx, `SELECT estado FROM public.reserva_garantia_operacion_ensayo_local WHERE garantia_id=$1 AND tipo='liberacion' FOR UPDATE`, guaranteeID).Scan(&opState); err != nil {
			return err
		}
		switch opState {
		case "pendiente":
			state = "liberacion_pendiente"
		case "por_conciliar":
			state = "liberacion_por_conciliar"
		case "confirmada":
			if authorized-captured-released == 0 {
				state = "liberada"
			} else {
				state = "parcialmente_capturada"
			}
		}
	} else if state == "pendiente_pago" || state == "pendiente_autorizacion" || state == "por_conciliar" {
		// Keep a pending or uncertain authorization outstanding for its durable
		// callback/reconciliation path; do not convert it into a release.
		state = "por_conciliar"
	}
	_, err = tx.Exec(ctx, `UPDATE public.reserva_garantia_ensayo_local SET estado=$2,actualizada_en=$3 WHERE id=$1`, guaranteeID, state, now)
	return err
}
