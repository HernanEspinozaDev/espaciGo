-- LOCAL-COMM-01: synthetic reviews/reports and transactional Mailpit notice intents.
-- These records have local prototype retention only; the periods are not legal claims.

CREATE TABLE public.resena_ensayo_local (
    id uuid PRIMARY KEY,
    reserva_id uuid NOT NULL REFERENCES public.reserva_ensayo_local(id) ON DELETE RESTRICT,
    espacio_id uuid NOT NULL REFERENCES public.espacio(id) ON DELETE RESTRICT,
    autor_id uuid REFERENCES public.usuario(id) ON DELETE RESTRICT,
    destinatario_tipo text NOT NULL CHECK (destinatario_tipo IN ('espacio','arrendatario')),
    destinatario_id uuid REFERENCES public.usuario(id) ON DELETE RESTRICT,
    puntuacion smallint NOT NULL CHECK (puntuacion BETWEEN 1 AND 5),
    comentario text NOT NULL DEFAULT '',
    estado text NOT NULL DEFAULT 'publicada' CHECK (estado IN ('publicada','reportada','oculta')),
    creada_en timestamptz NOT NULL,
    retirar_en timestamptz NOT NULL,
    huella_solicitud bytea NOT NULL CHECK (octet_length(huella_solicitud)=32),
    clave_idempotencia text NOT NULL CHECK (length(btrim(clave_idempotencia)) BETWEEN 8 AND 200),
    UNIQUE (reserva_id,autor_id),
    CHECK ((destinatario_tipo='espacio' AND destinatario_id IS NULL) OR
           (destinatario_tipo='arrendatario' AND destinatario_id IS NOT NULL)),
    CHECK (retirar_en = creada_en + interval '24 months')
);
CREATE INDEX resena_espacio_publica_idx ON public.resena_ensayo_local(espacio_id,creada_en DESC,id)
    WHERE estado IN ('publicada','reportada');
CREATE INDEX resena_arrendatario_idx ON public.resena_ensayo_local(destinatario_id,creada_en DESC,id)
    WHERE destinatario_tipo='arrendatario' AND estado IN ('publicada','reportada');
CREATE INDEX resena_retirar_idx ON public.resena_ensayo_local(retirar_en,id);

CREATE TABLE public.reporte_resena_ensayo_local (
    id uuid PRIMARY KEY,
    resena_id uuid NOT NULL UNIQUE REFERENCES public.resena_ensayo_local(id) ON DELETE RESTRICT,
    espacio_id uuid NOT NULL REFERENCES public.espacio(id) ON DELETE RESTRICT,
    anfitrion_id uuid REFERENCES public.usuario(id) ON DELETE RESTRICT,
    motivo_codigo text NOT NULL CHECK (motivo_codigo IN ('insultos_acoso','datos_personales','spam','ajeno_experiencia')),
    estado text NOT NULL CHECK (estado IN ('pendiente','desestimada','oculta')),
    creada_en timestamptz NOT NULL,
    resolver_en timestamptz,
    resuelta_por uuid REFERENCES public.usuario(id) ON DELETE RESTRICT,
    motivo_resolucion_codigo text CHECK (motivo_resolucion_codigo IN ('sin_infraccion','contenido_inadecuado','duplicado','error_registro')),
    retirar_en timestamptz,
    clave_idempotencia text NOT NULL CHECK (length(btrim(clave_idempotencia)) BETWEEN 8 AND 200),
    huella_solicitud bytea NOT NULL CHECK (octet_length(huella_solicitud)=32),
    CHECK ((estado='pendiente' AND resolver_en IS NULL AND resuelta_por IS NULL AND motivo_resolucion_codigo IS NULL AND retirar_en IS NULL)
        OR (estado IN ('desestimada','oculta') AND resolver_en IS NOT NULL AND resuelta_por IS NOT NULL AND motivo_resolucion_codigo IS NOT NULL AND retirar_en=resolver_en + interval '24 months'))
);
CREATE INDEX reporte_resena_retencion_idx ON public.reporte_resena_ensayo_local(retirar_en,id) WHERE retirar_en IS NOT NULL;
CREATE TABLE public.reporte_resena_historial_local (
    secuencia bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    reporte_id uuid NOT NULL REFERENCES public.reporte_resena_ensayo_local(id) ON DELETE RESTRICT,
    estado_anterior text CHECK (estado_anterior IN ('pendiente','desestimada','oculta')),
    estado_nuevo text NOT NULL CHECK (estado_nuevo IN ('pendiente','desestimada','oculta')),
    actor_id uuid REFERENCES public.usuario(id) ON DELETE RESTRICT,
    motivo_codigo text NOT NULL,
    ocurrida_en timestamptz NOT NULL
);
CREATE INDEX reporte_resena_historial_idx ON public.reporte_resena_historial_local(reporte_id,secuencia);

