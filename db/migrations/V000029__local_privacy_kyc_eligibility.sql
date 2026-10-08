-- A local privacy withdrawal keeps verification facts and their original
-- retention deadlines, while explicitly ending the derived eligibility.
ALTER TABLE public.elegibilidad_verificacion_local
    DROP CONSTRAINT elegibilidad_verificacion_local_estado_check,
    ADD CONSTRAINT elegibilidad_verificacion_local_estado_check
        CHECK (estado IN ('elegible','revocada','retirada_privacidad'));
ALTER TABLE public.elegibilidad_verificacion_local
    DROP CONSTRAINT elegibilidad_revocacion_ck,
    ADD CONSTRAINT elegibilidad_revocacion_ck CHECK (
        (estado='elegible' AND revocada_en IS NULL AND revocada_por IS NULL AND motivo_revocacion_codigo IS NULL)
        OR (estado='revocada' AND revocada_en IS NOT NULL AND revocada_por IS NOT NULL
            AND motivo_revocacion_codigo IN ('aprobacion_fixture_incorrecta','revision_fixture_actualizada'))
        OR (estado='retirada_privacidad' AND revocada_en IS NOT NULL AND revocada_por IS NOT NULL
            AND motivo_revocacion_codigo='baja_privacidad')
    );

ALTER TABLE public.verificacion_historial_local
    DROP CONSTRAINT verificacion_historial_local_accion_check,
    ADD CONSTRAINT verificacion_historial_local_accion_check
        CHECK (accion IN ('solicitada','revision_aprobada','revision_rechazada','subsanacion_solicitada','elegibilidad_revocada','elegibilidad_retirada_privacidad','estado_observado_en_migracion'));
ALTER TABLE public.verificacion_historial_local
    DROP CONSTRAINT verificacion_historial_estado_ck,
    ADD CONSTRAINT verificacion_historial_estado_ck CHECK (
        (accion='solicitada' AND estado_anterior IS NULL AND estado_nuevo='en_revision')
        OR (accion='revision_aprobada' AND estado_anterior='en_revision' AND estado_nuevo='aprobada' AND motivo_codigo IS NULL)
        OR (accion='revision_rechazada' AND estado_anterior='en_revision' AND estado_nuevo='rechazada' AND motivo_codigo IS NOT NULL)
        OR (accion='subsanacion_solicitada' AND estado_anterior='rechazada' AND estado_nuevo='rechazada' AND motivo_codigo IS NOT NULL)
        OR (accion='elegibilidad_revocada' AND estado_anterior='aprobada' AND estado_nuevo='revocada' AND motivo_codigo IS NOT NULL)
        OR (accion='elegibilidad_retirada_privacidad' AND estado_anterior='aprobada' AND estado_nuevo='retirada_privacidad' AND motivo_codigo='baja_privacidad')
        OR (accion='estado_observado_en_migracion' AND estado_anterior IS NULL AND estado_nuevo='retirada_privacidad')
    );
