-- Synthetic-only geography for explicitly enabled local booking fixtures.
-- Coordinates are declared by category and never derived from draft addresses.
CREATE EXTENSION IF NOT EXISTS postgis;

CREATE TABLE public.reserva_ensayo_local_ubicacion_sintetica (
    espacio_id uuid PRIMARY KEY
        REFERENCES public.reserva_ensayo_local_fixture(espacio_id) ON DELETE CASCADE,
    latitud double precision NOT NULL CHECK (latitud BETWEEN -90 AND 90),
    longitud double precision NOT NULL CHECK (longitud BETWEEN -180 AND 180),
    punto geography(Point,4326) GENERATED ALWAYS AS (
        ST_SetSRID(ST_MakePoint(longitud, latitud),4326)::geography
    ) STORED,
    es_sintetica boolean NOT NULL DEFAULT true CHECK (es_sintetica)
);

CREATE INDEX reserva_ensayo_local_ubicacion_gist
    ON public.reserva_ensayo_local_ubicacion_sintetica USING gist (punto);

-- Stable category samples around the synthetic mock center (-33.4560,-70.6693).
-- The gaps make inclusion/exclusion visible for the supported radii.
INSERT INTO public.reserva_ensayo_local_ubicacion_sintetica(espacio_id,latitud,longitud,es_sintetica)
SELECT f.espacio_id,
       CASE e.categoria_codigo
           WHEN 'oficina' THEN -33.4560
           WHEN 'sala_multiproposito' THEN -33.4632
           WHEN 'bodega' THEN -33.4785
           WHEN 'estacionamiento' THEN -33.4965
           WHEN 'local_flexible' THEN -33.5415
           WHEN 'stand' THEN -33.5505
           WHEN 'quincho' THEN -33.6765
           WHEN 'parcela_eventos' THEN -33.6855
           ELSE -33.4560
       END,
       -70.6693,
       true
FROM public.reserva_ensayo_local_fixture f
JOIN public.espacio e ON e.id=f.espacio_id
WHERE f.habilitada
ON CONFLICT (espacio_id) DO NOTHING;
