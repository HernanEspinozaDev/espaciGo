-- Local M04 publication state for owner-managed offers. This does not expose
-- spaces in a general/public catalog; discovery is connected in its own cut.
ALTER TABLE public.espacio
    DROP CONSTRAINT IF EXISTS espacio_estado_check,
    ADD CONSTRAINT espacio_estado_local_publication_ck
        CHECK (estado IN ('borrador','activa','oculta'));

CREATE TABLE public.espacio_publicacion_historial_local (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    espacio_id uuid NOT NULL REFERENCES public.espacio(id) ON DELETE RESTRICT,
    actor_id uuid NOT NULL REFERENCES public.usuario(id) ON DELETE RESTRICT,
    estado_anterior text NOT NULL CHECK (estado_anterior IN ('borrador','activa','oculta')),
    estado_nuevo text NOT NULL CHECK (estado_nuevo IN ('activa','oculta')),
    ocurrida_en timestamptz NOT NULL,
    correlacion_id text NOT NULL CHECK (length(btrim(correlacion_id)) BETWEEN 1 AND 120),
    CONSTRAINT espacio_publicacion_transicion_ck CHECK (
        (estado_anterior IN ('borrador','oculta') AND estado_nuevo='activa')
        OR (estado_anterior='activa' AND estado_nuevo='oculta')
    )
);
CREATE INDEX espacio_publicacion_historial_order_idx
    ON public.espacio_publicacion_historial_local(espacio_id,id);

CREATE OR REPLACE FUNCTION public.prevent_local_publication_history_mutation() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'local publication history is append-only' USING ERRCODE='55000';
END;
$$;
CREATE TRIGGER espacio_publicacion_historial_append_only
    BEFORE UPDATE OR DELETE ON public.espacio_publicacion_historial_local
    FOR EACH ROW EXECUTE FUNCTION public.prevent_local_publication_history_mutation();
