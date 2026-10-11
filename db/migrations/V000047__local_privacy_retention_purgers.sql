-- LOCAL-CLOSE-01B: execute already-ratified local retention deadlines.
-- The private blob is unlinked only after the durable job succeeds; KYC
-- metadata and history remain available until their independent two-year date.
ALTER TABLE public.verificacion_evidencia_sintetica
    ADD COLUMN archivo_id uuid,
    ADD COLUMN retirar_archivo_en timestamptz,
    ADD COLUMN archivo_retirado_en timestamptz;

UPDATE public.verificacion_evidencia_sintetica e
SET archivo_id=e.id,
    retirar_archivo_en=CASE
      WHEN v.estado='retirada_privacidad' THEN v.retirada_privacidad_en
      WHEN v.resuelta_en IS NOT NULL THEN v.resuelta_en + interval '90 days'
      ELSE NULL END
FROM public.verificacion v WHERE v.id=e.verificacion_id;

-- Existing evidence rows receive their storage reference and deadline before
-- the invariant is installed. Validate it explicitly so upgrades from V46 do
-- not fail while legacy rows are still being backfilled.
ALTER TABLE public.verificacion_evidencia_sintetica
    ADD CONSTRAINT verificacion_evidencia_archivo_retencion_ck CHECK (
        (archivo_id IS NULL AND archivo_retirado_en IS NOT NULL)
        OR (archivo_id IS NOT NULL AND archivo_retirado_en IS NULL)
    ) NOT VALID;
ALTER TABLE public.verificacion_evidencia_sintetica
    VALIDATE CONSTRAINT verificacion_evidencia_archivo_retencion_ck;

CREATE INDEX verificacion_evidencia_archivo_retencion_idx
    ON public.verificacion_evidencia_sintetica(retirar_archivo_en,id)
    WHERE archivo_id IS NOT NULL AND retirar_archivo_en IS NOT NULL;

CREATE TABLE public.verificacion_archivo_retiro_pendiente_local (
    evidencia_id uuid PRIMARY KEY,
    archivo_id uuid NOT NULL,
    disponible_en timestamptz NOT NULL,
    lease_hasta timestamptz,
    intentos integer NOT NULL DEFAULT 0 CHECK (intentos >= 0),
    ultimo_codigo_error text,
    CONSTRAINT verificacion_archivo_retiro_error_ck CHECK (
        ultimo_codigo_error IS NULL OR ultimo_codigo_error='archivo_sintetico_no_retirado'
    )
);
CREATE INDEX verificacion_archivo_retiro_claim_idx
    ON public.verificacion_archivo_retiro_pendiente_local(disponible_en,evidencia_id);

CREATE OR REPLACE FUNCTION public.set_verification_evidence_retention_deadline()
RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public AS $$
BEGIN
  IF NEW.estado='retirada_privacidad' AND NEW.retirada_privacidad_en IS NOT NULL THEN
    UPDATE public.verificacion_evidencia_sintetica
       SET retirar_archivo_en=LEAST(COALESCE(retirar_archivo_en,NEW.retirada_privacidad_en),NEW.retirada_privacidad_en)
     WHERE verificacion_id=NEW.id AND archivo_id IS NOT NULL;
  ELSIF NEW.estado IN ('aprobada','rechazada','revocada') AND NEW.resuelta_en IS NOT NULL THEN
    UPDATE public.verificacion_evidencia_sintetica
       SET retirar_archivo_en=NEW.resuelta_en + interval '90 days'
     WHERE verificacion_id=NEW.id AND archivo_id IS NOT NULL;
  END IF;
  RETURN NEW;
END;
$$;
CREATE TRIGGER verificacion_evidencia_retencion_trg
AFTER UPDATE OF estado,resuelta_en,retirada_privacidad_en ON public.verificacion
FOR EACH ROW EXECUTE FUNCTION public.set_verification_evidence_retention_deadline();

