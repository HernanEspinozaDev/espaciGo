-- F5: purpose-scoped document verification metadata, durable jobs and reviewer grants.
CREATE TABLE public.document_verification_case (
 id uuid PRIMARY KEY,
 purpose text NOT NULL CHECK (purpose IN ('identity','space')),
 subject_id uuid NOT NULL REFERENCES public.usuario(id) ON DELETE RESTRICT,
 space_id uuid,
 state text NOT NULL DEFAULT 'borrador' CHECK (state IN ('borrador','enviada','en_revision','requiere_antecedentes','aprobada','rechazada','vencida','revocada')),
 revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0),
 reviewer_id uuid REFERENCES public.usuario(id) ON DELETE RESTRICT,
 verified_at timestamptz,
 verified_until timestamptz,
 document_expires_at timestamptz,
 revoked_at timestamptz,
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
 CONSTRAINT document_verification_case_scope_ck CHECK ((purpose='identity' AND space_id IS NULL) OR (purpose='space' AND space_id IS NOT NULL)),
 CONSTRAINT document_verification_case_space_fk FOREIGN KEY(space_id) REFERENCES public.espacio(id) ON DELETE RESTRICT,
 CONSTRAINT document_verification_case_reviewer_ck CHECK ((state IN ('borrador','enviada') AND reviewer_id IS NULL) OR (state NOT IN ('borrador','enviada') AND reviewer_id IS NOT NULL)),
 CONSTRAINT document_verification_case_verified_ck CHECK ((state IN ('aprobada','vencida','revocada') AND verified_at IS NOT NULL AND verified_until > verified_at) OR (state NOT IN ('aprobada','vencida','revocada') AND verified_at IS NULL AND verified_until IS NULL)),
 CONSTRAINT document_verification_case_revoked_ck CHECK ((state='revocada' AND revoked_at IS NOT NULL AND revoked_at >= verified_at) OR (state<>'revocada' AND revoked_at IS NULL)),
 CONSTRAINT document_verification_case_id_scope_uq UNIQUE(id,purpose,subject_id,space_id)
);
CREATE UNIQUE INDEX document_verification_one_open_identity_uq ON public.document_verification_case(subject_id,purpose) WHERE purpose='identity' AND state IN ('borrador','enviada','en_revision','requiere_antecedentes');
CREATE UNIQUE INDEX document_verification_one_open_space_uq ON public.document_verification_case(space_id,purpose) WHERE purpose='space' AND state IN ('borrador','enviada','en_revision','requiere_antecedentes');
CREATE INDEX document_verification_identity_current_idx ON public.document_verification_case(subject_id,verified_until DESC) WHERE purpose='identity' AND state='aprobada';

CREATE TABLE public.document_verification_file (
 id uuid PRIMARY KEY,
 case_id uuid NOT NULL REFERENCES public.document_verification_case(id) ON DELETE RESTRICT,
 state text NOT NULL DEFAULT 'pendiente' CHECK(state IN ('pendiente','limpio','bloqueado','error')),
 size_bytes bigint NOT NULL CHECK(size_bytes BETWEEN 1 AND 10485760),
 mime_type text NOT NULL CHECK(mime_type IN ('image/jpeg','image/png','application/pdf')),
 pages integer NOT NULL CHECK(pages BETWEEN 0 AND 20),
 content_sha256 bytea NOT NULL CHECK(octet_length(content_sha256)=32),
 scan_attempts smallint NOT NULL DEFAULT 0 CHECK(scan_attempts BETWEEN 0 AND 5),
 scanner_version text,
 signatures_at timestamptz,
 inspected_at timestamptz,
 last_error_code text CHECK(last_error_code IS NULL OR last_error_code IN ('scanner_unavailable','scanner_malformed','signatures_stale','malware_detected','inspection_rejected','retention_store_error')),
 retention_at timestamptz,
 retention_reason_code text CHECK(retention_reason_code IS NULL OR retention_reason_code IN ('abandoned_draft','malicious_file','inspection_error','identity_decision','space_expiry','rejected_or_replaced','revoked')),
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
 CONSTRAINT document_verification_file_clean_ck CHECK(state<>'limpio' OR (inspected_at IS NOT NULL AND scanner_version IS NOT NULL AND signatures_at IS NOT NULL)),
 UNIQUE(id,case_id)
);
CREATE INDEX document_verification_file_case_idx ON public.document_verification_file(case_id);
CREATE INDEX document_verification_file_retention_idx ON public.document_verification_file(retention_at) WHERE retention_at IS NOT NULL;

