package spacespg

import (
	"context"
	"encoding/json"
	"time"
)

// ExportOwnArchiveSections delegates draft selection to the same owner-scoped
// repository contract used by the spaces module's authenticated API.
func (r *Repository) ExportOwnArchiveSections(ctx context.Context, owner string) (map[string]json.RawMessage, error) {
	drafts, err := r.ListOwn(ctx, owner)
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(drafts)
	if err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx, `SELECT h.espacio_id::text,h.estado_anterior,h.estado_nuevo,h.ocurrida_en,'self'::text AS actor,h.correlacion_id
		FROM public.espacio_publicacion_historial_local h
		JOIN public.espacio e ON e.id=h.espacio_id
		WHERE e.propietario_id=$1
		ORDER BY h.ocurrida_en,h.id`, owner)
	if err != nil {
		return nil, err
	}
	type publicationEvent struct {
		SpaceID     string    `json:"space_id"`
		From        string    `json:"from_state"`
		To          string    `json:"to_state"`
		At          time.Time `json:"occurred_at"`
		Actor       string    `json:"actor"`
		Correlation string    `json:"correlation_id"`
	}
	history := make([]publicationEvent, 0)
	for rows.Next() {
		var item publicationEvent
		if err := rows.Scan(&item.SpaceID, &item.From, &item.To, &item.At, &item.Actor, &item.Correlation); err != nil {
			rows.Close()
			return nil, err
		}
		history = append(history, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	historyJSON, err := json.Marshal(history)
	if err != nil {
		return nil, err
	}
	return map[string]json.RawMessage{"spaces": encoded, "space_publication_history": historyJSON}, nil
}

var _ interface {
	ExportOwnArchiveSections(context.Context, string) (map[string]json.RawMessage, error)
} = (*Repository)(nil)