-- Per-recipient durable intent. No recipient address, message body, SMTP response or secrets are persisted.
CREATE TABLE public.aviso_local (
    id uuid PRIMARY KEY,
    tipo_evento text NOT NULL CHECK (tipo_evento IN ('checkin_registrado','resena_reportada','reclamo_abierto','reserva_cancelada')),
    agregado_tipo text NOT NULL CHECK (agregado_tipo IN ('reserva','reporte_resena')),
    agregado_id uuid NOT NULL,
    destinatario_id uuid NOT NULL REFERENCES public.usuario(id) ON DELETE RESTRICT,
    clave_deduplicacion text NOT NULL UNIQUE,
    estado text NOT NULL DEFAULT 'pendiente' CHECK (estado IN ('pendiente','procesando','entregada','fallo_terminal','cancelada')),
    ciclo integer NOT NULL DEFAULT 1 CHECK (ciclo > 0),
    intentos_total integer NOT NULL DEFAULT 0 CHECK (intentos_total >= 0),
    intentos_ciclo integer NOT NULL DEFAULT 0 CHECK (intentos_ciclo BETWEEN 0 AND 8),
    creada_en timestamptz NOT NULL,
    proximo_intento_en timestamptz NOT NULL,
    lease_hasta timestamptz,
    entregada_en timestamptz,
    fallo_terminal_en timestamptz,
    cancelada_en timestamptz,
    codigo_error text,
    retirar_en timestamptz,
    CHECK ((estado='entregada' AND entregada_en IS NOT NULL AND retirar_en=entregada_en + interval '30 days')
        OR (estado='fallo_terminal' AND fallo_terminal_en IS NOT NULL AND retirar_en=fallo_terminal_en + interval '30 days')
        OR (estado='cancelada' AND cancelada_en IS NOT NULL AND retirar_en=cancelada_en + interval '30 days')
        OR (estado IN ('pendiente','procesando') AND retirar_en IS NULL))
);
CREATE INDEX aviso_local_due_idx ON public.aviso_local(estado,proximo_intento_en,id);
CREATE INDEX aviso_local_recipient_idx ON public.aviso_local(destinatario_id,creada_en DESC,id);
CREATE TABLE public.aviso_local_ciclo (
    aviso_id uuid NOT NULL REFERENCES public.aviso_local(id) ON DELETE RESTRICT,
    ciclo integer NOT NULL,
    estado text NOT NULL CHECK (estado IN ('pendiente','entregada','fallo_terminal','cancelada')),
    iniciada_en timestamptz NOT NULL,
    finalizada_en timestamptz,
    intentos integer NOT NULL DEFAULT 0 CHECK (intentos BETWEEN 0 AND 8),
    codigo_resultado text,
    actor_reapertura_id uuid REFERENCES public.usuario(id) ON DELETE RESTRICT,
    motivo_reapertura_codigo text CHECK (motivo_reapertura_codigo IN ('smtp_restaurado','reintento_operativo')),
    correlacion_id text,
    clave_idempotencia text,
    PRIMARY KEY(aviso_id,ciclo), UNIQUE(aviso_id,clave_idempotencia),
    CHECK ((estado='pendiente')=(finalizada_en IS NULL))
);
CREATE TABLE public.aviso_local_recuperacion_auditoria (
    id uuid PRIMARY KEY,
    aviso_id uuid NOT NULL,
    actor_id uuid NOT NULL REFERENCES public.usuario(id) ON DELETE RESTRICT,
    motivo_codigo text NOT NULL CHECK (motivo_codigo IN ('smtp_restaurado','reintento_operativo')),
    ocurrida_en timestamptz NOT NULL,
    correlacion_id text NOT NULL,
    clave_idempotencia text NOT NULL,
    retirar_en timestamptz NOT NULL,
    UNIQUE(aviso_id,clave_idempotencia),
    CHECK (retirar_en=ocurrida_en + interval '5 years')
);

