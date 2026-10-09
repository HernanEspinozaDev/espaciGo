-- LOCAL-M04-GALLERY-01: fixed synthetic images in private local storage only.
ALTER TABLE public.espacio ADD CONSTRAINT espacio_owner_id_uq UNIQUE (id,propietario_id);

CREATE TABLE public.espacio_galeria_sintetica_local (
    id uuid PRIMARY KEY,
    espacio_id uuid NOT NULL,
    propietario_id uuid NOT NULL,
    archivo_id uuid UNIQUE,
    fixture_code text NOT NULL CHECK (fixture_code='synthetic-png-v1'),
    mime_type text NOT NULL CHECK (mime_type='image/png'),
    sha256 char(64) NOT NULL CHECK (sha256 ~ '^[0-9a-f]{64}$'),
    size_bytes bigint NOT NULL CHECK (size_bytes BETWEEN 1 AND 1048576),
    estado text NOT NULL CHECK (estado IN ('activa','retirada','retirada_baja')),
    creada_en timestamptz NOT NULL,
    retirada_en timestamptz,
    proximo_intento_en timestamptz,
    limpia_en timestamptz,
    intentos_limpieza integer NOT NULL DEFAULT 0 CHECK (intentos_limpieza >= 0),
    ultimo_codigo_error text,
    clave_idempotencia text NOT NULL CHECK (length(btrim(clave_idempotencia)) BETWEEN 8 AND 128),
    CONSTRAINT espacio_galeria_sintetica_owner_fk FOREIGN KEY (espacio_id,propietario_id)
        REFERENCES public.espacio(id,propietario_id) ON DELETE RESTRICT,
    CONSTRAINT espacio_galeria_sintetica_file_ck CHECK ((archivo_id IS NULL) = (limpia_en IS NOT NULL)),
    CONSTRAINT espacio_galeria_sintetica_state_ck CHECK (
        (estado='activa' AND retirada_en IS NULL)
        OR (estado IN ('retirada','retirada_baja') AND retirada_en IS NOT NULL)
    ),
    CONSTRAINT espacio_galeria_sintetica_cleanup_ck CHECK (
        (estado='activa' AND proximo_intento_en IS NULL)
        OR (estado<>'activa' AND (archivo_id IS NULL OR proximo_intento_en IS NOT NULL))
    ),
    UNIQUE (espacio_id,propietario_id,clave_idempotencia)
);
CREATE INDEX espacio_galeria_activa_orden_idx
    ON public.espacio_galeria_sintetica_local(espacio_id,creada_en,id) WHERE estado='activa';
CREATE INDEX espacio_galeria_cleanup_idx
    ON public.espacio_galeria_sintetica_local(proximo_intento_en,id)
    WHERE estado<>'activa' AND archivo_id IS NOT NULL;

-- The API owns listing and deleting each row; history is retained locally and
-- is never exposed as active gallery content after withdrawal.
