-- MPV2-12 imports publication availability through the audited global-admin
-- schedule API. Hourly schedules apply to ordinary publications as well as
-- the single local booking fixture; reservation scenarios remain separate.
CREATE OR REPLACE FUNCTION public.validar_horario_semanal_fixture_hora()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.activo AND NOT EXISTS (
        SELECT 1
        FROM public.espacio e
        JOIN LATERAL (
            SELECT modalidad
            FROM public.tarifa_espacio
            WHERE espacio_id=e.id
            ORDER BY version DESC
            LIMIT 1
        ) t ON true
        WHERE e.id=NEW.espacio_id
          AND e.estado <> 'eliminada'
          AND t.modalidad='hora'
    ) THEN
        RAISE EXCEPTION 'el horario semanal activo requiere una publicacion vigente con tarifa por hora'
            USING ERRCODE='23514', CONSTRAINT='horario_semanal_publicacion_hora_ck';
    END IF;
    RETURN NEW;
END;
$$;
