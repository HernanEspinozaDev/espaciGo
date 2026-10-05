-- Local-only M03 prototype metadata. The blob itself is stored outside the repo
-- and is always generated from the fixed synthetic fixture by the API.
CREATE TABLE public.verificacion_evidencia_sintetica (
    id uuid NOT NULL,
    verificacion_id uuid NOT NULL,
    codigo_fixture text NOT NULL,
    mime_type text NOT NULL,
    tamano_bytes bigint NOT NULL,
    sha256 char(64) NOT NULL,
    creada_en timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT verificacion_evidencia_sintetica_pk PRIMARY KEY (id),
    CONSTRAINT verificacion_evidencia_verificacion_fk FOREIGN KEY (verificacion_id)
        REFERENCES public.verificacion (id) ON DELETE RESTRICT,
    CONSTRAINT verificacion_evidencia_fixture_ck CHECK (codigo_fixture = 'synthetic-png-v1'),
    CONSTRAINT verificacion_evidencia_mime_ck CHECK (mime_type = 'image/png'),
    CONSTRAINT verificacion_evidencia_size_ck CHECK (tamano_bytes BETWEEN 1 AND 1048576),
    CONSTRAINT verificacion_evidencia_sha_ck CHECK (sha256 ~ '^[0-9a-f]{64}$')
);
CREATE INDEX verificacion_evidencia_case_idx ON public.verificacion_evidencia_sintetica (verificacion_id, creada_en DESC);
