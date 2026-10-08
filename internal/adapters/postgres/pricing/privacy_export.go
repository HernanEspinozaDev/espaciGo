package pricingpg

import (
	"context"
	"encoding/json"
	"fmt"
)

func (r *Repository) ExportOwnArchiveSections(ctx context.Context, owner string) (map[string]json.RawMessage, error) {
	queries := map[string]string{
		"rates":       `SELECT COALESCE(jsonb_agg(jsonb_build_object('space_id',t.espacio_id::text,'version',t.version,'rate_unit',t.modalidad,'unit_price_clp',t.precio_base_clp,'currency',t.moneda,'created_at',t.creada_en) ORDER BY t.espacio_id,t.version),'[]'::jsonb) FROM public.tarifa_espacio t JOIN public.espacio e ON e.id=t.espacio_id WHERE e.propietario_id=$1`,
		"simulations": `SELECT COALESCE(jsonb_agg(jsonb_build_object('id',s.id::text,'space_id',s.espacio_id::text,'rate_version',s.tarifa_version,'rate_unit',s.modalidad,'unit_price_clp',s.precio_unitario_clp,'billed_units',s.unidades_facturadas,'currency',s.moneda,'subtotal_clp',s.subtotal_clp,'start_at',s.inicio,'end_at',s.termino,'time_zone',s.zona_horaria,'created_at',s.creada_en) ORDER BY s.creada_en,s.id),'[]'::jsonb) FROM public.simulacion_precio_privada s WHERE s.propietario_id=$1`,
	}
	result := make(map[string]json.RawMessage, len(queries))
	for section, query := range queries {
		var raw []byte
		if err := r.pool.QueryRow(ctx, query, owner).Scan(&raw); err != nil {
			return nil, fmt.Errorf("pricing archive %s: %w", section, err)
		}
		if !json.Valid(raw) {
			return nil, fmt.Errorf("pricing archive %s returned invalid JSON", section)
		}
		result[section] = json.RawMessage(raw)
	}
	return result, nil
}
