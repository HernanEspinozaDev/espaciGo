-- LOCAL-FIN-SETTLEMENT-01. Additive snapshots and durable local fake payout.
-- Historical quotes/reservations remain NULL; no retroactive policy is assigned.
ALTER TABLE public.cotizacion_reserva_ensayo
    ADD COLUMN liquidacion_politica_version text,
    ADD COLUMN liquidacion_comision_bps integer,
    ADD CONSTRAINT cotizacion_reserva_liquidacion_snapshot_ck CHECK (
      (liquidacion_politica_version IS NULL AND liquidacion_comision_bps IS NULL)
      OR (liquidacion_politica_version='liquidacion_local_0pct_v1' AND liquidacion_comision_bps=0)
    );

ALTER TABLE public.reserva_ensayo_local
    ADD COLUMN liquidacion_politica_version text,
    ADD COLUMN liquidacion_comision_bps integer,
    ADD CONSTRAINT reserva_ensayo_liquidacion_snapshot_ck CHECK (
      (liquidacion_politica_version IS NULL AND liquidacion_comision_bps IS NULL)
      OR (liquidacion_politica_version='liquidacion_local_0pct_v1' AND liquidacion_comision_bps=0)
    );

CREATE TABLE public.reserva_liquidacion_ensayo_local (
    id uuid PRIMARY KEY,
    reserva_id uuid NOT NULL UNIQUE REFERENCES public.reserva_ensayo_local(id) ON DELETE RESTRICT,
    anfitrion_id uuid REFERENCES public.usuario(id) ON DELETE RESTRICT,
    politica_version text NOT NULL CHECK (politica_version='liquidacion_local_0pct_v1'),
    moneda char(3) NOT NULL CHECK (moneda='CLP'),
    arriendo_confirmado_clp bigint NOT NULL CHECK (arriendo_confirmado_clp>=0),
    devoluciones_confirmadas_clp bigint NOT NULL CHECK (devoluciones_confirmadas_clp>=0),
    comision_bps integer NOT NULL CHECK (comision_bps BETWEEN 0 AND 10000),
    comision_clp bigint NOT NULL CHECK (comision_clp>=0),
    transferencia_clp bigint NOT NULL CHECK (transferencia_clp>=0),
    estado text NOT NULL CHECK (estado IN ('pendiente','por_conciliar','liquidada','rechazada')),
    ultimo_resultado text CHECK (ultimo_resultado IN ('exito_simulado','rechazo_simulado','sin_respuesta_simulada')),
    clave_idempotencia text NOT NULL CHECK (length(btrim(clave_idempotencia)) BETWEEN 1 AND 200),
    huella_solicitud bytea NOT NULL CHECK (octet_length(huella_solicitud)=32),
    creada_en timestamptz NOT NULL,
    actualizada_en timestamptz NOT NULL,
    completada_en timestamptz,
    UNIQUE(reserva_id,clave_idempotencia),
    CHECK (transferencia_clp = arriendo_confirmado_clp - devoluciones_confirmadas_clp - comision_clp),
    CHECK ((estado IN ('liquidada','rechazada')) = (completada_en IS NOT NULL))
);

CREATE TABLE public.reserva_liquidacion_operacion_ensayo_local (
    id uuid PRIMARY KEY,
    liquidacion_id uuid NOT NULL UNIQUE REFERENCES public.reserva_liquidacion_ensayo_local(id) ON DELETE RESTRICT,
    resultado_solicitado text NOT NULL CHECK (resultado_solicitado IN ('exito','rechazo','sin_respuesta')),
    creada_en timestamptz NOT NULL,
    actualizada_en timestamptz NOT NULL
);

CREATE TABLE public.reserva_liquidacion_resultado_fake_ensayo_local (
    operacion_id uuid PRIMARY KEY REFERENCES public.reserva_liquidacion_operacion_ensayo_local(id) ON DELETE RESTRICT,
    event_id text UNIQUE CHECK (event_id IS NULL OR length(event_id) BETWEEN 1 AND 200),
    resultado text NOT NULL CHECK (resultado IN ('exito_simulado','rechazo_simulado','sin_respuesta_simulada')),
    registrada_en timestamptz NOT NULL,
    CHECK ((resultado='sin_respuesta_simulada' AND event_id IS NULL) OR
           (resultado IN ('exito_simulado','rechazo_simulado') AND event_id IS NOT NULL))
);

