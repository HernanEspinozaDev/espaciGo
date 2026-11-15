-- V93: preserve a verified account claim across declaration retention purge and
-- allow a new authoritative approval to rebind that same account's claim.
DO $guard$
DECLARE
    v_definition text;
    v_owner text;
    v_security_definer boolean;
    v_search_path text[];
    v_acl text[];
BEGIN
    IF session_user <> 'espacigo_migrator_login' OR current_user <> 'espacigo_schema_owner'
       OR current_schema() <> 'public'
       OR NOT pg_catalog.has_schema_privilege(current_user, 'public', 'CREATE') THEN
        RAISE EXCEPTION 'V93 requires the pinned migrator/schema-owner identity and public CREATE';
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_catalog.pg_roles r
                    WHERE r.rolname = 'espacigo_runtime' AND NOT r.rolcanlogin
                      AND NOT r.rolsuper AND NOT r.rolinherit AND NOT r.rolcreaterole
                      AND NOT r.rolcreatedb AND NOT r.rolreplication AND NOT r.rolbypassrls) THEN
        RAISE EXCEPTION 'V93 runtime role is missing or has unsafe role attributes';
    END IF;
    SELECT pg_catalog.pg_get_functiondef(p.oid), pg_catalog.pg_get_userbyid(p.proowner),
           p.prosecdef, p.proconfig,
           ARRAY(SELECT x::text FROM pg_catalog.unnest(COALESCE(p.proacl, pg_catalog.acldefault('f', p.proowner))) x ORDER BY x::text)
      INTO v_definition, v_owner, v_security_definer, v_search_path, v_acl
      FROM pg_catalog.pg_proc p
     WHERE p.oid = 'public.commit_verified_identity_decision(uuid,uuid,text,text,timestamptz)'::regprocedure;
    IF pg_catalog.encode(pg_catalog.sha256(pg_catalog.convert_to(v_definition, 'UTF8')), 'hex')
       <> '1d89b78eef8fbea571e8d8c1b914c52394d24a4b8b0a653c2b748bc41c27d646'
       OR v_owner <> current_user OR NOT v_security_definer
       OR v_search_path IS DISTINCT FROM ARRAY['search_path=pg_catalog']::text[] THEN
        RAISE EXCEPTION 'V93 predecessor function definition/owner/security/search_path mismatch';
    END IF;
    IF NOT (pg_catalog.has_function_privilege('espacigo_runtime',
              'public.commit_verified_identity_decision(uuid,uuid,text,text,timestamptz)', 'EXECUTE'))
       OR EXISTS (SELECT 1 FROM pg_catalog.pg_proc p
          CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(p.proacl,pg_catalog.acldefault('f',p.proowner))) acl
          WHERE p.oid='public.commit_verified_identity_decision(uuid,uuid,text,text,timestamptz)'::regprocedure
            AND acl.grantee=0 AND acl.privilege_type='EXECUTE')
       OR v_acl IS DISTINCT FROM ARRAY[
              'espacigo_runtime=X/espacigo_schema_owner',
              'espacigo_schema_owner=X/espacigo_schema_owner'
          ]::text[] THEN
        RAISE EXCEPTION 'V93 requires runtime-only predecessor function EXECUTE ACL';
    END IF;
END
$guard$;

CREATE OR REPLACE FUNCTION public.commit_verified_identity_decision(
    p_attempt_id uuid,
    p_event_row_id uuid,
    p_rut_fingerprint text,
    p_session_id text,
    p_processed_at timestamptz
)
RETURNS public.identity_provider_attempt
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog
AS $$
DECLARE
    v_attempt public.identity_provider_attempt%ROWTYPE;
    v_event public.identity_provider_event%ROWTYPE;
    v_claim public.identity_rut_claim%ROWTYPE;
    v_declaration_id uuid;
    v_account_id uuid;
    v_processed_at timestamptz;
    v_dedup_key text;
    v_rows bigint;
    v_related record;
