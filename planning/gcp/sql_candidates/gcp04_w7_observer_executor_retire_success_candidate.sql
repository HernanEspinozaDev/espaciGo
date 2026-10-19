-- REVIEW CANDIDATE ONLY. Run after users.insert and exact read-only catalog
-- verification of the API-created login membership.
\set ON_ERROR_STOP on
BEGIN;
DO $retire_guard$
DECLARE
    actor pg_catalog.pg_roles%ROWTYPE;
    executor_oid oid;
    observer_oid oid;
    login_oid oid;
BEGIN
    IF current_database() <> 'espacigo' OR current_user <> session_user THEN
        RAISE EXCEPTION 'W7 executor retirement requires a direct actor session';
    END IF;
    SELECT * INTO actor FROM pg_catalog.pg_roles WHERE rolname=session_user;
    SELECT oid INTO executor_oid FROM pg_catalog.pg_roles WHERE rolname='espacigo_gcp04_w7_observer_executor';
    SELECT oid INTO observer_oid FROM pg_catalog.pg_roles WHERE rolname='espacigo_db_observer';
    SELECT oid INTO login_oid FROM pg_catalog.pg_roles WHERE rolname='espacigo_gcp04_w7_observer';
    IF NOT FOUND OR executor_oid IS NULL OR observer_oid IS NULL OR login_oid IS NULL
       OR NOT actor.rolcanlogin OR NOT actor.rolcreaterole OR actor.rolsuper
       OR actor.rolcreatedb OR actor.rolreplication OR actor.rolbypassrls THEN
        RAISE EXCEPTION 'W7 actor, executor, observer, or API login is absent/unexpected';
    END IF;
    IF (SELECT count(*) FROM pg_catalog.pg_auth_members m
        WHERE m.roleid=executor_oid AND m.member=(SELECT oid FROM pg_catalog.pg_roles WHERE rolname=session_user)) <> 2
       OR NOT EXISTS (
          SELECT 1 FROM pg_catalog.pg_auth_members m
          WHERE m.roleid=executor_oid AND m.member=(SELECT oid FROM pg_catalog.pg_roles WHERE rolname=session_user)
            AND m.admin_option AND NOT m.inherit_option AND NOT m.set_option
       )
       OR NOT EXISTS (
          SELECT 1 FROM pg_catalog.pg_auth_members m
          WHERE m.roleid=executor_oid AND m.member=(SELECT oid FROM pg_catalog.pg_roles WHERE rolname=session_user)
            AND NOT m.admin_option AND NOT m.inherit_option AND m.set_option
       ) THEN
        RAISE EXCEPTION 'W7 executor has unexpected actor membership edges';
    END IF;
    IF NOT EXISTS (
       SELECT 1 FROM pg_catalog.pg_auth_members m
       WHERE m.roleid=observer_oid AND m.member=executor_oid
         AND m.admin_option AND NOT m.inherit_option AND NOT m.set_option
    ) THEN RAISE EXCEPTION 'W7 executor lacks its exact automatic observer ADMIN edge'; END IF;
    IF EXISTS (
       SELECT 1 FROM pg_catalog.pg_auth_members m
       WHERE m.roleid=observer_oid AND m.member=login_oid
         AND m.grantor=executor_oid
    ) THEN RAISE EXCEPTION 'W7 login membership is executor-granted; retire is unsafe'; END IF;
    IF (SELECT count(*) FROM pg_catalog.pg_auth_members m WHERE m.roleid=observer_oid) <> 2
       OR NOT EXISTS (
         SELECT 1 FROM pg_catalog.pg_auth_members m
         WHERE m.roleid=observer_oid AND m.member=login_oid
           AND NOT m.admin_option AND m.inherit_option AND NOT m.set_option
       ) THEN RAISE EXCEPTION 'W7 group membership is not exactly executor plus contracted login'; END IF;
    IF EXISTS (SELECT 1 FROM pg_catalog.pg_auth_members m WHERE m.member=observer_oid) THEN
        RAISE EXCEPTION 'W7 observer is a member of another role';
    END IF;
    IF EXISTS (SELECT 1 FROM pg_catalog.pg_shdepend d
        WHERE d.refclassid='pg_catalog.pg_authid'::regclass
          AND d.refobjid IN (executor_oid,observer_oid) AND d.deptype='o') THEN
        RAISE EXCEPTION 'W7 executor/observer owns objects; reconcile before retirement';
    END IF;
END
$retire_guard$;

-- Removing the executor's own ADMIN membership leaves the independently
-- API-granted login edge intact. The API membership grantor must not be the
-- executor; otherwise this transaction fails closed.
DROP ROLE espacigo_gcp04_w7_observer_executor;
DO $retire_assert$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname='espacigo_gcp04_w7_observer_executor') THEN
        RAISE EXCEPTION 'W7 executor still exists after retirement';
    END IF;
    IF (SELECT count(*) FROM pg_catalog.pg_auth_members m
        JOIN pg_catalog.pg_roles r ON r.oid=m.roleid
        WHERE r.rolname='espacigo_db_observer') <> 1
       OR NOT EXISTS (
          SELECT 1 FROM pg_catalog.pg_auth_members m
          JOIN pg_catalog.pg_roles r ON r.oid=m.roleid
          JOIN pg_catalog.pg_roles member ON member.oid=m.member
          WHERE r.rolname='espacigo_db_observer'
            AND member.rolname='espacigo_gcp04_w7_observer'
            AND NOT m.admin_option AND m.inherit_option AND NOT m.set_option
       ) THEN RAISE EXCEPTION 'W7 login edge did not remain exact after executor removal'; END IF;
END
$retire_assert$;
COMMIT;
