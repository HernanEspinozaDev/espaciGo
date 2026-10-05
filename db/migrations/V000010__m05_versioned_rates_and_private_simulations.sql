-- M05 private price simulation for existing owner-scoped drafts.
-- Each rate revision is immutable; simulation rows are private snapshots.
ALTER TABLE public.espacio
    ADD CONSTRAINT espacio_id_propietario_uk UNIQUE (id, propietario_id);

CREATE TABLE public.tarifa_espacio (
    espacio_id uuid NOT NULL REFERENCES public.espacio(id) ON DELETE RESTRICT,
    version integer NOT NULL CHECK (version > 0),
    modalidad text NOT NULL CHECK (modalidad IN ('hora', 'dia', 'mes')),
    precio_base_clp bigint NOT NULL CHECK (precio_base_clp > 5000),
    moneda char(3) NOT NULL DEFAULT 'CLP' CHECK (moneda = 'CLP'),
    creada_en timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (espacio_id, version)
);

INSERT INTO public.tarifa_espacio (espacio_id, version, modalidad, precio_base_clp)
SELECT id, 1, modalidad_tarifa, precio_base_clp FROM public.espacio;

CREATE TABLE public.simulacion_precio_privada (
    id uuid PRIMARY KEY,
    espacio_id uuid NOT NULL,
    propietario_id uuid NOT NULL REFERENCES public.usuario(id) ON DELETE RESTRICT,
    tarifa_version integer NOT NULL,
    modalidad text NOT NULL CHECK (modalidad IN ('hora', 'dia', 'mes')),
    precio_unitario_clp bigint NOT NULL CHECK (precio_unitario_clp > 5000),
    moneda char(3) NOT NULL CHECK (moneda = 'CLP'),
    unidades_facturadas bigint NOT NULL CHECK (unidades_facturadas > 0),
    subtotal_clp bigint NOT NULL CHECK (subtotal_clp > 0),
    inicio timestamptz NOT NULL,
    termino timestamptz NOT NULL CHECK (termino > inicio),
    zona_horaria text NOT NULL CHECK (length(btrim(zona_horaria)) BETWEEN 1 AND 100),
    creada_en timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT simulacion_precio_tarifa_fk FOREIGN KEY (espacio_id, tarifa_version)
        REFERENCES public.tarifa_espacio(espacio_id, version) ON DELETE RESTRICT,
    CONSTRAINT simulacion_precio_espacio_propietario_fk FOREIGN KEY (espacio_id, propietario_id)
        REFERENCES public.espacio(id, propietario_id) ON DELETE RESTRICT
);

CREATE INDEX simulacion_precio_espacio_propietario_idx
    ON public.simulacion_precio_privada (espacio_id, propietario_id, creada_en DESC, id);
