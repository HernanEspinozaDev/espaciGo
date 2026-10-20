-- Distinguish the candidate-scoped GCP bootstrap CLI from the local CLI while
-- preserving the existing audit actor and append-only event contract.
ALTER TABLE public.administrador_global_bootstrap_evento
    DROP CONSTRAINT administrador_global_bootstrap_evento_actor_tipo_check;

ALTER TABLE public.administrador_global_bootstrap_evento
    ADD CONSTRAINT administrador_global_bootstrap_evento_actor_tipo_check
    CHECK (actor_tipo IN ('cli_local', 'cli_gcp06'));
