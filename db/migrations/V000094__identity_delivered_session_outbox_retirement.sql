-- V94: retire only acknowledged DIDit session-deletion outbox markers after
-- their operational 30-day post-ACK retention window. This does not prove or
-- guarantee exactly-once provider requests after the deduplication key is gone.
DO $guard$
DECLARE
    actor oid := (SELECT r.oid FROM pg_catalog.pg_roles r WHERE r.rolname = current_user);
    worker oid;
    relation oid := pg_catalog.to_regclass('public.outbox_evento_local');
BEGIN
    IF session_user <> 'espacigo_migrator_login'
       OR current_user <> 'espacigo_schema_owner'
       OR current_schema() <> 'public'
       OR NOT pg_catalog.has_schema_privilege(current_user, 'public', 'CREATE') THEN
        RAISE EXCEPTION 'V94 requires the pinned migrator/schema-owner identity and public CREATE';
    END IF;
    IF pg_catalog.to_regprocedure('public.purge_delivered_identity_session_outbox()') IS NOT NULL THEN
        RAISE EXCEPTION 'V94 retirement function already exists';
    END IF;
    SELECT r.oid INTO worker FROM pg_catalog.pg_roles r
     WHERE r.rolname = 'espacigo_privacy_cleanup_worker'
       AND NOT r.rolcanlogin AND NOT r.rolsuper AND NOT r.rolinherit
       AND NOT r.rolcreaterole AND NOT r.rolcreatedb
       AND NOT r.rolreplication AND NOT r.rolbypassrls;
    IF actor IS NULL OR worker IS NULL OR relation IS NULL
       OR NOT EXISTS (SELECT 1 FROM pg_catalog.pg_roles r WHERE r.oid=actor
           AND NOT r.rolcanlogin AND NOT r.rolsuper AND NOT r.rolinherit
           AND NOT r.rolcreaterole AND NOT r.rolcreatedb
           AND NOT r.rolreplication AND NOT r.rolbypassrls) THEN
        RAISE EXCEPTION 'V94 requires safe schema-owner and cleanup-worker roles, and outbox relation';
    END IF;
    IF EXISTS (
        SELECT 1 FROM pg_catalog.pg_auth_members m
        WHERE (m.roleid = worker OR m.member = worker)
          AND NOT (
              m.roleid = worker
              AND m.member = COALESCE((SELECT oid FROM pg_catalog.pg_roles
                  WHERE rolname='espacigo_privacy_worker_login' AND rolcanlogin
                    AND NOT rolsuper AND NOT rolinherit AND NOT rolcreaterole
                    AND NOT rolcreatedb AND NOT rolreplication AND NOT rolbypassrls),0::oid)
              AND NOT m.admin_option AND NOT m.inherit_option AND m.set_option
          )
    ) THEN
        RAISE EXCEPTION 'V94 cleanup-worker membership differs from the reviewed SET ROLE edge';
    END IF;
    IF (SELECT pg_catalog.pg_get_userbyid(c.relowner) FROM pg_catalog.pg_class c WHERE c.oid=relation) <> 'postgres'
       OR NOT pg_catalog.has_table_privilege(current_user, relation, 'DELETE')
       OR NOT pg_catalog.has_column_privilege(current_user,relation,'id','UPDATE')
       OR EXISTS(SELECT 1 FROM pg_catalog.unnest(ARRAY[
          'id','agregado_tipo','tipo','clave_deduplicacion','entregada_en',
          'retirar_en','cancelada_en','fallo_terminal_en'
       ]) required(name) WHERE NOT pg_catalog.has_column_privilege(current_user,relation,required.name,'SELECT')) THEN
        RAISE EXCEPTION 'V94 requires reviewed outbox owner and explicit definer SELECT/DELETE/row-lock privileges';
    END IF;
    IF (SELECT count(*) FROM pg_catalog.pg_attribute a
         WHERE a.attrelid=relation AND NOT a.attisdropped
           AND (a.attname, a.atttypid) IN (
               ('id', 'uuid'::regtype),
               ('agregado_tipo', 'text'::regtype),
               ('tipo', 'text'::regtype),
               ('clave_deduplicacion', 'text'::regtype),
               ('entregada_en', 'timestamp with time zone'::regtype),
               ('retirar_en', 'timestamp with time zone'::regtype),
               ('cancelada_en', 'timestamp with time zone'::regtype),
               ('fallo_terminal_en', 'timestamp with time zone'::regtype)
           )) <> 8
       OR EXISTS (
           SELECT 1 FROM pg_catalog.unnest(ARRAY[
               'id','agregado_tipo','tipo','clave_deduplicacion','entregada_en',
               'retirar_en','cancelada_en','fallo_terminal_en'
           ]) required(name)
           WHERE NOT EXISTS (SELECT 1 FROM pg_catalog.pg_attribute a
               WHERE a.attrelid=relation AND a.attname=required.name
                 AND a.attnum>0 AND NOT a.attisdropped)
       ) THEN
        RAISE EXCEPTION 'V94 outbox required column names/types do not match the reviewed contract';
    END IF;
