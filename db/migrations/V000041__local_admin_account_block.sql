-- Administrative local block is distinct from login throttling, KYC revocation,
-- and privacy suppression. Current state is separate from the account lifecycle.
CREATE TABLE public.bloqueo_cuenta_administrativo_local (
    cuenta_id uuid PRIMARY KEY REFERENCES public.usuario(id) ON DELETE RESTRICT,
    bloqueada_por uuid NOT NULL REFERENCES public.usuario(id) ON DELETE RESTRICT,
    motivo_codigo text NOT NULL CHECK (motivo_codigo IN ('riesgo_seguridad','uso_indebido','revision_administrativa')),
    bloqueada_en timestamptz NOT NULL,
    correlacion_id text NOT NULL CHECK (length(btrim(correlacion_id)) BETWEEN 1 AND 120)
);

CREATE TABLE public.bloqueo_cuenta_historial_local (
    id uuid PRIMARY KEY,
    cuenta_id uuid NOT NULL REFERENCES public.usuario(id) ON DELETE RESTRICT,
    actor_id uuid NOT NULL REFERENCES public.usuario(id) ON DELETE RESTRICT,
    accion text NOT NULL CHECK (accion IN ('bloquear','desbloquear')),
    motivo_codigo text NOT NULL CHECK (motivo_codigo IN ('riesgo_seguridad','uso_indebido','revision_administrativa','revision_concluida','error_bloqueo','riesgo_resuelto')),
    ocurrida_en timestamptz NOT NULL,
    correlacion_id text NOT NULL CHECK (length(btrim(correlacion_id)) BETWEEN 1 AND 120)
);
CREATE INDEX bloqueo_cuenta_historial_local_idx ON public.bloqueo_cuenta_historial_local(cuenta_id,ocurrida_en,id);

COMMENT ON TABLE public.bloqueo_cuenta_administrativo_local IS 'Bloqueo sintético separado de usuario.estado, KYC y baja; conserva reservas y obligaciones.';
COMMENT ON TABLE public.bloqueo_cuenta_historial_local IS 'Historial append-only de gobierno local de cuentas; auditoría de cinco años también queda en evento_auditoria_local.';
