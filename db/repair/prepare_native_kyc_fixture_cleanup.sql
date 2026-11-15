-- Native synthetic KYC fixture cleanup only; pair with finalizer and no workers.
-- SELECT is column-scoped; DELETE is temporary, used only by the migrator's
-- reviewed account/declaration/correlation-scoped cleanup, never API runtime.
BEGIN;
DO $guard$
DECLARE col text;
BEGIN
 IF current_database()<>'espacigo_dms_stage_20261020' OR (SELECT oid FROM pg_database WHERE datname=current_database())<>16388
 OR session_user<>'postgres' OR current_user<>'postgres' OR current_setting('server_version_num')<>'180006' THEN
  RAISE EXCEPTION 'native fixture cleanup target/administrator mismatch';
 END IF;
 IF (SELECT count(*) FROM public.schema_migrations)<>90 OR (SELECT min(version) FROM public.schema_migrations)<>1
 OR (SELECT max(version) FROM public.schema_migrations)<>90
 OR (SELECT checksum FROM public.schema_migrations WHERE version=90) IS DISTINCT FROM '23bbacde2cf992f912926f84945c701422b097598e698b2e8f38158b61622592' THEN
  RAISE EXCEPTION 'native fixture cleanup requires the reviewed ledger90';
 END IF;
 IF has_table_privilege('espacigo_schema_owner','public.evento_auditoria_local','DELETE')
 OR has_schema_privilege('espacigo_schema_owner','public','CREATE')
 OR has_table_privilege('espacigo_schema_owner','public.outbox_evento_local','DELETE') THEN
  RAISE EXCEPTION 'refusing pre-existing temporary cleanup authority';
 END IF;
 FOREACH col IN ARRAY ARRAY['actor_id','recurso_tipo','recurso_id','accion','correlacion_id'] LOOP
  IF has_column_privilege('espacigo_schema_owner','public.evento_auditoria_local',col,'SELECT') THEN
   RAISE EXCEPTION 'refusing pre-existing audit-column SELECT authority';
  END IF;
 END LOOP;
END
$guard$;
GRANT SELECT (actor_id,recurso_tipo,recurso_id,accion,correlacion_id), DELETE
 ON TABLE public.evento_auditoria_local TO espacigo_schema_owner;
GRANT DELETE ON TABLE public.outbox_evento_local TO espacigo_schema_owner;
GRANT CREATE ON SCHEMA public TO espacigo_schema_owner;
COMMIT;
