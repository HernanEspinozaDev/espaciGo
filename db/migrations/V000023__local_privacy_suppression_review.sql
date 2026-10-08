-- LOCAL-PRIV-01A: extend the ratified append-only audit stream so suppression
-- assessments are durable/idempotent without creating a second retention log.
ALTER TABLE public.evento_auditoria_local
    ADD COLUMN clave_idempotencia text,
    ADD COLUMN detalle_codigos jsonb NOT NULL DEFAULT '{"obligations_detected":[],"pending_checks":[]}';

ALTER TABLE public.evento_auditoria_local
    ADD CONSTRAINT evento_auditoria_idempotencia_ck CHECK (
        clave_idempotencia IS NULL OR (
            length(btrim(clave_idempotencia)) BETWEEN 1 AND 200
            AND accion='privacy.suppression.review'
            AND recurso_tipo='solicitud_titular'
        )
    ),
    ADD CONSTRAINT evento_auditoria_codigos_ck CHECK (
        jsonb_typeof(detalle_codigos)='object'
        AND detalle_codigos ? 'obligations_detected'
        AND detalle_codigos ? 'pending_checks'
        AND (detalle_codigos - ARRAY['obligations_detected','pending_checks']::text[])='{}'::jsonb
        AND jsonb_typeof(detalle_codigos->'obligations_detected')='array'
        AND jsonb_typeof(detalle_codigos->'pending_checks')='array'
        AND (detalle_codigos->'obligations_detected') <@ '["reserva_activa","pago_o_devolucion_pendiente"]'::jsonb
        AND (detalle_codigos->'pending_checks') <@ '["fuente_disputas_no_modelada","matriz_retencion_historicos_incompleta"]'::jsonb
    );

CREATE UNIQUE INDEX evento_auditoria_idempotente_uq
    ON public.evento_auditoria_local(recurso_id, accion, clave_idempotencia)
    WHERE clave_idempotencia IS NOT NULL;
