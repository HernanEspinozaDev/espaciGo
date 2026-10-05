-- M02 local slice: profile and rights intake. No automatic retention/deletion.
CREATE TABLE public.perfil_usuario (
    usuario_id uuid NOT NULL,
    nombre_visible text NOT NULL,
    telefono_normalizado text,
    actualizado_en timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT perfil_usuario_pk PRIMARY KEY (usuario_id),
    CONSTRAINT perfil_usuario_usuario_fk FOREIGN KEY (usuario_id)
        REFERENCES public.usuario (id) ON DELETE RESTRICT,
    CONSTRAINT perfil_usuario_nombre_ck CHECK (
        length(btrim(nombre_visible)) BETWEEN 1 AND 120
        AND nombre_visible ~ '^[[:alpha:] ]+$'
    ),
    CONSTRAINT perfil_usuario_telefono_ck CHECK (
        telefono_normalizado IS NULL OR telefono_normalizado ~ '^[0-9]{9}$'
    )
);

CREATE TABLE public.solicitud_titular (
    id uuid NOT NULL,
    usuario_id uuid NOT NULL,
    tipo text NOT NULL,
    canal text NOT NULL,
    estado text NOT NULL DEFAULT 'en_revision',
    solicitada_en timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT solicitud_titular_pk PRIMARY KEY (id),
    CONSTRAINT solicitud_titular_usuario_fk FOREIGN KEY (usuario_id)
        REFERENCES public.usuario (id) ON DELETE RESTRICT,
    CONSTRAINT solicitud_titular_tipo_ck CHECK (tipo IN ('acceso', 'supresion')),
    CONSTRAINT solicitud_titular_canal_ck CHECK (canal IN ('web', 'api')),
    CONSTRAINT solicitud_titular_estado_ck CHECK (estado IN (
        'recibida', 'identidad_pendiente', 'en_revision', 'resuelta', 'denegada_fundada'
    ))
);
CREATE INDEX solicitud_titular_usuario_fecha_idx
    ON public.solicitud_titular (usuario_id, solicitada_en DESC);
