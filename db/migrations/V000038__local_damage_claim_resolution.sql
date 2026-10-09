-- LOCAL-DIS-01: record an administrative synthetic adjudication, separately
-- from every financial effect. LOCAL-FIN-01 owns any capture/release/settlement.
ALTER TABLE public.reclamo_dano_ensayo_local
  DROP CONSTRAINT reclamo_dano_ensayo_local_estado_check,
  ADD CONSTRAINT reclamo_dano_ensayo_local_estado_check CHECK (estado IN ('abierto','resuelta'));

CREATE TABLE public.reclamo_dano_resolucion_ensayo_local (
  reclamo_id uuid PRIMARY KEY REFERENCES public.reclamo_dano_ensayo_local(id) ON DELETE RESTRICT,
  resultado text NOT NULL CHECK (resultado IN ('acogido','rechazado')),
  motivo_codigo text NOT NULL CHECK (motivo_codigo IN ('evidencia_suficiente','evidencia_insuficiente','hecho_no_acreditado','informacion_insuficiente')),
  administrador_id uuid NOT NULL REFERENCES public.usuario(id) ON DELETE RESTRICT,
  resuelta_en timestamptz NOT NULL,
  clave_idempotencia text NOT NULL CHECK (length(btrim(clave_idempotencia)) BETWEEN 8 AND 200),
  huella_solicitud bytea NOT NULL CHECK (octet_length(huella_solicitud)=32),
  CHECK ((resultado='acogido' AND motivo_codigo='evidencia_suficiente') OR
         (resultado='rechazado' AND motivo_codigo<>'evidencia_suficiente')),
  UNIQUE(administrador_id,clave_idempotencia)
);

ALTER TABLE public.reclamo_dano_historial_ensayo_local
  DROP CONSTRAINT reclamo_dano_historial_ensayo_local_accion_check,
  ADD CONSTRAINT reclamo_dano_historial_ensayo_local_accion_check CHECK (accion IN ('reclamo_abierto','descargo_registrado','reclamo_resuelto'));

ALTER TABLE public.evento_auditoria_local DROP CONSTRAINT evento_auditoria_idempotencia_ck;
ALTER TABLE public.evento_auditoria_local ADD CONSTRAINT evento_auditoria_idempotencia_ck CHECK (
  clave_idempotencia IS NULL OR (
    length(btrim(clave_idempotencia)) BETWEEN 1 AND 200
    AND ((accion IN ('privacy.suppression.review','privacy.suppression.execute') AND recurso_tipo='solicitud_titular')
      OR (accion='identity.credential_notice.reopen' AND recurso_tipo='outbox_evento')
      OR (accion='reputation.review.moderate' AND recurso_tipo='reporte_resena')
      OR (accion='local.damage_claim.resolve' AND recurso_tipo='reclamo_dano'))
  )
);

ALTER TABLE public.aviso_local DROP CONSTRAINT aviso_local_tipo_evento_check,
  ADD CONSTRAINT aviso_local_tipo_evento_check CHECK (tipo_evento IN ('checkin_registrado','resena_reportada','reclamo_abierto','reclamo_resuelto','reserva_cancelada'));

CREATE FUNCTION public.enqueue_local_damage_claim_resolution_notice() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE claim public.reclamo_dano_ensayo_local%ROWTYPE;
BEGIN
  SELECT * INTO claim FROM public.reclamo_dano_ensayo_local WHERE id=NEW.reclamo_id;
  PERFORM public.enqueue_local_notice('reclamo_resuelto','reserva',claim.reserva_id,claim.anfitrion_id,
    'claim-resolved:'||claim.id::text||':'||claim.anfitrion_id::text,NEW.resuelta_en);
  PERFORM public.enqueue_local_notice('reclamo_resuelto','reserva',claim.reserva_id,claim.arrendatario_id,
    'claim-resolved:'||claim.id::text||':'||claim.arrendatario_id::text,NEW.resuelta_en);
  RETURN NEW;
END $$;
CREATE TRIGGER local_damage_claim_resolution_notice_trg
  AFTER INSERT ON public.reclamo_dano_resolucion_ensayo_local
  FOR EACH ROW EXECUTE FUNCTION public.enqueue_local_damage_claim_resolution_notice();

COMMENT ON TABLE public.reclamo_dano_resolucion_ensayo_local IS
  'Synthetic local adjudication only. It stores no guarantee amount and performs no capture, release, refund or settlement; those effects are deferred to LOCAL-FIN-01.';
