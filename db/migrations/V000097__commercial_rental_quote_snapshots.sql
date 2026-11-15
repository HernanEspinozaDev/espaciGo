-- Commercial, short-lived quote snapshots. These are not reservations,
-- payment intents, signatures, or local M06 booking-trial records.
CREATE TABLE public.cotizacion_arriendo_comercial (
    id uuid PRIMARY KEY,
    espacio_id uuid NOT NULL REFERENCES public.espacio(id) ON DELETE CASCADE,
    propietario_id uuid NOT NULL REFERENCES public.usuario(id) ON DELETE CASCADE,
    arrendatario_id uuid NOT NULL REFERENCES public.usuario(id) ON DELETE CASCADE,
    titulo_espacio text NOT NULL,
    tarifa_version integer NOT NULL,
    modalidad text NOT NULL,
    precio_unitario_clp bigint NOT NULL,
    moneda text NOT NULL,
    unidades bigint NOT NULL,
    subtotal_clp bigint NOT NULL,
    inicio timestamptz NOT NULL,
    termino timestamptz NOT NULL,
    zona_horaria text NOT NULL,
    creada_en timestamptz NOT NULL,
    vence_en timestamptz NOT NULL,
    CONSTRAINT cotizacion_arriendo_comercial_participantes_ck
        CHECK (propietario_id <> arrendatario_id),
    CONSTRAINT cotizacion_arriendo_comercial_titulo_ck
        CHECK (char_length(btrim(titulo_espacio)) BETWEEN 1 AND 70),
    CONSTRAINT cotizacion_arriendo_comercial_tarifa_ck
        CHECK (tarifa_version > 0 AND modalidad IN ('hora','dia','mes') AND precio_unitario_clp > 0 AND moneda = 'CLP'),
    CONSTRAINT cotizacion_arriendo_comercial_unidades_ck
        CHECK (
            (modalidad = 'hora' AND unidades IN (1, 2, 4)) OR
            (modalidad = 'dia' AND unidades BETWEEN 1 AND 3) OR
            (modalidad = 'mes' AND unidades = 1)
        ),
    CONSTRAINT cotizacion_arriendo_comercial_subtotal_ck
        CHECK (subtotal_clp = precio_unitario_clp * unidades),
    CONSTRAINT cotizacion_arriendo_comercial_intervalo_ck
        CHECK (termino > inicio AND zona_horaria <> ''),
    CONSTRAINT cotizacion_arriendo_comercial_vigencia_ck
        CHECK (vence_en = creada_en + interval '15 minutes')
);

CREATE INDEX cotizacion_arriendo_comercial_expira_idx
    ON public.cotizacion_arriendo_comercial (vence_en, id);

COMMENT ON TABLE public.cotizacion_arriendo_comercial IS
    'Commercial 15-minute quote snapshot; excludes legal identity and is purged at expiry or participant/space removal.';

DO $grants$ BEGIN
    IF EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname = 'espacigo_runtime') THEN
        GRANT SELECT, INSERT, DELETE ON TABLE public.cotizacion_arriendo_comercial TO espacigo_runtime;
    END IF;
END $grants$;
