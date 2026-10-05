-- Complete deterministic display order on every M04 v1 category profile.
UPDATE public.categoria_perfil_atributos AS profile
SET perfil = jsonb_set(
    profile.perfil,
    '{attributes}',
    (
        SELECT jsonb_agg(
            CASE
                WHEN item.value ? 'order' THEN item.value
                ELSE item.value || jsonb_build_object('order', item.ordinality)
            END
            ORDER BY item.ordinality
        )
        FROM jsonb_array_elements(profile.perfil->'attributes') WITH ORDINALITY AS item(value, ordinality)
    )
)
WHERE EXISTS (
    SELECT 1
    FROM jsonb_array_elements(profile.perfil->'attributes') AS item(value)
    WHERE NOT item.value ? 'order'
);
