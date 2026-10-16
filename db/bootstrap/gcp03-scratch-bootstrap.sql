-- REVIEW ONLY: never run this file without a separately reviewed scratch execution plan.
-- It contains no password or production database reference. Run only against the
-- explicitly named disposable database after confirming its instance connection name.
\set ON_ERROR_STOP on

DO $$
BEGIN
    IF current_database() <> 'espacigo_scratch' THEN
        RAISE EXCEPTION 'refusing bootstrap in unexpected database %', current_database();
    END IF;
END
$$;

-- Cloud SQL requires elevated scratch-only migration authority to provision these
-- supported extensions. The temporary migration login is not the application role.
CREATE EXTENSION IF NOT EXISTS btree_gist;
CREATE EXTENSION IF NOT EXISTS postgis;
CREATE EXTENSION IF NOT EXISTS unaccent;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'espacigo_runtime') THEN
        CREATE ROLE espacigo_runtime
            NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT;
    END IF;

    IF EXISTS (
        SELECT 1 FROM pg_roles
        WHERE rolname = 'espacigo_runtime'
          AND (rolcanlogin OR rolsuper OR rolcreatedb OR rolcreaterole OR rolreplication OR rolbypassrls OR rolinherit)
    ) THEN
        RAISE EXCEPTION 'existing espacigo_runtime role has broader attributes than expected';
    END IF;
END
$$;

GRANT CONNECT ON DATABASE espacigo_scratch TO espacigo_runtime;
