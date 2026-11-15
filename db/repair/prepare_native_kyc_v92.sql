-- Exact native V92 CREATE window; invoke only after reviewed source and backup.
BEGIN;
SELECT pg_catalog.pg_advisory_xact_lock(x'45535047'::int,x'4d494752'::int);
DO $guard$
BEGIN
 IF current_database()<>'espacigo_dms_stage_20261020'
 OR (SELECT oid FROM pg_catalog.pg_database WHERE datname=current_database())<>16388
 OR current_setting('server_version_num')<>'180006'
 OR session_user<>'postgres' OR current_user<>'postgres' THEN
  RAISE EXCEPTION 'native V92 preparation target/administrator mismatch';
 END IF;
 IF (SELECT count(*) FROM public.schema_migrations)<>91
 OR (SELECT min(version) FROM public.schema_migrations)<>1
 OR (SELECT max(version) FROM public.schema_migrations)<>91
 OR (SELECT checksum FROM public.schema_migrations WHERE version=91) IS DISTINCT FROM '6ceee080736bd965a27226e3810d28ede0ecf2deb52477716a504e2b425c1250' THEN
  RAISE EXCEPTION 'native V92 requires the reviewed V91 ledger boundary';
 END IF;
 IF pg_catalog.has_schema_privilege('espacigo_schema_owner','public','CREATE') THEN
  RAISE EXCEPTION 'native V92 refuses pre-existing schema CREATE authority';
 END IF;
 IF NOT EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname='espacigo_schema_owner'
 AND NOT rolcanlogin AND NOT rolsuper AND NOT rolinherit AND NOT rolcreaterole
 AND NOT rolcreatedb AND NOT rolreplication AND NOT rolbypassrls) THEN
  RAISE EXCEPTION 'native V92 schema-owner flags mismatch';
 END IF;
 IF NOT EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname='espacigo_runtime'
 AND NOT rolcanlogin AND NOT rolsuper AND NOT rolinherit AND NOT rolcreaterole
 AND NOT rolcreatedb AND NOT rolreplication AND NOT rolbypassrls) THEN
  RAISE EXCEPTION 'native V92 runtime role flags mismatch';
 END IF;
 IF pg_catalog.has_function_privilege('espacigo_runtime',
 'public.purge_expired_identity_declaration_revisions()','EXECUTE')
 OR NOT pg_catalog.has_function_privilege('espacigo_privacy_cleanup_worker',
 'public.purge_expired_identity_declaration_revisions()','EXECUTE') THEN
  RAISE EXCEPTION 'native V92 worker-only predecessor ACL mismatch';
 END IF;
END
$guard$;
GRANT CREATE ON SCHEMA public TO espacigo_schema_owner;
DO $verify$
BEGIN
 IF NOT pg_catalog.has_schema_privilege('espacigo_schema_owner','public','CREATE') THEN
  RAISE EXCEPTION 'native V92 temporary CREATE readback failed';
 END IF;
END
$verify$;
COMMIT;
