-- F5-only role bootstrap before restoring the V1-V82 archive.
-- Apply through the authorized runner in one transaction. The runner must
-- reuse the existing api/migrator passfiles unchanged and set SCRAM verifier
-- values in memory in this same transaction; this file contains no passwords.
-- This creates roles/memberships only: no schemas, object ACLs, ownership,
-- ledger or migration changes. pg_restore must restore owners and ACLs.
-- This is preparation only; it is not an archive restore or migration apply.
-- Catalog checks are a preflight, not protection from concurrent superuser changes;
-- isolation/session quiescence is operational and does not mutate ACLs or networking.

SELECT pg_catalog.pg_advisory_xact_lock(1163087943, 1296648018);

DO $f5_restore_target_guard$
BEGIN
    IF current_database() <> 'espacigo_f5_gate_20261101'
       OR current_setting('server_version_num') <> '180006'
       OR session_user <> 'postgres'
       OR current_user <> 'postgres'
       OR NOT EXISTS (
           SELECT 1 FROM pg_catalog.pg_roles
            WHERE rolname = session_user AND rolsuper
       )
       OR inet_server_addr() IS DISTINCT FROM '172.28.0.2'::inet THEN
        RAISE EXCEPTION 'F5 role restore requires the exact PostgreSQL 18.6 target, server address, and postgres superuser identity';
    END IF;

    IF EXISTS (
        SELECT 1 FROM (VALUES ('plpgsql','1.0'),('postgis','3.6.4'),
            ('postgis_topology','3.6.4'),('fuzzystrmatch','1.2'),
            ('postgis_tiger_geocoder','3.6.4')) expected(extname,extversion)
        FULL JOIN pg_catalog.pg_extension e USING (extname)
        WHERE expected.extversion IS DISTINCT FROM e.extversion
    ) OR (SELECT count(*) FROM pg_catalog.pg_extension) <> 5 THEN
        RAISE EXCEPTION 'F5 role restore requires the pinned extension set and versions';
    END IF;

    IF EXISTS (
        SELECT 1 FROM pg_catalog.pg_class c
        JOIN pg_catalog.pg_roles r ON r.oid=c.relowner
        WHERE c.relnamespace='public'::regnamespace
          AND NOT (r.rolname='postgres' AND (
            (c.relname IN ('geometry_dump','valid_detail') AND c.relkind='c' AND EXISTS (
                SELECT 1 FROM pg_type t JOIN pg_depend d ON d.classid='pg_type'::regclass AND d.objid=t.oid
                JOIN pg_extension e ON e.oid=d.refobjid
                WHERE t.typrelid=c.oid AND d.refclassid='pg_extension'::regclass
                  AND d.deptype='e' AND e.extname='postgis')) OR
            (c.relname='spatial_ref_sys' AND c.relkind='r' AND EXISTS (
                SELECT 1 FROM pg_depend d JOIN pg_extension e ON e.oid=d.refobjid
                WHERE d.classid='pg_class'::regclass AND d.objid=c.oid
                  AND d.refclassid='pg_extension'::regclass AND d.deptype='e' AND e.extname='postgis')) OR
            (c.relname='spatial_ref_sys_pkey' AND c.relkind='i' AND EXISTS (
                SELECT 1 FROM pg_index i JOIN pg_depend d ON d.classid='pg_class'::regclass AND d.objid=i.indrelid
                JOIN pg_extension e ON e.oid=d.refobjid
                WHERE i.indexrelid=c.oid AND i.indisprimary
                  AND d.refclassid='pg_extension'::regclass AND d.deptype='e' AND e.extname='postgis')) OR
            (c.relname IN ('geometry_columns','geography_columns') AND c.relkind='v' AND EXISTS (
                SELECT 1 FROM pg_depend d JOIN pg_extension e ON e.oid=d.refobjid
                WHERE d.classid='pg_class'::regclass AND d.objid=c.oid
                  AND d.refclassid='pg_extension'::regclass AND d.deptype='e' AND e.extname='postgis'))
          ))
    ) OR (SELECT count(*) FROM pg_catalog.pg_class WHERE relnamespace='public'::regnamespace) <> 6
      OR EXISTS (SELECT 1 FROM pg_catalog.pg_proc p WHERE p.pronamespace='public'::regnamespace AND NOT EXISTS (SELECT 1 FROM pg_depend d WHERE d.classid='pg_proc'::regclass AND d.objid=p.oid AND d.refclassid='pg_extension'::regclass AND d.deptype='e'))
      OR EXISTS (SELECT 1 FROM pg_catalog.pg_type t WHERE t.typnamespace='public'::regnamespace AND t.typtype IN ('b','c') AND t.typisdefined AND NOT EXISTS (SELECT 1 FROM pg_depend d WHERE d.classid='pg_type'::regclass AND d.objid=t.oid AND d.refclassid='pg_extension'::regclass AND d.deptype='e'))
      OR EXISTS (SELECT 1 FROM pg_catalog.pg_operator o WHERE o.oprnamespace='public'::regnamespace AND NOT EXISTS (SELECT 1 FROM pg_depend d WHERE d.classid='pg_operator'::regclass AND d.objid=o.oid AND d.refclassid='pg_extension'::regclass AND d.deptype='e'))
      OR EXISTS (SELECT 1 FROM pg_catalog.pg_default_acl)
      OR EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE starts_with(rolname,'espacigo_')) THEN
        RAISE EXCEPTION 'F5 role restore refuses non-extension public objects, application roles or default ACLs';
    END IF;

    IF to_regclass('public.schema_migrations') IS NOT NULL
       OR to_regclass('public.usuario') IS NOT NULL
       OR to_regclass('public.espacio') IS NOT NULL THEN
        RAISE EXCEPTION 'F5 role restore requires the empty pre-restore ledger and application relations';
    END IF;

    -- Extension namespaces are identified by extnamespace, not extension
    -- membership: PostgreSQL can install an extension into an existing schema.
    IF EXISTS (
        SELECT 1
          FROM pg_catalog.pg_namespace
         WHERE nspname NOT IN ('pg_catalog', 'information_schema', 'public')
           AND nspname NOT LIKE 'pg_toast%'
           AND nspname NOT LIKE 'pg_temp_%'
           AND NOT (nspname='tiger' AND nspowner='postgres'::regrole AND EXISTS (SELECT 1 FROM pg_extension e JOIN pg_namespace en ON en.oid=e.extnamespace WHERE e.extname='postgis_tiger_geocoder' AND en.oid=pg_namespace.oid))
           AND NOT (nspname='topology' AND nspowner='postgres'::regrole AND EXISTS (SELECT 1 FROM pg_extension e JOIN pg_namespace en ON en.oid=e.extnamespace WHERE e.extname='postgis_topology' AND en.oid=pg_namespace.oid))
    ) THEN
        RAISE EXCEPTION 'F5 role restore refuses pre-existing application schemas';
    END IF;

    IF EXISTS (
        SELECT 1
          FROM pg_catalog.pg_roles
         WHERE rolname = ANY (ARRAY[
             'espacigo_runtime', 'espacigo_api_login', 'espacigo_schema_owner',
             'espacigo_migrator_login', 'espacigo_document_verification_fn',
             'espacigo_privacy_cleanup_worker'
         ])
    ) THEN
        RAISE EXCEPTION 'F5 role restore requires all six contract roles to be absent';
    END IF;

    IF EXISTS (
        SELECT 1
          FROM pg_catalog.pg_stat_activity
         WHERE pid <> pg_catalog.pg_backend_pid()
           AND backend_type = 'client backend'
    ) THEN
        RAISE EXCEPTION 'F5 role restore requires no other client backend sessions';
    END IF;
