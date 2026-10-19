-- SUPERSEDED historical candidate from PR #429. Do not execute; see
-- gcp04_w7_executor_membership_finding_20261019.md for the tested lifecycle.
-- REVIEW CANDIDATE ONLY. Not approved for Cloud SQL or production execution.
-- Requires a separately verified direct LOGIN CREATEROLE actor plus GRANT
-- OPTION on database CONNECT, public schema USAGE, and ledger SELECT if present.
-- Refuses PUBLIC TEMP instead of modifying it. A direct CREATEROLE creator
-- retains ADMIN OPTION on the new role; see the W7 decision document.
\set ON_ERROR_STOP on

BEGIN;

DO $guard$
DECLARE
    actor pg_catalog.pg_roles%ROWTYPE;
BEGIN
    IF current_database() <> 'espacigo' OR current_user <> session_user THEN
        RAISE EXCEPTION 'W7 requires a direct session on database espacigo';
    END IF;

    SELECT * INTO actor FROM pg_catalog.pg_roles WHERE rolname = session_user;
    IF NOT FOUND OR NOT actor.rolcanlogin OR NOT actor.rolcreaterole
       OR actor.rolsuper OR actor.rolcreatedb OR actor.rolreplication
       OR actor.rolbypassrls OR actor.rolinherit THEN
        RAISE EXCEPTION 'W7 requires a direct LOGIN CREATEROLE actor with elevated attributes disabled';
    END IF;
    IF EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname IN
        ('espacigo_db_observer','espacigo_gcp04_w7_observer_executor')) THEN
        RAISE EXCEPTION 'W7 role collision; reconcile without adopting or changing existing roles';
    END IF;
    IF has_database_privilege('public', current_database(), 'TEMP')
       OR NOT has_database_privilege(session_user, current_database(), 'CONNECT WITH GRANT OPTION')
       OR NOT has_schema_privilege(session_user, 'public', 'USAGE WITH GRANT OPTION') THEN
        RAISE EXCEPTION 'W7 PUBLIC baseline or database/schema grant authority is not satisfied';
    END IF;
    IF to_regclass('public.schema_migrations') IS NOT NULL
       AND NOT has_table_privilege(session_user, 'public.schema_migrations', 'SELECT WITH GRANT OPTION') THEN
        RAISE EXCEPTION 'W7 actor lacks ledger SELECT grant authority';
    END IF;
END
$guard$;

CREATE ROLE espacigo_db_observer
    NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE
    NOREPLICATION NOBYPASSRLS NOINHERIT;

GRANT CONNECT ON DATABASE espacigo TO espacigo_db_observer;
GRANT USAGE ON SCHEMA public TO espacigo_db_observer;

DO $ledger$
BEGIN
    IF to_regclass('public.schema_migrations') IS NOT NULL THEN
        EXECUTE 'GRANT SELECT ON TABLE public.schema_migrations TO espacigo_db_observer';
    END IF;
END
$ledger$;

COMMIT;
