-- Guard the replacement-only record field when the shared slot trigger runs
-- for the synthetic gallery/candidate tables, whose row types lack that field.
DO $migration$
DECLARE
    function_definition text;
    original_fragment CONSTANT text :=
        'IF TG_TABLE_NAME=''media_asset'' AND NEW.replacement_target_space_media_id IS NOT NULL THEN';
    safe_fragment CONSTANT text :=
        'IF TG_TABLE_NAME=''media_asset'' AND (to_jsonb(NEW)->>''replacement_target_space_media_id'') IS NOT NULL THEN';
BEGIN
    SELECT pg_get_functiondef('public.img01b_enforce_gallery_slot()'::regprocedure)
    INTO function_definition;

    IF position(original_fragment IN function_definition) = 0
       OR position(original_fragment IN replace(function_definition, original_fragment, '')) > 0 THEN
        RAISE EXCEPTION 'expected exactly one replacement-only record-field guard in img01b_enforce_gallery_slot';
    END IF;

    EXECUTE replace(function_definition, original_fragment, safe_fragment);
END;
$migration$;
