-- Add the terminal signature-expiry state without changing V53's checksum.
-- Older rows may have NULL link-retention deadlines, so the bounded purge
-- also derives and persists the effective value after acquiring its locks.
CREATE OR REPLACE FUNCTION public.schedule_local_reservation_link_retention() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.estado IN ('finalizada','cerrada','cancelada_por_pago','rechazada_arrendador','vencida_pago','vencida_host','cancelada_arrendatario','cancelada_por_firma')
       AND (TG_OP='INSERT' OR OLD.estado IS DISTINCT FROM NEW.estado) THEN
        NEW.vinculos_retirar_en := GREATEST(COALESCE(NEW.vinculos_retirar_en,'epoch'::timestamptz),NEW.actualizada_en + interval '24 months');
    END IF;
    RETURN NEW;
END;
$$;

DROP INDEX IF EXISTS public.reserva_ensayo_retencion_local_idx;
CREATE INDEX reserva_ensayo_retencion_local_idx ON public.reserva_ensayo_local(vinculos_retirar_en,id)
WHERE estado IN ('finalizada','cerrada','cancelada_por_pago','rechazada_arrendador','vencida_pago','vencida_host','cancelada_arrendatario','cancelada_por_firma');

-- Repair legacy signature-expiry rows so both domain-document cleanup and
-- identity-link minimization can find them in their regular bounded batches.
UPDATE public.reserva_ensayo_local r
   SET vinculos_retirar_en=public.local_contract_effective_retention_deadline(r.id)
 WHERE r.estado='cancelada_por_firma' AND r.vinculos_retirar_en IS NULL;

CREATE OR REPLACE FUNCTION public.purge_expired_local_contracts_and_publication_history(p_at timestamptz,p_limit integer)
RETURNS TABLE(contracts_purged bigint,publication_history_purged bigint)
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public AS $$
DECLARE candidate record; space_candidate record; host_id uuid; renter_id uuid; document_id uuid;
        effective_deadline timestamptz; stored_deadline timestamptz;
        selected_space_ids uuid[]:=ARRAY[]::uuid[];
        selected_deletion_ids bigint[]:=ARRAY[]::bigint[]; locked_deletion_id bigint;
BEGIN
  IF p_limit<1 OR p_limit>500 THEN RAISE EXCEPTION 'invalid batch'; END IF;
  contracts_purged:=0; publication_history_purged:=0;
  PERFORM set_config('app.local_retention_purge','on',true);

  -- Lock participants in stable order, then the reservation. Re-read the
  -- current stored deadline and derive the latest effective close while that
  -- lock is held; never trust a stale/NULL value observed by the batch scan.
  FOR candidate IN
    SELECT r.id,r.anfitrion_id,r.arrendatario_id,r.vinculos_retirar_en
      FROM public.reserva_ensayo_local r
      JOIN public.contrato_ensayo_local c ON c.reserva_id=r.id
     WHERE COALESCE(r.vinculos_retirar_en,public.local_contract_effective_retention_deadline(r.id))<=p_at
       AND r.estado IN ('finalizada','cerrada','cancelada_por_pago','rechazada_arrendador','vencida_pago','vencida_host','cancelada_arrendatario','cancelada_por_firma','en_disputa')
     ORDER BY COALESCE(r.vinculos_retirar_en,public.local_contract_effective_retention_deadline(r.id)),r.id
     LIMIT p_limit
  LOOP
    host_id:=candidate.anfitrion_id; renter_id:=candidate.arrendatario_id;
    PERFORM u.id FROM public.usuario u
     WHERE u.id=ANY(ARRAY[host_id,renter_id]::uuid[])
     ORDER BY u.id FOR UPDATE;
    PERFORM r.id FROM public.reserva_ensayo_local r WHERE r.id=candidate.id FOR UPDATE;
    IF NOT EXISTS(SELECT 1 FROM public.reserva_ensayo_local r WHERE r.id=candidate.id
      AND r.estado IN ('finalizada','cerrada','cancelada_por_pago','rechazada_arrendador','vencida_pago','vencida_host','cancelada_arrendatario','cancelada_por_firma','en_disputa')) THEN
      CONTINUE;
    END IF;
    IF NOT EXISTS(SELECT 1 FROM public.contrato_ensayo_local c WHERE c.reserva_id=candidate.id) THEN
      CONTINUE;
    END IF;
    SELECT r.vinculos_retirar_en INTO stored_deadline FROM public.reserva_ensayo_local r WHERE r.id=candidate.id;
    SELECT public.local_contract_effective_retention_deadline(candidate.id) INTO effective_deadline;
    IF effective_deadline IS NULL THEN CONTINUE; END IF;
    IF stored_deadline IS NOT NULL AND stored_deadline>effective_deadline THEN
      effective_deadline:=stored_deadline;
    END IF;
    IF stored_deadline IS NULL OR stored_deadline<effective_deadline THEN
      UPDATE public.reserva_ensayo_local SET vinculos_retirar_en=effective_deadline WHERE id=candidate.id;
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

  -- Keep V53's single eligible-space set for both related history ledgers.
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
