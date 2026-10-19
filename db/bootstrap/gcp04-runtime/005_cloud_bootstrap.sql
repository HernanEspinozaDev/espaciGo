-- GCP-04 W7 runtime bootstrap. Fixed, one-shot SQL executed through the
-- Cloud SQL connector as the observed temporary B1 login. API/migrator logins
-- are created separately with Cloud SQL users.insert after this transaction.
BEGIN;

DO $preflight$
DECLARE actor_oid oid;
BEGIN
    IF current_database() <> 'espacigo'
       OR session_user <> 'espacigo_gcp04_w7_bootstrap'
       OR current_user <> session_user THEN
        RAISE EXCEPTION 'unexpected W7 bootstrap target or session';
    END IF;
    SELECT oid INTO actor_oid FROM pg_catalog.pg_roles
     WHERE rolname = session_user AND rolcanlogin AND NOT rolsuper
       AND rolcreatedb AND rolcreaterole AND NOT rolreplication
       AND NOT rolbypassrls AND rolinherit;
    IF actor_oid IS NULL THEN RAISE EXCEPTION 'B1 actor attributes changed'; END IF;
    IF (SELECT count(*) FROM pg_catalog.pg_auth_members m
         JOIN pg_catalog.pg_roles r ON r.oid = m.roleid
        WHERE m.member = actor_oid) <> 1
       OR NOT EXISTS (
           SELECT 1 FROM pg_catalog.pg_auth_members m
           JOIN pg_catalog.pg_roles r ON r.oid = m.roleid
           WHERE m.member = actor_oid AND r.rolname='cloudsqlsuperuser'
             AND NOT m.admin_option AND m.inherit_option AND m.set_option
       ) THEN RAISE EXCEPTION 'B1 membership changed'; END IF;
    IF (SELECT pg_catalog.pg_get_userbyid(datdba) FROM pg_catalog.pg_database WHERE datname='espacigo') <> 'cloudsqlsuperuser'
       OR (SELECT pg_catalog.pg_get_userbyid(nspowner) FROM pg_catalog.pg_namespace WHERE nspname='public') <> 'pg_database_owner'
       OR to_regclass('public.schema_migrations') IS NOT NULL THEN
        RAISE EXCEPTION 'database/schema ownership or migration baseline changed';
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_catalog.pg_database d,
        LATERAL aclexplode(COALESCE(d.datacl, acldefault('d', d.datdba))) a
        WHERE d.datname='espacigo' AND a.grantee=0 AND a.privilege_type='CONNECT')
       OR NOT EXISTS (SELECT 1 FROM pg_catalog.pg_database d,
        LATERAL aclexplode(COALESCE(d.datacl, acldefault('d', d.datdba))) a
        WHERE d.datname='espacigo' AND a.grantee=0 AND a.privilege_type='TEMPORARY')
       OR NOT EXISTS (SELECT 1 FROM pg_catalog.pg_namespace n,
        LATERAL aclexplode(COALESCE(n.nspacl, acldefault('n', n.nspowner))) a
        WHERE n.nspname='public' AND a.grantee=0 AND a.privilege_type='USAGE')
       OR EXISTS (SELECT 1 FROM pg_catalog.pg_database d,
        LATERAL aclexplode(COALESCE(d.datacl, acldefault('d', d.datdba))) a
        WHERE d.datname='espacigo' AND a.grantee=0 AND a.privilege_type='CREATE')
       OR EXISTS (SELECT 1 FROM pg_catalog.pg_namespace n,
        LATERAL aclexplode(COALESCE(n.nspacl, acldefault('n', n.nspowner))) a
        WHERE n.nspname='public' AND a.grantee=0 AND a.privilege_type='CREATE') THEN
        RAISE EXCEPTION 'PUBLIC ACL baseline changed';
    END IF;
    IF EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname IN
       ('espacigo_schema_owner','espacigo_runtime','espacigo_privacy_cleanup_worker','espacigo_gadm_bootstrap')) THEN
        RAISE EXCEPTION 'one or more runtime groups already exist; reconcile before bootstrap';
    END IF;
END
$preflight$;

