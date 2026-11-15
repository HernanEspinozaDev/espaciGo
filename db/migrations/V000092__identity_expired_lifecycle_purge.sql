-- V92: bounded, transactional expiry of declaration-linked identity lifecycle rows.
DO $guard$
DECLARE actor text := current_user;
BEGIN
    IF session_user <> 'espacigo_migrator_login' OR actor <> 'espacigo_schema_owner'
       OR current_schema() <> 'public'
       OR NOT pg_catalog.has_schema_privilege(actor, 'public', 'CREATE') THEN
        RAISE EXCEPTION 'V92 requires the pinned migrator/schema-owner identity and public CREATE';
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_catalog.pg_roles r WHERE r.rolname='espacigo_runtime'
       AND NOT r.rolcanlogin AND NOT r.rolsuper AND NOT r.rolinherit AND NOT r.rolcreaterole
       AND NOT r.rolcreatedb AND NOT r.rolreplication AND NOT r.rolbypassrls) THEN
        RAISE EXCEPTION 'V92 runtime role is missing or unsafe';
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_catalog.pg_roles r WHERE r.rolname='espacigo_privacy_cleanup_worker'
       AND NOT r.rolcanlogin AND NOT r.rolsuper AND NOT r.rolinherit AND NOT r.rolcreaterole
       AND NOT r.rolcreatedb AND NOT r.rolreplication AND NOT r.rolbypassrls) THEN
        RAISE EXCEPTION 'V92 cleanup worker role is missing or unsafe';
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_catalog.pg_proc p
        WHERE p.oid='public.purge_expired_identity_declaration_revisions()'::regprocedure
          AND p.proowner=actor::regrole AND p.prosecdef
          AND p.proconfig=ARRAY['search_path=pg_catalog']::text[])
       OR EXISTS (SELECT 1 FROM pg_catalog.pg_class c
          WHERE c.oid IN ('public.identity_provider_attempt'::regclass,
                         'public.identity_provider_event'::regclass,
                         'public.identity_rut_claim'::regclass,
                         'public.identity_declaration_revision'::regclass)
            AND c.relowner<>actor::regrole) THEN
        RAISE EXCEPTION 'V92 requires exact function/table ownership and security mode';
    END IF;
    IF (SELECT count(*) FROM pg_catalog.pg_proc p
        CROSS JOIN LATERAL pg_catalog.aclexplode(p.proacl) acl
        WHERE p.oid='public.purge_expired_identity_declaration_revisions()'::regprocedure
          AND acl.grantee<>p.proowner)<>1
       OR NOT EXISTS (SELECT 1 FROM pg_catalog.pg_proc p
        CROSS JOIN LATERAL pg_catalog.aclexplode(p.proacl) acl
        WHERE p.oid='public.purge_expired_identity_declaration_revisions()'::regprocedure
          AND acl.grantee='espacigo_privacy_cleanup_worker'::regrole
          AND acl.grantor=p.proowner AND acl.privilege_type='EXECUTE' AND NOT acl.is_grantable)
       OR pg_catalog.has_function_privilege('espacigo_runtime',
          'public.purge_expired_identity_declaration_revisions()','EXECUTE') THEN
        RAISE EXCEPTION 'V92 requires the exact worker-only predecessor ACL';
    END IF;
END
$guard$;

CREATE OR REPLACE FUNCTION public.purge_expired_identity_declaration_revisions()
RETURNS bigint
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog
AS $$
DECLARE
    purged bigint := 0;
    d record;
    a record;
    session_key text;
    dedup_key text;
    deleted_rows bigint;
    locked_revision uuid;
