-- REVIEW CANDIDATE ONLY. Local PostgreSQL 18 validation required before use.
-- B1 is an elevated, temporary bootstrap login. This SQL creates only the
-- durable NOLOGIN read-only observer group; the observer login is a separate
-- Cloud SQL Admin API operation and is deliberately absent here.
\set ON_ERROR_STOP on

BEGIN;

DO $prepare_guard$
DECLARE actor pg_catalog.pg_roles%ROWTYPE;
        direct_memberships integer;
BEGIN
    IF current_database() <> 'espacigo'
       OR session_user <> 'espacigo_gcp04_w7_bootstrap'
       OR current_user <> session_user THEN
        RAISE EXCEPTION 'W7 B1 observer prepare requires the exact direct bootstrap session';
    END IF;

    SELECT * INTO actor FROM pg_catalog.pg_roles WHERE rolname=session_user;
    IF NOT FOUND OR NOT actor.rolcanlogin OR NOT actor.rolcreaterole
       OR NOT actor.rolcreatedb OR actor.rolsuper OR actor.rolreplication
       OR actor.rolbypassrls OR NOT actor.rolinherit THEN
        RAISE EXCEPTION 'W7 B1 bootstrap actor attributes differ from the reviewed candidate';
    END IF;

    SELECT count(*) INTO direct_memberships
    FROM pg_catalog.pg_auth_members m
    JOIN pg_catalog.pg_roles granted ON granted.oid=m.roleid
    WHERE m.member=actor.oid AND granted.rolname='cloudsqlsuperuser'
      AND NOT m.admin_option AND m.inherit_option;
    IF direct_memberships <> 1 OR
       (SELECT count(*) FROM pg_catalog.pg_auth_members WHERE member=actor.oid) <> 1 THEN
        RAISE EXCEPTION 'W7 B1 actor must have only the expected cloudsqlsuperuser membership';
    END IF;

    IF has_database_privilege('public', current_database(), 'TEMP') THEN
        RAISE EXCEPTION 'W7 PUBLIC TEMP baseline is not satisfied; this candidate never changes PUBLIC ACLs';
    END IF;
    IF NOT has_database_privilege(session_user, current_database(), 'CONNECT WITH GRANT OPTION')
       OR NOT has_schema_privilege(session_user, 'public', 'USAGE WITH GRANT OPTION') THEN
        RAISE EXCEPTION 'W7 B1 actor lacks effective database CONNECT or public schema USAGE grant option';
    END IF;
    IF to_regclass('public.schema_migrations') IS NOT NULL
       AND NOT has_table_privilege(session_user, 'public.schema_migrations', 'SELECT WITH GRANT OPTION') THEN
        RAISE EXCEPTION 'W7 B1 actor lacks effective ledger SELECT grant option';
    END IF;
    IF EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname IN
       ('espacigo_db_observer','espacigo_gcp04_w7_observer_executor','espacigo_gcp04_w7_observer')) THEN
        RAISE EXCEPTION 'W7 reserved observer identity collision; do not adopt or alter it';
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

GRANT CONNECT ON DATABASE espacigo TO espacigo_db_observer;
GRANT USAGE ON SCHEMA public TO espacigo_db_observer;
DO $ledger_grant$
BEGIN
    IF to_regclass('public.schema_migrations') IS NOT NULL THEN
        EXECUTE 'GRANT SELECT ON TABLE public.schema_migrations TO espacigo_db_observer';
    END IF;
END
$ledger_grant$;

DROP ROLE espacigo_gcp04_w7_observer_executor;

DO $prepare_assert$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname='espacigo_gcp04_w7_observer_executor') THEN
        RAISE EXCEPTION 'W7 temporary observer executor remains after preparation';
    END IF;
    IF EXISTS (SELECT 1 FROM pg_catalog.pg_auth_members m
        JOIN pg_catalog.pg_roles granted ON granted.oid=m.roleid
        JOIN pg_catalog.pg_roles member ON member.oid=m.member
        WHERE granted.rolname='espacigo_db_observer' OR member.rolname='espacigo_db_observer') THEN
        RAISE EXCEPTION 'W7 observer group must have no membership or steward edge while idle';
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname='espacigo_db_observer'
        AND NOT rolcanlogin AND NOT rolsuper AND NOT rolcreatedb AND NOT rolcreaterole
        AND NOT rolreplication AND NOT rolbypassrls AND NOT rolinherit) THEN
        RAISE EXCEPTION 'W7 observer group attributes differ from the minimum-privilege contract';
    END IF;
    IF NOT has_database_privilege('espacigo_db_observer','espacigo','CONNECT')
       OR has_database_privilege('espacigo_db_observer','espacigo','TEMP')
       OR has_database_privilege('espacigo_db_observer','espacigo','CREATE')
       OR NOT has_schema_privilege('espacigo_db_observer','public','USAGE')
       OR has_schema_privilege('espacigo_db_observer','public','CREATE') THEN
        RAISE EXCEPTION 'W7 observer effective database/schema ACL exceeds the read-only contract';
    END IF;
    IF to_regclass('public.schema_migrations') IS NOT NULL
       AND (NOT has_table_privilege('espacigo_db_observer','public.schema_migrations','SELECT')
            OR has_table_privilege('espacigo_db_observer','public.schema_migrations','INSERT')
            OR has_table_privilege('espacigo_db_observer','public.schema_migrations','UPDATE')
            OR has_table_privilege('espacigo_db_observer','public.schema_migrations','DELETE')) THEN
        RAISE EXCEPTION 'W7 observer ledger ACL is not SELECT-only';
    END IF;
    IF EXISTS (SELECT 1 FROM pg_catalog.pg_default_acl d
        WHERE d.defaclrole=(SELECT oid FROM pg_catalog.pg_roles WHERE rolname='espacigo_db_observer')) THEN
        RAISE EXCEPTION 'W7 observer group has unexpected default privileges';
    END IF;
END
$prepare_assert$;

COMMIT;
