-- Close only the separately authorized V96 temporary public CREATE grant.
-- Works after failed V96 apply (ledger V95) or committed V96 (ledger V96).
-- Run with psql -X -v ON_ERROR_STOP=1; post-COMMIT checks cannot undo REVOKE.
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
        RAISE EXCEPTION 'native V96 finalizer target/peer administrator mismatch';
    END IF;
    SELECT oid INTO v_schema_owner FROM pg_catalog.pg_roles WHERE rolname='espacigo_schema_owner';
    SELECT oid INTO v_database_owner FROM pg_catalog.pg_roles WHERE rolname='pg_database_owner';
    IF v_schema_owner IS NULL OR v_database_owner IS NULL THEN
        RAISE EXCEPTION 'native V96 finalizer requires existing schema/database-owner roles';
    END IF;
    IF EXISTS (
      SELECT 1 FROM pg_catalog.pg_namespace n
      CROSS JOIN LATERAL pg_catalog.aclexplode(n.nspacl) a
      WHERE n.nspname='public' AND a.grantee=v_schema_owner
        AND a.grantor=v_database_owner AND a.privilege_type='CREATE'
        AND a.is_grantable
    ) THEN
        RAISE EXCEPTION 'native V96 finalizer refuses to alter a pre-existing CREATE grant option';
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
    v_v96_name text;
    v_v96_checksum text;
    v_objects integer;
    v_all_owned boolean;
    v_schema_owner oid := 'espacigo_schema_owner'::regrole;
    v_v96 boolean;
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
        RAISE EXCEPTION 'native V96 finalizer post-commit identity mismatch';
    END IF;
    IF pg_catalog.has_schema_privilege(v_schema_owner,'public','CREATE') THEN
        RAISE EXCEPTION 'native V96 finalizer post-commit readback found public CREATE still effective';
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
        RAISE EXCEPTION 'native V96 finalizer closed CREATE but public ACL differs from the exact baseline';
    END IF;

    SELECT count(*),min(version),max(version),
           (SELECT name FROM public.schema_migrations WHERE version=94),
           (SELECT checksum FROM public.schema_migrations WHERE version=94),
           (SELECT name FROM public.schema_migrations WHERE version=95),
           (SELECT checksum FROM public.schema_migrations WHERE version=95),
           (SELECT name FROM public.schema_migrations WHERE version=96),
           (SELECT checksum FROM public.schema_migrations WHERE version=96)
      INTO v_count,v_min,v_max,v_v94_name,v_v94_checksum,v_v95_name,v_v95_checksum,v_v96_name,v_v96_checksum
      FROM public.schema_migrations;
    v_v96 := v_count=96;
    IF NOT (
      (v_count=95 AND v_min=1 AND v_max=95
       AND v_v94_name='V000094__identity_delivered_session_outbox_retirement.sql'
       AND v_v94_checksum='a4ee15a45ab352527e93c00e82763056b57b3750064483b8b90aa6891c0672a9'
       AND v_v95_name='V000095__identity_document_case_declaration_binding.sql'
       AND v_v95_checksum='351a9bbdbd890e34b3b5db46ecffa971d49b1f5d4519d9bf7f2b1eae1e897275')
      OR
      (v_count=96 AND v_min=1 AND v_max=96
       AND v_v94_name='V000094__identity_delivered_session_outbox_retirement.sql'
       AND v_v94_checksum='a4ee15a45ab352527e93c00e82763056b57b3750064483b8b90aa6891c0672a9'
       AND v_v95_name='V000095__identity_document_case_declaration_binding.sql'
       AND v_v95_checksum='351a9bbdbd890e34b3b5db46ecffa971d49b1f5d4519d9bf7f2b1eae1e897275'
       AND v_v96_name='V000096__identity_reviewer_observation_audit.sql'
       AND v_v96_checksum='a206e74995db47112c6b382c33ffab5badcab2ec9b9859af0fbd018478cc7ca5')
    ) THEN
        RAISE EXCEPTION 'native V96 finalizer closed CREATE but ledger is outside the exact V95/V96 cleanup boundary';
    END IF;

    SELECT count(*)::integer,COALESCE(bool_and(pg_catalog.pg_get_userbyid(c.relowner)='espacigo_schema_owner'),false)
      INTO v_objects,v_all_owned
      FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
     WHERE n.nspname='public' AND c.relname = ANY(ARRAY[
       'document_verification_case','document_verification_event','document_verification_event_id_seq',
       'document_verification_case_pkey','document_verification_case_id_scope_uq',
       'document_verification_one_open_identity_uq','document_verification_identity_current_idx',
       'document_verification_one_open_space_subject_uq','document_verification_case_identity_declaration_idx',
       'document_verification_event_pkey','document_verification_event_case_idx','document_verification_event_file_idx'
     ]);
    IF v_objects<>12 OR NOT v_all_owned THEN
        RAISE EXCEPTION 'native V96 finalizer owner readback differs from the exact 12 authorized objects';
    END IF;

    WITH actual(object_name,column_name,grantee,grantor,privilege_type,is_grantable) AS (
      SELECT c.relname::text,''::text,COALESCE(r.rolname,'PUBLIC'),g.rolname,a.privilege_type,a.is_grantable
      FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
      CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(c.relacl,pg_catalog.acldefault(CASE WHEN c.relkind='S' THEN 'S'::"char" ELSE 'r'::"char" END,c.relowner))) a
      LEFT JOIN pg_catalog.pg_roles r ON r.oid=a.grantee JOIN pg_catalog.pg_roles g ON g.oid=a.grantor
      WHERE n.nspname='public' AND c.relname IN ('document_verification_case','document_verification_event','document_verification_event_id_seq') AND a.grantee<>c.relowner
      UNION ALL
      SELECT c.relname::text,att.attname::text,COALESCE(r.rolname,'PUBLIC'),g.rolname,a.privilege_type,a.is_grantable
      FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
      JOIN pg_catalog.pg_attribute att ON att.attrelid=c.oid
      CROSS JOIN LATERAL pg_catalog.aclexplode(CASE WHEN pg_catalog.cardinality(att.attacl)>0 THEN att.attacl ELSE NULL::aclitem[] END) a
      LEFT JOIN pg_catalog.pg_roles r ON r.oid=a.grantee JOIN pg_catalog.pg_roles g ON g.oid=a.grantor
      WHERE n.nspname='public' AND c.relname IN ('document_verification_case','document_verification_event') AND att.attnum>0 AND NOT att.attisdropped AND a.grantee<>c.relowner
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
      UNION ALL SELECT 'document_verification_case','document_format_observed','espacigo_runtime','espacigo_schema_owner','UPDATE',false WHERE v_v96
      UNION ALL SELECT 'document_verification_event','identity_review_observations','espacigo_runtime','espacigo_schema_owner','INSERT',false WHERE v_v96
    ), delta AS (
      (SELECT * FROM actual EXCEPT SELECT * FROM expected)
      UNION ALL (SELECT * FROM expected EXCEPT SELECT * FROM actual)
    ) SELECT count(*) INTO v_delta FROM delta;
    IF v_delta<>0 THEN
        RAISE EXCEPTION 'native V96 finalizer runtime ACL/grantor tuples differ from the exact V95/V96 contract';
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
        RAISE EXCEPTION 'native V96 finalizer refuses migration-ledger direct ACL drift';
    END IF;

    IF v_v96 AND (
       NOT EXISTS (SELECT 1 FROM pg_catalog.pg_attribute a WHERE a.attrelid='public.document_verification_case'::regclass AND a.attname='document_format_observed' AND a.attnum>0 AND NOT a.attisdropped)
       OR NOT EXISTS (SELECT 1 FROM pg_catalog.pg_attribute a WHERE a.attrelid='public.document_verification_event'::regclass AND a.attname='identity_review_observations' AND a.attnum>0 AND NOT a.attisdropped)
       OR NOT EXISTS (SELECT 1 FROM pg_catalog.pg_proc p WHERE p.oid=pg_catalog.to_regprocedure('public.document_verification_identity_review_audit_guard()') AND p.proowner=v_schema_owner AND p.prosecdef=false)
       OR NOT EXISTS (SELECT 1 FROM pg_catalog.pg_constraint c WHERE c.conrelid='public.document_verification_case'::regclass AND c.conname='document_verification_case_observed_format_ck')
       OR NOT EXISTS (SELECT 1 FROM pg_catalog.pg_constraint c WHERE c.conrelid='public.document_verification_event'::regclass AND c.conname='document_verification_event_identity_observations_ck')
    ) THEN
        RAISE EXCEPTION 'native V96 finalizer ledger is V96 but required V96 schema is incomplete';
    ELSIF NOT v_v96 AND (
       EXISTS (SELECT 1 FROM pg_catalog.pg_attribute a WHERE a.attrelid='public.document_verification_case'::regclass AND a.attname='document_format_observed' AND a.attnum>0 AND NOT a.attisdropped)
       OR EXISTS (SELECT 1 FROM pg_catalog.pg_attribute a WHERE a.attrelid='public.document_verification_event'::regclass AND a.attname='identity_review_observations' AND a.attnum>0 AND NOT a.attisdropped)
       OR pg_catalog.to_regprocedure('public.document_verification_identity_review_audit_guard()') IS NOT NULL
    ) THEN
        RAISE EXCEPTION 'native V96 finalizer found partial V96 schema beside the V95 ledger';
    END IF;
END
$verify$;
