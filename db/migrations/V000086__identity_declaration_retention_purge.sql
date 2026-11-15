-- V86: bounded deletion of expired encrypted identity declaration revisions.
-- Runtime receives only EXECUTE on a parameterless, fixed-policy purge function.
DO $guard$
DECLARE
    actor text := current_user;
BEGIN
    IF session_user <> 'espacigo_migrator_login' OR actor <> 'espacigo_schema_owner'
       OR NOT pg_catalog.has_schema_privilege(actor, 'public', 'CREATE') THEN
        RAISE EXCEPTION 'V86 requires the pinned migrator/schema-owner role with public CREATE';
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_catalog.pg_roles r
                    WHERE r.rolname = 'espacigo_runtime'
                      AND NOT r.rolcanlogin AND NOT r.rolsuper AND NOT r.rolinherit
                      AND NOT r.rolcreaterole AND NOT r.rolcreatedb
                      AND NOT r.rolreplication AND NOT r.rolbypassrls) THEN
        RAISE EXCEPTION 'V86 runtime role is missing or has unsafe role attributes';
    END IF;
END
$guard$;

CREATE FUNCTION public.purge_expired_identity_declaration_revisions()
RETURNS bigint
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog
AS $$
DECLARE
    purged bigint := 0;
BEGIN
    WITH candidates AS (
        SELECT d.id
          FROM public.identity_declaration_revision AS d
         WHERE d.retention_at <= pg_catalog.clock_timestamp()
           AND NOT EXISTS (
               SELECT 1
                 FROM public.identity_provider_attempt AS a
                WHERE a.declaration_id = d.id
                  AND a.account_id = d.account_id
           )
         ORDER BY d.retention_at, d.id
         LIMIT 100
         FOR UPDATE OF d SKIP LOCKED
    )
    DELETE FROM public.identity_declaration_revision AS d
     USING candidates AS c
     WHERE d.id = c.id;

    GET DIAGNOSTICS purged = ROW_COUNT;
    RETURN purged;
END;
$$;

REVOKE ALL ON FUNCTION public.purge_expired_identity_declaration_revisions() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION public.purge_expired_identity_declaration_revisions() TO espacigo_runtime;

DO $acl$
BEGIN
    IF EXISTS (
        SELECT 1
          FROM pg_catalog.pg_proc AS p
          CROSS JOIN LATERAL pg_catalog.aclexplode(p.proacl) AS acl
          JOIN pg_catalog.pg_roles AS r ON r.oid = acl.grantee
         WHERE p.oid = 'public.purge_expired_identity_declaration_revisions()'::regprocedure
           AND acl.grantee <> p.proowner
           AND r.rolname <> 'espacigo_runtime'
    ) THEN
        RAISE EXCEPTION 'V86 purge function has unexpected role grants';
    END IF;
END
$acl$;
