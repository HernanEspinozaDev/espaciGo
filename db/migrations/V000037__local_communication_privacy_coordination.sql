-- LOCAL-COMM-01 follow-up: retain anonymous review uniqueness and serialize
-- review/report/outbox operations with local account suppression.

DO $$
DECLARE c record;
BEGIN
  FOR c IN
    SELECT conname FROM pg_constraint
    WHERE conrelid='public.resena_ensayo_local'::regclass AND contype='c'
      AND pg_get_constraintdef(oid) LIKE '%destinatario_tipo%'
  LOOP
    EXECUTE format('ALTER TABLE public.resena_ensayo_local DROP CONSTRAINT %I', c.conname);
  END LOOP;
END $$;

ALTER TABLE public.resena_ensayo_local
  DROP CONSTRAINT IF EXISTS resena_destinatario_privacidad_ck,
  ADD CONSTRAINT resena_destinatario_privacidad_ck CHECK (
    (destinatario_tipo='espacio' AND destinatario_id IS NULL) OR
    (destinatario_tipo='arrendatario' AND destinatario_id IS NOT NULL) OR
    (destinatario_tipo='arrendatario_retirado' AND destinatario_id IS NULL)
  );
ALTER TABLE public.resena_ensayo_local
  ADD CONSTRAINT resena_destinatario_tipo_ck CHECK (destinatario_tipo IN ('espacio','arrendatario','arrendatario_retirado'));

-- Digest of reservation UUID + author UUID. It contains no review content or
-- directly identifying foreign key and is retained only as long as reservation
-- links can still authorize a new review.
CREATE TABLE public.resena_autoria_marca_local (
  huella_autoria bytea PRIMARY KEY CHECK (octet_length(huella_autoria)=32),
  creada_en timestamptz NOT NULL,
  retirar_en timestamptz
);
INSERT INTO public.resena_autoria_marca_local(huella_autoria,creada_en,retirar_en)
SELECT sha256(convert_to(r.reserva_id::text || ':' || r.autor_id::text,'UTF8')), r.creada_en,
       CASE WHEN b.vinculos_retirar_en IS NULL THEN NULL
            ELSE GREATEST(r.creada_en + interval '24 months',b.vinculos_retirar_en) END
FROM public.resena_ensayo_local r
JOIN public.reserva_ensayo_local b ON b.id=r.reserva_id
WHERE r.autor_id IS NOT NULL
ON CONFLICT DO NOTHING;

CREATE INDEX resena_autoria_marca_retencion_idx ON public.resena_autoria_marca_local(retirar_en,huella_autoria);

CREATE OR REPLACE FUNCTION public.purge_expired_local_communication(p_at timestamptz,p_limit integer)
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

  DELETE FROM public.resena_autoria_marca_local WHERE retirar_en<=p_at;
  DELETE FROM public.aviso_local_recuperacion_auditoria
  WHERE id IN (SELECT id FROM public.aviso_local_recuperacion_auditoria ORDER BY retirar_en,id LIMIT p_limit)
    AND retirar_en<=p_at;
  RETURN NEXT;
END $$;

COMMENT ON TABLE public.resena_autoria_marca_local IS 'Opaque uniqueness marker only; no rating/comment/account identifier. A NULL retirement date means the reservation still permits reviews; otherwise retained through the reservation-link deadline.';