CREATE FUNCTION public.enqueue_local_notice(p_type text,p_aggregate_type text,p_aggregate uuid,p_recipient uuid,p_key text,p_at timestamptz)
RETURNS void LANGUAGE plpgsql AS $$
DECLARE notice_id uuid := gen_random_uuid();
BEGIN
    IF p_recipient IS NULL THEN RETURN; END IF;
    INSERT INTO public.aviso_local(id,tipo_evento,agregado_tipo,agregado_id,destinatario_id,clave_deduplicacion,creada_en,proximo_intento_en)
    VALUES(notice_id,p_type,p_aggregate_type,p_aggregate,p_recipient,p_key,p_at,p_at)
    ON CONFLICT(clave_deduplicacion) DO NOTHING;
    IF EXISTS (SELECT 1 FROM public.aviso_local WHERE clave_deduplicacion=p_key) THEN
      INSERT INTO public.aviso_local_ciclo(aviso_id,ciclo,estado,iniciada_en)
      SELECT id,ciclo,'pendiente',creada_en FROM public.aviso_local WHERE clave_deduplicacion=p_key
      ON CONFLICT DO NOTHING;
    END IF;
END $$;

CREATE FUNCTION public.enqueue_local_checkin_notice() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE host uuid; BEGIN
  IF NEW.tipo='checkin' THEN
    SELECT anfitrion_id INTO host FROM public.reserva_ensayo_local WHERE id=NEW.reserva_id;
    PERFORM public.enqueue_local_notice('checkin_registrado','reserva',NEW.reserva_id,host,'checkin:'||NEW.id::text,NEW.ocurrio_en);
  END IF; RETURN NEW;
END $$;
CREATE TRIGGER local_checkin_notice_trg AFTER INSERT ON public.operacion_arriendo_ensayo_local FOR EACH ROW EXECUTE FUNCTION public.enqueue_local_checkin_notice();

CREATE FUNCTION public.enqueue_local_claim_notice() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  PERFORM public.enqueue_local_notice('reclamo_abierto','reserva',NEW.reserva_id,NEW.arrendatario_id,'claim:'||NEW.id::text,NEW.abierto_en);
  RETURN NEW;
END $$;
CREATE TRIGGER local_claim_notice_trg AFTER INSERT ON public.reclamo_dano_ensayo_local FOR EACH ROW EXECUTE FUNCTION public.enqueue_local_claim_notice();

CREATE FUNCTION public.enqueue_local_cancel_notice() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF OLD.estado NOT IN ('cancelada_por_pago','rechazada_arrendador','vencida_pago','vencida_host','cancelada_arrendatario','cancelada_por_firma') AND NEW.estado IN ('cancelada_por_pago','rechazada_arrendador','vencida_pago','vencida_host','cancelada_arrendatario','cancelada_por_firma') THEN
    PERFORM public.enqueue_local_notice('reserva_cancelada','reserva',NEW.id,NEW.anfitrion_id,'cancel:'||NEW.id::text||':'||NEW.estado||':host',NEW.actualizada_en);
    PERFORM public.enqueue_local_notice('reserva_cancelada','reserva',NEW.id,NEW.arrendatario_id,'cancel:'||NEW.id::text||':'||NEW.estado||':renter',NEW.actualizada_en);
  END IF; RETURN NEW;
END $$;
CREATE TRIGGER local_cancel_notice_trg AFTER UPDATE OF estado ON public.reserva_ensayo_local FOR EACH ROW EXECUTE FUNCTION public.enqueue_local_cancel_notice();

CREATE FUNCTION public.enqueue_local_review_report_notice() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE admin record; BEGIN
  FOR admin IN SELECT ru.usuario_id FROM public.rol_usuario ru JOIN public.usuario u ON u.id=ru.usuario_id WHERE ru.rol='administrador' AND u.estado='activo' LOOP
    PERFORM public.enqueue_local_notice('resena_reportada','reporte_resena',NEW.id,admin.usuario_id,'review-report:'||NEW.id::text||':'||admin.usuario_id::text,NEW.creada_en);
  END LOOP; RETURN NEW;
