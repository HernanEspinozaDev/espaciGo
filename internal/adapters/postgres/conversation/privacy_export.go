package conversationpg

import (
	"context"
	"encoding/json"
	"fmt"
)

func (r *Repository) ExportOwnArchiveSections(ctx context.Context, owner string) (map[string]json.RawMessage, error) {
	queries := map[string]string{
		"messages_written_by_owner": `SELECT COALESCE(jsonb_agg(jsonb_build_object('reservation_id',m.reserva_id::text,'sequence',m.secuencia,'body',m.cuerpo,'created_at',m.creada_en) ORDER BY m.reserva_id,m.secuencia),'[]'::jsonb) FROM public.mensaje_reserva_ensayo m WHERE m.autor_id=$1`,
		"conversation_read_cursors": `SELECT COALESCE(jsonb_agg(jsonb_build_object('reservation_id',c.reserva_id::text,'last_sequence_read',c.ultima_secuencia_leida,'updated_at',c.actualizada_en) ORDER BY c.reserva_id),'[]'::jsonb) FROM public.reserva_mensaje_lectura c WHERE c.participante_id=$1`,
	}
	result := make(map[string]json.RawMessage, len(queries))
	for section, query := range queries {
		var raw []byte
		if err := r.pool.QueryRow(ctx, query, owner).Scan(&raw); err != nil {
			return nil, fmt.Errorf("conversation export %s: %w", section, err)
		}
		if !json.Valid(raw) {
			return nil, fmt.Errorf("conversation export %s returned invalid JSON", section)
		}
		result[section] = json.RawMessage(raw)
	}
	return result, nil
}
