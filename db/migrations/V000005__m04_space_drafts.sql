-- M04 local slice: private owner-scoped drafts only. Commercial lifecycle,
-- geocoding, media and calendar are intentionally introduced by later cuts.
CREATE TABLE public.categoria_espacio (
    codigo text PRIMARY KEY,
    nombre text NOT NULL UNIQUE,
    orden smallint NOT NULL UNIQUE CHECK (orden > 0),
    activa boolean NOT NULL DEFAULT true
);

INSERT INTO public.categoria_espacio (codigo, nombre, orden) VALUES
    ('oficina', 'Oficina', 1),
    ('sala_multiproposito', 'Sala o espacio multipropósito', 2),
    ('bodega', 'Bodega', 3),
    ('estacionamiento', 'Estacionamiento', 4),
    ('local_flexible', 'Local flexible', 5),
    ('stand', 'Stand', 6),
    ('quincho', 'Quincho', 7),
    ('parcela_eventos', 'Parcela o espacio para eventos', 8);

CREATE TABLE public.espacio (
    id uuid PRIMARY KEY,
    propietario_id uuid NOT NULL REFERENCES public.usuario(id) ON DELETE RESTRICT,
    categoria_codigo text NOT NULL REFERENCES public.categoria_espacio(codigo) ON DELETE RESTRICT,
    titulo varchar(70) NOT NULL CHECK (length(btrim(titulo)) BETWEEN 1 AND 70),
    descripcion text NOT NULL CHECK (length(btrim(descripcion)) >= 100),
    superficie_m2 numeric(10,2) NOT NULL CHECK (superficie_m2 > 0),
    capacidad_maxima integer NOT NULL CHECK (capacidad_maxima > 0),
    reglas_uso varchar(250) NOT NULL CHECK (length(btrim(reglas_uso)) BETWEEN 1 AND 250),
    modalidad_tarifa text NOT NULL CHECK (modalidad_tarifa IN ('hora', 'dia', 'mes')),
    precio_base_clp bigint NOT NULL CHECK (precio_base_clp > 5000),
    direccion text NOT NULL CHECK (length(btrim(direccion)) BETWEEN 1 AND 500),
    estado text NOT NULL DEFAULT 'borrador' CHECK (estado = 'borrador'),
    creado_en timestamptz NOT NULL DEFAULT now(),
    actualizado_en timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX espacio_propietario_actualizacion_idx
    ON public.espacio (propietario_id, actualizado_en DESC, id);
