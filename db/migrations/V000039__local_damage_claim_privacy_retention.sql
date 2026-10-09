-- LOCAL-DIS-01 privacy follow-through. A resolved synthetic claim stops being
-- an open blocker, its free text is scrubbed when an eligible baja executes,
-- and identifying links are retained only through the existing 24-month
-- reservation-history window before the privacy purger unlinks them.
ALTER TABLE public.reclamo_dano_ensayo_local
    ALTER COLUMN reserva_id DROP NOT NULL,
    ALTER COLUMN anfitrion_id DROP NOT NULL,
    ALTER COLUMN arrendatario_id DROP NOT NULL,
    ALTER COLUMN checkout_operacion_id DROP NOT NULL,
    ALTER COLUMN checkout_evidencia_id DROP NOT NULL;

ALTER TABLE public.reclamo_dano_descargo_ensayo_local
    ALTER COLUMN actor_id DROP NOT NULL;

ALTER TABLE public.reclamo_dano_historial_ensayo_local
    ALTER COLUMN actor_id DROP NOT NULL;

ALTER TABLE public.reclamo_dano_resolucion_ensayo_local
    ALTER COLUMN administrador_id DROP NOT NULL;

ALTER TABLE public.operacion_arriendo_ensayo_local
    ALTER COLUMN reserva_id DROP NOT NULL,
    ALTER COLUMN actor_id DROP NOT NULL;

ALTER TABLE public.operacion_arriendo_historial_ensayo_local
    ALTER COLUMN reserva_id DROP NOT NULL,
    ALTER COLUMN actor_id DROP NOT NULL;

ALTER TABLE public.operacion_arriendo_archivo_candidato_local
    ALTER COLUMN reserva_id DROP NOT NULL,
    ALTER COLUMN actor_id DROP NOT NULL;

COMMENT ON COLUMN public.reclamo_dano_ensayo_local.reserva_id IS
    'Retained until the reservation privacy deadline; nulled by the local 24-month history-link purger.';
