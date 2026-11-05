-- Expand the ordered gallery without rewriting the original schema migration.
ALTER TABLE public.gallery_item_order
    DROP CONSTRAINT gallery_item_order_position_check,
    ADD CONSTRAINT gallery_item_order_position_check CHECK (position BETWEEN 1 AND 30);

-- A pending replacement claims the existing real-media slot, not a new slot.
ALTER TABLE public.media_asset
    ADD COLUMN replacement_target_space_media_id uuid,
    ADD CONSTRAINT media_asset_replacement_target_fk
        FOREIGN KEY (espacio_id,propietario_id,replacement_target_space_media_id)
        REFERENCES public.space_media(espacio_id,propietario_id,id) ON DELETE RESTRICT;

CREATE UNIQUE INDEX media_asset_pending_replacement_target_uq
    ON public.media_asset(espacio_id,propietario_id,replacement_target_space_media_id)
    WHERE estado='pending' AND replacement_target_space_media_id IS NOT NULL;

CREATE OR REPLACE FUNCTION public.img01b_enforce_gallery_slot() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, public AS $$
DECLARE
    target_space uuid;
    target_owner uuid;
    candidate_exclusion uuid;
    gallery_exclusion uuid;
    asset_exclusion uuid;
    old_asset_exclusion uuid;
    occupied bigint;
    claims_slot boolean;
    validate_target boolean := false;
