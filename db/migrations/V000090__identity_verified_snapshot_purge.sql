-- V90: make authoritative VERIFIED transition and local snapshot purge atomic.
-- Retain the account-bound HMAC claim and minimal attempt/event metadata, while
-- removing all encrypted identity snapshots and queuing remote session deletion.
DO $guard$
BEGIN
    IF session_user <> 'espacigo_migrator_login' OR current_user <> 'espacigo_schema_owner'
       OR current_schema() <> 'public'
       OR NOT pg_catalog.has_schema_privilege(current_user, 'public', 'CREATE') THEN
        RAISE EXCEPTION 'V90 requires the pinned migrator/schema-owner identity and public CREATE';
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_catalog.pg_roles r
                    WHERE r.rolname = 'espacigo_runtime'
                      AND NOT r.rolcanlogin AND NOT r.rolsuper AND NOT r.rolinherit
                      AND NOT r.rolcreaterole AND NOT r.rolcreatedb
                      AND NOT r.rolreplication AND NOT r.rolbypassrls) THEN
        RAISE EXCEPTION 'V90 runtime role is missing or has unsafe role attributes';
    END IF;
END
$guard$;

-- Nullable only for authoritative post-VERIFIED detachment. Existing restrictive
-- foreign keys remain in place; the commit function explicitly detaches children
-- before deleting the declaration, and unrelated deletes still fail closed.
ALTER TABLE public.identity_rut_claim
    ALTER COLUMN declaration_id DROP NOT NULL;
ALTER TABLE public.identity_rut_claim
    ADD CONSTRAINT identity_rut_claim_declaration_ck
    CHECK (verified_at IS NOT NULL OR declaration_id IS NOT NULL);

ALTER TABLE public.identity_provider_attempt
    ALTER COLUMN declaration_id DROP NOT NULL;
ALTER TABLE public.identity_provider_attempt
    ADD CONSTRAINT identity_provider_attempt_detached_approved_ck
    CHECK (declaration_id IS NOT NULL OR
           (status = 'APPROVED' AND decision_key_version IS NULL
            AND decision_algorithm IS NULL AND encrypted_decision IS NULL));

CREATE FUNCTION public.commit_verified_identity_decision(
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
     WHERE id = p_attempt_id
     FOR UPDATE;
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
    IF NOT FOUND OR v_claim.verified_at IS NOT NULL OR v_claim.expires_at IS NULL THEN
        RAISE EXCEPTION 'verified identity claim conflict' USING ERRCODE = '23505';
    END IF;

    v_processed_at := GREATEST(p_processed_at, v_attempt.created_at,
                               v_event.received_at, v_claim.created_at);
    v_dedup_key := 'didit_session_deletion:' || btrim(p_session_id);
    IF length(v_dedup_key) > 120 THEN
        RAISE EXCEPTION 'provider session identifier exceeds outbox limit' USING ERRCODE = '22023';
    END IF;

    -- The outbox intent is committed with the authoritative decision. The
    -- external provider call is deliberately left to the application afterward.
    INSERT INTO public.outbox_evento_local (
        id, agregado_tipo, agregado_id, tipo, clave_deduplicacion,
        version, creada_en, disponible_en
    ) VALUES (
        gen_random_uuid(), 'didit_session', p_attempt_id,
        'didit.session_remote_deletion', v_dedup_key,
        1, v_processed_at, v_processed_at
    ) ON CONFLICT (clave_deduplicacion) DO NOTHING;
    IF NOT EXISTS (
        SELECT 1
          FROM public.outbox_evento_local
         WHERE clave_deduplicacion = v_dedup_key
           AND agregado_tipo = 'didit_session'
           AND agregado_id = p_attempt_id
           AND tipo = 'didit.session_remote_deletion'
    ) THEN
        RAISE EXCEPTION 'session deletion outbox key is bound to another record' USING ERRCODE = '23505';
    END IF;

    UPDATE public.identity_rut_claim
       SET declaration_id = NULL,
           verified_at = v_processed_at,
           expires_at = NULL,
           updated_at = v_processed_at
     WHERE fingerprint = p_rut_fingerprint
       AND account_id = v_attempt.account_id
       AND declaration_id = v_attempt.declaration_id
       AND verified_at IS NULL
       AND expires_at > v_processed_at;
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
