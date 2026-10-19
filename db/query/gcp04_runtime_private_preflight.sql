-- Read-only GCP-04 runtime preflight for the exact existing database.
-- Run through the reviewed private Cloud Run Job path; never emit query text,
-- connection addresses, secret values, credential hashes, or row payloads.
\set ON_ERROR_STOP on
BEGIN READ ONLY;

SELECT current_database() AS database_name,
       current_setting('server_version') AS server_version,
       current_user AS observer_role,
       current_setting('transaction_read_only') AS transaction_read_only;

SELECT current_setting('max_connections')::integer AS max_connections,
       current_setting('reserved_connections')::integer AS reserved_connections,
       current_setting('superuser_reserved_connections')::integer AS superuser_reserved_connections,
       count(*) FILTER (WHERE backend_type = 'client backend')::integer AS client_backends,
       greatest(
           current_setting('max_connections')::integer
           - current_setting('reserved_connections')::integer
           - current_setting('superuser_reserved_connections')::integer
           - count(*) FILTER (WHERE backend_type = 'client backend'),
           0
       )::integer AS currently_unreserved_slots
FROM pg_catalog.pg_stat_activity;

SELECT datname AS database_name,
       pg_catalog.pg_get_userbyid(datdba) AS database_owner,
       has_database_privilege('espacigo_runtime', oid, 'CONNECT') AS runtime_connect,
       has_database_privilege('espacigo_runtime', oid, 'CREATE') AS runtime_database_create,
       has_database_privilege('espacigo_runtime', oid, 'TEMP') AS runtime_temp,
       has_database_privilege('espacigo_schema_owner', oid, 'TEMP') AS schema_owner_temp,
       has_database_privilege('espacigo_privacy_cleanup_worker', oid, 'CONNECT') AS worker_connect
FROM pg_catalog.pg_database
WHERE datname = current_database();

SELECT n.nspname AS schema_name,
       pg_catalog.pg_get_userbyid(n.nspowner) AS schema_owner,
       has_schema_privilege('espacigo_runtime', n.oid, 'USAGE') AS runtime_usage,
       has_schema_privilege('espacigo_runtime', n.oid, 'CREATE') AS runtime_create,
       has_schema_privilege('espacigo_privacy_cleanup_worker', n.oid, 'USAGE') AS worker_usage
FROM pg_catalog.pg_namespace n
WHERE n.nspname = 'public';

SELECT c.relkind AS object_kind,
       pg_catalog.pg_get_userbyid(c.relowner) AS object_owner,
       count(*)::integer AS object_count
FROM pg_catalog.pg_class c
JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
WHERE n.nspname = 'public'
GROUP BY c.relkind, c.relowner
ORDER BY c.relkind, object_owner;

SELECT rolname,
       rolcanlogin,
       rolsuper,
       rolcreatedb,
       rolcreaterole,
       rolreplication,
       rolbypassrls,
       rolinherit
FROM pg_catalog.pg_roles
WHERE rolname IN (
    'espacigo_schema_owner', 'espacigo_runtime', 'espacigo_privacy_cleanup_worker', 'espacigo_db_observer', 'espacigo_gadm_bootstrap',
    'espacigo_api_login', 'espacigo_worker_login', 'espacigo_migrator_login', 'espacigo_observer_login'
)
ORDER BY rolname;

SELECT granted_role.rolname AS granted_role,
       member_role.rolname AS member_role,
       m.admin_option,
       m.inherit_option,
       m.set_option
FROM pg_catalog.pg_auth_members m
JOIN pg_catalog.pg_roles granted_role ON granted_role.oid = m.roleid
JOIN pg_catalog.pg_roles member_role ON member_role.oid = m.member
WHERE granted_role.rolname IN ('espacigo_schema_owner', 'espacigo_runtime', 'espacigo_privacy_cleanup_worker', 'espacigo_db_observer')
   OR member_role.rolname IN ('espacigo_schema_owner', 'espacigo_runtime', 'espacigo_privacy_cleanup_worker', 'espacigo_db_observer')
ORDER BY granted_role.rolname, member_role.rolname;

SELECT extname, extversion
FROM pg_catalog.pg_extension
ORDER BY extname;

SELECT count(*)::integer AS migration_count,
       min(version) AS first_version,
       max(version) AS latest_version
FROM public.schema_migrations;

COMMIT;
