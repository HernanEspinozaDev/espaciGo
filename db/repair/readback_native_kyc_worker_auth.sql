-- Authentication and effective-role verification only: never invoke purgadores.
\set ON_ERROR_STOP on
\set ECHO none
BEGIN READ ONLY;
SET LOCAL statement_timeout='15s';
DO $guard$
BEGIN
 IF current_database()<>'espacigo_dms_stage_20261020'
 OR (SELECT oid FROM pg_catalog.pg_database WHERE datname=current_database())<>16388
 OR current_setting('server_version_num')<>'180006'
 OR inet_server_addr() IS DISTINCT FROM '127.0.0.1'::inet
 OR inet_server_port()<>5432
 OR session_user<>'espacigo_privacy_worker_login'
 OR current_user<>'espacigo_privacy_worker_login'
 OR current_setting('transaction_read_only')<>'on' THEN
  RAISE EXCEPTION 'native worker authentication target/identity mismatch';
 END IF;
 IF NOT EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname=session_user AND rolcanlogin AND NOT rolsuper AND NOT rolcreatedb AND NOT rolcreaterole AND NOT rolinherit AND NOT rolreplication AND NOT rolbypassrls AND rolconnlimit=2)
 OR NOT pg_catalog.pg_has_role(session_user,'espacigo_privacy_cleanup_worker','SET')
 OR pg_catalog.pg_has_role(session_user,'espacigo_privacy_cleanup_worker','USAGE')
 OR pg_catalog.has_function_privilege(session_user,'public.purge_expired_identity_declaration_revisions()','EXECUTE')
 OR pg_catalog.has_function_privilege(session_user,'public.purge_delivered_identity_session_outbox()','EXECUTE') THEN
  RAISE EXCEPTION 'native worker LOGIN privilege boundary mismatch';
 END IF;
END
$guard$;
SET LOCAL ROLE espacigo_privacy_cleanup_worker;
DO $verify$
BEGIN
 IF current_user<>'espacigo_privacy_cleanup_worker'
 OR NOT pg_catalog.has_function_privilege(current_user,'public.purge_expired_identity_declaration_revisions()','EXECUTE')
 OR NOT pg_catalog.has_function_privilege(current_user,'public.purge_delivered_identity_session_outbox()','EXECUTE')
 OR pg_catalog.has_table_privilege(current_user,'public.outbox_evento_local','DELETE')
 OR pg_catalog.has_schema_privilege(current_user,'public','CREATE')
 OR pg_catalog.has_function_privilege('espacigo_runtime','public.purge_expired_identity_declaration_revisions()','EXECUTE')
 OR pg_catalog.has_function_privilege('espacigo_runtime','public.purge_delivered_identity_session_outbox()','EXECUTE') THEN
  RAISE EXCEPTION 'native worker effective privilege boundary mismatch';
 END IF;
END
$verify$;
SELECT jsonb_build_object('database',current_database(),'session_user',session_user,'effective_user',current_user,'read_only',current_setting('transaction_read_only'),'login_scram_and_set_role','PASS','purge_invoked',false);
ROLLBACK;
BEGIN READ ONLY;
DO $reset$
BEGIN
 IF current_user<>session_user OR current_user<>'espacigo_privacy_worker_login' THEN
  RAISE EXCEPTION 'native worker LOCAL ROLE did not reset';
 END IF;
END
$reset$;
SELECT jsonb_build_object('role_reset','PASS','read_only',current_setting('transaction_read_only'));
ROLLBACK;
