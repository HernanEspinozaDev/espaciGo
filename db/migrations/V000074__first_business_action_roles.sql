-- New business memberships are earned by the first corresponding action.
-- Registration itself grants neither renter nor landlord membership.
CREATE TABLE public.rol_usuario_evento (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    usuario_id uuid NOT NULL REFERENCES public.usuario(id) ON DELETE RESTRICT,
    rol text NOT NULL CHECK (rol IN ('arrendatario', 'arrendador')),
    accion text NOT NULL CHECK (accion IN ('primer_borrador_espacio', 'primera_solicitud_reserva')),
    recurso_id uuid NOT NULL,
    ocurrida_en timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT rol_usuario_evento_usuario_rol_uk UNIQUE (usuario_id, rol),
    CONSTRAINT rol_usuario_evento_accion_rol_ck CHECK (
      (rol = 'arrendador' AND accion = 'primer_borrador_espacio') OR
      (rol = 'arrendatario' AND accion = 'primera_solicitud_reserva')
    )
);

REVOKE ALL ON TABLE public.rol_usuario_evento FROM PUBLIC;
GRANT INSERT ON TABLE public.rol_usuario_evento TO espacigo_runtime;
