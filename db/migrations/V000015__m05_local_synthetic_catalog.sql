-- Local M05 catalog: explicitly allowlisted synthetic drafts, never general
-- draft discovery. A quote retains the selected category/profile snapshot.
ALTER TABLE public.reserva_ensayo_local_fixture
    DROP CONSTRAINT reserva_ensayo_local_fixture_pkey,
    DROP CONSTRAINT reserva_ensayo_local_fixture_espacio_id_key,
    DROP CONSTRAINT reserva_ensayo_local_fixture_singleton_check,
    DROP COLUMN singleton,
    ADD COLUMN habilitada boolean NOT NULL DEFAULT true,
    ADD PRIMARY KEY (espacio_id);

ALTER TABLE public.cotizacion_reserva_ensayo
    ADD COLUMN categoria_codigo text,
    ADD COLUMN perfil_version integer,
    ADD COLUMN perfil_valores_snapshot jsonb;

UPDATE public.cotizacion_reserva_ensayo q
SET categoria_codigo=e.categoria_codigo,
    perfil_version=c.perfil_version,
    perfil_valores_snapshot=c.valores
FROM public.espacio e
JOIN public.espacio_caracteristicas c ON c.espacio_id=e.id
WHERE e.id=q.espacio_id;

ALTER TABLE public.cotizacion_reserva_ensayo
    ALTER COLUMN categoria_codigo SET NOT NULL,
    ALTER COLUMN perfil_version SET NOT NULL,
    ALTER COLUMN perfil_valores_snapshot SET NOT NULL,
    ALTER COLUMN perfil_valores_snapshot SET DEFAULT '{}'::jsonb,
    ADD CONSTRAINT cotizacion_ensayo_perfil_fk
        FOREIGN KEY (categoria_codigo,perfil_version)
        REFERENCES public.categoria_perfil_atributos(categoria_codigo,version)
        ON DELETE RESTRICT,
    ADD CONSTRAINT cotizacion_ensayo_snapshot_object_ck
        CHECK (jsonb_typeof(perfil_valores_snapshot)='object'
            AND octet_length(perfil_valores_snapshot::text)<=16384);

CREATE INDEX reserva_ensayo_local_fixture_enabled_idx
    ON public.reserva_ensayo_local_fixture(espacio_id)
    WHERE habilitada;
