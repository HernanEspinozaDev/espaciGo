-- LOCAL-KYC-01: append-only case history and type-scoped synthetic eligibility.
-- No provider/document data is introduced; this migration does not touch bookings.
ALTER TABLE public.verificacion
    ADD COLUMN correccion_codigo text,
    ADD COLUMN revocada_en timestamptz,
    ADD COLUMN revocada_por uuid REFERENCES public.usuario(id) ON DELETE RESTRICT,
    ADD COLUMN motivo_revocacion_codigo text;

-- Revoking eligibility does not restart or erase the already ratified two-year
-- metadata deadline, just as a privacy-retirement timestamp is tracked apart
-- from the original terminal decision.
CREATE OR REPLACE FUNCTION public.set_local_verification_retention_deadline() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP='UPDATE' AND NEW.estado IN ('retirada_privacidad','revocada') AND OLD.resuelta_en IS NOT NULL THEN
        NEW.retirar_en := OLD.retirar_en;
    ELSIF NEW.estado IN ('aprobada','rechazada','retirada_privacidad') AND NEW.resuelta_en IS NOT NULL THEN
        NEW.retirar_en := NEW.resuelta_en + interval '2 years';
    ELSE
        NEW.retirar_en := NULL;
    END IF;
    RETURN NEW;
END;
$$;

ALTER TABLE public.verificacion DROP CONSTRAINT verificacion_estado_ck;
ALTER TABLE public.verificacion ADD CONSTRAINT verificacion_estado_ck
    CHECK (estado IN ('en_revision','aprobada','rechazada','revocada','retirada_privacidad'));
ALTER TABLE public.verificacion DROP CONSTRAINT verificacion_motivo_ck;
ALTER TABLE public.verificacion ADD CONSTRAINT verificacion_motivo_ck CHECK (
    (estado='rechazada' AND revisor_id IS NOT NULL AND motivo_codigo IS NOT NULL AND resuelta_en IS NOT NULL)
 OR (estado='aprobada' AND revisor_id IS NOT NULL AND motivo_codigo IS NULL AND resuelta_en IS NOT NULL)
 OR (estado='en_revision' AND revisor_id IS NULL AND motivo_codigo IS NULL AND resuelta_en IS NULL)
 OR (estado='revocada' AND revisor_id IS NOT NULL AND motivo_codigo IS NULL AND resuelta_en IS NOT NULL
     AND revocada_en IS NOT NULL AND revocada_por IS NOT NULL AND motivo_revocacion_codigo IS NOT NULL)
 OR (estado='retirada_privacidad' AND revisor_id IS NULL AND motivo_codigo='baja_privacidad' AND resuelta_en IS NOT NULL)
);
-- Old retries predate typed corrections. Preserve them with an explicit legacy
-- marker instead of inferring which evidence the user changed.
UPDATE public.verificacion
SET correccion_codigo='legado_pre_v28_sin_codigo'
WHERE reintento_de IS NOT NULL AND correccion_codigo IS NULL;
ALTER TABLE public.verificacion ADD CONSTRAINT verificacion_correccion_codigo_ck CHECK (
    correccion_codigo IS NULL OR correccion_codigo IN (
        'fixture_vigente_actualizado','antecedentes_fixture_actualizados','inicio_actividades_fixture_actualizadas',
        'legado_pre_v28_sin_codigo'
    )
);
ALTER TABLE public.verificacion ADD CONSTRAINT verificacion_revocacion_codigo_ck CHECK (
    (revocada_en IS NULL AND revocada_por IS NULL AND motivo_revocacion_codigo IS NULL)
 OR (revocada_en IS NOT NULL AND revocada_por IS NOT NULL AND motivo_revocacion_codigo IN (
        'aprobacion_fixture_incorrecta','revision_fixture_actualizada'
    ))
);
ALTER TABLE public.verificacion ADD CONSTRAINT verificacion_correccion_retry_ck CHECK (
    (reintento_de IS NULL AND correccion_codigo IS NULL)
 OR (reintento_de IS NOT NULL AND correccion_codigo IS NOT NULL)
);

