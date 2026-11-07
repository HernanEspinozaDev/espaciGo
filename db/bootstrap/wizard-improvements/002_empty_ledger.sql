-- One-time empty-ledger preparation for espacigo_wizard_20261106 only.
-- Execute with psql -X -v ON_ERROR_STOP=1 -f; this file owns its transaction.
BEGIN;
-- Match cmd/dbmigrate's transaction-scoped lock: ESPG / MIGR.
SELECT pg_catalog.pg_advisory_xact_lock(1163087943, 1296648018);
DO $guard$
DECLARE
  owner_oid oid;
  ledger_oid oid;
BEGIN
  IF current_database() <> 'espacigo_wizard_20261106'
     OR session_user <> 'postgres'
     OR current_user <> 'postgres'
     OR current_setting('server_version_num')::integer <> 180006 THEN
    RAISE EXCEPTION 'wizard empty-ledger target identity mismatch';
  END IF;

  SELECT oid INTO STRICT owner_oid FROM pg_catalog.pg_roles
    WHERE rolname = 'espacigo_schema_owner';
  IF NOT EXISTS (SELECT 1 FROM pg_catalog.pg_roles
                 WHERE rolname='postgres' AND rolsuper AND rolcanlogin)
     OR EXISTS (SELECT 1 FROM pg_catalog.pg_roles
                WHERE rolname IN ('espacigo_schema_owner','espacigo_runtime',
                  'espacigo_migrator_login','espacigo_api_login')
                  AND (rolsuper OR rolcreatedb OR rolcreaterole OR rolreplication
                       OR rolbypassrls OR rolinherit
                       OR (rolname IN ('espacigo_schema_owner','espacigo_runtime') AND rolcanlogin)
                       OR (rolname IN ('espacigo_migrator_login','espacigo_api_login') AND NOT rolcanlogin))) THEN
    RAISE EXCEPTION 'wizard role attributes mismatch';
  END IF;
  IF (SELECT count(*) FROM pg_catalog.pg_roles
      WHERE rolname IN ('espacigo_schema_owner','espacigo_runtime','espacigo_gadm_bootstrap',
        'espacigo_privacy_cleanup_worker','espacigo_gcp06_test_access','espacigo_migrator_login','espacigo_api_login')) <> 7
     OR EXISTS (SELECT 1 FROM pg_catalog.pg_roles
       WHERE rolname IN ('espacigo_schema_owner','espacigo_runtime','espacigo_gadm_bootstrap',
         'espacigo_privacy_cleanup_worker','espacigo_gcp06_test_access','espacigo_migrator_login','espacigo_api_login')
         AND (rolsuper OR rolcreatedb OR rolcreaterole OR rolreplication OR rolbypassrls OR rolinherit
           OR (rolname IN ('espacigo_schema_owner','espacigo_runtime','espacigo_gadm_bootstrap','espacigo_privacy_cleanup_worker','espacigo_gcp06_test_access') AND rolcanlogin)
           OR (rolname IN ('espacigo_migrator_login','espacigo_api_login') AND NOT rolcanlogin)))
     OR (SELECT count(*) FROM pg_catalog.pg_auth_members m
         JOIN pg_catalog.pg_roles g ON g.oid=m.roleid
         JOIN pg_catalog.pg_roles u ON u.oid=m.member
         WHERE (g.rolname='espacigo_schema_owner' AND u.rolname='espacigo_migrator_login'
                AND NOT m.admin_option AND NOT m.inherit_option AND m.set_option)
            OR (g.rolname='espacigo_runtime' AND u.rolname='espacigo_api_login'
                AND NOT m.admin_option AND NOT m.inherit_option AND m.set_option)) <> 2
     OR EXISTS (SELECT 1 FROM pg_catalog.pg_auth_members m
         JOIN pg_catalog.pg_roles g ON g.oid=m.roleid
         JOIN pg_catalog.pg_roles u ON u.oid=m.member
         WHERE (g.rolname IN ('espacigo_schema_owner','espacigo_runtime','espacigo_gadm_bootstrap','espacigo_privacy_cleanup_worker','espacigo_gcp06_test_access','espacigo_migrator_login','espacigo_api_login')
             OR u.rolname IN ('espacigo_schema_owner','espacigo_runtime','espacigo_gadm_bootstrap','espacigo_privacy_cleanup_worker','espacigo_gcp06_test_access','espacigo_migrator_login','espacigo_api_login'))
           AND NOT ((g.rolname='espacigo_schema_owner' AND u.rolname='espacigo_migrator_login'
                     AND NOT m.admin_option AND NOT m.inherit_option AND m.set_option)
                 OR (g.rolname='espacigo_runtime' AND u.rolname='espacigo_api_login'
                     AND NOT m.admin_option AND NOT m.inherit_option AND m.set_option))) THEN
    RAISE EXCEPTION 'wizard role membership mismatch';
  END IF;
  IF EXISTS (SELECT 1 FROM pg_catalog.pg_namespace n
             WHERE n.nspname='public' AND n.nspowner <> (SELECT oid FROM pg_catalog.pg_roles WHERE rolname='pg_database_owner')) THEN
    RAISE EXCEPTION 'public schema owner mismatch';
  END IF;
  IF EXISTS (SELECT 1 FROM pg_catalog.pg_class c
             JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
             WHERE n.nspname='public' AND c.relkind IN ('r','p','v','m','S','f')
               AND c.relname <> 'schema_migrations'
               AND NOT EXISTS (SELECT 1 FROM pg_catalog.pg_depend d
                 WHERE d.classid='pg_catalog.pg_class'::regclass AND d.objid=c.oid
                   AND d.refclassid='pg_catalog.pg_extension'::regclass AND d.deptype='e')) THEN
    RAISE EXCEPTION 'application objects already exist';
  END IF;
  IF EXISTS (SELECT 1 FROM pg_catalog.pg_type t
             JOIN pg_catalog.pg_namespace n ON n.oid=t.typnamespace
             WHERE n.nspname='public' AND t.typtype <> 'p'
               AND NOT EXISTS (SELECT 1 FROM pg_catalog.pg_depend d
                 WHERE d.classid='pg_catalog.pg_type'::regclass AND d.objid=t.oid
                   AND d.refclassid='pg_catalog.pg_extension'::regclass AND d.deptype='e')
               AND NOT (t.typtype='c' AND EXISTS (SELECT 1 FROM pg_catalog.pg_depend d
                 WHERE d.classid='pg_catalog.pg_class'::regclass AND d.objid=t.typrelid
                   AND d.refclassid='pg_catalog.pg_extension'::regclass AND d.deptype='e'))
               AND NOT (t.typrelid=0 AND t.typtype='b' AND t.typelem<>0 AND EXISTS (
                 SELECT 1 FROM pg_catalog.pg_type base WHERE base.oid=t.typelem
                   AND base.typelem=0 AND EXISTS (SELECT 1 FROM pg_catalog.pg_depend d
                     WHERE d.classid='pg_catalog.pg_type'::regclass AND d.objid=base.oid
                       AND d.refclassid='pg_catalog.pg_extension'::regclass AND d.deptype='e')))) THEN
    RAISE EXCEPTION 'application types already exist';
  END IF;
  IF to_regclass('public.schema_migrations') IS NOT NULL THEN
    RAISE EXCEPTION 'migration ledger already exists';
  END IF;
  IF EXISTS (SELECT 1 FROM pg_catalog.pg_default_acl WHERE defaclnamespace IN
             (0, (SELECT oid FROM pg_catalog.pg_namespace WHERE nspname='public'))) THEN
    RAISE EXCEPTION 'unexpected default ACLs';
  END IF;
  IF EXISTS (SELECT 1 FROM pg_catalog.pg_namespace n
             CROSS JOIN LATERAL aclexplode(COALESCE(n.nspacl, acldefault('n',n.nspowner))) a
             WHERE n.nspname='public' AND a.grantee <> n.nspowner
               AND NOT (a.grantee=0 AND a.privilege_type='USAGE' AND NOT a.is_grantable)
               AND a.grantee NOT IN (owner_oid,
                 (SELECT oid FROM pg_catalog.pg_roles WHERE rolname='espacigo_runtime'))) THEN
    RAISE EXCEPTION 'unexpected public schema ACL grantee';
  END IF;
