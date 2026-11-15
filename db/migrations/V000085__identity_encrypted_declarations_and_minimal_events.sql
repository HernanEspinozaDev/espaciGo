-- V85: encrypted identity declarations and minimal provider-event records.
-- This is a fail-closed security tightening, not a data migration. Existing V84
-- plaintext rows block the migration; no plaintext is copied or deleted here.
DO $guard$
DECLARE
    actor text := current_user;
BEGIN
    IF session_user <> 'espacigo_migrator_login' OR actor <> 'espacigo_schema_owner'
       OR NOT pg_catalog.has_schema_privilege(actor, 'public', 'CREATE')
       OR NOT pg_catalog.has_column_privilege(actor, 'public.usuario', 'id', 'REFERENCES') THEN
        RAISE EXCEPTION 'V85 requires pinned migrator/schema-owner role with CREATE and REFERENCES(usuario.id)';
    END IF;
    IF EXISTS (SELECT 1 FROM pg_catalog.pg_roles r
                WHERE r.rolname = 'espacigo_runtime'
                  AND (r.rolcanlogin OR r.rolsuper OR r.rolinherit OR r.rolcreaterole OR r.rolcreatedb OR r.rolreplication OR r.rolbypassrls))
       OR NOT EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname = 'espacigo_runtime') THEN
        RAISE EXCEPTION 'V85 runtime role has unsafe role attributes';
    END IF;
END
$guard$;

LOCK TABLE public.identity_verification_attempts, public.didit_webhook_inbox IN ACCESS EXCLUSIVE MODE;
DO $empty$
BEGIN
    IF EXISTS (SELECT 1 FROM public.identity_verification_attempts LIMIT 1)
       OR EXISTS (SELECT 1 FROM public.didit_webhook_inbox LIMIT 1) THEN
        RAISE EXCEPTION 'V85 blocked: legacy V84 identity tables contain rows';
    END IF;
END
$empty$;

CREATE TABLE public.identity_declaration_revision (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id uuid NOT NULL REFERENCES public.usuario(id),
    revision bigint NOT NULL CHECK (revision > 0),
    key_version varchar(64) NOT NULL CHECK (key_version ~ '^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$'),
    ciphertext bytea NOT NULL CHECK (octet_length(ciphertext) BETWEEN 29 AND 65564),
    schema_version varchar(32) NOT NULL DEFAULT '1' CHECK (schema_version = '1'),
    encryption_algorithm varchar(16) NOT NULL DEFAULT 'AES-256-GCM/v1' CHECK (encryption_algorithm = 'AES-256-GCM/v1'),
    consent_version varchar(32) NOT NULL CHECK (length(consent_version) BETWEEN 1 AND 32),
    consent_given_at timestamptz NOT NULL,
    retention_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    UNIQUE (account_id, revision),
    UNIQUE (id, account_id),
    CHECK (retention_at > created_at),
    CHECK (consent_given_at <= created_at)
);

CREATE TABLE public.identity_provider_attempt (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id uuid NOT NULL,
    declaration_id uuid NOT NULL,
    provider varchar(16) NOT NULL CHECK (provider = 'didit'),
    environment varchar(8) NOT NULL CHECK (environment IN ('SANDBOX','LIVE')),
    application_id varchar(128) NOT NULL CHECK (length(application_id) BETWEEN 1 AND 128),
    workflow_id varchar(128) NOT NULL CHECK (length(workflow_id) BETWEEN 1 AND 128),
    workflow_version varchar(64) NOT NULL CHECK (length(workflow_version) BETWEEN 1 AND 64),
    idempotency_key uuid NOT NULL,
    correlation_id uuid NOT NULL DEFAULT gen_random_uuid(),
    provider_session_id varchar(128) CHECK (provider_session_id IS NULL OR length(btrim(provider_session_id)) > 0),
    status varchar(24) NOT NULL DEFAULT 'CREATED' CHECK (status IN ('CREATED','PENDING_USER_INPUT','PROCESSING','APPROVED','IN_REVIEW','REJECTED','EXPIRED','FAILED')),
    decision_key_version varchar(64),
    decision_algorithm varchar(16),
    encrypted_decision bytea,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    retention_at timestamptz NOT NULL,
    FOREIGN KEY (declaration_id, account_id) REFERENCES public.identity_declaration_revision(id, account_id),
    UNIQUE (id, provider, environment, application_id),
    UNIQUE (provider, environment, application_id, idempotency_key),
    UNIQUE (provider, environment, application_id, correlation_id),
    UNIQUE (provider, environment, application_id, provider_session_id),
    CHECK ((encrypted_decision IS NULL AND decision_key_version IS NULL AND decision_algorithm IS NULL)
        OR (encrypted_decision IS NOT NULL AND decision_key_version IS NOT NULL AND decision_algorithm IS NOT NULL AND decision_algorithm = 'AES-256-GCM/v1'
            AND decision_key_version ~ '^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$'
            AND octet_length(encrypted_decision) BETWEEN 29 AND 65564)),
    CHECK (retention_at > created_at),
    CHECK (updated_at >= created_at)
);
CREATE UNIQUE INDEX identity_provider_attempt_one_active_intent
    ON public.identity_provider_attempt(account_id, declaration_id, provider, environment, application_id)
    WHERE status IN ('CREATED','PENDING_USER_INPUT','PROCESSING','IN_REVIEW');

