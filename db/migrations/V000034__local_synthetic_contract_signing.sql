-- LOCAL-CONT-01: synthetic contract/signature rehearsal only. Historical
-- reservation rows are retained and no legal-signature provider is implied.
ALTER TABLE public.reserva_ensayo_local
    DROP CONSTRAINT reserva_ensayo_local_estado_check,
    ADD CONSTRAINT reserva_ensayo_local_estado_check CHECK (estado IN (
        'pendiente_de_pago','pagada','aprobada_host','firma_parcial','lista_para_checkin',
        'cancelada_por_pago','rechazada_arrendador','vencida_pago','vencida_host',
        'cancelada_arrendatario','cancelada_por_firma'
    ));

-- M09 document owner: immutable synthetic PDF bytes and metadata, retrieved
-- only through participant-authorized module APIs.
CREATE TABLE public.documento_privado_sintetico_local (
    id uuid PRIMARY KEY,
    reserva_id uuid NOT NULL REFERENCES public.reserva_ensayo_local(id) ON DELETE RESTRICT,
    mime_type text NOT NULL CHECK (mime_type='application/pdf'),
    encryption_algorithm text NOT NULL CHECK (encryption_algorithm='aes-256-gcm'),
    sha256 bytea NOT NULL CHECK (octet_length(sha256)=32),
    size_bytes integer NOT NULL CHECK (size_bytes BETWEEN 1 AND 2097152),
    contenido bytea NOT NULL,
    creada_en timestamptz NOT NULL
);
CREATE TABLE public.contrato_ensayo_local (
    id uuid PRIMARY KEY,
    reserva_id uuid NOT NULL UNIQUE REFERENCES public.reserva_ensayo_local(id) ON DELETE RESTRICT,
    version integer NOT NULL CHECK (version > 0),
    estado text NOT NULL CHECK (estado IN ('generado','firma_parcial','firmado','anulado')),
    snapshot jsonb NOT NULL,
    documento_id uuid NOT NULL UNIQUE REFERENCES public.documento_privado_sintetico_local(id) ON DELETE RESTRICT,
    sha256 bytea NOT NULL CHECK (octet_length(sha256)=32),
    creada_en timestamptz NOT NULL,
    actualizada_en timestamptz NOT NULL,
    firmado_en timestamptz,
    UNIQUE(reserva_id,version),
    CHECK ((estado='firmado') = (firmado_en IS NOT NULL))
);
CREATE TABLE public.contrato_ensayo_firma (
    contrato_id uuid NOT NULL REFERENCES public.contrato_ensayo_local(id) ON DELETE RESTRICT,
    firmante_id uuid NOT NULL REFERENCES public.usuario(id) ON DELETE RESTRICT,
    rol text NOT NULL CHECK (rol IN ('anfitrion','arrendatario')),
    estado text NOT NULL CHECK (estado IN ('pendiente','firmada','rechazada')),
    motivo text NOT NULL DEFAULT '',
    actualizada_en timestamptz NOT NULL,
    PRIMARY KEY(contrato_id,firmante_id)
);
CREATE TABLE public.contrato_ensayo_historial (
    id uuid PRIMARY KEY,
    contrato_id uuid NOT NULL REFERENCES public.contrato_ensayo_local(id) ON DELETE RESTRICT,
    secuencia bigint NOT NULL CHECK(secuencia > 0),
    actor_id uuid REFERENCES public.usuario(id) ON DELETE RESTRICT,
    accion text NOT NULL CHECK (accion IN ('generado','firmado','rechazado','firmado_completo','vencido','reserva_terminal')),
    motivo text NOT NULL DEFAULT '',
    creada_en timestamptz NOT NULL,
    UNIQUE(contrato_id,secuencia)
);
CREATE INDEX contrato_ensayo_historial_idx ON public.contrato_ensayo_historial(contrato_id,secuencia);
