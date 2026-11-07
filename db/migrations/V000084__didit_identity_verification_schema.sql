-- KYC-02: persistence for Didit identity-verification attempts and webhook inbox.
-- Does not store captured documents, images, video, or biometric templates.
CREATE TABLE identity_verification_attempts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id UUID NOT NULL REFERENCES public.usuario(id) ON DELETE CASCADE,
    status VARCHAR(30) NOT NULL DEFAULT 'CREATED'
        CHECK (status IN ('CREATED', 'PENDING_USER_INPUT', 'PROCESSING', 'APPROVED', 'IN_REVIEW', 'REJECTED', 'EXPIRED', 'FAILED')),
    id_country VARCHAR(3) NOT NULL DEFAULT 'CHL',
    document_type VARCHAR(20) NOT NULL DEFAULT 'ID',
    declared_first_name VARCHAR(150) NOT NULL,
    declared_last_name VARCHAR(150) NOT NULL,
    declared_date_of_birth DATE NOT NULL,
    declared_rut_normalized VARCHAR(20) NOT NULL,
    didit_session_id VARCHAR(100),
    didit_session_url TEXT,
    didit_workflow_id VARCHAR(100),
    environment VARCHAR(20) NOT NULL DEFAULT 'SANDBOX',
    consent_given_at TIMESTAMPTZ NOT NULL,
    consent_version VARCHAR(20) NOT NULL,
    decision_status VARCHAR(50),
    decision_warnings JSONB,
    extracted_data JSONB,
    mismatch_flags JSONB,
    verified_at TIMESTAMPTZ,
    rejection_reason TEXT,
    review_notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

CREATE INDEX idx_identity_verification_attempts_account_id
    ON identity_verification_attempts (account_id);
CREATE INDEX idx_identity_verification_attempts_didit_session_id
    ON identity_verification_attempts (didit_session_id);
CREATE INDEX idx_identity_verification_attempts_status
    ON identity_verification_attempts (status);

COMMENT ON TABLE identity_verification_attempts IS
    'Didit identity-verification attempt and declared-identity snapshot; no captured media is stored.';

CREATE TABLE didit_webhook_inbox (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id VARCHAR(120) NOT NULL UNIQUE,
    session_id VARCHAR(100) NOT NULL,
    event_type VARCHAR(80) NOT NULL,
    payload JSONB NOT NULL,
    signature_header VARCHAR(255),
    status VARCHAR(30) NOT NULL DEFAULT 'PENDING'
        CHECK (status IN ('PENDING', 'PROCESSED', 'FAILED', 'IGNORED')),
    error_message TEXT,
    received_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    processed_at TIMESTAMPTZ
);

CREATE INDEX idx_didit_webhook_inbox_session_id ON didit_webhook_inbox (session_id);
CREATE INDEX idx_didit_webhook_inbox_status ON didit_webhook_inbox (status);

COMMENT ON TABLE didit_webhook_inbox IS
    'Durable, deduplicated inbox for authenticated Didit webhook events.';

DO $grants$ BEGIN
    IF EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname = 'espacigo_runtime') THEN
        GRANT SELECT, INSERT, UPDATE ON TABLE public.identity_verification_attempts TO espacigo_runtime;
        GRANT SELECT, INSERT, UPDATE ON TABLE public.didit_webhook_inbox TO espacigo_runtime;
    END IF;
END $grants$;
