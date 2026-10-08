-- LOCAL-PRIV-01A2: local synthetic dispute records only as privacy blockers.
-- This does not transition a reservation or represent a financial resolution.
ALTER TABLE public.evento_auditoria_local
    DROP CONSTRAINT evento_auditoria_codigos_ck;

ALTER TABLE public.evento_auditoria_local
    ADD CONSTRAINT evento_auditoria_codigos_ck CHECK (
        jsonb_typeof(detalle_codigos)='object'
        AND detalle_codigos ? 'obligations_detected'
        AND detalle_codigos ? 'pending_checks'
        AND (detalle_codigos - ARRAY['obligations_detected','pending_checks']::text[])='{}'::jsonb
        AND jsonb_typeof(detalle_codigos->'obligations_detected')='array'
        AND jsonb_typeof(detalle_codigos->'pending_checks')='array'
        AND (detalle_codigos->'obligations_detected') <@ '["reserva_activa","pago_o_devolucion_pendiente","disputa_abierta"]'::jsonb
        AND (detalle_codigos->'pending_checks') <@ '["fuente_disputas_no_modelada","matriz_retencion_historicos_incompleta"]'::jsonb
    );

CREATE TABLE public.disputa_ensayo_local (
    id uuid PRIMARY KEY,
    reserva_id uuid NOT NULL REFERENCES public.reserva_ensayo_local(id) ON DELETE RESTRICT,
    anfitrion_id uuid NOT NULL REFERENCES public.usuario(id) ON DELETE RESTRICT,
    arrendatario_id uuid NOT NULL REFERENCES public.usuario(id) ON DELETE RESTRICT,
    abierta_por uuid NOT NULL REFERENCES public.usuario(id) ON DELETE RESTRICT,
    motivo_codigo text NOT NULL CHECK (motivo_codigo='ensayo_privacidad'),
    estado text NOT NULL CHECK (estado IN ('abierta','cerrada')),
    clave_idempotencia text NOT NULL CHECK (length(btrim(clave_idempotencia)) BETWEEN 1 AND 200),
    huella_solicitud bytea NOT NULL CHECK (octet_length(huella_solicitud)=32),
    abierta_en timestamptz NOT NULL,
    cerrada_por uuid REFERENCES public.usuario(id) ON DELETE RESTRICT,
    motivo_cierre_codigo text CHECK (motivo_cierre_codigo IN ('ensayo_finalizado','registro_erroneo','duplicada')),
    cerrada_en timestamptz,
    CONSTRAINT disputa_ensayo_participantes_ck CHECK (anfitrion_id <> arrendatario_id),
    CONSTRAINT disputa_ensayo_apertura_ck CHECK (abierta_por=anfitrion_id),
    CONSTRAINT disputa_ensayo_cierre_ck CHECK (
        (estado='abierta' AND cerrada_por IS NULL AND motivo_cierre_codigo IS NULL AND cerrada_en IS NULL)
        OR (estado='cerrada' AND cerrada_por IS NOT NULL AND motivo_cierre_codigo IS NOT NULL AND cerrada_en IS NOT NULL)
    ),
    UNIQUE (anfitrion_id, clave_idempotencia)
);
CREATE UNIQUE INDEX disputa_ensayo_una_abierta_por_reserva_uq
    ON public.disputa_ensayo_local(reserva_id) WHERE estado='abierta';
CREATE INDEX disputa_ensayo_anfitrion_idx ON public.disputa_ensayo_local(anfitrion_id, abierta_en DESC, id);
CREATE INDEX disputa_ensayo_arrendatario_idx ON public.disputa_ensayo_local(arrendatario_id, abierta_en DESC, id);
CREATE INDEX disputa_ensayo_abiertas_idx ON public.disputa_ensayo_local(abierta_en, id) WHERE estado='abierta';

CREATE TABLE public.disputa_ensayo_historial (
    secuencia bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    disputa_id uuid NOT NULL REFERENCES public.disputa_ensayo_local(id) ON DELETE RESTRICT,
    estado_anterior text CHECK (estado_anterior IN ('abierta','cerrada')),
    estado_nuevo text NOT NULL CHECK (estado_nuevo IN ('abierta','cerrada')),
    actor_id uuid NOT NULL REFERENCES public.usuario(id) ON DELETE RESTRICT,
    motivo_codigo text NOT NULL CHECK (motivo_codigo IN ('ensayo_privacidad','ensayo_finalizado','registro_erroneo','duplicada')),
    ocurrida_en timestamptz NOT NULL,
    CONSTRAINT disputa_ensayo_historial_transicion_ck CHECK (
        (estado_anterior IS NULL AND estado_nuevo='abierta' AND motivo_codigo='ensayo_privacidad')
        OR (estado_anterior='abierta' AND estado_nuevo='cerrada' AND motivo_codigo IN ('ensayo_finalizado','registro_erroneo','duplicada'))
    )
);
CREATE INDEX disputa_ensayo_historial_idx ON public.disputa_ensayo_historial(disputa_id, secuencia);

GRANT SELECT, INSERT ON public.disputa_ensayo_local TO espacigo_runtime;
GRANT UPDATE (estado,cerrada_por,motivo_cierre_codigo,cerrada_en) ON public.disputa_ensayo_local TO espacigo_runtime;
GRANT SELECT, INSERT ON public.disputa_ensayo_historial TO espacigo_runtime;
GRANT USAGE, SELECT ON SEQUENCE public.disputa_ensayo_historial_secuencia_seq TO espacigo_runtime;
