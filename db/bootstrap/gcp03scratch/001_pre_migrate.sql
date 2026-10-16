-- GCP-03 scratch executor phase: bootstrap, schema revision 1.
-- The Go command embeds this file and separately verifies the exact instance,
-- database, login, and operator confirmation before opening a connection.
DO $guard$
BEGIN
    IF current_database() <> 'espacigo_scratch' THEN
        RAISE EXCEPTION 'GCP-03 bootstrap refused unexpected database';
    END IF;
    IF current_user <> 'espacigo_scratch_migrator' THEN
        RAISE EXCEPTION 'GCP-03 bootstrap refused unexpected login';
    END IF;
    IF to_regclass('public.schema_migrations') IS NOT NULL THEN
        RAISE EXCEPTION 'GCP-03 bootstrap must run before migrations';
    END IF;
END
$guard$;

-- The temporary Cloud SQL built-in login supplies cloudsqlsuperuser only while
-- this isolated database installs the supported extensions and runs V1-V61.
CREATE EXTENSION IF NOT EXISTS btree_gist;
CREATE EXTENSION IF NOT EXISTS postgis;
CREATE EXTENSION IF NOT EXISTS unaccent;

DO $runtime$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname = 'espacigo_runtime') THEN
        CREATE ROLE espacigo_runtime
            NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT;
    ELSIF EXISTS (
        SELECT 1 FROM pg_catalog.pg_roles
        WHERE rolname = 'espacigo_runtime'
          AND (rolcanlogin OR rolsuper OR rolcreatedb OR rolcreaterole OR rolreplication OR rolbypassrls OR rolinherit)
    ) THEN
        RAISE EXCEPTION 'GCP-03 runtime role has unsafe attributes';
    END IF;
    IF EXISTS (
        SELECT 1
        FROM pg_catalog.pg_auth_members m
        JOIN pg_catalog.pg_roles role_name ON role_name.oid = m.roleid
        JOIN pg_catalog.pg_roles member_name ON member_name.oid = m.member
        WHERE role_name.rolname = 'espacigo_runtime' OR member_name.rolname = 'espacigo_runtime'
    ) THEN
        RAISE EXCEPTION 'GCP-03 runtime role must have no memberships';
    END IF;
END
$runtime$;

GRANT CONNECT ON DATABASE espacigo_scratch TO espacigo_runtime;
