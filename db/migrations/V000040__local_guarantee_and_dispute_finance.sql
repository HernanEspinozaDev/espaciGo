-- Synthetic-only guarantee snapshots and durable local financial operations.
-- Existing reservations and quotes remain NULL; no retroactive guarantee is assigned.
ALTER TABLE public.cotizacion_reserva_ensayo
    ADD COLUMN garantia_politica_version text,
    ADD COLUMN garantia_moneda char(3),
    ADD COLUMN garantia_prevista_clp numeric(14,0),
    ADD CONSTRAINT cotizacion_reserva_garantia_snapshot_ck CHECK (
        (garantia_politica_version IS NULL AND garantia_moneda IS NULL AND garantia_prevista_clp IS NULL)
        OR (garantia_politica_version = 'garantia_local_fija_v1' AND garantia_moneda = 'CLP' AND garantia_prevista_clp = 50000)
    );

ALTER TABLE public.reserva_ensayo_local
    ADD COLUMN garantia_politica_version text,
    ADD COLUMN garantia_moneda char(3),
    ADD COLUMN garantia_prevista_clp numeric(14,0),
    ADD CONSTRAINT reserva_ensayo_garantia_snapshot_ck CHECK (
        (garantia_politica_version IS NULL AND garantia_moneda IS NULL AND garantia_prevista_clp IS NULL)
        OR (garantia_politica_version = 'garantia_local_fija_v1' AND garantia_moneda = 'CLP' AND garantia_prevista_clp = 50000)
    );

ALTER TABLE public.reserva_ensayo_local
    DROP CONSTRAINT reserva_ensayo_local_estado_check,
    ADD CONSTRAINT reserva_ensayo_local_estado_check CHECK (estado IN (
        'pendiente_de_pago','pagada','aprobada_host','firma_parcial','lista_para_checkin',
        'en_curso','finalizada','en_disputa','cancelada_por_pago','rechazada_arrendador',
        'vencida_pago','vencida_host','cancelada_arrendatario','cancelada_por_firma'
    ));

CREATE TABLE public.reserva_garantia_ensayo_local (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    reserva_id uuid NOT NULL UNIQUE REFERENCES public.reserva_ensayo_local(id) ON DELETE RESTRICT,
    politica_version text NOT NULL CHECK (politica_version = 'garantia_local_fija_v1'),
    moneda char(3) NOT NULL CHECK (moneda = 'CLP'),
    previsto_clp numeric(14,0) NOT NULL CHECK (previsto_clp = 50000),
    autorizado_clp numeric(14,0) NOT NULL DEFAULT 0 CHECK (autorizado_clp >= 0),
    capturado_clp numeric(14,0) NOT NULL DEFAULT 0 CHECK (capturado_clp >= 0),
    liberado_clp numeric(14,0) NOT NULL DEFAULT 0 CHECK (liberado_clp >= 0),
    estado text NOT NULL CHECK (estado IN (
        'pendiente_pago','pendiente_autorizacion','autorizada','no_disponible','por_conciliar',
        'captura_pendiente','captura_por_conciliar','parcialmente_capturada',
        'liberacion_pendiente','liberacion_por_conciliar','liberada','cancelada'
    )),
    creada_en timestamptz NOT NULL,
    actualizada_en timestamptz NOT NULL,
    CHECK (autorizado_clp <= previsto_clp),
    CHECK (capturado_clp + liberado_clp <= autorizado_clp)
);

CREATE TABLE public.reserva_garantia_operacion_ensayo_local (
    id uuid PRIMARY KEY,
    garantia_id uuid NOT NULL REFERENCES public.reserva_garantia_ensayo_local(id) ON DELETE RESTRICT,
    tipo text NOT NULL CHECK (tipo IN ('autorizacion','captura','liberacion')),
    clave_idempotencia text NOT NULL CHECK (length(clave_idempotencia) BETWEEN 1 AND 200),
    huella_solicitud bytea NOT NULL CHECK (octet_length(huella_solicitud) = 32),
    importe_clp numeric(14,0) NOT NULL CHECK (importe_clp >= 0),
    resultado_solicitado text NOT NULL CHECK (resultado_solicitado IN ('exito','rechazo','sin_respuesta')),
    estado text NOT NULL CHECK (estado IN ('pendiente','por_conciliar','confirmada','rechazada','vencida')),
    primer_intento_en timestamptz NOT NULL,
    vence_en timestamptz,
    ultimo_resultado text,
    completada_en timestamptz,
    creada_en timestamptz NOT NULL,
    actualizada_en timestamptz NOT NULL,
    UNIQUE (garantia_id, tipo),
    UNIQUE (garantia_id, clave_idempotencia),
    CHECK (ultimo_resultado IS NULL OR ultimo_resultado IN ('exito_simulado','rechazo_simulado','sin_respuesta_simulada','resultado_tardio','autorizacion_vencida','cancelada'))
);

