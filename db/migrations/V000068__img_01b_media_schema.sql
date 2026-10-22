-- IMG-01B: additive relational model for real media. Synthetic gallery rows
-- remain in their historical tables and are represented only in the shared
-- ordering table.

CREATE TABLE public.media_asset (
    id uuid PRIMARY KEY,
    propietario_id uuid NOT NULL REFERENCES public.usuario(id) ON DELETE RESTRICT,
    espacio_id uuid,
    proposito text NOT NULL CHECK (proposito IN ('espacio','perfil')),
    estado text NOT NULL CHECK (estado IN ('pending','ready','deleting','deleted','failed')),
    idempotency_key text NOT NULL CHECK (length(btrim(idempotency_key)) BETWEEN 8 AND 128),
    input_sha256 char(64) NOT NULL CHECK (input_sha256 ~ '^[0-9a-f]{64}$'),
    fencing_token bigint NOT NULL DEFAULT 1 CHECK (fencing_token > 0),
    lease_until timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz,
    CONSTRAINT media_asset_space_owner_fk FOREIGN KEY (espacio_id, propietario_id)
        REFERENCES public.espacio(id, propietario_id) ON DELETE RESTRICT,
    CONSTRAINT media_asset_purpose_scope_ck CHECK (
        (proposito='espacio' AND espacio_id IS NOT NULL)
        OR (proposito='perfil' AND espacio_id IS NULL)
    ),
    CONSTRAINT media_asset_terminal_ck CHECK (
        (estado='deleted' AND deleted_at IS NOT NULL)
        OR (estado<>'deleted' AND deleted_at IS NULL)
    ),
    CONSTRAINT media_asset_owner_id_uq UNIQUE (id,propietario_id),
    CONSTRAINT media_asset_space_owner_id_uq UNIQUE (id,espacio_id,propietario_id)
);
CREATE INDEX media_asset_space_state_idx ON public.media_asset(espacio_id,estado)
    WHERE proposito='espacio' AND estado IN ('pending','ready');
CREATE INDEX media_asset_owner_profile_idx ON public.media_asset(propietario_id,created_at DESC)
    WHERE proposito='perfil';
CREATE UNIQUE INDEX media_asset_space_idempotency_uq
    ON public.media_asset(propietario_id,espacio_id,idempotency_key) WHERE proposito='espacio';
CREATE UNIQUE INDEX media_asset_profile_idempotency_uq
    ON public.media_asset(propietario_id,idempotency_key) WHERE proposito='perfil';

CREATE TABLE public.media_variant (
    media_id uuid NOT NULL REFERENCES public.media_asset(id) ON DELETE RESTRICT,
    variant text NOT NULL CHECK (variant IN ('display','thumbnail','avatar')),
    provider text NOT NULL CHECK (provider='gcs'),
    bucket_name text NOT NULL CHECK (length(btrim(bucket_name)) BETWEEN 1 AND 222),
    object_key text NOT NULL CHECK (length(btrim(object_key)) BETWEEN 1 AND 1024),
    generation bigint NOT NULL CHECK (generation > 0),
    mime_type text NOT NULL CHECK (mime_type='image/webp'),
    size_bytes bigint NOT NULL CHECK (size_bytes BETWEEN 1 AND 10485760),
    sha256 char(64) NOT NULL CHECK (sha256 ~ '^[0-9a-f]{64}$'),
    width integer NOT NULL CHECK (width BETWEEN 1 AND 1600),
    height integer NOT NULL CHECK (height BETWEEN 1 AND 1600),
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (media_id,variant),
    CONSTRAINT media_variant_asset_uq UNIQUE (media_id,variant,object_key),
    CONSTRAINT media_variant_dims_ck CHECK (width::bigint * height::bigint <= 25000000),
    CONSTRAINT media_variant_purpose_ck CHECK (
        (variant='avatar' AND width<=512 AND height<=512)
        OR (variant='thumbnail' AND width<=480 AND height<=480)
        OR (variant='display' AND width<=1600 AND height<=1600)
    )
);
CREATE UNIQUE INDEX media_variant_object_generation_uq
    ON public.media_variant(provider,bucket_name,object_key,generation);

