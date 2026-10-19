-- REVIEW CANDIDATE ONLY. Run after API reconciliation proves the exact W7
-- login is absent (or after users.delete and a fresh absence check). The
-- short-lived executor must still exist so it can drop its observer group.
\set ON_ERROR_STOP on
BEGIN;
DO $cleanup_guard$
DECLARE
    actor pg_catalog.pg_roles%ROWTYPE;
    observer_oid oid;
BEGIN
    IF current_database() <> 'espacigo' OR current_user <> session_user THEN
        RAISE EXCEPTION 'W7 compensation requires direct session on database espacigo';
    END IF;
    SELECT * INTO actor FROM pg_catalog.pg_roles WHERE rolname=session_user;
    SELECT oid INTO observer_oid FROM pg_catalog.pg_roles WHERE rolname='espacigo_db_observer';
    IF NOT FOUND OR observer_oid IS NULL OR NOT actor.rolcanlogin OR NOT actor.rolcreaterole
       OR actor.rolsuper OR actor.rolcreatedb OR actor.rolreplication OR actor.rolbypassrls THEN
        RAISE EXCEPTION 'W7 compensation actor or observer group is absent/unexpected';
    END IF;
    IF EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname='espacigo_gcp04_w7_observer') THEN
        RAISE EXCEPTION 'reconcile/delete the exact API login before W7 compensation';
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname='espacigo_gcp04_w7_observer_executor') THEN
        RAISE EXCEPTION 'W7 temporary executor is absent; do not infer DROP ROLE authority';
    END IF;
    IF (SELECT count(*) FROM pg_catalog.pg_auth_members m WHERE m.roleid=observer_oid) <> 1
       OR EXISTS (SELECT 1 FROM pg_catalog.pg_auth_members m WHERE m.member=observer_oid) THEN
        RAISE EXCEPTION 'W7 observer has unexpected memberships; stop without CASCADE';
    END IF;
    IF EXISTS (SELECT 1 FROM pg_catalog.pg_shdepend d
        WHERE d.refclassid='pg_catalog.pg_authid'::regclass AND d.refobjid=observer_oid AND d.deptype='o') THEN
        RAISE EXCEPTION 'W7 observer owns objects; stop without DROP OWNED';
    END IF;
    IF NOT has_database_privilege(session_user,current_database(),'CONNECT WITH GRANT OPTION')
       OR NOT has_schema_privilege(session_user,'public','USAGE WITH GRANT OPTION') THEN
        RAISE EXCEPTION 'W7 original actor lost required database/schema grant options';
    END IF;
    IF to_regclass('public.schema_migrations') IS NOT NULL
       AND NOT has_table_privilege(session_user,'public.schema_migrations','SELECT WITH GRANT OPTION') THEN
        RAISE EXCEPTION 'W7 original actor lost ledger grant option';
    END IF;
END
$cleanup_guard$;
REVOKE CONNECT ON DATABASE espacigo FROM espacigo_db_observer;
REVOKE USAGE ON SCHEMA public FROM espacigo_db_observer;
DO $ledger_revoke$
BEGIN
    IF to_regclass('public.schema_migrations') IS NOT NULL THEN
        EXECUTE 'REVOKE SELECT ON TABLE public.schema_migrations FROM espacigo_db_observer';
    END IF;
END
$ledger_revoke$;
SET ROLE espacigo_gcp04_w7_observer_executor;
DROP ROLE espacigo_db_observer;
RESET ROLE;
DROP ROLE espacigo_gcp04_w7_observer_executor;
DO $cleanup_assert$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname IN
       ('espacigo_db_observer','espacigo_gcp04_w7_observer_executor')) THEN
        RAISE EXCEPTION 'W7 compensation left a reserved role';
    END IF;
END
$cleanup_assert$;
COMMIT;
