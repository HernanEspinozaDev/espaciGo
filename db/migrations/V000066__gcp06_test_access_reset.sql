-- GCP-06 candidate-only bulk reset of synthetic imported test credentials.
-- The temporary login can assume this NOLOGIN role, which cannot administer
-- accounts, roles, application data, or read password hashes.
DO $role$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_catalog.pg_roles WHERE rolname = 'espacigo_gcp06_test_access'
    ) THEN
        CREATE ROLE espacigo_gcp06_test_access
            NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT;
    ELSIF EXISTS (
        SELECT 1 FROM pg_catalog.pg_roles
        WHERE rolname = 'espacigo_gcp06_test_access'
          AND (rolcanlogin OR rolsuper OR rolcreatedb OR rolcreaterole OR rolreplication OR rolbypassrls OR rolinherit)
    ) THEN
        RAISE EXCEPTION 'espacigo_gcp06_test_access exists with unsafe role attributes';
    END IF;

    IF EXISTS (
        SELECT 1 FROM pg_catalog.pg_auth_members m
        JOIN pg_catalog.pg_roles r ON r.oid = m.roleid
        JOIN pg_catalog.pg_roles member ON member.oid = m.member
        WHERE r.rolname = 'espacigo_gcp06_test_access' OR member.rolname = 'espacigo_gcp06_test_access'
    ) THEN
        RAISE EXCEPTION 'espacigo_gcp06_test_access must have no role memberships';
    END IF;

    IF EXISTS (
        SELECT 1 FROM pg_catalog.pg_shdepend sd
        WHERE sd.refclassid='pg_catalog.pg_authid'::regclass
          AND sd.refobjid=(SELECT oid FROM pg_catalog.pg_roles WHERE rolname='espacigo_gcp06_test_access')
          AND sd.deptype='o'
    ) OR EXISTS (
        SELECT 1 FROM pg_catalog.pg_class c CROSS JOIN LATERAL pg_catalog.aclexplode(c.relacl) a
        WHERE a.grantee=(SELECT oid FROM pg_catalog.pg_roles WHERE rolname='espacigo_gcp06_test_access')
    ) OR EXISTS (
        SELECT 1 FROM pg_catalog.pg_proc p CROSS JOIN LATERAL pg_catalog.aclexplode(p.proacl) a
        WHERE a.grantee=(SELECT oid FROM pg_catalog.pg_roles WHERE rolname='espacigo_gcp06_test_access')
    ) OR EXISTS (
        SELECT 1 FROM pg_catalog.pg_namespace n CROSS JOIN LATERAL pg_catalog.aclexplode(n.nspacl) a
        WHERE a.grantee=(SELECT oid FROM pg_catalog.pg_roles WHERE rolname='espacigo_gcp06_test_access')
    ) THEN
        RAISE EXCEPTION 'espacigo_gcp06_test_access must not have pre-existing ownership or object grants';
    END IF;
END
$role$;

DO $connect$
BEGIN
    EXECUTE format('GRANT CONNECT ON DATABASE %I TO espacigo_gcp06_test_access', current_database());
END
$connect$;

GRANT USAGE ON SCHEMA public TO espacigo_gcp06_test_access;

CREATE TABLE public.gcp06_test_access_batch_audit (
    id uuid PRIMARY KEY,
    target_count integer NOT NULL CHECK (target_count BETWEEN 1 AND 10000),
    target_set_sha256 char(64) NOT NULL CHECK (target_set_sha256 ~ '^[0-9a-f]{64}$'),
    occurred_at timestamptz NOT NULL
);
COMMENT ON TABLE public.gcp06_test_access_batch_audit IS
    'Aggregate-only append-only evidence for an explicitly confirmed synthetic GCP-06 test-account credential batch; contains no emails, password material, or account UUIDs.';

CREATE FUNCTION public.gcp06_test_access_target_ids() RETURNS SETOF uuid
LANGUAGE sql STABLE SECURITY DEFINER
SET search_path = pg_catalog, public
AS $function$
    SELECT u.id FROM public.usuario u
    WHERE u.correo_normalizado <> 'hernan.espinoza.dev@gmail.com'
      AND u.estado <> 'desidentificado'
    ORDER BY u.id
$function$;

CREATE FUNCTION public.gcp06_test_access_owner_ready() RETURNS boolean
LANGUAGE sql STABLE SECURITY DEFINER
SET search_path = pg_catalog, public
AS $function$
    SELECT EXISTS (
        SELECT 1 FROM public.usuario u
        JOIN public.administrador_global g ON g.usuario_id=u.id
        WHERE u.correo_normalizado='hernan.espinoza.dev@gmail.com'
          AND u.estado='activo'
          AND (u.bloqueado_hasta IS NULL OR u.bloqueado_hasta<=now())
    )
$function$;

REVOKE ALL ON FUNCTION public.gcp06_test_access_target_ids() FROM PUBLIC;
REVOKE ALL ON FUNCTION public.gcp06_test_access_owner_ready() FROM PUBLIC;

CREATE FUNCTION public.gcp06_test_access_operation_exists(p_id uuid) RETURNS boolean
LANGUAGE sql STABLE SECURITY DEFINER
SET search_path = pg_catalog, public
AS $function$
    SELECT EXISTS (SELECT 1 FROM public.gcp06_test_access_batch_audit WHERE id=p_id)
$function$;
REVOKE ALL ON FUNCTION public.gcp06_test_access_operation_exists(uuid) FROM PUBLIC;

GRANT SELECT (id)
    ON public.usuario TO espacigo_gcp06_test_access;
GRANT UPDATE (hash_clave, actualizado_en)
    ON public.usuario TO espacigo_gcp06_test_access;
GRANT SELECT (usuario_id, revocada_en), UPDATE (revocada_en)
    ON public.sesion TO espacigo_gcp06_test_access;
GRANT SELECT (usuario_id, invalidado_en, consumido_en), UPDATE (invalidado_en)
    ON public.token_accion TO espacigo_gcp06_test_access;
GRANT INSERT (id, target_count, target_set_sha256, occurred_at)
    ON public.gcp06_test_access_batch_audit TO espacigo_gcp06_test_access;
GRANT EXECUTE ON FUNCTION public.gcp06_test_access_target_ids() TO espacigo_gcp06_test_access;
GRANT EXECUTE ON FUNCTION public.gcp06_test_access_owner_ready() TO espacigo_gcp06_test_access;
GRANT EXECUTE ON FUNCTION public.gcp06_test_access_operation_exists(uuid) TO espacigo_gcp06_test_access;

REVOKE UPDATE, DELETE, TRUNCATE ON public.gcp06_test_access_batch_audit FROM espacigo_gcp06_test_access;

COMMENT ON ROLE espacigo_gcp06_test_access IS
    'GCP-06 synthetic test credential reset only; temporary one-purpose login membership, no password hash reads or unrelated administration.';
