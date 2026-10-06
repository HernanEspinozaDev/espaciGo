-- Local-only reservation conversation. Synthetic text remains until an
-- explicit, thread-scoped local cleanup; no automatic retention/TTL is applied.
CREATE TABLE public.mensaje_reserva_ensayo (
    secuencia bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    id uuid NOT NULL UNIQUE,
    reserva_id uuid NOT NULL REFERENCES public.reserva_ensayo_local(id) ON DELETE RESTRICT,
    autor_id uuid NOT NULL REFERENCES public.usuario(id) ON DELETE RESTRICT,
    clave_idempotencia text NOT NULL CHECK (length(btrim(clave_idempotencia)) BETWEEN 1 AND 200),
    huella_solicitud bytea NOT NULL CHECK (octet_length(huella_solicitud) = 32),
    cuerpo text NOT NULL CHECK (char_length(cuerpo) BETWEEN 1 AND 2000 AND length(btrim(cuerpo)) > 0),
    creada_en timestamptz NOT NULL,
    UNIQUE (reserva_id, autor_id, clave_idempotencia)
);
CREATE INDEX mensaje_reserva_ensayo_hilo_idx
    ON public.mensaje_reserva_ensayo(reserva_id, secuencia DESC);
