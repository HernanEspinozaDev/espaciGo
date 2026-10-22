-- Persist each generated object key before the external write. This lets the
-- bounded cleanup worker recover the narrow GCS/Postgres crash window.
CREATE TABLE public.media_variant_intent (
    media_id uuid NOT NULL REFERENCES public.media_asset(id) ON DELETE RESTRICT,
    variant text NOT NULL CHECK (variant IN ('display','thumbnail','avatar')),
    object_key text NOT NULL CHECK (object_key LIKE 'media/v1/%' AND length(object_key) BETWEEN 1 AND 1024),
    provider text CHECK (provider IN ('gcs','filesystem')),
    bucket_name text,
    generation bigint CHECK (generation IS NULL OR generation > 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (media_id,variant),
    UNIQUE (provider,bucket_name,object_key),
    CONSTRAINT media_variant_intent_storage_ck CHECK (
        (provider IS NULL AND bucket_name IS NULL AND generation IS NULL)
        OR (provider IS NOT NULL AND bucket_name IS NOT NULL AND generation IS NOT NULL)
    )
);
CREATE INDEX media_variant_intent_cleanup_idx ON public.media_variant_intent(media_id);
DO $runtime_grants$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname='espacigo_runtime') THEN
        GRANT SELECT, INSERT, UPDATE, DELETE ON TABLE public.media_variant_intent TO espacigo_runtime;
    END IF;
END
$runtime_grants$;
