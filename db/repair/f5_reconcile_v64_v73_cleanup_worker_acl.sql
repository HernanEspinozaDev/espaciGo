-- Repair ID: F5-R01 (Git-versioned, one-time F5-only repair).
-- F5-only repair for a database-only --no-owner/--no-acl restore.
-- Reconstitutes the F5-approved cleanup-worker ACL contract without replaying
-- either migration and without changing schema_migrations or object ownership.
-- For this target only, the user's latest decision supersedes V64/V73's
-- historical PUBLIC EXECUTE revokes: pin and preserve the complete existing
-- PUBLIC/default ACL baseline, including PUBLIC EXECUTE on the four cleanup
-- routines. Keep the exact V64/V73 worker grants; do not add other ACLs.
BEGIN;
SET LOCAL lock_timeout = '10s';
SET LOCAL statement_timeout = '90s';

DO $preflight$
DECLARE
    role_present boolean;
    migration_count integer;
    migration_fingerprint text;
    database_owner name;
BEGIN
    IF pg_catalog.current_database() <> 'espacigo_f5_gate_20261101'
       OR session_user <> 'postgres' OR current_user <> 'postgres'
       OR NOT (SELECT rolsuper FROM pg_catalog.pg_roles WHERE rolname=current_user) THEN
        RAISE EXCEPTION 'unexpected F5 target or administrative identity; refusing repair';
    END IF;

    SELECT count(*) INTO migration_count FROM public.schema_migrations;
    SELECT pg_catalog.md5(pg_catalog.string_agg(version::text || '|' || name || '|' || checksum, E'\n' ORDER BY version))
      INTO migration_fingerprint FROM public.schema_migrations;
    IF migration_count <> 80
       OR (SELECT min(version) FROM public.schema_migrations) <> 1
       OR (SELECT max(version) FROM public.schema_migrations) <> 80
       OR migration_fingerprint IS DISTINCT FROM 'f24540e905a985bcc58a527032767041'
       OR (SELECT count(*) FROM public.schema_migrations
           WHERE (version=64 AND name='V000064__gcp04_privacy_cleanup_worker_role.sql'
                  AND checksum='9b791ae6950dd9f23d87c31ae59dcbeb1feb9d85631ac21fa95e1e9a6a4763bf')
              OR (version=73 AND name='V000073__img_01e_privacy_media_cleanup.sql'
                  AND checksum='ac2bfc0ba17e97ee40297dc75d8aaaa25f8506e5602a2d281d09415158670ada')) <> 2 THEN
        RAISE EXCEPTION 'F5 ledger is not the reviewed V1-V80 baseline; refusing repair';
    END IF;

    SELECT EXISTS (SELECT 1 FROM pg_catalog.pg_roles
                   WHERE rolname='espacigo_privacy_cleanup_worker') INTO role_present;
    IF role_present THEN
        RAISE EXCEPTION 'cleanup-worker role already exists; this repair is only for the verified missing-role restore drift';
    END IF;

    SELECT pg_catalog.pg_get_userbyid(datdba) INTO database_owner
      FROM pg_catalog.pg_database WHERE datname=pg_catalog.current_database();
    IF database_owner <> 'postgres' THEN
        RAISE EXCEPTION 'unexpected F5 database owner; refusing ACL reconciliation';
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_catalog.pg_namespace
         WHERE nspname='public' AND nspowner='pg_database_owner'::regrole::oid
    ) THEN
        RAISE EXCEPTION 'unexpected public schema owner; refusing ACL reconciliation';
    END IF;

    IF to_regclass('public.ejecucion_baja_local') IS NULL
       OR to_regclass('public.baja_archivo_pendiente_local') IS NULL
       OR to_regclass('public.verificacion_evidencia_sintetica') IS NULL
       OR to_regclass('public.operacion_arriendo_archivo_candidato_local') IS NULL
       OR to_regclass('public.operacion_arriendo_evidencia_ensayo_local') IS NULL
       OR to_regclass('public.foto_perfil_sintetica_local') IS NULL
       OR to_regclass('public.espacio_galeria_sintetica_local') IS NULL
       OR to_regclass('public.media_asset') IS NULL
       OR to_regclass('public.media_variant') IS NULL
       OR to_regclass('public.media_variant_intent') IS NULL THEN
        RAISE EXCEPTION 'one or more V64/V73 ACL targets are absent; refusing repair';
    END IF;

    IF EXISTS (
        SELECT 1 FROM pg_catalog.pg_class c
        WHERE c.oid IN (
            'public.ejecucion_baja_local'::regclass,
            'public.baja_archivo_pendiente_local'::regclass,
            'public.verificacion_evidencia_sintetica'::regclass,
            'public.operacion_arriendo_archivo_candidato_local'::regclass,
            'public.operacion_arriendo_evidencia_ensayo_local'::regclass,
            'public.foto_perfil_sintetica_local'::regclass,
            'public.espacio_galeria_sintetica_local'::regclass,
            'public.media_asset'::regclass,
            'public.media_variant'::regclass,
            'public.media_variant_intent'::regclass)
          AND c.relowner <> 'postgres'::regrole
    ) THEN
        RAISE EXCEPTION 'V64/V73 relation owners differ from the reviewed no-owner restore';
    END IF;

    IF EXISTS (
        SELECT 1 FROM pg_catalog.pg_proc p
        WHERE p.oid IN (
            to_regprocedure('public.finalize_pending_suppression_cleanup(uuid,timestamptz)'),
            to_regprocedure('public.claim_private_media_cleanup(integer,timestamptz)'),
            to_regprocedure('public.complete_private_media_cleanup(uuid,bigint,timestamptz)'),
            to_regprocedure('public.fail_private_media_cleanup(uuid,bigint,timestamptz)'))
          AND (p.proowner <> 'postgres'::regrole
               OR NOT p.prosecdef
               OR NOT ('search_path=pg_catalog, public' = ANY(COALESCE(p.proconfig, ARRAY[]::text[])))
               OR NOT EXISTS (SELECT 1 FROM unnest(COALESCE(p.proconfig, ARRAY[]::text[])) setting
                              WHERE lower(setting)='timezone=utc'))
    ) OR (SELECT count(*) FROM pg_catalog.pg_proc p
          WHERE p.oid IN (
              to_regprocedure('public.finalize_pending_suppression_cleanup(uuid,timestamptz)'),
              to_regprocedure('public.claim_private_media_cleanup(integer,timestamptz)'),
              to_regprocedure('public.complete_private_media_cleanup(uuid,bigint,timestamptz)'),
              to_regprocedure('public.fail_private_media_cleanup(uuid,bigint,timestamptz)'))
         ) <> 4 THEN
        RAISE EXCEPTION 'V64/V73 SECURITY DEFINER routine owner or settings differ from reviewed restore';
    END IF;
