-- M03 local prototype: synthetic references only; no identity documents or RUT.
CREATE TABLE public.verificacion (
    id uuid NOT NULL,
    usuario_id uuid NOT NULL,
    tipo text NOT NULL,
    estado text NOT NULL DEFAULT 'en_revision',
    proveedor_ref text NOT NULL DEFAULT 'local-fixture-v1',
    referencia_evidencia text NOT NULL,
    clave_idempotencia text NOT NULL,
    reintento_de uuid,
    revisor_id uuid,
    motivo_codigo text,
    creada_en timestamptz NOT NULL DEFAULT now(),
    resuelta_en timestamptz,
    CONSTRAINT verificacion_pk PRIMARY KEY (id),
    CONSTRAINT verificacion_usuario_fk FOREIGN KEY (usuario_id)
        REFERENCES public.usuario (id) ON DELETE RESTRICT,
    CONSTRAINT verificacion_reintento_fk FOREIGN KEY (reintento_de)
        REFERENCES public.verificacion (id) ON DELETE RESTRICT,
    CONSTRAINT verificacion_revisor_fk FOREIGN KEY (revisor_id)
        REFERENCES public.usuario (id) ON DELETE RESTRICT,
    CONSTRAINT verificacion_tipo_ck CHECK (tipo IN ('kyc', 'kyb')),
    CONSTRAINT verificacion_estado_ck CHECK (estado IN ('en_revision', 'aprobada', 'rechazada')),
    CONSTRAINT verificacion_proveedor_ck CHECK (proveedor_ref = 'local-fixture-v1'),
    CONSTRAINT verificacion_ref_sintetica_ck CHECK (referencia_evidencia ~ '^fixture:[0-9a-f-]{36}$'),
    CONSTRAINT verificacion_idempotencia_ck CHECK (length(clave_idempotencia) BETWEEN 8 AND 80),
    CONSTRAINT verificacion_motivo_ck CHECK (
        (estado = 'rechazada' AND revisor_id IS NOT NULL AND motivo_codigo IS NOT NULL AND resuelta_en IS NOT NULL)
        OR (estado = 'aprobada' AND revisor_id IS NOT NULL AND motivo_codigo IS NULL AND resuelta_en IS NOT NULL)
        OR (estado = 'en_revision' AND revisor_id IS NULL AND motivo_codigo IS NULL AND resuelta_en IS NULL)
    ),
    CONSTRAINT verificacion_reintento_no_self_ck CHECK (reintento_de IS NULL OR reintento_de <> id),
    CONSTRAINT verificacion_idempotencia_uk UNIQUE (usuario_id, clave_idempotencia)
);
CREATE INDEX verificacion_usuario_fecha_idx ON public.verificacion (usuario_id, creada_en DESC);
CREATE INDEX verificacion_revision_idx ON public.verificacion (creada_en) WHERE estado = 'en_revision';
CREATE UNIQUE INDEX verificacion_reintento_unico_idx ON public.verificacion (reintento_de) WHERE reintento_de IS NOT NULL;
