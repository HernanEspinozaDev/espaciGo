-- V13 introduced the persisted sequence. Rebuild legacy ties from the
-- reservation state chain, never from the random transition UUID.
ALTER TABLE public.reserva_ensayo_transicion
    DROP CONSTRAINT reserva_ensayo_transicion_reserva_secuencia_uq;

WITH ordered AS (
    SELECT id,
           row_number() OVER (
               PARTITION BY reserva_id
               ORDER BY creada_en,
                        CASE
                            WHEN estado_anterior IS NULL THEN 0
                            WHEN estado_anterior = 'pendiente_de_pago' THEN 1
                            WHEN estado_anterior = 'pagada' THEN 2
                            ELSE 3
                        END,
                        estado_nuevo,
                        COALESCE(estado_anterior, ''),
                        motivo
           ) AS n
    FROM public.reserva_ensayo_transicion
)
UPDATE public.reserva_ensayo_transicion t
SET secuencia = ordered.n
FROM ordered
WHERE ordered.id = t.id;

ALTER TABLE public.reserva_ensayo_transicion
    ADD CONSTRAINT reserva_ensayo_transicion_reserva_secuencia_uq UNIQUE (reserva_id, secuencia);
