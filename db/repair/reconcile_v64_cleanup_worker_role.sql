-- PROPOSAL ONLY. Do not execute without a separately approved DBA change.
-- Repairs only the missing worker role/grants when V64 is in the ledger and
-- the SECURITY DEFINER routine still exists with its reviewed owner/settings.
-- Does not modify schema_migrations, replace the routine, change object owners,
-- or grant privileges to runtime/migrator logins.
BEGIN;

DO $preflight$
DECLARE
    role_present boolean;
    migration_checksum text;
    routine_owner text;
    routine_is_security_definer boolean;
    routine_config text;
BEGIN
    IF current_database() <> 'espacigo_dms_stage_20261020' THEN
        RAISE EXCEPTION 'unexpected database; refusing V64 role reconciliation';
    END IF;
    IF session_user <> 'postgres' THEN
        RAISE EXCEPTION 'reconciliation requires the reviewed postgres session';
    END IF;
    IF current_user <> 'postgres' THEN
        RAISE EXCEPTION 'reconciliation requires current_user=postgres';
    END IF;

    IF (SELECT count(*) FROM public.schema_migrations
         WHERE version = 64 AND name = 'V000064__gcp04_privacy_cleanup_worker_role.sql') <> 1 THEN
        RAISE EXCEPTION 'expected exactly one V64 ledger row; stop for manual review';
    END IF;
    SELECT checksum INTO migration_checksum
      FROM public.schema_migrations
     WHERE version = 64 AND name = 'V000064__gcp04_privacy_cleanup_worker_role.sql';
    IF migration_checksum IS DISTINCT FROM '9b791ae6950dd9f23d87c31ae59dcbeb1feb9d85631ac21fa95e1e9a6a4763bf' THEN
        RAISE EXCEPTION 'V64 ledger row absent or checksum differs; stop for manual review';
    END IF;

    SELECT EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname = 'espacigo_privacy_cleanup_worker')
      INTO role_present;
    IF role_present THEN
        RAISE EXCEPTION 'cleanup worker role exists; this repair targets only the reported missing-role drift';
    END IF;

    IF to_regclass('public.ejecucion_baja_local') IS NULL
       OR to_regclass('public.baja_archivo_pendiente_local') IS NULL
       OR to_regclass('public.verificacion_evidencia_sintetica') IS NULL
       OR to_regclass('public.operacion_arriendo_archivo_candidato_local') IS NULL
       OR to_regclass('public.operacion_arriendo_evidencia_ensayo_local') IS NULL
       OR to_regclass('public.foto_perfil_sintetica_local') IS NULL
       OR to_regclass('public.espacio_galeria_sintetica_local') IS NULL THEN
        RAISE EXCEPTION 'one or more V64 grant targets are missing; stop for drift review';
    END IF;

    IF to_regprocedure('public.finalize_pending_suppression_cleanup(uuid,timestamp with time zone)') IS NOT NULL THEN
        SELECT pg_catalog.pg_get_userbyid(p.proowner), p.prosecdef,
               COALESCE(array_to_string(p.proconfig, ','), '')
          INTO routine_owner, routine_is_security_definer, routine_config
          FROM pg_catalog.pg_proc p
         WHERE p.oid = to_regprocedure('public.finalize_pending_suppression_cleanup(uuid,timestamp with time zone)');
        IF routine_owner IS DISTINCT FROM 'postgres'
           OR routine_is_security_definer IS DISTINCT FROM true
           OR routine_config !~* 'search_path=pg_catalog, public'
           OR routine_config !~* 'timezone=UTC' THEN
            RAISE EXCEPTION 'existing V64 cleanup routine owner/security settings differ; stop for reviewed repair';
        END IF;
    END IF;
END
$preflight$;

CREATE ROLE espacigo_privacy_cleanup_worker
    NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT;

DO $database_connect$
BEGIN
    EXECUTE format('GRANT CONNECT ON DATABASE %I TO espacigo_privacy_cleanup_worker', current_database());
END
$database_connect$;
GRANT USAGE ON SCHEMA public TO espacigo_privacy_cleanup_worker;

GRANT SELECT (id, solicitud_id, estado, iniciada_en)
    ON public.ejecucion_baja_local TO espacigo_privacy_cleanup_worker;
GRANT SELECT (ejecucion_id, evidencia_id, completada_en, disponible_en, intentos,
              ultimo_codigo_error, claim_owner, claim_hasta, claim_generacion)
    ON public.baja_archivo_pendiente_local TO espacigo_privacy_cleanup_worker;
GRANT UPDATE (completada_en, disponible_en, intentos, ultimo_codigo_error,
              claim_owner, claim_hasta, claim_generacion)
    ON public.baja_archivo_pendiente_local TO espacigo_privacy_cleanup_worker;
GRANT SELECT (id) ON public.verificacion_evidencia_sintetica TO espacigo_privacy_cleanup_worker;
GRANT DELETE ON public.verificacion_evidencia_sintetica TO espacigo_privacy_cleanup_worker;
GRANT SELECT (archivo_id) ON public.operacion_arriendo_archivo_candidato_local TO espacigo_privacy_cleanup_worker;
GRANT DELETE ON public.operacion_arriendo_archivo_candidato_local TO espacigo_privacy_cleanup_worker;
GRANT SELECT (id, retirar_archivo_en, archivo_retirado_en)
    ON public.operacion_arriendo_evidencia_ensayo_local TO espacigo_privacy_cleanup_worker;
GRANT UPDATE (retirar_archivo_en, archivo_retirado_en)
    ON public.operacion_arriendo_evidencia_ensayo_local TO espacigo_privacy_cleanup_worker;
GRANT SELECT (id, estado) ON public.foto_perfil_sintetica_local TO espacigo_privacy_cleanup_worker;
GRANT UPDATE (archivo_id, mime_type, sha256, size_bytes, limpia_en, proximo_intento_en, ultimo_codigo_error)
    ON public.foto_perfil_sintetica_local TO espacigo_privacy_cleanup_worker;
GRANT SELECT (id, estado) ON public.espacio_galeria_sintetica_local TO espacigo_privacy_cleanup_worker;
GRANT UPDATE (archivo_id, limpia_en, proximo_intento_en, ultimo_codigo_error)
    ON public.espacio_galeria_sintetica_local TO espacigo_privacy_cleanup_worker;

DO $routine_grant$
BEGIN
    -- V73 uses CREATE OR REPLACE to restore this routine if it is absent.
    IF to_regprocedure('public.finalize_pending_suppression_cleanup(uuid,timestamp with time zone)') IS NOT NULL THEN
        EXECUTE 'REVOKE ALL ON FUNCTION public.finalize_pending_suppression_cleanup(uuid,timestamp with time zone) FROM PUBLIC';
        EXECUTE 'GRANT EXECUTE ON FUNCTION public.finalize_pending_suppression_cleanup(uuid,timestamp with time zone) TO espacigo_privacy_cleanup_worker';
    END IF;
END
$routine_grant$;

COMMIT;
