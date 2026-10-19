-- REVIEW CANDIDATE ONLY. PostgreSQL 18 local validation required before use.
-- The executor is retained only for the short users.insert/verification
-- window. Cloud SQL users.insert assigns the group; this SQL does not issue it.
\set ON_ERROR_STOP on

BEGIN;

DO $prepare_guard$
DECLARE actor pg_catalog.pg_roles%ROWTYPE;
BEGIN
    IF current_database() <> 'espacigo' OR current_user <> session_user THEN
        RAISE EXCEPTION 'W7 prepare requires a direct session on database espacigo';
    END IF;
    SELECT * INTO actor FROM pg_catalog.pg_roles WHERE rolname=session_user;
    IF NOT FOUND OR NOT actor.rolcanlogin OR NOT actor.rolcreaterole
       OR actor.rolsuper OR actor.rolcreatedb OR actor.rolreplication
       OR actor.rolbypassrls OR actor.rolinherit THEN
        RAISE EXCEPTION 'W7 prepare requires direct LOGIN CREATEROLE actor with elevated attributes disabled';
    END IF;
    IF has_database_privilege('public', current_database(), 'TEMP') THEN
        RAISE EXCEPTION 'W7 PUBLIC TEMP baseline is not satisfied; do not change PUBLIC ACL here';
    END IF;
    IF NOT has_database_privilege(session_user, current_database(), 'CONNECT WITH GRANT OPTION')
       OR NOT has_schema_privilege(session_user, 'public', 'USAGE WITH GRANT OPTION') THEN
        RAISE EXCEPTION 'W7 actor lacks database CONNECT or public schema USAGE grant option';
    END IF;
    IF to_regclass('public.schema_migrations') IS NOT NULL
       AND NOT has_table_privilege(session_user, 'public.schema_migrations', 'SELECT WITH GRANT OPTION') THEN
        RAISE EXCEPTION 'W7 actor lacks ledger SELECT grant option';
    END IF;
    IF EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname IN
       ('espacigo_db_observer','espacigo_gcp04_w7_observer_executor','espacigo_gcp04_w7_observer')) THEN
        RAISE EXCEPTION 'W7 reserved role collision; reconcile without adopting existing roles';
    END IF;
END
$prepare_guard$;

CREATE ROLE espacigo_gcp04_w7_observer_executor
    NOLOGIN NOSUPERUSER NOCREATEDB CREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT;
GRANT espacigo_gcp04_w7_observer_executor TO CURRENT_USER
    WITH ADMIN FALSE, INHERIT FALSE, SET TRUE;

SET ROLE espacigo_gcp04_w7_observer_executor;
CREATE ROLE espacigo_db_observer
    NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT;
RESET ROLE;

-- Object grants stay attributed to the original actor, which already passed
-- the exact GRANT OPTION checks above.
GRANT CONNECT ON DATABASE espacigo TO espacigo_db_observer;
GRANT USAGE ON SCHEMA public TO espacigo_db_observer;
DO $ledger_grant$
BEGIN
    IF to_regclass('public.schema_migrations') IS NOT NULL THEN
        EXECUTE 'GRANT SELECT ON TABLE public.schema_migrations TO espacigo_db_observer';
    END IF;
END
$ledger_grant$;

DO $prepare_assert$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname='espacigo_gcp04_w7_observer_executor') THEN
        RAISE EXCEPTION 'temporary CREATEROLE executor missing during API window';
    END IF;
    IF (SELECT count(*) FROM pg_catalog.pg_auth_members m JOIN pg_catalog.pg_roles r ON r.oid=m.roleid
        WHERE r.rolname='espacigo_db_observer') <> 1
       OR NOT EXISTS (
        SELECT 1 FROM pg_catalog.pg_auth_members m
        JOIN pg_catalog.pg_roles r ON r.oid=m.roleid
        JOIN pg_catalog.pg_roles member ON member.oid=m.member
        WHERE r.rolname='espacigo_db_observer'
          AND member.rolname='espacigo_gcp04_w7_observer_executor'
          AND m.admin_option AND NOT m.inherit_option AND NOT m.set_option
       ) THEN
        RAISE EXCEPTION 'W7 executor membership of observer group is not exact';
    END IF;
    IF NOT has_database_privilege('espacigo_db_observer','espacigo','CONNECT')
       OR has_database_privilege('espacigo_db_observer','espacigo','TEMP')
       OR has_database_privilege('espacigo_db_observer','espacigo','CREATE')
       OR NOT has_schema_privilege('espacigo_db_observer','public','USAGE')
       OR has_schema_privilege('espacigo_db_observer','public','CREATE') THEN
        RAISE EXCEPTION 'W7 group effective database/schema ACL is not the read-only contract';
    END IF;
    IF to_regclass('public.schema_migrations') IS NOT NULL
       AND (NOT has_table_privilege('espacigo_db_observer','public.schema_migrations','SELECT')
            OR has_table_privilege('espacigo_db_observer','public.schema_migrations','INSERT')
            OR has_table_privilege('espacigo_db_observer','public.schema_migrations','UPDATE')
            OR has_table_privilege('espacigo_db_observer','public.schema_migrations','DELETE')) THEN
        RAISE EXCEPTION 'W7 ledger ACL is not SELECT-only';
    END IF;
END
$prepare_assert$;

COMMIT;
