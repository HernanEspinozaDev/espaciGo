-- Run inside the primary V64 import transaction as espacigo_schema_owner.
DO $$
BEGIN
  IF current_database() IS DISTINCT FROM 'espacigo'
     OR session_user IS DISTINCT FROM 'espacigo_migrator_login'
     OR current_user IS DISTINCT FROM 'espacigo_schema_owner' THEN
    RAISE EXCEPTION 'V64 transfer target identity mismatch';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname='espacigo_runtime' AND rolcanlogin=false) THEN
    RAISE EXCEPTION 'V64 transfer runtime role is not safely disabled';
  END IF;
END;
$$;

UPDATE public.sesion
SET revocada_en = greatest(transaction_timestamp(), creada_en)
WHERE revocada_en IS NULL;

UPDATE public.token_accion
SET invalidado_en = greatest(transaction_timestamp(), creado_en)
WHERE consumido_en IS NULL AND invalidado_en IS NULL;

UPDATE public.aviso_local
   SET estado='cancelada',
       cancelada_en=greatest(transaction_timestamp(),creada_en),
       retirar_en=greatest(transaction_timestamp(),creada_en)+interval '30 days',
       lease_hasta=NULL,
       codigo_error='gcp03_v64_transfer_quarantine'
 WHERE estado IN ('pendiente','procesando');

UPDATE public.aviso_local_ciclo cycle
   SET estado='cancelada',
       finalizada_en=greatest(transaction_timestamp(),cycle.iniciada_en),
       codigo_resultado='gcp03_v64_transfer_quarantine'
  FROM public.aviso_local notice
 WHERE cycle.aviso_id=notice.id
   AND cycle.ciclo=notice.ciclo
   AND notice.estado='cancelada'
   AND notice.codigo_error='gcp03_v64_transfer_quarantine'
   AND cycle.estado='pendiente';

-- No old delivery work is imported; this remains a defensive postcondition.
UPDATE public.outbox_evento_local SET lease_hasta=NULL WHERE lease_hasta IS NOT NULL;
