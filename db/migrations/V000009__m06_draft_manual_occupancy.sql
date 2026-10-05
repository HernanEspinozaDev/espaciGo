-- M06-owned calendar slice for private M04 draft administration. The
-- reservation FK is added with the reservation aggregate; this cut stores only
-- manual blocks and never creates a parallel calendar.
CREATE EXTENSION IF NOT EXISTS btree_gist;

ALTER TABLE public.espacio
    ADD COLUMN zona_horaria text,
    ADD CONSTRAINT espacio_zona_horaria_ck CHECK (
        zona_horaria IS NULL OR length(btrim(zona_horaria)) BETWEEN 1 AND 100
    );

CREATE TABLE public.ocupacion (
    id uuid PRIMARY KEY,
    espacio_id uuid NOT NULL REFERENCES public.espacio(id) ON DELETE RESTRICT,
    reserva_id uuid,
    intervalo tstzrange NOT NULL,
    tipo text NOT NULL CHECK (tipo IN ('retencion', 'reserva', 'bloqueo_manual')),
    activo boolean NOT NULL DEFAULT true,
    expira_en timestamptz,
    motivo text,
    creada_en timestamptz NOT NULL DEFAULT now(),
    desactivada_en timestamptz,
    CONSTRAINT ocupacion_intervalo_ck CHECK (
        NOT isempty(intervalo)
        AND NOT lower_inf(intervalo)
        AND NOT upper_inf(intervalo)
        AND lower_inc(intervalo)
        AND NOT upper_inc(intervalo)
        AND lower(intervalo) < upper(intervalo)
    ),
    CONSTRAINT ocupacion_tipo_reserva_ck CHECK (
        (tipo = 'bloqueo_manual' AND reserva_id IS NULL AND expira_en IS NULL
            AND motivo IS NOT NULL AND length(btrim(motivo)) BETWEEN 1 AND 500)
        OR (tipo IN ('retencion', 'reserva') AND reserva_id IS NOT NULL
            AND (tipo <> 'retencion' OR expira_en IS NOT NULL))
    ),
    CONSTRAINT ocupacion_desactivacion_ck CHECK (
        (activo AND desactivada_en IS NULL) OR (NOT activo AND desactivada_en IS NOT NULL)
    ),
    CONSTRAINT ocupacion_reserva_id_uk UNIQUE (reserva_id)
);

ALTER TABLE public.ocupacion
    ADD CONSTRAINT ocupacion_no_solapa_excl
    EXCLUDE USING gist (espacio_id WITH =, intervalo WITH &&) WHERE (activo);

CREATE INDEX ocupacion_espacio_activa_idx
    ON public.ocupacion (espacio_id, lower(intervalo)) WHERE activo;