CREATE TABLE public.reserva_garantia_resultado_fake_ensayo_local (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    operacion_id uuid NOT NULL UNIQUE REFERENCES public.reserva_garantia_operacion_ensayo_local(id) ON DELETE RESTRICT,
    evento_id text UNIQUE CHECK (evento_id IS NULL OR length(evento_id) BETWEEN 1 AND 200),
    resultado text NOT NULL CHECK (resultado IN ('exito_simulado','rechazo_simulado','sin_respuesta_simulada')),
    registrada_en timestamptz NOT NULL,
    CHECK ((resultado='sin_respuesta_simulada' AND evento_id IS NULL) OR
           (resultado IN ('exito_simulado','rechazo_simulado') AND evento_id IS NOT NULL))
);

CREATE TABLE public.reserva_garantia_evento_ensayo_local (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    operacion_id uuid NOT NULL REFERENCES public.reserva_garantia_operacion_ensayo_local(id) ON DELETE RESTRICT,
    evento_id text NOT NULL UNIQUE CHECK (length(evento_id) BETWEEN 1 AND 200),
    resultado text NOT NULL CHECK (resultado IN ('exito_simulado','rechazo_simulado')),
    autenticado_en timestamptz NOT NULL,
    UNIQUE (operacion_id, evento_id)
);

CREATE TABLE public.reserva_garantia_evento_aplicacion_ensayo_local (
    evento_id uuid PRIMARY KEY REFERENCES public.reserva_garantia_evento_ensayo_local(id) ON DELETE RESTRICT,
    estado text NOT NULL CHECK (estado IN ('pendiente','aplicada','por_conciliar','ignorada')),
    codigo_resultado text,
    creada_en timestamptz NOT NULL,
    procesada_en timestamptz
);

CREATE TABLE public.reserva_decision_financiera_ensayo_local (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    garantia_id uuid NOT NULL UNIQUE REFERENCES public.reserva_garantia_ensayo_local(id) ON DELETE RESTRICT,
    reserva_id uuid NOT NULL REFERENCES public.reserva_ensayo_local(id) ON DELETE RESTRICT,
    reclamo_id uuid REFERENCES public.reclamo_dano_ensayo_local(id) ON DELETE RESTRICT,
    resultado_reclamo text NOT NULL CHECK (resultado_reclamo IN ('acogido','rechazado')),
    deduccion_clp numeric(14,0) NOT NULL CHECK (deduccion_clp >= 0),
    motivo_codigo text NOT NULL CHECK (motivo_codigo IN ('dano_acreditado','faltante_acreditado','sin_deduccion')),
    evidencia_id uuid REFERENCES public.operacion_arriendo_evidencia_ensayo_local(id) ON DELETE RESTRICT,
    administrador_id uuid NOT NULL REFERENCES public.usuario(id) ON DELETE RESTRICT,
    clave_idempotencia text NOT NULL CHECK (length(clave_idempotencia) BETWEEN 1 AND 200),
    huella_solicitud bytea NOT NULL CHECK (octet_length(huella_solicitud) = 32),
    estado text NOT NULL CHECK (estado IN ('pendiente','por_conciliar','aplicada','sin_deduccion')),
    creada_en timestamptz NOT NULL,
    actualizada_en timestamptz NOT NULL,
    UNIQUE (reserva_id, clave_idempotencia),
    CHECK ((deduccion_clp = 0 AND motivo_codigo = 'sin_deduccion' AND evidencia_id IS NULL)
        OR (deduccion_clp > 0 AND motivo_codigo IN ('dano_acreditado','faltante_acreditado') AND evidencia_id IS NOT NULL)),
    CHECK (reclamo_id IS NOT NULL),
    CHECK (resultado_reclamo <> 'rechazado' OR deduccion_clp = 0)
);

CREATE TABLE public.reserva_finanzas_historial_ensayo_local (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    secuencia bigserial NOT NULL UNIQUE,
    reserva_id uuid NOT NULL REFERENCES public.reserva_ensayo_local(id) ON DELETE RESTRICT,
    tipo text NOT NULL CHECK (tipo IN ('garantia_solicitada','garantia_autorizada','garantia_rechazada','garantia_vencida','garantia_liberada','garantia_capturada','decision_financiera_registrada','operacion_pendiente_conciliacion')),
    actor_id uuid REFERENCES public.usuario(id) ON DELETE RESTRICT,
    detalle_codigo text NOT NULL,
    ocurrida_en timestamptz NOT NULL
);

CREATE INDEX reserva_garantia_estado_idx ON public.reserva_garantia_ensayo_local(estado,actualizada_en);
CREATE INDEX reserva_garantia_operacion_pendiente_idx ON public.reserva_garantia_operacion_ensayo_local(estado,vence_en) WHERE estado IN ('pendiente','por_conciliar');
CREATE INDEX reserva_garantia_evento_pendiente_idx ON public.reserva_garantia_evento_aplicacion_ensayo_local(estado,creada_en) WHERE estado='pendiente';

COMMENT ON TABLE public.reserva_garantia_ensayo_local IS 'Obligación fake independiente del pago del arriendo; no representa dinero ni custodia real.';
COMMENT ON TABLE public.reserva_decision_financiera_ensayo_local IS 'Decisión económica local inmutable y separada de la resolución histórica del reclamo.';

