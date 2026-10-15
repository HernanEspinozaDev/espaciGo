-- GADM local activation separates short-lived login credentials from the
-- narrowly scoped role used by the audited bootstrap CLI.
DO $role$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_catalog.pg_roles WHERE rolname = 'espacigo_gadm_bootstrap'
    ) THEN
        CREATE ROLE espacigo_gadm_bootstrap
            NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT;
    ELSIF EXISTS (
        SELECT 1 FROM pg_catalog.pg_roles
        WHERE rolname = 'espacigo_gadm_bootstrap'
          AND (rolcanlogin OR rolsuper OR rolcreatedb OR rolcreaterole OR rolreplication OR rolbypassrls OR rolinherit)
    ) THEN
        RAISE EXCEPTION 'espacigo_gadm_bootstrap exists with unsafe role attributes';
    END IF;

    IF EXISTS (
        SELECT 1 FROM pg_catalog.pg_auth_members m
        JOIN pg_catalog.pg_roles r ON r.oid = m.roleid
        JOIN pg_catalog.pg_roles member ON member.oid = m.member
        WHERE r.rolname = 'espacigo_gadm_bootstrap' OR member.rolname = 'espacigo_gadm_bootstrap'
    ) THEN
        RAISE EXCEPTION 'espacigo_gadm_bootstrap must not have role memberships';
    END IF;
END
$role$;

DO $connect$
BEGIN
    EXECUTE format('GRANT CONNECT ON DATABASE %I TO espacigo_gadm_bootstrap', current_database());
END
$connect$;

GRANT USAGE ON SCHEMA public TO espacigo_gadm_bootstrap;

-- UPDATE(id/singleton) is granted only because PostgreSQL requires an UPDATE
-- privilege for SELECT ... FOR UPDATE. The CLI never updates these columns;
-- their key/check constraints prevent changing the bootstrap identity/anchor.
GRANT SELECT (id, estado, bloqueado_hasta), UPDATE (id)
    ON public.usuario TO espacigo_gadm_bootstrap;
GRANT SELECT (cuenta_id)
    ON public.bloqueo_cuenta_administrativo_local TO espacigo_gadm_bootstrap;
GRANT SELECT (singleton, usuario_id), INSERT (singleton, usuario_id, creado_en), UPDATE (singleton)
    ON public.administrador_global_bootstrap TO espacigo_gadm_bootstrap;
GRANT SELECT (usuario_id), INSERT (usuario_id, concedido_en, motivo_codigo)
    ON public.administrador_global TO espacigo_gadm_bootstrap;
GRANT INSERT (id, usuario_objetivo_id, actor_tipo, accion, resultado, motivo_codigo, correlacion_id, ocurrida_en)
    ON public.administrador_global_bootstrap_evento TO espacigo_gadm_bootstrap;

-- UPDATE(id) exists solely for SELECT ... FOR UPDATE. Prevent that lock
-- privilege from becoming an account-identity mutation path.
CREATE FUNCTION public.reject_gadm_bootstrap_identity_change() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF current_user = 'espacigo_gadm_bootstrap' AND NEW.id IS DISTINCT FROM OLD.id THEN
        RAISE EXCEPTION 'GADM bootstrap lock role cannot change account identity'
            USING ERRCODE = '42501';
    END IF;
    RETURN NEW;
END
$$;
CREATE TRIGGER usuario_gadm_bootstrap_identity_immutable
    BEFORE UPDATE OF id ON public.usuario
    FOR EACH ROW EXECUTE FUNCTION public.reject_gadm_bootstrap_identity_change();

COMMENT ON ROLE espacigo_gadm_bootstrap IS
    'NOLOGIN GADM local bootstrap group; temporary isolated login membership only.';
