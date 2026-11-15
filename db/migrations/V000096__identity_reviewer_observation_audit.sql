-- V96: persist reviewer-observed identity metadata with an allowlisted,
-- append-only audit object. No historical case/event is backfilled.
DO $guard$
DECLARE
    v_case oid := pg_catalog.to_regclass('public.document_verification_case');
    v_event oid := pg_catalog.to_regclass('public.document_verification_event');
    v_runtime oid;
    v_owner oid := (SELECT r.oid FROM pg_catalog.pg_roles r WHERE r.rolname='espacigo_schema_owner');
BEGIN
    IF session_user <> 'espacigo_migrator_login'
       OR current_user <> 'espacigo_schema_owner'
       OR current_schema() <> 'public'
       OR NOT pg_catalog.has_schema_privilege(current_user,'public','CREATE') THEN
        RAISE EXCEPTION 'V96 requires the pinned migrator/schema-owner identity and public CREATE';
    END IF;
    SELECT r.oid INTO v_runtime FROM pg_catalog.pg_roles r
     WHERE r.rolname='espacigo_runtime' AND NOT r.rolcanlogin AND NOT r.rolsuper
       AND NOT r.rolinherit AND NOT r.rolcreaterole AND NOT r.rolcreatedb
       AND NOT r.rolreplication AND NOT r.rolbypassrls;
    IF v_case IS NULL OR v_event IS NULL OR v_runtime IS NULL OR v_owner IS NULL
       OR EXISTS (SELECT 1 FROM pg_catalog.pg_attribute a
                   WHERE a.attrelid=v_case AND a.attname='document_format_observed'
                     AND a.attnum>0 AND NOT a.attisdropped)
       OR EXISTS (SELECT 1 FROM pg_catalog.pg_attribute a
                   WHERE a.attrelid=v_event AND a.attname='identity_review_observations'
                     AND a.attnum>0 AND NOT a.attisdropped) THEN
        RAISE EXCEPTION 'V96 predecessor schema or role contract is incomplete or drifted';
    END IF;
    IF (SELECT c.relowner FROM pg_catalog.pg_class c WHERE c.oid=v_case) <> v_owner
       OR (SELECT c.relowner FROM pg_catalog.pg_class c WHERE c.oid=v_event) <> v_owner
       OR NOT pg_catalog.has_column_privilege(v_runtime,v_case,'document_expires_at','UPDATE')
       OR NOT pg_catalog.has_table_privilege(v_runtime,v_case,'SELECT')
       OR NOT pg_catalog.has_table_privilege(v_runtime,v_event,'INSERT') THEN
        RAISE EXCEPTION 'V96 requires schema-owned verification tables and the reviewed runtime read/write path';
    END IF;
END
$guard$;

ALTER TABLE public.document_verification_case
    ADD COLUMN document_format_observed text,
    ADD CONSTRAINT document_verification_case_observed_format_ck
        CHECK (document_format_observed IS NULL OR
               (purpose='identity' AND document_format_observed IN ('legacy','new','undetermined')));

