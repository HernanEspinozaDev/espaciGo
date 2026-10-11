-- LOCAL-M04-PUBLICATION-DELETE-01. Retire a listing without deleting the
-- space row: historical bookings and their snapshots retain restrictive FKs.
ALTER TABLE public.espacio
    DROP CONSTRAINT IF EXISTS espacio_estado_local_publication_ck,
    ADD CONSTRAINT espacio_estado_local_publication_ck
        CHECK (estado IN ('borrador','activa','oculta','eliminada'));

CREATE TABLE public.espacio_eliminacion_historial_local (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    espacio_id uuid NOT NULL REFERENCES public.espacio(id) ON DELETE RESTRICT,
    actor_id uuid NOT NULL REFERENCES public.usuario(id) ON DELETE RESTRICT,
    ocurrida_en timestamptz NOT NULL,
    correlacion_id text NOT NULL CHECK (length(btrim(correlacion_id)) BETWEEN 1 AND 120),
    archivos_pendientes integer NOT NULL CHECK (archivos_pendientes >= 0),
    resumen jsonb NOT NULL CHECK (
      jsonb_typeof(resumen)='object'
      AND resumen ? 'removed'
      AND resumen ? 'preserved'
    ),
    UNIQUE (espacio_id)
);
CREATE INDEX espacio_eliminacion_owner_order_idx
    ON public.espacio_eliminacion_historial_local(actor_id,ocurrida_en,id);

CREATE OR REPLACE FUNCTION public.prevent_local_space_deletion_history_mutation() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'local space deletion history is append-only' USING ERRCODE='55000';
END;
$$;
CREATE TRIGGER espacio_eliminacion_historial_append_only
    BEFORE UPDATE OR DELETE ON public.espacio_eliminacion_historial_local
    FOR EACH ROW EXECUTE FUNCTION public.prevent_local_space_deletion_history_mutation();