CREATE TABLE public.document_verification_job (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 file_id uuid NOT NULL REFERENCES public.document_verification_file(id) ON DELETE CASCADE,
 kind text NOT NULL CHECK(kind IN ('scan','retention_delete')),
 state text NOT NULL DEFAULT 'pendiente' CHECK(state IN ('pendiente','procesando','reintentar','completado')),
 attempts smallint NOT NULL DEFAULT 0 CHECK(attempts BETWEEN 0 AND 5),
 available_at timestamptz NOT NULL DEFAULT now(), lease_until timestamptz,
 last_error_code text CHECK(last_error_code IS NULL OR last_error_code IN ('scanner_unavailable','scanner_malformed','signatures_stale','malware_detected','inspection_rejected','retention_store_error')),
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
 CONSTRAINT document_verification_job_lease_ck CHECK((state='procesando')=(lease_until IS NOT NULL))
);
CREATE UNIQUE INDEX document_verification_job_active_uq ON public.document_verification_job(file_id,kind) WHERE state IN ('pendiente','procesando','reintentar');
CREATE INDEX document_verification_job_ready_idx ON public.document_verification_job(available_at,id) WHERE state IN ('pendiente','reintentar');

CREATE TABLE public.document_verification_reviewer_grant (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 reviewer_id uuid NOT NULL REFERENCES public.usuario(id) ON DELETE RESTRICT,
 capability text NOT NULL CHECK(capability IN ('review_identity','review_space')),
 scope_kind text NOT NULL CHECK(scope_kind IN ('identity_subject','space')),
 scope_id uuid NOT NULL,
 granted_by uuid NOT NULL REFERENCES public.usuario(id) ON DELETE RESTRICT,
 granted_at timestamptz NOT NULL DEFAULT now(),
 granted_reason_code text NOT NULL CHECK(granted_reason_code IN ('role_assignment','temporary_coverage','reviewer_rotation')),
 expires_at timestamptz,
 revoked_at timestamptz,
 revoked_by uuid REFERENCES public.usuario(id) ON DELETE RESTRICT,
 revocation_reason_code text CHECK(revocation_reason_code IS NULL OR revocation_reason_code IN ('role_assignment_ended','temporary_coverage_ended','reviewer_rotation','access_review')),
 CONSTRAINT document_verification_reviewer_grant_scope_ck CHECK((capability='review_identity' AND scope_kind='identity_subject') OR (capability='review_space' AND scope_kind='space')),
 CONSTRAINT document_verification_reviewer_grant_expiry_ck CHECK(expires_at IS NULL OR expires_at>granted_at),
 CONSTRAINT document_verification_reviewer_grant_revocation_ck CHECK((revoked_at IS NULL AND revoked_by IS NULL AND revocation_reason_code IS NULL) OR (revoked_at IS NOT NULL AND revoked_by IS NOT NULL AND revocation_reason_code IS NOT NULL AND revoked_at>=granted_at))
);
CREATE UNIQUE INDEX document_verification_reviewer_grant_active_uq ON public.document_verification_reviewer_grant(reviewer_id,capability,scope_kind,scope_id) WHERE revoked_at IS NULL;
CREATE INDEX document_verification_reviewer_grant_actor_idx ON public.document_verification_reviewer_grant(reviewer_id,capability,scope_id) WHERE revoked_at IS NULL;