CREATE ROLE espacigo_w7_bootstrap_executor NOLOGIN NOSUPERUSER NOCREATEDB CREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT;
GRANT espacigo_w7_bootstrap_executor TO espacigo_gcp04_w7_bootstrap WITH ADMIN FALSE, INHERIT FALSE, SET TRUE;
SET ROLE espacigo_w7_bootstrap_executor;
CREATE ROLE espacigo_schema_owner NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT;
CREATE ROLE espacigo_runtime NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT;
CREATE ROLE espacigo_privacy_cleanup_worker NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT;
CREATE ROLE espacigo_gadm_bootstrap NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT;
COMMENT ON ROLE espacigo_gadm_bootstrap IS
    'NOLOGIN GADM local bootstrap group; temporary isolated login membership only.';
RESET ROLE;
DROP ROLE espacigo_w7_bootstrap_executor;

SET ROLE cloudsqlsuperuser;

CREATE EXTENSION IF NOT EXISTS btree_gist;
CREATE EXTENSION IF NOT EXISTS postgis;
CREATE EXTENSION IF NOT EXISTS unaccent;

DO $extension_owners$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_catalog.pg_extension e
       JOIN pg_catalog.pg_roles r ON r.oid=e.extowner
       WHERE e.extname IN ('btree_gist','postgis','unaccent') AND r.rolname <> 'cloudsqlsuperuser') THEN
        RAISE EXCEPTION 'extension owner differs from stable Cloud SQL role';
    END IF;
END
$extension_owners$;

REVOKE TEMPORARY ON DATABASE espacigo FROM PUBLIC;
GRANT CONNECT ON DATABASE espacigo TO espacigo_schema_owner, espacigo_runtime, espacigo_privacy_cleanup_worker;
GRANT CONNECT ON DATABASE espacigo TO espacigo_schema_owner WITH GRANT OPTION;
GRANT TEMPORARY ON DATABASE espacigo TO espacigo_schema_owner;
GRANT USAGE, CREATE ON SCHEMA public TO espacigo_schema_owner;
GRANT USAGE ON SCHEMA public TO espacigo_schema_owner WITH GRANT OPTION;
GRANT USAGE ON SCHEMA public TO espacigo_runtime, espacigo_privacy_cleanup_worker, espacigo_gadm_bootstrap;
RESET ROLE;

DO $assertions$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_catalog.pg_database d,
        LATERAL aclexplode(COALESCE(d.datacl, acldefault('d', d.datdba))) a
        WHERE d.datname='espacigo' AND a.grantee=0 AND a.privilege_type='TEMPORARY') THEN
        RAISE EXCEPTION 'PUBLIC TEMPORARY revoke did not take effect';
    END IF;
    IF EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname IN
       ('espacigo_schema_owner','espacigo_runtime','espacigo_privacy_cleanup_worker','espacigo_gadm_bootstrap')
       AND (rolcanlogin OR rolsuper OR rolcreatedb OR rolcreaterole OR rolreplication OR rolbypassrls OR rolinherit)) THEN
        RAISE EXCEPTION 'runtime group attributes differ from contract';
    END IF;
    IF shobj_description((SELECT oid FROM pg_catalog.pg_roles WHERE rolname='espacigo_gadm_bootstrap'),'pg_authid') IS DISTINCT FROM
       'NOLOGIN GADM local bootstrap group; temporary isolated login membership only.' THEN
        RAISE EXCEPTION 'GADM role comment differs from V60';
    END IF;
    IF EXISTS (SELECT 1 FROM pg_catalog.pg_auth_members m
       JOIN pg_catalog.pg_roles r ON r.oid=m.roleid
       JOIN pg_catalog.pg_roles u ON u.oid=m.member
       WHERE r.rolname IN ('espacigo_schema_owner','espacigo_runtime','espacigo_privacy_cleanup_worker','espacigo_gadm_bootstrap')
          OR u.rolname IN ('espacigo_schema_owner','espacigo_runtime','espacigo_privacy_cleanup_worker','espacigo_gadm_bootstrap')) THEN
        RAISE EXCEPTION 'bootstrap left an unexpected role membership';
    END IF;
END
$assertions$;
COMMIT;
