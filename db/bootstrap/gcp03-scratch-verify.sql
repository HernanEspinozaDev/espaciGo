-- REVIEW ONLY: read-only post-migration checks for a disposable scratch database.
\set ON_ERROR_STOP on

SELECT current_database() AS database_name,
       current_setting('server_version') AS postgres_version;

SELECT extname, extversion
FROM pg_extension
WHERE extname IN ('btree_gist', 'postgis', 'unaccent')
ORDER BY extname;

DO $$
BEGIN
    IF (SELECT count(*) FROM pg_extension WHERE extname IN ('btree_gist', 'postgis', 'unaccent')) <> 3 THEN
        RAISE EXCEPTION 'one or more expected extensions are missing';
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_roles
        WHERE rolname = 'espacigo_gadm_bootstrap'
          AND NOT rolcanlogin AND NOT rolsuper AND NOT rolcreatedb AND NOT rolcreaterole
          AND NOT rolreplication AND NOT rolbypassrls
    ) THEN
        RAISE EXCEPTION 'V60 constrained espacigo_gadm_bootstrap role is missing or has unsafe attributes';
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_roles
        WHERE rolname = 'espacigo_runtime'
          AND NOT rolcanlogin AND NOT rolsuper AND NOT rolcreatedb AND NOT rolcreaterole
          AND NOT rolreplication AND NOT rolbypassrls AND NOT rolinherit
    ) THEN
        RAISE EXCEPTION 'restricted NOLOGIN espacigo_runtime role is missing or has unsafe attributes';
    END IF;
END
$$;
