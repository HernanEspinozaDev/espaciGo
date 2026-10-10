-- Administrative account-block state is removed when privacy suppression is
-- executed; its append-only history remains as a minimized technical record.
-- No retention deadline is assigned here because the user has not ratified one.
ALTER TABLE public.bloqueo_cuenta_historial_local
    ALTER COLUMN actor_id DROP NOT NULL;
ALTER TABLE public.bloqueo_cuenta_administrativo_local
    ALTER COLUMN bloqueada_por DROP NOT NULL;

COMMENT ON COLUMN public.bloqueo_cuenta_historial_local.actor_id IS
    'Nullable when the actor account is retired; privacy execution may remove the personal link while preserving the structured event.';
COMMENT ON COLUMN public.bloqueo_cuenta_administrativo_local.bloqueada_por IS
    'Nullable when the administrator actor is retired; the active block remains effective without retaining that personal link.';
