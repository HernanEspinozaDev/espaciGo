-- F5: ownership changes permanently detach prior document cases without adding
-- a user-facing case state. The current owner must create a new case; the old
-- case remains bound to its historical subject and enters the approved 30-day
-- replacement retention window.

ALTER TABLE public.document_verification_case
    ADD COLUMN superseded_at timestamptz,
    ADD CONSTRAINT document_verification_case_superseded_scope_ck
        CHECK (superseded_at IS NULL OR purpose='space');

DROP INDEX public.document_verification_one_open_space_uq;
CREATE UNIQUE INDEX document_verification_one_open_space_subject_uq
    ON public.document_verification_case(space_id,purpose,subject_id)
    WHERE purpose='space'
      AND superseded_at IS NULL
      AND state IN ('borrador','enviada','en_revision','requiere_antecedentes');

COMMENT ON COLUMN public.document_verification_case.superseded_at IS
    'Internal terminal marker for an ownership-change case replacement; does not add a user-facing case state.';
COMMENT ON INDEX public.document_verification_one_open_space_subject_uq IS
    'One active open space case per historical owner subject; current ownership is verified on every operation.';

ALTER TABLE public.document_verification_event
    DROP CONSTRAINT document_verification_event_action_check;
ALTER TABLE public.document_verification_event
    ADD CONSTRAINT document_verification_event_action_check
        CHECK (action IN ('case_created','case_transitioned','case_superseded','file_added','scan_result','review_decision','revoked','expired','retention_deleted'));

CREATE FUNCTION public.document_verification_space_owner_changed() RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    changed_at timestamptz := clock_timestamp();
    file_ids uuid[];
    retention_deadlines timestamptz[];
BEGIN
    IF NEW.propietario_id IS NOT DISTINCT FROM OLD.propietario_id THEN
        RETURN NEW;
    END IF;

    WITH superseded AS (
        UPDATE public.document_verification_case
        SET superseded_at=changed_at, revision=revision+1, updated_at=changed_at
        WHERE purpose='space'
          AND space_id=NEW.id
          AND subject_id<>NEW.propietario_id
          AND superseded_at IS NULL
        RETURNING id,purpose,subject_id,state,revision
    ), audit_events AS (
        INSERT INTO public.document_verification_event(
            case_id,purpose,subject_id,actor_kind,action,from_state,to_state,
            result_code,reason_code,revision
        )
        SELECT id,purpose,subject_id,'system','case_superseded',state,state,
               state,'cambio_titularidad',revision
        FROM superseded
        RETURNING case_id
    ), updated_files AS (
        UPDATE public.document_verification_file f
        SET retention_at=changed_at + interval '30 days',
            retention_reason_code='rejected_or_replaced',
            updated_at=changed_at
        FROM superseded s
        WHERE f.case_id=s.id
        RETURNING f.id,f.case_id,f.retention_at
    ), cancelled_scan_jobs AS (
        UPDATE public.document_verification_job j
        SET state='completado',lease_until=NULL,updated_at=changed_at
        WHERE j.kind='scan'
          AND j.state IN ('pendiente','procesando','reintentar')
          AND j.file_id IN (SELECT id FROM updated_files)
        RETURNING j.file_id
    )
    SELECT array_agg(uf.id ORDER BY uf.id) FILTER (WHERE uf.id IS NOT NULL),
           array_agg(uf.retention_at ORDER BY uf.id) FILTER (WHERE uf.id IS NOT NULL)
    INTO file_ids,retention_deadlines
    FROM superseded s
    LEFT JOIN updated_files uf ON uf.case_id=s.id;

    INSERT INTO public.document_verification_job(file_id,kind)
    SELECT f.file_id,'retention_delete'
    FROM unnest(file_ids) AS f(file_id)
    ON CONFLICT DO NOTHING;

    UPDATE public.document_verification_job j
    SET state='pendiente',attempts=0,available_at=f.retention_at,
        lease_until=NULL,last_error_code=NULL,updated_at=changed_at
    FROM unnest(file_ids,retention_deadlines) AS f(file_id,retention_at)
    WHERE j.file_id=f.file_id
      AND j.kind='retention_delete'
      AND j.state IN ('pendiente','procesando','reintentar');

    RETURN NEW;
END;
$$;

REVOKE EXECUTE ON FUNCTION public.document_verification_space_owner_changed() FROM PUBLIC;
CREATE TRIGGER document_verification_space_owner_changed_trg
    AFTER UPDATE OF propietario_id ON public.espacio
    FOR EACH ROW
    WHEN (OLD.propietario_id IS DISTINCT FROM NEW.propietario_id)
    EXECUTE FUNCTION public.document_verification_space_owner_changed();

DO $grants$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname='espacigo_runtime') THEN
        GRANT UPDATE(superseded_at) ON public.document_verification_case TO espacigo_runtime;
    END IF;
END
$grants$;
