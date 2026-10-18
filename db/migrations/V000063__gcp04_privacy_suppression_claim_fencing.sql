-- GCP-04-W1: durable claims for privacy-suppression evidence deletion.
-- V1-V62 remain immutable; the lease generation fences SQL acknowledgements.
ALTER TABLE public.baja_archivo_pendiente_local
    ADD COLUMN claim_owner uuid,
    ADD COLUMN claim_hasta timestamptz,
    ADD COLUMN claim_generacion bigint NOT NULL DEFAULT 0;

ALTER TABLE public.baja_archivo_pendiente_local
    ADD CONSTRAINT baja_archivo_claim_lease_pair_ck
        CHECK ((claim_owner IS NULL) = (claim_hasta IS NULL)),
    ADD CONSTRAINT baja_archivo_claim_generacion_ck
        CHECK (claim_generacion >= 0);

DROP INDEX public.baja_archivo_pendiente_claim_idx;
CREATE INDEX baja_archivo_pendiente_claim_idx
    ON public.baja_archivo_pendiente_local(disponible_en,claim_hasta,ejecucion_id,evidencia_id)
    WHERE completada_en IS NULL;

COMMENT ON COLUMN public.baja_archivo_pendiente_local.claim_owner IS
    'Unique worker-process UUID for the current privacy evidence deletion lease.';
COMMENT ON COLUMN public.baja_archivo_pendiente_local.claim_hasta IS
    'Lease deadline; expired claims may be atomically taken over.';
COMMENT ON COLUMN public.baja_archivo_pendiente_local.claim_generacion IS
    'Monotonic per-row fencing generation; stale workers cannot acknowledge a takeover.';
