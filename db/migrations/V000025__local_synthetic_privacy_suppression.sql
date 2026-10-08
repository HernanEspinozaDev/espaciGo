-- LOCAL-PRIV-01A2: execute ratified suppression only for synthetic local data.
-- Retained rows are not cascaded; timestamps are explicit retention deadlines.
ALTER TABLE public.usuario
    ADD COLUMN baja_iniciada_en timestamptz;

ALTER TABLE public.aceptacion_terminos
    ADD COLUMN retirar_en timestamptz;

ALTER TABLE public.solicitud_titular
    ADD COLUMN resuelta_en timestamptz,
    ADD COLUMN motivo_resolucion_codigo text,
    ADD COLUMN retirar_en timestamptz,
    ADD CONSTRAINT solicitud_titular_resolucion_ck CHECK (
        (resuelta_en IS NULL AND motivo_resolucion_codigo IS NULL AND retirar_en IS NULL)
        OR (resuelta_en IS NOT NULL AND motivo_resolucion_codigo IS NOT NULL AND retirar_en = resuelta_en + interval '5 years')
    ),
    ADD CONSTRAINT solicitud_titular_motivo_ck CHECK (
        motivo_resolucion_codigo IS NULL OR motivo_resolucion_codigo IN ('baja_local_minimizada','derecho_tramitado')
    );

ALTER TABLE public.verificacion ADD COLUMN retirar_en timestamptz;
UPDATE public.verificacion SET retirar_en = resuelta_en + interval '2 years' WHERE resuelta_en IS NOT NULL;
ALTER TABLE public.verificacion DROP CONSTRAINT verificacion_estado_ck;
ALTER TABLE public.verificacion ADD CONSTRAINT verificacion_estado_ck CHECK (estado IN ('en_revision','aprobada','rechazada','retirada_privacidad'));
ALTER TABLE public.verificacion DROP CONSTRAINT verificacion_motivo_ck;
ALTER TABLE public.verificacion ADD CONSTRAINT verificacion_motivo_ck CHECK (
    (estado='rechazada' AND revisor_id IS NOT NULL AND motivo_codigo IS NOT NULL AND resuelta_en IS NOT NULL)
    OR (estado='aprobada' AND revisor_id IS NOT NULL AND motivo_codigo IS NULL AND resuelta_en IS NOT NULL)
    OR (estado='en_revision' AND revisor_id IS NULL AND motivo_codigo IS NULL AND resuelta_en IS NULL)
    OR (estado='retirada_privacidad' AND revisor_id IS NULL AND motivo_codigo='baja_privacidad' AND resuelta_en IS NOT NULL)
);
ALTER TABLE public.verificacion DROP CONSTRAINT verificacion_reason_code_ck;
ALTER TABLE public.verificacion ADD CONSTRAINT verificacion_reason_code_ck CHECK (
    motivo_codigo IS NULL OR motivo_codigo IN ('documento_vencido','antecedentes_incompletos','inicio_actividades_no_confirmado','baja_privacidad')
);
ALTER TABLE public.cotizacion_reserva_ensayo ADD COLUMN retirar_en timestamptz;
UPDATE public.cotizacion_reserva_ensayo q SET retirar_en = q.vence_en + interval '90 days'
WHERE NOT EXISTS (SELECT 1 FROM public.reserva_ensayo_local r WHERE r.cotizacion_id=q.id);

ALTER TABLE public.reserva_ensayo_local ADD COLUMN vinculos_retirar_en timestamptz;
UPDATE public.reserva_ensayo_local r SET vinculos_retirar_en=(
    SELECT GREATEST(r.actualizada_en,
        COALESCE((SELECT max(p.actualizada_en) FROM public.reserva_pago_ensayo_operacion p WHERE p.reserva_id=r.id),r.actualizada_en),
        COALESCE((SELECT max(d.actualizada_en) FROM public.reserva_devolucion_ensayo d WHERE d.reserva_id=r.id),r.actualizada_en),
        COALESCE((SELECT max(x.cerrada_en) FROM public.disputa_ensayo_local x WHERE x.reserva_id=r.id),r.actualizada_en)
    ) + interval '24 months'
)
WHERE r.estado IN ('cancelada_por_pago','rechazada_arrendador','vencida_pago','vencida_host','cancelada_arrendatario');

