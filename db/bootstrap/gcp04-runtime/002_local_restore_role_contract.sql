-- LOCAL RESTORE TEST ONLY. Creates global roles outside the pg_dump artifact.
-- Never apply this file to Cloud SQL or a persistent database.
\set ON_ERROR_STOP on

BEGIN;

DO $roles$
DECLARE
    role_name text;
    role_row record;
BEGIN
    IF current_database() <> 'espacigo' THEN
        RAISE EXCEPTION 'refusing local restore role contract in unexpected database %', current_database();
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
            RAISE EXCEPTION 'role % has unexpected global attributes', role_name;
        END IF;
    END LOOP;

    FOREACH role_name IN ARRAY ARRAY[
        'espacigo_api_login',
        'espacigo_privacy_worker_login',
        'espacigo_migrator_login',
        'espacigo_observer_login'
    ] LOOP
        SELECT * INTO role_row FROM pg_catalog.pg_roles WHERE rolname = role_name;
        IF NOT FOUND THEN
            EXECUTE format('CREATE ROLE %I LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS INHERIT', role_name);
        ELSIF NOT role_row.rolcanlogin OR role_row.rolsuper OR role_row.rolcreatedb OR role_row.rolcreaterole
           OR role_row.rolreplication OR role_row.rolbypassrls OR NOT role_row.rolinherit THEN
            RAISE EXCEPTION 'login % has unexpected global attributes', role_name;
        END IF;
    END LOOP;
END
$roles$;

GRANT CONNECT ON DATABASE espacigo TO espacigo_schema_owner WITH GRANT OPTION;
GRANT CONNECT ON DATABASE espacigo TO espacigo_runtime, espacigo_privacy_cleanup_worker, espacigo_db_observer;
GRANT CONNECT ON DATABASE espacigo TO espacigo_gadm_bootstrap;
GRANT TEMP ON DATABASE espacigo TO espacigo_schema_owner;
REVOKE TEMP ON DATABASE espacigo FROM PUBLIC;
GRANT USAGE, CREATE ON SCHEMA public TO espacigo_schema_owner;
GRANT USAGE ON SCHEMA public TO espacigo_schema_owner WITH GRANT OPTION;
GRANT USAGE ON SCHEMA public TO espacigo_runtime, espacigo_privacy_cleanup_worker, espacigo_db_observer, espacigo_gadm_bootstrap;
REVOKE CREATE ON SCHEMA public FROM PUBLIC;

GRANT espacigo_runtime TO espacigo_api_login WITH ADMIN FALSE, INHERIT FALSE, SET TRUE;
GRANT espacigo_privacy_cleanup_worker TO espacigo_privacy_worker_login WITH ADMIN FALSE, INHERIT FALSE, SET TRUE;
GRANT espacigo_schema_owner TO espacigo_migrator_login WITH ADMIN FALSE, INHERIT FALSE, SET TRUE;
GRANT espacigo_db_observer TO espacigo_observer_login WITH ADMIN FALSE, INHERIT TRUE, SET FALSE;

DO $assertions$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM pg_catalog.pg_auth_members m
        JOIN pg_catalog.pg_roles granted_role ON granted_role.oid = m.roleid
        JOIN pg_catalog.pg_roles member_role ON member_role.oid = m.member
        WHERE granted_role.rolname = 'espacigo_gadm_bootstrap'
           OR member_role.rolname = 'espacigo_gadm_bootstrap'
           OR m.admin_option
    ) THEN
        RAISE EXCEPTION 'local role contract contains an ADMIN OPTION or GADM membership';
    END IF;
END
$assertions$;

COMMIT;
