-- Record when an object-store writer has returned. Cleanup can then distinguish
-- an in-flight write from a completed write whose object does not exist.
ALTER TABLE public.media_variant_intent
    ADD COLUMN write_complete boolean NOT NULL DEFAULT false;

DO $runtime_grants$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname='espacigo_runtime') THEN
        GRANT SELECT, INSERT, UPDATE, DELETE ON TABLE public.media_variant_intent TO espacigo_runtime;
    END IF;
END
$runtime_grants$;
