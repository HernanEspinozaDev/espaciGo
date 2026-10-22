-- IMG-01C adds a private local filesystem adapter for isolated developer and
-- integration environments. Cloud objects continue to use the same media/v1
-- namespace with provider=gcs; no bucket or IAM setting is changed.
ALTER TABLE public.media_variant
    DROP CONSTRAINT media_variant_provider_check,
    ADD CONSTRAINT media_variant_provider_check CHECK (provider IN ('gcs','filesystem'));
