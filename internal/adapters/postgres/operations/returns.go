package operationspg

import (
	"context"

	"github.com/HernanEspinozaDev/espaciGo/internal/operation"
	"github.com/jackc/pgx/v5"
)

// ListSpaceReturns reads a consistent current-owner projection, not the private
// participant/contract history of the previous owner. Confirmation separately
// revalidates and locks ownership in the existing reception transaction.
func (r *Repository) ListSpaceReturns(ctx context.Context, actor, space string) ([]operation.SpaceReturn, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var authorized bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.espacio e
 JOIN public.usuario u ON u.id=e.propietario_id AND u.estado='activo'
 WHERE e.id=$1 AND e.propietario_id=$2 AND e.estado<>'eliminada'
 AND NOT EXISTS(SELECT 1 FROM public.bloqueo_cuenta_administrativo_local b WHERE b.cuenta_id=u.id))`, space, actor).Scan(&authorized)
	if err != nil {
		return nil, mapError(err)
	}
	if !authorized {
		return nil, operation.ErrNotFound
	}
	rows, err := tx.Query(ctx, `SELECT r.id,r.estado,r.inicio,r.termino,
 EXISTS(SELECT 1 FROM public.operacion_arriendo_ensayo_local o WHERE o.reserva_id=r.id AND o.tipo='checkout'),
 EXISTS(SELECT 1 FROM public.operacion_arriendo_ensayo_local o WHERE o.reserva_id=r.id AND o.tipo='recepcion')
 AND EXISTS(SELECT 1 FROM public.ocupacion oc WHERE oc.id=r.ocupacion_id AND oc.espacio_id=r.espacio_id AND oc.reserva_id=r.id AND oc.tipo='reserva' AND NOT oc.activo AND oc.intervalo=tstzrange(r.inicio,r.termino,'[)'))
 AND NOT EXISTS(SELECT 1 FROM public.ocupacion oc WHERE oc.reserva_id=r.id AND oc.activo)
 FROM public.reserva_ensayo_local r WHERE r.espacio_id=$1 AND r.estado='finalizada'
 ORDER BY r.termino DESC,r.id`, space)
	if err != nil {
		return nil, mapError(err)
	}
	items := make([]operation.SpaceReturn, 0)
	for rows.Next() {
		var item operation.SpaceReturn
		if err = rows.Scan(&item.ReservationID, &item.State, &item.StartAt, &item.EndAt, &item.CheckOutRecorded, &item.VacancyConfirmed); err != nil {
			rows.Close()
			return nil, mapError(err)
		}
		items = append(items, item)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return nil, mapError(err)
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return items, nil
}