CREATE OR REPLACE FUNCTION public.purge_expired_local_privacy_rows(p_at timestamptz,p_limit integer)
RETURNS TABLE(terms_purged bigint,rights_purged bigint,quotes_purged bigint,verifications_purged bigint,files_queued bigint)
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public AS $$
DECLARE ids uuid[]; owner_ids uuid[]; verification_ids uuid[];
BEGIN
  IF p_limit<1 OR p_limit>500 THEN RAISE EXCEPTION 'invalid batch'; END IF;
  terms_purged:=0; rights_purged:=0; quotes_purged:=0; verifications_purged:=0; files_queued:=0;

  INSERT INTO public.verificacion_archivo_retiro_pendiente_local(evidencia_id,archivo_id,disponible_en)
  SELECT e.id,e.archivo_id,p_at
    FROM public.verificacion_evidencia_sintetica e JOIN public.verificacion v ON v.id=e.verificacion_id
   WHERE e.archivo_id IS NOT NULL
     AND v.estado<>'retirada_privacidad'
     AND (e.retirar_archivo_en<=p_at OR v.retirar_en<=p_at)
     AND NOT EXISTS(SELECT 1 FROM public.verificacion_archivo_retiro_pendiente_local j WHERE j.evidencia_id=e.id)
   ORDER BY COALESCE(e.retirar_archivo_en,v.retirar_en),e.id
   LIMIT p_limit
  ON CONFLICT(evidencia_id) DO NOTHING;
  GET DIAGNOSTICS files_queued=ROW_COUNT;

  WITH due AS (
    SELECT id FROM public.aceptacion_terminos WHERE retirar_en<=p_at
    ORDER BY retirar_en,id FOR UPDATE SKIP LOCKED LIMIT p_limit
  ) DELETE FROM public.aceptacion_terminos a USING due WHERE a.id=due.id;
  GET DIAGNOSTICS terms_purged=ROW_COUNT;

  -- A completed suppression's operation record shares its five-year period.
  -- Remove only completed filesystem jobs first; pending work has no deadline.
  SELECT array_agg(id) INTO ids FROM (
    SELECT id FROM public.ejecucion_baja_local WHERE retirar_en<=p_at
    ORDER BY retirar_en,id FOR UPDATE SKIP LOCKED LIMIT p_limit
  ) due;
  IF cardinality(ids)>0 THEN
    DELETE FROM public.baja_archivo_pendiente_local WHERE ejecucion_id=ANY(ids) AND completada_en IS NOT NULL;
    DELETE FROM public.ejecucion_baja_local WHERE id=ANY(ids);
  END IF;
  WITH due AS (
    SELECT q.id FROM public.solicitud_titular q
    WHERE q.retirar_en<=p_at
      AND NOT EXISTS(SELECT 1 FROM public.ejecucion_baja_local x WHERE x.solicitud_id=q.id)
    ORDER BY q.retirar_en,q.id FOR UPDATE SKIP LOCKED LIMIT p_limit
  ) DELETE FROM public.solicitud_titular q USING due WHERE q.id=due.id;
  GET DIAGNOSTICS rights_purged=ROW_COUNT;

  WITH due AS (
    SELECT q.id FROM public.cotizacion_reserva_ensayo q
    WHERE q.retirar_en<=p_at
      AND NOT EXISTS(SELECT 1 FROM public.reserva_ensayo_local r WHERE r.cotizacion_id=q.id)
    ORDER BY q.retirar_en,q.id FOR UPDATE SKIP LOCKED LIMIT p_limit
  ) DELETE FROM public.cotizacion_reserva_ensayo q USING due WHERE q.id=due.id;
  GET DIAGNOSTICS quotes_purged=ROW_COUNT;

  -- Follow the account -> verification lock order used by verification and
  -- suppression writers. SKIP LOCKED makes concurrent passes retryable.
  SELECT array_agg(id) INTO owner_ids FROM (
    SELECT u.id FROM public.usuario u
    WHERE EXISTS(SELECT 1 FROM public.verificacion v WHERE v.usuario_id=u.id
      AND v.retirar_en<=p_at AND v.estado IN ('aprobada','rechazada','revocada','retirada_privacidad'))
    ORDER BY u.id FOR UPDATE SKIP LOCKED LIMIT p_limit
  ) owners;
  IF cardinality(owner_ids)>0 THEN
    SELECT array_agg(id) INTO verification_ids FROM (
      SELECT v.id FROM public.verificacion v
      WHERE v.usuario_id=ANY(owner_ids) AND v.retirar_en<=p_at
        AND v.estado IN ('aprobada','rechazada','revocada','retirada_privacidad')
        AND NOT EXISTS(SELECT 1 FROM public.verificacion_evidencia_sintetica e
                       WHERE e.verificacion_id=v.id AND e.archivo_id IS NOT NULL)
        AND NOT EXISTS(SELECT 1 FROM public.ejecucion_baja_local b
                       WHERE b.usuario_id=v.usuario_id AND b.estado='limpieza_pendiente')
      ORDER BY v.usuario_id,v.retirar_en,v.id FOR UPDATE SKIP LOCKED LIMIT p_limit
    ) due_verifications;
  END IF;
  IF cardinality(verification_ids)>0 THEN
    UPDATE public.verificacion SET reintento_de=NULL WHERE reintento_de=ANY(verification_ids);
    DELETE FROM public.elegibilidad_verificacion_local WHERE verificacion_id=ANY(verification_ids);
    DELETE FROM public.verificacion_historial_local WHERE verificacion_id=ANY(verification_ids);
    DELETE FROM public.verificacion_evidencia_sintetica WHERE verificacion_id=ANY(verification_ids);
    DELETE FROM public.verificacion WHERE id=ANY(verification_ids);
    GET DIAGNOSTICS verifications_purged=ROW_COUNT;
  END IF;
  RETURN NEXT;
