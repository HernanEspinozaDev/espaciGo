-- Pair only with prepare_native_kyc_v88.sql: withdraw its temporary CREATE.
BEGIN;
SELECT pg_catalog.pg_advisory_xact_lock(x'45535047'::int,x'4d494752'::int);
DO $guard$
DECLARE n bigint; v bigint;
BEGIN
 IF current_database()<>'espacigo_dms_stage_20261020'
 OR (SELECT oid FROM pg_catalog.pg_database WHERE datname=current_database())<>16388
 OR current_setting('server_version_num')<>'180006'
 OR session_user<>'postgres' OR current_user<>'postgres' THEN
  RAISE EXCEPTION 'native V88 finalizer target/administrator mismatch';
 END IF;
 SELECT count(*),max(version) INTO n,v FROM public.schema_migrations;
 IF n NOT IN (87,88) OR v<>n OR (SELECT min(version) FROM public.schema_migrations)<>1 THEN
  RAISE EXCEPTION 'native V88 finalizer requires ledger87 or ledger88';
 END IF;
 IF v=88 AND (SELECT checksum FROM public.schema_migrations WHERE version=88) IS DISTINCT FROM '44cc5e74b8f8fcfba1633f01009024a7fdeae5683077b13677ed7bf2894083a8' THEN
  RAISE EXCEPTION 'native V88 finalizer checksum mismatch';
 END IF;
END
$guard$;
REVOKE CREATE ON SCHEMA public FROM espacigo_schema_owner;
DO $verify$
BEGIN
 IF pg_catalog.has_schema_privilege('espacigo_schema_owner','public','CREATE') THEN
  RAISE EXCEPTION 'native V88 temporary CREATE remained after revocation';
 END IF;
END
$verify$;
COMMIT;
