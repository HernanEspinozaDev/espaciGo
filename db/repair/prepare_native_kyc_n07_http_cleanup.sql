-- Temporary minimum cleanup grants for the journaled N07 HTTP/BFF rehearsal.
-- DELETE is table-scoped; the test helper restricts rows by fixture IDs and journal.
BEGIN;
DO $guard$
DECLARE col text;
BEGIN
 IF current_database()<>'espacigo_dms_stage_20261020'
 OR (SELECT oid FROM pg_database WHERE datname=current_database())<>16388
 OR session_user<>'postgres' OR current_user<>'postgres'
 OR current_setting('server_version_num')<>'180006' THEN
  RAISE EXCEPTION 'native N07 cleanup target/administrator mismatch';
 END IF;
 IF (SELECT count(*) FROM public.schema_migrations) NOT IN (93,94)
 OR (SELECT min(version) FROM public.schema_migrations)<>1
 OR (SELECT max(version) FROM public.schema_migrations)<>(SELECT count(*) FROM public.schema_migrations)
 OR (SELECT checksum FROM public.schema_migrations WHERE version=93)
   IS DISTINCT FROM '885d3f926ffc7d3420f2391a257ca2ed68663e71d4cd54f4cf310c79bd8b7292'
 OR ((SELECT max(version) FROM public.schema_migrations)=94 AND ((SELECT name FROM public.schema_migrations WHERE version=94) IS DISTINCT FROM 'V000094__identity_delivered_session_outbox_retirement.sql' OR (SELECT checksum FROM public.schema_migrations WHERE version=94) IS DISTINCT FROM 'a4ee15a45ab352527e93c00e82763056b57b3750064483b8b90aa6891c0672a9')) THEN
  RAISE EXCEPTION 'native N07 cleanup requires exact reviewed ledger93 or ledger94';
 END IF;
 IF has_table_privilege('espacigo_schema_owner','public.evento_auditoria_local','DELETE')
 OR has_schema_privilege('espacigo_schema_owner','public','CREATE')
 OR ((SELECT max(version) FROM public.schema_migrations)=93 AND has_table_privilege('espacigo_schema_owner','public.outbox_evento_local','DELETE'))
 OR ((SELECT max(version) FROM public.schema_migrations)=94 AND (NOT has_table_privilege('espacigo_schema_owner','public.outbox_evento_local','DELETE') OR NOT has_column_privilege('espacigo_schema_owner','public.outbox_evento_local','id','UPDATE') OR NOT has_column_privilege('espacigo_schema_owner','public.outbox_evento_local','retirar_en','SELECT'))) THEN
  RAISE EXCEPTION 'refusing pre-existing N07 authority or mismatched permanent V94 outbox ACL';
 END IF;
 FOREACH col IN ARRAY ARRAY['actor_id','recurso_tipo','recurso_id','accion','correlacion_id'] LOOP
  IF has_column_privilege('espacigo_schema_owner','public.evento_auditoria_local',col,'SELECT') THEN
   RAISE EXCEPTION 'refusing pre-existing N07 audit-column SELECT authority';
  END IF;
 END LOOP;
END
$guard$;
GRANT SELECT (actor_id,recurso_tipo,recurso_id,accion,correlacion_id), DELETE
 ON TABLE public.evento_auditoria_local TO espacigo_schema_owner;
GRANT DELETE ON TABLE public.outbox_evento_local TO espacigo_schema_owner;
DO $check$
DECLARE col text;
BEGIN
 IF NOT has_table_privilege('espacigo_schema_owner','public.evento_auditoria_local','DELETE')
 OR NOT has_table_privilege('espacigo_schema_owner','public.outbox_evento_local','DELETE')
 OR has_schema_privilege('espacigo_schema_owner','public','CREATE') THEN
  RAISE EXCEPTION 'N07 cleanup grants do not match the minimum contract';
 END IF;
 FOREACH col IN ARRAY ARRAY['actor_id','recurso_tipo','recurso_id','accion','correlacion_id'] LOOP
  IF NOT has_column_privilege('espacigo_schema_owner','public.evento_auditoria_local',col,'SELECT') THEN
   RAISE EXCEPTION 'N07 cleanup audit SELECT grant is incomplete';
  END IF;
 END LOOP;
END
$check$;
COMMIT;
