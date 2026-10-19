-- LOCAL DIAGNOSTIC ONLY. Not executable on the Cloud SQL candidate until the
-- V60 role-comment/membership compatibility is resolved and reviewed.
-- In Cloud SQL, a CREATEROLE creator may receive an ADMIN membership edge;
-- the fail-closed membership check below is expected to abort and roll back.
-- Never run this file against persistent Cloud SQL in its current form.
\set ON_ERROR_STOP on

BEGIN;

DO $bootstrap$
DECLARE
    role_name text;
    role_row record;
BEGIN
    IF current_database() <> 'espacigo' THEN
        RAISE EXCEPTION 'refusing GCP-04 role bootstrap in unexpected database %', current_database();
    END IF;

    FOREACH role_name IN ARRAY ARRAY[
        'espacigo_schema_owner',
        'espacigo_runtime',
        'espacigo_privacy_cleanup_worker',
        'espacigo_db_observer',
        'espacigo_gadm_bootstrap'
    ] LOOP
        SELECT * INTO role_row FROM pg_catalog.pg_roles WHERE rolname = role_name;
        IF NOT FOUND THEN
            EXECUTE format('CREATE ROLE %I NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT', role_name);
        ELSIF role_row.rolcanlogin OR role_row.rolsuper OR role_row.rolcreatedb OR role_row.rolcreaterole
           OR role_row.rolreplication OR role_row.rolbypassrls OR role_row.rolinherit THEN
            RAISE EXCEPTION 'existing role % has unexpected attributes', role_name;
        END IF;

        IF EXISTS (
            SELECT 1 FROM pg_catalog.pg_auth_members m
            JOIN pg_catalog.pg_roles granted_role ON granted_role.oid = m.roleid
            JOIN pg_catalog.pg_roles member_role ON member_role.oid = m.member
            WHERE granted_role.rolname = role_name OR member_role.rolname = role_name
        ) THEN
            RAISE EXCEPTION 'role % already participates in a membership; reconcile before bootstrap', role_name;
        END IF;
    END LOOP;

END
$bootstrap$;

-- Cloud SQL extension installation is intentionally performed by the
-- reviewed administrative bootstrap identity, before migrations run as
-- espacigo_schema_owner. No runtime role receives extension privileges.
CREATE EXTENSION IF NOT EXISTS btree_gist;
CREATE EXTENSION IF NOT EXISTS postgis;
CREATE EXTENSION IF NOT EXISTS unaccent;

GRANT CONNECT ON DATABASE espacigo TO espacigo_schema_owner WITH GRANT OPTION;
GRANT CONNECT ON DATABASE espacigo TO espacigo_runtime, espacigo_privacy_cleanup_worker, espacigo_db_observer;
-- V57 uses a temporary table while applying its typed category-filter update.
GRANT TEMP ON DATABASE espacigo TO espacigo_schema_owner;
GRANT USAGE ON SCHEMA public TO espacigo_runtime, espacigo_privacy_cleanup_worker, espacigo_db_observer, espacigo_gadm_bootstrap;
GRANT USAGE, CREATE ON SCHEMA public TO espacigo_schema_owner;
GRANT USAGE ON SCHEMA public TO espacigo_schema_owner WITH GRANT OPTION;
REVOKE CREATE ON SCHEMA public FROM PUBLIC;
REVOKE TEMP ON DATABASE espacigo FROM PUBLIC;
-- Temporary bootstrap edge: the first V1-V64 run connects as built-in
-- postgres and explicitly SET ROLEs to schema_owner. Stage 002 revokes this.
GRANT espacigo_schema_owner TO postgres WITH ADMIN FALSE, INHERIT FALSE, SET TRUE;

COMMIT;
