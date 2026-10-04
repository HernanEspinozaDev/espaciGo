-- AUTH-DB-02: six M01 persistence tables only.
-- Email validation/canonicalization is performed by the application; PostgreSQL
-- enforces uniqueness of the already-derived key.

CREATE TABLE public.usuario (
    id uuid NOT NULL,
    correo_original text NOT NULL,
    correo_normalizado text NOT NULL,
    hash_clave text NOT NULL,
    estado text NOT NULL DEFAULT 'correo_pendiente',
    creado_en timestamptz NOT NULL DEFAULT now(),
    actualizado_en timestamptz NOT NULL DEFAULT now(),
    baja_solicitada_en timestamptz,
    intentos_fallidos_consecutivos integer NOT NULL DEFAULT 0,
    bloqueado_hasta timestamptz,
    CONSTRAINT usuario_pk PRIMARY KEY (id),
    CONSTRAINT usuario_correo_normalizado_uk UNIQUE (correo_normalizado),
    CONSTRAINT usuario_estado_ck CHECK (estado IN (
        'correo_pendiente', 'activo', 'bloqueado', 'baja_solicitada', 'desidentificado'
    )),
    CONSTRAINT usuario_intentos_fallidos_ck CHECK (intentos_fallidos_consecutivos >= 0)
);

CREATE TABLE public.rol_usuario (
    usuario_id uuid NOT NULL,
    rol text NOT NULL,
    concedido_en timestamptz NOT NULL DEFAULT now(),
    concedido_por uuid,
    CONSTRAINT rol_usuario_pk PRIMARY KEY (usuario_id, rol),
    CONSTRAINT rol_usuario_usuario_fk FOREIGN KEY (usuario_id)
        REFERENCES public.usuario (id) ON DELETE RESTRICT,
    CONSTRAINT rol_usuario_concedido_por_fk FOREIGN KEY (concedido_por)
        REFERENCES public.usuario (id) ON DELETE RESTRICT,
    CONSTRAINT rol_usuario_rol_ck CHECK (rol IN ('arrendatario', 'arrendador', 'administrador'))
);
CREATE INDEX rol_usuario_concedido_por_idx
    ON public.rol_usuario (concedido_por) WHERE concedido_por IS NOT NULL;

CREATE TABLE public.sesion (
    id uuid NOT NULL,
    usuario_id uuid NOT NULL,
    token_hash char(64) NOT NULL,
    creada_en timestamptz NOT NULL DEFAULT now(),
    ultima_actividad_en timestamptz NOT NULL DEFAULT now(),
    expira_en timestamptz NOT NULL,
    revocada_en timestamptz,
    cliente_resumen text,
    CONSTRAINT sesion_pk PRIMARY KEY (id),
    CONSTRAINT sesion_usuario_fk FOREIGN KEY (usuario_id)
        REFERENCES public.usuario (id) ON DELETE RESTRICT,
    CONSTRAINT sesion_token_hash_uk UNIQUE (token_hash),
    CONSTRAINT sesion_token_hash_ck CHECK (token_hash ~ '^[0-9a-f]{64}$'),
    CONSTRAINT sesion_creacion_expiracion_ck CHECK (
        expira_en > creada_en AND expira_en <= creada_en + interval '8 hours'
    ),
    CONSTRAINT sesion_actividad_ck CHECK (
        ultima_actividad_en >= creada_en AND ultima_actividad_en <= expira_en
    ),
    CONSTRAINT sesion_revocacion_ck CHECK (revocada_en IS NULL OR revocada_en >= creada_en)
);
CREATE INDEX sesion_usuario_activa_idx
    ON public.sesion (usuario_id, expira_en) WHERE revocada_en IS NULL;

