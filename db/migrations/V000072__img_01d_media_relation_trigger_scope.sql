-- IMG-01D runtime repair: keep V68 history immutable while avoiding record
-- field access on a table that does not have that field.
CREATE OR REPLACE FUNCTION public.img01b_check_media_relation_scope() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE asset public.media_asset%ROWTYPE;
BEGIN
    SELECT * INTO asset FROM public.media_asset WHERE id=NEW.media_id;
    IF NOT FOUND OR asset.propietario_id IS DISTINCT FROM NEW.propietario_id THEN
        RAISE EXCEPTION 'media owner mismatch' USING ERRCODE='23503';
    END IF;

    IF TG_TABLE_NAME='profile_media' THEN
        IF asset.proposito<>'perfil' THEN
            RAISE EXCEPTION 'profile media purpose mismatch' USING ERRCODE='23514';
        END IF;
        RETURN NEW;
    ELSIF TG_TABLE_NAME='space_media' THEN
        IF asset.proposito<>'espacio' OR asset.espacio_id IS DISTINCT FROM NEW.espacio_id THEN
            RAISE EXCEPTION 'space media scope mismatch' USING ERRCODE='23514';
        END IF;
        RETURN NEW;
    END IF;

    RAISE EXCEPTION 'unsupported media relation table: %', TG_TABLE_NAME USING ERRCODE='23514';
END
$$;
