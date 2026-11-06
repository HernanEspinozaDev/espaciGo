-- Close only this reviewed V82/V83 DDL window after committed V83.
BEGIN;
SET LOCAL lock_timeout='5s';
SET LOCAL statement_timeout='60s';
-- Same advisory key as cmd/dbmigrate; released with this transaction.
SELECT pg_catalog.pg_advisory_xact_lock(1163087943,1296648018);
DO $guard$
BEGIN
 IF current_database()<>'espacigo_dms_stage_20261020' OR session_user<>'postgres' OR current_user<>'postgres'
    OR current_setting('server_version_num')::int<>180006
    OR NOT (SELECT ssl FROM pg_stat_ssl WHERE pid=pg_backend_pid())
    OR (SELECT count(*) FROM public.schema_migrations)<>83
    OR (SELECT max(version) FROM public.schema_migrations)<>83 THEN
    RAISE EXCEPTION 'exact administrative target with committed V83 required';
 END IF;
 IF (SELECT proowner FROM pg_proc WHERE oid='public.document_verification_space_owner_changed()'::regprocedure)<>'espacigo_schema_owner'::regrole THEN
    RAISE EXCEPTION 'reviewed temporary function ownership missing';
 END IF;
END;
$guard$;
-- Preserve the exact V83 runtime privilege with its original table owner as
-- grantor before retiring the dependent, temporary schema-owner grant chain.
GRANT UPDATE(id) ON public.document_verification_reviewer_grant TO espacigo_runtime;
REVOKE UPDATE(id) ON public.document_verification_reviewer_grant FROM espacigo_schema_owner CASCADE;
-- Restore the pre-existing function owner; no application-wide ownership moves.
ALTER FUNCTION public.document_verification_space_owner_changed() OWNER TO postgres;
REVOKE CREATE ON SCHEMA public FROM espacigo_schema_owner;
REVOKE espacigo_schema_owner FROM postgres;
DO $postcheck$
BEGIN
 IF NOT EXISTS (
     SELECT 1 FROM pg_catalog.pg_attribute a
     CROSS JOIN LATERAL pg_catalog.aclexplode(a.attacl) acl
     WHERE a.attrelid='public.document_verification_reviewer_grant'::regclass
       AND a.attname='id' AND NOT a.attisdropped
       AND acl.grantee='espacigo_runtime'::regrole
       AND acl.grantor='postgres'::regrole
       AND acl.privilege_type='UPDATE' AND NOT acl.is_grantable
 ) OR EXISTS (
     SELECT 1 FROM pg_catalog.pg_attribute a
     CROSS JOIN LATERAL pg_catalog.aclexplode(a.attacl) acl
     WHERE a.attrelid='public.document_verification_reviewer_grant'::regclass
       AND a.attname='id' AND NOT a.attisdropped
       AND (
           acl.grantee='espacigo_schema_owner'::regrole
           OR (acl.grantee='espacigo_runtime'::regrole
               AND (acl.grantor<>'postgres'::regrole OR acl.is_grantable))
       )
 ) THEN
    RAISE EXCEPTION 'runtime direct column ACL or retired grant chain differs';
 END IF;
 IF has_schema_privilege('espacigo_schema_owner','public','CREATE')
    OR has_schema_privilege('espacigo_migrator_login','public','CREATE')
    OR has_schema_privilege('espacigo_runtime','public','CREATE')
    OR has_schema_privilege('espacigo_api_login','public','CREATE')
    OR pg_has_role('postgres','espacigo_schema_owner','MEMBER')
    OR has_column_privilege('espacigo_schema_owner','public.document_verification_reviewer_grant','id','UPDATE')
    OR NOT has_column_privilege('espacigo_runtime','public.document_verification_reviewer_grant','id','UPDATE')
    OR (SELECT proowner FROM pg_proc WHERE oid='public.document_verification_space_owner_changed()'::regprocedure)<>'postgres'::regrole THEN
    RAISE EXCEPTION 'temporary privileges remain or V83 runtime privilege missing';
 END IF;
 IF EXISTS(SELECT 1 FROM pg_roles WHERE rolname IN ('espacigo_schema_owner','espacigo_document_verification_fn','espacigo_runtime')
      AND (rolcanlogin OR rolsuper OR rolcreatedb OR rolcreaterole OR rolreplication OR rolbypassrls OR rolinherit)) THEN
    RAISE EXCEPTION 'restricted roles gained unsafe attributes';
 END IF;
END;
$postcheck$;
COMMIT;
