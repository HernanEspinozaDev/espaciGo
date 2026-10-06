package expiry

import (
	"context"
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
		next, reason = "vencida_pago", "venció plazo de pago local"
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
	if state == "pagada" {
		if _, err = tx.Exec(ctx, `INSERT INTO public.reserva_pago_ensayo(id,reserva_id,resultado,clave_idempotencia,creada_en) VALUES(gen_random_uuid(),$1,'devolucion_simulada',$2,$3)`, reservationID, "devolucion-expiracion:"+reservationID, now); err != nil {
			return false, err
		}
	}
	return true, nil
}
