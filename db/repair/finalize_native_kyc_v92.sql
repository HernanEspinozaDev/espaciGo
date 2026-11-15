-- Revoke the exact native V92 CREATE window after rehearsal or application.
BEGIN;
SELECT pg_catalog.pg_advisory_xact_lock(x'45535047'::int,x'4d494752'::int);
DO $guard$
DECLARE n bigint; v bigint;
BEGIN
 IF current_database()<>'espacigo_dms_stage_20261020'
 OR (SELECT oid FROM pg_catalog.pg_database WHERE datname=current_database())<>16388
 OR current_setting('server_version_num')<>'180006'
 OR session_user<>'postgres' OR current_user<>'postgres' THEN
  RAISE EXCEPTION 'native V92 finalizer target/administrator mismatch';
 END IF;
 SELECT count(*),max(version) INTO n,v FROM public.schema_migrations;
 IF n NOT IN (91,92) OR v<>n OR (SELECT min(version) FROM public.schema_migrations)<>1
 OR (SELECT checksum FROM public.schema_migrations WHERE version=91) IS DISTINCT FROM '6ceee080736bd965a27226e3810d28ede0ecf2deb52477716a504e2b425c1250' THEN
  RAISE EXCEPTION 'native V92 finalizer requires contiguous ledger91 or ledger92';
 END IF;
 IF v=92 AND (SELECT checksum FROM public.schema_migrations WHERE version=92) IS DISTINCT FROM 'bf7db344af0752ac479b4422dfa0e770cd11623c3be2b51c4449dba89c93cc03' THEN
  RAISE EXCEPTION 'native V92 finalizer checksum mismatch';
 END IF;
END
$guard$;
REVOKE CREATE ON SCHEMA public FROM espacigo_schema_owner;
DO $verify$
BEGIN
 IF pg_catalog.has_schema_privilege('espacigo_schema_owner','public','CREATE') THEN
  RAISE EXCEPTION 'native V92 temporary CREATE authority remained';
 END IF;
END
$verify$;
COMMIT;
