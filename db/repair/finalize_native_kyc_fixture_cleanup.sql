-- Idempotently close the exact temporary native KYC fixture cleanup grants.
BEGIN;
DO $guard$
BEGIN
 IF current_database()<>'espacigo_dms_stage_20261020' OR (SELECT oid FROM pg_database WHERE datname=current_database())<>16388
 OR session_user<>'postgres' OR current_user<>'postgres' OR current_setting('server_version_num')<>'180006' THEN
  RAISE EXCEPTION 'native fixture cleanup finalizer target/administrator mismatch';
 END IF;
END
$guard$;
REVOKE SELECT (actor_id,recurso_tipo,recurso_id,accion,correlacion_id), DELETE
 ON TABLE public.evento_auditoria_local FROM espacigo_schema_owner;
REVOKE DELETE ON TABLE public.outbox_evento_local FROM espacigo_schema_owner;
REVOKE CREATE ON SCHEMA public FROM espacigo_schema_owner;
DO $check$
DECLARE col text;
BEGIN
 IF has_table_privilege('espacigo_schema_owner','public.evento_auditoria_local','DELETE') THEN
  RAISE EXCEPTION 'native fixture cleanup DELETE authority still effective';
 END IF;
 IF has_schema_privilege('espacigo_schema_owner','public','CREATE') THEN
  RAISE EXCEPTION 'native fixture cleanup CREATE authority still effective';
 END IF;
 IF has_table_privilege('espacigo_schema_owner','public.outbox_evento_local','DELETE') THEN
  RAISE EXCEPTION 'native fixture cleanup outbox authority still effective';
 END IF;
 FOREACH col IN ARRAY ARRAY['actor_id','recurso_tipo','recurso_id','accion','correlacion_id'] LOOP
  IF has_column_privilege('espacigo_schema_owner','public.evento_auditoria_local',col,'SELECT') THEN
   RAISE EXCEPTION 'native fixture cleanup SELECT authority still effective';
  END IF;
 END LOOP;
END
$check$;
COMMIT;
