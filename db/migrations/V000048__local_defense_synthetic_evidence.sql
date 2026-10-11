-- LOCAL-CLOSE-01C: attach server-generated private synthetic evidence to the
-- renter's already-persisted defense. Reservation evidence follows the
-- ratified 24-month terminal reservation/history retention window.
ALTER TABLE public.operacion_arriendo_ensayo_local
    DROP CONSTRAINT operacion_arriendo_ensayo_local_tipo_check,
    ADD CONSTRAINT operacion_arriendo_ensayo_local_tipo_check
        CHECK (tipo IN ('checkin','checkout','recepcion','descargo'));

ALTER TABLE public.operacion_arriendo_archivo_candidato_local
    DROP CONSTRAINT operacion_arriendo_archivo_candidato_local_tipo_check,
    ADD CONSTRAINT operacion_arriendo_archivo_candidato_local_tipo_check
        CHECK (tipo IN ('checkin','checkout','recepcion','descargo'));

ALTER TABLE public.operacion_arriendo_ensayo_local
    DROP CONSTRAINT operacion_arriendo_ensayo_local_check,
    ADD CONSTRAINT operacion_arriendo_ensayo_local_resultado_tipo_check CHECK (
        (tipo='recepcion' AND resultado IN ('recepcion_conforme','recepcion_con_observaciones'))
        OR (tipo IN ('checkin','checkout','descargo') AND resultado='registrada')
    ),
    ADD CONSTRAINT operacion_arriendo_ensayo_local_campos_tipo_check CHECK (
        (tipo IN ('checkin','checkout') OR comentarios='') AND (tipo='recepcion' OR observacion='')
    );

ALTER TABLE public.operacion_arriendo_evidencia_ensayo_local
    ADD COLUMN retirar_archivo_en timestamptz,
    ADD COLUMN archivo_retirado_en timestamptz;

UPDATE public.operacion_arriendo_evidencia_ensayo_local e
SET retirar_archivo_en=r.vinculos_retirar_en
FROM public.operacion_arriendo_ensayo_local o
JOIN public.reserva_ensayo_local r ON r.id=o.reserva_id
WHERE e.operacion_id=o.id AND o.tipo='descargo' AND r.vinculos_retirar_en IS NOT NULL;

CREATE INDEX operacion_arriendo_evidencia_retencion_idx
    ON public.operacion_arriendo_evidencia_ensayo_local(retirar_archivo_en,id)
    WHERE retirar_archivo_en IS NOT NULL AND archivo_retirado_en IS NULL;

ALTER TABLE public.operacion_arriendo_historial_ensayo_local
    DROP CONSTRAINT operacion_arriendo_historial_ensayo_local_accion_check,
    ADD CONSTRAINT operacion_arriendo_historial_ensayo_local_accion_check
        CHECK (accion IN ('checkin_registrado','checkout_registrado','recepcion_confirmada','observacion_registrada','descargo_evidencia'));

COMMENT ON COLUMN public.operacion_arriendo_evidencia_ensayo_local.retirar_archivo_en IS
    'Plazo derivado de la retención ratificada de reservas sintéticas: 24 meses desde el último cierre relacionado.';
COMMENT ON COLUMN public.operacion_arriendo_evidencia_ensayo_local.archivo_retirado_en IS
    'Marca mínima de limpieza durable del blob; metadatos/historial pueden permanecer minimizados.';
