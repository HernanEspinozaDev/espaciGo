-- Weekly opening hours for explicitly authorized local hourly fixtures only.
-- Manual blocks and reservations remain in the single occupancy calendar.
CREATE TABLE public.espacio_horario_semanal (
    espacio_id uuid PRIMARY KEY REFERENCES public.espacio(id) ON DELETE RESTRICT,
    activo boolean NOT NULL DEFAULT false,
    actualizado_en timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE public.espacio_horario_semanal_tramo (
    espacio_id uuid NOT NULL REFERENCES public.espacio_horario_semanal(espacio_id) ON DELETE CASCADE,
    dia_iso smallint NOT NULL CHECK (dia_iso BETWEEN 1 AND 7),
    orden smallint NOT NULL CHECK (orden BETWEEN 1 AND 2),
    apertura_minuto smallint NOT NULL CHECK (apertura_minuto BETWEEN 0 AND 1439),
    cierre_minuto smallint NOT NULL CHECK (cierre_minuto BETWEEN 1 AND 1440),
    PRIMARY KEY (espacio_id,dia_iso,orden),
    CHECK (cierre_minuto > apertura_minuto),
    EXCLUDE USING gist (
        espacio_id WITH =,
        dia_iso WITH =,
        int4range(apertura_minuto,cierre_minuto,'[)') WITH &&
    )
);

CREATE OR REPLACE FUNCTION public.guardar_zona_sin_horario_activo()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.zona_horaria IS DISTINCT FROM OLD.zona_horaria
       AND EXISTS (SELECT 1 FROM public.espacio_horario_semanal h
                   WHERE h.espacio_id=OLD.id AND h.activo) THEN
        RAISE EXCEPTION 'desactive el horario semanal antes de cambiar la zona horaria'
            USING ERRCODE='23514', CONSTRAINT='zona_horaria_horario_semanal_activo';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER espacio_zona_horario_semanal_guard
BEFORE UPDATE OF zona_horaria ON public.espacio
FOR EACH ROW EXECUTE FUNCTION public.guardar_zona_sin_horario_activo();

CREATE OR REPLACE FUNCTION public.guardar_modalidad_horario_activo()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.modalidad_tarifa IS DISTINCT FROM OLD.modalidad_tarifa
       AND NEW.modalidad_tarifa <> 'hora'
       AND EXISTS (SELECT 1 FROM public.espacio_horario_semanal h
                   WHERE h.espacio_id=OLD.id AND h.activo) THEN
        RAISE EXCEPTION 'desactive el horario semanal antes de cambiar la modalidad tarifaria'
            USING ERRCODE='23514', CONSTRAINT='modalidad_horario_semanal_activo_ck';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER espacio_modalidad_horario_semanal_guard
BEFORE UPDATE OF modalidad_tarifa ON public.espacio
FOR EACH ROW EXECUTE FUNCTION public.guardar_modalidad_horario_activo();

CREATE OR REPLACE FUNCTION public.validar_horario_semanal_fixture_hora()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.activo AND NOT EXISTS (
        SELECT 1 FROM public.reserva_ensayo_local_fixture f
        JOIN public.espacio e ON e.id=f.espacio_id AND e.propietario_id=f.anfitrion_id
        JOIN LATERAL (SELECT modalidad FROM public.tarifa_espacio
                      WHERE espacio_id=e.id ORDER BY version DESC LIMIT 1) t ON true
        WHERE f.espacio_id=NEW.espacio_id AND f.habilitada AND e.estado='borrador'
          AND t.modalidad='hora'
    ) THEN
        RAISE EXCEPTION 'el horario semanal activo requiere un fixture sintético habilitado con tarifa por hora'
            USING ERRCODE='23514', CONSTRAINT='horario_semanal_fixture_hora_ck';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER horario_semanal_fixture_hora_guard
BEFORE INSERT OR UPDATE OF activo ON public.espacio_horario_semanal
FOR EACH ROW EXECUTE FUNCTION public.validar_horario_semanal_fixture_hora();

CREATE OR REPLACE FUNCTION public.rechazar_cambio_tarifa_con_horario_activo()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.modalidad <> 'hora'
       AND EXISTS (SELECT 1 FROM public.espacio_horario_semanal h
                   WHERE h.espacio_id=NEW.espacio_id AND h.activo) THEN
        RAISE EXCEPTION 'desactive el horario semanal antes de cambiar la modalidad tarifaria'
            USING ERRCODE='23514', CONSTRAINT='tarifa_horario_semanal_activo_ck';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER tarifa_horario_semanal_guard
BEFORE INSERT ON public.tarifa_espacio
FOR EACH ROW EXECUTE FUNCTION public.rechazar_cambio_tarifa_con_horario_activo();