END $guard$;

CREATE TABLE public.schema_migrations (
  version bigint PRIMARY KEY CHECK (version > 0),
  name text NOT NULL UNIQUE,
  checksum text NOT NULL CHECK (checksum ~ '^[0-9a-f]{64}$'),
  runner_checksum text NOT NULL CHECK (runner_checksum ~ '^[0-9a-f]{64}$'),
  applied_at timestamptz NOT NULL DEFAULT clock_timestamp()
);

REVOKE ALL ON TABLE public.schema_migrations FROM PUBLIC, espacigo_schema_owner,
  espacigo_runtime, espacigo_migrator_login, espacigo_api_login;
GRANT SELECT, INSERT ON TABLE public.schema_migrations TO espacigo_schema_owner;

DO $verify$
DECLARE
  ledger_oid oid;
  owner_oid oid;
  actual_acl text[];
  owner_acl text[];
BEGIN
  SELECT c.oid, c.relowner INTO STRICT ledger_oid, owner_oid
    FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
    WHERE n.nspname='public' AND c.relname='schema_migrations' AND c.relkind='r';
  IF owner_oid <> (SELECT oid FROM pg_catalog.pg_roles WHERE rolname='postgres')
     OR (SELECT count(*) FROM pg_catalog.pg_attribute WHERE attrelid=ledger_oid AND attnum>0 AND NOT attisdropped) <> 5
     OR EXISTS (SELECT 1 FROM public.schema_migrations) THEN
    RAISE EXCEPTION 'ledger structure, owner, or emptiness mismatch';
  END IF;
  SELECT array_agg(r.rolname || ':' || a.privilege_type || ':' || a.is_grantable ORDER BY r.rolname,a.privilege_type)
    INTO actual_acl
    FROM pg_catalog.pg_class c CROSS JOIN LATERAL aclexplode(c.relacl) a
    JOIN pg_catalog.pg_roles r ON r.oid=a.grantee
    WHERE c.oid=ledger_oid AND a.grantee <> owner_oid;
  SELECT array_agg(a.privilege_type || ':' || a.is_grantable ORDER BY a.privilege_type)
    INTO owner_acl
    FROM aclexplode(acldefault('r',owner_oid)) a;
  IF actual_acl IS DISTINCT FROM ARRAY['espacigo_schema_owner:INSERT:false','espacigo_schema_owner:SELECT:false']::text[]
     OR (SELECT array_agg(a.privilege_type || ':' || a.is_grantable ORDER BY a.privilege_type)
         FROM pg_catalog.pg_class c CROSS JOIN LATERAL aclexplode(c.relacl) a
         WHERE c.oid=ledger_oid AND a.grantee=owner_oid) IS DISTINCT FROM owner_acl
     OR EXISTS (SELECT 1 FROM pg_catalog.pg_class c CROSS JOIN LATERAL aclexplode(c.relacl) a
                WHERE c.oid=ledger_oid AND (a.grantee=0 OR a.grantor <> owner_oid OR a.is_grantable)) THEN
    RAISE EXCEPTION 'ledger ACL mismatch';
  END IF;
END $verify$;
COMMIT;
