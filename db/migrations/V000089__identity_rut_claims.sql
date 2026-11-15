-- V89: durable, globally unique HMAC-only Chilean RUT claims.
DO $guard$
DECLARE
    actor text := current_user;
BEGIN
    IF session_user <> 'espacigo_migrator_login' OR actor <> 'espacigo_schema_owner'
       OR current_schema() <> 'public'
       OR NOT pg_catalog.has_schema_privilege(actor, 'public', 'CREATE') THEN
        RAISE EXCEPTION 'V89 requires the pinned migrator/schema-owner identity and public CREATE';
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_catalog.pg_roles r
                    WHERE r.rolname = 'espacigo_runtime'
                      AND NOT r.rolcanlogin AND NOT r.rolsuper AND NOT r.rolinherit
                      AND NOT r.rolcreaterole AND NOT r.rolcreatedb
                      AND NOT r.rolreplication AND NOT r.rolbypassrls) THEN
        RAISE EXCEPTION 'V89 runtime role is missing or has unsafe role attributes';
    END IF;
END
$guard$;

CREATE TABLE public.identity_rut_claim (
    fingerprint text NOT NULL,
    account_id uuid NOT NULL,
    declaration_id uuid NOT NULL,
    expires_at timestamptz,
    verified_at timestamptz,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CONSTRAINT identity_rut_claim_pk PRIMARY KEY (fingerprint),
    CONSTRAINT identity_rut_claim_account_uk UNIQUE (account_id),
    CONSTRAINT identity_rut_claim_user_fk FOREIGN KEY (account_id)
        REFERENCES public.usuario (id) ON DELETE RESTRICT,
    CONSTRAINT identity_rut_claim_revision_fk FOREIGN KEY (declaration_id, account_id)
        REFERENCES public.identity_declaration_revision (id, account_id) ON DELETE RESTRICT,
    CONSTRAINT identity_rut_claim_fingerprint_ck CHECK (
        fingerprint ~ '^[A-Za-z0-9][A-Za-z0-9._-]{0,31}:[0-9a-f]{64}$'
    ),
    CONSTRAINT identity_rut_claim_state_ck CHECK (
        (verified_at IS NULL AND expires_at IS NOT NULL) OR
        (verified_at IS NOT NULL AND expires_at IS NULL)
    ),
    CONSTRAINT identity_rut_claim_times_ck CHECK (
        updated_at >= created_at AND (verified_at IS NULL OR verified_at >= created_at)
    )
);

CREATE INDEX identity_rut_claim_pending_expiry_idx
    ON public.identity_rut_claim (expires_at)
    WHERE verified_at IS NULL;

REVOKE ALL ON TABLE public.identity_rut_claim FROM PUBLIC;
GRANT SELECT, INSERT, UPDATE, DELETE ON TABLE public.identity_rut_claim TO espacigo_runtime;

DO $verify$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_catalog.pg_tables
        WHERE schemaname = 'public' AND tablename = 'identity_rut_claim'
    ) THEN
        RAISE EXCEPTION 'V89 table creation readback failed';
    END IF;
END
$verify$;
