-- The helper role and its private schema are administrator-preprovisioned.
-- Fail closed unless the exact non-login owner, schema, grants, and SET ROLE
-- membership contract are present before changing migration state. Routine
-- runs use the dedicated migrator; the isolated F5 target also permits the
-- versioned runner's explicit-confirmation application_name marker. This is
-- an accidental-bypass tripwire, not a security boundary against a superuser.
DO $preconditions$
DECLARE
    migrator_oid oid;
    api_login_oid oid;
    schema_owner_oid oid;
    helper_oid oid;
    runtime_oid oid;
    private_schema_oid oid;
BEGIN
    IF NOT (
        (session_user = 'espacigo_migrator_login' AND current_user = 'espacigo_schema_owner')
        OR (current_database() = 'espacigo_f5_gate_20261101'
            AND session_user = 'postgres' AND current_user = 'postgres'
            AND current_setting('application_name', true) = 'f5-confirmed:espacigo_f5_gate_20261101')
    ) THEN
        RAISE EXCEPTION 'V81 requires the dedicated migrator identity or the confirmed F5 versioned migration context';
    END IF;
    SELECT oid INTO migrator_oid FROM pg_catalog.pg_roles WHERE rolname='espacigo_migrator_login'
      AND rolcanlogin AND NOT rolsuper AND NOT rolcreaterole AND NOT rolcreatedb
      AND NOT rolreplication AND NOT rolbypassrls AND NOT rolinherit;
    SELECT oid INTO api_login_oid FROM pg_catalog.pg_roles WHERE rolname='espacigo_api_login'
      AND rolcanlogin AND NOT rolsuper AND NOT rolcreaterole AND NOT rolcreatedb
      AND NOT rolreplication AND NOT rolbypassrls AND NOT rolinherit;
    SELECT oid INTO schema_owner_oid FROM pg_catalog.pg_roles WHERE rolname='espacigo_schema_owner'
      AND NOT rolcanlogin AND NOT rolsuper AND NOT rolcreaterole AND NOT rolcreatedb
      AND NOT rolreplication AND NOT rolbypassrls AND NOT rolinherit;
    SELECT oid INTO helper_oid FROM pg_catalog.pg_roles WHERE rolname='espacigo_document_verification_fn'
      AND NOT rolsuper AND NOT rolcreaterole AND NOT rolcreatedb AND NOT rolreplication
      AND NOT rolbypassrls AND NOT rolcanlogin AND NOT rolinherit;
    SELECT oid INTO runtime_oid FROM pg_catalog.pg_roles WHERE rolname='espacigo_runtime'
      AND NOT rolcanlogin AND NOT rolsuper AND NOT rolcreaterole AND NOT rolcreatedb
      AND NOT rolreplication AND NOT rolbypassrls AND NOT rolinherit;
    SELECT oid INTO private_schema_oid FROM pg_catalog.pg_namespace
     WHERE nspname='document_verification_private' AND nspowner=helper_oid;
    IF migrator_oid IS NULL OR api_login_oid IS NULL OR schema_owner_oid IS NULL
       OR helper_oid IS NULL OR runtime_oid IS NULL OR private_schema_oid IS NULL THEN
        RAISE EXCEPTION 'V81 prerequisite missing: provision restricted migration, API, runtime, and helper roles plus the owned private schema';
    END IF;
    IF NOT EXISTS (
           SELECT 1 FROM pg_catalog.pg_auth_members m
            WHERE m.roleid=schema_owner_oid AND m.member=migrator_oid
              AND m.set_option AND NOT m.inherit_option AND NOT m.admin_option
       )
       OR NOT EXISTS (
           SELECT 1 FROM pg_catalog.pg_auth_members m
            WHERE m.roleid=helper_oid AND m.member=schema_owner_oid
              AND m.set_option AND NOT m.inherit_option AND NOT m.admin_option
       )
       OR NOT EXISTS (
           SELECT 1 FROM pg_catalog.pg_auth_members m
            WHERE m.roleid=runtime_oid AND m.member=api_login_oid
              AND m.set_option AND NOT m.inherit_option AND NOT m.admin_option
       )
       OR EXISTS (SELECT 1 FROM pg_catalog.pg_auth_members m WHERE m.member=migrator_oid AND m.roleid<>schema_owner_oid)
       OR EXISTS (SELECT 1 FROM pg_catalog.pg_auth_members m WHERE m.member=api_login_oid AND m.roleid<>runtime_oid)
       OR EXISTS (SELECT 1 FROM pg_catalog.pg_auth_members m WHERE m.member=schema_owner_oid AND m.roleid<>helper_oid)
       OR EXISTS (SELECT 1 FROM pg_catalog.pg_auth_members m WHERE m.roleid=migrator_oid)
       OR EXISTS (SELECT 1 FROM pg_catalog.pg_auth_members m WHERE m.roleid=api_login_oid)
       OR EXISTS (SELECT 1 FROM pg_catalog.pg_auth_members m WHERE m.roleid=schema_owner_oid AND m.member<>migrator_oid)
       OR EXISTS (SELECT 1 FROM pg_catalog.pg_auth_members m WHERE m.member=helper_oid)
       OR EXISTS (SELECT 1 FROM pg_catalog.pg_auth_members m WHERE m.roleid=helper_oid AND m.member<>schema_owner_oid)
       OR EXISTS (SELECT 1 FROM pg_catalog.pg_auth_members m WHERE m.member=runtime_oid)
       OR NOT pg_catalog.has_schema_privilege(helper_oid, private_schema_oid, 'CREATE')
       OR NOT pg_catalog.has_schema_privilege(helper_oid, 'public', 'USAGE')
       OR EXISTS (
           SELECT 1 FROM (VALUES
               ('usuario','id','SELECT'),('usuario','estado','SELECT'),('usuario','id','UPDATE'),
               ('espacio','id','SELECT'),('espacio','propietario_id','SELECT'),('espacio','id','UPDATE')
           ) required(table_name,column_name,privilege)
            WHERE NOT EXISTS (
                SELECT 1 FROM pg_catalog.pg_class c
                JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
                JOIN pg_catalog.pg_attribute a ON a.attrelid=c.oid AND a.attnum>0 AND NOT a.attisdropped
                CROSS JOIN LATERAL pg_catalog.aclexplode(a.attacl) acl
                 WHERE n.nspname='public' AND c.relname=required.table_name
                   AND a.attname=required.column_name AND acl.grantee=helper_oid
                   AND acl.grantor=c.relowner AND acl.privilege_type=required.privilege
                   AND NOT acl.is_grantable
            )
       )
       OR pg_catalog.has_schema_privilege(helper_oid, 'public', 'CREATE')
       OR EXISTS (
           SELECT 1 FROM pg_catalog.pg_namespace n
            WHERE n.nspname NOT IN ('pg_catalog','information_schema')
              AND n.oid<>pg_catalog.pg_my_temp_schema()
              AND n.oid<>private_schema_oid
              AND pg_catalog.has_schema_privilege(helper_oid, n.oid, 'CREATE')
       )
       OR EXISTS (
           SELECT 1 FROM pg_catalog.pg_class c
            JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
            CROSS JOIN LATERAL pg_catalog.aclexplode(c.relacl) acl
            WHERE n.nspname NOT IN ('pg_catalog','information_schema')
              AND n.oid<>pg_catalog.pg_my_temp_schema()
               AND c.relkind IN ('r','p','v','m','f') AND acl.grantee=helper_oid
       )
       OR EXISTS (
           SELECT 1 FROM pg_catalog.pg_class c
            JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
            JOIN pg_catalog.pg_attribute a ON a.attrelid=c.oid AND a.attnum>0 AND NOT a.attisdropped
            CROSS JOIN LATERAL pg_catalog.aclexplode(a.attacl) acl
            WHERE n.nspname NOT IN ('pg_catalog','information_schema')
              AND n.oid<>pg_catalog.pg_my_temp_schema() AND c.relkind IN ('r','p','v','m','f')
              AND acl.grantee=helper_oid
              AND NOT (n.nspname='public' AND (
                    (c.relname='usuario' AND a.attname IN ('id','estado') AND acl.privilege_type='SELECT')
                    OR (c.relname='usuario' AND a.attname='id' AND acl.privilege_type='UPDATE')
                    OR (c.relname='espacio' AND a.attname IN ('id','propietario_id') AND acl.privilege_type='SELECT')
                    OR (c.relname='espacio' AND a.attname='id' AND acl.privilege_type='UPDATE')
              ) AND acl.grantor=c.relowner AND NOT acl.is_grantable)
       )
       OR EXISTS (
           SELECT 1 FROM pg_catalog.pg_class c
            JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
            CROSS JOIN LATERAL pg_catalog.aclexplode(c.relacl) acl
            WHERE n.nspname NOT IN ('pg_catalog','information_schema')
              AND n.oid<>pg_catalog.pg_my_temp_schema() AND c.relkind='S'
              AND acl.grantee=helper_oid
       )
       OR EXISTS (SELECT 1 FROM pg_catalog.pg_class c WHERE c.relowner=helper_oid AND c.relkind IN ('r','p','v','m','S','f'))
       OR EXISTS (SELECT 1 FROM pg_catalog.pg_proc p WHERE p.proowner=helper_oid)
       OR EXISTS (
           SELECT 1 FROM pg_catalog.pg_proc p
            CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(p.proacl,pg_catalog.acldefault('f',p.proowner))) acl
            WHERE acl.grantee=helper_oid AND acl.privilege_type='EXECUTE'
       )
       OR pg_catalog.has_database_privilege(helper_oid,current_database(),'CREATE')
       OR pg_catalog.has_database_privilege(migrator_oid,current_database(),'CREATE')
       OR pg_catalog.has_database_privilege(api_login_oid,current_database(),'CREATE')
       OR pg_catalog.has_database_privilege(schema_owner_oid,current_database(),'CREATE')
       OR pg_catalog.has_database_privilege(runtime_oid,current_database(),'CREATE')
       OR EXISTS (
           SELECT 1 FROM pg_catalog.pg_namespace n
            WHERE n.nspname NOT IN ('pg_catalog','information_schema')
              AND n.oid<>pg_catalog.pg_my_temp_schema()
              AND pg_catalog.has_schema_privilege(migrator_oid,n.oid,'CREATE')
       )
       OR EXISTS (
           SELECT 1 FROM pg_catalog.pg_namespace n
            WHERE n.nspname NOT IN ('pg_catalog','information_schema')
              AND n.oid<>pg_catalog.pg_my_temp_schema()
              AND pg_catalog.has_schema_privilege(schema_owner_oid,n.oid,'CREATE')
       )
       OR EXISTS (
           SELECT 1 FROM pg_catalog.pg_namespace n
            WHERE n.nspname NOT IN ('pg_catalog','information_schema')
              AND n.oid<>pg_catalog.pg_my_temp_schema()
              AND pg_catalog.has_schema_privilege(api_login_oid,n.oid,'CREATE')
       )
       OR EXISTS (
           SELECT 1 FROM pg_catalog.pg_namespace n
            WHERE n.nspname NOT IN ('pg_catalog','information_schema')
              AND n.oid<>pg_catalog.pg_my_temp_schema()
              AND pg_catalog.has_schema_privilege(runtime_oid,n.oid,'CREATE')
       )
       OR NOT pg_catalog.has_table_privilege(schema_owner_oid,'public.schema_migrations','SELECT')
       OR NOT pg_catalog.has_table_privilege(schema_owner_oid,'public.schema_migrations','INSERT')
       OR EXISTS (
           SELECT 1
             FROM pg_catalog.pg_class c
             CROSS JOIN LATERAL pg_catalog.aclexplode(
                 COALESCE(c.relacl, pg_catalog.acldefault('r', c.relowner))
             ) acl
            WHERE c.oid='public.schema_migrations'::regclass
              AND (
                  (acl.grantee <> c.relowner AND acl.grantee <> schema_owner_oid)
                  OR (
                      acl.grantee = schema_owner_oid
                      AND (
                          acl.privilege_type NOT IN ('SELECT','INSERT')
                          OR acl.grantor <> c.relowner
                          OR acl.is_grantable
                      )
                  )
              )
       )
       OR EXISTS (
           SELECT 1
             FROM (VALUES ('SELECT'::text),('INSERT'::text)) expected(privilege_type)
            WHERE NOT EXISTS (
                SELECT 1
                  FROM pg_catalog.pg_class c
                  CROSS JOIN LATERAL pg_catalog.aclexplode(
                      COALESCE(c.relacl, pg_catalog.acldefault('r', c.relowner))
                  ) acl
                 WHERE c.oid='public.schema_migrations'::regclass
                   AND acl.grantee=schema_owner_oid
                   AND acl.grantor=c.relowner
                   AND acl.privilege_type=expected.privilege_type
                   AND NOT acl.is_grantable
            )
       )
       OR EXISTS (
           SELECT 1
             FROM pg_catalog.pg_attribute a
            WHERE a.attrelid='public.schema_migrations'::regclass
              AND a.attnum > 0 AND NOT a.attisdropped AND a.attacl IS NOT NULL
       )
       OR pg_catalog.pg_has_role(runtime_oid,helper_oid,'MEMBER') THEN
        RAISE EXCEPTION 'V81 prerequisite privilege contract is incomplete or too broad';
    END IF;