BEGIN
    IF p_attempt_id IS NULL OR p_event_row_id IS NULL
       OR p_rut_fingerprint IS NULL
       OR p_rut_fingerprint !~ '^[A-Za-z0-9][A-Za-z0-9._-]{0,31}:[0-9a-f]{64}$'
       OR p_session_id IS NULL OR length(btrim(p_session_id)) = 0
       OR p_processed_at IS NULL THEN
        RAISE EXCEPTION 'invalid verified identity decision input' USING ERRCODE = '22023';
    END IF;

    SELECT * INTO v_attempt
      FROM public.identity_provider_attempt
     WHERE id = p_attempt_id;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'provider attempt not found' USING ERRCODE = 'P0002';
    END IF;
    IF v_attempt.status NOT IN ('CREATED', 'PENDING_USER_INPUT', 'PROCESSING', 'IN_REVIEW')
       OR v_attempt.declaration_id IS NULL
       OR v_attempt.provider_session_id IS DISTINCT FROM btrim(p_session_id) THEN
        RAISE EXCEPTION 'provider attempt is not eligible for verified commit' USING ERRCODE = '23505';
    END IF;
    v_declaration_id := v_attempt.declaration_id;
    v_account_id := v_attempt.account_id;

    -- PrepareAttempt uses this account namespace. Acquire it before row locks,
    -- then lock every attempt/event of the snapshot in deterministic order.
    -- Otherwise a prior TTL retry can retain the FK and prevent snapshot purge,
    -- or competing approvals can each hold one attempt while waiting for another.
    PERFORM pg_catalog.pg_advisory_xact_lock(pg_catalog.hashtextextended(v_account_id::text, 1));
    PERFORM a.id FROM public.identity_provider_attempt a
     WHERE a.declaration_id=v_declaration_id AND a.account_id=v_account_id
     ORDER BY a.id FOR UPDATE OF a;
    SELECT * INTO v_attempt FROM public.identity_provider_attempt
     WHERE id=p_attempt_id FOR UPDATE;
    IF NOT FOUND OR v_attempt.account_id IS DISTINCT FROM v_account_id
       OR v_attempt.declaration_id IS DISTINCT FROM v_declaration_id
       OR v_attempt.status NOT IN ('CREATED', 'PENDING_USER_INPUT', 'PROCESSING', 'IN_REVIEW')
       OR v_attempt.provider_session_id IS DISTINCT FROM btrim(p_session_id) THEN
        RAISE EXCEPTION 'provider attempt changed before verified commit' USING ERRCODE='23505';
    END IF;
    PERFORM e.id FROM public.identity_provider_event e
     JOIN public.identity_provider_attempt a ON a.id=e.attempt_id
     WHERE a.declaration_id=v_declaration_id AND a.account_id=v_account_id
     ORDER BY e.id FOR UPDATE OF e;

    SELECT * INTO v_event
      FROM public.identity_provider_event
     WHERE id = p_event_row_id AND attempt_id = p_attempt_id
     FOR UPDATE;
    IF NOT FOUND OR v_event.processing_status NOT IN ('RECEIVED', 'FAILED') THEN
        RAISE EXCEPTION 'webhook event is not eligible for verified commit' USING ERRCODE = '23505';
    END IF;

    SELECT * INTO v_claim
      FROM public.identity_rut_claim
     WHERE fingerprint = p_rut_fingerprint
       AND account_id = v_attempt.account_id
       AND declaration_id = v_attempt.declaration_id
     FOR UPDATE;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'verified identity claim conflict' USING ERRCODE = '23505';
    END IF;
    v_processed_at := GREATEST(p_processed_at, v_attempt.created_at,
                               v_event.received_at, v_claim.created_at);
    IF NOT ((v_claim.verified_at IS NULL AND v_claim.expires_at > v_processed_at)
                        OR (v_claim.verified_at IS NOT NULL AND v_claim.expires_at IS NULL)) THEN
        RAISE EXCEPTION 'verified identity claim conflict' USING ERRCODE = '23505';
    END IF;

    -- Queue all known sessions before removing any obsolete retry metadata.
    -- Remote calls remain outside this transaction. Every existing key must be
    -- bound to its exact attempt, including sessions from earlier failed retries.
    FOR v_related IN SELECT a.id,a.provider_session_id
      FROM public.identity_provider_attempt a
     WHERE a.declaration_id=v_declaration_id AND a.account_id=v_account_id
     ORDER BY a.id
    LOOP
        IF v_related.provider_session_id IS NULL OR btrim(v_related.provider_session_id)='' THEN
            CONTINUE;
        END IF;
        v_dedup_key := 'didit_session_deletion:' || btrim(v_related.provider_session_id);
        IF length(v_dedup_key)>120 THEN
            RAISE EXCEPTION 'provider session identifier exceeds outbox limit' USING ERRCODE='22023';
        END IF;
        INSERT INTO public.outbox_evento_local (
            id,agregado_tipo,agregado_id,tipo,clave_deduplicacion,version,creada_en,disponible_en
        ) VALUES (
            gen_random_uuid(),'didit_session',v_related.id,'didit.session_remote_deletion',
            v_dedup_key,1,v_processed_at,v_processed_at
        ) ON CONFLICT (clave_deduplicacion) DO NOTHING;
        IF NOT EXISTS (SELECT 1 FROM public.outbox_evento_local
            WHERE clave_deduplicacion=v_dedup_key AND agregado_tipo='didit_session'
              AND agregado_id=v_related.id AND tipo='didit.session_remote_deletion') THEN
            RAISE EXCEPTION 'session deletion outbox key is bound to another record' USING ERRCODE='23505';
        END IF;
    END LOOP;

    UPDATE public.identity_rut_claim
       SET declaration_id = NULL,
           verified_at = COALESCE(v_claim.verified_at, v_processed_at),
           expires_at = NULL,
           updated_at = v_processed_at
     WHERE fingerprint = p_rut_fingerprint
       AND account_id = v_attempt.account_id
       AND declaration_id = v_attempt.declaration_id
       AND ((verified_at IS NULL AND expires_at > v_processed_at)
            OR (verified_at IS NOT NULL AND expires_at IS NULL));
    GET DIAGNOSTICS v_rows = ROW_COUNT;
    IF v_rows <> 1 THEN
        RAISE EXCEPTION 'verified identity claim expired or changed' USING ERRCODE = '23505';
    END IF;

    UPDATE public.identity_provider_attempt
       SET status = 'APPROVED',
           declaration_id = NULL,
           decision_key_version = NULL,
           decision_algorithm = NULL,
           encrypted_decision = NULL,
           updated_at = v_processed_at
     WHERE id = p_attempt_id
       AND status IN ('CREATED', 'PENDING_USER_INPUT', 'PROCESSING', 'IN_REVIEW')
       AND declaration_id = v_attempt.declaration_id
       AND provider_session_id = btrim(p_session_id)
     RETURNING * INTO v_attempt;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'provider attempt changed during verified commit' USING ERRCODE = '23505';
    END IF;

    UPDATE public.identity_provider_event
       SET processing_status = 'PROCESSED',
           processed_at = v_processed_at
     WHERE id = p_event_row_id
       AND attempt_id = p_attempt_id
       AND processing_status IN ('RECEIVED', 'FAILED');
    GET DIAGNOSTICS v_rows = ROW_COUNT;
    IF v_rows <> 1 THEN
        RAISE EXCEPTION 'webhook event changed during verified commit' USING ERRCODE = '23505';
    END IF;

    -- Only the new approved attempt remains bound to the current result. Known
    -- obsolete sessions have durable deletion intents; remove their dependent
    -- events explicitly. For an uncertain creation without a known session,
    -- retain minimal correlation metadata, expire it and remove all ciphertext.
    -- This is not confirmation that a remote unknown session was deleted.
    DELETE FROM public.identity_provider_event e USING public.identity_provider_attempt a
     WHERE e.attempt_id=a.id AND a.declaration_id=v_declaration_id
       AND a.account_id=v_account_id AND a.provider_session_id IS NOT NULL
       AND btrim(a.provider_session_id)<>'';
    DELETE FROM public.identity_provider_attempt a
     WHERE a.declaration_id=v_declaration_id AND a.account_id=v_account_id
       AND a.provider_session_id IS NOT NULL AND btrim(a.provider_session_id)<>'';
    UPDATE public.identity_provider_attempt a SET declaration_id=NULL,status='EXPIRED',
        decision_key_version=NULL,decision_algorithm=NULL,encrypted_decision=NULL,
        updated_at=GREATEST(a.updated_at,v_processed_at)
     WHERE a.declaration_id=v_declaration_id AND a.account_id=v_account_id
       AND (a.provider_session_id IS NULL OR btrim(a.provider_session_id)='');

    DELETE FROM public.identity_declaration_revision
     WHERE id = v_declaration_id
       AND account_id = v_account_id;
    GET DIAGNOSTICS v_rows = ROW_COUNT;
    IF v_rows <> 1 THEN
        RAISE EXCEPTION 'identity declaration snapshot could not be purged' USING ERRCODE = '23503';
    END IF;

    RETURN v_attempt;
END;
$$;


REVOKE ALL ON FUNCTION public.commit_verified_identity_decision(uuid, uuid, text, text, timestamptz) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION public.commit_verified_identity_decision(uuid, uuid, text, text, timestamptz) TO espacigo_runtime;