CREATE TABLE public.reserva_comprobante_ensayo_local (
    id uuid PRIMARY KEY,
    liquidacion_id uuid NOT NULL UNIQUE REFERENCES public.reserva_liquidacion_ensayo_local(id) ON DELETE RESTRICT,
    version smallint NOT NULL CHECK (version=1),
    contenido jsonb NOT NULL CHECK (contenido->>'kind'='comprobante_ensayo'
      AND contenido->>'banner'='ENSAYO LOCAL — SIN VALIDEZ FISCAL — SIN MOVIMIENTOS REALES'),
    emitido_en timestamptz NOT NULL
);

CREATE TABLE public.reserva_liquidacion_historial_ensayo_local (
    secuencia bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    liquidacion_id uuid NOT NULL REFERENCES public.reserva_liquidacion_ensayo_local(id) ON DELETE RESTRICT,
    accion text NOT NULL CHECK (accion IN ('liquidacion_solicitada','liquidacion_por_conciliar','liquidacion_confirmada','liquidacion_rechazada')),
    actor_id uuid REFERENCES public.usuario(id) ON DELETE RESTRICT,
    codigo text NOT NULL,
    ocurrida_en timestamptz NOT NULL
);

ALTER TABLE public.reserva_finanzas_historial_ensayo_local
    DROP CONSTRAINT reserva_finanzas_historial_ensayo_local_tipo_check,
    ADD CONSTRAINT reserva_finanzas_historial_ensayo_local_tipo_check CHECK (tipo IN (
      'garantia_solicitada','garantia_autorizada','garantia_rechazada','garantia_vencida','garantia_liberada','garantia_capturada',
      'decision_financiera_registrada','operacion_pendiente_conciliacion',
      'liquidacion_solicitada','liquidacion_confirmada','liquidacion_rechazada'
    ));

ALTER TABLE public.evento_auditoria_local DROP CONSTRAINT evento_auditoria_idempotencia_ck;
ALTER TABLE public.evento_auditoria_local ADD CONSTRAINT evento_auditoria_idempotencia_ck CHECK (
  clave_idempotencia IS NULL OR (
    length(btrim(clave_idempotencia)) BETWEEN 1 AND 200 AND
    ((accion IN ('privacy.suppression.review','privacy.suppression.execute') AND recurso_tipo='solicitud_titular') OR
     (accion='identity.credential_notice.reopen' AND recurso_tipo='outbox_evento') OR
     (accion='reputation.review.moderate' AND recurso_tipo='reporte_resena') OR
     (accion='local.damage_claim.resolve' AND recurso_tipo='reclamo_dano') OR
     (accion IN ('local.finance.decision','local.finance.guarantee.operation','local.finance.guarantee.reconcile','local.finance.settlement') AND recurso_tipo='reserva'))
  )
);

CREATE INDEX reserva_liquidacion_pendiente_idx ON public.reserva_liquidacion_ensayo_local(estado,actualizada_en,id) WHERE estado IN ('pendiente','por_conciliar');

-- Completed rentals enter the already-ratified 24-calendar-month transaction
-- retention window. This does not assign a new period to unrelated tables.
UPDATE public.reserva_ensayo_local r SET vinculos_retirar_en=GREATEST(
  COALESCE(r.vinculos_retirar_en,'epoch'::timestamptz),
  (SELECT GREATEST(r.actualizada_en,COALESCE((SELECT max(o.ocurrio_en) FROM public.operacion_arriendo_ensayo_local o WHERE o.reserva_id=r.id AND o.tipo='checkout'),r.actualizada_en)) + interval '24 months')
) WHERE r.estado='finalizada';

CREATE OR REPLACE FUNCTION public.schedule_local_reservation_link_retention() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.estado IN ('finalizada','cancelada_por_pago','rechazada_arrendador','vencida_pago','vencida_host','cancelada_arrendatario')
       AND (TG_OP='INSERT' OR OLD.estado IS DISTINCT FROM NEW.estado) THEN
        NEW.vinculos_retirar_en := GREATEST(COALESCE(NEW.vinculos_retirar_en,'epoch'::timestamptz),NEW.actualizada_en + interval '24 months');
    END IF;
    RETURN NEW;
END;
$$;
DROP INDEX IF EXISTS public.reserva_ensayo_retencion_local_idx;
CREATE INDEX reserva_ensayo_retencion_local_idx ON public.reserva_ensayo_local(vinculos_retirar_en,id)
WHERE estado IN ('finalizada','cancelada_por_pago','rechazada_arrendador','vencida_pago','vencida_host','cancelada_arrendatario');