END;
$$;

CREATE OR REPLACE FUNCTION public.claim_expired_local_verification_files(p_at timestamptz,p_limit integer)
RETURNS TABLE(evidencia_id uuid,archivo_id uuid)
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public AS $$
BEGIN
  IF p_limit<1 OR p_limit>500 THEN RAISE EXCEPTION 'invalid batch'; END IF;
  RETURN QUERY
  WITH due AS (
    SELECT j.evidencia_id FROM public.verificacion_archivo_retiro_pendiente_local j
    WHERE j.disponible_en<=p_at AND (j.lease_hasta IS NULL OR j.lease_hasta<=p_at)
    ORDER BY j.disponible_en,j.evidencia_id FOR UPDATE SKIP LOCKED LIMIT p_limit
  ), claimed AS (
    UPDATE public.verificacion_archivo_retiro_pendiente_local j
       SET lease_hasta=p_at+interval '5 minutes',intentos=intentos+1
      FROM due WHERE j.evidencia_id=due.evidencia_id
    RETURNING j.evidencia_id,j.archivo_id
  ) SELECT claimed.evidencia_id,claimed.archivo_id FROM claimed;
END;
$$;

CREATE OR REPLACE FUNCTION public.complete_expired_local_verification_file(p_evidencia uuid,p_at timestamptz)
RETURNS boolean LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public AS $$
DECLARE file_id uuid;
BEGIN
  SELECT archivo_id INTO file_id FROM public.verificacion_archivo_retiro_pendiente_local
   WHERE evidencia_id=p_evidencia FOR UPDATE;
  IF file_id IS NULL THEN RETURN false; END IF;
  UPDATE public.verificacion_evidencia_sintetica
     SET archivo_id=NULL,archivo_retirado_en=p_at
   WHERE id=p_evidencia AND archivo_id=file_id;
  DELETE FROM public.verificacion_archivo_retiro_pendiente_local WHERE evidencia_id=p_evidencia;
  RETURN true;
END;
$$;

CREATE OR REPLACE FUNCTION public.fail_expired_local_verification_file(p_evidencia uuid,p_at timestamptz)
RETURNS boolean LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public AS $$
BEGIN
  UPDATE public.verificacion_archivo_retiro_pendiente_local
     SET intentos=intentos+0,lease_hasta=NULL,
         disponible_en=p_at+LEAST(interval '1 hour',interval '5 seconds' * (1 << LEAST(intentos,9))),
         ultimo_codigo_error='archivo_sintetico_no_retirado'
   WHERE evidencia_id=p_evidencia;
  RETURN FOUND;
END;
$$;

REVOKE ALL ON FUNCTION public.purge_expired_local_privacy_rows(timestamptz,integer) FROM PUBLIC;
REVOKE ALL ON FUNCTION public.claim_expired_local_verification_files(timestamptz,integer) FROM PUBLIC;
REVOKE ALL ON FUNCTION public.complete_expired_local_verification_file(uuid,timestamptz) FROM PUBLIC;
REVOKE ALL ON FUNCTION public.fail_expired_local_verification_file(uuid,timestamptz) FROM PUBLIC;
REVOKE ALL ON TABLE public.verificacion_archivo_retiro_pendiente_local FROM PUBLIC;
COMMENT ON TABLE public.verificacion_archivo_retiro_pendiente_local IS 'Durable, retryable deletion of local synthetic verification blobs; contains no user or provider payload.';
