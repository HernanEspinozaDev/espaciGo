-- Exact native V93 CREATE window; invoke only after reviewed source and backup.
BEGIN;
SELECT pg_catalog.pg_advisory_xact_lock(x'45535047'::int,x'4d494752'::int);
DO $guard$
BEGIN
 IF current_database()<>'espacigo_dms_stage_20261020'
 OR (SELECT oid FROM pg_catalog.pg_database WHERE datname=current_database())<>16388
 OR current_setting('server_version_num')<>'180006'
 OR session_user<>'postgres' OR current_user<>'postgres' THEN
  RAISE EXCEPTION 'native V93 preparation target/administrator mismatch';
 END IF;
 IF (SELECT count(*) FROM public.schema_migrations)<>92
 OR (SELECT min(version) FROM public.schema_migrations)<>1
 OR (SELECT max(version) FROM public.schema_migrations)<>92
 OR (SELECT checksum FROM public.schema_migrations WHERE version=92) IS DISTINCT FROM 'bf7db344af0752ac479b4422dfa0e770cd11623c3be2b51c4449dba89c93cc03' THEN
  RAISE EXCEPTION 'native V93 requires the reviewed V92 ledger boundary';
 END IF;
 IF pg_catalog.has_schema_privilege('espacigo_schema_owner','public','CREATE') THEN
  RAISE EXCEPTION 'native V93 refuses pre-existing schema CREATE authority';
 END IF;
 IF NOT EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname='espacigo_schema_owner'
 AND NOT rolcanlogin AND NOT rolsuper AND NOT rolinherit AND NOT rolcreaterole
 AND NOT rolcreatedb AND NOT rolreplication AND NOT rolbypassrls) THEN
  RAISE EXCEPTION 'native V93 schema-owner flags mismatch';
 END IF;
 IF NOT EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname='espacigo_runtime'
 AND NOT rolcanlogin AND NOT rolsuper AND NOT rolinherit AND NOT rolcreaterole
 AND NOT rolcreatedb AND NOT rolreplication AND NOT rolbypassrls) THEN
  RAISE EXCEPTION 'native V93 runtime role flags mismatch';
 END IF;
 IF pg_catalog.has_function_privilege('espacigo_runtime',
 'public.purge_expired_identity_declaration_revisions()','EXECUTE')
 OR NOT pg_catalog.has_function_privilege('espacigo_privacy_cleanup_worker',
 'public.purge_expired_identity_declaration_revisions()','EXECUTE') THEN
  RAISE EXCEPTION 'native V93 worker-only predecessor ACL mismatch';
 END IF;
END
$guard$;
GRANT CREATE ON SCHEMA public TO espacigo_schema_owner;
DO $verify$
BEGIN
 IF NOT pg_catalog.has_schema_privilege('espacigo_schema_owner','public','CREATE') THEN
  RAISE EXCEPTION 'native V93 temporary CREATE readback failed';
 END IF;
END
$verify$;
COMMIT;
