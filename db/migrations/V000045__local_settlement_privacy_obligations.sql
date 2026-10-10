-- A positive, confirmed rent balance on a new-policy finalized reservation is
-- an open privacy obligation before the durable settlement intent is created.
-- Permit its structured code in the existing suppression audit projection.
ALTER TABLE public.evento_auditoria_local
    DROP CONSTRAINT evento_auditoria_codigos_ck;
ALTER TABLE public.evento_auditoria_local
    ADD CONSTRAINT evento_auditoria_codigos_ck CHECK (
        jsonb_typeof(detalle_codigos)='object'
        AND detalle_codigos ? 'obligations_detected'
        AND detalle_codigos ? 'pending_checks'
        AND (detalle_codigos - ARRAY['obligations_detected','pending_checks']::text[])='{}'::jsonb
        AND jsonb_typeof(detalle_codigos->'obligations_detected')='array'
        AND jsonb_typeof(detalle_codigos->'pending_checks')='array'
        AND (detalle_codigos->'obligations_detected') <@ '["reserva_activa","pago_o_devolucion_pendiente","disputa_abierta","liquidacion_pendiente"]'::jsonb
        AND (detalle_codigos->'pending_checks') <@ '["fuente_disputas_no_modelada","matriz_retencion_historicos_incompleta"]'::jsonb
    );
