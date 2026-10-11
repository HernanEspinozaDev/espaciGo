-- LOCAL-CLOSE-01: ratified local retention for synthetic contract documents
-- and publication/deletion history. Academic reference documents stay frozen.

ALTER TABLE public.contrato_ensayo_local
    ALTER COLUMN documento_id DROP NOT NULL,
    ADD COLUMN pdf_retirado_en timestamptz,
    ADD CONSTRAINT contrato_documento_retirado_ck CHECK (
      (documento_id IS NOT NULL AND pdf_retirado_en IS NULL)
      OR (documento_id IS NULL AND pdf_retirado_en IS NOT NULL)
    );

ALTER TABLE public.espacio_publicacion_historial_local
    ADD COLUMN retirar_en timestamptz;
ALTER TABLE public.espacio_eliminacion_historial_local
    ADD COLUMN retirar_en timestamptz;

CREATE OR REPLACE FUNCTION public.prevent_local_publication_history_mutation() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
  IF current_setting('app.local_retention_purge',true)='on' THEN
    IF TG_OP='DELETE' THEN RETURN OLD; END IF;
    IF (to_jsonb(OLD)-'retirar_en') IS DISTINCT FROM (to_jsonb(NEW)-'retirar_en') THEN
      RAISE EXCEPTION 'only retention metadata may change' USING ERRCODE='55000';
    END IF;
    RETURN NEW;
  END IF;
  RAISE EXCEPTION 'local publication history is append-only' USING ERRCODE='55000';
END;
$$;
CREATE OR REPLACE FUNCTION public.prevent_local_space_deletion_history_mutation() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
  IF current_setting('app.local_retention_purge',true)='on' THEN
    IF TG_OP='DELETE' THEN RETURN OLD; END IF;
    IF (to_jsonb(OLD)-'retirar_en') IS DISTINCT FROM (to_jsonb(NEW)-'retirar_en') THEN
      RAISE EXCEPTION 'only retention metadata may change' USING ERRCODE='55000';
    END IF;
    RETURN NEW;
  END IF;
  RAISE EXCEPTION 'local space deletion history is append-only' USING ERRCODE='55000';
END;
$$;
SELECT set_config('app.local_retention_purge','on',true);

UPDATE public.espacio_eliminacion_historial_local
   SET retirar_en=ocurrida_en + interval '24 months';
UPDATE public.espacio_publicacion_historial_local p
   SET retirar_en=d.ocurrida_en + interval '24 months'
  FROM public.espacio_eliminacion_historial_local d
 WHERE d.espacio_id=p.espacio_id;
SELECT set_config('app.local_retention_purge','off',true);

CREATE INDEX espacio_publicacion_historial_retention_idx
    ON public.espacio_publicacion_historial_local(retirar_en,id)
    WHERE retirar_en IS NOT NULL;
CREATE INDEX espacio_eliminacion_historial_retention_idx
    ON public.espacio_eliminacion_historial_local(retirar_en,id)
    WHERE retirar_en IS NOT NULL;

-- These append-only tables allow only a system-scheduled retirement timestamp
-- and bounded deletion from the SECURITY DEFINER retention function below.
CREATE OR REPLACE FUNCTION public.prevent_local_publication_history_mutation() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
  IF current_setting('app.local_retention_purge',true)='on' THEN
    IF TG_OP='DELETE' THEN RETURN OLD; END IF;
    IF (to_jsonb(OLD)-'retirar_en') IS DISTINCT FROM (to_jsonb(NEW)-'retirar_en') THEN
      RAISE EXCEPTION 'only retention metadata may change' USING ERRCODE='55000';
    END IF;
    RETURN NEW;
  END IF;
  RAISE EXCEPTION 'local publication history is append-only' USING ERRCODE='55000';
END;
$$;
CREATE OR REPLACE FUNCTION public.prevent_local_space_deletion_history_mutation() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
  IF current_setting('app.local_retention_purge',true)='on' AND TG_OP='DELETE' THEN RETURN OLD; END IF;
  RAISE EXCEPTION 'local space deletion history is append-only' USING ERRCODE='55000';
END;
$$;

CREATE OR REPLACE FUNCTION public.assign_local_space_history_retention()
RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public AS $$
BEGIN
  NEW.retirar_en := NEW.ocurrida_en + interval '24 months';
  PERFORM set_config('app.local_retention_purge','on',true);
  UPDATE public.espacio_publicacion_historial_local
     SET retirar_en=NEW.retirar_en WHERE espacio_id=NEW.espacio_id;
  PERFORM set_config('app.local_retention_purge','off',true);
  RETURN NEW;
