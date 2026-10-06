-- Local cancellation/refund policy for explicitly enabled synthetic fixtures.
-- Keep this additive: reservations and earlier snapshots remain untouched.
ALTER TABLE public.reserva_ensayo_local_fixture
    ADD COLUMN politica_cancelacion_version text NOT NULL DEFAULT 'local_flexible_v1';

ALTER TABLE public.cotizacion_reserva_ensayo
    ADD COLUMN politica_cancelacion_version text NOT NULL DEFAULT 'local_flexible_v1';

ALTER TABLE public.reserva_ensayo_local
    ADD COLUMN politica_cancelacion_version text NOT NULL DEFAULT 'local_flexible_v1';

ALTER TABLE public.reserva_pago_ensayo
    ADD COLUMN importe_clp bigint NOT NULL DEFAULT 0 CHECK (importe_clp >= 0);

UPDATE public.reserva_pago_ensayo p
SET importe_clp = r.subtotal_clp
FROM public.reserva_ensayo_local r
WHERE r.id = p.reserva_id AND p.resultado = 'exito_simulado';

CREATE TABLE public.reserva_cancelacion_ensayo (
    id uuid PRIMARY KEY,
    reserva_id uuid NOT NULL UNIQUE REFERENCES public.reserva_ensayo_local(id) ON DELETE RESTRICT,
    arrendatario_id uuid NOT NULL REFERENCES public.usuario(id) ON DELETE RESTRICT,
    clave_idempotencia text NOT NULL CHECK (length(btrim(clave_idempotencia)) BETWEEN 1 AND 200),
    huella_solicitud bytea NOT NULL CHECK (octet_length(huella_solicitud) = 32),
    motivo text NOT NULL DEFAULT '',
    creada_en timestamptz NOT NULL,
    UNIQUE (reserva_id, clave_idempotencia)
);

CREATE TABLE public.reserva_devolucion_ensayo (
    id uuid PRIMARY KEY,
    reserva_id uuid NOT NULL UNIQUE REFERENCES public.reserva_ensayo_local(id) ON DELETE RESTRICT,
    operacion_id uuid NOT NULL UNIQUE,
    importe_clp bigint NOT NULL CHECK (importe_clp > 0),
    moneda char(3) NOT NULL CHECK (moneda = 'CLP'),
    estado text NOT NULL CHECK (estado IN ('pendiente', 'completada')),
    ultimo_resultado text CHECK (ultimo_resultado IN ('exito_simulado', 'fallo_simulado', 'sin_respuesta_simulada')),
    creada_en timestamptz NOT NULL,
    actualizada_en timestamptz NOT NULL,
    completada_en timestamptz,
    CHECK ((estado = 'completada') = (completada_en IS NOT NULL))
);

CREATE TABLE public.reserva_devolucion_intento_ensayo (
    id uuid PRIMARY KEY,
    devolucion_id uuid NOT NULL REFERENCES public.reserva_devolucion_ensayo(id) ON DELETE RESTRICT,
    secuencia bigint NOT NULL CHECK (secuencia > 0),
    resultado text NOT NULL CHECK (resultado IN ('exito_simulado', 'fallo_simulado', 'sin_respuesta_simulada')),
    creada_en timestamptz NOT NULL,
    UNIQUE (devolucion_id, secuencia)
);
CREATE INDEX reserva_devolucion_intento_historial_idx
    ON public.reserva_devolucion_intento_ensayo(devolucion_id, secuencia);
