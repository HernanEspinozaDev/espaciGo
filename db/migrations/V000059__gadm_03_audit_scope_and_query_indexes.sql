-- GADM-03 adds an explicit account subject for new account-scoped audit facts.
-- ON DELETE SET NULL preserves the append-only event without recreating an
-- identity if a future privacy flow removes its account row.
ALTER TABLE public.evento_auditoria_local
    ADD COLUMN cuenta_objetivo_id uuid REFERENCES public.usuario(id) ON DELETE SET NULL,
    ADD COLUMN registrado_en timestamptz NOT NULL DEFAULT clock_timestamp();

-- Keyset pages use event time and UUID; actor and account-target filters get
-- separate indexes so an actor is never confused with the affected account.
CREATE INDEX evento_auditoria_tiempo_id_idx
    ON public.evento_auditoria_local(ocurrido_en DESC, id DESC);

CREATE INDEX evento_auditoria_actor_tiempo_id_idx
    ON public.evento_auditoria_local(actor_id, ocurrido_en DESC, id DESC);

CREATE INDEX evento_auditoria_objetivo_tiempo_id_idx
    ON public.evento_auditoria_local(cuenta_objetivo_id, ocurrido_en DESC, id DESC)
    WHERE cuenta_objetivo_id IS NOT NULL;

COMMENT ON COLUMN public.evento_auditoria_local.cuenta_objetivo_id IS
    'Cuenta objetivo explícita para auditoría GADM; distinta del actor. NULL en eventos históricos o globales.';

COMMENT ON COLUMN public.evento_auditoria_local.registrado_en IS
    'Instante de inserción para describir el corte de exportaciones de auditoría; ocurrido_en sigue siendo el tiempo del evento.';
