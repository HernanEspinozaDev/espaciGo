-- Close the temporary CREATE window used only for the native V90 tail.
BEGIN;
SELECT pg_catalog.pg_advisory_xact_lock(x'45535047'::int,x'4d494752'::int);
DO $guard$
DECLARE n bigint; v bigint;
BEGIN
 IF current_database()<>'espacigo_dms_stage_20261020'
 OR (SELECT oid FROM pg_catalog.pg_database WHERE datname=current_database())<>16388
 OR current_setting('server_version_num')<>'180006'
 OR session_user<>'postgres' OR current_user<>'postgres' THEN
  RAISE EXCEPTION 'native V90 finalizer target/administrator mismatch';
 END IF;
 SELECT count(*),max(version) INTO n,v FROM public.schema_migrations;
 IF n NOT IN (89,90) OR v<>n OR (SELECT min(version) FROM public.schema_migrations)<>1
 OR (SELECT checksum FROM public.schema_migrations WHERE version=89) IS DISTINCT FROM '93671b484f3118c697eebc4654f4b32325cce4231582b488f2e1a8d9b7fece9c' THEN
  RAISE EXCEPTION 'native V90 finalizer requires the contiguous ledger89 or ledger90 boundary';
 END IF;
 IF v=90 AND (SELECT checksum FROM public.schema_migrations WHERE version=90) IS DISTINCT FROM '23bbacde2cf992f912926f84945c701422b097598e698b2e8f38158b61622592' THEN
  RAISE EXCEPTION 'native V90 finalizer checksum mismatch';
 END IF;
END
$guard$;
REVOKE CREATE ON SCHEMA public FROM espacigo_schema_owner;
DO $verify$
BEGIN
 IF pg_catalog.has_schema_privilege('espacigo_schema_owner','public','CREATE') THEN
  RAISE EXCEPTION 'native V90 temporary CREATE authority remained after revocation';
 END IF;
END
$verify$;
COMMIT;
