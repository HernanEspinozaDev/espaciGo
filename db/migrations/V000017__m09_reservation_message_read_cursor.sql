-- Per-participant read watermark for local reservation conversations.
-- Messages and cursors remain persisted until explicitly handled; no TTL.
CREATE TABLE public.reserva_mensaje_lectura (
    reserva_id uuid NOT NULL REFERENCES public.reserva_ensayo_local(id) ON DELETE RESTRICT,
    participante_id uuid NOT NULL REFERENCES public.usuario(id) ON DELETE RESTRICT,
    ultima_secuencia_leida bigint NOT NULL CHECK (ultima_secuencia_leida > 0),
    actualizada_en timestamptz NOT NULL,
    PRIMARY KEY (reserva_id, participante_id)
);
