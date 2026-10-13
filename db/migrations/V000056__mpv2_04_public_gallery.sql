-- MPV2-04: public exposure is an explicit, reversible per-photo decision.
-- Existing gallery photos remain private.
ALTER TABLE public.espacio_galeria_sintetica_local
    ADD COLUMN publicada_en_catalogo boolean NOT NULL DEFAULT false;

CREATE INDEX espacio_galeria_publicada_local_idx
    ON public.espacio_galeria_sintetica_local(espacio_id, id)
    WHERE estado='activa' AND publicada_en_catalogo AND archivo_id IS NOT NULL;
