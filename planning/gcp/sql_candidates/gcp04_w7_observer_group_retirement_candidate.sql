-- SUPERSEDED historical candidate from PR #429. Do not execute; see
-- gcp04_w7_executor_membership_finding_20261019.md for the tested lifecycle.
-- REVIEW CANDIDATE ONLY. Not approved for Cloud SQL or production execution.
-- First delete the exact W7 login via the approved Cloud SQL Admin API.
-- This transaction assumes the creating CREATEROLE actor holds the original
-- ACL grant options and only its automatic ADMIN edge remains on the group.
\set ON_ERROR_STOP on

BEGIN;

DO $retire_guard$
DECLARE
    actor pg_catalog.pg_roles%ROWTYPE;
    observer_oid oid;
BEGIN
    IF current_database() <> 'espacigo' OR current_user <> session_user THEN
        RAISE EXCEPTION 'W7 retirement requires a direct session on database espacigo';
    END IF;
    IF EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname = 'espacigo_gcp04_w7_observer') THEN
        RAISE EXCEPTION 'delete the exact W7 Cloud SQL login through users.delete first';
    END IF;

    SELECT * INTO actor FROM pg_catalog.pg_roles WHERE rolname = session_user;
    SELECT oid INTO observer_oid FROM pg_catalog.pg_roles WHERE rolname = 'espacigo_db_observer';
    IF NOT FOUND OR observer_oid IS NULL OR NOT actor.rolcanlogin OR NOT actor.rolcreaterole
       OR actor.rolsuper OR actor.rolcreatedb OR actor.rolreplication OR actor.rolbypassrls THEN
        RAISE EXCEPTION 'retirement actor or observer group is not the reviewed direct CREATEROLE context';
    END IF;
    IF EXISTS (
        SELECT 1 FROM pg_catalog.pg_auth_members m
        JOIN pg_catalog.pg_roles member ON member.oid = m.member
        WHERE m.roleid = observer_oid
          AND (member.rolname <> session_user OR NOT m.admin_option OR m.inherit_option OR m.set_option)
    ) THEN
        RAISE EXCEPTION 'observer group has an unexpected member; reconcile without CASCADE';
    END IF;
    IF EXISTS (
        SELECT 1 FROM pg_catalog.pg_shdepend d
        WHERE d.refclassid = 'pg_catalog.pg_authid'::regclass
          AND d.refobjid = observer_oid AND d.deptype = 'o'
    ) THEN
        RAISE EXCEPTION 'observer group owns objects; stop for explicit reconciliation';
    END IF;
    IF NOT has_database_privilege(session_user, current_database(), 'CONNECT WITH GRANT OPTION')
       OR NOT has_schema_privilege(session_user, 'public', 'USAGE WITH GRANT OPTION') THEN
        RAISE EXCEPTION 'retirement actor no longer holds the original database/schema grant options';
    END IF;
    IF to_regclass('public.schema_migrations') IS NOT NULL
       AND NOT has_table_privilege(session_user, 'public.schema_migrations', 'SELECT WITH GRANT OPTION') THEN
        RAISE EXCEPTION 'retirement actor no longer holds the original ledger grant option';
    END IF;
END
$retire_guard$;

REVOKE CONNECT ON DATABASE espacigo FROM espacigo_db_observer;
REVOKE USAGE ON SCHEMA public FROM espacigo_db_observer;

DO $ledger_revoke$
BEGIN
    IF to_regclass('public.schema_migrations') IS NOT NULL THEN
        EXECUTE 'REVOKE SELECT ON TABLE public.schema_migrations FROM espacigo_db_observer';
    END IF;
END
$ledger_revoke$;

DROP ROLE espacigo_db_observer;
COMMIT;