CREATE TABLE public.identity_provider_event (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    attempt_id uuid NOT NULL,
    provider varchar(16) NOT NULL CHECK (provider = 'didit'),
    environment varchar(8) NOT NULL CHECK (environment IN ('SANDBOX','LIVE')),
    application_id varchar(128) NOT NULL,
    event_id varchar(160) NOT NULL CHECK (length(event_id) BETWEEN 1 AND 160),
    event_type varchar(32) NOT NULL CHECK (event_type IN ('status.updated','data.updated','user.status.updated')),
    body_sha256 bytea NOT NULL CHECK (octet_length(body_sha256) = 32),
    processing_status varchar(16) NOT NULL DEFAULT 'RECEIVED' CHECK (processing_status IN ('RECEIVED','PROCESSED','FAILED','IGNORED')),
    received_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    processed_at timestamptz,
    retention_at timestamptz NOT NULL,
    FOREIGN KEY (attempt_id, provider, environment, application_id)
      REFERENCES public.identity_provider_attempt(id, provider, environment, application_id),
    UNIQUE (provider, environment, application_id, event_id),
    CHECK ((processing_status = 'RECEIVED' AND processed_at IS NULL)
        OR (processing_status IN ('PROCESSED','FAILED','IGNORED') AND processed_at IS NOT NULL)),
    CHECK (processed_at IS NULL OR processed_at >= received_at),
    CHECK (retention_at > received_at)
);

REVOKE ALL ON TABLE public.identity_declaration_revision, public.identity_provider_attempt, public.identity_provider_event FROM PUBLIC;
REVOKE ALL ON TABLE public.identity_verification_attempts, public.didit_webhook_inbox FROM espacigo_runtime;
DO $runtime$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname = 'espacigo_runtime') THEN
        GRANT SELECT (id, account_id, revision, key_version, ciphertext, schema_version, encryption_algorithm, consent_version, consent_given_at, retention_at, created_at),
              INSERT (id, account_id, revision, key_version, ciphertext, schema_version, consent_version, consent_given_at, retention_at)
          ON public.identity_declaration_revision TO espacigo_runtime;
        GRANT SELECT (id, account_id, declaration_id, provider, environment, application_id, workflow_id, workflow_version, idempotency_key, correlation_id, provider_session_id, status, decision_key_version, decision_algorithm, encrypted_decision, created_at, updated_at, retention_at),
              INSERT (id, account_id, declaration_id, provider, environment, application_id, workflow_id,workflow_version, idempotency_key, correlation_id, provider_session_id, status, decision_key_version, decision_algorithm, encrypted_decision, created_at, updated_at, retention_at),
              UPDATE (provider_session_id, status, decision_key_version, decision_algorithm, encrypted_decision, updated_at)
          ON public.identity_provider_attempt TO espacigo_runtime;
        GRANT SELECT (id, attempt_id, provider, environment, application_id, event_id, event_type, body_sha256, processing_status, received_at, processed_at, retention_at),
              INSERT (id, attempt_id, provider, environment, application_id, event_id, event_type, body_sha256, processing_status, received_at, processed_at, retention_at),
              UPDATE (processing_status, processed_at)
          ON public.identity_provider_event TO espacigo_runtime;
    END IF;
END
$runtime$;