CREATE TABLE public.document_verification_event (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 -- IDs deliberately have no FK: immutable minimal audit rows outlive PII and fixture cleanup.
 case_id uuid NOT NULL,
 purpose text NOT NULL CHECK(purpose IN ('identity','space')),
 subject_id uuid NOT NULL,
 file_id uuid,
 actor_kind text NOT NULL DEFAULT 'user' CHECK(actor_kind IN ('user','scanner','system')),
 actor_id uuid,
 action text NOT NULL CHECK(action IN ('case_created','case_transitioned','file_added','scan_result','review_decision','revoked','expired','retention_deleted')),
 from_state text CHECK(from_state IS NULL OR from_state IN ('borrador','enviada','en_revision','requiere_antecedentes','aprobada','rechazada','vencida','revocada')),
 to_state text NOT NULL CHECK(to_state IN ('borrador','enviada','en_revision','requiere_antecedentes','aprobada','rechazada','vencida','revocada')),
 result_code text NOT NULL CHECK(result_code IN ('borrador','enviada','en_revision','requiere_antecedentes','aprobada','rechazada','vencida','revocada','pendiente','limpio','bloqueado','error','eliminado')),
 reason_code text CHECK(reason_code IS NULL OR reason_code IN ('documento_ilegible','documento_invalido','documento_vencido','antecedentes_incompletos','cambio_titularidad','conflicto_interes','malware_detectado','scanner_no_disponible','firmas_desactualizadas','solicitud_titular','retencion_vencida')),
 revision bigint NOT NULL CHECK(revision > 0), occurred_at timestamptz NOT NULL DEFAULT now(),
 CONSTRAINT document_verification_event_actor_ck CHECK((actor_kind='user' AND actor_id IS NOT NULL) OR (actor_kind IN ('scanner','system') AND actor_id IS NULL)),
 CONSTRAINT document_verification_event_file_action_ck CHECK((action IN ('file_added','scan_result','retention_deleted') AND file_id IS NOT NULL) OR (action NOT IN ('file_added','scan_result','retention_deleted') AND file_id IS NULL))
);
CREATE INDEX document_verification_event_case_idx ON public.document_verification_event(case_id,occurred_at,id);
CREATE INDEX document_verification_event_file_idx ON public.document_verification_event(file_id,occurred_at,id) WHERE file_id IS NOT NULL;

CREATE TABLE public.document_verification_reviewer_grant_event (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 grant_id uuid NOT NULL,
 reviewer_id uuid NOT NULL,
 capability text NOT NULL CHECK(capability IN ('review_identity','review_space')),
 scope_kind text NOT NULL CHECK(scope_kind IN ('identity_subject','space')),
 scope_id uuid NOT NULL,
 actor_id uuid NOT NULL,
 action text NOT NULL CHECK(action IN ('granted','revoked')),
 reason_code text NOT NULL CHECK(reason_code IN ('role_assignment','temporary_coverage','reviewer_rotation','role_assignment_ended','temporary_coverage_ended','access_review')),
 occurred_at timestamptz NOT NULL DEFAULT now()
);

CREATE FUNCTION public.document_verification_event_immutable() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'document verification events are append-only' USING ERRCODE='23514'; END; $$;
CREATE TRIGGER document_verification_event_no_update_delete BEFORE UPDATE OR DELETE ON public.document_verification_event FOR EACH ROW EXECUTE FUNCTION public.document_verification_event_immutable();
CREATE TRIGGER document_verification_event_no_truncate BEFORE TRUNCATE ON public.document_verification_event FOR EACH STATEMENT EXECUTE FUNCTION public.document_verification_event_immutable();
CREATE TRIGGER document_verification_grant_event_no_update_delete BEFORE UPDATE OR DELETE ON public.document_verification_reviewer_grant_event FOR EACH ROW EXECUTE FUNCTION public.document_verification_event_immutable();
CREATE TRIGGER document_verification_grant_event_no_truncate BEFORE TRUNCATE ON public.document_verification_reviewer_grant_event FOR EACH STATEMENT EXECUTE FUNCTION public.document_verification_event_immutable();
REVOKE EXECUTE ON FUNCTION public.document_verification_event_immutable() FROM PUBLIC;

CREATE FUNCTION public.document_verification_reviewer_grant_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='DELETE' THEN
  RAISE EXCEPTION 'reviewer grants must be revoked, not deleted' USING ERRCODE='23514';
 END IF;
 PERFORM 1 FROM public.usuario WHERE id=NEW.reviewer_id AND estado='activo' FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'reviewer must be active' USING ERRCODE='23514'; END IF;
 IF TG_OP='INSERT' THEN
  PERFORM 1 FROM public.usuario WHERE id=NEW.granted_by AND estado='activo';
  IF NOT FOUND THEN RAISE EXCEPTION 'grantor must be active' USING ERRCODE='23514'; END IF;
  IF NEW.scope_kind='identity_subject' THEN
   PERFORM 1 FROM public.usuario WHERE id=NEW.scope_id AND estado='activo';
  ELSE
   PERFORM 1 FROM public.espacio WHERE id=NEW.scope_id;
  END IF;
  IF NOT FOUND THEN RAISE EXCEPTION 'review grant scope does not exist' USING ERRCODE='23514'; END IF;
 ELSIF ROW(NEW.id,NEW.reviewer_id,NEW.capability,NEW.scope_kind,NEW.scope_id,NEW.granted_by,NEW.granted_at,NEW.granted_reason_code,NEW.expires_at)
    IS DISTINCT FROM ROW(OLD.id,OLD.reviewer_id,OLD.capability,OLD.scope_kind,OLD.scope_id,OLD.granted_by,OLD.granted_at,OLD.granted_reason_code,OLD.expires_at)
    OR OLD.revoked_at IS NOT NULL OR NEW.revoked_at IS NULL THEN
  RAISE EXCEPTION 'reviewer grant may only transition once to revoked' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END; $$;