END
$preflight$;

-- Snapshot the approved F5 effective PUBLIC ACL set. COALESCE/acldefault
-- materializes built-in defaults for NULL ACL arrays; pg_default_acl is
-- checked separately because it controls future objects. Exclude this session's
-- temporary namespace/objects so the snapshot machinery cannot fingerprint
-- its own PUBLIC defaults. The view/table do not alter persistent ACLs.
CREATE TEMP VIEW f5_current_public_effective_acl AS
WITH effective_public_acl AS (
    SELECT 'pg_catalog.pg_database'::regclass::oid AS classid, d.oid AS objid,
           0::smallint AS objsubid, a.grantor, 0::oid AS grantee,
           a.privilege_type::text AS privilege_type, a.is_grantable
      FROM pg_catalog.pg_database d
      CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(d.datacl,pg_catalog.acldefault('d',d.datdba))) a
     WHERE d.datname=pg_catalog.current_database() AND a.grantee=0
    UNION ALL
    SELECT 'pg_catalog.pg_namespace'::regclass::oid, n.oid, 0::smallint,
           a.grantor, 0::oid, a.privilege_type::text, a.is_grantable
      FROM pg_catalog.pg_namespace n
      CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(n.nspacl,pg_catalog.acldefault('n',n.nspowner))) a
     WHERE a.grantee=0 AND n.oid<>pg_catalog.pg_my_temp_schema()
    UNION ALL
    SELECT 'pg_catalog.pg_class'::regclass::oid, c.oid, 0::smallint,
           a.grantor, 0::oid, a.privilege_type::text, a.is_grantable
      FROM pg_catalog.pg_class c
      CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(c.relacl,pg_catalog.acldefault((CASE WHEN c.relkind='S' THEN 's' ELSE 'r' END)::"char",c.relowner))) a
     WHERE a.grantee=0 AND c.relnamespace<>pg_catalog.pg_my_temp_schema()
    UNION ALL
    SELECT 'pg_catalog.pg_class'::regclass::oid, x.attrelid, x.attnum,
           a.grantor, 0::oid, a.privilege_type::text, a.is_grantable
      FROM pg_catalog.pg_attribute x
      JOIN pg_catalog.pg_class c ON c.oid=x.attrelid
      CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(x.attacl,pg_catalog.acldefault('c',c.relowner))) a
     WHERE x.attnum>0 AND NOT x.attisdropped AND a.grantee=0
       AND c.relnamespace<>pg_catalog.pg_my_temp_schema()
    UNION ALL
    SELECT 'pg_catalog.pg_proc'::regclass::oid, p.oid, 0::smallint,
           a.grantor, 0::oid, a.privilege_type::text, a.is_grantable
      FROM pg_catalog.pg_proc p
      CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(p.proacl,pg_catalog.acldefault('f',p.proowner))) a
     WHERE a.grantee=0 AND p.pronamespace<>pg_catalog.pg_my_temp_schema()
    UNION ALL
    SELECT 'pg_catalog.pg_type'::regclass::oid, t.oid, 0::smallint,
           a.grantor, 0::oid, a.privilege_type::text, a.is_grantable
      FROM pg_catalog.pg_type t
      CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(t.typacl,pg_catalog.acldefault('T',t.typowner))) a
     WHERE a.grantee=0 AND t.typnamespace<>pg_catalog.pg_my_temp_schema()
    UNION ALL
    SELECT 'pg_catalog.pg_language'::regclass::oid, l.oid, 0::smallint,
           a.grantor, 0::oid, a.privilege_type::text, a.is_grantable
      FROM pg_catalog.pg_language l
      CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(l.lanacl,pg_catalog.acldefault('l',l.lanowner))) a
     WHERE a.grantee=0
    UNION ALL
    SELECT 'pg_catalog.pg_foreign_data_wrapper'::regclass::oid, f.oid, 0::smallint,
           a.grantor, 0::oid, a.privilege_type::text, a.is_grantable
      FROM pg_catalog.pg_foreign_data_wrapper f
      CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(f.fdwacl,pg_catalog.acldefault('F',f.fdwowner))) a
     WHERE a.grantee=0
    UNION ALL
    SELECT 'pg_catalog.pg_foreign_server'::regclass::oid, s.oid, 0::smallint,
           a.grantor, 0::oid, a.privilege_type::text, a.is_grantable
      FROM pg_catalog.pg_foreign_server s
      CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(s.srvacl,pg_catalog.acldefault('S',s.srvowner))) a
     WHERE a.grantee=0
    UNION ALL
    SELECT 'pg_catalog.pg_tablespace'::regclass::oid, s.oid, 0::smallint,
           a.grantor, 0::oid, a.privilege_type::text, a.is_grantable
      FROM pg_catalog.pg_tablespace s
      CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(s.spcacl,pg_catalog.acldefault('t',s.spcowner))) a
     WHERE a.grantee=0
    UNION ALL
    SELECT 'pg_catalog.pg_largeobject_metadata'::regclass::oid, l.oid, 0::smallint,
           a.grantor, 0::oid, a.privilege_type::text, a.is_grantable
      FROM pg_catalog.pg_largeobject_metadata l
      CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(l.lomacl,pg_catalog.acldefault('L',l.lomowner))) a
     WHERE a.grantee=0
    UNION ALL
    SELECT 'pg_catalog.pg_parameter_acl'::regclass::oid, p.oid, 0::smallint,
           a.grantor, 0::oid, a.privilege_type::text, a.is_grantable
      FROM pg_catalog.pg_parameter_acl p
      CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(p.paracl,pg_catalog.acldefault('p',pg_catalog.to_regrole('postgres')::oid))) a
     WHERE a.grantee=0
)
SELECT classid, objid, objsubid, grantor, grantee, privilege_type, is_grantable
FROM effective_public_acl;