CREATE TABLE public.token_accion (
    id uuid NOT NULL,
    usuario_id uuid NOT NULL,
    proposito text NOT NULL,
    token_hash char(64) NOT NULL,
    creado_en timestamptz NOT NULL DEFAULT now(),
    expira_en timestamptz NOT NULL,
    consumido_en timestamptz,
    invalidado_en timestamptz,
    intentos integer NOT NULL DEFAULT 0,
    CONSTRAINT token_accion_pk PRIMARY KEY (id),
    CONSTRAINT token_accion_usuario_fk FOREIGN KEY (usuario_id)
        REFERENCES public.usuario (id) ON DELETE RESTRICT,
    CONSTRAINT token_accion_token_hash_uk UNIQUE (token_hash),
    CONSTRAINT token_accion_token_hash_ck CHECK (token_hash ~ '^[0-9a-f]{64}$'),
    CONSTRAINT token_accion_proposito_ck CHECK (proposito IN ('verificar_correo', 'recuperar_clave')),
    CONSTRAINT token_accion_intentos_ck CHECK (intentos BETWEEN 0 AND 5),
    CONSTRAINT token_accion_creacion_expiracion_ck CHECK (expira_en > creado_en),
    CONSTRAINT token_accion_consumido_ck CHECK (consumido_en IS NULL OR consumido_en >= creado_en),
    CONSTRAINT token_accion_invalidado_ck CHECK (invalidado_en IS NULL OR invalidado_en >= creado_en),
    CONSTRAINT token_accion_terminal_mutuo_ck CHECK (consumido_en IS NULL OR invalidado_en IS NULL)
);
CREATE INDEX token_accion_usuario_pendiente_idx
    ON public.token_accion (usuario_id, proposito, expira_en)
    WHERE consumido_en IS NULL AND invalidado_en IS NULL;
CREATE INDEX token_accion_usuario_emision_idx
    ON public.token_accion (usuario_id, proposito, creado_en);

CREATE TABLE public.version_terminos (
    id uuid NOT NULL,
    codigo text NOT NULL,
    tipo text NOT NULL,
    hash_sha256 char(64) NOT NULL,
    publicada_en timestamptz NOT NULL,
    CONSTRAINT version_terminos_pk PRIMARY KEY (id),
    CONSTRAINT version_terminos_codigo_uk UNIQUE (codigo),
    CONSTRAINT version_terminos_tipo_ck CHECK (tipo IN ('terminos', 'privacidad', 'politica_arrendador')),
    CONSTRAINT version_terminos_hash_ck CHECK (hash_sha256 ~ '^[0-9a-f]{64}$')
);

CREATE TABLE public.aceptacion_terminos (
    id uuid NOT NULL,
    usuario_id uuid NOT NULL,
    version_id uuid NOT NULL,
    aceptada_en timestamptz NOT NULL DEFAULT now(),
    canal text NOT NULL,
    CONSTRAINT aceptacion_terminos_pk PRIMARY KEY (id),
    CONSTRAINT aceptacion_terminos_usuario_fk FOREIGN KEY (usuario_id)
        REFERENCES public.usuario (id) ON DELETE RESTRICT,
    CONSTRAINT aceptacion_terminos_version_fk FOREIGN KEY (version_id)
        REFERENCES public.version_terminos (id) ON DELETE RESTRICT,
    CONSTRAINT aceptacion_terminos_usuario_version_uk UNIQUE (usuario_id, version_id),
    CONSTRAINT aceptacion_terminos_canal_ck CHECK (canal IN ('web', 'api', 'administrado'))
);
CREATE INDEX aceptacion_terminos_version_idx
    ON public.aceptacion_terminos (version_id);

-- Synthetic test catalog only; these hashes cover the exact fixture labels below,
-- not legal text and not production terms.
-- AUTH-DB-02 synthetic terms fixture v0.1; not legal text.
-- AUTH-DB-02 synthetic privacy fixture v0.1; not legal text.
-- AUTH-DB-02 synthetic landlord policy fixture v0.1; not legal text.
INSERT INTO public.version_terminos (id, codigo, tipo, hash_sha256, publicada_en) VALUES
    ('00000000-0000-4000-8000-000000000001', 'sintetico-terminos-v0.1', 'terminos',
     '1519668e1eaa6612ba184a1573db9fa7b95f61d3aab410b4f02028411ec483f2', '2026-10-04T00:00:00Z'),
    ('00000000-0000-4000-8000-000000000002', 'sintetico-privacidad-v0.1', 'privacidad',
     'e443086352ba2e1180afdf8521c03565769a583160775d416d807df0df8bcad3', '2026-10-04T00:00:00Z'),
    ('00000000-0000-4000-8000-000000000003', 'sintetico-politica-arrendador-v0.1', 'politica_arrendador',
     'cc1690d0d0088a9cd2f1d0ec588862f7c28dfe916350243336340776df290438', '2026-10-04T00:00:00Z');
