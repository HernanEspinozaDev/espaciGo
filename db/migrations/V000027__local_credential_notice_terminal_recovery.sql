-- LOCAL-PRIV-01A5: bounded automatic cycles and administrator recovery for
-- the synthetic credential-change notice only.
ALTER TABLE public.outbox_evento_local
    ADD COLUMN ciclo_actual integer NOT NULL DEFAULT 1 CHECK (ciclo_actual > 0),
    ADD COLUMN intentos_ciclo integer NOT NULL DEFAULT 0 CHECK (intentos_ciclo BETWEEN 0 AND 8),
    ADD COLUMN fallo_terminal_en timestamptz,
    ADD COLUMN codigo_fallo_terminal text,
    ADD CONSTRAINT outbox_terminal_failure_ck CHECK (
        (fallo_terminal_en IS NULL AND codigo_fallo_terminal IS NULL)
        OR (fallo_terminal_en IS NOT NULL AND codigo_fallo_terminal IS NOT NULL
            AND entregada_en IS NULL AND cancelada_en IS NULL AND lease_hasta IS NULL
            AND retirar_en = fallo_terminal_en + interval '30 days')
    );

-- Existing rows become cycle 1. The previous implementation allowed unbounded
-- retries; rows already past the new cap are frozen as terminal at migration time.
UPDATE public.outbox_evento_local
SET intentos_ciclo = LEAST(intentos, 8),
    fallo_terminal_en = CASE
        WHEN entregada_en IS NULL AND cancelada_en IS NULL AND intentos >= 8
        THEN CURRENT_TIMESTAMP ELSE NULL END,
    codigo_fallo_terminal = CASE
        WHEN entregada_en IS NULL AND cancelada_en IS NULL AND intentos >= 8
        THEN 'mailpit_delivery_failed' ELSE NULL END,
    lease_hasta = CASE
        WHEN entregada_en IS NULL AND cancelada_en IS NULL AND intentos >= 8
        THEN NULL ELSE lease_hasta END,
    retirar_en = CASE
        WHEN entregada_en IS NULL AND cancelada_en IS NULL AND intentos >= 8
        THEN CURRENT_TIMESTAMP + interval '30 days'
        ELSE retirar_en END;

CREATE TABLE public.outbox_evento_ciclo_local (
    evento_id uuid NOT NULL REFERENCES public.outbox_evento_local(id) ON DELETE CASCADE,
    numero_ciclo integer NOT NULL CHECK (numero_ciclo > 0),
    estado text NOT NULL CHECK (estado IN ('pendiente','entregada','fallo_terminal','cancelada')),
    iniciada_en timestamptz NOT NULL,
    finalizada_en timestamptz,
    intentos integer NOT NULL DEFAULT 0 CHECK (intentos BETWEEN 0 AND 8),
    codigo_resultado text,
    actor_reapertura_id uuid REFERENCES public.usuario(id) ON DELETE RESTRICT,
    motivo_reapertura_codigo text CHECK (motivo_reapertura_codigo IS NULL OR motivo_reapertura_codigo IN ('smtp_restaurado','reintento_operativo')),
    correlacion_id text,
    clave_idempotencia text,
    PRIMARY KEY (evento_id, numero_ciclo),
    UNIQUE (evento_id, clave_idempotencia),
    CHECK ((estado='pendiente') = (finalizada_en IS NULL)),
    CHECK ((actor_reapertura_id IS NULL AND motivo_reapertura_codigo IS NULL AND correlacion_id IS NULL AND clave_idempotencia IS NULL)
        OR (actor_reapertura_id IS NOT NULL AND motivo_reapertura_codigo IS NOT NULL AND correlacion_id IS NOT NULL AND clave_idempotencia IS NOT NULL)),
    CHECK (finalizada_en IS NULL OR finalizada_en >= iniciada_en)
);

INSERT INTO public.outbox_evento_ciclo_local(
    evento_id, numero_ciclo, estado, iniciada_en, finalizada_en, intentos, codigo_resultado
)
SELECT id, 1,
       CASE WHEN entregada_en IS NOT NULL THEN 'entregada'
            WHEN cancelada_en IS NOT NULL THEN 'cancelada'
            WHEN fallo_terminal_en IS NOT NULL THEN 'fallo_terminal'
            ELSE 'pendiente' END,
       creada_en,
       COALESCE(entregada_en, cancelada_en, fallo_terminal_en),
       LEAST(intentos_ciclo,8), codigo_fallo_terminal
