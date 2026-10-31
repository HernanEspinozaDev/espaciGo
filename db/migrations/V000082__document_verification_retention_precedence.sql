-- Preserve the stricter approved retention deadline when a space owner changes.
-- V80's replacement deadline applies only when no earlier mandatory deadline exists.
-- Malware retention is capped at 24 hours after inspection; absent or future
-- inspection timestamps are treated as immediately due, never as an extension.
-- Ownership transfer and retention deletion both fence on the space row before
-- case/file/job rows; the worker holds those locks through storage deletion.
CREATE OR REPLACE FUNCTION public.document_verification_space_owner_changed() RETURNS trigger
LANGUAGE plpgsql
SET search_path = pg_catalog, pg_temp
AS $function$
DECLARE
    changed_at timestamptz := clock_timestamp();
    replacement_deadline timestamptz;
    file_ids uuid[];
    retention_deadlines timestamptz[];
BEGIN
    IF NEW.propietario_id IS NOT DISTINCT FROM OLD.propietario_id THEN
        RETURN NEW;
    END IF;
    replacement_deadline := changed_at + interval '30 days';

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
        SET retention_at = CASE
                WHEN f.last_error_code='malware_detected'
                  OR f.retention_reason_code='malicious_file'
                    THEN LEAST(COALESCE(f.retention_at,policy.malware_deadline),policy.malware_deadline)
                WHEN f.retention_at IS NULL OR f.retention_at > replacement_deadline
                    THEN replacement_deadline
                ELSE f.retention_at
            END,
            retention_reason_code = CASE
                WHEN f.last_error_code='malware_detected'
                  OR f.retention_reason_code='malicious_file'
                    THEN 'malicious_file'
                WHEN f.retention_at IS NOT NULL AND f.retention_at <= replacement_deadline
                    THEN f.retention_reason_code
                ELSE 'rejected_or_replaced'
            END,
            updated_at=changed_at
        FROM superseded s
        CROSS JOIN LATERAL (
            SELECT CASE
                WHEN f.inspected_at IS NULL OR f.inspected_at > changed_at THEN changed_at
                ELSE f.inspected_at + interval '24 hours'
            END AS malware_deadline
        ) policy
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

    -- A completed retention job is durable evidence that this object's deletion
    -- already succeeded; never create another deletion job for that file.
    INSERT INTO public.document_verification_job(file_id,kind)
    SELECT f.file_id,'retention_delete'
    FROM unnest(file_ids) AS f(file_id)
    WHERE NOT EXISTS (
        SELECT 1
        FROM public.document_verification_job AS done
        WHERE done.file_id=f.file_id
          AND done.kind='retention_delete'
          AND done.state='completado'
    )
    ON CONFLICT DO NOTHING;

    UPDATE public.document_verification_job j
    SET state='pendiente',attempts=0,available_at=f.retention_at,
        lease_until=NULL,last_error_code=NULL,updated_at=changed_at
    FROM unnest(file_ids,retention_deadlines) AS f(file_id,retention_at)
    WHERE j.file_id=f.file_id
      AND j.kind='retention_delete'
      AND j.state IN ('pendiente','procesando','reintentar');

    -- If an older trigger left both a completed history row and an active
    -- duplicate, preserve the terminal deletion and retire that duplicate.
    UPDATE public.document_verification_job j
    SET state='completado',lease_until=NULL,last_error_code=NULL,updated_at=changed_at
    WHERE j.file_id = ANY(file_ids)
      AND j.kind='retention_delete'
      AND j.state IN ('pendiente','procesando','reintentar')
      AND EXISTS (
          SELECT 1
          FROM public.document_verification_job AS done
          WHERE done.file_id=j.file_id
            AND done.kind='retention_delete'
            AND done.state='completado'
      );

    RETURN NEW;
END;
$function$;

REVOKE EXECUTE ON FUNCTION public.document_verification_space_owner_changed() FROM PUBLIC;
COMMENT ON FUNCTION public.document_verification_space_owner_changed() IS
    'Closes prior-owner space cases atomically; preserves earlier mandatory file-retention deadlines, caps malware retention at 24 hours after inspection (immediate if its timestamp is absent or future), and schedules other replacement retention no later than 30 days after transfer.';
