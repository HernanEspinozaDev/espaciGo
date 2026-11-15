-- V87: permit the dedicated privacy cleanup role to invoke the bounded V86 purge.
-- The worker receives no direct DELETE privilege on declaration tables.
DO $guard$
DECLARE
    actor text := current_user;
    purge_function oid := pg_catalog.to_regprocedure('public.purge_expired_identity_declaration_revisions()');
    function_owner text;
    runtime_oid oid;
    worker_oid oid;
BEGIN
    IF session_user <> 'espacigo_migrator_login'
       OR actor <> 'espacigo_schema_owner'
       OR current_schema() <> 'public' THEN
        RAISE EXCEPTION 'V87 requires the pinned migrator/schema-owner identity and public search target';
    END IF;

    SELECT oid INTO runtime_oid FROM pg_catalog.pg_roles WHERE rolname = 'espacigo_runtime'
      AND NOT rolcanlogin AND NOT rolsuper AND NOT rolinherit
      AND NOT rolcreaterole AND NOT rolcreatedb AND NOT rolreplication AND NOT rolbypassrls;
    SELECT oid INTO worker_oid FROM pg_catalog.pg_roles WHERE rolname = 'espacigo_privacy_cleanup_worker'
      AND NOT rolcanlogin AND NOT rolsuper AND NOT rolinherit
      AND NOT rolcreaterole AND NOT rolcreatedb AND NOT rolreplication AND NOT rolbypassrls;
    IF runtime_oid IS NULL OR worker_oid IS NULL THEN
        RAISE EXCEPTION 'V87 requires safe runtime and dedicated cleanup-worker roles';
    END IF;
    IF EXISTS (
        SELECT 1
          FROM pg_catalog.pg_auth_members m
          JOIN pg_catalog.pg_roles granted_role ON granted_role.oid = m.roleid
          JOIN pg_catalog.pg_roles member_role ON member_role.oid = m.member
         WHERE (m.roleid = worker_oid OR m.member = worker_oid)
           AND NOT (
               granted_role.rolname = 'espacigo_privacy_cleanup_worker'
               AND member_role.rolname = 'espacigo_privacy_worker_login'
               AND NOT m.admin_option AND NOT m.inherit_option AND m.set_option
           )
    ) OR EXISTS (
        SELECT 1
          FROM pg_catalog.pg_auth_members m
          JOIN pg_catalog.pg_roles granted_role ON granted_role.oid = m.roleid
          JOIN pg_catalog.pg_roles member_role ON member_role.oid = m.member
         WHERE granted_role.rolname = 'espacigo_privacy_cleanup_worker'
           AND member_role.rolname = 'espacigo_privacy_worker_login'
           AND (m.admin_option OR m.inherit_option OR NOT m.set_option)
    ) THEN
        RAISE EXCEPTION 'V87 cleanup-worker membership differs from the reviewed SET ROLE edge';
    END IF;
    IF pg_catalog.has_schema_privilege(worker_oid, 'public', 'CREATE') THEN
        RAISE EXCEPTION 'V87 cleanup-worker role must not create objects in public';
    END IF;
    IF purge_function IS NULL THEN
        RAISE EXCEPTION 'V87 requires the V86 bounded purge function';
    END IF;
    SELECT pg_catalog.pg_get_userbyid(p.proowner) INTO function_owner
      FROM pg_catalog.pg_proc p WHERE p.oid = purge_function;
    IF function_owner IS DISTINCT FROM actor THEN
        RAISE EXCEPTION 'V87 schema owner must own the V86 bounded purge function';
    END IF;
    IF NOT pg_catalog.has_function_privilege(runtime_oid, purge_function, 'EXECUTE') THEN
        RAISE EXCEPTION 'V87 requires the existing runtime EXECUTE grant to remain';
    END IF;
    IF EXISTS (
        SELECT 1
          FROM pg_catalog.pg_proc p
          CROSS JOIN LATERAL pg_catalog.aclexplode(
              COALESCE(p.proacl, pg_catalog.acldefault('f', p.proowner))
          ) acl
         WHERE p.oid = purge_function
           AND acl.grantee <> p.proowner
           AND (acl.grantee <> runtime_oid
                OR acl.privilege_type <> 'EXECUTE'
                OR acl.is_grantable)
    ) THEN
        RAISE EXCEPTION 'V87 found an unexpected preexisting purge-function grant';
    END IF;
END
$guard$;

REVOKE ALL ON FUNCTION public.purge_expired_identity_declaration_revisions() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION public.purge_expired_identity_declaration_revisions()
    TO espacigo_privacy_cleanup_worker;

DO $acl$
DECLARE
    purge_function oid := pg_catalog.to_regprocedure('public.purge_expired_identity_declaration_revisions()');
    runtime_oid oid := pg_catalog.to_regrole('espacigo_runtime');
    worker_oid oid := pg_catalog.to_regrole('espacigo_privacy_cleanup_worker');
    owner_oid oid;
    non_owner_grants integer;
    valid_grants boolean;
    has_runtime boolean;
    has_worker boolean;
BEGIN
    SELECT p.proowner INTO owner_oid FROM pg_catalog.pg_proc p WHERE p.oid = purge_function;
    SELECT count(*),
           COALESCE(bool_and(acl.privilege_type = 'EXECUTE' AND NOT acl.is_grantable), false),
           COALESCE(bool_or(acl.grantee = runtime_oid), false),
           COALESCE(bool_or(acl.grantee = worker_oid), false)
      INTO non_owner_grants, valid_grants, has_runtime, has_worker
      FROM pg_catalog.pg_proc p
      CROSS JOIN LATERAL pg_catalog.aclexplode(p.proacl) acl
     WHERE p.oid = purge_function AND acl.grantee <> owner_oid;
    IF non_owner_grants <> 2 OR NOT valid_grants OR NOT has_runtime OR NOT has_worker THEN
        RAISE EXCEPTION 'V87 purge function grants differ from the runtime/cleanup-worker allowlist';
    END IF;
END
$acl$;
