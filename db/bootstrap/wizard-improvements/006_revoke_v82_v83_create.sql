-- Idempotent fail-safe cleanup: revoke first; ledger inspection never gates revoke.
BEGIN;
SELECT pg_advisory_xact_lock(1163087943, 1296648018);
DO $target$
BEGIN
 IF current_database()<>'espacigo_wizard_20261106' OR session_user<>'postgres'
    OR current_user<>'postgres' OR current_setting('server_version_num')<>'180006' THEN
   RAISE EXCEPTION 'wrong V82/V83 cleanup target or administrator';
 END IF;
END $target$;
DO $roles$
BEGIN
 IF NOT EXISTS(SELECT 1 FROM pg_roles WHERE rolname='espacigo_schema_owner' AND NOT rolcanlogin AND NOT rolsuper AND NOT rolcreatedb AND NOT rolcreaterole AND NOT rolreplication AND NOT rolbypassrls AND NOT rolinherit)
 OR NOT EXISTS(SELECT 1 FROM pg_roles WHERE rolname='espacigo_api_login' AND rolcanlogin AND NOT rolsuper AND NOT rolcreatedb AND NOT rolcreaterole AND NOT rolreplication AND NOT rolbypassrls AND NOT rolinherit)
 OR NOT EXISTS(SELECT 1 FROM pg_roles WHERE rolname='espacigo_runtime' AND NOT rolcanlogin AND NOT rolsuper AND NOT rolcreatedb AND NOT rolcreaterole AND NOT rolreplication AND NOT rolbypassrls) THEN RAISE EXCEPTION 'restricted role identity mismatch; refusing cleanup'; END IF;
END $roles$;
REVOKE CREATE ON SCHEMA public FROM espacigo_schema_owner;
DO $verify$
BEGIN
 IF has_schema_privilege('espacigo_schema_owner','public','CREATE') OR has_schema_privilege('espacigo_api_login','public','CREATE') OR has_schema_privilege('espacigo_runtime','public','CREATE') THEN RAISE EXCEPTION 'CREATE privilege remains after cleanup'; END IF;
END $verify$;
COMMIT;