END $$;
CREATE TRIGGER local_review_report_notice_trg AFTER INSERT ON public.reporte_resena_ensayo_local FOR EACH ROW EXECUTE FUNCTION public.enqueue_local_review_report_notice();

ALTER TABLE public.evento_auditoria_local DROP CONSTRAINT evento_auditoria_idempotencia_ck;
ALTER TABLE public.evento_auditoria_local ADD CONSTRAINT evento_auditoria_idempotencia_ck CHECK (
    clave_idempotencia IS NULL OR (
        length(btrim(clave_idempotencia)) BETWEEN 1 AND 200
        AND ((accion IN ('privacy.suppression.review','privacy.suppression.execute') AND recurso_tipo='solicitud_titular')
          OR (accion='identity.credential_notice.reopen' AND recurso_tipo='outbox_evento')
          OR (accion='reputation.review.moderate' AND recurso_tipo='reporte_resena'))
    )
);

-- Bounded retention worker API; application role gets EXECUTE, not unrestricted deletes.
CREATE FUNCTION public.purge_expired_local_communication(p_at timestamptz,p_limit integer)
RETURNS TABLE(reviews_purged bigint,reports_purged bigint,notices_purged bigint)
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public AS $$
DECLARE ids uuid[];
BEGIN
  IF p_limit<1 OR p_limit>500 THEN RAISE EXCEPTION 'invalid batch'; END IF;
  SELECT array_agg(id) INTO ids FROM (
    SELECT r.id FROM public.reporte_resena_ensayo_local r
    WHERE r.estado IN ('desestimada','oculta') AND r.retirar_en<=p_at
    ORDER BY r.retirar_en,r.id FOR UPDATE SKIP LOCKED LIMIT p_limit
  ) due;
  IF cardinality(ids)>0 THEN
    DELETE FROM public.reporte_resena_historial_local WHERE reporte_id=ANY(ids);
    DELETE FROM public.reporte_resena_ensayo_local WHERE id=ANY(ids);
    GET DIAGNOSTICS reports_purged = ROW_COUNT;
  ELSE reports_purged := 0; END IF;

  SELECT array_agg(id) INTO ids FROM (
    SELECT r.id FROM public.resena_ensayo_local r
    WHERE r.retirar_en<=p_at AND NOT EXISTS(SELECT 1 FROM public.reporte_resena_ensayo_local q WHERE q.resena_id=r.id)
    ORDER BY r.retirar_en,r.id FOR UPDATE SKIP LOCKED LIMIT p_limit
  ) due;
  IF cardinality(ids)>0 THEN
    DELETE FROM public.resena_ensayo_local WHERE id=ANY(ids);
    GET DIAGNOSTICS reviews_purged = ROW_COUNT;
  ELSE reviews_purged := 0; END IF;

  SELECT array_agg(id) INTO ids FROM (
    SELECT a.id FROM public.aviso_local a
    WHERE a.retirar_en<=p_at AND a.estado IN ('entregada','fallo_terminal','cancelada') AND a.lease_hasta IS NULL
    ORDER BY a.retirar_en,a.id FOR UPDATE SKIP LOCKED LIMIT p_limit
  ) due;
  IF cardinality(ids)>0 THEN
    DELETE FROM public.aviso_local_ciclo WHERE aviso_id=ANY(ids);
    DELETE FROM public.aviso_local WHERE id=ANY(ids);
    GET DIAGNOSTICS notices_purged = ROW_COUNT;
  ELSE notices_purged := 0; END IF;
  DELETE FROM public.aviso_local_recuperacion_auditoria
  WHERE id IN (SELECT id FROM public.aviso_local_recuperacion_auditoria ORDER BY retirar_en,id LIMIT p_limit)
    AND retirar_en<=p_at;
  RETURN NEXT;
END $$;
REVOKE ALL ON FUNCTION public.purge_expired_local_communication(timestamptz,integer) FROM PUBLIC;

COMMENT ON TABLE public.resena_ensayo_local IS 'Synthetic local reviews; remove after 24 calendar months from creation. Not a legal retention determination.';
COMMENT ON TABLE public.reporte_resena_ensayo_local IS 'Synthetic local moderation report; retain while pending and 24 months after resolution. Not a legal retention determination.';
COMMENT ON TABLE public.aviso_local IS 'Durable local Mailpit notice intent. SMTP may duplicate after uncertain acceptance; no exactly-once guarantee.';
