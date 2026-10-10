-- LOCAL-ADMIN-01 / ADMIN-BE-01: allow idempotent audit facts for the
-- already-persisted synthetic administrative financial mutations. This
-- extends the existing append-only ledger; it does not add an event outbox.
ALTER TABLE public.evento_auditoria_local
    DROP CONSTRAINT evento_auditoria_idempotencia_ck;

ALTER TABLE public.evento_auditoria_local
    ADD CONSTRAINT evento_auditoria_idempotencia_ck CHECK (
        clave_idempotencia IS NULL OR (
            length(btrim(clave_idempotencia)) BETWEEN 1 AND 200
            AND ((accion IN ('privacy.suppression.review','privacy.suppression.execute') AND recurso_tipo='solicitud_titular')
              OR (accion='identity.credential_notice.reopen' AND recurso_tipo='outbox_evento')
              OR (accion='reputation.review.moderate' AND recurso_tipo='reporte_resena')
              OR (accion='local.damage_claim.resolve' AND recurso_tipo='reclamo_dano')
              OR (accion IN ('local.finance.decision','local.finance.guarantee.operation','local.finance.guarantee.reconcile') AND recurso_tipo='reserva'))
        )
    );

COMMENT ON CONSTRAINT evento_auditoria_idempotencia_ck ON public.evento_auditoria_local IS
    'Idempotency keys are permitted only for audited operations with a stable resource owner. Financial fake actions use reserva and hashed keys.';
