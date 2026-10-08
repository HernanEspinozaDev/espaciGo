package dispute

import (
	"context"
	"encoding/json"
	"fmt"
)

func (r *Repository) ExportOwnArchiveSections(ctx context.Context, owner string) (map[string]json.RawMessage, error) {
	const query = `SELECT COALESCE(jsonb_agg(jsonb_build_object('id',d.id::text,'reservation_id',d.reserva_id::text,'state',d.estado,'opening_reason_code',d.motivo_codigo,'opened_at',d.abierta_en,'closed_at',d.cerrada_en,'close_reason_code',d.motivo_cierre_codigo,'history',(SELECT COALESCE(jsonb_agg(jsonb_build_object('sequence',h.secuencia,'from',h.estado_anterior,'to',h.estado_nuevo,'actor',CASE WHEN h.actor_id=$1 THEN 'self' WHEN h.actor_id IS NULL THEN 'system' ELSE 'counterparty' END,'reason_code',h.motivo_codigo,'at',h.ocurrida_en) ORDER BY h.secuencia),'[]'::jsonb) FROM public.disputa_ensayo_historial h WHERE h.disputa_id=d.id)) ORDER BY d.abierta_en,d.id),'[]'::jsonb) FROM public.disputa_ensayo_local d WHERE d.anfitrion_id=$1 OR d.arrendatario_id=$1`
	var raw []byte
	if err := r.pool.QueryRow(ctx, query, owner).Scan(&raw); err != nil {
		return nil, fmt.Errorf("dispute archive: %w", err)
	}
	if !json.Valid(raw) {
		return nil, fmt.Errorf("dispute archive returned invalid JSON")
	}
	return map[string]json.RawMessage{"disputes": json.RawMessage(raw)}, nil
}
