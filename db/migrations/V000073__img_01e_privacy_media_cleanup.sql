-- IMG-01E: the dedicated privacy cleanup worker may drain only the media
-- cleanup fields and variant rows required by the existing fenced cleaner.
-- It does not receive access to account, publication, or upload operations.
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname='espacigo_privacy_cleanup_worker') THEN
        RAISE EXCEPTION 'dedicated privacy cleanup role is required before IMG-01E';
    END IF;
END
$$;

GRANT SELECT (id,propietario_id,estado,fencing_token,lease_until,updated_at)
    ON public.media_asset TO espacigo_privacy_cleanup_worker;
REVOKE INSERT, UPDATE, DELETE, TRUNCATE ON public.media_asset FROM espacigo_privacy_cleanup_worker;
GRANT SELECT (media_id,variant,provider,bucket_name,object_key,generation,mime_type,size_bytes,sha256,width,height)
    ON public.media_variant TO espacigo_privacy_cleanup_worker;
REVOKE INSERT, UPDATE, DELETE, TRUNCATE ON public.media_variant FROM espacigo_privacy_cleanup_worker;
GRANT SELECT (media_id,variant,object_key,generation,write_complete)
    ON public.media_variant_intent TO espacigo_privacy_cleanup_worker;
REVOKE INSERT, UPDATE, DELETE, TRUNCATE ON public.media_variant_intent FROM espacigo_privacy_cleanup_worker;

CREATE OR REPLACE FUNCTION public.claim_private_media_cleanup(p_limit integer, p_at timestamptz)
RETURNS TABLE(id text, fencing_token bigint)
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, public
SET timezone = 'UTC'
AS $$
BEGIN
    IF pg_catalog.current_setting('role', true) IS DISTINCT FROM 'espacigo_privacy_cleanup_worker' THEN
        RAISE EXCEPTION 'privacy cleanup worker role is required' USING ERRCODE = '42501';
    END IF;
    IF p_limit IS NULL OR p_limit < 1 OR p_limit > 100 OR p_at IS NULL THEN
        RAISE EXCEPTION 'valid cleanup limit and timestamp are required' USING ERRCODE = '22023';
    END IF;
    RETURN QUERY
    WITH claimed AS (
        SELECT a.id FROM public.media_asset a
         WHERE (a.estado='deleting' AND (a.lease_until IS NULL OR a.lease_until <= p_at))
            OR (a.estado='pending' AND a.lease_until <= p_at)
         ORDER BY a.updated_at,a.id FOR UPDATE SKIP LOCKED LIMIT p_limit
    ), changed AS (
        UPDATE public.media_asset a
           SET estado='deleting',fencing_token=a.fencing_token+1,
               lease_until=p_at+interval '5 minutes',updated_at=p_at
          FROM claimed c WHERE a.id=c.id
        RETURNING a.id::text,a.fencing_token
    ) SELECT changed.id,changed.fencing_token FROM changed;
END
$$;

CREATE OR REPLACE FUNCTION public.complete_private_media_cleanup(p_id uuid, p_token bigint, p_at timestamptz)
RETURNS boolean
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, public
SET timezone = 'UTC'
AS $$
BEGIN
    IF pg_catalog.current_setting('role', true) IS DISTINCT FROM 'espacigo_privacy_cleanup_worker' THEN
        RAISE EXCEPTION 'privacy cleanup worker role is required' USING ERRCODE = '42501';
    END IF;
    IF p_at IS NULL THEN
        RAISE EXCEPTION 'cleanup completion time is required' USING ERRCODE = '22004';
    END IF;
    PERFORM 1 FROM public.media_asset
     WHERE id=p_id AND estado='deleting' AND fencing_token=p_token FOR UPDATE;
    IF NOT FOUND THEN RETURN false; END IF;
    DELETE FROM public.media_variant WHERE media_id=p_id;
    DELETE FROM public.media_variant_intent WHERE media_id=p_id;
    UPDATE public.media_asset SET estado='deleted',deleted_at=p_at,lease_until=NULL,updated_at=p_at
     WHERE id=p_id AND estado='deleting' AND fencing_token=p_token;
    RETURN FOUND;
END
$$;

CREATE OR REPLACE FUNCTION public.fail_private_media_cleanup(p_id uuid, p_token bigint, p_at timestamptz)
RETURNS boolean
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, public
SET timezone = 'UTC'
AS $$
BEGIN
    IF pg_catalog.current_setting('role', true) IS DISTINCT FROM 'espacigo_privacy_cleanup_worker' THEN
        RAISE EXCEPTION 'privacy cleanup worker role is required' USING ERRCODE = '42501';
    END IF;
    IF p_at IS NULL THEN
        RAISE EXCEPTION 'cleanup retry time is required' USING ERRCODE = '22004';
    END IF;
    UPDATE public.media_asset SET lease_until=p_at+interval '1 minute',updated_at=p_at
     WHERE id=p_id AND estado='deleting' AND fencing_token=p_token;
    RETURN FOUND;
END
$$;