CREATE TABLE public.verificacion_historial_local (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    verificacion_id uuid NOT NULL REFERENCES public.verificacion(id) ON DELETE RESTRICT,
    actor_id uuid NOT NULL REFERENCES public.usuario(id) ON DELETE RESTRICT,
    accion text NOT NULL CHECK (accion IN ('solicitada','revision_aprobada','revision_rechazada','subsanacion_solicitada','elegibilidad_revocada','estado_observado_en_migracion')),
    estado_anterior text,
    estado_nuevo text NOT NULL CHECK (estado_nuevo IN ('en_revision','aprobada','rechazada','revocada','retirada_privacidad')),
    motivo_codigo text,
    correlacion_id text NOT NULL CHECK (length(btrim(correlacion_id)) BETWEEN 1 AND 120),
    clave_idempotencia text,
    ocurrida_en timestamptz NOT NULL,
    CONSTRAINT verificacion_historial_estado_ck CHECK (
        (accion='solicitada' AND estado_anterior IS NULL AND estado_nuevo='en_revision')
        OR (accion='revision_aprobada' AND estado_anterior='en_revision' AND estado_nuevo='aprobada' AND motivo_codigo IS NULL)
        OR (accion='revision_rechazada' AND estado_anterior='en_revision' AND estado_nuevo='rechazada' AND motivo_codigo IS NOT NULL)
        OR (accion='subsanacion_solicitada' AND estado_anterior='rechazada' AND estado_nuevo='rechazada' AND motivo_codigo IS NOT NULL)
        OR (accion='elegibilidad_revocada' AND estado_anterior='aprobada' AND estado_nuevo='revocada' AND motivo_codigo IS NOT NULL)
        OR (accion='estado_observado_en_migracion' AND estado_anterior IS NULL AND estado_nuevo='retirada_privacidad')
    ),
    CONSTRAINT verificacion_historial_event_idempotency_uk UNIQUE (verificacion_id, clave_idempotencia)
);
CREATE INDEX verificacion_historial_order_idx ON public.verificacion_historial_local(verificacion_id,id);

CREATE TABLE public.elegibilidad_verificacion_local (
    usuario_id uuid NOT NULL REFERENCES public.usuario(id) ON DELETE RESTRICT,
    tipo text NOT NULL CHECK (tipo IN ('kyc','kyb')),
    verificacion_id uuid NOT NULL UNIQUE REFERENCES public.verificacion(id) ON DELETE RESTRICT,
    estado text NOT NULL CHECK (estado IN ('elegible','revocada')),
    concedida_en timestamptz NOT NULL,
    revocada_en timestamptz,
    revocada_por uuid REFERENCES public.usuario(id) ON DELETE RESTRICT,
    motivo_revocacion_codigo text,
    PRIMARY KEY (usuario_id,tipo),
    CONSTRAINT elegibilidad_revocacion_ck CHECK (
        (estado='elegible' AND revocada_en IS NULL AND revocada_por IS NULL AND motivo_revocacion_codigo IS NULL)
        OR (estado='revocada' AND revocada_en IS NOT NULL AND revocada_por IS NOT NULL
            AND motivo_revocacion_codigo IN ('aprobacion_fixture_incorrecta','revision_fixture_actualizada'))
    )
);
CREATE INDEX elegibilidad_local_active_idx ON public.elegibilidad_verificacion_local(usuario_id,tipo) WHERE estado='elegible';

-- Backfill a durable snapshot of currently approved synthetic cases. Past
-- transitions cannot be reconstructed beyond each row's current state.
INSERT INTO public.elegibilidad_verificacion_local(usuario_id,tipo,verificacion_id,estado,concedida_en)
SELECT usuario_id,tipo,id,'elegible',resuelta_en FROM (
    SELECT DISTINCT ON (usuario_id,tipo) usuario_id,tipo,id,resuelta_en
    FROM public.verificacion WHERE estado='aprobada'
    ORDER BY usuario_id,tipo,resuelta_en DESC,id DESC
) approved
ON CONFLICT (usuario_id,tipo) DO UPDATE SET
    verificacion_id=EXCLUDED.verificacion_id,
    estado='elegible',
    concedida_en=GREATEST(public.elegibilidad_verificacion_local.concedida_en,EXCLUDED.concedida_en),
    revocada_en=NULL,revocada_por=NULL,motivo_revocacion_codigo=NULL
WHERE EXCLUDED.concedida_en > public.elegibilidad_verificacion_local.concedida_en;

INSERT INTO public.verificacion_historial_local(verificacion_id,actor_id,accion,estado_anterior,estado_nuevo,motivo_codigo,correlacion_id,ocurrida_en)
SELECT id,usuario_id,'solicitada',NULL,'en_revision',NULL,'migration-v28:'||id::text,creada_en
FROM public.verificacion;
INSERT INTO public.verificacion_historial_local(verificacion_id,actor_id,accion,estado_anterior,estado_nuevo,motivo_codigo,correlacion_id,ocurrida_en)
SELECT id,revisor_id,CASE WHEN estado='aprobada' THEN 'revision_aprobada' ELSE 'revision_rechazada' END,
       'en_revision',estado,motivo_codigo,'migration-v28-resolution:'||id::text,resuelta_en
FROM public.verificacion WHERE estado IN ('aprobada','rechazada') AND revisor_id IS NOT NULL;
INSERT INTO public.verificacion_historial_local(verificacion_id,actor_id,accion,estado_nuevo,motivo_codigo,correlacion_id,ocurrida_en)
SELECT id,usuario_id,'estado_observado_en_migracion','retirada_privacidad','estado_actual_migrado',
       'migration-v28-privacy:'||id::text,COALESCE(retirada_privacidad_en,resuelta_en)
FROM public.verificacion WHERE estado='retirada_privacidad';

-- Runtime grants are applied by dbbootstrap after migrations, so an empty-database
-- migration remains independent of the optional runtime role.
