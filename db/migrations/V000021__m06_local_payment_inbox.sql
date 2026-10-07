-- Durable payment intent and authenticated event inbox for the local fake.
-- This does not model a production provider or move funds.
CREATE TABLE public.reserva_pago_ensayo_operacion (
    id uuid PRIMARY KEY,
    reserva_id uuid NOT NULL REFERENCES public.reserva_ensayo_local(id) ON DELETE RESTRICT,
    arrendatario_id uuid NOT NULL REFERENCES public.usuario(id) ON DELETE RESTRICT,
    clave_idempotencia text NOT NULL CHECK (length(btrim(clave_idempotencia)) BETWEEN 1 AND 200),
    huella_solicitud bytea NOT NULL CHECK (octet_length(huella_solicitud) = 32),
    resultado_solicitado text NOT NULL CHECK (resultado_solicitado IN ('exito','rechazo','sin_respuesta')),
    estado text NOT NULL CHECK (estado IN ('pendiente','aplicada','vencida')),
    resultado_final text CHECK (resultado_final IN ('exito_simulado','rechazo_simulado')),
    creada_en timestamptz NOT NULL,
    actualizada_en timestamptz NOT NULL,
    UNIQUE (reserva_id, clave_idempotencia),
    CHECK ((estado = 'aplicada') = (resultado_final IS NOT NULL))
);
CREATE UNIQUE INDEX reserva_pago_ensayo_operacion_pendiente_uq
    ON public.reserva_pago_ensayo_operacion(reserva_id)
    WHERE estado = 'pendiente';
CREATE INDEX reserva_pago_ensayo_operacion_pendiente_idx
    ON public.reserva_pago_ensayo_operacion(creada_en, id)
    WHERE estado = 'pendiente';

-- Only signature-verified events are inserted here. Event contents are
-- immutable; delivery/replay state lives in the separate processing table.
CREATE TABLE public.reserva_pago_evento_ensayo (
    id uuid PRIMARY KEY,
    operacion_id uuid NOT NULL REFERENCES public.reserva_pago_ensayo_operacion(id) ON DELETE RESTRICT,
    proveedor_evento_id text NOT NULL CHECK (length(btrim(proveedor_evento_id)) BETWEEN 1 AND 200),
    resultado text NOT NULL CHECK (resultado IN ('exito_simulado','rechazo_simulado')),
    huella_payload bytea NOT NULL CHECK (octet_length(huella_payload) = 32),
    autenticado_en timestamptz NOT NULL,
    recibido_en timestamptz NOT NULL,
    UNIQUE (proveedor_evento_id),
    UNIQUE (operacion_id, proveedor_evento_id)
);
CREATE INDEX reserva_pago_evento_ensayo_operacion_idx
    ON public.reserva_pago_evento_ensayo(operacion_id, recibido_en, id);

CREATE TABLE public.reserva_pago_evento_aplicacion_ensayo (
    evento_id uuid PRIMARY KEY REFERENCES public.reserva_pago_evento_ensayo(id) ON DELETE RESTRICT,
    estado text NOT NULL CHECK (estado IN ('pendiente','aplicada','ignorada','vencida')),
    codigo_resultado text CHECK (codigo_resultado IN ('aplicado','operacion_terminal','reserva_vencida')),
    procesado_en timestamptz,
    creada_en timestamptz NOT NULL,
    CHECK ((estado = 'pendiente') = (procesado_en IS NULL))
);
CREATE INDEX reserva_pago_evento_aplicacion_pendiente_idx
    ON public.reserva_pago_evento_aplicacion_ensayo(creada_en, evento_id)
    WHERE estado = 'pendiente';
