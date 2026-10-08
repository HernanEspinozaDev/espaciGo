-- LOCAL-PRIV-01A3: unlink retained synthetic transaction facts after their
-- ratified 24-calendar-month period. Keep the facts and their stable IDs.
ALTER TABLE public.reserva_ensayo_local
    ALTER COLUMN anfitrion_id DROP NOT NULL,
    ALTER COLUMN arrendatario_id DROP NOT NULL;
ALTER TABLE public.cotizacion_reserva_ensayo
    ALTER COLUMN anfitrion_id DROP NOT NULL,
    ALTER COLUMN arrendatario_id DROP NOT NULL;
ALTER TABLE public.reserva_pago_ensayo_operacion
    ALTER COLUMN arrendatario_id DROP NOT NULL;
ALTER TABLE public.reserva_cancelacion_ensayo
    ALTER COLUMN arrendatario_id DROP NOT NULL;
ALTER TABLE public.reserva_ensayo_local_fixture
    ALTER COLUMN anfitrion_id DROP NOT NULL,
    ALTER COLUMN arrendatario_id DROP NOT NULL;
ALTER TABLE public.disputa_ensayo_local
    ALTER COLUMN anfitrion_id DROP NOT NULL,
    ALTER COLUMN arrendatario_id DROP NOT NULL,
    ALTER COLUMN abierta_por DROP NOT NULL,
    DROP CONSTRAINT disputa_ensayo_apertura_ck,
    ADD CONSTRAINT disputa_ensayo_apertura_ck CHECK (
        abierta_por=anfitrion_id OR (abierta_por IS NULL AND anfitrion_id IS NULL)
    );
ALTER TABLE public.disputa_ensayo_historial
    ALTER COLUMN actor_id DROP NOT NULL;
ALTER TABLE public.mensaje_reserva_ensayo
    ALTER COLUMN autor_id DROP NOT NULL;
ALTER TABLE public.disputa_ensayo_local
    DROP CONSTRAINT disputa_ensayo_cierre_ck,
    ADD CONSTRAINT disputa_ensayo_cierre_ck CHECK (
        (estado='abierta' AND cerrada_por IS NULL AND motivo_cierre_codigo IS NULL AND cerrada_en IS NULL)
        OR (estado='cerrada' AND motivo_cierre_codigo IS NOT NULL AND cerrada_en IS NOT NULL
            AND (cerrada_por IS NOT NULL OR (anfitrion_id IS NULL AND arrendatario_id IS NULL)))
    );

UPDATE public.reserva_ensayo_local r SET vinculos_retirar_en=(
    SELECT GREATEST(r.actualizada_en,
        COALESCE((SELECT max(p.actualizada_en) FROM public.reserva_pago_ensayo_operacion p WHERE p.reserva_id=r.id),r.actualizada_en),
        COALESCE((SELECT max(d.actualizada_en) FROM public.reserva_devolucion_ensayo d WHERE d.reserva_id=r.id),r.actualizada_en),
        COALESCE((SELECT max(x.cerrada_en) FROM public.disputa_ensayo_local x WHERE x.reserva_id=r.id),r.actualizada_en)
    ) + interval '24 months'
) WHERE r.vinculos_retirar_en IS NULL AND r.estado IN
    ('cancelada_por_pago','rechazada_arrendador','vencida_pago','vencida_host','cancelada_arrendatario');

CREATE OR REPLACE FUNCTION public.schedule_local_reservation_link_retention() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.estado IN ('cancelada_por_pago','rechazada_arrendador','vencida_pago','vencida_host','cancelada_arrendatario')
       AND (TG_OP='INSERT' OR OLD.estado IS DISTINCT FROM NEW.estado) THEN
        NEW.vinculos_retirar_en := NEW.actualizada_en + interval '24 months';
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER reserva_vinculos_retencion_trg
    BEFORE INSERT OR UPDATE OF estado ON public.reserva_ensayo_local
    FOR EACH ROW EXECUTE FUNCTION public.schedule_local_reservation_link_retention();

CREATE INDEX reserva_ensayo_retencion_local_idx
    ON public.reserva_ensayo_local(vinculos_retirar_en, id)
    WHERE estado IN ('cancelada_por_pago','rechazada_arrendador','vencida_pago','vencida_host','cancelada_arrendatario');

CREATE TABLE public.reserva_vinculo_purgado_local (
    reserva_id uuid PRIMARY KEY REFERENCES public.reserva_ensayo_local(id) ON DELETE RESTRICT,
    vencio_en timestamptz NOT NULL,
    purgado_en timestamptz NOT NULL,
    campos_retirados text[] NOT NULL CHECK (cardinality(campos_retirados) > 0),
    CHECK (purgado_en >= vencio_en)
);

-- One row per replayed completed suppression and restore operation. It holds
-- only technical UUIDs and structured status; no contact or credential data.
CREATE TABLE public.reaplicacion_baja_local (
    restore_id uuid NOT NULL,
    ejecucion_origen_id uuid NOT NULL,
    usuario_id uuid NOT NULL REFERENCES public.usuario(id) ON DELETE RESTRICT,
    estado text NOT NULL CHECK (estado IN ('reaplicada','ya_presente','pendiente')),
    intentos integer NOT NULL DEFAULT 1 CHECK (intentos > 0),
    aplicada_en timestamptz NOT NULL,
    PRIMARY KEY (restore_id, ejecucion_origen_id)
);
CREATE INDEX reaplicacion_baja_local_usuario_idx
    ON public.reaplicacion_baja_local(usuario_id, aplicada_en);
