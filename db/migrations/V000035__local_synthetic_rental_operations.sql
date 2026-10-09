-- LOCAL-OPS-01: synthetic check-in/out and host receipt records.
-- Operations, evidence and booking transitions are append-only facts. The
-- reservation remains the sole owner of occupied time; no calendar is added.
ALTER TABLE public.reserva_ensayo_local
    DROP CONSTRAINT reserva_ensayo_local_estado_check,
    ADD CONSTRAINT reserva_ensayo_local_estado_check CHECK (estado IN (
        'pendiente_de_pago','pagada','aprobada_host','firma_parcial','lista_para_checkin',
        'en_curso','finalizada','en_disputa',
        'cancelada_por_pago','rechazada_arrendador','vencida_pago','vencida_host',
        'cancelada_arrendatario','cancelada_por_firma'
    ));

CREATE TABLE public.operacion_arriendo_ensayo_local (
    id uuid PRIMARY KEY,
    reserva_id uuid NOT NULL REFERENCES public.reserva_ensayo_local(id) ON DELETE RESTRICT,
    tipo text NOT NULL CHECK (tipo IN ('checkin','checkout','recepcion')),
    actor_id uuid NOT NULL REFERENCES public.usuario(id) ON DELETE RESTRICT,
    ocurrio_en timestamptz NOT NULL,
    zona_horaria text NOT NULL CHECK (length(btrim(zona_horaria)) BETWEEN 1 AND 100),
    ubicacion_sintetica jsonb NOT NULL CHECK (
        jsonb_typeof(ubicacion_sintetica)='object'
        AND ubicacion_sintetica->>'source'='synthetic-fixture-v1'
        AND ubicacion_sintetica->>'location_code'='santiago-demo-center-v1'
        AND (ubicacion_sintetica->>'latitude')::double precision BETWEEN -90 AND 90
        AND (ubicacion_sintetica->>'longitude')::double precision BETWEEN -180 AND 180
    ),
    comentarios text NOT NULL DEFAULT '',
    observacion text NOT NULL DEFAULT '',
    resultado text NOT NULL CHECK (resultado IN ('registrada','recepcion_conforme','recepcion_con_observaciones')),
    clave_idempotencia text NOT NULL CHECK (length(btrim(clave_idempotencia)) BETWEEN 8 AND 200),
    huella_solicitud bytea NOT NULL CHECK (octet_length(huella_solicitud)=32),
    creada_en timestamptz NOT NULL,
    UNIQUE (reserva_id,tipo),
    UNIQUE (reserva_id,tipo,clave_idempotencia),
    CHECK ((tipo='recepcion' AND resultado IN ('recepcion_conforme','recepcion_con_observaciones'))
        OR (tipo IN ('checkin','checkout') AND resultado='registrada')),
    CHECK ((tipo IN ('checkin','checkout') OR comentarios='') AND (tipo='recepcion' OR observacion=''))
);
CREATE INDEX operacion_arriendo_reserva_hist_idx
    ON public.operacion_arriendo_ensayo_local(reserva_id,ocurrio_en,id);

CREATE TABLE public.operacion_arriendo_evidencia_ensayo_local (
    id uuid PRIMARY KEY,
    operacion_id uuid NOT NULL REFERENCES public.operacion_arriendo_ensayo_local(id) ON DELETE RESTRICT,
    fixture_code text NOT NULL CHECK (fixture_code='synthetic-png-v1'),
    mime_type text NOT NULL CHECK (mime_type='image/png'),
    sha256 char(64) NOT NULL CHECK (sha256 ~ '^[0-9a-f]{64}$'),
    size_bytes bigint NOT NULL CHECK (size_bytes BETWEEN 1 AND 1048576),
    creada_en timestamptz NOT NULL
);
CREATE INDEX operacion_arriendo_evidencia_operacion_idx
    ON public.operacion_arriendo_evidencia_ensayo_local(operacion_id,id);

CREATE TABLE public.operacion_arriendo_archivo_candidato_local (
    archivo_id uuid PRIMARY KEY,
    reserva_id uuid NOT NULL REFERENCES public.reserva_ensayo_local(id) ON DELETE RESTRICT,
    actor_id uuid NOT NULL REFERENCES public.usuario(id) ON DELETE RESTRICT,
    tipo text NOT NULL CHECK (tipo IN ('checkin','checkout','recepcion')),
    estado text NOT NULL CHECK (estado IN ('reservado','pendiente_limpieza','limpiando')),
    creada_en timestamptz NOT NULL,
    proximo_intento_en timestamptz NOT NULL,
    lease_iniciado_en timestamptz,
    intentos_limpieza integer NOT NULL DEFAULT 0 CHECK (intentos_limpieza>=0),
    ultimo_codigo_error text
);
CREATE INDEX operacion_arriendo_candidato_cleanup_idx
    ON public.operacion_arriendo_archivo_candidato_local(estado,proximo_intento_en,archivo_id);

