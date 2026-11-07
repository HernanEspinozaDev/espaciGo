-- One-time preparation for the isolated wizard-improvements trial only.
-- Execute only through cmd/wizardtrialsetup after exact identity/emptiness guards.
BEGIN;
DO $guard$
BEGIN
  IF current_database() <> 'espacigo_wizard_20261106'
     OR current_user <> 'postgres'
     OR session_user <> 'postgres'
     OR current_setting('server_version_num')::int / 10000 <> 18 THEN
    RAISE EXCEPTION 'wizard trial bootstrap target identity mismatch';
  END IF;
  IF EXISTS (SELECT 1 FROM pg_catalog.pg_namespace WHERE nspname = 'public'
             AND nspowner <> (SELECT oid FROM pg_catalog.pg_roles WHERE rolname = 'pg_database_owner')) THEN
    RAISE EXCEPTION 'unexpected public schema owner';
  END IF;
  IF EXISTS (SELECT 1 FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
             WHERE n.nspname='public' AND c.relkind IN ('r','p','v','m','f') AND NOT EXISTS (
               SELECT 1 FROM pg_catalog.pg_depend d JOIN pg_catalog.pg_extension e ON e.oid=d.refobjid
               WHERE d.classid='pg_class'::regclass AND d.objid=c.oid
                 AND d.refclassid='pg_extension'::regclass AND d.deptype='e')) THEN
    RAISE EXCEPTION 'application objects already exist';
  END IF;
  IF EXISTS (SELECT 1 FROM pg_catalog.pg_type t JOIN pg_catalog.pg_namespace n ON n.oid=t.typnamespace
             WHERE n.nspname='public' AND t.typtype <> 'p'
               AND NOT EXISTS (SELECT 1 FROM pg_catalog.pg_depend d
                 WHERE d.classid='pg_type'::regclass AND d.objid=t.oid
                   AND d.refclassid='pg_extension'::regclass AND d.deptype='e')
               AND NOT (t.typtype='c' AND EXISTS (SELECT 1 FROM pg_catalog.pg_depend d
                 WHERE d.classid='pg_class'::regclass AND d.objid=t.typrelid
                   AND d.refclassid='pg_extension'::regclass AND d.deptype='e'))
               AND NOT (t.typrelid=0 AND t.typtype='b' AND t.typelem<>0 AND EXISTS (SELECT 1 FROM pg_catalog.pg_type base
                 WHERE base.oid=t.typelem AND base.typelem=0 AND EXISTS (SELECT 1 FROM pg_catalog.pg_depend d
                   WHERE d.classid='pg_type'::regclass AND d.objid=base.oid
                     AND d.refclassid='pg_extension'::regclass AND d.deptype='e')))
               ) THEN
    RAISE EXCEPTION 'application types already exist';
  END IF;
  IF to_regclass('public.schema_migrations') IS NOT NULL THEN
    RAISE EXCEPTION 'migration ledger already exists';
  END IF;
END $guard$;
DO $roles$
DECLARE r record;
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname='espacigo_schema_owner') THEN
    CREATE ROLE espacigo_schema_owner NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT;
  ELSIF EXISTS (SELECT 1 FROM pg_roles WHERE rolname='espacigo_schema_owner' AND (rolcanlogin OR rolsuper OR rolcreatedb OR rolcreaterole OR rolreplication OR rolbypassrls OR rolinherit)) THEN RAISE EXCEPTION 'schema owner attributes mismatch'; END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname='espacigo_runtime') THEN
    CREATE ROLE espacigo_runtime NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT;
  ELSIF EXISTS (SELECT 1 FROM pg_roles WHERE rolname='espacigo_runtime' AND (rolcanlogin OR rolsuper OR rolcreatedb OR rolcreaterole OR rolreplication OR rolbypassrls OR rolinherit)) THEN RAISE EXCEPTION 'runtime attributes mismatch'; END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname='espacigo_gadm_bootstrap') THEN
    CREATE ROLE espacigo_gadm_bootstrap NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname='espacigo_privacy_cleanup_worker') THEN
    CREATE ROLE espacigo_privacy_cleanup_worker NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname='espacigo_gcp06_test_access') THEN
    CREATE ROLE espacigo_gcp06_test_access NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname='espacigo_migrator_login') THEN
    CREATE ROLE espacigo_migrator_login LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT;
  ELSIF EXISTS (SELECT 1 FROM pg_roles WHERE rolname='espacigo_migrator_login' AND (NOT rolcanlogin OR rolsuper OR rolcreatedb OR rolcreaterole OR rolreplication OR rolbypassrls OR rolinherit)) THEN RAISE EXCEPTION 'migrator attributes mismatch'; END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname='espacigo_api_login') THEN
    CREATE ROLE espacigo_api_login LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT;
  ELSIF EXISTS (SELECT 1 FROM pg_roles WHERE rolname='espacigo_api_login' AND (NOT rolcanlogin OR rolsuper OR rolcreatedb OR rolcreaterole OR rolreplication OR rolbypassrls OR rolinherit)) THEN RAISE EXCEPTION 'API login attributes mismatch'; END IF;
  IF EXISTS (SELECT 1 FROM pg_auth_members m JOIN pg_roles g ON g.oid=m.roleid JOIN pg_roles u ON u.oid=m.member WHERE g.rolname IN ('espacigo_schema_owner','espacigo_runtime','espacigo_gadm_bootstrap','espacigo_privacy_cleanup_worker','espacigo_gcp06_test_access') OR u.rolname IN ('espacigo_schema_owner','espacigo_runtime','espacigo_migrator_login','espacigo_api_login','espacigo_gadm_bootstrap','espacigo_privacy_cleanup_worker','espacigo_gcp06_test_access')) THEN RAISE EXCEPTION 'unexpected role membership'; END IF;
END $roles$;
CREATE EXTENSION IF NOT EXISTS postgis;
CREATE EXTENSION IF NOT EXISTS btree_gist;
CREATE EXTENSION IF NOT EXISTS unaccent;
GRANT CONNECT ON DATABASE espacigo_wizard_20261106 TO espacigo_schema_owner, espacigo_runtime, espacigo_migrator_login, espacigo_api_login;
GRANT TEMP ON DATABASE espacigo_wizard_20261106 TO espacigo_schema_owner;
REVOKE TEMP ON DATABASE espacigo_wizard_20261106 FROM PUBLIC;
GRANT USAGE, CREATE ON SCHEMA public TO espacigo_schema_owner;
GRANT USAGE ON SCHEMA public TO espacigo_runtime;
REVOKE CREATE ON SCHEMA public FROM PUBLIC;
GRANT espacigo_schema_owner TO espacigo_migrator_login WITH ADMIN FALSE, INHERIT FALSE, SET TRUE;
GRANT espacigo_runtime TO espacigo_api_login WITH ADMIN FALSE, INHERIT FALSE, SET TRUE;
COMMIT;
