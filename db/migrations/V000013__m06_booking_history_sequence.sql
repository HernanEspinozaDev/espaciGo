-- Give each reservation a deterministic history order, even when multiple
-- transitions share the same event timestamp.
ALTER TABLE public.reserva_ensayo_transicion
    ADD COLUMN secuencia bigint;

WITH ordered AS (
    SELECT id,
           row_number() OVER (PARTITION BY reserva_id ORDER BY creada_en, id) AS n
    FROM public.reserva_ensayo_transicion
)
UPDATE public.reserva_ensayo_transicion t
SET secuencia = ordered.n
FROM ordered
WHERE ordered.id = t.id;

ALTER TABLE public.reserva_ensayo_transicion
    ALTER COLUMN secuencia SET NOT NULL,
    ADD CONSTRAINT reserva_ensayo_transicion_secuencia_ck CHECK (secuencia > 0),
    ADD CONSTRAINT reserva_ensayo_transicion_reserva_secuencia_uq UNIQUE (reserva_id, secuencia);

DROP INDEX public.reserva_ensayo_transicion_historial_idx;
CREATE INDEX reserva_ensayo_transicion_historial_idx
    ON public.reserva_ensayo_transicion(reserva_id, secuencia);