CREATE TRIGGER document_verification_reviewer_grant_guard BEFORE INSERT OR UPDATE OR DELETE ON public.document_verification_reviewer_grant FOR EACH ROW EXECUTE FUNCTION public.document_verification_reviewer_grant_guard();
REVOKE EXECUTE ON FUNCTION public.document_verification_reviewer_grant_guard() FROM PUBLIC;

CREATE FUNCTION public.document_verification_reviewer_grant_audit() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='INSERT' THEN
  INSERT INTO public.document_verification_reviewer_grant_event(grant_id,reviewer_id,capability,scope_kind,scope_id,actor_id,action,reason_code)
  VALUES(NEW.id,NEW.reviewer_id,NEW.capability,NEW.scope_kind,NEW.scope_id,NEW.granted_by,'granted',NEW.granted_reason_code);
 ELSE
  INSERT INTO public.document_verification_reviewer_grant_event(grant_id,reviewer_id,capability,scope_kind,scope_id,actor_id,action,reason_code)
  VALUES(NEW.id,NEW.reviewer_id,NEW.capability,NEW.scope_kind,NEW.scope_id,NEW.revoked_by,'revoked',NEW.revocation_reason_code);
 END IF;
 RETURN NEW;
END; $$;
CREATE TRIGGER document_verification_reviewer_grant_audit AFTER INSERT OR UPDATE ON public.document_verification_reviewer_grant FOR EACH ROW EXECUTE FUNCTION public.document_verification_reviewer_grant_audit();
REVOKE EXECUTE ON FUNCTION public.document_verification_reviewer_grant_audit() FROM PUBLIC;

REVOKE ALL ON TABLE public.document_verification_case, public.document_verification_file, public.document_verification_job, public.document_verification_reviewer_grant, public.document_verification_event, public.document_verification_reviewer_grant_event FROM PUBLIC;
REVOKE ALL ON FUNCTION public.document_verification_event_immutable(), public.document_verification_reviewer_grant_guard(), public.document_verification_reviewer_grant_audit() FROM PUBLIC;

DO $grants$ BEGIN
 IF EXISTS(SELECT 1 FROM pg_catalog.pg_roles WHERE rolname='espacigo_runtime') THEN
  GRANT SELECT ON public.document_verification_case,public.document_verification_file,public.document_verification_reviewer_grant TO espacigo_runtime;
  GRANT INSERT(id,purpose,subject_id,space_id,document_expires_at) ON public.document_verification_case TO espacigo_runtime;
  GRANT UPDATE(state,revision,reviewer_id,verified_at,verified_until,document_expires_at,revoked_at,updated_at) ON public.document_verification_case TO espacigo_runtime;
  GRANT INSERT(id,case_id,size_bytes,mime_type,pages,content_sha256) ON public.document_verification_file TO espacigo_runtime;
  GRANT UPDATE(state,scan_attempts,scanner_version,signatures_at,inspected_at,last_error_code,retention_at,retention_reason_code,updated_at) ON public.document_verification_file TO espacigo_runtime;
  GRANT SELECT ON public.document_verification_job TO espacigo_runtime;
  GRANT INSERT(file_id,kind) ON public.document_verification_job TO espacigo_runtime;
  GRANT UPDATE(state,attempts,available_at,lease_until,last_error_code,updated_at) ON public.document_verification_job TO espacigo_runtime;
  GRANT SELECT,INSERT ON public.document_verification_event TO espacigo_runtime;
  GRANT USAGE ON SEQUENCE public.document_verification_event_id_seq TO espacigo_runtime;
 END IF;
END $grants$;