CREATE TABLE public.operacion_arriendo_historial_ensayo_local (
    secuencia bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    reserva_id uuid NOT NULL REFERENCES public.reserva_ensayo_local(id) ON DELETE RESTRICT,
    operacion_id uuid NOT NULL REFERENCES public.operacion_arriendo_ensayo_local(id) ON DELETE RESTRICT,
    accion text NOT NULL CHECK (accion IN ('checkin_registrado','checkout_registrado','recepcion_confirmada','observacion_registrada')),
    actor_id uuid NOT NULL REFERENCES public.usuario(id) ON DELETE RESTRICT,
    ocurrida_en timestamptz NOT NULL,
    UNIQUE (operacion_id)
);
CREATE INDEX operacion_arriendo_historial_reserva_idx
    ON public.operacion_arriendo_historial_ensayo_local(reserva_id,secuencia);

-- A formal synthetic damage claim is M10-owned and distinct from the M02
-- privacy-blocker incident and from M08 receipt observations. This slice only
-- records the claim/defense and transitions the reservation to en_disputa; it
-- does not adjudicate damage, hold money, or execute a payout/refund.
CREATE TABLE public.reclamo_dano_ensayo_local (
    id uuid PRIMARY KEY,
    reserva_id uuid NOT NULL UNIQUE REFERENCES public.reserva_ensayo_local(id) ON DELETE RESTRICT,
    anfitrion_id uuid NOT NULL REFERENCES public.usuario(id) ON DELETE RESTRICT,
    arrendatario_id uuid NOT NULL REFERENCES public.usuario(id) ON DELETE RESTRICT,
    checkout_operacion_id uuid NOT NULL REFERENCES public.operacion_arriendo_ensayo_local(id) ON DELETE RESTRICT,
    checkout_evidencia_id uuid NOT NULL REFERENCES public.operacion_arriendo_evidencia_ensayo_local(id) ON DELETE RESTRICT,
    descripcion text NOT NULL CHECK (length(btrim(descripcion))>0),
    estado text NOT NULL CHECK (estado='abierto'),
    clave_idempotencia text NOT NULL CHECK (length(btrim(clave_idempotencia)) BETWEEN 8 AND 200),
    huella_solicitud bytea NOT NULL CHECK (octet_length(huella_solicitud)=32),
    abierto_en timestamptz NOT NULL,
    plazo_reclamo_hasta timestamptz NOT NULL,
    CHECK (anfitrion_id <> arrendatario_id),
    CHECK (plazo_reclamo_hasta > abierto_en),
    UNIQUE(anfitrion_id,clave_idempotencia)
);
CREATE TABLE public.reclamo_dano_descargo_ensayo_local (
    id uuid PRIMARY KEY,
    reclamo_id uuid NOT NULL UNIQUE REFERENCES public.reclamo_dano_ensayo_local(id) ON DELETE RESTRICT,
    actor_id uuid NOT NULL REFERENCES public.usuario(id) ON DELETE RESTRICT,
    descripcion text NOT NULL CHECK (length(btrim(descripcion))>0),
    clave_idempotencia text NOT NULL CHECK (length(btrim(clave_idempotencia)) BETWEEN 8 AND 200),
    huella_solicitud bytea NOT NULL CHECK (octet_length(huella_solicitud)=32),
    creado_en timestamptz NOT NULL,
    UNIQUE(reclamo_id,clave_idempotencia)
);
CREATE TABLE public.reclamo_dano_historial_ensayo_local (
    secuencia bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    reclamo_id uuid NOT NULL REFERENCES public.reclamo_dano_ensayo_local(id) ON DELETE RESTRICT,
    accion text NOT NULL CHECK (accion IN ('reclamo_abierto','descargo_registrado')),
    actor_id uuid NOT NULL REFERENCES public.usuario(id) ON DELETE RESTRICT,
    ocurrida_en timestamptz NOT NULL
);
CREATE INDEX reclamo_dano_historial_idx ON public.reclamo_dano_historial_ensayo_local(reclamo_id,secuencia);

-- Active reservations and an open formal M10 damage claim are privacy
-- obligations. A terminal reservation alone is historical; a claim remains
-- an obligation until its later M10 resolution.
ALTER TABLE public.evento_auditoria_local
    DROP CONSTRAINT evento_auditoria_codigos_ck;
ALTER TABLE public.evento_auditoria_local
    ADD CONSTRAINT evento_auditoria_codigos_ck CHECK (
        jsonb_typeof(detalle_codigos)='object'
        AND detalle_codigos ? 'obligations_detected'
        AND detalle_codigos ? 'pending_checks'
        AND (detalle_codigos - ARRAY['obligations_detected','pending_checks']::text[])='{}'::jsonb
        AND jsonb_typeof(detalle_codigos->'obligations_detected')='array'
        AND jsonb_typeof(detalle_codigos->'pending_checks')='array'
        AND (detalle_codigos->'obligations_detected') <@ ' ["reserva_activa","pago_o_devolucion_pendiente","disputa_abierta"]'::jsonb
        AND (detalle_codigos->'pending_checks') <@ '["fuente_disputas_no_modelada","matriz_retencion_historicos_incompleta"]'::jsonb
    );