END;
$$;
CREATE TRIGGER espacio_eliminacion_retention_trg
    BEFORE INSERT ON public.espacio_eliminacion_historial_local
    FOR EACH ROW EXECUTE FUNCTION public.assign_local_space_history_retention();

CREATE OR REPLACE FUNCTION public.local_retention_reservation_has_open_obligation(p_reservation uuid)
RETURNS boolean LANGUAGE sql STABLE SECURITY DEFINER SET search_path=pg_catalog,public AS $$
SELECT
  EXISTS(SELECT 1 FROM public.reserva_pago_ensayo_operacion p WHERE p.reserva_id=p_reservation AND p.estado='pendiente')
  OR EXISTS(SELECT 1 FROM public.reserva_pago_fake_resultado_ensayo f JOIN public.reserva_pago_ensayo_operacion p ON p.id=f.operacion_id WHERE p.reserva_id=p_reservation AND f.estado='resultado' AND NOT EXISTS(SELECT 1 FROM public.reserva_pago_evento_ensayo e WHERE e.operacion_id=p.id AND e.proveedor_evento_id=f.proveedor_evento_id))
  OR EXISTS(SELECT 1 FROM public.reserva_pago_evento_ensayo e JOIN public.reserva_pago_ensayo_operacion p ON p.id=e.operacion_id WHERE p.reserva_id=p_reservation AND (NOT EXISTS(SELECT 1 FROM public.reserva_pago_evento_aplicacion_ensayo a WHERE a.evento_id=e.id) OR EXISTS(SELECT 1 FROM public.reserva_pago_evento_aplicacion_ensayo a WHERE a.evento_id=e.id AND a.estado IN ('pendiente','pendiente_conciliacion'))))
  OR EXISTS(SELECT 1 FROM public.reserva_devolucion_ensayo d WHERE d.reserva_id=p_reservation AND d.estado='pendiente')
  OR EXISTS(SELECT 1 FROM public.reserva_garantia_ensayo_local g WHERE g.reserva_id=p_reservation AND (g.estado IN ('pendiente_pago','pendiente_autorizacion','por_conciliar','captura_pendiente','captura_por_conciliar','liberacion_pendiente','liberacion_por_conciliar') OR g.autorizado_clp>g.capturado_clp+g.liberado_clp))
  OR EXISTS(SELECT 1 FROM public.reserva_garantia_operacion_ensayo_local o JOIN public.reserva_garantia_ensayo_local g ON g.id=o.garantia_id WHERE g.reserva_id=p_reservation AND o.estado IN ('pendiente','por_conciliar'))
  OR EXISTS(SELECT 1 FROM public.reserva_garantia_evento_aplicacion_ensayo_local a JOIN public.reserva_garantia_evento_ensayo_local e ON e.id=a.evento_id JOIN public.reserva_garantia_operacion_ensayo_local o ON o.id=e.operacion_id JOIN public.reserva_garantia_ensayo_local g ON g.id=o.garantia_id WHERE g.reserva_id=p_reservation AND a.estado IN ('pendiente','por_conciliar'))
  OR EXISTS(SELECT 1 FROM public.reserva_decision_financiera_ensayo_local d WHERE d.reserva_id=p_reservation AND d.estado IN ('pendiente','por_conciliar'))
  OR EXISTS(SELECT 1 FROM public.reserva_liquidacion_ensayo_local s WHERE s.reserva_id=p_reservation AND s.estado IN ('pendiente','por_conciliar'))
  OR EXISTS(SELECT 1 FROM public.reserva_ensayo_local r WHERE r.id=p_reservation AND r.estado='finalizada' AND r.liquidacion_politica_version='liquidacion_local_0pct_v1'
       AND EXISTS(SELECT 1 FROM public.operacion_arriendo_ensayo_local checkout WHERE checkout.reserva_id=r.id AND checkout.tipo='checkout')
       AND COALESCE((SELECT sum(p.importe_clp) FROM public.reserva_pago_ensayo p WHERE p.reserva_id=r.id AND p.resultado='exito_simulado'),0) > COALESCE((SELECT sum(d.importe_clp) FROM public.reserva_devolucion_ensayo d WHERE d.reserva_id=r.id AND d.estado='completada'),0)
       AND NOT EXISTS(SELECT 1 FROM public.reserva_liquidacion_ensayo_local s WHERE s.reserva_id=r.id))
  OR EXISTS(SELECT 1 FROM public.disputa_ensayo_local d WHERE d.reserva_id=p_reservation AND d.estado='abierta')
  OR EXISTS(SELECT 1 FROM public.reclamo_dano_ensayo_local c WHERE c.reserva_id=p_reservation AND c.estado='abierto')
  OR EXISTS(SELECT 1 FROM public.operacion_arriendo_archivo_candidato_local f WHERE f.reserva_id=p_reservation AND f.estado IN ('reservado','pendiente_limpieza','limpiando'));
