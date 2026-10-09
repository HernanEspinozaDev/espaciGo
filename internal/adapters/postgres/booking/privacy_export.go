package bookingpg

import (
	"context"
	"encoding/json"
	"fmt"
)

func (r *Repository) ExportOwnArchiveSections(ctx context.Context, owner string) (map[string]json.RawMessage, error) {
	queries := map[string]string{
		"quotes":              `SELECT COALESCE(jsonb_agg(jsonb_build_object('id',q.id::text,'space_id',q.espacio_id::text,'category_code',q.categoria_codigo,'profile_version',q.perfil_version,'profile_values',q.perfil_valores_snapshot,'rate_version',q.tarifa_version,'rate_unit',q.modalidad,'unit_price_clp',q.precio_unitario_clp,'units',q.unidades,'currency',q.moneda,'subtotal_clp',q.subtotal_clp,'start_at',q.inicio,'end_at',q.termino,'time_zone',q.zona_horaria,'cancellation_policy_version',q.politica_cancelacion_version,'guarantee_policy_version',q.garantia_politica_version,'guarantee_currency',q.garantia_moneda,'guarantee_expected_clp',q.garantia_prevista_clp,'created_at',q.creada_en,'expires_at',q.vence_en,'converted_to_reservation',EXISTS(SELECT 1 FROM public.reserva_ensayo_local r WHERE r.cotizacion_id=q.id)) ORDER BY q.creada_en,q.id),'[]'::jsonb) FROM public.cotizacion_reserva_ensayo q WHERE q.arrendatario_id=$1 OR EXISTS(SELECT 1 FROM public.espacio e WHERE e.id=q.espacio_id AND e.propietario_id=$1)`,
		"reservations":        `SELECT COALESCE(jsonb_agg(jsonb_build_object('id',r.id::text,'quote_id',r.cotizacion_id::text,'role',CASE WHEN r.anfitrion_id=$1 THEN 'host' ELSE 'renter' END,'space_id',r.espacio_id::text,'state',r.estado,'rate_unit',r.modalidad,'unit_price_clp',r.precio_unitario_clp,'units',r.unidades,'currency',r.moneda,'subtotal_clp',r.subtotal_clp,'start_at',r.inicio,'end_at',r.termino,'time_zone',r.zona_horaria,'cancellation_policy_version',r.politica_cancelacion_version,'guarantee_policy_version',r.garantia_politica_version,'guarantee_currency',r.garantia_moneda,'guarantee_expected_clp',r.garantia_prevista_clp,'payment_deadline',r.pago_vence_en,'host_deadline',r.anfitrion_vence_en,'created_at',r.creada_en,'updated_at',r.actualizada_en,'history',(SELECT COALESCE(jsonb_agg(jsonb_build_object('sequence',h.secuencia,'from',h.estado_anterior,'to',h.estado_nuevo,'actor',CASE WHEN h.actor_id=$1 THEN 'self' WHEN h.actor_id IS NULL THEN 'system' ELSE 'counterparty' END,'at',h.creada_en) ORDER BY h.secuencia),'[]'::jsonb) FROM public.reserva_ensayo_transicion h WHERE h.reserva_id=r.id)) ORDER BY r.creada_en,r.id),'[]'::jsonb) FROM public.reserva_ensayo_local r WHERE r.anfitrion_id=$1 OR r.arrendatario_id=$1`,
		"payments":            `SELECT COALESCE(jsonb_agg(jsonb_build_object('reservation_id',p.reserva_id::text,'result',p.resultado,'amount_clp',p.importe_clp,'currency','CLP','occurred_at',p.creada_en) ORDER BY p.creada_en,p.id),'[]'::jsonb) FROM public.reserva_pago_ensayo p JOIN public.reserva_ensayo_local r ON r.id=p.reserva_id WHERE r.anfitrion_id=$1 OR r.arrendatario_id=$1`,
		"payment_operations":  `SELECT COALESCE(jsonb_agg(jsonb_build_object('reservation_id',o.reserva_id::text,'state',o.estado,'result',o.resultado_final,'created_at',o.creada_en,'updated_at',o.actualizada_en,'events',(SELECT COALESCE(jsonb_agg(jsonb_build_object('result',a.estado,'outcome',e.resultado,'received_at',e.recibido_en,'processed_at',a.procesado_en) ORDER BY e.recibido_en,e.id),'[]'::jsonb) FROM public.reserva_pago_evento_ensayo e JOIN public.reserva_pago_evento_aplicacion_ensayo a ON a.evento_id=e.id WHERE e.operacion_id=o.id)) ORDER BY o.creada_en,o.id),'[]'::jsonb) FROM public.reserva_pago_ensayo_operacion o JOIN public.reserva_ensayo_local r ON r.id=o.reserva_id WHERE r.anfitrion_id=$1 OR r.arrendatario_id=$1`,
		"refunds":             `SELECT COALESCE(jsonb_agg(jsonb_build_object('reservation_id',d.reserva_id::text,'state',d.estado,'amount_clp',d.importe_clp,'currency',d.moneda,'last_result',d.ultimo_resultado,'created_at',d.creada_en,'updated_at',d.actualizada_en,'completed_at',d.completada_en,'attempts',(SELECT COALESCE(jsonb_agg(jsonb_build_object('result',a.resultado,'at',a.creada_en) ORDER BY a.secuencia),'[]'::jsonb) FROM public.reserva_devolucion_intento_ensayo a WHERE a.devolucion_id=d.id)) ORDER BY d.creada_en,d.id),'[]'::jsonb) FROM public.reserva_devolucion_ensayo d JOIN public.reserva_ensayo_local r ON r.id=d.reserva_id WHERE r.anfitrion_id=$1 OR r.arrendatario_id=$1`,
		"guarantees":          `SELECT COALESCE(jsonb_agg(jsonb_build_object('reservation_id',r.id::text,'policy_version',g.politica_version,'currency',g.moneda,'expected_clp',g.previsto_clp,'authorized_clp',g.autorizado_clp,'captured_clp',g.capturado_clp,'released_clp',g.liberado_clp,'state',g.estado,'operations',(SELECT COALESCE(jsonb_agg(jsonb_build_object('kind',o.tipo,'amount_clp',o.importe_clp,'state',o.estado,'last_result',o.ultimo_resultado,'created_at',o.creada_en,'updated_at',o.actualizada_en) ORDER BY o.creada_en,o.id),'[]'::jsonb) FROM public.reserva_garantia_operacion_ensayo_local o WHERE o.garantia_id=g.id)) ORDER BY r.creada_en,r.id),'[]'::jsonb) FROM public.reserva_garantia_ensayo_local g JOIN public.reserva_ensayo_local r ON r.id=g.reserva_id WHERE r.anfitrion_id=$1 OR r.arrendatario_id=$1`,
		"financial_decisions": `SELECT COALESCE(jsonb_agg(jsonb_build_object('reservation_id',r.id::text,'outcome',d.resultado_reclamo,'deduction_clp',d.deduccion_clp,'reason_code',d.motivo_codigo,'evidence_id',d.evidencia_id::text,'state',d.estado,'created_at',d.creada_en,'updated_at',d.actualizada_en) ORDER BY d.creada_en,d.id),'[]'::jsonb) FROM public.reserva_decision_financiera_ensayo_local d JOIN public.reserva_ensayo_local r ON r.id=d.reserva_id WHERE r.anfitrion_id=$1 OR r.arrendatario_id=$1`,
	}
	result := make(map[string]json.RawMessage, len(queries))
	for section, query := range queries {
		var raw []byte
		if err := r.pool.QueryRow(ctx, query, owner).Scan(&raw); err != nil {
			return nil, fmt.Errorf("booking export %s: %w", section, err)
		}
		if !json.Valid(raw) {
			return nil, fmt.Errorf("booking export %s returned invalid JSON", section)
		}
		result[section] = json.RawMessage(raw)
	}
	return result, nil
}