END
$guard$;

CREATE FUNCTION public.purge_delivered_identity_session_outbox()
RETURNS bigint
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog
AS $$
DECLARE
    v_now timestamptz := pg_catalog.clock_timestamp();
    v_deleted bigint;
BEGIN
    WITH candidate AS MATERIALIZED (
        SELECT o.id
          FROM public.outbox_evento_local AS o
         WHERE o.agregado_tipo = 'didit_session'
           AND o.tipo = 'didit.session_remote_deletion'
           AND pg_catalog.starts_with(o.clave_deduplicacion, 'didit_session_deletion:')
           AND pg_catalog.char_length(o.clave_deduplicacion) > pg_catalog.char_length('didit_session_deletion:')
           AND pg_catalog.char_length(o.clave_deduplicacion) <= 120
           AND pg_catalog.substr(o.clave_deduplicacion,
                 pg_catalog.char_length('didit_session_deletion:') + 1)
               = pg_catalog.btrim(pg_catalog.substr(o.clave_deduplicacion,
                 pg_catalog.char_length('didit_session_deletion:') + 1))
           AND o.entregada_en IS NOT NULL
           AND o.retirar_en IS NOT NULL
           AND o.retirar_en <= v_now
           AND o.retirar_en >= o.entregada_en + interval '30 days'
           AND o.cancelada_en IS NULL
           AND o.fallo_terminal_en IS NULL
         ORDER BY o.retirar_en, o.id
         LIMIT 100
         FOR UPDATE OF o SKIP LOCKED
    )
    DELETE FROM public.outbox_evento_local AS o
     USING candidate AS c
     WHERE o.id = c.id
       AND o.agregado_tipo = 'didit_session'
       AND o.tipo = 'didit.session_remote_deletion'
       AND pg_catalog.starts_with(o.clave_deduplicacion, 'didit_session_deletion:')
       AND pg_catalog.char_length(o.clave_deduplicacion) > pg_catalog.char_length('didit_session_deletion:')
       AND pg_catalog.char_length(o.clave_deduplicacion) <= 120
       AND pg_catalog.substr(o.clave_deduplicacion,
             pg_catalog.char_length('didit_session_deletion:') + 1)
           = pg_catalog.btrim(pg_catalog.substr(o.clave_deduplicacion,
             pg_catalog.char_length('didit_session_deletion:') + 1))
       AND o.entregada_en IS NOT NULL
       AND o.retirar_en IS NOT NULL
       AND o.retirar_en <= v_now
       AND o.retirar_en >= o.entregada_en + interval '30 days'
       AND o.cancelada_en IS NULL
       AND o.fallo_terminal_en IS NULL;
    GET DIAGNOSTICS v_deleted = ROW_COUNT;
    IF v_deleted < 0 OR v_deleted > 100 THEN
        RAISE EXCEPTION 'V94 retirement exceeded its batch bound';
    END IF;
    RETURN v_deleted;
END;
$$;

REVOKE ALL ON FUNCTION public.purge_delivered_identity_session_outbox() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION public.purge_delivered_identity_session_outbox()
    TO espacigo_privacy_cleanup_worker;
