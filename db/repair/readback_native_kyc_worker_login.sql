-- Read-only catalog receipt; no credential values or application rows.
\set ON_ERROR_STOP on
\set ECHO none
BEGIN READ ONLY;
SET LOCAL statement_timeout='15s';
DO $guard$
BEGIN
 IF current_database()<>'espacigo_dms_stage_20261020'
 OR (SELECT oid FROM pg_catalog.pg_database WHERE datname=current_database())<>16388
 OR current_setting('server_version_num')<>'180006'
 OR session_user<>'postgres' OR current_user<>'postgres'
 OR inet_server_addr() IS NOT NULL THEN
  RAISE EXCEPTION 'native worker readback target/session mismatch';
 END IF;
END
$guard$;
SELECT jsonb_build_object(
 'database',current_database(),
 'oid',(SELECT oid FROM pg_catalog.pg_database WHERE datname=current_database()),
 'engine',current_setting('server_version_num'),
 'session_user',session_user,'effective_user',current_user,
 'ledger',(SELECT jsonb_agg(jsonb_build_object('version',version,'name',name,'checksum',checksum) ORDER BY version) FROM public.schema_migrations),
 'databases',(SELECT jsonb_agg(jsonb_build_object('oid',oid,'name',datname,'owner',datdba,'acl',datacl::text,'allow_connect',datallowconn) ORDER BY oid) FROM pg_catalog.pg_database),
 'roles',(SELECT jsonb_agg(jsonb_build_object('name',rolname,'oid',oid,'login',rolcanlogin,'super',rolsuper,'createdb',rolcreatedb,'createrole',rolcreaterole,'inherit',rolinherit,'replication',rolreplication,'bypassrls',rolbypassrls) ORDER BY oid) FROM pg_catalog.pg_roles),
 'memberships',(SELECT jsonb_agg(to_jsonb(m) ORDER BY roleid,member) FROM pg_catalog.pg_auth_members m),
 'worker_dependencies',(SELECT jsonb_agg(to_jsonb(d) ORDER BY dbid,classid,objid) FROM pg_catalog.pg_shdepend d WHERE refclassid='pg_authid'::regclass AND refobjid=(SELECT oid FROM pg_catalog.pg_roles WHERE rolname='espacigo_privacy_cleanup_worker')),
 'relation_acl',(SELECT md5(string_agg(format('%s %s %s %s',oid,relowner,relacl,relname),E'\n' ORDER BY oid)) FROM pg_catalog.pg_class),
 'column_acl',(SELECT md5(string_agg(format('%s %s %s',attrelid,attnum,attacl),E'\n' ORDER BY attrelid,attnum)) FROM pg_catalog.pg_attribute WHERE attnum>0),
 'function_acl',(SELECT md5(string_agg(format('%s %s %s',oid,proowner,proacl),E'\n' ORDER BY oid)) FROM pg_catalog.pg_proc),
 'schema_acl',(SELECT md5(string_agg(format('%s %s %s',oid,nspowner,nspacl),E'\n' ORDER BY oid)) FROM pg_catalog.pg_namespace),
 'default_acl',(SELECT md5(string_agg(to_jsonb(d)::text,E'\n' ORDER BY oid)) FROM pg_catalog.pg_default_acl d)
);
ROLLBACK;
