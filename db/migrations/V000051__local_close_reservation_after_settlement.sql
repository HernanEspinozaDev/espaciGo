-- LOCAL-M10-BOOKING-CLOSE-01. A successful, persisted fake settlement may
-- close a finalized reservation. No snapshot, amount or historical row is
-- rewritten by this state extension.
ALTER TABLE public.reserva_ensayo_local
    DROP CONSTRAINT reserva_ensayo_local_estado_check,
    ADD CONSTRAINT reserva_ensayo_local_estado_check CHECK (estado IN (
        'pendiente_de_pago','pagada','aprobada_host','firma_parcial','lista_para_checkin',
        'en_curso','finalizada','en_disputa','cerrada','cancelada_por_pago','rechazada_arrendador',
        'vencida_pago','vencida_host','cancelada_arrendatario','cancelada_por_firma'
    ));

-- Keep the terminal transition tied to the persisted result in the same
-- transaction. A handler-only guard would allow a runtime writer to close a
-- reservation before the fake settlement had actually been applied.
CREATE OR REPLACE FUNCTION public.guard_local_reservation_close_after_settlement() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF OLD.estado IS DISTINCT FROM 'cerrada' AND NEW.estado='cerrada' THEN
        IF OLD.estado IS DISTINCT FROM 'finalizada' OR NOT EXISTS (
            SELECT 1
            FROM public.reserva_liquidacion_ensayo_local s
            JOIN public.reserva_liquidacion_operacion_ensayo_local o ON o.liquidacion_id=s.id
            JOIN public.reserva_liquidacion_resultado_fake_ensayo_local f ON f.operacion_id=o.id
            WHERE s.reserva_id=NEW.id
              AND s.estado='liquidada'
              AND s.ultimo_resultado='exito_simulado'
              AND f.resultado='exito_simulado'
        ) THEN
            RAISE EXCEPTION 'only a finalized reservation with a confirmed fake settlement can be closed'
                USING ERRCODE='23514', CONSTRAINT='reserva_cerrada_liquidacion_fake_ck';
        END IF;
    END IF;
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS reserva_cerrada_liquidacion_fake_guard ON public.reserva_ensayo_local;
CREATE TRIGGER reserva_cerrada_liquidacion_fake_guard
BEFORE UPDATE OF estado ON public.reserva_ensayo_local
FOR EACH ROW EXECUTE FUNCTION public.guard_local_reservation_close_after_settlement();

-- Closing is a terminal booking state and starts the already-ratified link
-- retention period from the actual close transition; no shared retention is
-- added to unrelated tables.
CREATE OR REPLACE FUNCTION public.schedule_local_reservation_link_retention() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.estado IN ('finalizada','cerrada','cancelada_por_pago','rechazada_arrendador','vencida_pago','vencida_host','cancelada_arrendatario')
       AND (TG_OP='INSERT' OR OLD.estado IS DISTINCT FROM NEW.estado) THEN
        NEW.vinculos_retirar_en := GREATEST(COALESCE(NEW.vinculos_retirar_en,'epoch'::timestamptz),NEW.actualizada_en + interval '24 months');
    END IF;
    RETURN NEW;
END;
$$;

DROP INDEX IF EXISTS public.reserva_ensayo_retencion_local_idx;
CREATE INDEX reserva_ensayo_retencion_local_idx ON public.reserva_ensayo_local(vinculos_retirar_en,id)
WHERE estado IN ('finalizada','cerrada','cancelada_por_pago','rechazada_arrendador','vencida_pago','vencida_host','cancelada_arrendatario');
