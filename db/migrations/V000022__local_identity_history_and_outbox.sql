-- Ratified LOCAL-AUTH-01 and local CORE base. All timestamps are supplied by
-- the Backend clock so tests and calendar-month retention are deterministic.
ALTER TABLE public.usuario
    ADD COLUMN preferencia_uso text;
ALTER TABLE public.usuario
    ADD CONSTRAINT usuario_preferencia_uso_ck
    CHECK (preferencia_uso IS NULL OR preferencia_uso IN ('ofrecer', 'arrendar'));

CREATE TABLE public.historial_clave_local (
    id uuid PRIMARY KEY,
    usuario_id uuid NOT NULL REFERENCES public.usuario(id) ON DELETE RESTRICT,
    hash_clave text NOT NULL CHECK (length(hash_clave) BETWEEN 1 AND 200),
    dejo_de_ser_vigente_en timestamptz NOT NULL,
    retirar_en timestamptz NOT NULL,
    creado_en timestamptz NOT NULL,
    CONSTRAINT historial_clave_ventana_ck CHECK (retirar_en > dejo_de_ser_vigente_en)
);
CREATE INDEX historial_clave_usuario_vigente_idx
    ON public.historial_clave_local(usuario_id, retirar_en DESC);

-- A typed local outbox row contains only its aggregate ID and event type. It
-- never stores credentials, tokens, email body, or free-form personal data.
CREATE TABLE public.outbox_evento_local (
    id uuid PRIMARY KEY,
    agregado_tipo text NOT NULL CHECK (length(btrim(agregado_tipo)) BETWEEN 1 AND 40),
    agregado_id uuid NOT NULL,
    tipo text NOT NULL CHECK (tipo ~ '^[a-z][a-z0-9_.-]{2,100}$'),
    clave_deduplicacion text NOT NULL UNIQUE CHECK (length(btrim(clave_deduplicacion)) BETWEEN 1 AND 120),
    version smallint NOT NULL CHECK (version > 0),
    creada_en timestamptz NOT NULL,
    disponible_en timestamptz NOT NULL,
    lease_hasta timestamptz,
    intentos integer NOT NULL DEFAULT 0 CHECK (intentos >= 0),
    entregada_en timestamptz,
    ultimo_error text,
    CONSTRAINT outbox_evento_lease_ck CHECK (lease_hasta IS NULL OR lease_hasta >= creada_en),
    CONSTRAINT outbox_evento_entrega_ck CHECK (entregada_en IS NULL OR entregada_en >= creada_en)
);
CREATE INDEX outbox_evento_local_pendiente_idx
    ON public.outbox_evento_local(disponible_en, creada_en, id)
    WHERE entregada_en IS NULL;

-- Append-only audit facts: application role can insert/read but never update
-- or delete. Retention belongs to these facts alone (five years from event).
CREATE TABLE public.evento_auditoria_local (
    id uuid PRIMARY KEY,
    actor_id uuid NOT NULL REFERENCES public.usuario(id) ON DELETE RESTRICT,
    recurso_tipo text NOT NULL CHECK (length(btrim(recurso_tipo)) BETWEEN 1 AND 40),
    recurso_id uuid NOT NULL,
    accion text NOT NULL CHECK (length(btrim(accion)) BETWEEN 1 AND 60),
    resultado text NOT NULL CHECK (resultado IN ('exito', 'rechazo')),
    motivo_codigo text NOT NULL CHECK (length(btrim(motivo_codigo)) BETWEEN 1 AND 60),
    correlacion_id text NOT NULL CHECK (length(btrim(correlacion_id)) BETWEEN 1 AND 120),
    ocurrido_en timestamptz NOT NULL,
    retirar_en timestamptz NOT NULL,
    CONSTRAINT evento_auditoria_retencion_ck CHECK (retirar_en > ocurrido_en),
    CONSTRAINT evento_auditoria_five_years_ck CHECK (retirar_en = ocurrido_en + interval '5 years')
);
CREATE INDEX evento_auditoria_recurso_idx
    ON public.evento_auditoria_local(recurso_tipo, recurso_id, ocurrido_en, id);
