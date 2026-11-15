-- V91: permit terminal webhook decisions after V90 detached the attempt.
-- Decision ciphertext is never retained for a detached attempt.
DO $guard$
DECLARE
    v_definition text;

BEGIN
    IF session_user <> 'espacigo_migrator_login'
       OR current_user <> 'espacigo_schema_owner'
       OR current_schema() <> 'public' THEN
        RAISE EXCEPTION 'V91 requires the pinned migrator/schema-owner identity and public schema';
    END IF;
    IF NOT EXISTS (
        SELECT 1
          FROM pg_catalog.pg_class t
          JOIN pg_catalog.pg_namespace n ON n.oid = t.relnamespace
          JOIN pg_catalog.pg_roles owner_role ON owner_role.oid = t.relowner
         WHERE n.nspname = 'public'
           AND t.relname = 'identity_provider_attempt'
           AND t.relkind = 'r'
           AND owner_role.rolname = 'espacigo_schema_owner'
    ) THEN
        RAISE EXCEPTION 'V91 requires espacigo_schema_owner to own public.identity_provider_attempt';
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_catalog.pg_roles r
         WHERE r.rolname = 'espacigo_runtime'
           AND NOT r.rolcanlogin AND NOT r.rolsuper AND NOT r.rolinherit
           AND NOT r.rolcreaterole AND NOT r.rolcreatedb
           AND NOT r.rolreplication AND NOT r.rolbypassrls
    ) THEN
        RAISE EXCEPTION 'V91 runtime role is missing or has unsafe role attributes';
    END IF;

    SELECT pg_catalog.pg_get_constraintdef(c.oid)
      INTO v_definition
      FROM pg_catalog.pg_constraint c
     WHERE c.conrelid = 'public.identity_provider_attempt'::regclass
       AND c.conname = 'identity_provider_attempt_detached_approved_ck'
       AND c.contype = 'c';
    IF v_definition IS NULL THEN
        RAISE EXCEPTION 'V91 expected V90 detached-attempt constraint is missing';
    END IF;
    IF v_definition <> 'CHECK (((declaration_id IS NOT NULL) OR (((status)::text = ''APPROVED''::text) AND (decision_key_version IS NULL) AND (decision_algorithm IS NULL) AND (encrypted_decision IS NULL))))' THEN
        RAISE EXCEPTION 'V91 found unexpected V90 detached-attempt constraint definition: %', v_definition;
    END IF;
END
$guard$;

ALTER TABLE public.identity_provider_attempt
    DROP CONSTRAINT identity_provider_attempt_detached_approved_ck;

ALTER TABLE public.identity_provider_attempt
    ADD CONSTRAINT identity_provider_attempt_detached_approved_ck
    CHECK (
        declaration_id IS NOT NULL
        OR (
            status IN ('APPROVED', 'REJECTED', 'EXPIRED')
            AND decision_key_version IS NULL
            AND decision_algorithm IS NULL
            AND encrypted_decision IS NULL
        )
    );