BEGIN
    candidate_exclusion := NULL;
    gallery_exclusion := NULL;
    asset_exclusion := NULL;
    old_asset_exclusion := NULL;

    IF TG_TABLE_NAME='media_asset' THEN
        IF TG_OP='UPDATE' AND (NEW.id,NEW.propietario_id,NEW.espacio_id,NEW.proposito,
                               NEW.replacement_target_space_media_id)
             IS DISTINCT FROM (OLD.id,OLD.propietario_id,OLD.espacio_id,OLD.proposito,
                               OLD.replacement_target_space_media_id) THEN
            RAISE EXCEPTION 'media asset scope is immutable' USING ERRCODE='23514';
        END IF;
        IF TG_OP='INSERT' AND NEW.replacement_target_space_media_id IS NOT NULL
           AND (NEW.proposito IS DISTINCT FROM 'espacio' OR NEW.estado IS DISTINCT FROM 'pending') THEN
            RAISE EXCEPTION 'replacement must start as a pending space asset' USING ERRCODE='23514';
        END IF;
        IF TG_OP='UPDATE' AND OLD.estado='failed' AND NEW.estado='ready'
           AND NEW.replacement_target_space_media_id IS NOT NULL THEN
            RAISE EXCEPTION 'failed replacement must re-enter pending before ready' USING ERRCODE='23514';
        END IF;
        validate_target := NEW.replacement_target_space_media_id IS NOT NULL
            AND ((TG_OP='INSERT' AND NEW.estado='pending')
                 OR (TG_OP='UPDATE' AND NEW.estado='pending' AND OLD.estado<>'pending')
                 OR (TG_OP='UPDATE' AND OLD.estado='pending' AND NEW.estado='ready'));
        IF NEW.proposito<>'espacio' OR NEW.estado NOT IN ('pending','ready') THEN RETURN NEW; END IF;
        target_space := NEW.espacio_id;
        target_owner := NEW.propietario_id;
        asset_exclusion := NEW.id;
        claims_slot := NOT (TG_OP='UPDATE' AND OLD.estado IN ('pending','ready'));
    ELSIF TG_TABLE_NAME='espacio_galeria_archivo_candidato_local' THEN
        IF TG_OP='UPDATE' AND (NEW.archivo_id,NEW.propietario_id,NEW.espacio_id)
             IS DISTINCT FROM (OLD.archivo_id,OLD.propietario_id,OLD.espacio_id) THEN
            RAISE EXCEPTION 'synthetic candidate scope is immutable' USING ERRCODE='23514';
        END IF;
        IF NEW.estado<>'reservado' THEN RETURN NEW; END IF;
        IF TG_OP='UPDATE' AND OLD.estado='reservado' THEN RETURN NEW; END IF;
        target_space := NEW.espacio_id;
        target_owner := NEW.propietario_id;
        candidate_exclusion := NEW.archivo_id;
        claims_slot := true;
    ELSE
        IF TG_OP='UPDATE' AND (NEW.id,NEW.espacio_id,NEW.propietario_id)
             IS DISTINCT FROM (OLD.id,OLD.espacio_id,OLD.propietario_id) THEN
            RAISE EXCEPTION 'synthetic gallery scope is immutable' USING ERRCODE='23514';
        END IF;
        IF NEW.estado<>'activa' THEN RETURN NEW; END IF;
        IF TG_OP='UPDATE' AND OLD.estado='activa' THEN RETURN NEW; END IF;
        target_space := NEW.espacio_id;
        target_owner := NEW.propietario_id;
        candidate_exclusion := NEW.archivo_id;
        gallery_exclusion := NEW.id;
        claims_slot := true;
    END IF;

    IF NOT claims_slot AND NOT validate_target THEN RETURN NEW; END IF;
    PERFORM 1 FROM public.espacio WHERE id=target_space AND propietario_id=target_owner FOR UPDATE;
    IF NOT FOUND THEN RAISE EXCEPTION 'space owner mismatch' USING ERRCODE='23503'; END IF;

    -- Validate and lock the active slot and its real ready asset only after the
    -- owning space lock, serializing replacement checks with slot retirement.
    IF validate_target THEN
        PERFORM 1
        FROM public.space_media sm
        JOIN public.media_asset prior ON prior.id=sm.media_id
        WHERE sm.id=NEW.replacement_target_space_media_id
          AND sm.espacio_id=target_space
          AND sm.propietario_id=target_owner
          AND sm.retired_at IS NULL
          AND prior.proposito='espacio'
          AND prior.espacio_id=target_space
          AND prior.propietario_id=target_owner
          AND prior.estado='ready'
        FOR UPDATE OF sm,prior;
        IF NOT FOUND THEN
            RAISE EXCEPTION 'invalid replacement target' USING ERRCODE='23514';
        END IF;
    END IF;
    IF NOT claims_slot THEN RETURN NEW; END IF;

    -- Exclude only the ready asset behind this validated active slot while its
    -- pending replacement exists; all other pending/ready assets remain counted.
    IF TG_TABLE_NAME='media_asset' AND NEW.replacement_target_space_media_id IS NOT NULL THEN
        SELECT sm.media_id INTO old_asset_exclusion
        FROM public.space_media sm
        WHERE sm.id=NEW.replacement_target_space_media_id
          AND sm.espacio_id=target_space AND sm.propietario_id=target_owner
          AND sm.retired_at IS NULL;
    END IF;
    SELECT
      (SELECT count(*) FROM public.espacio_galeria_sintetica_local g
       WHERE g.espacio_id=target_space AND g.estado='activa' AND g.id IS DISTINCT FROM gallery_exclusion) +
      (SELECT count(*) FROM public.espacio_galeria_archivo_candidato_local c
       WHERE c.espacio_id=target_space AND c.estado='reservado' AND c.archivo_id IS DISTINCT FROM candidate_exclusion) +
      (SELECT count(*) FROM public.media_asset a
       WHERE a.espacio_id=target_space AND a.proposito='espacio' AND a.estado IN ('pending','ready')
         AND a.id IS DISTINCT FROM asset_exclusion
         AND a.id IS DISTINCT FROM old_asset_exclusion
         AND NOT EXISTS (
             SELECT 1 FROM public.media_asset replacement
             WHERE replacement.espacio_id=target_space
               AND replacement.propietario_id=target_owner
               AND replacement.estado='pending'
               AND replacement.replacement_target_space_media_id IS NOT NULL
               AND EXISTS (SELECT 1 FROM public.space_media old_slot
                           WHERE old_slot.id=replacement.replacement_target_space_media_id
                             AND old_slot.media_id=a.id
                             AND old_slot.retired_at IS NULL)
         ))
    INTO occupied;
    IF occupied >= 30 THEN RAISE EXCEPTION 'space media limit of 30 reached' USING ERRCODE='23514'; END IF;
    RETURN NEW;
END
$$;

-- Keep historical V1 keys intact; V2 is limited to the approved space-image
-- path and UUID/timestamp/fencing/variant WebP filename structure.
ALTER TABLE public.media_variant_intent
    DROP CONSTRAINT media_variant_intent_object_key_check,
    ADD CONSTRAINT media_variant_intent_object_key_check CHECK (
        length(object_key) BETWEEN 1 AND 1024
        AND (
            object_key LIKE 'media/v1/%'
            OR object_key ~ '^users/[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}/spaces/[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}/images/v2/[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}/[0-9]{8}T[0-9]{6}[.][0-9]{9}Z_[1-9][0-9]*_(display|thumbnail)[.]webp$'
        )
    );