BEGIN
    -- This scan takes no revision lock: VERIFIED writers acquire attempt, event,
    -- claim, revision, in that order. Every potentially contended row lock below
    -- is NOWAIT, inside a per-group subtransaction. A busy group rolls back all
    -- its locks and mutations rather than deleting only some of its children.
    FOR d IN SELECT r.id, r.account_id, r.retention_at
               FROM public.identity_declaration_revision r
              WHERE r.retention_at <= pg_catalog.clock_timestamp()
              ORDER BY r.retention_at, r.id
              LIMIT 100
    LOOP
        BEGIN
            -- Save and Prepare use independent account namespaces 0 and 1.
            IF NOT pg_catalog.pg_try_advisory_xact_lock(pg_catalog.hashtextextended(d.account_id::text, 0))
               OR NOT pg_catalog.pg_try_advisory_xact_lock(pg_catalog.hashtextextended(d.account_id::text, 1)) THEN
                -- Unwind any first-namespace lock acquired by this group.
                -- CONTINUE would retain it until the outer transaction ends.
                RAISE lock_not_available;
            END IF;
            PERFORM x.id FROM public.identity_provider_attempt x
             WHERE x.declaration_id=d.id AND x.account_id=d.account_id
             ORDER BY x.id FOR UPDATE OF x NOWAIT;
            PERFORM e.id FROM public.identity_provider_event e
             JOIN public.identity_provider_attempt x ON x.id=e.attempt_id
             WHERE x.declaration_id=d.id AND x.account_id=d.account_id
             ORDER BY e.id FOR UPDATE OF e NOWAIT;
            PERFORM c.fingerprint FROM public.identity_rut_claim c
             WHERE c.declaration_id=d.id AND c.account_id=d.account_id
             ORDER BY c.fingerprint FOR UPDATE OF c NOWAIT;
            locked_revision := NULL;
            SELECT r.id INTO locked_revision FROM public.identity_declaration_revision r
             WHERE r.id=d.id AND r.account_id=d.account_id
               AND r.retention_at<=pg_catalog.clock_timestamp()
             FOR UPDATE OF r NOWAIT;
            IF locked_revision IS NULL THEN RAISE lock_not_available; END IF;

        -- Durable remote-deletion intent precedes all local deletion; no network call occurs here.
        FOR a IN SELECT x.id, x.provider_session_id, x.created_at
                   FROM public.identity_provider_attempt x
                  WHERE x.declaration_id=d.id AND x.account_id=d.account_id
                  ORDER BY x.id
        LOOP
            session_key := pg_catalog.btrim(a.provider_session_id);
            IF session_key IS NOT NULL AND session_key <> '' THEN
                dedup_key := 'didit_session_deletion:' || session_key;
                IF pg_catalog.char_length(dedup_key) > 120 THEN
                    RAISE EXCEPTION 'provider session identifier exceeds outbox limit' USING ERRCODE='22023';
                END IF;

                INSERT INTO public.outbox_evento_local(id, agregado_tipo, agregado_id, tipo, clave_deduplicacion, version, creada_en, disponible_en)
                VALUES (pg_catalog.gen_random_uuid(), 'didit_session', a.id, 'didit.session_remote_deletion', dedup_key, 1, pg_catalog.clock_timestamp(), pg_catalog.clock_timestamp())
                ON CONFLICT (clave_deduplicacion) DO NOTHING;
                IF NOT EXISTS (SELECT 1 FROM public.outbox_evento_local o WHERE o.clave_deduplicacion=dedup_key AND o.agregado_tipo='didit_session' AND o.agregado_id=a.id AND o.tipo='didit.session_remote_deletion') THEN
                    RAISE EXCEPTION 'session deletion outbox key is bound to another record' USING ERRCODE='23505';
                END IF;
            END IF;
        END LOOP;

        -- All group locks are already held. Keep exact FK deletion order;
        -- verified anti-reuse claims survive without a reference to ciphertext.
        DELETE FROM public.identity_provider_event e USING public.identity_provider_attempt x
         WHERE e.attempt_id=x.id AND x.declaration_id=d.id AND x.account_id=d.account_id;
        DELETE FROM public.identity_provider_attempt x WHERE x.declaration_id=d.id AND x.account_id=d.account_id;
        DELETE FROM public.identity_rut_claim c WHERE c.declaration_id=d.id AND c.account_id=d.account_id AND c.verified_at IS NULL;
        UPDATE public.identity_rut_claim c SET declaration_id=NULL,
            updated_at=GREATEST(c.updated_at,pg_catalog.clock_timestamp())
         WHERE c.declaration_id=d.id AND c.account_id=d.account_id AND c.verified_at IS NOT NULL;
        DELETE FROM public.identity_declaration_revision r WHERE r.id=d.id AND r.account_id=d.account_id AND r.retention_at <= pg_catalog.clock_timestamp();
        GET DIAGNOSTICS deleted_rows = ROW_COUNT;
        purged := purged + deleted_rows;
        EXCEPTION WHEN lock_not_available THEN
            -- No partial outbox or deletion survives this group's subtransaction.
            NULL;
        END;
    END LOOP;
    RETURN purged;
END;
$$;

REVOKE ALL ON FUNCTION public.purge_expired_identity_declaration_revisions() FROM PUBLIC;
REVOKE ALL ON FUNCTION public.purge_expired_identity_declaration_revisions() FROM espacigo_runtime;
GRANT EXECUTE ON FUNCTION public.purge_expired_identity_declaration_revisions() TO espacigo_privacy_cleanup_worker;
