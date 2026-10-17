-- Cloud import sanitation. Fixed target and administrative identity are checked before mutation.
SET gcp03.copy_quarantine = 'enabled';
BEGIN;

DO $$
BEGIN
  IF current_database() IS DISTINCT FROM 'espacigo_scratch' OR current_user IS DISTINCT FROM 'postgres' THEN
    RAISE EXCEPTION 'quarantine target identity mismatch';
  END IF;
  IF current_setting('gcp03.copy_quarantine', true) IS DISTINCT FROM 'enabled' THEN
    RAISE EXCEPTION 'copy quarantine guard is absent';
  END IF;
  IF to_regclass('public.sesion') IS NULL
     OR to_regclass('public.token_accion') IS NULL
     OR to_regclass('public.aviso_local') IS NULL
     OR to_regclass('public.aviso_local_ciclo') IS NULL
     OR to_regclass('public.outbox_evento_local') IS NULL THEN
    RAISE EXCEPTION 'expected V1-V61 copy tables are absent';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'espacigo_runtime') THEN
    RAISE EXCEPTION 'runtime role is absent; copy cannot be quarantined';
  END IF;
END;
$$;

UPDATE public.sesion
SET revocada_en = greatest(transaction_timestamp(), creada_en)
WHERE revocada_en IS NULL;

UPDATE public.token_accion
SET invalidado_en = greatest(transaction_timestamp(), creado_en)
WHERE consumido_en IS NULL AND invalidado_en IS NULL;

CREATE TEMP TABLE gcp03_quarantined_notices ON COMMIT DROP AS
WITH cancelled AS (
  UPDATE public.aviso_local
     SET estado='cancelada',
         cancelada_en=greatest(transaction_timestamp(),creada_en),
         retirar_en=greatest(transaction_timestamp(),creada_en)+interval '30 days',
         lease_hasta=NULL,
         codigo_error='gcp03_snapshot_quarantine'
   WHERE estado IN ('pendiente','procesando')
   RETURNING id,ciclo
)
SELECT id,ciclo FROM cancelled;

UPDATE public.aviso_local_ciclo cycle
   SET estado='cancelada',
       finalizada_en=greatest(transaction_timestamp(),cycle.iniciada_en),
       codigo_resultado='gcp03_snapshot_quarantine'
  FROM gcp03_quarantined_notices notice
 WHERE cycle.aviso_id=notice.id
   AND cycle.ciclo=notice.ciclo
   AND cycle.estado='pendiente';

-- Preserve any remaining identity event rows for audit, clear leases, and make
-- them non-runnable by disabling the copied runtime login. No sender is started.
UPDATE public.outbox_evento_local SET lease_hasta=NULL WHERE lease_hasta IS NOT NULL;
ALTER ROLE espacigo_runtime NOLOGIN;

COMMIT;

SELECT jsonb_build_object(
  'valid_sessions', (SELECT count(*) FROM public.sesion WHERE revocada_en IS NULL AND expira_en>transaction_timestamp()),
  'usable_action_tokens', (SELECT count(*) FROM public.token_accion WHERE consumido_en IS NULL AND invalidado_en IS NULL AND expira_en>transaction_timestamp()),
  'pending_notices', (SELECT count(*) FROM public.aviso_local WHERE estado IN ('pendiente','procesando')),
  'quarantined_notices', (SELECT count(*) FROM public.aviso_local WHERE estado='cancelada' AND codigo_error='gcp03_snapshot_quarantine'),
  'undelivered_identity_events', (SELECT count(*) FROM public.outbox_evento_local WHERE entregada_en IS NULL AND cancelada_en IS NULL AND fallo_terminal_en IS NULL),
  'identity_event_leases', (SELECT count(*) FROM public.outbox_evento_local WHERE lease_hasta IS NOT NULL AND lease_hasta>transaction_timestamp()),
  'runtime_role_login', (SELECT rolcanlogin FROM pg_roles WHERE rolname='espacigo_runtime')
)::text;
