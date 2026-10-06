-- M06 local-only vertical slice. The allowlist identifies exactly one
-- synthetic draft and its two authorized participants; it is never searchable.
CREATE TABLE public.reserva_ensayo_local_fixture (
    singleton boolean PRIMARY KEY DEFAULT true CHECK (singleton),
    espacio_id uuid NOT NULL UNIQUE REFERENCES public.espacio(id) ON DELETE RESTRICT,
    anfitrion_id uuid NOT NULL REFERENCES public.usuario(id) ON DELETE RESTRICT,
    arrendatario_id uuid NOT NULL REFERENCES public.usuario(id) ON DELETE RESTRICT,
    habilitada_en timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT fixture_participantes_distintos_ck CHECK (anfitrion_id <> arrendatario_id)
);

CREATE TABLE public.cotizacion_reserva_ensayo (
    id uuid PRIMARY KEY,
    espacio_id uuid NOT NULL REFERENCES public.espacio(id) ON DELETE RESTRICT,
    anfitrion_id uuid NOT NULL REFERENCES public.usuario(id) ON DELETE RESTRICT,
    arrendatario_id uuid NOT NULL REFERENCES public.usuario(id) ON DELETE RESTRICT,
    tarifa_version integer NOT NULL,
    modalidad text NOT NULL CHECK (modalidad IN ('hora','dia','mes')),
    precio_unitario_clp bigint NOT NULL CHECK (precio_unitario_clp > 5000),
    moneda char(3) NOT NULL CHECK (moneda='CLP'),
    unidades bigint NOT NULL CHECK (unidades > 0),
    subtotal_clp bigint NOT NULL CHECK (subtotal_clp > 0),
    inicio timestamptz NOT NULL,
    termino timestamptz NOT NULL CHECK (termino > inicio),
    zona_horaria text NOT NULL CHECK (length(btrim(zona_horaria)) BETWEEN 1 AND 100),
    creada_en timestamptz NOT NULL,
    vence_en timestamptz NOT NULL CHECK (vence_en > creada_en),
    FOREIGN KEY (espacio_id, tarifa_version) REFERENCES public.tarifa_espacio(espacio_id,version) ON DELETE RESTRICT
);
CREATE INDEX cotizacion_reserva_ensayo_titular_idx ON public.cotizacion_reserva_ensayo(arrendatario_id,creada_en DESC);

CREATE TABLE public.reserva_ensayo_local (
    id uuid PRIMARY KEY,
    cotizacion_id uuid NOT NULL UNIQUE REFERENCES public.cotizacion_reserva_ensayo(id) ON DELETE RESTRICT,
    espacio_id uuid NOT NULL REFERENCES public.espacio(id) ON DELETE RESTRICT,
    anfitrion_id uuid NOT NULL REFERENCES public.usuario(id) ON DELETE RESTRICT,
    arrendatario_id uuid NOT NULL REFERENCES public.usuario(id) ON DELETE RESTRICT,
    clave_idempotencia text NOT NULL CHECK (length(btrim(clave_idempotencia)) BETWEEN 1 AND 200),
    huella_solicitud bytea NOT NULL CHECK (octet_length(huella_solicitud)=32),
    ocupacion_id uuid NOT NULL UNIQUE,
    estado text NOT NULL CHECK (estado IN ('pendiente_de_pago','pagada','aprobada_host','cancelada_por_pago','rechazada_arrendador','vencida_pago','vencida_host','cancelada_arrendatario')),
    precio_unitario_clp bigint NOT NULL,
    unidades bigint NOT NULL,
    subtotal_clp bigint NOT NULL,
    modalidad text NOT NULL,
    moneda char(3) NOT NULL CHECK (moneda='CLP'),
    inicio timestamptz NOT NULL,
    termino timestamptz NOT NULL CHECK (termino > inicio),
    zona_horaria text NOT NULL,
    pago_vence_en timestamptz NOT NULL,
    anfitrion_vence_en timestamptz,
    creada_en timestamptz NOT NULL,
    actualizada_en timestamptz NOT NULL,
    UNIQUE(arrendatario_id,clave_idempotencia)
);
ALTER TABLE public.ocupacion ADD CONSTRAINT ocupacion_reserva_ensayo_fk
    FOREIGN KEY (reserva_id) REFERENCES public.reserva_ensayo_local(id) ON DELETE RESTRICT DEFERRABLE INITIALLY DEFERRED;
ALTER TABLE public.reserva_ensayo_local ADD CONSTRAINT reserva_ensayo_ocupacion_fk
    FOREIGN KEY (ocupacion_id) REFERENCES public.ocupacion(id) ON DELETE RESTRICT DEFERRABLE INITIALLY DEFERRED;
CREATE INDEX reserva_ensayo_local_anfitrion_idx ON public.reserva_ensayo_local(anfitrion_id,actualizada_en DESC,id);
CREATE INDEX reserva_ensayo_local_arrendatario_idx ON public.reserva_ensayo_local(arrendatario_id,actualizada_en DESC,id);

CREATE TABLE public.reserva_ensayo_transicion (
    id uuid PRIMARY KEY,
    reserva_id uuid NOT NULL REFERENCES public.reserva_ensayo_local(id) ON DELETE RESTRICT,
    estado_anterior text,
    estado_nuevo text NOT NULL,
    actor_id uuid REFERENCES public.usuario(id) ON DELETE RESTRICT,
    motivo text NOT NULL,
    creada_en timestamptz NOT NULL
);
CREATE INDEX reserva_ensayo_transicion_historial_idx ON public.reserva_ensayo_transicion(reserva_id,creada_en,id);

CREATE TABLE public.reserva_pago_ensayo (
    id uuid PRIMARY KEY,
    reserva_id uuid NOT NULL REFERENCES public.reserva_ensayo_local(id) ON DELETE RESTRICT,
    resultado text NOT NULL CHECK (resultado IN ('exito_simulado','rechazo_simulado','sin_respuesta_simulada','devolucion_simulada')),
    clave_idempotencia text NOT NULL CHECK(length(btrim(clave_idempotencia)) BETWEEN 1 AND 200),
    creada_en timestamptz NOT NULL,
    UNIQUE(reserva_id,clave_idempotencia)
);
