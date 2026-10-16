-- Fixed GCP-03 phase: expose migration history to the exact database owner
-- login for the subsequent audited retirement check. No grant option or PUBLIC.
GRANT SELECT ON TABLE public.schema_migrations TO postgres;