ALTER TABLE public.outbox_evento_local
    ADD COLUMN cancelada_en timestamptz,
    ADD COLUMN motivo_cancelacion_codigo text,
    ADD COLUMN retirar_en timestamptz,
    ADD CONSTRAINT outbox_cancelacion_ck CHECK (
        (cancelada_en IS NULL AND motivo_cancelacion_codigo IS NULL)
        OR (cancelada_en IS NOT NULL AND motivo_cancelacion_codigo='baja_local_sin_finalidad')
    );
UPDATE public.outbox_evento_local SET retirar_en=entregada_en + interval '30 days' WHERE entregada_en IS NOT NULL;

ALTER TABLE public.evento_auditoria_local DROP CONSTRAINT evento_auditoria_idempotencia_ck;
ALTER TABLE public.evento_auditoria_local ADD CONSTRAINT evento_auditoria_idempotencia_ck CHECK (
    clave_idempotencia IS NULL OR (
        length(btrim(clave_idempotencia)) BETWEEN 1 AND 200
        AND accion IN ('privacy.suppression.review','privacy.suppression.execute')
        AND recurso_tipo='solicitud_titular'
    )
);

CREATE TABLE public.ejecucion_baja_local (
    id uuid PRIMARY KEY,
    solicitud_id uuid NOT NULL REFERENCES public.solicitud_titular(id) ON DELETE RESTRICT,
    usuario_id uuid NOT NULL REFERENCES public.usuario(id) ON DELETE RESTRICT,
    actor_id uuid NOT NULL REFERENCES public.usuario(id) ON DELETE RESTRICT,
    clave_idempotencia text NOT NULL CHECK (length(btrim(clave_idempotencia)) BETWEEN 1 AND 200),
    estado text NOT NULL CHECK (estado IN ('bloqueada','limpieza_pendiente','completada')),
    motivo_codigo text NOT NULL CHECK (motivo_codigo IN ('supresion_con_obligaciones','baja_local_minimizada')),
    iniciada_en timestamptz NOT NULL,
    completada_en timestamptz,
    retirar_en timestamptz,
    detalle jsonb NOT NULL CHECK (jsonb_typeof(detalle)='object'),
    UNIQUE (solicitud_id, clave_idempotencia),
    CHECK ((estado='completada') = (completada_en IS NOT NULL)),
    CHECK ((estado='completada') = (retirar_en IS NOT NULL)),
    CHECK (retirar_en IS NULL OR retirar_en = completada_en + interval '5 years')
);
CREATE INDEX ejecucion_baja_local_pendiente_idx ON public.ejecucion_baja_local(iniciada_en,id)
    WHERE estado='limpieza_pendiente';

CREATE OR REPLACE FUNCTION public.set_local_verification_retention_deadline() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.estado IN ('aprobada','rechazada','retirada_privacidad') AND NEW.resuelta_en IS NOT NULL THEN
        NEW.retirar_en := NEW.resuelta_en + interval '2 years';
    ELSE
        NEW.retirar_en := NULL;
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER verificacion_retencion_sintetica_trg
    BEFORE INSERT OR UPDATE OF estado,resuelta_en ON public.verificacion
    FOR EACH ROW EXECUTE FUNCTION public.set_local_verification_retention_deadline();

-- The file ID is enough to derive the private path. The row deliberately has
-- no FK to evidence so success can delete metadata while preserving this job.
CREATE TABLE public.baja_archivo_pendiente_local (
    ejecucion_id uuid NOT NULL REFERENCES public.ejecucion_baja_local(id) ON DELETE RESTRICT,
    evidencia_id uuid NOT NULL,
    intentos integer NOT NULL DEFAULT 0 CHECK (intentos >= 0),
    disponible_en timestamptz NOT NULL,
    ultimo_codigo_error text,
    completada_en timestamptz,
    PRIMARY KEY (ejecucion_id,evidencia_id)
);
CREATE INDEX baja_archivo_pendiente_claim_idx ON public.baja_archivo_pendiente_local(disponible_en,ejecucion_id,evidencia_id)
    WHERE completada_en IS NULL;
