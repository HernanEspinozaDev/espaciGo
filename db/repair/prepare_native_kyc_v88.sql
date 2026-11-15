-- Native KYC V88 only: reviewed temporary schema CREATE for the schema owner.
BEGIN;
SELECT pg_catalog.pg_advisory_xact_lock(x'45535047'::int,x'4d494752'::int);
DO $guard$
BEGIN
 IF current_database()<>'espacigo_dms_stage_20261020'
 OR (SELECT oid FROM pg_catalog.pg_database WHERE datname=current_database())<>16388
 OR current_setting('server_version_num')<>'180006'
 OR session_user<>'postgres' OR current_user<>'postgres' THEN
  RAISE EXCEPTION 'native V88 preparation target/administrator mismatch';
 END IF;
 IF (SELECT count(*) FROM public.schema_migrations)<>87
 OR (SELECT min(version) FROM public.schema_migrations)<>1
 OR (SELECT max(version) FROM public.schema_migrations)<>87
 OR (SELECT checksum FROM public.schema_migrations WHERE version=87) IS DISTINCT FROM '5eda043ddf3a3d5bb89e8d470eaf4e5450ed0964a9d59fe232bdcb37888b1d45' THEN
  RAISE EXCEPTION 'native V88 requires the reviewed V87 ledger boundary';
 END IF;
 IF NOT EXISTS (SELECT 1 FROM pg_catalog.pg_proc WHERE oid=pg_catalog.to_regprocedure('public.purge_expired_identity_declaration_revisions()') AND pg_catalog.pg_get_userbyid(proowner)='espacigo_schema_owner' AND prosecdef) THEN
  RAISE EXCEPTION 'native V88 purge function owner/shape mismatch';
 END IF;
 IF pg_catalog.has_schema_privilege('espacigo_schema_owner','public','CREATE') THEN
  RAISE EXCEPTION 'native V88 refuses pre-existing CREATE authority';
 END IF;
 IF NOT EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname='espacigo_schema_owner'
 AND NOT rolcanlogin AND NOT rolsuper AND NOT rolinherit AND NOT rolcreaterole
 AND NOT rolcreatedb AND NOT rolreplication AND NOT rolbypassrls) THEN
  RAISE EXCEPTION 'native V88 schema-owner flags mismatch';
 END IF;
END
$guard$;
GRANT CREATE ON SCHEMA public TO espacigo_schema_owner;
DO $verify$
BEGIN
 IF NOT pg_catalog.has_schema_privilege('espacigo_schema_owner','public','CREATE') THEN
  RAISE EXCEPTION 'native V88 temporary authority readback failed';
 END IF;
END
$verify$;
COMMIT;