REVOKE ALL ON FUNCTION public.claim_private_media_cleanup(integer,timestamptz) FROM PUBLIC;
REVOKE ALL ON FUNCTION public.complete_private_media_cleanup(uuid,bigint,timestamptz) FROM PUBLIC;
REVOKE ALL ON FUNCTION public.fail_private_media_cleanup(uuid,bigint,timestamptz) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION public.claim_private_media_cleanup(integer,timestamptz) TO espacigo_privacy_cleanup_worker;
GRANT EXECUTE ON FUNCTION public.complete_private_media_cleanup(uuid,bigint,timestamptz) TO espacigo_privacy_cleanup_worker;
GRANT EXECUTE ON FUNCTION public.fail_private_media_cleanup(uuid,bigint,timestamptz) TO espacigo_privacy_cleanup_worker;

COMMENT ON TABLE public.media_asset IS
    'IMG-01 private media lifecycle; the privacy cleanup worker can only claim and finalize deleting assets.';

-- Keep the dedicated worker finalizer from completing an account suppression
-- while any of that account's media objects remain in the durable cleanup
-- state machine.
CREATE OR REPLACE FUNCTION public.finalize_pending_suppression_cleanup(p_request_id uuid, p_at timestamptz)
RETURNS boolean
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, public
SET timezone = 'UTC'
AS $$
DECLARE
    v_execution_id uuid;
    v_subject_id uuid;
    v_request_state text;
    v_execution_state text;
    v_completed_at timestamptz;
    v_requested_at timestamptz;
    v_pending integer;
    v_retire_at timestamptz;
BEGIN
    IF pg_catalog.current_setting('role', true) IS DISTINCT FROM 'espacigo_privacy_cleanup_worker' THEN
        RAISE EXCEPTION 'privacy cleanup worker role is required' USING ERRCODE = '42501';
    END IF;

    SELECT x.id, x.usuario_id, x.estado, x.completada_en, s.estado, s.solicitada_en
      INTO v_execution_id, v_subject_id, v_execution_state, v_completed_at, v_request_state, v_requested_at
      FROM public.ejecucion_baja_local x
      JOIN public.solicitud_titular s ON s.id = x.solicitud_id
     WHERE x.solicitud_id = p_request_id
       AND s.tipo = 'supresion'
       AND x.estado IN ('limpieza_pendiente', 'completada')
     ORDER BY x.iniciada_en DESC
     LIMIT 1
     FOR UPDATE OF x, s;
    IF NOT FOUND THEN
        RETURN false;
    END IF;
    IF v_request_state NOT IN ('en_revision', 'resuelta') THEN
        RETURN false;
    END IF;

    SELECT count(*) INTO v_pending
      FROM public.baja_archivo_pendiente_local
     WHERE ejecucion_id = v_execution_id AND completada_en IS NULL;
    SELECT v_pending + count(*) INTO v_pending
      FROM public.media_asset
     WHERE propietario_id=v_subject_id AND estado NOT IN ('deleted','failed');
    IF v_pending > 0 THEN
        RETURN false;
    END IF;

    IF p_at IS NULL THEN
        RAISE EXCEPTION 'cleanup completion time is required' USING ERRCODE = '22004';
    END IF;
    v_retire_at := p_at + interval '60 months';

    IF v_execution_state = 'limpieza_pendiente' THEN
        UPDATE public.solicitud_titular
           SET estado = 'resuelta', resuelta_en = p_at,
               motivo_resolucion_codigo = 'baja_local_minimizada', retirar_en = v_retire_at
         WHERE id = p_request_id AND estado = 'en_revision';
        IF NOT FOUND AND v_request_state <> 'resuelta' THEN
            RETURN false;
        END IF;
        UPDATE public.ejecucion_baja_local
           SET estado = 'completada', completada_en = p_at, retirar_en = v_retire_at
         WHERE id = v_execution_id AND estado = 'limpieza_pendiente';
        v_completed_at := p_at;
    END IF;

    IF v_completed_at IS NULL THEN
        RETURN false;
    END IF;

    INSERT INTO public.registro_replay_baja_local
        (ejecucion_origen_id, solicitud_id, usuario_id, solicitada_en, completada_en)
    SELECT v_execution_id, p_request_id, v_subject_id, v_requested_at, v_completed_at
    ON CONFLICT (ejecucion_origen_id) DO NOTHING;
    IF NOT FOUND AND NOT EXISTS (
        SELECT 1
          FROM public.registro_replay_baja_local r
         WHERE r.ejecucion_origen_id = v_execution_id
           AND r.solicitud_id = p_request_id
           AND r.usuario_id = v_subject_id
           AND r.solicitada_en = v_requested_at
           AND r.completada_en = v_completed_at
    ) THEN
        RAISE EXCEPTION 'privacy replay registry conflicts with completed cleanup' USING ERRCODE = '23514';
    END IF;
    RETURN true;
END
$$;

REVOKE ALL ON FUNCTION public.finalize_pending_suppression_cleanup(uuid, timestamptz) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION public.finalize_pending_suppression_cleanup(uuid, timestamptz)
    TO espacigo_privacy_cleanup_worker;