FROM public.outbox_evento_local;

ALTER TABLE public.evento_auditoria_local DROP CONSTRAINT evento_auditoria_idempotencia_ck;
ALTER TABLE public.evento_auditoria_local ADD CONSTRAINT evento_auditoria_idempotencia_ck CHECK (
    clave_idempotencia IS NULL OR (
        length(btrim(clave_idempotencia)) BETWEEN 1 AND 200
        AND ((accion IN ('privacy.suppression.review','privacy.suppression.execute') AND recurso_tipo='solicitud_titular')
          OR (accion='identity.credential_notice.reopen' AND recurso_tipo='outbox_evento'))
    )
);

ALTER TABLE public.evento_auditoria_local DROP CONSTRAINT evento_auditoria_codigos_ck;
ALTER TABLE public.evento_auditoria_local ADD CONSTRAINT evento_auditoria_codigos_ck CHECK (
    jsonb_typeof(detalle_codigos)='object'
    AND detalle_codigos ? 'obligations_detected'
    AND detalle_codigos ? 'pending_checks'
    AND (detalle_codigos - ARRAY['obligations_detected','pending_checks']::text[])='{}'::jsonb
    AND jsonb_typeof(detalle_codigos->'obligations_detected')='array'
    AND jsonb_typeof(detalle_codigos->'pending_checks')='array'
    AND (detalle_codigos->'obligations_detected') <@ '["reserva_activa","pago_o_devolucion_pendiente","disputa_abierta"]'::jsonb
    AND (detalle_codigos->'pending_checks') <@ '["fuente_disputas_no_modelada","matriz_retencion_historicos_incompleta"]'::jsonb
);

ALTER TABLE public.evento_auditoria_local ADD CONSTRAINT evento_auditoria_reapertura_motivo_ck CHECK (
    accion <> 'identity.credential_notice.reopen'
    OR (motivo_codigo IN ('smtp_restaurado','reintento_operativo') AND resultado='exito')
);

ALTER TABLE public.outbox_evento_local DROP CONSTRAINT outbox_evento_entrega_ck;
ALTER TABLE public.outbox_evento_local ADD CONSTRAINT outbox_evento_entrega_ck CHECK (
    entregada_en IS NULL OR entregada_en >= creada_en
);

CREATE INDEX outbox_evento_local_terminal_retention_idx
    ON public.outbox_evento_local(retirar_en,id)
    WHERE retirar_en IS NOT NULL;

CREATE FUNCTION public.purge_expired_local_credential_notices(p_at timestamptz, p_limit integer)
RETURNS bigint
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, public
AS $$
DECLARE removed bigint;
BEGIN
    WITH candidate AS (
        SELECT event.id FROM public.outbox_evento_local AS event
        WHERE event.tipo='identidad.credencial_cambiada' AND event.retirar_en<=p_at
          AND event.lease_hasta IS NULL AND (event.entregada_en IS NOT NULL OR event.cancelada_en IS NOT NULL OR event.fallo_terminal_en IS NOT NULL)
        ORDER BY event.retirar_en,event.id FOR UPDATE SKIP LOCKED LIMIT p_limit
    )
    DELETE FROM public.outbox_evento_local AS event USING candidate
    WHERE event.id=candidate.id;
    GET DIAGNOSTICS removed = ROW_COUNT;
    RETURN removed;
END;
$$;
REVOKE ALL ON FUNCTION public.purge_expired_local_credential_notices(timestamptz,integer) FROM PUBLIC;

ALTER TABLE public.outbox_evento_local DROP CONSTRAINT outbox_cancelacion_ck;
ALTER TABLE public.outbox_evento_local ADD CONSTRAINT outbox_cancelacion_ck CHECK (
    (cancelada_en IS NULL AND motivo_cancelacion_codigo IS NULL)
    OR (cancelada_en IS NOT NULL AND motivo_cancelacion_codigo IN ('baja_local_sin_finalidad','destinatario_inactivo'))
);
