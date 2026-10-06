-- Preserve the owner-entered use rules as part of the quote and reservation
-- contract; keep this additive because V11 has already been applied locally.
ALTER TABLE public.cotizacion_reserva_ensayo
    ADD COLUMN condiciones_snapshot text NOT NULL DEFAULT '';
ALTER TABLE public.reserva_ensayo_local
    ADD COLUMN condiciones_snapshot text NOT NULL DEFAULT '';

UPDATE public.cotizacion_reserva_ensayo q
SET condiciones_snapshot=e.reglas_uso
FROM public.espacio e
WHERE e.id=q.espacio_id AND q.condiciones_snapshot='';

UPDATE public.reserva_ensayo_local r
SET condiciones_snapshot=q.condiciones_snapshot
FROM public.cotizacion_reserva_ensayo q
WHERE q.id=r.cotizacion_id AND r.condiciones_snapshot='';

ALTER TABLE public.cotizacion_reserva_ensayo ALTER COLUMN condiciones_snapshot DROP DEFAULT;
ALTER TABLE public.reserva_ensayo_local ALTER COLUMN condiciones_snapshot DROP DEFAULT;