CREATE TABLE public.space_media (
    id uuid PRIMARY KEY,
    espacio_id uuid NOT NULL,
    propietario_id uuid NOT NULL,
    media_id uuid NOT NULL UNIQUE,
    alt_text varchar(250) NOT NULL DEFAULT '' CHECK (length(alt_text)<=250),
    visible boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    retired_at timestamptz,
    CONSTRAINT space_media_space_owner_fk FOREIGN KEY (espacio_id,propietario_id)
        REFERENCES public.espacio(id,propietario_id) ON DELETE RESTRICT,
    CONSTRAINT space_media_asset_scope_fk FOREIGN KEY (media_id,espacio_id,propietario_id)
        REFERENCES public.media_asset(id,espacio_id,propietario_id) ON DELETE RESTRICT,
    CONSTRAINT space_media_id_scope_uq UNIQUE (espacio_id,propietario_id,id),
    CONSTRAINT space_media_retirement_ck CHECK ((retired_at IS NULL) OR NOT visible)
);
CREATE INDEX space_media_active_idx ON public.space_media(espacio_id,created_at,id)
    WHERE retired_at IS NULL;

CREATE TABLE public.profile_media (
    propietario_id uuid PRIMARY KEY REFERENCES public.usuario(id) ON DELETE RESTRICT,
    media_id uuid NOT NULL UNIQUE,
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT profile_media_asset_owner_fk FOREIGN KEY (media_id,propietario_id)
        REFERENCES public.media_asset(id,propietario_id) ON DELETE RESTRICT
);

CREATE FUNCTION public.img01b_check_media_relation_scope() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE asset public.media_asset%ROWTYPE;
BEGIN
    SELECT * INTO asset FROM public.media_asset WHERE id=NEW.media_id;
    IF NOT FOUND OR asset.propietario_id IS DISTINCT FROM NEW.propietario_id THEN
        RAISE EXCEPTION 'media owner mismatch' USING ERRCODE='23503';
    END IF;
    IF TG_TABLE_NAME='space_media' AND (asset.proposito<>'espacio' OR asset.espacio_id IS DISTINCT FROM NEW.espacio_id) THEN
        RAISE EXCEPTION 'space media scope mismatch' USING ERRCODE='23514';
    END IF;
    IF TG_TABLE_NAME='profile_media' AND asset.proposito<>'perfil' THEN
        RAISE EXCEPTION 'profile media purpose mismatch' USING ERRCODE='23514';
    END IF;
    RETURN NEW;
END
$$;
CREATE TRIGGER space_media_scope_guard BEFORE INSERT OR UPDATE OF media_id,propietario_id,espacio_id
    ON public.space_media FOR EACH ROW EXECUTE FUNCTION public.img01b_check_media_relation_scope();
CREATE TRIGGER profile_media_scope_guard BEFORE INSERT OR UPDATE OF media_id,propietario_id
    ON public.profile_media FOR EACH ROW EXECUTE FUNCTION public.img01b_check_media_relation_scope();

CREATE TABLE public.gallery_item_order (
    id uuid PRIMARY KEY,
    espacio_id uuid NOT NULL,
    propietario_id uuid NOT NULL,
    synthetic_photo_id uuid,
    space_media_id uuid,
    position smallint NOT NULL CHECK (position BETWEEN 1 AND 10),
    CONSTRAINT gallery_order_exactly_one_ck CHECK ((synthetic_photo_id IS NULL) <> (space_media_id IS NULL)),
    CONSTRAINT gallery_order_space_owner_fk FOREIGN KEY (espacio_id,propietario_id)
        REFERENCES public.espacio(id,propietario_id) ON DELETE RESTRICT,
    CONSTRAINT gallery_order_synthetic_fk FOREIGN KEY (espacio_id,propietario_id,synthetic_photo_id)
        REFERENCES public.espacio_galeria_sintetica_local(espacio_id,propietario_id,id) ON DELETE RESTRICT,
    CONSTRAINT gallery_order_real_fk FOREIGN KEY (espacio_id,propietario_id,space_media_id)
        REFERENCES public.space_media(espacio_id,propietario_id,id) ON DELETE RESTRICT,
    CONSTRAINT gallery_order_synthetic_uq UNIQUE (synthetic_photo_id),
    CONSTRAINT gallery_order_real_uq UNIQUE (space_media_id),
    CONSTRAINT gallery_order_position_deferred_uq UNIQUE (espacio_id,position)
        DEFERRABLE INITIALLY DEFERRED
);

