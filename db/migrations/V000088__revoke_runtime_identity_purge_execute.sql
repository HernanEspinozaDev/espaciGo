-- V88: retain identity-retention purge EXECUTE only for the dedicated worker.
-- This migration changes only the exact V86 purge-function ACL.
DO $guard$
DECLARE
    actor text := current_user;
    purge_function oid := pg_catalog.to_regprocedure('public.purge_expired_identity_declaration_revisions()');
    function_owner oid;
    runtime_oid oid;
    worker_oid oid;
    grant_count integer;
    grants_valid boolean;
    runtime_grant boolean;
    worker_grant boolean;
    public_grant boolean;
BEGIN
    IF session_user <> 'espacigo_migrator_login' OR actor <> 'espacigo_schema_owner'
       OR current_schema() <> 'public'
       OR NOT pg_catalog.has_schema_privilege(actor, 'public', 'CREATE') THEN
        RAISE EXCEPTION 'V88 requires the pinned migrator/schema-owner identity and public CREATE';
    END IF;
    SELECT oid INTO runtime_oid FROM pg_catalog.pg_roles WHERE rolname='espacigo_runtime'
      AND NOT rolcanlogin AND NOT rolsuper AND NOT rolinherit AND NOT rolcreaterole
      AND NOT rolcreatedb AND NOT rolreplication AND NOT rolbypassrls;
    SELECT oid INTO worker_oid FROM pg_catalog.pg_roles WHERE rolname='espacigo_privacy_cleanup_worker'
      AND NOT rolcanlogin AND NOT rolsuper AND NOT rolinherit AND NOT rolcreaterole
      AND NOT rolcreatedb AND NOT rolreplication AND NOT rolbypassrls;
    IF runtime_oid IS NULL OR worker_oid IS NULL OR purge_function IS NULL THEN
        RAISE EXCEPTION 'V88 requires safe runtime/worker roles and the V86 purge function';
    END IF;
    SELECT p.proowner INTO function_owner FROM pg_catalog.pg_proc p WHERE p.oid=purge_function;
    IF function_owner <> (SELECT oid FROM pg_catalog.pg_roles WHERE rolname=actor) THEN
        RAISE EXCEPTION 'V88 requires schema owner to own the V86 purge function';
    END IF;
    SELECT count(*),
           COALESCE(bool_and(acl.privilege_type='EXECUTE' AND NOT acl.is_grantable),false),
           COALESCE(bool_or(acl.grantee=runtime_oid),false),
           COALESCE(bool_or(acl.grantee=worker_oid),false),
           COALESCE(bool_or(acl.grantee=0),false)
      INTO grant_count, grants_valid, runtime_grant, worker_grant, public_grant
      FROM pg_catalog.pg_proc p
      CROSS JOIN LATERAL pg_catalog.aclexplode(p.proacl) acl
     WHERE p.oid=purge_function AND acl.grantee<>function_owner;
    IF grant_count <> 2 OR NOT grants_valid OR NOT runtime_grant OR NOT worker_grant OR public_grant THEN
        RAISE EXCEPTION 'V88 requires exactly the V87 runtime and dedicated-worker EXECUTE grants, with no PUBLIC grant';
    END IF;
END
$guard$;

REVOKE EXECUTE ON FUNCTION public.purge_expired_identity_declaration_revisions() FROM espacigo_runtime;

DO $verify$
DECLARE
    purge_function oid := pg_catalog.to_regprocedure('public.purge_expired_identity_declaration_revisions()');
    runtime_oid oid := pg_catalog.to_regrole('espacigo_runtime');
    worker_oid oid := pg_catalog.to_regrole('espacigo_privacy_cleanup_worker');
    owner_oid oid;
    grant_count integer;
    grants_valid boolean;
    runtime_grant boolean;
    worker_grant boolean;
    public_grant boolean;
BEGIN
    SELECT p.proowner INTO owner_oid FROM pg_catalog.pg_proc p WHERE p.oid=purge_function;
    SELECT count(*),
           COALESCE(bool_and(acl.privilege_type='EXECUTE' AND NOT acl.is_grantable),false),
           COALESCE(bool_or(acl.grantee=runtime_oid),false),
           COALESCE(bool_or(acl.grantee=worker_oid),false),
           COALESCE(bool_or(acl.grantee=0),false)
      INTO grant_count, grants_valid, runtime_grant, worker_grant, public_grant
      FROM pg_catalog.pg_proc p
      CROSS JOIN LATERAL pg_catalog.aclexplode(p.proacl) acl
     WHERE p.oid=purge_function AND acl.grantee<>owner_oid;
    IF grant_count<>1 OR NOT grants_valid OR runtime_grant OR NOT worker_grant OR public_grant
       OR pg_catalog.has_function_privilege(runtime_oid,purge_function,'EXECUTE')
       OR NOT pg_catalog.has_function_privilege(worker_oid,purge_function,'EXECUTE') THEN
        RAISE EXCEPTION 'V88 purge-function ACL readback differs from the worker-only contract';
    END IF;
END
$verify$;
