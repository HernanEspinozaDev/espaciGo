-- GCP-03 scratch executor phase: narrow the database TEMP ACL, schema revision 1.
-- Run only through the exact database-owner phase after the temporary login
-- roles are preprovisioned and before V1-V61 start.
DO $guard$
BEGIN
    IF current_database() <> 'espacigo_scratch' THEN
        RAISE EXCEPTION 'GCP-03 database ACL refused unexpected database';
    END IF;
    IF current_user <> 'postgres' THEN
        RAISE EXCEPTION 'GCP-03 database ACL requires the reviewed database owner login';
    END IF;
    IF to_regclass('public.schema_migrations') IS NOT NULL THEN
        RAISE EXCEPTION 'GCP-03 database ACL must run before migrations';
    END IF;
END
$guard$;

REVOKE TEMPORARY ON DATABASE espacigo_scratch FROM PUBLIC;
GRANT TEMPORARY ON DATABASE espacigo_scratch TO espacigo_scratch_migrator;
REVOKE CREATE ON SCHEMA public FROM PUBLIC;
GRANT USAGE, CREATE ON SCHEMA public TO espacigo_scratch_migrator;
