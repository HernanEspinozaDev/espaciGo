-- LOCAL-CLOSE-01A: finite transactional local notice producers for the
-- ratified synthetic flows. Mailpit may duplicate delivery after uncertainty.
ALTER TABLE public.aviso_local DROP CONSTRAINT aviso_local_tipo_evento_check;
ALTER TABLE public.aviso_local ADD CONSTRAINT aviso_local_tipo_evento_check
  CHECK (tipo_evento IN (
    'checkin_registrado','resena_reportada','reclamo_abierto','reclamo_resuelto','reserva_cancelada',
    'cuenta_bloqueada_login','verificacion_aprobada','verificacion_rechazada','pago_arriendo_confirmado','pago_arriendo_rechazado',
    'nueva_reserva_pagada','invitacion_firma','contrato_firmado'
  ));
ALTER TABLE public.aviso_local DROP CONSTRAINT aviso_local_agregado_tipo_check;
ALTER TABLE public.aviso_local ADD CONSTRAINT aviso_local_agregado_tipo_check
  CHECK (agregado_tipo IN ('reserva','reporte_resena','usuario','verificacion','contrato'));

-- A fifth failed login updates bloqueado_hasta under the account transaction;
-- this trigger inserts only an event code/reference, never email or credentials.
CREATE FUNCTION public.enqueue_login_lock_notice() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF NEW.bloqueado_hasta IS NOT NULL AND NEW.bloqueado_hasta IS DISTINCT FROM OLD.bloqueado_hasta THEN
    PERFORM public.enqueue_local_notice(
      'cuenta_bloqueada_login','usuario',NEW.id,NEW.id,
      'login-lock:'||NEW.id::text||':'||extract(epoch FROM NEW.bloqueado_hasta)::text,
      COALESCE(NEW.actualizado_en,NEW.bloqueado_hasta));
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER local_login_lock_notice_trg AFTER UPDATE OF bloqueado_hasta ON public.usuario
  FOR EACH ROW EXECUTE FUNCTION public.enqueue_login_lock_notice();

CREATE FUNCTION public.enqueue_verification_result_notice() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF OLD.estado='en_revision' AND NEW.estado IN ('aprobada','rechazada') THEN
    PERFORM public.enqueue_local_notice('verificacion_'||NEW.estado,'verificacion',NEW.id,NEW.usuario_id,
      'verification-result:'||NEW.id::text||':'||NEW.estado,NEW.resuelta_en);
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER local_verification_result_notice_trg AFTER UPDATE OF estado ON public.verificacion
  FOR EACH ROW EXECUTE FUNCTION public.enqueue_verification_result_notice();

CREATE FUNCTION public.enqueue_booking_payment_notices() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF OLD.estado IS DISTINCT FROM NEW.estado AND NEW.estado='pagada' THEN
    PERFORM public.enqueue_local_notice('pago_arriendo_confirmado','reserva',NEW.id,NEW.arrendatario_id,
      'rental-payment:'||NEW.id::text||':confirmed',NEW.actualizada_en);
    PERFORM public.enqueue_local_notice('nueva_reserva_pagada','reserva',NEW.id,NEW.anfitrion_id,
      'paid-booking:'||NEW.id::text||':host',NEW.actualizada_en);
  ELSIF OLD.estado IS DISTINCT FROM NEW.estado AND NEW.estado='cancelada_por_pago' THEN
    -- Guarantee rejection/expiry also cancels a previously paid rental. Only
    -- emit RQF-201 when the rental payment operation itself was rejected.
    IF EXISTS (SELECT 1 FROM public.reserva_pago_ensayo p
      WHERE p.reserva_id=NEW.id AND p.resultado='rechazo_simulado') THEN
      PERFORM public.enqueue_local_notice('pago_arriendo_rechazado','reserva',NEW.id,NEW.arrendatario_id,
        'rental-payment:'||NEW.id::text||':rejected',NEW.actualizada_en);
    END IF;
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER local_booking_payment_notice_trg AFTER UPDATE OF estado ON public.reserva_ensayo_local
  FOR EACH ROW EXECUTE FUNCTION public.enqueue_booking_payment_notices();

-- Invitations identify a local resource only. The emailed link contains no
-- session token; the mock/API must authenticate and re-check participant ACL.
CREATE FUNCTION public.enqueue_contract_signature_notices() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE contract_row public.contrato_ensayo_local%ROWTYPE;
BEGIN
  SELECT * INTO contract_row FROM public.contrato_ensayo_local WHERE id=NEW.contrato_id;
  IF NEW.estado='pendiente' THEN
    PERFORM public.enqueue_local_notice('invitacion_firma','contrato',NEW.contrato_id,NEW.firmante_id,
      'contract-invite:'||NEW.contrato_id::text||':'||NEW.firmante_id::text,NEW.actualizada_en);
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER local_contract_signature_notice_trg AFTER INSERT ON public.contrato_ensayo_firma
  FOR EACH ROW EXECUTE FUNCTION public.enqueue_contract_signature_notices();

CREATE FUNCTION public.enqueue_contract_complete_notices() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE reservation_row public.reserva_ensayo_local%ROWTYPE;
BEGIN
  IF OLD.estado IS DISTINCT FROM NEW.estado AND NEW.estado='firmado' THEN
    SELECT * INTO reservation_row FROM public.reserva_ensayo_local WHERE id=NEW.reserva_id;
    PERFORM public.enqueue_local_notice('contrato_firmado','contrato',NEW.id,reservation_row.anfitrion_id,
      'contract-signed:'||NEW.id::text||':'||reservation_row.anfitrion_id::text,NEW.firmado_en);
    PERFORM public.enqueue_local_notice('contrato_firmado','contrato',NEW.id,reservation_row.arrendatario_id,
      'contract-signed:'||NEW.id::text||':'||reservation_row.arrendatario_id::text,NEW.firmado_en);
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER local_contract_complete_notice_trg AFTER UPDATE OF estado ON public.contrato_ensayo_local
  FOR EACH ROW EXECUTE FUNCTION public.enqueue_contract_complete_notices();

COMMENT ON FUNCTION public.enqueue_login_lock_notice() IS 'Durable synthetic login lockout notice. Never stores a password, token or email.';
COMMENT ON FUNCTION public.enqueue_contract_signature_notices() IS 'Authenticated-link invitation event; no bearer secret is placed in the URL.';
