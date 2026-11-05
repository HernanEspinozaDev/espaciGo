-- Persist the compare-and-swap revision as part of a gallery replacement's
-- idempotency request identity. Existing replacement rows remain NULL and
-- are rejected on replay because their original revision cannot be recovered.
ALTER TABLE public.media_asset
    ADD COLUMN replacement_expected_revision timestamptz;

CREATE OR REPLACE FUNCTION public.img01b_guard_replacement_request_revision() RETURNS trigger
LANGUAGE plpgsql SET search_path = pg_catalog, public AS $$
BEGIN
    IF TG_OP='INSERT' THEN
        IF (NEW.replacement_target_space_media_id IS NULL)
           <> (NEW.replacement_expected_revision IS NULL) THEN
            RAISE EXCEPTION 'replacement target and expected revision must be supplied together'
                USING ERRCODE='23514';
        END IF;
    ELSIF (NEW.replacement_target_space_media_id,NEW.replacement_expected_revision)
          IS DISTINCT FROM
          (OLD.replacement_target_space_media_id,OLD.replacement_expected_revision) THEN
        RAISE EXCEPTION 'replacement request identity is immutable'
            USING ERRCODE='23514';
    END IF;
    RETURN NEW;
END
$$;

CREATE TRIGGER media_asset_replacement_request_revision_guard
BEFORE INSERT OR UPDATE OF replacement_target_space_media_id,replacement_expected_revision
ON public.media_asset
FOR EACH ROW EXECUTE FUNCTION public.img01b_guard_replacement_request_revision();
