-- Close only the reviewed V95 temporary public CREATE grant. This cleanup is
-- intentionally independent of whether V95 reached its exact ledger boundary.
-- Run with psql -X -v ON_ERROR_STOP=1; the post-COMMIT checks cannot roll back REVOKE.
BEGIN;
SELECT pg_catalog.pg_advisory_xact_lock(x'45535047'::int,x'4d494752'::int);
SET LOCAL search_path TO public;

DO $guard$
DECLARE
    v_schema_owner oid;
    v_database_owner oid;
BEGIN
    IF current_database()<>'espacigo_dms_stage_20261020'
       OR (SELECT oid FROM pg_catalog.pg_database WHERE datname=current_database())<>16388
       OR (SELECT datdba FROM pg_catalog.pg_database WHERE datname=current_database())<>'postgres'::regrole
       OR NOT (SELECT datallowconn FROM pg_catalog.pg_database WHERE datname=current_database())
       OR (SELECT datistemplate FROM pg_catalog.pg_database WHERE datname=current_database())
       OR current_setting('server_version_num')<>'180007'
       OR current_setting('port')<>'5432'
       OR session_user<>'postgres' OR current_user<>'postgres'
       OR inet_client_addr() IS NOT NULL OR inet_server_addr() IS NOT NULL THEN
        RAISE EXCEPTION 'native V95 finalizer target/peer administrator mismatch';
    END IF;
    SELECT oid INTO v_schema_owner FROM pg_catalog.pg_roles WHERE rolname='espacigo_schema_owner';
    SELECT oid INTO v_database_owner FROM pg_catalog.pg_roles WHERE rolname='pg_database_owner';
    IF v_schema_owner IS NULL OR v_database_owner IS NULL THEN
        RAISE EXCEPTION 'native V95 finalizer requires the existing schema/database-owner roles';
    END IF;
    IF EXISTS (
      SELECT 1 FROM pg_catalog.pg_namespace n
      CROSS JOIN LATERAL pg_catalog.aclexplode(n.nspacl) a
      WHERE n.nspname='public' AND a.grantee=v_schema_owner
        AND a.grantor=v_database_owner AND a.privilege_type='CREATE'
        AND a.is_grantable
    ) THEN
        RAISE EXCEPTION 'native V95 finalizer refuses to alter a pre-existing CREATE grant option';
    END IF;
END
$guard$;

SET LOCAL ROLE pg_database_owner;
REVOKE CREATE ON SCHEMA public FROM espacigo_schema_owner;
RESET ROLE;
COMMIT;

DO $verify$
DECLARE
    v_actual jsonb;
    v_expected jsonb;
    v_delta bigint;
    v_count bigint;
    v_min bigint;
    v_max bigint;
    v_v94_name text;
    v_v94_checksum text;
    v_v95_name text;
    v_v95_checksum text;
    v_objects integer;
    v_all_owned boolean;
