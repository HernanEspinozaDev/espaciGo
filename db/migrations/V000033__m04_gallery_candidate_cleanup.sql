-- Durable cleanup jobs for generated files that never become an active photo.
-- A successful gallery insert deletes its candidate row in the same transaction.
CREATE TABLE public.espacio_galeria_archivo_candidato_local (
    archivo_id uuid PRIMARY KEY,
    propietario_id uuid NOT NULL,
    espacio_id uuid NOT NULL,
    estado text NOT NULL CHECK (estado IN ('reservado','pendiente_limpieza','limpiando')),
    creada_en timestamptz NOT NULL,
    proximo_intento_en timestamptz,
    intentos_limpieza integer NOT NULL DEFAULT 0 CHECK (intentos_limpieza >= 0),
    ultimo_codigo_error text,
    CONSTRAINT espacio_galeria_candidato_owner_fk FOREIGN KEY (espacio_id,propietario_id)
        REFERENCES public.espacio(id,propietario_id) ON DELETE RESTRICT,
    CONSTRAINT espacio_galeria_candidato_state_ck CHECK (
        (estado='reservado' AND proximo_intento_en IS NULL)
        OR (estado IN ('pendiente_limpieza','limpiando') AND proximo_intento_en IS NOT NULL)
    )
);
CREATE INDEX espacio_galeria_candidato_cleanup_idx
    ON public.espacio_galeria_archivo_candidato_local(estado,proximo_intento_en,creada_en,archivo_id);