$$;
REVOKE ALL ON FUNCTION public.local_retention_reservation_has_open_obligation(uuid) FROM PUBLIC;

-- Do not rely only on reserva_ensayo_local.vinculos_retirar_en: a related
-- terminal event can be committed after a prior purge batch scheduled it.
CREATE OR REPLACE FUNCTION public.local_contract_effective_retention_deadline(p_reservation uuid)
RETURNS timestamptz LANGUAGE sql STABLE SECURITY DEFINER SET search_path=pg_catalog,public AS $$
SELECT GREATEST(r.actualizada_en,
  COALESCE((SELECT max(p.actualizada_en) FROM public.reserva_pago_ensayo_operacion p WHERE p.reserva_id=r.id),r.actualizada_en),
  COALESCE((SELECT max(d.actualizada_en) FROM public.reserva_devolucion_ensayo d WHERE d.reserva_id=r.id),r.actualizada_en),
  COALESCE((SELECT max(d.cerrada_en) FROM public.disputa_ensayo_local d WHERE d.reserva_id=r.id),r.actualizada_en),
  COALESCE((SELECT max(x.resuelta_en) FROM public.reclamo_dano_resolucion_ensayo_local x JOIN public.reclamo_dano_ensayo_local c ON c.id=x.reclamo_id WHERE c.reserva_id=r.id),r.actualizada_en),
  COALESCE((SELECT max(s.actualizada_en) FROM public.reserva_liquidacion_ensayo_local s WHERE s.reserva_id=r.id),r.actualizada_en)
)+interval '24 months'
FROM public.reserva_ensayo_local r WHERE r.id=p_reservation;
$$;
REVOKE ALL ON FUNCTION public.local_contract_effective_retention_deadline(uuid) FROM PUBLIC;

CREATE OR REPLACE FUNCTION public.purge_expired_local_contracts_and_publication_history(p_at timestamptz,p_limit integer)
RETURNS TABLE(contracts_purged bigint,publication_history_purged bigint)
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public AS $$
DECLARE candidate record; space_candidate record; host_id uuid; renter_id uuid; document_id uuid;
        effective_deadline timestamptz; selected_space_ids uuid[]:=ARRAY[]::uuid[];
        selected_deletion_ids bigint[]:=ARRAY[]::bigint[]; locked_deletion_id bigint;
