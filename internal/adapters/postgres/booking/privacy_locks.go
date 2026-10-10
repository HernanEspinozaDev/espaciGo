package bookingpg

import (
	"context"
	"sort"

	"github.com/HernanEspinozaDev/espaciGo/internal/booking"
	"github.com/jackc/pgx/v5"
)

// lockActiveAccounts shares the account row lock used by local suppression.
// Business mutations must keep it until commit so a baja cannot race a new
// reservation, payment, cancellation or other obligation.
func lockActiveAccounts(ctx context.Context, tx pgx.Tx, ids ...string) error {
	ordered := append([]string(nil), ids...)
	sort.Strings(ordered)
	unique := ordered[:0]
	for _, id := range ordered {
		if len(unique) == 0 || unique[len(unique)-1] != id {
			unique = append(unique, id)
		}
	}
	for _, id := range unique {
		var state string
		if err := tx.QueryRow(ctx, `SELECT estado FROM public.usuario WHERE id=$1 FOR UPDATE`, id).Scan(&state); err != nil {
			return mapErr(err)
		}
		if state != "activo" {
			return booking.ErrConflict
		}
	}
	return nil
}

func requireUnblockedAccounts(ctx context.Context, tx pgx.Tx, ids ...string) error {
	for _, id := range ids {
		var blocked bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.bloqueo_cuenta_administrativo_local WHERE cuenta_id=$1)`, id).Scan(&blocked); err != nil {
			return err
		}
		if blocked {
			return booking.ErrConflict
		}
	}
	return nil
}

// requireLocalKYCEligibility is evaluated only after the participant account
// rows are locked. Verification approval and privacy suppression take the
// same account lock, so eligibility cannot be revoked between this check and
// reservation/occupancy commit.
func requireLocalKYCEligibility(ctx context.Context, tx pgx.Tx, ids ...string) error {
	for _, id := range ids {
		var eligible bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (
			SELECT 1 FROM public.elegibilidad_verificacion_local eligibility
			JOIN public.verificacion verification ON verification.id=eligibility.verificacion_id
			JOIN public.usuario account ON account.id=eligibility.usuario_id
			WHERE eligibility.usuario_id=$1 AND eligibility.tipo='kyc' AND eligibility.estado='elegible'
			  AND verification.estado='aprobada' AND account.estado='activo'
		)`, id).Scan(&eligible); err != nil {
			return err
		}
		if !eligible {
			return booking.ErrConflict
		}
	}
	return nil
}

func lockReservationParties(ctx context.Context, tx pgx.Tx, reservationID string) error {
	var hostID, renterID string
	if err := tx.QueryRow(ctx, `SELECT anfitrion_id::text,arrendatario_id::text FROM public.reserva_ensayo_local WHERE id=$1`, reservationID).Scan(&hostID, &renterID); err != nil {
		return mapErr(err)
	}
	return lockActiveAccounts(ctx, tx, hostID, renterID)
}