END;
$f5_restore_target_guard$;

CREATE ROLE espacigo_runtime
    NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT;
CREATE ROLE espacigo_api_login
    LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT;
CREATE ROLE espacigo_schema_owner
    NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT;
CREATE ROLE espacigo_migrator_login
    LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT;
CREATE ROLE espacigo_document_verification_fn
    NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT;
CREATE ROLE espacigo_privacy_cleanup_worker
    NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT;

GRANT espacigo_runtime TO espacigo_api_login
    WITH ADMIN FALSE, INHERIT FALSE, SET TRUE;
GRANT espacigo_schema_owner TO espacigo_migrator_login
    WITH ADMIN FALSE, INHERIT FALSE, SET TRUE;
GRANT espacigo_document_verification_fn TO espacigo_schema_owner
    WITH ADMIN FALSE, INHERIT FALSE, SET TRUE;

DO $f5_restore_role_postcheck$
DECLARE
    bad_roles bigint;
    bad_memberships bigint;
    contract_memberships bigint;
BEGIN
    SELECT count(*)
      INTO bad_roles
      FROM (VALUES
          ('espacigo_runtime', false),
          ('espacigo_api_login', true),
          ('espacigo_schema_owner', false),
          ('espacigo_migrator_login', true),
          ('espacigo_document_verification_fn', false),
          ('espacigo_privacy_cleanup_worker', false)
      ) AS expected(rolname, can_login)
      LEFT JOIN pg_catalog.pg_roles r USING (rolname)
     WHERE r.oid IS NULL
        OR r.rolcanlogin IS DISTINCT FROM expected.can_login
        OR r.rolsuper
        OR r.rolcreatedb
        OR r.rolcreaterole
        OR r.rolreplication
        OR r.rolbypassrls
        OR r.rolinherit;

    SELECT count(*)
      INTO bad_memberships
      FROM (
          SELECT granted.rolname AS role_name, member.rolname AS member_name,
                 m.admin_option, m.inherit_option, m.set_option,
                 m.grantor = 'postgres'::regrole AS expected_grantor
            FROM pg_catalog.pg_auth_members m
            JOIN pg_catalog.pg_roles granted ON granted.oid = m.roleid
            JOIN pg_catalog.pg_roles member ON member.oid = m.member
           WHERE granted.rolname IN (
                     'espacigo_runtime', 'espacigo_api_login', 'espacigo_schema_owner',
                     'espacigo_migrator_login', 'espacigo_document_verification_fn',
                     'espacigo_privacy_cleanup_worker'
                 )
              OR member.rolname IN (
                     'espacigo_runtime', 'espacigo_api_login', 'espacigo_schema_owner',
                     'espacigo_migrator_login', 'espacigo_document_verification_fn',
                     'espacigo_privacy_cleanup_worker'
                 )
      ) actual
     WHERE NOT (expected_grantor AND (
         (role_name = 'espacigo_runtime' AND member_name = 'espacigo_api_login'
             AND NOT admin_option AND NOT inherit_option AND set_option)
         OR (role_name = 'espacigo_schema_owner' AND member_name = 'espacigo_migrator_login'
             AND NOT admin_option AND NOT inherit_option AND set_option)
         OR (role_name = 'espacigo_document_verification_fn' AND member_name = 'espacigo_schema_owner'
             AND NOT admin_option AND NOT inherit_option AND set_option)
     ));

    SELECT count(*)
      INTO contract_memberships
      FROM pg_catalog.pg_auth_members m
      JOIN pg_catalog.pg_roles granted ON granted.oid = m.roleid
      JOIN pg_catalog.pg_roles member ON member.oid = m.member
     WHERE NOT m.admin_option AND NOT m.inherit_option AND m.set_option
       AND m.grantor = 'postgres'::regrole
       AND ((granted.rolname = 'espacigo_runtime' AND member.rolname = 'espacigo_api_login')
         OR (granted.rolname = 'espacigo_schema_owner' AND member.rolname = 'espacigo_migrator_login')
         OR (granted.rolname = 'espacigo_document_verification_fn' AND member.rolname = 'espacigo_schema_owner'));

    IF bad_roles <> 0 OR bad_memberships <> 0 OR contract_memberships <> 3 THEN
        RAISE EXCEPTION 'F5 role restore postcheck found role flags or membership edges outside the exact contract';
    END IF;
END;
$f5_restore_role_postcheck$;
