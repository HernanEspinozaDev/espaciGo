-- GCP-04-W2: separate the privacy cleanup worker from the broad API runtime.
-- V1-V63 remain immutable. The database login receives SET membership only
-- during a reviewed runtime bootstrap; this role itself cannot log in.
DO $$
DECLARE
    role_exists boolean;
    role_can_login boolean;
    role_superuser boolean;
    role_createdb boolean;
    role_createrole boolean;
    role_replication boolean;
    role_bypassrls boolean;
    role_inherit boolean;
    has_memberships boolean;
BEGIN
    SELECT EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname = 'espacigo_privacy_cleanup_worker')
      INTO role_exists;
    IF NOT role_exists THEN
        CREATE ROLE espacigo_privacy_cleanup_worker
            NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT;
    ELSE
        SELECT rolcanlogin, rolsuper, rolcreatedb, rolcreaterole, rolreplication, rolbypassrls, rolinherit
          INTO role_can_login, role_superuser, role_createdb, role_createrole, role_replication, role_bypassrls, role_inherit
          FROM pg_catalog.pg_roles WHERE rolname = 'espacigo_privacy_cleanup_worker';
        IF role_can_login OR role_superuser OR role_createdb OR role_createrole OR role_replication OR role_bypassrls OR role_inherit THEN
            RAISE EXCEPTION 'espacigo_privacy_cleanup_worker has unsafe role attributes';
        END IF;
    END IF;
    SELECT EXISTS (
        SELECT 1 FROM pg_catalog.pg_auth_members m
        JOIN pg_catalog.pg_roles r ON r.oid = m.roleid
        JOIN pg_catalog.pg_roles member ON member.oid = m.member
        WHERE r.rolname = 'espacigo_privacy_cleanup_worker'
           OR member.rolname = 'espacigo_privacy_cleanup_worker'
    ) INTO has_memberships;
    IF has_memberships THEN
        RAISE EXCEPTION 'espacigo_privacy_cleanup_worker must not have role memberships';
    END IF;
END
$$;

DO $$
BEGIN
    EXECUTE format('GRANT CONNECT ON DATABASE %I TO espacigo_privacy_cleanup_worker', current_database());
END
$$;
GRANT USAGE ON SCHEMA public TO espacigo_privacy_cleanup_worker;

-- The worker can inspect and claim only queue scheduling/fencing columns.
GRANT SELECT (id, solicitud_id, estado, iniciada_en)
    ON public.ejecucion_baja_local TO espacigo_privacy_cleanup_worker;
GRANT SELECT (ejecucion_id, evidencia_id, completada_en, disponible_en, intentos,
              ultimo_codigo_error, claim_owner, claim_hasta, claim_generacion)
    ON public.baja_archivo_pendiente_local TO espacigo_privacy_cleanup_worker;
GRANT UPDATE (completada_en, disponible_en, intentos, ultimo_codigo_error,
              claim_owner, claim_hasta, claim_generacion)
    ON public.baja_archivo_pendiente_local TO espacigo_privacy_cleanup_worker;

-- These are the only evidence references the external cleaner must reconcile.
GRANT SELECT (id) ON public.verificacion_evidencia_sintetica TO espacigo_privacy_cleanup_worker;
GRANT DELETE ON public.verificacion_evidencia_sintetica TO espacigo_privacy_cleanup_worker;
GRANT SELECT (archivo_id) ON public.operacion_arriendo_archivo_candidato_local TO espacigo_privacy_cleanup_worker;
GRANT DELETE ON public.operacion_arriendo_archivo_candidato_local TO espacigo_privacy_cleanup_worker;
GRANT SELECT (id) ON public.operacion_arriendo_evidencia_ensayo_local TO espacigo_privacy_cleanup_worker;
GRANT UPDATE (retirar_archivo_en, archivo_retirado_en)
    ON public.operacion_arriendo_evidencia_ensayo_local TO espacigo_privacy_cleanup_worker;
GRANT SELECT (id, estado) ON public.foto_perfil_sintetica_local TO espacigo_privacy_cleanup_worker;
GRANT UPDATE (archivo_id, mime_type, sha256, size_bytes, limpia_en, proximo_intento_en, ultimo_codigo_error)
    ON public.foto_perfil_sintetica_local TO espacigo_privacy_cleanup_worker;
GRANT SELECT (id, estado) ON public.espacio_galeria_sintetica_local TO espacigo_privacy_cleanup_worker;
GRANT UPDATE (archivo_id, limpia_en, proximo_intento_en, ultimo_codigo_error)
    ON public.espacio_galeria_sintetica_local TO espacigo_privacy_cleanup_worker;

-- Finalization performs privacy-sensitive row transitions and ledger writes
-- under the migration owner, while exposing no account, key, or detail fields.
CREATE FUNCTION public.finalize_pending_suppression_cleanup(p_request_id uuid, p_at timestamptz)
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
