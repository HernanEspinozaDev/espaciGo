-- GADM-01 keeps global membership outside rol_usuario. The runtime can assign
-- ordinary application roles during registration, so it must never write the
-- global membership table.
CREATE TABLE public.administrador_global (
    usuario_id uuid PRIMARY KEY REFERENCES public.usuario(id) ON DELETE RESTRICT,
    concedido_en timestamptz NOT NULL,
    concedido_por uuid REFERENCES public.usuario(id) ON DELETE SET NULL,
    motivo_codigo text NOT NULL CHECK (motivo_codigo IN ('bootstrap_propietario','recuperacion_local','concesion_administrativa'))
);

-- One initial owner binding. A local recovery can restore only this account;
-- later multi-admin grants belong to a separately reviewed GADM cut.
CREATE TABLE public.administrador_global_bootstrap (
    singleton boolean PRIMARY KEY DEFAULT true CHECK (singleton),
    usuario_id uuid NOT NULL UNIQUE REFERENCES public.usuario(id) ON DELETE RESTRICT,
    creado_en timestamptz NOT NULL
);

-- Minimal append-only CLI audit, including rejected attempts. Failed targets
-- intentionally have no FK so a nonexistent UUID can still be recorded.
CREATE TABLE public.administrador_global_bootstrap_evento (
    id uuid PRIMARY KEY,
    usuario_objetivo_id uuid NOT NULL,
    actor_tipo text NOT NULL CHECK (actor_tipo = 'cli_local'),
    accion text NOT NULL CHECK (accion IN ('bootstrap','recuperacion')),
    resultado text NOT NULL CHECK (resultado IN ('exito','rechazo')),
    motivo_codigo text NOT NULL CHECK (length(btrim(motivo_codigo)) BETWEEN 1 AND 60),
    correlacion_id uuid NOT NULL,
    ocurrida_en timestamptz NOT NULL
);
CREATE INDEX administrador_global_bootstrap_evento_objetivo_idx
    ON public.administrador_global_bootstrap_evento(usuario_objetivo_id, ocurrida_en DESC, id);

CREATE INDEX usuario_estado_creado_id_idx
    ON public.usuario(estado, creado_en DESC, id DESC);
CREATE INDEX usuario_creado_id_idx
    ON public.usuario(creado_en DESC, id DESC);
CREATE INDEX usuario_correo_normalizado_prefix_idx
    ON public.usuario(correo_normalizado text_pattern_ops);
CREATE INDEX perfil_usuario_nombre_lower_prefix_idx
    ON public.perfil_usuario(lower(nombre_visible) text_pattern_ops);

COMMENT ON TABLE public.administrador_global IS
    'Membresía global de aplicación. Se concede por herramienta local auditada; el runtime solo puede leerla.';
COMMENT ON TABLE public.administrador_global_bootstrap IS
    'Vincula el bootstrap inicial y permite recuperación local únicamente de la misma cuenta registrada/verificada.';
COMMENT ON TABLE public.administrador_global_bootstrap_evento IS
    'Auditoría mínima append-only de bootstrap/recuperación local, sin credenciales ni datos de contacto.';

-- Preserve notices for existing administrators and also target the separate
-- global membership without inserting a legacy rol_usuario row.
CREATE OR REPLACE FUNCTION public.enqueue_local_review_report_notice() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE admin record; BEGIN
  FOR admin IN SELECT u.id AS usuario_id FROM public.usuario u
    WHERE u.estado='activo' AND (
      EXISTS (SELECT 1 FROM public.rol_usuario ru WHERE ru.usuario_id=u.id AND ru.rol='administrador')
      OR EXISTS (SELECT 1 FROM public.administrador_global ag WHERE ag.usuario_id=u.id)
    ) LOOP
    PERFORM public.enqueue_local_notice('resena_reportada','reporte_resena',NEW.id,admin.usuario_id,'review-report:'||NEW.id::text||':'||admin.usuario_id::text,NEW.creada_en);
  END LOOP; RETURN NEW;
END $$;
