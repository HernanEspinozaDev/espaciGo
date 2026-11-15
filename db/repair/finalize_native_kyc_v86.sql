-- Revoke only the temporary V86 owner CREATE granted by its paired preparation.
-- Safe after a rejected V86 (ledger85) or committed V86 (ledger86); no replay.
BEGIN;
SELECT pg_catalog.pg_advisory_xact_lock(x'45535047'::int,x'4d494752'::int);
DO $guard$
DECLARE n bigint; v bigint;
BEGIN
 IF current_database()<>'espacigo_dms_stage_20261020'
 OR (SELECT oid FROM pg_catalog.pg_database WHERE datname=current_database())<>16388
 OR current_setting('server_version_num')<>'180006'
 OR session_user<>'postgres' OR current_user<>'postgres' THEN
  RAISE EXCEPTION 'native V86 finalizer target/administrator mismatch';
 END IF;
 SELECT count(*),max(version) INTO n,v FROM public.schema_migrations;
 IF n NOT IN (85,86) OR v<>n OR (SELECT min(version) FROM public.schema_migrations)<>1 THEN
  RAISE EXCEPTION 'native V86 finalizer requires ledger85 or ledger86';
 END IF;
 IF v=86 AND (SELECT checksum FROM public.schema_migrations WHERE version=86)<>'af864b793d8f4cbc9cf28dbfe65b8ed6d2388a0ade9a739fc8b480470bd3dc8e' THEN
  RAISE EXCEPTION 'native V86 finalizer checksum mismatch';
 END IF;
END
$guard$;
REVOKE CREATE ON SCHEMA public FROM espacigo_schema_owner;
DO $verify$
BEGIN
 IF pg_catalog.has_schema_privilege('espacigo_schema_owner','public','CREATE') THEN
  RAISE EXCEPTION 'native V86 temporary CREATE remained after revocation';
 END IF;
END
$verify$;
COMMIT;