CREATE TEMP TABLE f5_public_acl_baseline ON COMMIT DROP AS
SELECT * FROM pg_temp.f5_current_public_effective_acl;

DO $public_baseline_precheck$
DECLARE baseline_count bigint; baseline_hash text; default_count bigint; default_hash text;
BEGIN
    SELECT count(*), pg_catalog.md5(COALESCE(pg_catalog.string_agg(classid::text || '|' || objid::text || '|' || objsubid::text || '|' || grantor::text || '|' || privilege_type || '|' || is_grantable::text, chr(10) ORDER BY classid,objid,objsubid,grantor,privilege_type,is_grantable),''))
      INTO baseline_count, baseline_hash FROM pg_temp.f5_public_acl_baseline;
    SELECT count(*), pg_catalog.md5(COALESCE(pg_catalog.string_agg(defaclrole::text || '|' || defaclnamespace::text || '|' || defaclobjtype::text || '|' || COALESCE(defaclacl::text,'NULL'), chr(10) ORDER BY defaclrole,defaclnamespace,defaclobjtype),''))
      INTO default_count, default_hash FROM pg_catalog.pg_default_acl;
    IF current_setting('server_version_num')::integer<>180006
       OR baseline_count<>5762 OR baseline_hash<>'527e02ed8bf413337ae39caf6e4edaa7'
       OR default_count<>0 OR default_hash<>'d41d8cd98f00b204e9800998ecf8427e' THEN
        RAISE EXCEPTION 'F5 effective PUBLIC/default ACL baseline differs from approved fingerprint; refusing repair';
    END IF;
END
$public_baseline_precheck$;

DO $create_role$
BEGIN
    EXECUTE 'CREATE ROLE espacigo_privacy_cleanup_worker NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT';
END
$create_role$;

DO $database_connect$
BEGIN
    EXECUTE format('GRANT CONNECT ON DATABASE %I TO espacigo_privacy_cleanup_worker', pg_catalog.current_database());
END
$database_connect$;
GRANT USAGE ON SCHEMA public TO espacigo_privacy_cleanup_worker;

-- V64's exact table/column scope.
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

-- V73's exact media-cleanup scope and SECURITY DEFINER entry points.
GRANT SELECT (id, propietario_id, estado, fencing_token, lease_until, updated_at)
    ON public.media_asset TO espacigo_privacy_cleanup_worker;
REVOKE INSERT, UPDATE, DELETE, TRUNCATE ON public.media_asset FROM espacigo_privacy_cleanup_worker;
GRANT SELECT (media_id, variant, provider, bucket_name, object_key, generation, mime_type, size_bytes, sha256, width, height)
    ON public.media_variant TO espacigo_privacy_cleanup_worker;
REVOKE INSERT, UPDATE, DELETE, TRUNCATE ON public.media_variant FROM espacigo_privacy_cleanup_worker;
GRANT SELECT (media_id, variant, object_key, generation, write_complete)
    ON public.media_variant_intent TO espacigo_privacy_cleanup_worker;
REVOKE INSERT, UPDATE, DELETE, TRUNCATE ON public.media_variant_intent FROM espacigo_privacy_cleanup_worker;

GRANT EXECUTE ON FUNCTION public.finalize_pending_suppression_cleanup(uuid, timestamptz) TO espacigo_privacy_cleanup_worker;
GRANT EXECUTE ON FUNCTION public.claim_private_media_cleanup(integer, timestamptz) TO espacigo_privacy_cleanup_worker;
GRANT EXECUTE ON FUNCTION public.complete_private_media_cleanup(uuid, bigint, timestamptz) TO espacigo_privacy_cleanup_worker;
GRANT EXECUTE ON FUNCTION public.fail_private_media_cleanup(uuid, bigint, timestamptz) TO espacigo_privacy_cleanup_worker;