ALTER TABLE public.espacio ADD COLUMN portada_media_id uuid;
ALTER TABLE public.espacio
    ADD CONSTRAINT espacio_portada_media_fk
        FOREIGN KEY (id,propietario_id,portada_media_id)
        REFERENCES public.space_media(espacio_id,propietario_id,id) ON DELETE RESTRICT,
    ADD CONSTRAINT espacio_single_cover_ck CHECK (portada_foto_id IS NULL OR portada_media_id IS NULL);
COMMENT ON COLUMN public.espacio.portada_media_id IS
    'Selección de portada real propia; mutuamente excluyente con portada_foto_id.';

-- The historical source is not copied or rewritten. Refuse an invalid legacy
-- gallery rather than silently truncating or changing its order.
DO $backfill_guard$
BEGIN
    IF EXISTS (
        SELECT 1 FROM public.espacio_galeria_sintetica_local
        WHERE estado='activa'
        GROUP BY espacio_id HAVING count(*) > 10
    ) THEN
        RAISE EXCEPTION 'active synthetic gallery exceeds the common limit of 10';
    END IF;
END
$backfill_guard$;

INSERT INTO public.gallery_item_order(id,espacio_id,propietario_id,synthetic_photo_id,position)
SELECT gen_random_uuid(),ranked.espacio_id,ranked.propietario_id,ranked.id,ranked.position::smallint
FROM (
    SELECT g.espacio_id,g.propietario_id,g.id,
           row_number() OVER (PARTITION BY g.espacio_id ORDER BY g.creada_en,g.id) AS position
    FROM public.espacio_galeria_sintetica_local g
    WHERE g.estado='activa'
) AS ranked;

-- New real-space reservations serialize on the space row and reserve from the
-- same ten slots as active historical gallery entries and in-flight candidates.
CREATE FUNCTION public.img01b_reserve_media_slot() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, public AS $$
DECLARE
    occupied bigint;
BEGIN
    IF NEW.proposito <> 'espacio' OR NEW.estado NOT IN ('pending','ready') THEN
        RETURN NEW;
    END IF;
    PERFORM 1 FROM public.espacio WHERE id=NEW.espacio_id AND propietario_id=NEW.propietario_id FOR UPDATE;
    IF NOT FOUND THEN RAISE EXCEPTION 'space owner mismatch' USING ERRCODE='23503'; END IF;
    SELECT
      (SELECT count(*) FROM public.espacio_galeria_sintetica_local g WHERE g.espacio_id=NEW.espacio_id AND g.estado='activa') +
      (SELECT count(*) FROM public.espacio_galeria_archivo_candidato_local c WHERE c.espacio_id=NEW.espacio_id AND c.estado='reservado') +
      (SELECT count(*) FROM public.media_asset a WHERE a.espacio_id=NEW.espacio_id AND a.proposito='espacio' AND a.estado IN ('pending','ready'))
    INTO occupied;
    IF occupied >= 10 THEN RAISE EXCEPTION 'space media limit of 10 reached' USING ERRCODE='23514'; END IF;
    RETURN NEW;
END
$$;
CREATE TRIGGER media_asset_slot_reservation
    BEFORE INSERT ON public.media_asset
    FOR EACH ROW EXECUTE FUNCTION public.img01b_reserve_media_slot();

-- Explicit application grants for the existing restricted runtime role. No
-- role is created here and no PUBLIC/default grants are introduced.
DO $runtime_grants$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname='espacigo_runtime') THEN
        GRANT SELECT, INSERT, UPDATE, DELETE ON TABLE
            public.media_asset, public.media_variant, public.space_media,
            public.profile_media, public.gallery_item_order TO espacigo_runtime;
    END IF;
END
$runtime_grants$;
REVOKE EXECUTE ON FUNCTION public.img01b_reserve_media_slot() FROM PUBLIC;
REVOKE EXECUTE ON FUNCTION public.img01b_check_media_relation_scope() FROM PUBLIC;
