-- Keep immutable rate rows non-deletable for the runtime role. Publication
-- retirement may remove only unreferenced rates after the owning space has
-- been tombstoned; referenced snapshots remain protected by their FK.
CREATE OR REPLACE FUNCTION public.purge_unreferenced_local_publication_rates(p_space_id uuid)
RETURNS integer
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, public
AS $$
DECLARE
    removed_count integer;
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM public.espacio
        WHERE id = p_space_id AND estado = 'eliminada'
    ) THEN
        RAISE EXCEPTION 'publication must be retired before rate cleanup'
            USING ERRCODE = '42501';
    END IF;

    DELETE FROM public.tarifa_espacio AS rate
    WHERE rate.espacio_id = p_space_id
      AND NOT EXISTS (
          SELECT 1 FROM public.cotizacion_reserva_ensayo AS quote
          WHERE quote.espacio_id = rate.espacio_id
            AND quote.tarifa_version = rate.version
      );
    GET DIAGNOSTICS removed_count = ROW_COUNT;
    RETURN removed_count;
END;
$$;

REVOKE ALL ON FUNCTION public.purge_unreferenced_local_publication_rates(uuid) FROM PUBLIC;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'espacigo_runtime') THEN
        EXECUTE 'REVOKE DELETE ON public.tarifa_espacio FROM espacigo_runtime';
        EXECUTE 'GRANT EXECUTE ON FUNCTION public.purge_unreferenced_local_publication_rates(uuid) TO espacigo_runtime';
    END IF;
END;
$$;