BEGIN
    IF current_database()<>'espacigo_dms_stage_20261020'
       OR (SELECT oid FROM pg_catalog.pg_database WHERE datname=current_database())<>16388
       OR (SELECT datdba FROM pg_catalog.pg_database WHERE datname=current_database())<>'postgres'::regrole
       OR NOT (SELECT datallowconn FROM pg_catalog.pg_database WHERE datname=current_database())
       OR (SELECT datistemplate FROM pg_catalog.pg_database WHERE datname=current_database())
       OR current_setting('server_version_num')<>'180007'
       OR current_setting('port')<>'5432'
       OR session_user<>'postgres' OR current_user<>'postgres'
       OR inet_client_addr() IS NOT NULL OR inet_server_addr() IS NOT NULL THEN
        RAISE EXCEPTION 'native V95 finalizer post-commit identity mismatch';
    END IF;
    IF pg_catalog.has_schema_privilege('espacigo_schema_owner','public','CREATE') THEN
        RAISE EXCEPTION 'native V95 finalizer post-commit readback found public CREATE still effective';
    END IF;
    WITH actual AS (
      SELECT COALESCE(r.rolname,'PUBLIC') grantee,g.rolname grantor,a.privilege_type,a.is_grantable
      FROM pg_catalog.pg_namespace n CROSS JOIN LATERAL pg_catalog.aclexplode(n.nspacl) a
      LEFT JOIN pg_catalog.pg_roles r ON r.oid=a.grantee
      JOIN pg_catalog.pg_roles g ON g.oid=a.grantor
      WHERE n.nspname='public'
    ), expected(grantee,grantor,privilege_type,is_grantable) AS (
      VALUES ('PUBLIC','pg_database_owner','USAGE',false),
             ('dms_migrator','pg_database_owner','USAGE',false),
             ('espacigo_document_verification_fn','pg_database_owner','USAGE',false),
             ('espacigo_gcp06_test_access','pg_database_owner','USAGE',false),
             ('espacigo_privacy_cleanup_worker','pg_database_owner','USAGE',false),
             ('espacigo_runtime','pg_database_owner','USAGE',false),
             ('espacigo_schema_owner','pg_database_owner','USAGE',false),
             ('pg_database_owner','pg_database_owner','CREATE',false),
             ('pg_database_owner','pg_database_owner','USAGE',false)
    ), delta AS (
      (SELECT * FROM actual EXCEPT SELECT * FROM expected)
      UNION ALL (SELECT * FROM expected EXCEPT SELECT * FROM actual)
    ) SELECT count(*) INTO v_delta FROM delta;
    IF v_delta<>0 THEN
        RAISE EXCEPTION 'native V95 finalizer closed its grant but public ACL does not match the exact baseline';
    END IF;

    SELECT count(*),min(version),max(version),
           (SELECT name FROM public.schema_migrations WHERE version=94),
           (SELECT checksum FROM public.schema_migrations WHERE version=94),
           (SELECT name FROM public.schema_migrations WHERE version=95),
           (SELECT checksum FROM public.schema_migrations WHERE version=95)
      INTO v_count,v_min,v_max,v_v94_name,v_v94_checksum,v_v95_name,v_v95_checksum
      FROM public.schema_migrations;
    IF NOT (
      (v_count=94 AND v_min=1 AND v_max=94
       AND v_v94_name='V000094__identity_delivered_session_outbox_retirement.sql'
       AND v_v94_checksum='a4ee15a45ab352527e93c00e82763056b57b3750064483b8b90aa6891c0672a9')
      OR
      (v_count=95 AND v_min=1 AND v_max=95
       AND v_v94_name='V000094__identity_delivered_session_outbox_retirement.sql'
       AND v_v94_checksum='a4ee15a45ab352527e93c00e82763056b57b3750064483b8b90aa6891c0672a9'
       AND v_v95_name='V000095__identity_document_case_declaration_binding.sql'
       AND v_v95_checksum='351a9bbdbd890e34b3b5db46ecffa971d49b1f5d4519d9bf7f2b1eae1e897275')
    ) THEN
        RAISE EXCEPTION 'native V95 finalizer closed CREATE but ledger is outside the exact V94/V95 cleanup boundary';
    END IF;

    SELECT count(*)::integer,COALESCE(bool_and(pg_catalog.pg_get_userbyid(c.relowner)='espacigo_schema_owner'),false)
      INTO v_objects,v_all_owned
      FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
     WHERE n.nspname='public' AND c.relname = ANY(ARRAY[
       'document_verification_case','document_verification_event','document_verification_event_id_seq',
       'document_verification_case_pkey','document_verification_case_id_scope_uq',
       'document_verification_one_open_identity_uq','document_verification_identity_current_idx',
       'document_verification_one_open_space_subject_uq','document_verification_event_pkey',
       'document_verification_event_case_idx','document_verification_event_file_idx',
       'document_verification_case_identity_declaration_idx'
     ]);
    IF (v_count=94 AND (v_objects<>11 OR NOT v_all_owned))
       OR (v_count=95 AND (v_objects<>12 OR NOT v_all_owned)) THEN
        RAISE EXCEPTION 'native V95 finalizer owner readback differs from the exact authorized V94/V95 object set';
    END IF;
END
$verify$;
