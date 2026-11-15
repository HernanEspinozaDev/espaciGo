-- Authorized native development target only. Temporary authority for exact V86.
-- Execute only after the reviewed backup/source/ledger gates; pair with finalizer.
BEGIN;
SELECT pg_catalog.pg_advisory_xact_lock(x'45535047'::int,x'4d494752'::int);
DO $guard$
BEGIN
 IF current_database()<>'espacigo_dms_stage_20261020'
 OR (SELECT oid FROM pg_catalog.pg_database WHERE datname=current_database())<>16388
 OR current_setting('server_version_num')<>'180006'
 OR session_user<>'postgres' OR current_user<>'postgres' THEN
  RAISE EXCEPTION 'native V86 preparation target/administrator mismatch';
 END IF;
 IF (SELECT count(*) FROM public.schema_migrations)<>85
 OR (SELECT min(version) FROM public.schema_migrations)<>1
 OR (SELECT max(version) FROM public.schema_migrations)<>85
 OR (SELECT checksum FROM public.schema_migrations WHERE version=85)<>'7b78094558e2d9bde7804919b01c54d34caf2ac31ac34fb1b86a63999aeeaf51' THEN
  RAISE EXCEPTION 'native V86 preparation requires the reviewed V85 ledger boundary';
 END IF;
 IF pg_catalog.to_regprocedure('public.purge_expired_identity_declaration_revisions()') IS NOT NULL THEN
  RAISE EXCEPTION 'native V86 refuses an existing unledgered purge function';
 END IF;
 IF pg_catalog.has_schema_privilege('espacigo_schema_owner','public','CREATE') THEN
  RAISE EXCEPTION 'native V86 refuses pre-existing CREATE authority';
 END IF;
 IF NOT EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname='espacigo_schema_owner'
 AND NOT rolcanlogin AND NOT rolsuper AND NOT rolinherit AND NOT rolcreaterole
 AND NOT rolcreatedb AND NOT rolreplication AND NOT rolbypassrls) THEN
  RAISE EXCEPTION 'native V86 schema-owner flags mismatch';
 END IF;
END
$guard$;
GRANT CREATE ON SCHEMA public TO espacigo_schema_owner;
DO $verify$
BEGIN
 IF NOT pg_catalog.has_schema_privilege('espacigo_schema_owner','public','CREATE') THEN
  RAISE EXCEPTION 'native V86 temporary authority readback failed';
 END IF;
END
$verify$;
COMMIT;