END;
$preconditions$;

SET LOCAL ROLE espacigo_document_verification_fn;
CREATE FUNCTION document_verification_private.document_verification_lock_active_actor(p_actor_id uuid)
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

CREATE FUNCTION document_verification_private.document_verification_lock_space_owner(p_space_id uuid, p_expected_owner_id uuid)
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

CREATE FUNCTION document_verification_private.document_verification_space_owned_by(p_space_id uuid, p_expected_owner_id uuid)
RETURNS boolean
LANGUAGE plpgsql
VOLATILE
SECURITY DEFINER
SET search_path = pg_catalog, pg_temp
AS $function$
DECLARE
    current_owner_id uuid;
BEGIN
    -- The row lock lasts only through the caller's transaction and serializes
    -- this ownership check with a concurrent ownership change.
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

CREATE FUNCTION document_verification_private.document_verification_lock_space_row(p_space_id uuid)
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

REVOKE ALL ON FUNCTION document_verification_private.document_verification_lock_active_actor(uuid) FROM PUBLIC;
REVOKE ALL ON FUNCTION document_verification_private.document_verification_lock_space_owner(uuid, uuid) FROM PUBLIC;
REVOKE ALL ON FUNCTION document_verification_private.document_verification_space_owned_by(uuid, uuid) FROM PUBLIC;
REVOKE ALL ON FUNCTION document_verification_private.document_verification_lock_space_row(uuid) FROM PUBLIC;

DO $runtime_grants$
BEGIN
    GRANT USAGE ON SCHEMA document_verification_private TO espacigo_runtime;
    GRANT EXECUTE ON FUNCTION document_verification_private.document_verification_lock_active_actor(uuid) TO espacigo_runtime;
    GRANT EXECUTE ON FUNCTION document_verification_private.document_verification_lock_space_owner(uuid, uuid) TO espacigo_runtime;
    GRANT EXECUTE ON FUNCTION document_verification_private.document_verification_space_owned_by(uuid, uuid) TO espacigo_runtime;
    GRANT EXECUTE ON FUNCTION document_verification_private.document_verification_lock_space_row(uuid) TO espacigo_runtime;
END;
$runtime_grants$;

-- The migrator writes public.schema_migrations after this file, in the same
-- transaction. Return to its expected role before the migration runner resumes.
SET LOCAL ROLE espacigo_schema_owner;