DO $postcheck$
DECLARE
    role_safe boolean;
    ledger_intact boolean;
    direct_acl_count integer;
    expected_acl_count integer;
    missing_acl_count integer;
    unexpected_acl_count integer;
    effective_mismatch_count integer;
    worker_role_oid oid;
BEGIN
    SELECT r.rolcanlogin=false AND r.rolsuper=false AND r.rolcreatedb=false
       AND r.rolcreaterole=false AND r.rolreplication=false AND r.rolbypassrls=false
       AND r.rolinherit=false
       AND NOT EXISTS (SELECT 1 FROM pg_catalog.pg_auth_members m
                       WHERE m.roleid=r.oid OR m.member=r.oid)
      INTO role_safe FROM pg_catalog.pg_roles r
     WHERE r.rolname='espacigo_privacy_cleanup_worker';
    IF role_safe IS DISTINCT FROM true THEN
        RAISE EXCEPTION 'cleanup worker role attributes or memberships violate V64';
    END IF;

    SELECT oid INTO worker_role_oid FROM pg_catalog.pg_roles
     WHERE rolname='espacigo_privacy_cleanup_worker';
    IF EXISTS (SELECT 1 FROM pg_catalog.pg_database WHERE datdba=worker_role_oid)
       OR EXISTS (SELECT 1 FROM pg_catalog.pg_namespace WHERE nspowner=worker_role_oid)
       OR EXISTS (SELECT 1 FROM pg_catalog.pg_class WHERE relowner=worker_role_oid)
       OR EXISTS (SELECT 1 FROM pg_catalog.pg_proc WHERE proowner=worker_role_oid)
       OR EXISTS (SELECT 1 FROM pg_catalog.pg_type WHERE typowner=worker_role_oid)
       OR EXISTS (SELECT 1 FROM pg_catalog.pg_language WHERE lanowner=worker_role_oid)
       OR EXISTS (SELECT 1 FROM pg_catalog.pg_foreign_data_wrapper WHERE fdwowner=worker_role_oid)
       OR EXISTS (SELECT 1 FROM pg_catalog.pg_foreign_server WHERE srvowner=worker_role_oid)
       OR EXISTS (SELECT 1 FROM pg_catalog.pg_tablespace WHERE spcowner=worker_role_oid)
       OR EXISTS (SELECT 1 FROM pg_catalog.pg_largeobject_metadata WHERE lomowner=worker_role_oid) THEN
        RAISE EXCEPTION 'cleanup worker owns a database object outside the approved ACL contract';
    END IF;
    -- Compare the complete direct ACL tuple set against V64/V73. This includes
    -- grantor, grantability, object identity, column scope, and every catalog
    -- ACL class; a representative privilege probe or row count cannot hide an
    -- unexpected worker grant elsewhere in this database/cluster.
    -- PostgreSQL records a superuser GRANT as issued by the object owner; the
    -- preflight pins those owners, including pg_database_owner for public.
    WITH expected_acl AS (
        SELECT 'pg_catalog.pg_database'::regclass::oid AS classid,
               d.oid AS objid, 0::smallint AS objsubid,
               'postgres'::regrole::oid AS grantor, worker_role_oid AS grantee,
               'CONNECT'::text AS privilege_type, false AS is_grantable
          FROM pg_catalog.pg_database d
         WHERE d.datname=pg_catalog.current_database()
        UNION ALL
        SELECT 'pg_catalog.pg_namespace'::regclass::oid,
               n.oid, 0::smallint, 'pg_database_owner'::regrole::oid,
               worker_role_oid, 'USAGE'::text, false
          FROM pg_catalog.pg_namespace n
         WHERE n.nspname='public'
        UNION ALL
        SELECT 'pg_catalog.pg_class'::regclass::oid,
               c.oid, 0::smallint, 'postgres'::regrole::oid,
               worker_role_oid, 'DELETE'::text, false
          FROM (VALUES
                ('verificacion_evidencia_sintetica'),
                ('operacion_arriendo_archivo_candidato_local')) AS rel(relname)
          JOIN pg_catalog.pg_class c
            ON c.relnamespace='public'::regnamespace AND c.relname=rel.relname
        UNION ALL
        SELECT 'pg_catalog.pg_class'::regclass::oid,
               c.oid, a.attnum, 'postgres'::regrole::oid,
               worker_role_oid, acl.privilege_type, false
          FROM (VALUES
                ('ejecucion_baja_local', 'id', 'SELECT'),
                ('ejecucion_baja_local', 'solicitud_id', 'SELECT'),
                ('ejecucion_baja_local', 'estado', 'SELECT'),
                ('ejecucion_baja_local', 'iniciada_en', 'SELECT'),
                ('baja_archivo_pendiente_local', 'ejecucion_id', 'SELECT'),
                ('baja_archivo_pendiente_local', 'evidencia_id', 'SELECT'),
                ('baja_archivo_pendiente_local', 'completada_en', 'SELECT'),
                ('baja_archivo_pendiente_local', 'disponible_en', 'SELECT'),
                ('baja_archivo_pendiente_local', 'intentos', 'SELECT'),
                ('baja_archivo_pendiente_local', 'ultimo_codigo_error', 'SELECT'),
                ('baja_archivo_pendiente_local', 'claim_owner', 'SELECT'),
                ('baja_archivo_pendiente_local', 'claim_hasta', 'SELECT'),
                ('baja_archivo_pendiente_local', 'claim_generacion', 'SELECT'),
                ('baja_archivo_pendiente_local', 'completada_en', 'UPDATE'),
                ('baja_archivo_pendiente_local', 'disponible_en', 'UPDATE'),
                ('baja_archivo_pendiente_local', 'intentos', 'UPDATE'),
                ('baja_archivo_pendiente_local', 'ultimo_codigo_error', 'UPDATE'),
                ('baja_archivo_pendiente_local', 'claim_owner', 'UPDATE'),
                ('baja_archivo_pendiente_local', 'claim_hasta', 'UPDATE'),
                ('baja_archivo_pendiente_local', 'claim_generacion', 'UPDATE'),
                ('verificacion_evidencia_sintetica', 'id', 'SELECT'),
                ('operacion_arriendo_archivo_candidato_local', 'archivo_id', 'SELECT'),
                ('operacion_arriendo_evidencia_ensayo_local', 'id', 'SELECT'),
                ('operacion_arriendo_evidencia_ensayo_local', 'retirar_archivo_en', 'SELECT'),
                ('operacion_arriendo_evidencia_ensayo_local', 'archivo_retirado_en', 'SELECT'),
                ('operacion_arriendo_evidencia_ensayo_local', 'retirar_archivo_en', 'UPDATE'),
                ('operacion_arriendo_evidencia_ensayo_local', 'archivo_retirado_en', 'UPDATE'),
                ('foto_perfil_sintetica_local', 'id', 'SELECT'),
                ('foto_perfil_sintetica_local', 'estado', 'SELECT'),
                ('foto_perfil_sintetica_local', 'archivo_id', 'UPDATE'),
                ('foto_perfil_sintetica_local', 'mime_type', 'UPDATE'),
                ('foto_perfil_sintetica_local', 'sha256', 'UPDATE'),
                ('foto_perfil_sintetica_local', 'size_bytes', 'UPDATE'),
                ('foto_perfil_sintetica_local', 'limpia_en', 'UPDATE'),
                ('foto_perfil_sintetica_local', 'proximo_intento_en', 'UPDATE'),
                ('foto_perfil_sintetica_local', 'ultimo_codigo_error', 'UPDATE'),
                ('espacio_galeria_sintetica_local', 'id', 'SELECT'),
                ('espacio_galeria_sintetica_local', 'estado', 'SELECT'),
                ('espacio_galeria_sintetica_local', 'archivo_id', 'UPDATE'),
                ('espacio_galeria_sintetica_local', 'limpia_en', 'UPDATE'),
                ('espacio_galeria_sintetica_local', 'proximo_intento_en', 'UPDATE'),
                ('espacio_galeria_sintetica_local', 'ultimo_codigo_error', 'UPDATE'),
                ('media_asset', 'id', 'SELECT'),
                ('media_asset', 'propietario_id', 'SELECT'),
                ('media_asset', 'estado', 'SELECT'),
                ('media_asset', 'fencing_token', 'SELECT'),
                ('media_asset', 'lease_until', 'SELECT'),
                ('media_asset', 'updated_at', 'SELECT'),
                ('media_variant', 'media_id', 'SELECT'),
                ('media_variant', 'variant', 'SELECT'),
                ('media_variant', 'provider', 'SELECT'),
                ('media_variant', 'bucket_name', 'SELECT'),
                ('media_variant', 'object_key', 'SELECT'),
                ('media_variant', 'generation', 'SELECT'),
                ('media_variant', 'mime_type', 'SELECT'),
                ('media_variant', 'size_bytes', 'SELECT'),
                ('media_variant', 'sha256', 'SELECT'),
                ('media_variant', 'width', 'SELECT'),
                ('media_variant', 'height', 'SELECT'),
                ('media_variant_intent', 'media_id', 'SELECT'),
                ('media_variant_intent', 'variant', 'SELECT'),
                ('media_variant_intent', 'object_key', 'SELECT'),
                ('media_variant_intent', 'generation', 'SELECT'),
                ('media_variant_intent', 'write_complete', 'SELECT')
          ) AS acl(relname, attname, privilege_type)
          JOIN pg_catalog.pg_class c
            ON c.relnamespace='public'::regnamespace AND c.relname=acl.relname
          JOIN pg_catalog.pg_attribute a
            ON a.attrelid=c.oid AND a.attname=acl.attname
           AND a.attnum > 0 AND NOT a.attisdropped
        UNION ALL
        SELECT 'pg_catalog.pg_proc'::regclass::oid,
               p.oid, 0::smallint, 'postgres'::regrole::oid,
               worker_role_oid, 'EXECUTE'::text, false
          FROM unnest(ARRAY[
                'public.finalize_pending_suppression_cleanup(uuid,timestamptz)'::regprocedure::oid,
                'public.claim_private_media_cleanup(integer,timestamptz)'::regprocedure::oid,
                'public.complete_private_media_cleanup(uuid,bigint,timestamptz)'::regprocedure::oid,
                'public.fail_private_media_cleanup(uuid,bigint,timestamptz)'::regprocedure::oid
          ]) AS expected_function(oid)
          JOIN pg_catalog.pg_proc p ON p.oid=expected_function.oid
    ), actual_acl AS (
        SELECT 'pg_catalog.pg_database'::regclass::oid AS classid,
               d.oid AS objid, 0::smallint AS objsubid,
               a.grantor, a.grantee, a.privilege_type::text, a.is_grantable
          FROM pg_catalog.pg_database d
          CROSS JOIN LATERAL pg_catalog.aclexplode(d.datacl) a
         WHERE a.grantee=worker_role_oid OR a.grantor=worker_role_oid
        UNION ALL
        SELECT 'pg_catalog.pg_namespace'::regclass::oid,
               n.oid, 0::smallint, a.grantor, a.grantee,
               a.privilege_type::text, a.is_grantable
          FROM pg_catalog.pg_namespace n
          CROSS JOIN LATERAL pg_catalog.aclexplode(n.nspacl) a
         WHERE a.grantee=worker_role_oid OR a.grantor=worker_role_oid
        UNION ALL
        SELECT 'pg_catalog.pg_class'::regclass::oid,
               c.oid, 0::smallint, a.grantor, a.grantee,
               a.privilege_type::text, a.is_grantable
          FROM pg_catalog.pg_class c
          CROSS JOIN LATERAL pg_catalog.aclexplode(c.relacl) a
         WHERE a.grantee=worker_role_oid OR a.grantor=worker_role_oid
        UNION ALL
        SELECT 'pg_catalog.pg_class'::regclass::oid,
               a.attrelid, a.attnum, x.grantor, x.grantee,
               x.privilege_type::text, x.is_grantable
          FROM pg_catalog.pg_attribute a
          CROSS JOIN LATERAL pg_catalog.aclexplode(a.attacl) x
         WHERE a.attnum > 0 AND NOT a.attisdropped
           AND (x.grantee=worker_role_oid OR x.grantor=worker_role_oid)
        UNION ALL
        SELECT 'pg_catalog.pg_proc'::regclass::oid,
               p.oid, 0::smallint, a.grantor, a.grantee,
               a.privilege_type::text, a.is_grantable
          FROM pg_catalog.pg_proc p
          CROSS JOIN LATERAL pg_catalog.aclexplode(p.proacl) a
         WHERE a.grantee=worker_role_oid OR a.grantor=worker_role_oid
        UNION ALL
        SELECT 'pg_catalog.pg_type'::regclass::oid,
               t.oid, 0::smallint, a.grantor, a.grantee,
               a.privilege_type::text, a.is_grantable
          FROM pg_catalog.pg_type t
          CROSS JOIN LATERAL pg_catalog.aclexplode(t.typacl) a
         WHERE a.grantee=worker_role_oid OR a.grantor=worker_role_oid
        UNION ALL
        SELECT 'pg_catalog.pg_language'::regclass::oid,
               l.oid, 0::smallint, a.grantor, a.grantee,
               a.privilege_type::text, a.is_grantable
          FROM pg_catalog.pg_language l
          CROSS JOIN LATERAL pg_catalog.aclexplode(l.lanacl) a
         WHERE a.grantee=worker_role_oid OR a.grantor=worker_role_oid
        UNION ALL
        SELECT 'pg_catalog.pg_foreign_data_wrapper'::regclass::oid,
               f.oid, 0::smallint, a.grantor, a.grantee,
               a.privilege_type::text, a.is_grantable
          FROM pg_catalog.pg_foreign_data_wrapper f
          CROSS JOIN LATERAL pg_catalog.aclexplode(f.fdwacl) a
         WHERE a.grantee=worker_role_oid OR a.grantor=worker_role_oid
        UNION ALL
        SELECT 'pg_catalog.pg_foreign_server'::regclass::oid,
               s.oid, 0::smallint, a.grantor, a.grantee,
               a.privilege_type::text, a.is_grantable
          FROM pg_catalog.pg_foreign_server s
          CROSS JOIN LATERAL pg_catalog.aclexplode(s.srvacl) a
         WHERE a.grantee=worker_role_oid OR a.grantor=worker_role_oid
        UNION ALL
        SELECT 'pg_catalog.pg_tablespace'::regclass::oid,
               s.oid, 0::smallint, a.grantor, a.grantee,
               a.privilege_type::text, a.is_grantable
          FROM pg_catalog.pg_tablespace s
          CROSS JOIN LATERAL pg_catalog.aclexplode(s.spcacl) a
         WHERE a.grantee=worker_role_oid OR a.grantor=worker_role_oid
        UNION ALL
        SELECT 'pg_catalog.pg_largeobject_metadata'::regclass::oid,
               l.oid, 0::smallint, a.grantor, a.grantee,
               a.privilege_type::text, a.is_grantable
          FROM pg_catalog.pg_largeobject_metadata l
          CROSS JOIN LATERAL pg_catalog.aclexplode(l.lomacl) a
         WHERE a.grantee=worker_role_oid OR a.grantor=worker_role_oid
        UNION ALL
        SELECT 'pg_catalog.pg_parameter_acl'::regclass::oid,
               p.oid, 0::smallint, a.grantor, a.grantee,
               a.privilege_type::text, a.is_grantable
          FROM pg_catalog.pg_parameter_acl p
          CROSS JOIN LATERAL pg_catalog.aclexplode(p.paracl) a
         WHERE a.grantee=worker_role_oid OR a.grantor=worker_role_oid
        UNION ALL
        SELECT 'pg_catalog.pg_default_acl'::regclass::oid,
               d.oid, 0::smallint, a.grantor, a.grantee,
               a.privilege_type::text, a.is_grantable
          FROM pg_catalog.pg_default_acl d
          CROSS JOIN LATERAL pg_catalog.aclexplode(d.defaclacl) a
         WHERE a.grantee=worker_role_oid OR a.grantor=worker_role_oid
    ), missing_acl AS (
        SELECT * FROM expected_acl EXCEPT SELECT * FROM actual_acl
    ), unexpected_acl AS (
        SELECT * FROM actual_acl EXCEPT SELECT * FROM expected_acl
    ),
    -- Evaluate effective rights for the worker, not just its direct ACL.
    -- Expected column access comes from the direct source-derived set above;
    -- checking every column catches PUBLIC/other-principal grants on any
    -- application relation while allowing the intended column grants. This
    -- checks the pinned F5 PUBLIC baseline and exact direct ACLs; full PUBLIC fingerprint,
    -- no membership/ownership, and unchanged defaults cover remaining ACL classes.
    effective_checks AS (
        SELECT 'column ' || n.nspname || '.' || c.relname || '.' || a.attname || ' ' || p.privilege AS scope,
               EXISTS (SELECT 1 FROM f5_public_acl_baseline b
                        WHERE b.classid='pg_catalog.pg_class'::regclass AND b.objid=c.oid
                          AND b.objsubid IN (0,a.attnum) AND b.privilege_type=p.privilege)
               OR EXISTS (SELECT 1 FROM expected_acl e
                        WHERE e.classid='pg_catalog.pg_class'::regclass
                          AND e.objid=c.oid AND e.privilege_type=p.privilege
                          AND e.objsubid IN (0,a.attnum)) AS expected,
               pg_catalog.has_column_privilege(worker_role_oid,c.oid,a.attname,p.privilege) AS actual
          FROM pg_catalog.pg_class c
          JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
          JOIN pg_catalog.pg_attribute a ON a.attrelid=c.oid AND a.attnum>0 AND NOT a.attisdropped
          CROSS JOIN (VALUES ('SELECT'),('INSERT'),('UPDATE'),('REFERENCES')) p(privilege)
         WHERE n.nspname <> 'information_schema' AND n.nspname !~ '^pg_'
           AND c.relkind IN ('r','p','v','m','f')
        UNION ALL
        SELECT 'relation ' || n.nspname || '.' || c.relname || ' ' || p.privilege,
               EXISTS (SELECT 1 FROM expected_acl e
                        WHERE e.classid='pg_catalog.pg_class'::regclass AND e.objid=c.oid
                          AND e.objsubid=0 AND e.privilege_type=p.privilege)
               OR EXISTS (SELECT 1 FROM pg_temp.f5_public_acl_baseline b
                          WHERE b.classid='pg_catalog.pg_class'::regclass AND b.objid=c.oid
                            AND b.objsubid=0 AND b.privilege_type=p.privilege),
               pg_catalog.has_table_privilege(worker_role_oid,c.oid,p.privilege)
          FROM pg_catalog.pg_class c
          JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
          CROSS JOIN (VALUES ('DELETE'),('TRUNCATE'),('TRIGGER'),('MAINTAIN')) p(privilege)
         WHERE n.nspname <> 'information_schema' AND n.nspname !~ '^pg_'
           AND c.relkind IN ('r','p','v','m','f')
        UNION ALL
        SELECT 'sequence ' || n.nspname || '.' || c.relname || ' ' || p.privilege,
               EXISTS (SELECT 1 FROM pg_temp.f5_public_acl_baseline b
                        WHERE b.classid='pg_catalog.pg_class'::regclass AND b.objid=c.oid
                          AND b.objsubid=0 AND b.privilege_type=p.privilege)
               OR EXISTS (SELECT 1 FROM expected_acl e
                          WHERE e.classid='pg_catalog.pg_class'::regclass AND e.objid=c.oid
                            AND e.objsubid=0 AND e.privilege_type=p.privilege),
               pg_catalog.has_sequence_privilege(worker_role_oid,c.oid,p.privilege)
          FROM pg_catalog.pg_class c
          JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
          CROSS JOIN (VALUES ('USAGE'),('SELECT'),('UPDATE')) p(privilege)
         WHERE n.nspname <> 'information_schema' AND n.nspname !~ '^pg_'
           AND c.relkind='S'
        UNION ALL
        SELECT 'function ' || p.oid::text || ' EXECUTE',
               EXISTS (SELECT 1 FROM pg_temp.f5_public_acl_baseline b
                        WHERE b.classid='pg_catalog.pg_proc'::regclass AND b.objid=p.oid
                          AND b.privilege_type='EXECUTE')
               OR EXISTS (SELECT 1 FROM expected_acl e
                           WHERE e.classid='pg_catalog.pg_proc'::regclass AND e.objid=p.oid
                             AND e.privilege_type='EXECUTE'),
               pg_catalog.has_function_privilege(worker_role_oid,p.oid,'EXECUTE')
          FROM pg_catalog.pg_proc p
        UNION ALL
        SELECT 'schema ' || n.nspname || ' ' || p.privilege,
               EXISTS (SELECT 1 FROM pg_temp.f5_public_acl_baseline b
                          WHERE b.classid='pg_catalog.pg_namespace'::regclass AND b.objid=n.oid
                            AND b.privilege_type=p.privilege)
               OR EXISTS (SELECT 1 FROM expected_acl e
                          WHERE e.classid='pg_catalog.pg_namespace'::regclass AND e.objid=n.oid
                            AND e.privilege_type=p.privilege),
               pg_catalog.has_schema_privilege(worker_role_oid,n.oid,p.privilege)
          FROM pg_catalog.pg_namespace n
          CROSS JOIN (VALUES ('USAGE'),('CREATE')) p(privilege)
         WHERE n.nspname <> 'information_schema' AND n.nspname !~ '^pg_'
        UNION ALL
        SELECT 'database ' || d.datname || ' ' || p.privilege,
               EXISTS (SELECT 1 FROM pg_temp.f5_public_acl_baseline b
                          WHERE b.classid='pg_catalog.pg_database'::regclass AND b.objid=d.oid
                            AND b.privilege_type=p.privilege)
               OR EXISTS (SELECT 1 FROM expected_acl e
                          WHERE e.classid='pg_catalog.pg_database'::regclass AND e.objid=d.oid
                            AND e.privilege_type=p.privilege),
               pg_catalog.has_database_privilege(worker_role_oid,d.oid,p.privilege)
          FROM pg_catalog.pg_database d
          CROSS JOIN (VALUES ('CONNECT'),('CREATE'),('TEMPORARY')) p(privilege)
         WHERE d.datname=pg_catalog.current_database()
    )
    SELECT (SELECT count(*) FROM expected_acl),
           (SELECT count(*) FROM actual_acl),
           (SELECT count(*) FROM missing_acl),
           (SELECT count(*) FROM unexpected_acl),
           (SELECT count(*) FROM effective_checks WHERE expected IS DISTINCT FROM actual)
      INTO expected_acl_count, direct_acl_count, missing_acl_count, unexpected_acl_count,
           effective_mismatch_count;
    IF expected_acl_count <> 72 OR direct_acl_count <> expected_acl_count
       OR missing_acl_count <> 0 OR unexpected_acl_count <> 0 THEN
        RAISE EXCEPTION 'cleanup-worker direct ACL tuples differ from exact V64/V73 contract (expected %, actual %, missing %, unexpected %)',
            expected_acl_count, direct_acl_count, missing_acl_count, unexpected_acl_count;
    END IF;
    IF effective_mismatch_count <> 0 THEN
        RAISE EXCEPTION 'cleanup-worker effective ACL matrix differs from V64/V73 contract (% privilege mismatches across relations, columns, sequences, schemas, and database)',
            effective_mismatch_count;
    END IF;
    IF EXISTS (
        SELECT 1
          FROM pg_catalog.pg_init_privs ip
          CROSS JOIN LATERAL pg_catalog.aclexplode(ip.initprivs) a
         WHERE a.grantee=worker_role_oid OR a.grantor=worker_role_oid
    ) OR EXISTS (
        SELECT 1 FROM pg_catalog.pg_default_acl d
         WHERE d.defaclrole=worker_role_oid
    ) THEN
        RAISE EXCEPTION 'cleanup-worker occurs in extension initial privileges or owns default ACL state outside V64/V73';
    END IF;

    IF (SELECT count(*) FROM pg_temp.f5_current_public_effective_acl) <> 5762
       OR (SELECT pg_catalog.md5(COALESCE(pg_catalog.string_agg(classid::text || '|' || objid::text || '|' || objsubid::text || '|' || grantor::text || '|' || privilege_type || '|' || is_grantable::text, chr(10) ORDER BY classid,objid,objsubid,grantor,privilege_type,is_grantable),''))
             FROM pg_temp.f5_current_public_effective_acl) <> '527e02ed8bf413337ae39caf6e4edaa7'
       OR EXISTS (SELECT * FROM pg_temp.f5_public_acl_baseline EXCEPT SELECT * FROM pg_temp.f5_current_public_effective_acl)
       OR EXISTS (SELECT * FROM pg_temp.f5_current_public_effective_acl EXCEPT SELECT * FROM pg_temp.f5_public_acl_baseline)
       OR (SELECT count(*) FROM pg_catalog.pg_default_acl) <> 0
       OR (SELECT pg_catalog.md5(COALESCE(pg_catalog.string_agg(defaclrole::text || '|' || defaclnamespace::text || '|' || defaclobjtype::text || '|' || COALESCE(defaclacl::text,'NULL'), chr(10) ORDER BY defaclrole,defaclnamespace,defaclobjtype),'')) FROM pg_catalog.pg_default_acl) <> 'd41d8cd98f00b204e9800998ecf8427e' THEN
        RAISE EXCEPTION 'F5 PUBLIC/default ACL baseline changed during repair';
    END IF;

    SELECT count(*)=80 AND min(version)=1 AND max(version)=80
       AND pg_catalog.md5(pg_catalog.string_agg(version::text || '|' || name || '|' || checksum, E'\n' ORDER BY version))='f24540e905a985bcc58a527032767041'
       AND count(*) FILTER (WHERE version=64 AND name='V000064__gcp04_privacy_cleanup_worker_role.sql'
                             AND checksum='9b791ae6950dd9f23d87c31ae59dcbeb1feb9d85631ac21fa95e1e9a6a4763bf')=1
       AND count(*) FILTER (WHERE version=73 AND name='V000073__img_01e_privacy_media_cleanup.sql'
                             AND checksum='ac2bfc0ba17e97ee40297dc75d8aaaa25f8506e5602a2d281d09415158670ada')=1
      INTO ledger_intact FROM public.schema_migrations;
    IF ledger_intact IS DISTINCT FROM true THEN
        RAISE EXCEPTION 'F5 migration ledger changed during ACL repair';
    END IF;
END
$postcheck$;

DROP VIEW pg_temp.f5_current_public_effective_acl;
COMMIT;
