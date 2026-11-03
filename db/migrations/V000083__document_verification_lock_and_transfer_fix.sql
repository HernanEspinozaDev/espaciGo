-- Forward-only repair for V82's UPDATE target alias reference and the
-- column-level UPDATE permission required by runtime FOR SHARE reads.
DO $migration_identity$
BEGIN
    IF NOT (
        (session_user = 'espacigo_migrator_login' AND current_user = 'espacigo_schema_owner')
        OR (current_database() = 'espacigo_f5_gate_20261101'
            AND session_user = 'postgres' AND current_user = 'postgres'
            AND current_setting('application_name', true) = 'f5-confirmed:espacigo_f5_gate_20261101')
    ) THEN
        RAISE EXCEPTION 'V83 requires the dedicated migrator identity or the confirmed F5 versioned migration context';
    END IF;
END;
$migration_identity$;

DO $runtime_role_contract$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_catalog.pg_roles
        WHERE rolname = 'espacigo_runtime'
          AND NOT rolcanlogin
          AND NOT rolsuper
          AND NOT rolcreatedb
          AND NOT rolcreaterole
          AND NOT rolreplication
          AND NOT rolbypassrls
          AND NOT rolinherit
    ) THEN
        RAISE EXCEPTION 'V83 requires the pre-provisioned restricted espacigo_runtime role';
    END IF;
END;
$runtime_role_contract$;

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
    ), file_policy AS (
        -- Keep the updated table out of UPDATE ... FROM's lateral scope: derive
        -- deadlines in a separate read joined to the rows superseded above.
        SELECT f.id AS file_id,
               CASE
                   WHEN f.inspected_at IS NULL OR f.inspected_at > changed_at THEN changed_at
                   ELSE f.inspected_at + interval '24 hours'
               END AS malware_deadline
        FROM public.document_verification_file f
        JOIN superseded s ON s.id=f.case_id
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
                    THEN CASE
                        WHEN f.retention_at IS NOT NULL
                          AND f.retention_at < policy.malware_deadline
                            THEN f.retention_reason_code
                        ELSE 'malicious_file'
                    END
                WHEN f.retention_at IS NOT NULL AND f.retention_at <= replacement_deadline
                    THEN f.retention_reason_code
                ELSE 'rejected_or_replaced'
            END,
            updated_at=changed_at
        FROM file_policy policy
        WHERE f.id=policy.file_id
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

    -- Require the atomic deletion audit event: scan cancellation also marks a
    -- retention job completed, but does not mean the object was deleted.
    INSERT INTO public.document_verification_job(file_id,kind)
    SELECT f.file_id,'retention_delete'
    FROM unnest(file_ids) AS f(file_id)
    WHERE NOT EXISTS (
        SELECT 1
        FROM public.document_verification_job AS done
        WHERE done.file_id=f.file_id
          AND done.kind='retention_delete'
          AND done.state='completado'
          AND EXISTS (
              SELECT 1
              FROM public.document_verification_event e
              WHERE e.file_id=done.file_id
                AND e.action='retention_deleted'
          )
    )
    ON CONFLICT DO NOTHING;

    UPDATE public.document_verification_job j
    SET state='pendiente',attempts=0,available_at=f.retention_at,
        lease_until=NULL,last_error_code=NULL,updated_at=changed_at
    FROM unnest(file_ids,retention_deadlines) AS f(file_id,retention_at)
    WHERE j.file_id=f.file_id
      AND j.kind='retention_delete'
      AND j.state IN ('pendiente','procesando','reintentar');

    -- If a successful deletion and an active duplicate coexist, preserve the
    -- deletion result and retire that duplicate.
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
            AND EXISTS (
                SELECT 1
                FROM public.document_verification_event e
                WHERE e.file_id=done.file_id
                  AND e.action='retention_deleted'
            )
      );

    RETURN NEW;
END;
$function$;

-- A locking SELECT needs SELECT plus UPDATE on at least one column. Grant only
-- the immutable primary-key column; the V79 guard prevents runtime mutation.
GRANT UPDATE (id) ON TABLE public.document_verification_reviewer_grant TO espacigo_runtime;

COMMENT ON FUNCTION public.document_verification_space_owner_changed() IS
    'Closes prior-owner space cases atomically; preserves earlier mandatory file-retention deadlines, caps malware retention at 24 hours after inspection (immediate if its timestamp is absent or future), and schedules other replacement retention no later than 30 days after transfer.';

-- The migrator records this version in the same transaction. Restore the
-- schema-owner role so the ledger write remains within its explicit contract.
SET LOCAL ROLE espacigo_schema_owner;