BEGIN
  IF p_limit<1 OR p_limit>500 THEN RAISE EXCEPTION 'invalid batch'; END IF;
  contracts_purged:=0; publication_history_purged:=0;
  PERFORM set_config('app.local_retention_purge','on',true);

  -- Lock participants in stable order, then the reservation and its contract.
  -- A rolled-back invocation leaves the DB-resident PDF and metadata intact;
  -- a later pass retries the complete removal transactionally.
  FOR candidate IN
    SELECT r.id,r.anfitrion_id,r.arrendatario_id,r.vinculos_retirar_en
      FROM public.reserva_ensayo_local r
      JOIN public.contrato_ensayo_local c ON c.reserva_id=r.id
     WHERE COALESCE(r.vinculos_retirar_en,public.local_contract_effective_retention_deadline(r.id))<=p_at
       AND r.estado IN ('finalizada','cerrada','cancelada_por_pago','rechazada_arrendador','vencida_pago','vencida_host','cancelada_arrendatario','cancelada_por_firma','en_disputa')
     ORDER BY r.vinculos_retirar_en,r.id
     LIMIT p_limit
  LOOP
    host_id:=candidate.anfitrion_id; renter_id:=candidate.arrendatario_id;
    PERFORM u.id FROM public.usuario u
     WHERE u.id=ANY(ARRAY[host_id,renter_id]::uuid[])
     ORDER BY u.id FOR UPDATE;
    PERFORM r.id FROM public.reserva_ensayo_local r WHERE r.id=candidate.id FOR UPDATE;
    IF NOT EXISTS(SELECT 1 FROM public.reserva_ensayo_local r WHERE r.id=candidate.id AND r.vinculos_retirar_en<=p_at
      AND r.estado IN ('finalizada','cerrada','cancelada_por_pago','rechazada_arrendador','vencida_pago','vencida_host','cancelada_arrendatario','cancelada_por_firma','en_disputa')) THEN
      CONTINUE;
    END IF;
    IF NOT EXISTS(SELECT 1 FROM public.contrato_ensayo_local c WHERE c.reserva_id=candidate.id) THEN
      CONTINUE;
    END IF;
    SELECT public.local_contract_effective_retention_deadline(candidate.id) INTO effective_deadline;
    IF effective_deadline IS NULL THEN CONTINUE; END IF;
    IF candidate.vinculos_retirar_en IS NULL OR candidate.vinculos_retirar_en<effective_deadline THEN
      UPDATE public.reserva_ensayo_local SET vinculos_retirar_en=effective_deadline WHERE id=candidate.id;
    ELSE
      effective_deadline:=GREATEST(effective_deadline,candidate.vinculos_retirar_en);
    END IF;
    IF effective_deadline>p_at THEN CONTINUE; END IF;
    IF public.local_retention_reservation_has_open_obligation(candidate.id) THEN
      CONTINUE;
    END IF;
    SELECT c.documento_id INTO document_id FROM public.contrato_ensayo_local c WHERE c.reserva_id=candidate.id FOR UPDATE;
    DELETE FROM public.contrato_ensayo_firma WHERE contrato_id IN (SELECT id FROM public.contrato_ensayo_local WHERE reserva_id=candidate.id);
    DELETE FROM public.contrato_ensayo_historial WHERE contrato_id IN (SELECT id FROM public.contrato_ensayo_local WHERE reserva_id=candidate.id);
    DELETE FROM public.contrato_ensayo_local WHERE reserva_id=candidate.id;
    IF document_id IS NOT NULL THEN DELETE FROM public.documento_privado_sintetico_local WHERE id=document_id; END IF;
    contracts_purged:=contracts_purged+1;
  END LOOP;

  -- Select due spaces once, skip known blockers before applying the batch
  -- limit, then lock participants/reservations and revalidate under those
  -- locks. Both append-only ledgers are removed from this exact set.
  FOR space_candidate IN
    SELECT h.id,h.espacio_id
      FROM public.espacio_eliminacion_historial_local h
     WHERE h.retirar_en<=p_at
       AND NOT EXISTS(SELECT 1 FROM public.reserva_ensayo_local r WHERE r.espacio_id=h.espacio_id AND (
         r.estado NOT IN ('finalizada','cerrada','cancelada_por_pago','rechazada_arrendador','vencida_pago','vencida_host','cancelada_arrendatario','cancelada_por_firma')
         OR GREATEST(COALESCE(r.vinculos_retirar_en,'epoch'::timestamptz),public.local_contract_effective_retention_deadline(r.id))>p_at
         OR public.local_retention_reservation_has_open_obligation(r.id)))
     ORDER BY h.retirar_en,h.id
     LIMIT p_limit
  LOOP
    PERFORM u.id FROM public.usuario u
     WHERE u.id IN (SELECT r.anfitrion_id FROM public.reserva_ensayo_local r WHERE r.espacio_id=space_candidate.espacio_id
                    UNION SELECT r.arrendatario_id FROM public.reserva_ensayo_local r WHERE r.espacio_id=space_candidate.espacio_id)
     ORDER BY u.id FOR UPDATE;
    PERFORM r.id FROM public.reserva_ensayo_local r WHERE r.espacio_id=space_candidate.espacio_id ORDER BY r.id FOR UPDATE;
    SELECT h.id INTO locked_deletion_id FROM public.espacio_eliminacion_historial_local h
     WHERE h.id=space_candidate.id AND h.retirar_en<=p_at FOR UPDATE SKIP LOCKED;
    IF locked_deletion_id IS NULL THEN CONTINUE; END IF;
    IF EXISTS(SELECT 1 FROM public.reserva_ensayo_local r WHERE r.espacio_id=space_candidate.espacio_id AND (
         r.estado NOT IN ('finalizada','cerrada','cancelada_por_pago','rechazada_arrendador','vencida_pago','vencida_host','cancelada_arrendatario','cancelada_por_firma')
         OR GREATEST(COALESCE(r.vinculos_retirar_en,'epoch'::timestamptz),public.local_contract_effective_retention_deadline(r.id))>p_at
         OR public.local_retention_reservation_has_open_obligation(r.id))) THEN
      CONTINUE;
    END IF;
    selected_space_ids:=array_append(selected_space_ids,space_candidate.espacio_id);
    selected_deletion_ids:=array_append(selected_deletion_ids,space_candidate.id);
  END LOOP;
  IF cardinality(selected_space_ids)>0 THEN
    DELETE FROM public.espacio_publicacion_historial_local p WHERE p.espacio_id=ANY(selected_space_ids);
    GET DIAGNOSTICS publication_history_purged=ROW_COUNT;
    DELETE FROM public.espacio_eliminacion_historial_local h WHERE h.id=ANY(selected_deletion_ids);
  END IF;
  PERFORM set_config('app.local_retention_purge','off',true);
  RETURN NEXT;
END;
$$;

REVOKE ALL ON FUNCTION public.purge_expired_local_contracts_and_publication_history(timestamptz,integer) FROM PUBLIC;
