-- Pair only with prepare_native_kyc_v89.sql: withdraw its temporary CREATE and REFERENCES.
BEGIN;
SELECT pg_catalog.pg_advisory_xact_lock(x'45535047'::int,x'4d494752'::int);
DO $guard$
DECLARE n bigint; v bigint;
BEGIN
 IF current_database()<>'espacigo_dms_stage_20261020'
 OR (SELECT oid FROM pg_catalog.pg_database WHERE datname=current_database())<>16388
 OR current_setting('server_version_num')<>'180006'
 OR session_user<>'postgres' OR current_user<>'postgres' THEN
  RAISE EXCEPTION 'native V89 finalizer target/administrator mismatch';
 END IF;
 SELECT count(*),max(version) INTO n,v FROM public.schema_migrations;
 IF n NOT IN (88,89) OR v<>n OR (SELECT min(version) FROM public.schema_migrations)<>1 THEN
  RAISE EXCEPTION 'native V89 finalizer requires ledger88 or ledger89';
 END IF;
 IF v=89 AND (SELECT checksum FROM public.schema_migrations WHERE version=89) IS DISTINCT FROM '93671b484f3118c697eebc4654f4b32325cce4231582b488f2e1a8d9b7fece9c' THEN
  RAISE EXCEPTION 'native V89 finalizer checksum mismatch';
 END IF;
END
$guard$;
REVOKE CREATE ON SCHEMA public FROM espacigo_schema_owner;
REVOKE REFERENCES(id) ON TABLE public.usuario FROM espacigo_schema_owner;
DO $verify$
BEGIN
 IF pg_catalog.has_schema_privilege('espacigo_schema_owner','public','CREATE')
 OR pg_catalog.has_column_privilege('espacigo_schema_owner','public.usuario','id','REFERENCES') THEN
  RAISE EXCEPTION 'native V89 temporary CREATE or REFERENCES remained after revocation';
 END IF;
END
$verify$;
COMMIT;
