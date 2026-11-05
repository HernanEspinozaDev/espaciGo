package operationspg

import (
	"context"
	"sort"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/operation"
	"github.com/jackc/pgx/v5"
)

type operationScope struct {
	host, renter, owner, space, occupation, state, zone string
	start, end                                          time.Time
}

// Locks follow account -> space -> reservation, matching publication and booking.
// Ownership is observed before account locks, then revalidated under the space
// lock. A concurrent change cannot silently transfer private contract authority.
func lockOperationScope(ctx context.Context, tx pgx.Tx, actor, reservation, kind string) (operationScope, error) {
	var s operationScope
	err := tx.QueryRow(ctx, `SELECT COALESCE(r.anfitrion_id::text,''),COALESCE(r.arrendatario_id::text,''),r.espacio_id::text,e.propietario_id::text
 FROM public.reserva_ensayo_local r JOIN public.espacio e ON e.id=r.espacio_id WHERE r.id=$1`, reservation).Scan(&s.host, &s.renter, &s.space, &s.owner)
	if err != nil {
		return s, mapError(err)
	}
	if s.host == "" || s.renter == "" || kind == operation.Receipt && actor != s.owner || kind != operation.Receipt && actor != s.host && actor != s.renter {
		return s, operation.ErrNotFound
	}
	ids := []string{s.host, s.renter}
	if kind == operation.Receipt {
		ids = append(ids, s.owner)
	}
	sort.Strings(ids)
	for i, id := range ids {
		if i > 0 && ids[i-1] == id {
			continue
		}
		var active bool
		if err = tx.QueryRow(ctx, `SELECT estado='activo' FROM public.usuario WHERE id=$1 FOR UPDATE`, id).Scan(&active); err != nil {
			return s, mapError(err)
		}
		if !active {
			return s, operation.ErrConflict
		}
	}
	var owner, state string
	if err = tx.QueryRow(ctx, `SELECT propietario_id::text,estado FROM public.espacio WHERE id=$1 FOR UPDATE`, s.space).Scan(&owner, &state); err != nil {
		return s, mapError(err)
	}
	if kind == operation.Receipt {
		if owner != s.owner || owner != actor || state == "eliminada" {
			return s, operation.ErrNotFound
		}
		var blocked bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.bloqueo_cuenta_administrativo_local WHERE cuenta_id=$1)`, actor).Scan(&blocked); err != nil {
			return s, mapError(err)
		}
		if blocked {
			return s, operation.ErrConflict
		}
	}
	var host, renter, space string
	err = tx.QueryRow(ctx, `SELECT COALESCE(anfitrion_id::text,''),COALESCE(arrendatario_id::text,''),espacio_id::text,ocupacion_id::text,estado,inicio,termino,zona_horaria
 FROM public.reserva_ensayo_local WHERE id=$1 FOR UPDATE`, reservation).Scan(&host, &renter, &space, &s.occupation, &s.state, &s.start, &s.end, &s.zone)
	if err != nil {
		return s, mapError(err)
	}
	if host != s.host || renter != s.renter || space != s.space {
		return s, operation.ErrNotFound
	}
	return s, nil
}

func (w *candidateWriter) verifyReturnOccupation(ctx context.Context, wantActive bool) error {
	if w.state != "finalizada" {
		return operation.ErrConflict
	}
	var checkedOut bool
	if err := w.tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.operacion_arriendo_ensayo_local WHERE reserva_id=$1 AND tipo='checkout')`, w.reservation).Scan(&checkedOut); err != nil {
		return mapError(err)
	}
	if !checkedOut {
		return operation.ErrConflict
	}
	var active bool
	err := w.tx.QueryRow(ctx, `SELECT activo FROM public.ocupacion
 WHERE id=$1 AND espacio_id=$2 AND reserva_id=$3 AND tipo='reserva'
 AND intervalo=tstzrange($4,$5,'[)') FOR UPDATE`, w.occupation, w.space, w.reservation, w.start, w.end).Scan(&active)
	if err != nil {
		return mapError(err)
	}
	if active != wantActive {
		return operation.ErrConflict
	}
	// A malformed extra occupation for this reservation must not be silently
	// stranded or swept. Other reservations and manual blocks are never touched.
	var extra bool
	if err = w.tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.ocupacion WHERE reserva_id=$1 AND id<>$2 AND activo)`, w.reservation, w.occupation).Scan(&extra); err != nil {
		return mapError(err)
	}
	if extra {
		return operation.ErrConflict
	}
	return nil
}
