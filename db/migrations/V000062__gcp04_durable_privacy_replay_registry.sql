-- GCP-04 local backend slice: keep the minimal, completed suppression replay
-- inventory in PostgreSQL so API instances and process restarts share one source.
-- No FK is intentional: this restore registry must outlive retention of source
-- account/request/execution rows and contains only pseudonymous technical IDs.
CREATE TABLE public.registro_replay_baja_local (
    ejecucion_origen_id uuid PRIMARY KEY,
    solicitud_id uuid NOT NULL,
    usuario_id uuid NOT NULL,
    solicitada_en timestamptz NOT NULL,
    completada_en timestamptz NOT NULL
);

CREATE INDEX registro_replay_baja_local_completed_idx
    ON public.registro_replay_baja_local(completada_en, ejecucion_origen_id);

-- Seed already completed work once during upgrade. The registry remains after
-- source rows are removed by their separately governed retention policy.
INSERT INTO public.registro_replay_baja_local
    (ejecucion_origen_id, solicitud_id, usuario_id, solicitada_en, completada_en)
SELECT x.id, x.solicitud_id, x.usuario_id, s.solicitada_en, x.completada_en
FROM public.ejecucion_baja_local x
JOIN public.solicitud_titular s ON s.id = x.solicitud_id
WHERE x.estado = 'completada' AND x.completada_en IS NOT NULL
ON CONFLICT (ejecucion_origen_id) DO NOTHING;
