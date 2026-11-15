-- Close and verify the exact temporary grants used by the N07 HTTP/BFF rehearsal.
BEGIN;
DO $guard$
BEGIN
 IF current_database()<>'espacigo_dms_stage_20261020'
 OR (SELECT oid FROM pg_database WHERE datname=current_database())<>16388
 OR session_user<>'postgres' OR current_user<>'postgres'
 OR current_setting('server_version_num')<>'180006' THEN
  RAISE EXCEPTION 'native N07 cleanup finalizer target/administrator mismatch';
 END IF;
END
$guard$;
REVOKE SELECT (actor_id,recurso_tipo,recurso_id,accion,correlacion_id), DELETE
 ON TABLE public.evento_auditoria_local FROM espacigo_schema_owner;
DO $outbox$
BEGIN
 IF (SELECT max(version) FROM public.schema_migrations)=93 THEN
  EXECUTE 'REVOKE DELETE ON TABLE public.outbox_evento_local FROM espacigo_schema_owner';
 END IF;
END
$outbox$;
DO $check$
DECLARE col text;
BEGIN
 IF (SELECT count(*) FROM public.schema_migrations) NOT IN (93,94)
 OR (SELECT min(version) FROM public.schema_migrations)<>1
 OR (SELECT max(version) FROM public.schema_migrations)<>(SELECT count(*) FROM public.schema_migrations)
 OR (SELECT checksum FROM public.schema_migrations WHERE version=93) IS DISTINCT FROM '885d3f926ffc7d3420f2391a257ca2ed68663e71d4cd54f4cf310c79bd8b7292'
 OR ((SELECT max(version) FROM public.schema_migrations)=94 AND ((SELECT name FROM public.schema_migrations WHERE version=94) IS DISTINCT FROM 'V000094__identity_delivered_session_outbox_retirement.sql' OR (SELECT checksum FROM public.schema_migrations WHERE version=94) IS DISTINCT FROM 'a4ee15a45ab352527e93c00e82763056b57b3750064483b8b90aa6891c0672a9')) THEN
  RAISE EXCEPTION 'native N07 cleanup requires exact reviewed ledger93 or ledger94';
 END IF;
 IF has_table_privilege('espacigo_schema_owner','public.evento_auditoria_local','DELETE')
 OR has_schema_privilege('espacigo_schema_owner','public','CREATE')
 OR ((SELECT max(version) FROM public.schema_migrations)=93 AND has_table_privilege('espacigo_schema_owner','public.outbox_evento_local','DELETE'))
 OR ((SELECT max(version) FROM public.schema_migrations)=94 AND (NOT has_table_privilege('espacigo_schema_owner','public.outbox_evento_local','DELETE') OR NOT has_column_privilege('espacigo_schema_owner','public.outbox_evento_local','id','UPDATE') OR NOT has_column_privilege('espacigo_schema_owner','public.outbox_evento_local','retirar_en','SELECT'))) THEN
  RAISE EXCEPTION 'N07 temporary authority remains or permanent V94 outbox ACL was lost';
 END IF;
 FOREACH col IN ARRAY ARRAY['actor_id','recurso_tipo','recurso_id','accion','correlacion_id'] LOOP
  IF has_column_privilege('espacigo_schema_owner','public.evento_auditoria_local',col,'SELECT') THEN
   RAISE EXCEPTION 'N07 cleanup audit SELECT authority remains effective';
  END IF;
 END LOOP;
END
$check$;
COMMIT;
