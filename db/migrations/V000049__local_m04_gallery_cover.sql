-- LOCAL-M04-COVER-RATE-01: a cover is a selected owner-scoped synthetic
-- gallery image. Existing gallery rows remain unselected.
ALTER TABLE public.espacio
    ADD COLUMN portada_foto_id uuid;

ALTER TABLE public.espacio_galeria_sintetica_local
    ADD CONSTRAINT espacio_galeria_cover_owner_uq
    UNIQUE (espacio_id, propietario_id, id);

ALTER TABLE public.espacio
    ADD CONSTRAINT espacio_portada_galeria_fk
    FOREIGN KEY (id, propietario_id, portada_foto_id)
    REFERENCES public.espacio_galeria_sintetica_local(espacio_id, propietario_id, id)
    ON DELETE RESTRICT;

COMMENT ON COLUMN public.espacio.portada_foto_id IS
    'Selección de portada sintética propia; NULL cuando no hay selección vigente.';
