-- Native V96 preparation only: no table ownership changes are permitted here.
-- Run with psql -X -v ON_ERROR_STOP=1 on the exact reviewed native target.
BEGIN;
SELECT pg_catalog.pg_advisory_xact_lock(x'45535047'::int,x'4d494752'::int);
SET LOCAL search_path TO public;

DO $guard$
DECLARE
    v_schema_owner oid;
    v_migrator oid;
    v_runtime oid;
    v_function_role oid;
    v_worker oid;
    v_worker_login oid;
    v_database_owner oid;
    v_case oid;
    v_event oid;
    v_sequence oid;
    v_declaration oid;
    v_purge oid;
    v_case_owner oid;
    v_event_owner oid;
    v_window_create boolean;
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
    v_object_count integer;
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
        RAISE EXCEPTION 'native V96 preparation target/peer administrator mismatch';
    END IF;

    SELECT oid INTO v_schema_owner FROM pg_catalog.pg_roles
     WHERE rolname='espacigo_schema_owner' AND NOT rolcanlogin AND NOT rolsuper
       AND NOT rolinherit AND NOT rolcreaterole AND NOT rolcreatedb
       AND NOT rolreplication AND NOT rolbypassrls;
    SELECT oid INTO v_migrator FROM pg_catalog.pg_roles
     WHERE rolname='espacigo_migrator_login' AND rolcanlogin AND NOT rolsuper
       AND NOT rolinherit AND NOT rolcreaterole AND NOT rolcreatedb
       AND NOT rolreplication AND NOT rolbypassrls;
    SELECT oid INTO v_runtime FROM pg_catalog.pg_roles
     WHERE rolname='espacigo_runtime' AND NOT rolcanlogin AND NOT rolsuper
       AND NOT rolinherit AND NOT rolcreaterole AND NOT rolcreatedb
       AND NOT rolreplication AND NOT rolbypassrls;
    SELECT oid INTO v_function_role FROM pg_catalog.pg_roles
     WHERE rolname='espacigo_document_verification_fn' AND NOT rolcanlogin
       AND NOT rolsuper AND NOT rolinherit AND NOT rolcreaterole AND NOT rolcreatedb
       AND NOT rolreplication AND NOT rolbypassrls;
    SELECT oid INTO v_worker FROM pg_catalog.pg_roles
     WHERE rolname='espacigo_privacy_cleanup_worker' AND NOT rolcanlogin
       AND NOT rolsuper AND NOT rolinherit AND NOT rolcreaterole AND NOT rolcreatedb
       AND NOT rolreplication AND NOT rolbypassrls;
    SELECT oid INTO v_worker_login FROM pg_catalog.pg_roles
     WHERE rolname='espacigo_privacy_worker_login' AND rolcanlogin
       AND NOT rolsuper AND NOT rolinherit AND NOT rolcreaterole AND NOT rolcreatedb
       AND NOT rolreplication AND NOT rolbypassrls;
    SELECT oid INTO v_database_owner FROM pg_catalog.pg_roles WHERE rolname='pg_database_owner';
    v_case := pg_catalog.to_regclass('public.document_verification_case');
    v_event := pg_catalog.to_regclass('public.document_verification_event');
    v_sequence := pg_catalog.to_regclass('public.document_verification_event_id_seq');
    v_declaration := pg_catalog.to_regclass('public.identity_declaration_revision');
    v_purge := pg_catalog.to_regprocedure('public.purge_expired_identity_declaration_revisions()');
    IF v_schema_owner IS NULL OR v_migrator IS NULL OR v_runtime IS NULL
       OR v_function_role IS NULL OR v_worker IS NULL OR v_worker_login IS NULL
       OR v_database_owner IS NULL OR v_declaration IS NULL OR v_purge IS NULL
       OR v_case IS NULL OR v_event IS NULL OR v_sequence IS NULL
       OR (SELECT nspowner FROM pg_catalog.pg_namespace WHERE nspname='public')<>v_database_owner
       OR (SELECT relowner FROM pg_catalog.pg_class WHERE oid=v_declaration)<>v_schema_owner THEN
        RAISE EXCEPTION 'native V96 preparation role/schema/object contract mismatch';
    END IF;

    SELECT count(*),min(version),max(version),
           (SELECT name FROM public.schema_migrations WHERE version=94),
           (SELECT checksum FROM public.schema_migrations WHERE version=94),
           (SELECT name FROM public.schema_migrations WHERE version=95),
           (SELECT checksum FROM public.schema_migrations WHERE version=95)
      INTO v_count,v_min,v_max,v_v94_name,v_v94_checksum,v_v95_name,v_v95_checksum
      FROM public.schema_migrations;
    IF v_count<>95 OR v_min<>1 OR v_max<>95
       OR v_v94_name IS DISTINCT FROM 'V000094__identity_delivered_session_outbox_retirement.sql'
       OR v_v94_checksum IS DISTINCT FROM 'a4ee15a45ab352527e93c00e82763056b57b3750064483b8b90aa6891c0672a9'
       OR v_v95_name IS DISTINCT FROM 'V000095__identity_document_case_declaration_binding.sql'
       OR v_v95_checksum IS DISTINCT FROM '351a9bbdbd890e34b3b5db46ecffa971d49b1f5d4519d9bf7f2b1eae1e897275' THEN
        RAISE EXCEPTION 'native V96 preparation requires exact contiguous V1..V95 ledger';
    END IF;

    SELECT relowner INTO v_case_owner FROM pg_catalog.pg_class WHERE oid=v_case AND relkind='r';
    SELECT relowner INTO v_event_owner FROM pg_catalog.pg_class WHERE oid=v_event AND relkind='r';
    IF v_case_owner<>v_schema_owner OR v_event_owner<>v_schema_owner
       OR (SELECT relowner FROM pg_catalog.pg_class WHERE oid=v_sequence)<>v_schema_owner
       OR pg_catalog.pg_get_serial_sequence('public.document_verification_event','id')::regclass::oid IS DISTINCT FROM v_sequence
       OR NOT pg_catalog.has_table_privilege(v_runtime,v_case,'SELECT')
       OR NOT pg_catalog.has_table_privilege(v_runtime,v_event,'INSERT')
       OR NOT pg_catalog.has_column_privilege(v_runtime,v_case,'document_expires_at','UPDATE')
       OR (SELECT prosecdef FROM pg_catalog.pg_proc WHERE oid=v_purge) IS DISTINCT FROM true
       OR (SELECT proowner FROM pg_catalog.pg_proc WHERE oid=v_purge)<>v_schema_owner
       OR (SELECT proconfig FROM pg_catalog.pg_proc WHERE oid=v_purge) IS DISTINCT FROM ARRAY['search_path=pg_catalog']::text[]
       OR (SELECT count(*) FROM pg_catalog.pg_proc p CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(p.proacl,pg_catalog.acldefault('f'::"char",p.proowner))) a WHERE p.oid=v_purge AND a.grantee<>p.proowner)<>1
       OR NOT EXISTS (SELECT 1 FROM pg_catalog.pg_proc p CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(p.proacl,pg_catalog.acldefault('f'::"char",p.proowner))) a WHERE p.oid=v_purge AND a.grantee=v_worker AND a.grantor=v_schema_owner AND a.privilege_type='EXECUTE' AND NOT a.is_grantable)
       OR pg_catalog.has_function_privilege(v_runtime,v_purge,'EXECUTE')
       OR NOT pg_catalog.has_function_privilege(v_worker,v_purge,'EXECUTE') THEN
        RAISE EXCEPTION 'native V96 preparation requires schema-owned V95 tables and the reviewed runtime path';
    END IF;

    IF EXISTS (SELECT 1 FROM pg_catalog.pg_attribute a
                WHERE a.attrelid=v_case AND a.attname='document_format_observed'
                  AND a.attnum>0 AND NOT a.attisdropped)
       OR EXISTS (SELECT 1 FROM pg_catalog.pg_attribute a
                  WHERE a.attrelid=v_event AND a.attname='identity_review_observations'
                    AND a.attnum>0 AND NOT a.attisdropped)
       OR pg_catalog.to_regprocedure('public.document_verification_identity_review_audit_guard()') IS NOT NULL THEN
        RAISE EXCEPTION 'native V96 preparation refuses a partial V96 schema beside the V95 ledger';
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_catalog.pg_attribute a WHERE a.attrelid=v_case AND a.attname='identity_declaration_revision_id' AND a.attnum>0 AND NOT a.attisdropped)
       OR NOT EXISTS (SELECT 1 FROM pg_catalog.pg_attribute a WHERE a.attrelid=v_case AND a.attname='document_format_declared' AND a.attnum>0 AND NOT a.attisdropped)
       OR NOT EXISTS (SELECT 1 FROM pg_catalog.pg_attribute a WHERE a.attrelid=v_case AND a.attname='capture_consent_version' AND a.attnum>0 AND NOT a.attisdropped)
       OR NOT EXISTS (SELECT 1 FROM pg_catalog.pg_attribute a WHERE a.attrelid=v_case AND a.attname='capture_consented_at' AND a.attnum>0 AND NOT a.attisdropped) THEN
        RAISE EXCEPTION 'native V96 preparation requires all V95 declaration/consent columns';
    END IF;

    WITH expected(relname,table_oid) AS (
      VALUES ('document_verification_case_pkey',v_case),
             ('document_verification_case_id_scope_uq',v_case),
             ('document_verification_one_open_identity_uq',v_case),
             ('document_verification_identity_current_idx',v_case),
             ('document_verification_one_open_space_subject_uq',v_case),
             ('document_verification_case_identity_declaration_idx',v_case),
             ('document_verification_event_pkey',v_event),
             ('document_verification_event_case_idx',v_event),
             ('document_verification_event_file_idx',v_event)
    ), actual AS (
      SELECT idx.relname::text,i.indrelid
      FROM pg_catalog.pg_index i JOIN pg_catalog.pg_class idx ON idx.oid=i.indexrelid
      WHERE i.indrelid IN (v_case,v_event)
    ), delta AS (
      (SELECT * FROM actual EXCEPT SELECT * FROM expected)
      UNION ALL (SELECT * FROM expected EXCEPT SELECT * FROM actual)
    ) SELECT (SELECT count(*) FROM actual),(SELECT count(*) FROM delta)
      INTO v_object_count,v_delta;
    IF v_object_count<>9 OR v_delta<>0
       OR EXISTS (SELECT 1 FROM pg_catalog.pg_index i JOIN pg_catalog.pg_class idx ON idx.oid=i.indexrelid
                  WHERE i.indrelid IN (v_case,v_event) AND idx.relowner<>v_schema_owner)
       OR EXISTS (SELECT 1 FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
                  WHERE n.nspname='public' AND c.relname=ANY(ARRAY[
                    'document_verification_case','document_verification_event','document_verification_event_id_seq',
                    'document_verification_case_pkey','document_verification_case_id_scope_uq',
                    'document_verification_one_open_identity_uq','document_verification_identity_current_idx',
                    'document_verification_one_open_space_subject_uq','document_verification_case_identity_declaration_idx',
                    'document_verification_event_pkey','document_verification_event_case_idx','document_verification_event_file_idx'
                  ]) AND c.relowner<>v_schema_owner) THEN
        RAISE EXCEPTION 'native V96 preparation refuses owner/index drift from exact post-V95 state';
    END IF;

    SELECT EXISTS (SELECT 1 FROM pg_catalog.pg_namespace n CROSS JOIN LATERAL pg_catalog.aclexplode(n.nspacl) a
                   WHERE n.nspname='public' AND a.grantee=v_schema_owner
                     AND a.grantor=v_database_owner AND a.privilege_type='CREATE' AND NOT a.is_grantable)
      INTO v_window_create;
    IF pg_catalog.has_schema_privilege(v_schema_owner,'public','CREATE') IS DISTINCT FROM v_window_create THEN
        RAISE EXCEPTION 'native V96 preparation effective CREATE differs from the exact direct ACL';
    END IF;
    WITH actual AS (
      SELECT COALESCE(r.rolname,'PUBLIC') grantee,g.rolname grantor,a.privilege_type,a.is_grantable
      FROM pg_catalog.pg_namespace n CROSS JOIN LATERAL pg_catalog.aclexplode(n.nspacl) a
      LEFT JOIN pg_catalog.pg_roles r ON r.oid=a.grantee JOIN pg_catalog.pg_roles g ON g.oid=a.grantor
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
      UNION ALL SELECT 'espacigo_schema_owner','pg_database_owner','CREATE',false WHERE v_window_create
    ), delta AS (
      (SELECT * FROM actual EXCEPT SELECT * FROM expected)
      UNION ALL (SELECT * FROM expected EXCEPT SELECT * FROM actual)
    ) SELECT count(*) INTO v_delta FROM delta;
    IF v_delta<>0 THEN
        RAISE EXCEPTION 'native V96 preparation public-schema ACL baseline mismatch';
    END IF;

    WITH actual(object_name,column_name,grantee,grantor,privilege_type,is_grantable) AS (
      SELECT c.relname::text,''::text,COALESCE(r.rolname,'PUBLIC'),g.rolname,a.privilege_type,a.is_grantable
      FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
      CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(c.relacl,pg_catalog.acldefault((CASE WHEN c.relkind='S' THEN 'S' ELSE 'r' END)::pg_catalog."char",c.relowner))) a
      LEFT JOIN pg_catalog.pg_roles r ON r.oid=a.grantee JOIN pg_catalog.pg_roles g ON g.oid=a.grantor
      WHERE n.nspname='public' AND c.oid IN (v_case,v_event,v_sequence) AND a.grantee<>c.relowner
      UNION ALL
      SELECT c.relname::text,att.attname::text,COALESCE(r.rolname,'PUBLIC'),g.rolname,a.privilege_type,a.is_grantable
      FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace JOIN pg_catalog.pg_attribute att ON att.attrelid=c.oid
      CROSS JOIN LATERAL pg_catalog.aclexplode(CASE WHEN pg_catalog.cardinality(att.attacl)>0 THEN att.attacl ELSE NULL::aclitem[] END) a
      LEFT JOIN pg_catalog.pg_roles r ON r.oid=a.grantee JOIN pg_catalog.pg_roles g ON g.oid=a.grantor
      WHERE n.nspname='public' AND c.oid IN (v_case,v_event) AND att.attnum>0 AND NOT att.attisdropped AND a.grantee<>c.relowner
    ), expected(object_name,column_name,grantee,grantor,privilege_type,is_grantable) AS (
      VALUES ('document_verification_case','','espacigo_runtime','espacigo_schema_owner','SELECT',false),
             ('document_verification_event','','espacigo_runtime','espacigo_schema_owner','SELECT',false),
             ('document_verification_event','','espacigo_runtime','espacigo_schema_owner','INSERT',false),
             ('document_verification_event_id_seq','','espacigo_runtime','espacigo_schema_owner','USAGE',false),
             ('document_verification_case','document_expires_at','espacigo_runtime','espacigo_schema_owner','INSERT',false),
             ('document_verification_case','id','espacigo_runtime','espacigo_schema_owner','INSERT',false),
             ('document_verification_case','purpose','espacigo_runtime','espacigo_schema_owner','INSERT',false),
             ('document_verification_case','space_id','espacigo_runtime','espacigo_schema_owner','INSERT',false),
             ('document_verification_case','subject_id','espacigo_runtime','espacigo_schema_owner','INSERT',false),
             ('document_verification_case','document_expires_at','espacigo_runtime','espacigo_schema_owner','UPDATE',false),
             ('document_verification_case','reviewer_id','espacigo_runtime','espacigo_schema_owner','UPDATE',false),
             ('document_verification_case','revision','espacigo_runtime','espacigo_schema_owner','UPDATE',false),
             ('document_verification_case','revoked_at','espacigo_runtime','espacigo_schema_owner','UPDATE',false),
             ('document_verification_case','state','espacigo_runtime','espacigo_schema_owner','UPDATE',false),
             ('document_verification_case','superseded_at','espacigo_runtime','espacigo_schema_owner','UPDATE',false),
             ('document_verification_case','updated_at','espacigo_runtime','espacigo_schema_owner','UPDATE',false),
             ('document_verification_case','verified_at','espacigo_runtime','espacigo_schema_owner','UPDATE',false),
             ('document_verification_case','verified_until','espacigo_runtime','espacigo_schema_owner','UPDATE',false),
             ('document_verification_case','identity_declaration_revision_id','espacigo_runtime','espacigo_schema_owner','INSERT',false),
             ('document_verification_case','document_format_declared','espacigo_runtime','espacigo_schema_owner','INSERT',false),
             ('document_verification_case','capture_consent_version','espacigo_runtime','espacigo_schema_owner','INSERT',false),
             ('document_verification_case','capture_consented_at','espacigo_runtime','espacigo_schema_owner','INSERT',false),
             ('document_verification_case','identity_declaration_revision_id','espacigo_runtime','espacigo_schema_owner','UPDATE',false)
    ), delta AS (
      (SELECT * FROM actual EXCEPT SELECT * FROM expected)
      UNION ALL (SELECT * FROM expected EXCEPT SELECT * FROM actual)
    ) SELECT count(*) INTO v_delta FROM delta;
    IF v_delta<>0 THEN
        RAISE EXCEPTION 'native V96 preparation V95 runtime ACL/grantor baseline mismatch';
    END IF;

    IF (SELECT relowner FROM pg_catalog.pg_class WHERE oid='public.schema_migrations'::regclass)<>'postgres'::regrole
       OR NOT pg_catalog.has_table_privilege(v_schema_owner,'public.schema_migrations','SELECT')
       OR NOT pg_catalog.has_table_privilege(v_schema_owner,'public.schema_migrations','INSERT') THEN
        RAISE EXCEPTION 'native V96 preparation migration-ledger owner/effective ACL mismatch';
    END IF;
    WITH actual(object_name,column_name,grantee,grantor,privilege_type,is_grantable) AS (
      SELECT c.relname::text,''::text,COALESCE(r.rolname,'PUBLIC'),g.rolname,a.privilege_type,a.is_grantable
      FROM pg_catalog.pg_class c CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(c.relacl,pg_catalog.acldefault('r'::"char",c.relowner))) a
      LEFT JOIN pg_catalog.pg_roles r ON r.oid=a.grantee JOIN pg_catalog.pg_roles g ON g.oid=a.grantor
      WHERE c.oid='public.schema_migrations'::regclass AND a.grantee<>c.relowner
      UNION ALL
      SELECT c.relname::text,att.attname::text,COALESCE(r.rolname,'PUBLIC'),g.rolname,a.privilege_type,a.is_grantable
      FROM pg_catalog.pg_class c JOIN pg_catalog.pg_attribute att ON att.attrelid=c.oid
      CROSS JOIN LATERAL pg_catalog.aclexplode(CASE WHEN pg_catalog.cardinality(att.attacl)>0 THEN att.attacl ELSE NULL::aclitem[] END) a
      LEFT JOIN pg_catalog.pg_roles r ON r.oid=a.grantee JOIN pg_catalog.pg_roles g ON g.oid=a.grantor
      WHERE c.oid='public.schema_migrations'::regclass AND att.attnum>0 AND NOT att.attisdropped AND a.grantee<>c.relowner
    ), expected(object_name,column_name,grantee,grantor,privilege_type,is_grantable) AS (
      VALUES ('schema_migrations','','espacigo_schema_owner','postgres','SELECT',false),
             ('schema_migrations','','espacigo_schema_owner','postgres','INSERT',false)
    ), delta AS (
      (SELECT * FROM actual EXCEPT SELECT * FROM expected)
      UNION ALL (SELECT * FROM expected EXCEPT SELECT * FROM actual)
    ) SELECT count(*) INTO v_delta FROM delta;
    IF v_delta<>0 THEN
        RAISE EXCEPTION 'native V96 preparation refuses migration-ledger direct ACL drift';
    END IF;

    IF (SELECT count(*) FROM pg_catalog.pg_auth_members m WHERE m.roleid=v_schema_owner OR m.member=v_schema_owner)<>2
       OR EXISTS (SELECT 1 FROM pg_catalog.pg_auth_members m
                  WHERE (m.roleid=v_schema_owner OR m.member=v_schema_owner)
                    AND NOT (m.grantor='postgres'::regrole AND NOT m.admin_option
                      AND NOT m.inherit_option AND m.set_option
                      AND ((m.roleid=v_schema_owner AND m.member=v_migrator)
                        OR (m.roleid=v_function_role AND m.member=v_schema_owner))))
       OR (SELECT count(*) FROM pg_catalog.pg_auth_members m WHERE m.roleid=v_worker OR m.member=v_worker)<>1
       OR NOT EXISTS (SELECT 1 FROM pg_catalog.pg_auth_members m WHERE m.roleid=v_worker AND m.member=v_worker_login AND m.grantor='postgres'::regrole AND NOT m.admin_option AND NOT m.inherit_option AND m.set_option) THEN
        RAISE EXCEPTION 'native V96 preparation role-membership contract mismatch';
    END IF;