ALTER TABLE public.document_verification_event
    ADD COLUMN identity_review_observations jsonb,
    ADD CONSTRAINT document_verification_event_identity_observations_ck
        CHECK (
            identity_review_observations IS NULL OR
            (
                purpose='identity' AND action='review_decision'
                AND pg_catalog.jsonb_typeof(identity_review_observations)='object'
                AND identity_review_observations ?& ARRAY[
                    'document_format_observed',
                    'document_expires_at_observed',
                    'identity_review_checklist'
                ]
                AND identity_review_observations = pg_catalog.jsonb_build_object(
                    'document_format_observed', identity_review_observations->'document_format_observed',
                    'document_expires_at_observed', identity_review_observations->'document_expires_at_observed',
                    'identity_review_checklist', identity_review_observations->'identity_review_checklist'
                )
                AND identity_review_observations->>'document_format_observed' IS NOT NULL
                AND identity_review_observations->>'document_format_observed' IN ('legacy','new','undetermined')
                AND CASE
                    WHEN pg_catalog.jsonb_typeof(identity_review_observations->'document_expires_at_observed')='null' THEN true
                    WHEN pg_catalog.jsonb_typeof(identity_review_observations->'document_expires_at_observed')='string' THEN
                        identity_review_observations->>'document_expires_at_observed' ~
                        '^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}([.][0-9]+)?Z$'
                    ELSE false
                END
                AND pg_catalog.jsonb_typeof(identity_review_observations->'identity_review_checklist')='object'
                AND (identity_review_observations->'identity_review_checklist') ?& ARRAY[
                    'names_match','rut_matches','birth_date_and_majority_match',
                    'portrait_matches_selfie','evidence_legible','document_unexpired',
                    'issue_date_and_format_verified'
                ]
                AND identity_review_observations->'identity_review_checklist' = pg_catalog.jsonb_build_object(
                    'names_match', identity_review_observations->'identity_review_checklist'->'names_match',
                    'rut_matches', identity_review_observations->'identity_review_checklist'->'rut_matches',
                    'birth_date_and_majority_match', identity_review_observations->'identity_review_checklist'->'birth_date_and_majority_match',
                    'portrait_matches_selfie', identity_review_observations->'identity_review_checklist'->'portrait_matches_selfie',
                    'evidence_legible', identity_review_observations->'identity_review_checklist'->'evidence_legible',
                    'document_unexpired', identity_review_observations->'identity_review_checklist'->'document_unexpired',
                    'issue_date_and_format_verified', identity_review_observations->'identity_review_checklist'->'issue_date_and_format_verified'
                )
                AND pg_catalog.jsonb_typeof(identity_review_observations->'identity_review_checklist'->'names_match')='boolean'
                AND pg_catalog.jsonb_typeof(identity_review_observations->'identity_review_checklist'->'rut_matches')='boolean'
                AND pg_catalog.jsonb_typeof(identity_review_observations->'identity_review_checklist'->'birth_date_and_majority_match')='boolean'
                AND pg_catalog.jsonb_typeof(identity_review_observations->'identity_review_checklist'->'portrait_matches_selfie')='boolean'
                AND pg_catalog.jsonb_typeof(identity_review_observations->'identity_review_checklist'->'evidence_legible')='boolean'
                AND pg_catalog.jsonb_typeof(identity_review_observations->'identity_review_checklist'->'document_unexpired')='boolean'
                AND pg_catalog.jsonb_typeof(identity_review_observations->'identity_review_checklist'->'issue_date_and_format_verified')='boolean'
            )
        );

-- Every new identity review event must carry the exact allowlisted observations
-- already written to its case row in the same transaction.
CREATE FUNCTION public.document_verification_identity_review_audit_guard()
RETURNS trigger
LANGUAGE plpgsql
SET search_path = pg_catalog
AS $$
DECLARE
    v_state text;
    v_revision bigint;
    v_reviewer uuid;
    v_format text;
    v_expiry timestamptz;
BEGIN
    IF NEW.purpose='identity' AND NEW.action='review_decision' THEN
        IF NEW.identity_review_observations IS NULL THEN
            RAISE EXCEPTION 'identity review decision requires observed metadata and checklist'
                USING ERRCODE='23514';
        END IF;
        SELECT c.state,c.revision,c.reviewer_id,c.document_format_observed,c.document_expires_at
          INTO v_state,v_revision,v_reviewer,v_format,v_expiry
          FROM public.document_verification_case c
         WHERE c.id=NEW.case_id AND c.purpose=NEW.purpose AND c.subject_id=NEW.subject_id;
        IF NOT FOUND
           OR v_state IS DISTINCT FROM NEW.to_state
           OR v_state IS DISTINCT FROM NEW.result_code
           OR v_revision IS DISTINCT FROM NEW.revision
           OR v_reviewer IS DISTINCT FROM NEW.actor_id
           OR v_format IS DISTINCT FROM NEW.identity_review_observations->>'document_format_observed'
           OR v_expiry IS DISTINCT FROM NULLIF(NEW.identity_review_observations->>'document_expires_at_observed','')::timestamptz THEN
            RAISE EXCEPTION 'identity review event does not match the case transition'
                USING ERRCODE='23514';
        END IF;
    ELSIF NEW.identity_review_observations IS NOT NULL THEN
        RAISE EXCEPTION 'identity review observations are only valid on identity review decisions'
            USING ERRCODE='23514';
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER document_verification_identity_review_audit_guard
BEFORE INSERT ON public.document_verification_event
FOR EACH ROW EXECUTE FUNCTION public.document_verification_identity_review_audit_guard();
REVOKE ALL ON FUNCTION public.document_verification_identity_review_audit_guard() FROM PUBLIC;

DO $runtime$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname='espacigo_runtime') THEN
        GRANT UPDATE (document_format_observed)
            ON public.document_verification_case TO espacigo_runtime;
        GRANT INSERT (identity_review_observations)
            ON public.document_verification_event TO espacigo_runtime;
    END IF;
END
$runtime$;
