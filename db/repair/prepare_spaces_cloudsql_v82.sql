-- Narrow DDL window proposal. Apply only after the V81 commit, as postgres.
-- Do not run V81 again against this intermediate post-V81 contract.
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
    OR (SELECT count(*) FROM public.schema_migrations)<>81
    OR (SELECT max(version) FROM public.schema_migrations)<>81 THEN
    RAISE EXCEPTION 'exact administrative target with committed V81 required';
 END IF;
 IF pg_has_role('postgres','espacigo_schema_owner','MEMBER')
    OR has_schema_privilege('espacigo_schema_owner','public','CREATE')
    OR (SELECT proowner FROM pg_proc WHERE oid='public.document_verification_space_owner_changed()'::regprocedure)<>'postgres'::regrole
    OR (SELECT relowner FROM pg_class WHERE oid='public.document_verification_reviewer_grant'::regclass)<>'postgres'::regrole THEN
    RAISE EXCEPTION 'unexpected ownership or pre-existing DDL window';
 END IF;
END;
$guard$;
-- This edge exists only across V82/V83 and is removed by the finalizer.
-- INHERIT lets the existing administrator restore the function owner afterward.
GRANT espacigo_schema_owner TO postgres WITH ADMIN FALSE, INHERIT TRUE, SET TRUE;
GRANT CREATE ON SCHEMA public TO espacigo_schema_owner;
ALTER FUNCTION public.document_verification_space_owner_changed() OWNER TO espacigo_schema_owner;
GRANT UPDATE(id) ON public.document_verification_reviewer_grant TO espacigo_schema_owner WITH GRANT OPTION;
DO $postcheck$
BEGIN
 IF (SELECT proowner FROM pg_proc WHERE oid='public.document_verification_space_owner_changed()'::regprocedure)<>'espacigo_schema_owner'::regrole
    OR NOT has_schema_privilege('espacigo_schema_owner','public','CREATE')
    OR NOT has_column_privilege('espacigo_schema_owner','public.document_verification_reviewer_grant','id','UPDATE WITH GRANT OPTION') THEN
    RAISE EXCEPTION 'bounded V82/V83 authority missing';
 END IF;
END;
$postcheck$;
COMMIT;
