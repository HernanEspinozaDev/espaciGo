-- GCP-04-W5 LOCAL/REVIEW CANDIDATE. Global role bootstrap before logical
-- restore only; it does not replay or replace any migration, including V60.
-- Run directly as a dedicated NOSUPERUSER CREATEROLE actor. The transient
-- CREATEROLE intermediary exists only in this transaction and is dropped
-- before COMMIT, removing its automatic ADMIN memberships.
\set ON_ERROR_STOP on

BEGIN;

DO $preflight$
DECLARE
    actor_name name := session_user;
    target_row record;
    target_oid oid;
BEGIN
    IF current_user <> session_user THEN
        RAISE EXCEPTION 'GADM bootstrap must run directly as session_user, without SET ROLE';
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_catalog.pg_roles
         WHERE rolname = actor_name AND rolcanlogin AND NOT rolsuper
           AND NOT rolcreatedb AND rolcreaterole AND NOT rolreplication
           AND NOT rolbypassrls AND NOT rolinherit
    ) THEN
        RAISE EXCEPTION 'GADM bootstrap requires a direct LOGIN NOSUPERUSER CREATEROLE actor with other elevated attributes disabled';
    END IF;

    IF EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname='espacigo_gadm_bootstrap_executor') THEN
        RAISE EXCEPTION 'unexpected pre-existing GADM bootstrap executor role';
    END IF;

    SELECT oid INTO target_oid FROM pg_catalog.pg_roles WHERE rolname='espacigo_gadm_bootstrap';
    IF target_oid IS NULL THEN
        RETURN;
    END IF;

    SELECT * INTO target_row FROM pg_catalog.pg_roles WHERE oid=target_oid;
    IF target_row.rolcanlogin OR target_row.rolsuper OR target_row.rolcreatedb
       OR target_row.rolcreaterole OR target_row.rolreplication
       OR target_row.rolbypassrls OR target_row.rolinherit OR target_row.rolconnlimit <> -1 THEN
        RAISE EXCEPTION 'existing espacigo_gadm_bootstrap has unexpected role attributes';
    END IF;
    IF shobj_description(target_oid,'pg_authid') IS DISTINCT FROM
       'NOLOGIN GADM local bootstrap group; temporary isolated login membership only.' THEN
        RAISE EXCEPTION 'existing espacigo_gadm_bootstrap has unexpected comment';
    END IF;
    IF EXISTS (
        SELECT 1 FROM pg_catalog.pg_auth_members m
         WHERE m.roleid=target_oid OR m.member=target_oid
    ) THEN
        RAISE EXCEPTION 'existing espacigo_gadm_bootstrap has role memberships';
    END IF;
    IF EXISTS (
        SELECT 1 FROM pg_catalog.pg_shdepend d
         WHERE d.refclassid='pg_catalog.pg_authid'::regclass
           AND d.refobjid=target_oid AND d.deptype='o'
    ) THEN
        RAISE EXCEPTION 'existing espacigo_gadm_bootstrap owns database objects';
    END IF;
    IF EXISTS (SELECT 1 FROM pg_catalog.pg_db_role_setting WHERE setrole=target_oid) THEN
        RAISE EXCEPTION 'existing espacigo_gadm_bootstrap has role settings';
    END IF;
END
$preflight$;

SELECT EXISTS (
    SELECT 1 FROM pg_catalog.pg_roles WHERE rolname='espacigo_gadm_bootstrap'
) AS gadm_exists \gset

\if :gadm_exists
\echo 'Existing exact GADM role contract validated; no changes made.'
\else
DO $executor$
DECLARE actor_name name := session_user;
BEGIN
    EXECUTE 'CREATE ROLE espacigo_gadm_bootstrap_executor NOLOGIN NOSUPERUSER NOCREATEDB CREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT';
    -- CREATE ROLE grants the actor an ADMIN edge with SET disabled. Add a
    -- separate actor-granted edge allowing this session alone to SET ROLE.
    EXECUTE format(
        'GRANT espacigo_gadm_bootstrap_executor TO %I WITH ADMIN FALSE, INHERIT FALSE, SET TRUE',
        actor_name
    );
END
$executor$;

SET ROLE espacigo_gadm_bootstrap_executor;
CREATE ROLE espacigo_gadm_bootstrap
    NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT;
COMMENT ON ROLE espacigo_gadm_bootstrap IS
    'NOLOGIN GADM local bootstrap group; temporary isolated login membership only.';
RESET ROLE;

-- Dropping the transaction-local CREATEROLE identity removes its automatic
-- ADMIN membership to GADM; the outer transaction keeps creation+cleanup
-- atomic. Do not replace this with a long-lived actor/member grant.
DROP ROLE espacigo_gadm_bootstrap_executor;

DO $assertions$
DECLARE target_oid oid;
BEGIN
    SELECT oid INTO target_oid FROM pg_catalog.pg_roles WHERE rolname='espacigo_gadm_bootstrap';
    IF target_oid IS NULL OR EXISTS (
        SELECT 1 FROM pg_catalog.pg_roles
         WHERE oid=target_oid AND (rolcanlogin OR rolsuper OR rolcreatedb OR rolcreaterole
                                   OR rolreplication OR rolbypassrls OR rolinherit OR rolconnlimit <> -1)
    ) THEN
        RAISE EXCEPTION 'created GADM role does not match the required attributes';
    END IF;
    IF shobj_description(target_oid,'pg_authid') IS DISTINCT FROM
       'NOLOGIN GADM local bootstrap group; temporary isolated login membership only.' THEN
        RAISE EXCEPTION 'created GADM role comment differs from the V60 contract';
    END IF;
    IF EXISTS (
        SELECT 1 FROM pg_catalog.pg_auth_members m
         WHERE m.roleid=target_oid OR m.member=target_oid
    ) THEN
        RAISE EXCEPTION 'GADM bootstrap left a membership edge';
    END IF;
    IF EXISTS (
        SELECT 1 FROM pg_catalog.pg_shdepend d
         WHERE d.refclassid='pg_catalog.pg_authid'::regclass
           AND d.refobjid=target_oid AND d.deptype='o'
    ) THEN
        RAISE EXCEPTION 'GADM bootstrap left owned objects';
    END IF;
    IF EXISTS (SELECT 1 FROM pg_catalog.pg_db_role_setting WHERE setrole=target_oid) THEN
        RAISE EXCEPTION 'GADM bootstrap left role settings';
    END IF;
END
$assertions$;
\endif

COMMIT;
