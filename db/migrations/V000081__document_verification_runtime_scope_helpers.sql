-- F5: expose scoped authoritative-row checks through a dedicated, restricted owner.
CREATE ROLE espacigo_document_verification_fn NOLOGIN NOSUPERUSER NOCREATEROLE NOCREATEDB NOREPLICATION NOBYPASSRLS NOINHERIT;
GRANT USAGE ON SCHEMA public TO espacigo_document_verification_fn;
GRANT SELECT(id,estado) ON public.usuario TO espacigo_document_verification_fn;
GRANT UPDATE(id) ON public.usuario TO espacigo_document_verification_fn;
GRANT SELECT(id,propietario_id) ON public.espacio TO espacigo_document_verification_fn;
GRANT UPDATE(id) ON public.espacio TO espacigo_document_verification_fn;
REVOKE espacigo_document_verification_fn FROM espacigo_runtime;
CREATE FUNCTION public.document_verification_lock_active_actor(p_actor_id uuid)
RETURNS boolean
LANGUAGE plpgsql
VOLATILE
SECURITY DEFINER
SET search_path = pg_catalog, pg_temp
AS $function$
DECLARE
    active boolean;
BEGIN
    SELECT u.estado = 'activo'
      INTO active
      FROM public.usuario AS u
     WHERE u.id = p_actor_id
     -- KEY SHARE is acquired by the owner foreign key during transfer. This
     -- still excludes concurrent actor updates but avoids actor→space / space→actor deadlocks.
     FOR NO KEY UPDATE;
    RETURN COALESCE(active, false);
END;
$function$;

CREATE FUNCTION public.document_verification_lock_space_owner(p_space_id uuid, p_expected_owner_id uuid)
RETURNS boolean
LANGUAGE plpgsql
VOLATILE
SECURITY DEFINER
SET search_path = pg_catalog, pg_temp
AS $function$
DECLARE
    current_owner_id uuid;
BEGIN
    SELECT e.propietario_id
      INTO current_owner_id
      FROM public.espacio AS e
     WHERE e.id = p_space_id
     FOR UPDATE;
    IF NOT FOUND THEN
        RETURN false;
    END IF;
    RETURN current_owner_id = p_expected_owner_id;
END;
$function$;

CREATE FUNCTION public.document_verification_space_owned_by(p_space_id uuid, p_expected_owner_id uuid)
RETURNS boolean
LANGUAGE plpgsql
VOLATILE
SECURITY DEFINER
SET search_path = pg_catalog, pg_temp
AS $function$
DECLARE
    current_owner_id uuid;
BEGIN
    -- The caller retains this row lock through its document read, serializing
    -- owner-scoped visibility with the ownership-change trigger.
    SELECT e.propietario_id
      INTO current_owner_id
      FROM public.espacio AS e
     WHERE e.id = p_space_id
     FOR UPDATE;
    IF NOT FOUND THEN
        RETURN false;
    END IF;
    RETURN current_owner_id = p_expected_owner_id;
END;
$function$;

CREATE FUNCTION public.document_verification_lock_space_row(p_space_id uuid)
RETURNS boolean
LANGUAGE plpgsql
VOLATILE
SECURITY DEFINER
SET search_path = pg_catalog, pg_temp
AS $function$
BEGIN
    PERFORM 1
      FROM public.espacio AS e
     WHERE e.id = p_space_id
     FOR UPDATE;
    RETURN FOUND;
END;
$function$;

-- CREATE is required during ownership transfer; do not retain schema creation rights.
GRANT CREATE ON SCHEMA public TO espacigo_document_verification_fn;
ALTER FUNCTION public.document_verification_lock_active_actor(uuid) OWNER TO espacigo_document_verification_fn;
ALTER FUNCTION public.document_verification_lock_space_owner(uuid, uuid) OWNER TO espacigo_document_verification_fn;
ALTER FUNCTION public.document_verification_space_owned_by(uuid, uuid) OWNER TO espacigo_document_verification_fn;
ALTER FUNCTION public.document_verification_lock_space_row(uuid) OWNER TO espacigo_document_verification_fn;
REVOKE CREATE ON SCHEMA public FROM espacigo_document_verification_fn;

REVOKE ALL ON FUNCTION public.document_verification_lock_active_actor(uuid) FROM PUBLIC;
REVOKE ALL ON FUNCTION public.document_verification_lock_space_owner(uuid, uuid) FROM PUBLIC;
REVOKE ALL ON FUNCTION public.document_verification_space_owned_by(uuid, uuid) FROM PUBLIC;
REVOKE ALL ON FUNCTION public.document_verification_lock_space_row(uuid) FROM PUBLIC;

DO $runtime_grants$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname = 'espacigo_runtime') THEN
        GRANT EXECUTE ON FUNCTION public.document_verification_lock_active_actor(uuid) TO espacigo_runtime;
        GRANT EXECUTE ON FUNCTION public.document_verification_lock_space_owner(uuid, uuid) TO espacigo_runtime;
        GRANT EXECUTE ON FUNCTION public.document_verification_space_owned_by(uuid, uuid) TO espacigo_runtime;
        GRANT EXECUTE ON FUNCTION public.document_verification_lock_space_row(uuid) TO espacigo_runtime;
    END IF;
END;
$runtime_grants$;