END
$guard$;

GRANT CREATE ON SCHEMA public TO espacigo_schema_owner;

DO $verify$
DECLARE
    v_schema_owner oid := 'espacigo_schema_owner'::regrole;
    v_database_owner oid := 'pg_database_owner'::regrole;
    v_delta bigint;
    v_count bigint;
    v_min bigint;
    v_max bigint;
BEGIN
    IF NOT pg_catalog.has_schema_privilege(v_schema_owner,'public','CREATE')
       OR (SELECT nspowner FROM pg_catalog.pg_namespace WHERE nspname='public')<>v_database_owner
       OR (SELECT relowner FROM pg_catalog.pg_class WHERE oid='public.document_verification_case'::regclass)<>v_schema_owner
       OR (SELECT relowner FROM pg_catalog.pg_class WHERE oid='public.document_verification_event'::regclass)<>v_schema_owner
       OR (SELECT relowner FROM pg_catalog.pg_class WHERE oid='public.document_verification_event_id_seq'::regclass)<>v_schema_owner THEN
        RAISE EXCEPTION 'native V96 preparation post-owner or temporary CREATE verification failed';
    END IF;
    WITH actual AS (
      SELECT COALESCE(r.rolname,'PUBLIC') grantee,g.rolname grantor,a.privilege_type,a.is_grantable
      FROM pg_catalog.pg_namespace n CROSS JOIN LATERAL pg_catalog.aclexplode(n.nspacl) a
      LEFT JOIN pg_catalog.pg_roles r ON r.oid=a.grantee JOIN pg_catalog.pg_roles g ON g.oid=a.grantor
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
             ('pg_database_owner','pg_database_owner','USAGE',false),
             ('espacigo_schema_owner','pg_database_owner','CREATE',false)
    ), delta AS (
      (SELECT * FROM actual EXCEPT SELECT * FROM expected)
      UNION ALL (SELECT * FROM expected EXCEPT SELECT * FROM actual)
    ) SELECT count(*) INTO v_delta FROM delta;
    IF v_delta<>0 THEN
        RAISE EXCEPTION 'native V96 preparation post-public ACL is not exact baseline plus temporary CREATE';
    END IF;
    IF EXISTS (SELECT 1 FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
               WHERE n.nspname='public' AND c.relname=ANY(ARRAY[
                 'document_verification_case','document_verification_event','document_verification_event_id_seq',
                 'document_verification_case_pkey','document_verification_case_id_scope_uq',
                 'document_verification_one_open_identity_uq','document_verification_identity_current_idx',
                 'document_verification_one_open_space_subject_uq','document_verification_case_identity_declaration_idx',
                 'document_verification_event_pkey','document_verification_event_case_idx','document_verification_event_file_idx'
               ]) AND c.relowner<>v_schema_owner) THEN
        RAISE EXCEPTION 'native V96 preparation changed an existing reviewed owner';
    END IF;
    SELECT count(*),min(version),max(version) INTO v_count,v_min,v_max FROM public.schema_migrations;
    IF v_count<>95 OR v_min<>1 OR v_max<>95
       OR (SELECT name FROM public.schema_migrations WHERE version=95) IS DISTINCT FROM 'V000095__identity_document_case_declaration_binding.sql'
       OR (SELECT checksum FROM public.schema_migrations WHERE version=95) IS DISTINCT FROM '351a9bbdbd890e34b3b5db46ecffa971d49b1f5d4519d9bf7f2b1eae1e897275' THEN
        RAISE EXCEPTION 'native V96 preparation changed or drifted the V95 migration ledger';
    END IF;
END
$verify$;
COMMIT;
