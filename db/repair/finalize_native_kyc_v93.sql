-- Revoke the exact native V93 CREATE window after rehearsal or application.
BEGIN;
SELECT pg_catalog.pg_advisory_xact_lock(x'45535047'::int,x'4d494752'::int);
DO $guard$
DECLARE n bigint; v bigint;
BEGIN
 IF current_database()<>'espacigo_dms_stage_20261020'
 OR (SELECT oid FROM pg_catalog.pg_database WHERE datname=current_database())<>16388
 OR current_setting('server_version_num')<>'180006'
 OR session_user<>'postgres' OR current_user<>'postgres' THEN
  RAISE EXCEPTION 'native V93 finalizer target/administrator mismatch';
 END IF;
 SELECT count(*),max(version) INTO n,v FROM public.schema_migrations;
 IF n NOT IN (92,93) OR v<>n OR (SELECT min(version) FROM public.schema_migrations)<>1
 OR (SELECT checksum FROM public.schema_migrations WHERE version=92) IS DISTINCT FROM 'bf7db344af0752ac479b4422dfa0e770cd11623c3be2b51c4449dba89c93cc03' THEN
  RAISE EXCEPTION 'native V93 finalizer requires contiguous ledger92 or ledger93';
 END IF;
 IF v=93 AND (SELECT checksum FROM public.schema_migrations WHERE version=93) IS DISTINCT FROM '885d3f926ffc7d3420f2391a257ca2ed68663e71d4cd54f4cf310c79bd8b7292' THEN
  RAISE EXCEPTION 'native V93 finalizer checksum mismatch';
 END IF;
END
$guard$;
REVOKE CREATE ON SCHEMA public FROM espacigo_schema_owner;
DO $verify$
BEGIN
 IF pg_catalog.has_schema_privilege('espacigo_schema_owner','public','CREATE') THEN
  RAISE EXCEPTION 'native V93 temporary CREATE authority remained';
 END IF;
END
$verify$;
COMMIT;
